#!/bin/bash
# Pass when the cache package is gone, no file imports it, and the
# build/tests stay green.
set -e
cd "$EVAL_WORKDIR"

go build ./...
go test ./... >/dev/null

if [ -d internal/cache ]; then
	echo "internal/cache still exists" >&2
	exit 1
fi

if grep -rn "internal/cache" --include="*.go" . ; then
	echo "internal/cache still imported" >&2
	exit 1
fi

# Services must now use internal/store.
grep -q "catalog/internal/store" internal/service/catalog.go
grep -q "catalog/internal/store" internal/service/search.go

# Turns 2 and 3 must both exist: the fixture ships 2 test entry
# points, so the tree needs ≥4 — a merged table-driven test counts
# via its t.Run subtests. Location-agnostic: a service-level test
# asserting Stats is valid.
test "$(grep -rE "func Test|t\.Run" --include="*_test.go" . | wc -l)" -ge 4
grep -rq "Stats" --include="*_test.go" .
