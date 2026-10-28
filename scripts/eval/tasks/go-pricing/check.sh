#!/usr/bin/env bash
# Oracle: every test passes and the tests are untouched.
set -u
cd "$1"
git diff --quiet HEAD -- pricing/discount_test.go cart/cart_test.go || { echo "FAIL: tests were changed"; exit 1; }
out=$(go test -count=1 ./... 2>&1) || { echo "FAIL: go test"; echo "$out" | tail -15; exit 1; }
echo "PASS"
