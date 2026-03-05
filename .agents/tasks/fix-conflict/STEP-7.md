# Verify LLM/User-driven compaction mode

Status: COMPLETED

## Sub tasks

1. [x] Verify `compactionFlags(CompactionLLM, false)` returns `DisableAutoSummarize=true, DisableContextStatus=false`
2. [x] Verify existing `context_status_test.go` tests still pass
3. [x] Full `go test ./internal/...` passes

## NOTES

Verified via `TestCompactionFlags` and full test suite. LLM mode correctly enables context_status injection and disables the StopWhen auto-summarize trigger.
