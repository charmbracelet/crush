# Investigate how `<context_status>` is injected

Status: COMPLETED

## Sub tasks

1. [x] Find all references to `context_status` in the codebase
2. [x] Determine if it's persisted to history or ephemeral
3. [x] Document findings in CURRENT_DESIGN.md

## NOTES

The `<context_status>` is already implemented efficiently:

- **Generated** in `internal/agent/agent.go:932-953` by `contextStatusMessage()`
- **Injected** in `internal/agent/agent.go:295-299` inside `PrepareStep` — appended to the in-flight message slice, **not** persisted to conversation history
- Each turn gets a fresh message; previous turns' status messages do not accumulate
- Gated by `!a.isSubAgent` and `!a.disableContextStatus.Get()`
- Only active with `compaction_method: "llm"` (controlled by `compactionFlags()` in `coordinator.go:408-418`)

**Conclusion**: No changes needed. The implementation already matches the desired behavior.
