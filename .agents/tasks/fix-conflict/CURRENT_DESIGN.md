# Current Design: Fix Conflict Between Context Status and Auto-Compaction

## Goal
The goal of this task is to give the user control over how context is managed when a session becomes too large. Specifically, we want to resolve the conflict between the `new_session` tool/context limit injection and Crush's existing auto-compaction (summarization) by offering a UI toggle to select one or the other.

## Relevant Systems

### 1. Auto-Compaction Algorithm (Backend Summarization)
Crush checks the remaining tokens inside `internal/agent/agent.go:StopWhen`:
- For models > 200,000 max tokens, it triggers summarization when `remaining <= 20,000`.
- For smaller models, it triggers when `remaining <= 20%` of context window (at 80% usage).
When these thresholds are hit, `StopWhen` returns true and `shouldSummarize` is set to true. The backend generates a summary message and chops off the history internally.

### 2. Context Status Message & new_session Tool
The `new_session` tool allows the LLM to preemptively close its own session (suggested at 75% usage) to start a fresh context with a summary it generates itself. To accomplish this, the LLM is fed a `<context_status>` system message so it knows when it's approaching this limit.

### The Conflict & Resolution
If both are active, `new_session` at 75% will always preclude the backend auto-compaction (which waits until 80% or 20k tokens) from running.

**Resolution**: Add a "Select compaction method" item to the main Commands menu in the UI.
- Option 1: "Auto-compaction" (default, or user choice) - Disables the injected `<context_status>` message so the LLM doesn't invoke `new_session`, relying solely on the backend `StopWhen` compaction.
- Option 2: "LLM/User-driven compaction" - Disables the internal `StopWhen`/`shouldSummarize` logic entirely. Enables the `<context_status>` message so the LLM can use the `new_session` tool when limit is approached.
