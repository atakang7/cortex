"""Prints graded runs (grade.sh output on stdin) as a table, with seconds
taken from run.sh's own JSON lines when the run predates the seconds file."""

import glob
import json
import os
import sys

out_dir = sys.argv[1]
secs = {}
for f in glob.glob(os.path.join(out_dir, "*.json")):
    try:
        d = json.load(open(f, encoding="utf-8"))
        secs[(d["task"], d["label"])] = d["seconds"]
    except (ValueError, KeyError):
        pass

for line in sys.stdin:
    d = json.loads(line)
    s = d["seconds"] or secs.get((d["task"], d["label"]), 0)
    print(f'{d["task"]:14} {d["label"]:14} {s:5}s req={d["requests"]:3} calls={d["tool_calls"]:3} '
          f'out={d["completion_tokens"]:6} cost=${d["cost"]:.4f} cache={d["cache_hit"]:.2f} '
          f'goal={d["tools"].get("goal", 0)} | {d["verdict"]}')
