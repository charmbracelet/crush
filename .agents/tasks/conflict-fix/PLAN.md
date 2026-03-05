# Plan: Fix Auto-Compaction Safety Net in LLM Compaction Mode

## Problem

In LLM compaction mode, `compactionFlags()` sets `disableAutoSummarize=true`, which fully disables the `StopWhen` auto-compaction check. If the LLM fails to call `new_session`, the context overflows with no fallback.

## Solution

Change `compactionFlags()` so that in `CompactionLLM` mode, `disableAutoSummarize` is set to `false` (not `true`). This allows the `StopWhen` auto-compaction to still fire as a safety net at 80% context usage.

The key insight is that `disableAutoSummarize` controls two things:
1. The `StopWhen` condition (safety net) — should always be enabled
2. Nothing else in LLM mode that would conflict

In LLM mode, the LLM is expected to call `new_session` at ~75% (per the tool description). If it fails, the auto-compaction at 80% acts as a safety net. These two mechanisms are complementary, not conflicting.

### Change Summary

1. **`internal/agent/coordinator.go:413-414`**: Change `compactionFlags()` for `CompactionLLM` case from `return true, false` to `return false, false` — keep auto-summarize enabled as safety net while also enabling context status.
2. **`internal/agent/compaction_flags_test.go`**: Update the test expectations for LLM mode to expect `disableAutoSummarize=false`.

## Risks

- Minimal. The auto-compaction at 80% only fires if the LLM hasn't already called `new_session` (which fires at 75%). If `new_session` succeeds, the context resets and auto-compaction never triggers.
- The existing `Summarize()` path and the `new_session` tool path are independent — both create new sessions but via different mechanisms. Having both enabled doesn't cause conflicts.
