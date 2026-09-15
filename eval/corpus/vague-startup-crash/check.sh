#!/bin/bash
# Pass when the program runs to completion and prints the marker.
set -e
cd "$EVAL_WORKDIR"
out="$(go run . 2>&1)" || exit 1
echo "$out" | grep -q "startup ok"
