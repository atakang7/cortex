#!/usr/bin/env bash
# Runs one eval task with one cortex binary and prints one line of metrics.
#
#   scripts/eval/run.sh CORTEX_BIN TASK MODEL OUT_DIR [LABEL]
#
# The task's fixture is copied into a fresh git repository, cortex runs its
# prompt once (-prompt) with its own data dir, then the task's check.sh decides
# pass or fail. Metrics come from the session file: requests, tokens, cache
# hits, cost, and the shape of the work (tool calls per reply, tools used).
set -euo pipefail

bin=$1 task=$2 model=$3 out=$4 label=${5:-run}
here=$(cd "$(dirname "$0")" && pwd)
tdir=$here/tasks/$task
[ -d "$tdir/fixture" ] || { echo "no task $task" >&2; exit 2; }

# The first Python that runs: on Windows "python3" may be the Store stub.
PY=$(for c in python3 python; do "$c" -c "" >/dev/null 2>&1 && { echo "$c"; break; }; done)
[ -n "$PY" ] || { echo "no python" >&2; exit 2; }

run=$out/$task-$label-$(date +%H%M%S)
mkdir -p "$run/data"
cp -r "$tdir/fixture" "$run/repo"
(cd "$run/repo" && git init -q && git add -A && git -c user.email=eval@local -c user.name=eval commit -qm fixture)

cat > "$run/config.yaml" <<EOF
provider: openrouter
model: $model
EOF

export AXON_DATA_DIR="$run/data"
start=$(date +%s)
set +e
(cd "$run/repo" && timeout 1800 "$bin" -config "$run/config.yaml" -prompt "$(cat "$tdir/prompt.md")") > "$run/transcript.txt" 2>&1
code=$?
set -e
secs=$(( $(date +%s) - start ))
echo "$secs" > "$run/seconds"

verdict=$(bash "$tdir/check.sh" "$run/repo" "$tdir/fixture" 2>&1 | tail -5 | tr '\n' ' ')

"$PY" - "$run/data" "$task" "$label" "$secs" "$code" "$verdict" <<'PY'
import glob, json, os, sys
data, task, label, secs, code, verdict = sys.argv[1:]
files = glob.glob(os.path.join(data, "sessions", "*.json"))
s = json.load(open(max(files, key=os.path.getsize), encoding="utf-8")) if files else {}
asst = [m for m in s.get("messages", []) if m.get("role") == "assistant"]
calls = [c["function"]["name"] for m in asst for c in (m.get("tool_calls") or [])]
tools = {}
for n in calls:
    tools[n] = tools.get(n, 0) + 1
prompt = s.get("prompt_tokens", 0)
print(json.dumps({
    "task": task, "label": label, "verdict": verdict.strip()[:160], "exit": int(code),
    "seconds": int(secs), "requests": s.get("requests", len(asst)), "replies": len(asst),
    "tool_calls": len(calls), "calls_per_reply": round(len(calls) / max(1, len(asst)), 2),
    "tools": tools, "prompt_tokens": prompt, "completion_tokens": s.get("completion_tokens", 0),
    "cache_hit": round(s.get("cached_tokens", 0) / prompt, 2) if prompt else 0,
    "cost": round(s.get("cost", 0), 4),
}))
PY
