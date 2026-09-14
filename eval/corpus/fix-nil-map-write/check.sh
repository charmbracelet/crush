#!/bin/bash
# Pass when the program runs and prints both counts.
set -e
cd "$EVAL_WORKDIR"
out="$(go run . 2>&1)" || exit 1
echo "$out" | grep -q "apple" && echo "$out" | grep -q "banana"
