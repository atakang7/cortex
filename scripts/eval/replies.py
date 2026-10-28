"""Per-reply breakdown of runs: reasoning, answer and tool-argument sizes, and
the calls made. Shows where a model's output tokens (and so its time) go.

    python3 scripts/eval/replies.py OUT_DIR/RUN [...]
"""

import glob
import json
import os
import sys

for run in sys.argv[1:]:
    for f in glob.glob(os.path.join(run, "data", "sessions", "*.json")):
        s = json.load(open(f, encoding="utf-8"))
        print("==", os.path.basename(run.rstrip("/")))
        tot = {"reason": 0, "content": 0, "args": 0}
        for m in s.get("messages", []):
            if m.get("role") != "assistant":
                continue
            calls = m.get("tool_calls") or []
            r = len(m.get("reasoning") or "")
            c = len(m.get("content") or "")
            a = sum(len(x["function"]["arguments"]) for x in calls)
            tot["reason"] += r
            tot["content"] += c
            tot["args"] += a
            print(f"  reason={r:6} content={c:5} args={a:6} {[x['function']['name'] for x in calls]}")
        print("  total chars:", tot)
