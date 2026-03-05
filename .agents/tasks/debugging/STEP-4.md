# Harden the retry loop or use atomic version selection

Status: COMPLETED

## Sub tasks

1. [x] Make `Create()` delegate to `CreateVersion()` instead of hardcoding `InitialVersion = 0`
2. [x] Verify `TestCreateDuplicateVersion0ExhaustsRetries` passes (was FAIL: UNIQUE constraint → now succeeds)
3. [x] Verify `TestCreateDuplicateVersion0` still passes (retry handles single collision)

## NOTES

Previously `Create()` always called `createWithVersion(ctx, ..., InitialVersion)` which hardcoded version 0. The retry loop in `createWithVersion()` only tried 3 increments (0, 1, 2), so if all three versions already existed for `(path, session_id)`, it exhausted retries and returned a UNIQUE constraint error.

### Fix

`Create()` now delegates to `CreateVersion()`, which first queries the latest version for this path+session and picks `max + 1` (or 0 if none exist). The retry loop in `createWithVersion()` is still there as a safety net for concurrent writes, but it no longer needs to handle the common case of pre-existing versions.

### Changed file

- `internal/history/file.go:Create()` — one-line change, delegates to `CreateVersion()`
