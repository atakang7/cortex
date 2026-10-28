#!/usr/bin/env bash
# Re-grades every run under OUT_DIR with the current oracles and prints one
# JSON line per run (the same shape run.sh prints), so a change to an oracle
# never leaves old verdicts behind.
#
#   scripts/eval/grade.sh OUT_DIR
set -uo pipefail
out=$1
here=$(cd "$(dirname "$0")" && pwd)
PY=$(for c in python3 python; do "$c" -c "" >/dev/null 2>&1 && { echo "$c"; break; }; done)
for run in "$out"/*/; do
	run=${run%/}
	[ -d "$run/repo" ] || continue
	name=$(basename "$run")
	task=""
	for t in "$here"/tasks/*/; do
		t=$(basename "$t")
		case "$name" in "$t"-*) [ ${#t} -gt ${#task} ] && task=$t ;; esac
	done
	[ -n "$task" ] || continue
	label=${name#"$task"-}
	label=${label%-*}
	verdict=$(bash "$here/tasks/$task/check.sh" "$run/repo" "$here/tasks/$task/fixture" 2>&1 | tail -1)
	secs=$(cat "$run/seconds" 2>/dev/null || echo 0)
	"$PY" - "$run/data" "$task" "$label" "$verdict" "$secs" <<'PY'
import glob, json, os, sys
data, task, label, verdict, secs = sys.argv[1:]
files = glob.glob(os.path.join(data, "sessions", "*.json"))
s = json.load(open(max(files, key=os.path.getsize), encoding="utf-8")) if files else {}
asst = [m for m in s.get("messages", []) if m.get("role") == "assistant"]
calls = [c["function"]["name"] for m in asst for c in (m.get("tool_calls") or [])]
tools = {}
for n in calls:
    tools[n] = tools.get(n, 0) + 1
prompt = s.get("prompt_tokens", 0)
print(json.dumps({
    "task": task, "label": label, "verdict": verdict.strip(), "seconds": int(secs),
    "requests": s.get("requests", len(asst)), "tool_calls": len(calls), "tools": tools,
    "prompt_tokens": prompt, "completion_tokens": s.get("completion_tokens", 0),
    "cache_hit": round(s.get("cached_tokens", 0) / prompt, 2) if prompt else 0,
    "cost": round(s.get("cost", 0), 4),
}))
PY
done
