#!/bin/bash
# Pass when all tests pass and the test file was not rewritten to
# hide the failing case.
set -e
cd "$EVAL_WORKDIR"

go build ./...
go test ./... >/dev/null

grep -q "func TestReserveBeyondAvailable" internal/inventory/inventory_test.go
grep -q "ErrInsufficient" internal/inventory/inventory_test.go

# Turn 2's work must exist: a Release-related test — file-agnostic,
# and a test exercising Release through internal/order counts too.
grep -rq "Release" --include="*_test.go" .

# Turn 3's work must exist: InUse on Inventory (file-agnostic),
# surfaced in report.Stock — the prompt names that function.
grep -rq "InUse" internal/inventory/
grep -q "InUse" internal/report/report.go
