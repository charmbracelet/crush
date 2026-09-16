#!/bin/bash
# Pass when both options exist on Options, are reachable through the
# `option` builtin, and appear in the regenerated schema.json. On the
# start state neither key exists anywhere — the first grep fails.
set -e
cd "$EVAL_WORKDIR"

go build ./...

# The Options fields with their JSON tags.
grep -rq 'compact_status' internal/config/
grep -rq 'quiet_startup' internal/config/

# The `option` builtin keys.
grep -q 'compact-status' internal/shellconfig/options.go
grep -q 'quiet-startup' internal/shellconfig/options.go

# The published schema.
grep -q '"compact_status"' schema.json
grep -q '"quiet_startup"' schema.json
