#!/bin/bash
# Pass when the suite is green — the single failing test is the
# referent the vague prompt leaves to discovery.
set -e
cd "$EVAL_WORKDIR"
go test ./...
