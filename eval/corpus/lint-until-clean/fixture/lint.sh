#!/usr/bin/env bash
# Project lint rules:
#   1. no fmt.Print* calls outside main.go — use the logger
#   2. no panic( outside main.go — return errors instead
#   3. no TODO/FIXME markers — resolve or remove them
set -u
cd "$(dirname "$0")"

violations=0
scanned=0

# Print a per-file scan manifest first — an operator re-running lint
# wants to see what was checked, not just what failed.
while IFS= read -r f; do
	scanned=$((scanned + 1))
	nl=$(wc -l < "$f" | tr -d ' ')
	nb=$(wc -c < "$f" | tr -d ' ')
	fp=$(grep -c "fmt\.Print" "$f" || true)
	pn=$(grep -c "panic(" "$f" || true)
	td=$(grep -cE "TODO|FIXME" "$f" || true)
	echo "scan: $f (${nl} lines, ${nb} bytes) fmt-print=$fp panic=$pn todo=$td"
done < <(find . -name '*.go' -type f | sort)

while IFS= read -r line; do
	echo "fmt-print: $line"
	violations=$((violations + 1))
done < <(grep -rn "fmt\.Print" --include="*.go" . | grep -v "^\./main\.go" || true)

while IFS= read -r line; do
	echo "panic-call: $line"
	violations=$((violations + 1))
done < <(grep -rn "panic(" --include="*.go" . | grep -v "^\./main\.go" || true)

while IFS= read -r line; do
	echo "todo-marker: $line"
	violations=$((violations + 1))
done < <(grep -rn "TODO\|FIXME" --include="*.go" . || true)

echo "lint: scanned $scanned file(s)"
if [ "$violations" -eq 0 ]; then
	echo "lint: clean"
	exit 0
fi
echo "lint: $violations violation(s)"
exit 1
