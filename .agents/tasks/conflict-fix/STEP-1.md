# Fix compactionFlags() to keep auto-summarize enabled in LLM mode

Status: COMPLETED

## Sub tasks

1. [x] Change `compactionFlags()` in `coordinator.go` line 414: `return true, false` → `return false, false`
2. [x] Update comment on `compactionFlags()` to reflect new behavior

## NOTES

Changed `coordinator.go:408-418`. The `CompactionLLM` case now returns `false, false` instead of `true, false`, keeping auto-summarize enabled as a safety net while also enabling context status injection for LLM-driven compaction.
