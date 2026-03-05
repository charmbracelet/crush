# Knowledge Base: fix-conflict

- Use the `agent` tool when querying questions about the codebase. Also use the `tldr` command for quickly searching for function references (you can pass the output of this tool to subagents).
  - Wheenver you use the `agent` tool, always warn the agent not to perform expensive operations like calling `grep` for something in the entire home directory, but to call these tools in a way that restricts their scope (so that they don't take forever to run).
- Ensure UI additions follow Bubble Tea patterns used in Crush's existing TUI framework (e.g., matching styles of "Select Reasoning Effort").
- Check if configuring the `Compaction Method` state needs to be serialized via the global configuration, `crush.json`, or if it should be an ephemeral session-bound UI state.
- **Bug discovery**: `UpdateModels` (`coordinator.go:892`) only hot-swaps models and tools. It does NOT re-derive or apply compaction flags (`disableAutoSummarize`, `disableContextStatus`). These are plain `bool` fields on `sessionAgent` set once at construction in `buildAgent` (`coordinator.go:371`). There is no setter on the `SessionAgent` interface for them. This means switching compaction mode via the UI has no effect on the running agent.
- **Thread safety**: The compaction flag fields need to be `*csync.Value[bool]` (not plain `bool`) because they're read during `PrepareStep`/`StopWhen` which run concurrently with any potential setter call from the UI thread (via `UpdateModels`). Follow the same pattern as `largeModel`/`smallModel` which use `*csync.Value[Model]`.
- **Key read sites for compaction flags**:
  - `agent.go:294` — `PrepareStep`: reads `a.disableContextStatus`
  - `agent.go:931` — `contextStatusMessage`: no direct flag check (caller gates it)
  - `agent.go:256` — `StopWhen`: reads `a.disableAutoSummarize`
