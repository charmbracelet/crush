# Fix createNewFile() in edit.go to check history before inserting version 0

Status: COMPLETED

## Sub tasks

1. [x] Review `createNewFile()` in `edit.go` (line 110-188)
2. [x] Determine that the `os.Stat` guard (line 111-116) prevents most duplicate calls
3. [x] Fix the misleading comment at line 163 ("File can't be in the history")
4. [x] Confirm that `Create()` now safely handles duplicates via the service-layer fix

## NOTES

The `createNewFile()` path in `edit.go` checks `os.Stat(filePath)` and returns "file already exists" if the file is present on disk, so it only runs for truly new files. However, the comment "File can't be in the history" was incorrect — a file could exist in history if created, deleted, and recreated in the same session. Updated the comment to "Store the original (empty) content in file history."

No further code changes needed here because `Create()` now delegates to `CreateVersion()`, which always picks the correct version number regardless of prior history state.

Changed file: `internal/agent/tools/edit.go:163`
