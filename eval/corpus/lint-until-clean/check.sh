#!/bin/bash
# Pass when lint.sh reports clean and the build/tests stay green.
set -e
cd "$EVAL_WORKDIR"

go build ./...
go test ./... >/dev/null

# Turn 2's lint rule must exist — env access belongs to config.
# Match "Getenv" loosely: the fixture style escapes dots (os\.Getenv).
grep -q "Getenv" lint.sh

# The rule must have teeth: no os.Getenv in Go sources outside config.
if grep -rn "os.Getenv" --include="*.go" . | grep -v "internal/config"; then
	echo "os.Getenv still used outside internal/config" >&2
	exit 1
fi

# Turn 3's work must exist: a test under internal/fileutil.
grep -rq "func Test" internal/fileutil/

bash ./lint.sh

# Probe: a seeded os.Getenv violation must make lint.sh fail — the
# rule must actually fire, not just exist as text.
probe=internal/fileutil/lint_probe.go
cat > "$probe" <<'EOF'
package fileutil

import "os"

var probeEnv = os.Getenv("TOOLKIT_LINT_PROBE")
EOF
trap 'rm -f "$probe"' EXIT
if bash ./lint.sh >/dev/null 2>&1; then
	echo "lint.sh did not fire on a seeded os.Getenv violation" >&2
	exit 1
fi
rm -f "$probe"
trap - EXIT
