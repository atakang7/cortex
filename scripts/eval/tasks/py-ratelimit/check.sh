#!/usr/bin/env bash
# Oracle: hidden acceptance tests (never shown to the agent) on a copy of the
# result, plus the repository's own tests, the README and new tests.
set -u
repo=$1 here=$(cd "$(dirname "$0")" && pwd)
PY=$(for c in python3 python; do "$c" -c "" >/dev/null 2>&1 && { echo "$c"; break; }; done)
work=$(mktemp -d)
cp -r "$repo/." "$work/"
cp -r "$here/_hidden/." "$work/"
cd "$work"
out=$("$PY" -m unittest -v tests.test_zz_hidden 2>&1)
pass=$(echo "$out" | grep -c '^test_hidden.* \.\.\. ok$')
total=$(grep -c 'def test_hidden' "$here/_hidden/tests/test_zz_hidden.py")
rm -f tests/test_zz_hidden.py
own=PASS; "$PY" -m unittest discover -s tests >/dev/null 2>&1 || own=FAIL
readme=no; grep -q KeyedLimiter README.md && readme=yes
tests=no; [ -n "$(git -C "$repo" diff --stat HEAD -- tests)" ] || [ -n "$(git -C "$repo" ls-files -o --exclude-standard -- tests)" ] && tests=yes
failed=$(echo "$out" | grep -E '^test_hidden.* \.\.\. (FAIL|ERROR)' | sed 's/ (.*//; s/test_hidden_//' | tr '\n' ',')
verdict=FAIL; [ "$pass" = "$total" ] && [ "$own" = PASS ] && [ "$readme" = yes ] && [ "$tests" = yes ] && verdict=PASS
echo "$verdict hidden=$pass/$total own_tests=$own readme=$readme new_tests=$tests failed=${failed%,}"
rm -rf "$work"
