#!/usr/bin/env bash
# Oracle: json and text both behave as asked, the suite passes and grew.
set -u
cd "$1"
PY=$(for c in python3 python; do "$c" -c "" >/dev/null 2>&1 && { echo "$c"; break; }; done)
# $2 is the untouched fixture: the default output must match it exactly.
text_old=$(cd "$2" && "$PY" -m reportgen.cli sales.csv)
text_new=$("$PY" -m reportgen.cli sales.csv) || { echo "FAIL: text run"; exit 1; }
[ "$text_old" = "$text_new" ] || { echo "FAIL: default text output changed"; exit 1; }
[ "$("$PY" -m reportgen.cli sales.csv --format text)" = "$text_old" ] || { echo "FAIL: --format text differs"; exit 1; }
json=$("$PY" -m reportgen.cli sales.csv --format json) || { echo "FAIL: json run"; exit 1; }
"$PY" - "$json" <<'PY' || { echo "FAIL: json content"; exit 1; }
import json, sys
from reportgen.report import build_report, load_rows
got = sys.argv[1]
want = json.dumps(build_report(load_rows("sales.csv")), sort_keys=True, indent=2)
assert got.strip() == want, (got, want)
PY
out=$("$PY" -m unittest discover -s tests 2>&1) || { echo "FAIL: unittest"; echo "$out" | tail -10; exit 1; }
n=$(echo "$out" | sed -n 's/^Ran \([0-9]*\) test.*/\1/p')
[ "${n:-0}" -gt 3 ] || { echo "FAIL: no new tests ($n)"; exit 1; }
grep -rqi json tests || { echo "FAIL: tests do not cover json"; exit 1; }
echo "PASS"
