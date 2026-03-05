# Add test for SetCompactionMethod()

Status: COMPLETED

## Sub tasks

1. [x] Create `internal/config/compaction_method_test.go`
2. [x] Test normal case (persists to disk + in-memory)
3. [x] Test nil Options case

## NOTES

Created `internal/config/compaction_method_test.go` with two tests:
- `TestSetCompactionMethod` — verifies both in-memory and on-disk persistence
- `TestSetCompactionMethod_NilOptions` — verifies nil Options is initialized
