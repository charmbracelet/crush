#!/bin/bash
# Pass when `crush env` exists, prints all four key=value rows, and
# the tree still builds. On the start state `env` is an unknown
# command — go run exits non-zero and the check fails.
set -e
cd "$EVAL_WORKDIR"

go build ./...

out="$(go run . env 2>&1)"
echo "$out" | grep -q '^goos='
echo "$out" | grep -q '^goarch='
echo "$out" | grep -q '^version='
echo "$out" | grep -q '^cwd='
