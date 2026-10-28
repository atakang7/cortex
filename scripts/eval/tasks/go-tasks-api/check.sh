#!/usr/bin/env bash
# Oracle: hidden acceptance tests (never shown to the agent) on a copy of the
# result, plus the repository's own tests, the README and new tests.
set -u
repo=$1 here=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
cp -r "$repo/." "$work/"
cp -r "$here/_hidden/." "$work/"
cd "$work"
out=$(go test -count=1 -v -run 'TestHidden' ./internal/api/ 2>&1)
pass=$(echo "$out" | grep -c '^--- PASS: TestHidden')
total=$(grep -c '^func TestHidden' "$here/_hidden/internal/api/zz_hidden_test.go")
own=PASS; (cd "$repo" && go test -count=1 ./... >/dev/null 2>&1) || own=FAIL
readme=no; grep -q next_cursor README.md && readme=yes
tests=no; [ "$(git -C "$repo" diff --stat HEAD -- '*_test.go' | wc -l)" -gt 0 ] || [ -n "$(git -C "$repo" ls-files -o --exclude-standard -- '*_test.go')" ] && tests=yes
failed=$(echo "$out" | grep '^--- FAIL: TestHidden' | sed 's/--- FAIL: TestHidden//; s/ (.*//' | tr '\n' ',')
verdict=FAIL; [ "$pass" = "$total" ] && [ "$own" = PASS ] && [ "$readme" = yes ] && [ "$tests" = yes ] && verdict=PASS
echo "$verdict hidden=$pass/$total own_tests=$own readme=$readme new_tests=$tests failed=${failed%,}"
rm -rf "$work"
