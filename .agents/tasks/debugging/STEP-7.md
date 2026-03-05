# Fix data race in `db.Connect()` and verify with `go test -race ./...`

Status: COMPLETED

## Sub tasks

1. [x] Add `sync.Once` to `internal/db/connect.go` guarding the goose global state calls
2. [x] Run `go test -race ./internal/history/...` to confirm race is gone
3. [x] Run `go test -race ./...` for full suite
4. [x] Format with `gofumpt`

## NOTES

### Root Cause

`goose.SetBaseFS(FS)` and `goose.SetDialect("sqlite3")` in `db.Connect()` (line 40-45) write to unsynchronized package-level globals in the `goose` package:
- `var baseFS fs.FS` in `goose.go:23`
- `var store legacystore.Store` in `dialect.go:37`

All four tests in `internal/history/file_test.go` use `t.Parallel()` and each calls `setupTest()` → `db.Connect()`, causing concurrent writes to these globals. The race detector catches a write from one goroutine (`SetBaseFS`) racing with a read from another (`CollectMigrations`).

### Fix Applied

Added `var gooseOnce sync.Once` at package level in `internal/db/connect.go` and wrapped the two goose global-state calls inside `gooseOnce.Do(func() { ... })`. The `dialectErr` is captured and checked after the `Do` block.

This is safe because:
1. The embedded FS is a package-level constant — never changes.
2. The dialect is always "sqlite3" for this project.
3. Both values are set-and-forget — they don't need to change per-connection.

### Verification

- `go test -race -count=1 ./internal/history/...` — PASS
- `go test -race -count=1 ./internal/filetracker/... ./internal/agent/...` — PASS
- `go test -race -failfast ./...` — all packages PASS, zero race warnings
