#!/bin/bash
# Pass when the rename landed everywhere: builds, tests green, old
# identifiers gone, new identifiers present.
set -e
cd "$EVAL_WORKDIR"

go build ./...
go test ./... >/dev/null

# Old names must be gone from Go sources and the YAML file.
if grep -rn "TimeoutMS\|timeout_ms" --include="*.go" --include="*.yaml" . ; then
	echo "old timeout_ms identifiers still present" >&2
	exit 1
fi

# New names must be present in code and config.
grep -rn "TimeoutSeconds" --include="*.go" . >/dev/null
grep -q "timeout_seconds" config.yaml

# Turn 2's work must exist: a Validate method on the config type —
# file-agnostic, a validate.go sibling is valid, but TestValidate in
# a _test.go does not count — and the prompt wires it into main.
grep -rq "func.*Validate" --include="*.go" --exclude="*_test.go" internal/config/
grep -q "Validate" main.go

# Turn 3's work must exist: Validate coverage in a test file.
grep -rq "Validate" --include="*_test.go" internal/config/
