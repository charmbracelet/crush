# Confirm the bug with a unit test

Status: COMPLETED

## Sub tasks

1. [x] Write test `TestCreate` — basic sanity check (PASS)
2. [x] Write test `TestCreateVersion` — basic version increment (PASS)
3. [x] Write test `TestCreateDuplicateVersion0` — retry handles single collision (PASS — retry bumps to version 1)
4. [x] Write test `TestCreateDuplicateVersion0ExhaustsRetries` — exhausts 3 retries (FAIL — confirms bug)
5. [x] Write test `TestCreateVersionCrossSessionCollision` — cross-session version leak (FAIL — confirms bug)

## NOTES

Both bugs are confirmed:

1. `TestCreateDuplicateVersion0ExhaustsRetries`: When versions 0, 1, 2 already exist for `(path, session_id)`, calling `Create()` (which starts at version 0 and retries 3 times) exhausts all retries and returns `UNIQUE constraint failed`.

2. `TestCreateVersionCrossSessionCollision`: `CreateVersion()` returns version 3 instead of 1 for session B, because `ListFilesByPath()` queries all sessions and finds version 2 from session A, then inserts version 3.

Test file: `internal/history/file_test.go`
