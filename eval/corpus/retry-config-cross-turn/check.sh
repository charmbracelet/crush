#!/bin/bash
# Pass when the retries field exists in config, is wired into the
# client, and a retry-path test exists — everything stays green.
set -e
cd "$EVAL_WORKDIR"

go build ./...
go test ./... >/dev/null

# Config carries an exported retries field parsed from the YAML key,
# and turn 1's default-value test exists — package-scoped, the
# prompts do not pin filenames. Test files are excluded from the
# field/wiring greps so a test's Config literal can't satisfy them.
grep -rq "Retries" --include="*.go" --exclude="*_test.go" internal/config/
grep -rq "Retries" --include="*_test.go" internal/config/

# The client package actually consults it — case-sensitive: the
# pre-existing "retries" prose comment must not satisfy this.
grep -rq "Retries" --include="*.go" --exclude="*_test.go" internal/client/

# Turn 3's work: a retry-path test against a real HTTP server.
grep -rq "httptest" --include="*_test.go" .
