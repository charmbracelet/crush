#!/bin/bash
# Pass when a shared order package exists, all three handlers call it,
# and the build stays green.
set -e
cd "$EVAL_WORKDIR"

go build ./...
go test ./... >/dev/null

# The shared package must exist.
test -d internal/order
grep -rq "package order" internal/order/

# Every handler must import the shared package.
for f in order.go return.go exchange.go; do
	grep -q "shop/internal/order" "internal/handlers/$f"
done

# Turn 2's work must exist: a test exercising the shared validator —
# file-agnostic, but it must reference the validation entry points,
# not just be any new test.
grep -rq "Validate" --include="*_test.go" .

# Turn 3's work must exist: an IsValid helper actually called from main.
grep -rq "IsValid" internal/order/
grep -q "IsValid" cmd/shop/main.go
