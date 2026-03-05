# Only trigger auto-summarization if Auto-compaction is selected

Status: COMPLETED

## Sub tasks

1. [x] Verify that `compactionFlags` sets `DisableAutoSummarize=true` for LLM mode and `false` for Auto mode
2. [x] Verify agent.go line 426 checks `!a.disableAutoSummarize` before setting shouldSummarize

## NOTES

No code changes needed. The existing agent.go StopWhen logic at line 426 already checks `!a.disableAutoSummarize`. The `compactionFlags` function correctly sets:
- Auto mode: `DisableAutoSummarize=false` (unless config override) → auto-summarize fires
- LLM mode: `DisableAutoSummarize=true` → auto-summarize disabled
