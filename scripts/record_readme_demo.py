#!/usr/bin/env python3
"""Record the real Cortex TUI against a deterministic local OpenAI-compatible model.

The model is scripted, but Cortex is not mocked: the binary runs through its real
Bubble Tea UI and Axon executes real search/read/write/exec tools against a
disposable Go repository. The PTY is captured and rendered into the README GIF.

The renderer deliberately looks like a normal terminal, not a product mockup.
That is Cortex's UI: shell scrollback above a tiny live area, with no fake window
chrome or alternate-screen frame invented for the README.
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

COLS = 118
ROWS = 30
POLL = 0.075

# Close to the dark GNOME-terminal palette in the real Cortex screenshot.
BG = "#1d1b1b"
DEFAULT_FG = "#d8dce8"

ANSI = {
    "black": "#1d1b1b",
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

        # Slow the scripted provider just enough for Cortex's actual spinner,
        # tool-card and stream states to be visible in the recording.
        time.sleep(0.34)
        for item in SCRIPT[index]:
            payload = f"data: {json.dumps(item)}\n\n".encode()
            self.wfile.write(payload)
            self.wfile.flush()
            time.sleep(0.15 if "tool_calls" in item["choices"][0]["delta"] else 0.11)
        self.wfile.write(b"data: [DONE]\n\n")
        self.wfile.flush()
        self.close_connection = True


class Server(http.server.ThreadingHTTPServer):
    daemon_threads = True


def setup_demo(root: Path, port: int, binary: str) -> tuple[Path, dict[str, str]]:
    home = root / "home"
    repo = root / "cortex-demo"
    bin_dir = root / "bin"
    (home / ".config" / "axon").mkdir(parents=True)
    (home / ".config" / "cortex").mkdir(parents=True)
    repo.mkdir(parents=True)
    bin_dir.mkdir(parents=True)

    # Keep the labels users see in the recording realistic while routing both
    # model names to the deterministic local server.
    (home / ".config" / "axon" / "axon.yaml").write_text(
        f"""providers:\n  demo:\n    base_url: http://127.0.0.1:{port}\n    models:\n      z-ai/glm-5.2:\n      deepseek/deepseek-v4-flash-0731:\nmodel:\n  exclude_reasoning: true\n  request_timeout: 30s\n  idle_timeout: 5s\n"""
    )
    (home / ".config" / "cortex" / "config.yaml").write_text(
        "provider: demo\nmodel: z-ai/glm-5.2\npruner:\n  provider: demo\n  model: deepseek/deepseek-v4-flash-0731\n"
    )

    (repo / "go.mod").write_text("module example.com/cortex-demo\n\ngo 1.26.2\n")
    (repo / "greeter.go").write_text(
        'package demo\n\nfunc Greeting() string {\n\treturn "hello"\n}\n'
    )
    (repo / "greeter_test.go").write_text(
        'package demo\n\nimport "testing"\n\nfunc TestGreeting(t *testing.T) {\n\tif got := Greeting(); got != "hello, cortex" {\n\t\tt.Fatalf("Greeting() = %q, want %q", got, "hello, cortex")\n\t}\n}\n'
    )

    # Launch Cortex through a real interactive shell so the GIF begins the way
    # the product actually begins: shell prompt → `cortex` → trace → TUI.
    target = bin_dir / "cortex"
    try:
        target.symlink_to(Path(binary).resolve())
    except FileExistsError:
        pass

    env = os.environ.copy()
    env.update(
        {
            "HOME": str(home),
            "XDG_CONFIG_HOME": str(home / ".config"),
            "XDG_CACHE_HOME": str(home / ".cache"),
            "XDG_DATA_HOME": str(home / ".local" / "share"),
            "PATH": str(bin_dir) + os.pathsep + env.get("PATH", ""),
            "TERM": "xterm-256color",
            "COLORTERM": "truecolor",
            "NO_COLOR": "",
            # A small two-line shell prompt in the spirit of the real capture.
            "PS1": "\001\033[1;36m\002cortex-demo\001\033[0m\002 on \001\033[1;35m\002main\001\033[0m\002 via \001\033[1;36m\002go1.26.2\001\033[0m\002\n\001\033[1;32m\002❯\001\033[0m\002 ",
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
    # Match an ordinary terminal viewport. No fake OS chrome, title bar,
    # rounded product frame or decorative labels.
    font, font_bold = fonts(15)
    bbox = font.getbbox("M")
    cell_w = bbox[2] - bbox[0]
    cell_h = 18
    pad_x, pad_y = 8, 8
    width = COLS * cell_w + pad_x * 2
    height = ROWS * cell_h + pad_y * 2

    image = Image.new("RGB", (width, height), BG)
    d = ImageDraw.Draw(image)

    for y in range(ROWS):
        row = screen.buffer[y]
        for x in range(COLS):
            cell = row[x]
            ch = cell.data
            if not ch or ch == " ":
                continue
            fg = color(cell.fg)
            used_font = font_bold if cell.bold else font
            d.text((pad_x + x * cell_w, pad_y + y * cell_h), ch, fill=fg, font=used_font)

    # The cursor is part of Cortex's actual composer. Render a thin underline
    # where the PTY says it is rather than inventing a marketing caret.
    if 0 <= screen.cursor.y < ROWS and 0 <= screen.cursor.x < COLS:
        x = pad_x + screen.cursor.x * cell_w
        y = pad_y + screen.cursor.y * cell_h + cell_h - 2
        d.rectangle((x, y, x + cell_w - 1, y + 1), fill="#fdba74")

    return image


def fingerprint(screen: pyte.Screen) -> tuple[Any, ...]:
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
        "/bin/bash",
        ["--noprofile", "--norc", "-i"],
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

    # Show the real shell launch instead of starting the binary off-screen.
    pump(0.8)
    for ch in "cortex":
        child.send(ch.encode())
        pump(0.07)
    pump(0.25)
    child.send(b"\r")
    pump(1.35)

    prompt = "fix the failing greeting test; keep the change minimal and verify it"
    for ch in prompt:
        child.send(ch.encode())
        pump(0.025)
    pump(0.35)
    child.send(b"\r")

    # Four real tool calls, then the streamed answer, then a brief idle hold so
    # the final Cortex status line is readable before the GIF loops.
    pump(9.0)
    pump(1.6)

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
            repo, env = setup_demo(Path(tmp), port, args.cortex)
            record(args.cortex, repo, env, args.output)
    finally:
        server.shutdown()
        server.server_close()


if __name__ == "__main__":
    main()
