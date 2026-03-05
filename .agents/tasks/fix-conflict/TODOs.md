1. [x] Define a configuration/state setting to track the current compaction method (e.g., `Auto` vs `LLM-driven`).
2. [x] Add "Select compaction method" menu item to the main Commands menu in the TUI.
3. [x] Implement the UI overlay/list for "Select compaction method" (similar to reasoning effort selection), presenting "Auto-compaction" and "LLM/User-driven compaction", and wire it up to the state.
4. [x] In `internal/agent/agent.go`, conditionally inject the `<context_status>` message only if "LLM/User-driven compaction" is selected.
5. [x] In `internal/agent/agent.go` inside `StopWhen`, only trigger auto-summarization (`shouldSummarize = true`) if "Auto-compaction" is selected.
6. [x] Verify that selecting "Auto-compaction" successfully hides token status from the LLM, leaving auto-compaction functional.
7. [x] Verify that selecting "LLM/User-driven compaction" correctly injects the context status message and disables the backend `StopWhen` trigger.
8. [x] Bug fix: Convert `disableAutoSummarize` and `disableContextStatus` fields on `sessionAgent` from plain `bool` to `*csync.Value[bool]` for thread-safe reads/writes.
9. [x] Bug fix: Add `SetCompactionFlags(disableAutoSummarize, disableContextStatus bool)` to the `SessionAgent` interface and implement it on `sessionAgent`.
10. [x] Bug fix: Update all read sites in `PrepareStep`, `StopWhen`, and `contextStatusMessage` to use `.Get()` on the new csync values.
11. [x] Bug fix: In `coordinator.UpdateModels`, re-derive compaction flags via `compactionFlags()` and apply them to the live agent via `SetCompactionFlags`.
12. [x] Bug fix: Add/update tests to verify that switching compaction mode mid-session correctly updates the agent's behavior (context_status injection toggles, auto-summarize toggles).
