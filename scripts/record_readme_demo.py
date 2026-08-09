#!/usr/bin/env python3
"""Record the real Cortex TUI against a deterministic local OpenAI-compatible model.

The model is scripted, but Cortex is not mocked: the binary runs through its real
Bubble Tea UI and Axon executes real search/read/write/exec tools against a
disposable Go repository. The PTY is captured and rendered into the README GIF.
"""

from __future__ import annotations

import argparse
import http.server
import json
import os
from pathlib import Path
import socket
import tempfile
import threading
import time
from typing import Any

import pexpect
import pyte
from PIL import Image, ImageDraw, ImageFont

COLS = 112
ROWS = 31
POLL = 0.075

BG = "#0b0d12"
CHROME = "#17191f"
BORDER = "#2b2f39"
DEFAULT_FG = "#d8dce8"

ANSI = {
    "black": "#111318",
    "red": "#fca5a5",
    "green": "#86efac",
    "brown": "#fdba74",
    "blue": "#a5b4fc",
    "magenta": "#d8b4fe",
    "cyan": "#7dd3fc",
    "white": "#eef2ff",
    "brightblack": "#6b7280",
    "brightred": "#fecaca",
    "brightgreen": "#bbf7d0",
    "brightbrown": "#fed7aa",
    "brightblue": "#c7d2fe",
    "brightmagenta": "#e9d5ff",
    "brightcyan": "#a5f3fc",
    "brightwhite": "#ffffff",
}


def free_port() -> int:
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return int(s.getsockname()[1])


def chunk(delta: dict[str, Any]) -> dict[str, Any]:
    return {"choices": [{"delta": delta}]}


def tool_chunks(call_id: str, name: str, args: dict[str, Any]) -> list[dict[str, Any]]:
    encoded = json.dumps(args, separators=(",", ":"))
    pivot = max(1, len(encoded) // 2)
    return [
        chunk({"tool_calls": [{"index": 0, "id": call_id, "type": "function", "function": {"name": name, "arguments": ""}}]}),
        chunk({"tool_calls": [{"index": 0, "function": {"arguments": encoded[:pivot]}}]}),
        chunk({"tool_calls": [{"index": 0, "function": {"arguments": encoded[pivot:]}}]}),
    ]


SCRIPT = [
    tool_chunks("call_search", "search", {"query": "Greeting", "path": "."}),
    tool_chunks("call_read", "read", {"path": "greeter.go", "mode": "full"}),
    tool_chunks(
        "call_write",
        "write",
        {
            "path": "greeter.go",
            "mode": "replace_string",
            "old_str": 'return "hello"',
            "content": 'return "hello, cortex"',
        },
    ),
    tool_chunks(
        "call_test",
        "exec",
        {
            "command": "go test ./...",
            "tail_lines": 20,
            "expected_outcome": "all tests pass",
        },
    ),
    [
        chunk({"content": "Fixed the failing greeting with one targeted edit. "}),
        chunk({"content": "`go test ./...` passes. "}),
        chunk({"content": "No unrelated files changed."}),
    ],
]


class DemoHandler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    calls = 0

    def log_message(self, *_: Any) -> None:
        pass

    def do_POST(self) -> None:  # noqa: N802
        if self.path != "/v1/chat/completions":
            self.send_error(404)
            return

        length = int(self.headers.get("Content-Length", "0"))
        if length:
            self.rfile.read(length)

        index = min(DemoHandler.calls, len(SCRIPT) - 1)
        DemoHandler.calls += 1

        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "close")
        self.end_headers()

        # Make the real Cortex spinner/tool lifecycle visible rather than
        # returning the entire scripted model response in one scheduler tick.
        time.sleep(0.38)
        for item in SCRIPT[index]:
            payload = f"data: {json.dumps(item)}\n\n".encode()
            self.wfile.write(payload)
            self.wfile.flush()
            time.sleep(0.16 if "tool_calls" in item["choices"][0]["delta"] else 0.12)
        self.wfile.write(b"data: [DONE]\n\n")
        self.wfile.flush()
        self.close_connection = True


class Server(http.server.ThreadingHTTPServer):
    daemon_threads = True


def setup_demo(root: Path, port: int) -> tuple[Path, dict[str, str]]:
    home = root / "home"
    repo = root / "repo"
    (home / ".config" / "axon").mkdir(parents=True)
    (home / ".config" / "cortex").mkdir(parents=True)
    repo.mkdir(parents=True)

    (home / ".config" / "axon" / "axon.yaml").write_text(
        f"""providers:\n  demo:\n    base_url: http://127.0.0.1:{port}\n    models:\n      cortex-demo:\nmodel:\n  exclude_reasoning: true\n  request_timeout: 30s\n  idle_timeout: 5s\n"""
    )
    (home / ".config" / "cortex" / "config.yaml").write_text(
        "provider: demo\nmodel: cortex-demo\n"
    )

    (repo / "go.mod").write_text("module example.com/cortex-demo\n\ngo 1.26.2\n")
    (repo / "greeter.go").write_text(
        'package demo\n\nfunc Greeting() string {\n\treturn "hello"\n}\n'
    )
    (repo / "greeter_test.go").write_text(
        'package demo\n\nimport "testing"\n\nfunc TestGreeting(t *testing.T) {\n\tif got := Greeting(); got != "hello, cortex" {\n\t\tt.Fatalf("Greeting() = %q, want %q", got, "hello, cortex")\n\t}\n}\n'
    )

    env = os.environ.copy()
    env.update(
        {
            "HOME": str(home),
            "XDG_CONFIG_HOME": str(home / ".config"),
            "XDG_CACHE_HOME": str(home / ".cache"),
            "XDG_DATA_HOME": str(home / ".local" / "share"),
            "TERM": "xterm-256color",
            "COLORTERM": "truecolor",
            "NO_COLOR": "",
        }
    )
    return repo, env


def color(value: str) -> str:
    if not value or value == "default":
        return DEFAULT_FG
    if value in ANSI:
        return ANSI[value]
    raw = value.lstrip("#")
    if len(raw) == 6 and all(c in "0123456789abcdefABCDEF" for c in raw):
        return "#" + raw
    if len(raw) == 3 and all(c in "0123456789abcdefABCDEF" for c in raw):
        return "#" + "".join(c * 2 for c in raw)
    return DEFAULT_FG


def fonts(size: int) -> tuple[ImageFont.FreeTypeFont, ImageFont.FreeTypeFont]:
    regular_candidates = [
        "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf",
        "/usr/share/fonts/truetype/liberation2/LiberationMono-Regular.ttf",
    ]
    bold_candidates = [
        "/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Bold.ttf",
        "/usr/share/fonts/truetype/liberation2/LiberationMono-Bold.ttf",
    ]
    regular = next(Path(p) for p in regular_candidates if Path(p).exists())
    bold = next(Path(p) for p in bold_candidates if Path(p).exists())
    return ImageFont.truetype(str(regular), size), ImageFont.truetype(str(bold), size)


def render(screen: pyte.Screen) -> Image.Image:
    font, font_bold = fonts(18)
    bbox = font.getbbox("M")
    cell_w = bbox[2] - bbox[0]
    cell_h = 23
    pad_x, pad_y = 24, 18
    title_h = 42
    width = COLS * cell_w + pad_x * 2
    height = ROWS * cell_h + pad_y * 2 + title_h

    image = Image.new("RGB", (width, height), BG)
    d = ImageDraw.Draw(image)
    d.rounded_rectangle((1, 1, width - 2, height - 2), radius=16, fill=BG, outline=BORDER, width=2)
    d.rounded_rectangle((2, 2, width - 3, title_h), radius=14, fill=CHROME)
    d.rectangle((2, title_h - 14, width - 3, title_h), fill=CHROME)
    for x, fill in ((22, "#ef6b63"), (43, "#e7bd52"), (64, "#62c554")):
        d.ellipse((x - 5, 16, x + 5, 26), fill=fill)
    d.text((width // 2, 12), "cortex · real run", fill="#9ca3af", font=font, anchor="ma")

    origin_y = title_h + pad_y
    for y in range(ROWS):
        row = screen.buffer[y]
        for x in range(COLS):
            cell = row[x]
            ch = cell.data
            if not ch or ch == " ":
                continue
            fg = color(cell.fg)
            used_font = font_bold if cell.bold else font
            d.text((pad_x + x * cell_w, origin_y + y * cell_h), ch, fill=fg, font=used_font)

    # Cortex's cursor is part of the product feel; pyte tracks where Bubble Tea
    # left it, so render a quiet underline rather than inventing a fake caret.
    if 0 <= screen.cursor.y < ROWS and 0 <= screen.cursor.x < COLS:
        x = pad_x + screen.cursor.x * cell_w
        y = origin_y + screen.cursor.y * cell_h + cell_h - 3
        d.rectangle((x, y, x + cell_w - 1, y + 1), fill="#fdba74")

    return image


def fingerprint(screen: pyte.Screen) -> tuple[Any, ...]:
    # Include attributes as well as glyphs so transitions such as the busy input
    # border are not dropped simply because the characters stayed the same.
    rows = []
    for y in range(ROWS):
        row = screen.buffer[y]
        rows.append(
            tuple((row[x].data, row[x].fg, row[x].bg, row[x].bold, row[x].italics) for x in range(COLS))
        )
    return tuple(rows), screen.cursor.x, screen.cursor.y


def record(binary: str, repo: Path, env: dict[str, str], output: Path) -> None:
    screen = pyte.Screen(COLS, ROWS)
    stream = pyte.ByteStream(screen)
    child = pexpect.spawn(
        binary,
        cwd=str(repo),
        env=env,
        dimensions=(ROWS, COLS),
        encoding=None,
        timeout=0.1,
    )

    frames: list[Image.Image] = []
    durations: list[int] = []
    last_fp: tuple[Any, ...] | None = None

    def snap(ms: int = int(POLL * 1000)) -> None:
        nonlocal last_fp
        fp = fingerprint(screen)
        if fp != last_fp:
            frames.append(render(screen))
            durations.append(ms)
            last_fp = fp
        elif durations:
            durations[-1] += ms

    def pump(seconds: float) -> None:
        end = time.monotonic() + seconds
        while time.monotonic() < end:
            try:
                data = child.read_nonblocking(65536, timeout=POLL)
                if data:
                    stream.feed(data)
            except pexpect.TIMEOUT:
                pass
            except pexpect.EOF:
                break
            snap()

    pump(1.0)
    prompt = "fix the failing greeting test; keep the change minimal and verify it"
    for ch in prompt:
        child.send(ch.encode())
        pump(0.026)
    pump(0.35)
    child.send(b"\r")

    # The scripted model performs four real tool calls and then streams a final
    # answer. Give the real TUI enough room to animate each state and return idle.
    pump(9.5)
    pump(1.4)

    child.close(force=True)
    if not frames:
        raise RuntimeError("no terminal frames captured")

    output.parent.mkdir(parents=True, exist_ok=True)
    frames[0].save(
        output,
        save_all=True,
        append_images=frames[1:],
        duration=durations,
        loop=0,
        optimize=True,
        disposal=1,
    )
    print(f"wrote {output} · {len(frames)} visual frames · {output.stat().st_size / 1024 / 1024:.1f} MiB")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--cortex", required=True)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()

    port = free_port()
    server = Server(("127.0.0.1", port), DemoHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()

    try:
        with tempfile.TemporaryDirectory(prefix="cortex-readme-") as tmp:
            repo, env = setup_demo(Path(tmp), port)
            record(args.cortex, repo, env, args.output)
    finally:
        server.shutdown()
        server.server_close()


if __name__ == "__main__":
    main()
