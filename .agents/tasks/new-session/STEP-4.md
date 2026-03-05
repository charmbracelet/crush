# Write unit tests and verify end-to-end functionality

Status: COMPLETED

## Sub tasks

1. [x] Create `internal/agent/tools/new_session_test.go` with unit tests.
2. [x] Verify all tests pass.
3. [x] Verify full project compiles with `go build .`.

## NOTES

Created 4 tests in `new_session_test.go`:
- `TestNewSessionTool` — verifies tool info (name, description, params schema).
- `TestNewSessionToolReturnsError` — verifies `Run()` returns `*NewSessionError` with correct summary.
- `TestNewSessionToolEmptySummary` — verifies empty summary still returns the sentinel error.
- `TestNewSessionErrorMessage` — verifies the error message string.

All tests pass. Project compiles cleanly.
