# Update compaction_flags_test.go to match new behavior

Status: COMPLETED

## Sub tasks

1. [x] Update "llm mode enables context status and disables auto summarize" test — auto-summarize should now be enabled (false)
2. [x] Update "llm mode ignores legacy disable auto summarize override" test — auto-summarize should now be enabled (false)
3. [x] Update test names to reflect the new behavior
4. [x] Fix round-trip tests in `set_compaction_flags_test.go` that also expected `disableAutoSummarize=true` in LLM mode

## NOTES

Updated 3 tests across 2 files:
- `compaction_flags_test.go`: 2 tests updated (expectations + names)
- `set_compaction_flags_test.go`: 2 assertions updated in `TestCompactionModeSwitchRoundTrip` (auto→llm and llm→auto sub-tests)
