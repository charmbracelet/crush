# Conditionally inject context_status message only if LLM compaction is selected

Status: COMPLETED

## Sub tasks

1. [x] Verify that `compactionFlags` in coordinator.go sets `DisableContextStatus=false` for LLM mode and `true` for Auto mode
2. [x] Verify agent.go line 294 checks `!a.disableContextStatus` before injecting

## NOTES

No code changes needed. The existing agent.go logic at line 294 already checks `!a.disableContextStatus`. The `compactionFlags` function in coordinator.go correctly sets:
- Auto mode: `DisableContextStatus=true` → context_status NOT injected
- LLM mode: `DisableContextStatus=false` → context_status IS injected
