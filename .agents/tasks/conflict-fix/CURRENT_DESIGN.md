# Current Design: Auto-Compaction vs LLM Compaction Conflict

## Overview

Issue #2331 introduced a `new_session` tool and a compaction method toggle (`auto` vs `llm`). In `llm` mode, the LLM gets `<context_status>` injected and is expected to call `new_session` proactively. However, auto-compaction is fully disabled in `llm` mode, meaning if the LLM fails to act, the context silently overflows.

## Key Files & Functions

### Compaction Flags Logic
- `internal/agent/coordinator.go:408-418` — `compactionFlags()`: Derives `disableAutoSummarize` and `disableContextStatus` from `CompactionMethod`.
  - `CompactionLLM` → `return true, false` (disables auto-summarize, enables context status)
  - `CompactionAuto` (default) → `return disableAutoSummarize, true`

### Auto-Compaction Trigger (StopWhen)
- `internal/agent/agent.go:416-432` — `StopWhen` condition checks remaining tokens against a threshold:
  - Large context (>200K tokens): triggers when ≤20K tokens remain
  - Small context (≤200K tokens): triggers when ≤20% of context remains (i.e., 80% full)
  - **Gated by**: `!a.disableAutoSummarize.Get()` — when `disableAutoSummarize=true`, this condition NEVER fires.

### Constants
- `internal/agent/agent.go:46-53`:
  - `largeContextWindowThreshold = 200_000`
  - `largeContextWindowBuffer = 20_000`
  - `smallContextWindowRatio = 0.2`

### Config
- `internal/config/config.go:54-63` — `CompactionMethod` type (`"auto"` / `"llm"`)
- `internal/config/config.go:265` — `DisableAutoSummarize` field
- `internal/config/config.go:275` — `CompactionMethod` field

### Agent Fields
- `internal/agent/agent.go:110` — `disableAutoSummarize *csync.Value[bool]`
- `internal/agent/agent.go:111` — `disableContextStatus *csync.Value[bool]`
- `internal/agent/agent.go:1058-1060` — `SetCompactionFlags()` updates both at runtime

### Tests
- `internal/agent/compaction_flags_test.go` — Tests for `compactionFlags()`
- `internal/agent/set_compaction_flags_test.go` — Tests for `SetCompactionFlags()`
- `internal/agent/context_status_test.go` — Tests for context status message injection

## The Bug

When `CompactionMethod = "llm"`, `compactionFlags()` returns `disableAutoSummarize=true`. This means the `StopWhen` auto-compaction condition at `agent.go:427` is gated off (`!a.disableAutoSummarize.Get()` == `false`). If the LLM fails to call `new_session`, there is NO safety net — context silently overflows.

The fix: auto-compaction should still fire as a fallback in LLM mode. The `StopWhen` condition should not be gated by `disableAutoSummarize` alone — it needs a separate mechanism or the flag semantics need to change so that LLM mode still allows auto-compaction as a safety net.
