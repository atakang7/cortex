"""Behaviour metrics per run, from the session log (BEHAVIORS.md).

    python3 scripts/eval/behaviors.py OUT_DIR

One line per run: replies, calls per reply, tool errors, repeated reads (the
same file read again with no write to it in between), whether a plan was
written, and whether the last change was followed by a passing check. Works
for both tool vocabularies (read/write/exec and read_file/edit/bash).
"""

import glob
import json
import os
import re
import sys

READ = {"read", "read_file"}
WRITE = {"write", "write_file", "edit"}
EXEC = {"exec", "bash"}
PLAN = {"task", "todo_write"}
TRIVIAL = set("ls cat head tail grep rg find echo printf pwd wc which type tree stat file du cd sed awk sort diff true sleep date env export git".split())


def is_check(cmd):
    for seg in re.split(r"[;&|\n]", cmd):
        f = seg.split()
        if f and f[0] not in TRIVIAL:
            return True
    return False


def exit_code(text):
    m = re.search(r"(?m)^exit_code: (-?\d+)", text or "")
    return int(m.group(1)) if m else -1


for run in sorted(glob.glob(os.path.join(sys.argv[1], "*-*"))):
    files = glob.glob(os.path.join(run, "data", "sessions", "*.json"))
    if not os.path.isdir(run) or not files:
        continue
    s = json.load(open(max(files, key=os.path.getsize), encoding="utf-8"))
    msgs = s.get("messages", [])
    calls = {}
    replies = ncalls = errors = rereads = plans = 0
    seen = {}  # path -> read since last write
    last_write = None
    verified = "none"
    for i, m in enumerate(msgs):
        if m.get("role") == "assistant":
            replies += 1
            for c in m.get("tool_calls") or []:
                ncalls += 1
                try:
                    args = json.loads(c["function"]["arguments"] or "{}")
                except ValueError:
                    args = {}
                calls[c.get("id")] = (c["function"]["name"], args)
        elif m.get("role") == "tool":
            name, args = calls.get(m.get("tool_call_id"), (m.get("tool_name", ""), {}))
            if m.get("tool_failed"):
                errors += 1
                continue
            path = args.get("path", "") if isinstance(args, dict) else ""
            if name in READ and path:
                if seen.get(path):
                    rereads += 1
                seen[path] = True
            elif name in WRITE:
                for p in [path] + [e.get("path", "") for e in args.get("edits", []) if isinstance(e, dict)]:
                    seen[p] = False
                last_write, verified = i, "unverified"
            elif name in PLAN:
                plans += 1
            elif name in EXEC and last_write is not None and is_check(args.get("command", "")):
                verified = "passed" if exit_code(m.get("content", "")) == 0 else "failed"
    label = os.path.basename(run.rstrip("/"))
    print(f"{label:40} replies={replies:3} calls/reply={ncalls / max(1, replies):.2f} errors={errors:2} rereads={rereads:2} plan={plans:2} last_change={verified}")
