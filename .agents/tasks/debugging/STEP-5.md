# Audit write.go and multiedit.go for the same patterns; add maintainer comments

Status: COMPLETED

## Sub tasks

1. [x] Review write.go file history pattern — safe, uses same `Create()`/`CreateVersion()` calls
2. [x] Review multiedit.go new-file path (line 208) — safe, same pattern
3. [x] Review multiedit.go existing-file path (line 354) — safe, same pattern
4. [x] No code changes needed in tool files — the service-layer fix covers all consumers
5. [x] Add explanatory comment to `ListFilesByPathAndSession` in `files.sql`
6. [x] Add explanatory comment to `Create()` in `file.go`
7. [x] Add explanatory comment to `CreateVersion()` in `file.go`
8. [x] Format and verify build + tests pass

## NOTES

No changes were needed in `write.go` or `multiedit.go` because they all call `Create()` and `CreateVersion()` from the history service, which is where the fix lives. The fix is entirely in the service layer:

- `Create()` now delegates to `CreateVersion()` (no more hardcoded version 0)
- `CreateVersion()` now uses session-scoped version queries

Added comments to `files.sql` and `file.go` explaining how the changes relate to the UNIQUE constraint bug that surfaces when the `new_session` tool creates a fresh session that re-edits files from a prior session.
