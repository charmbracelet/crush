# Current Design: Context Status Injection

## How `<context_status>` Works Today

The `<context_status>` block is generated and injected in `internal/agent/agent.go`.

### Generation (`agent.go:932-953`)

`contextStatusMessage()` builds a compact system message containing `used_pct`, `remaining_tokens`, and `context_window`. Token counts come from the previous step's API response (`session.PromptTokens` and `session.CompletionTokens`), which are **replaced** (not accumulated) after each step.

### Injection (`agent.go:295-299`)

Inside the `PrepareStep` callback, the status message is **appended to the end of the in-flight message slice** passed to the LLM — it is **not** persisted to conversation history or the session database.

```go
if !a.isSubAgent && !a.disableContextStatus.Get() {
    if statusMsg, ok := a.contextStatusMessage(currentSession, largeModel); ok {
        prepared.Messages = append(prepared.Messages, statusMsg)
    }
}
```

### Key Properties

- **Ephemeral**: each turn gets a freshly computed message; previous turns' context status messages do not accumulate in history.
- **Not for sub-agents**: gated by `!a.isSubAgent`.
- **Only with LLM compaction**: enabled when `compaction_method: "llm"`, disabled for `"auto"` (default). Controlled by `compactionFlags()` in `coordinator.go:408-418`.

## Conclusion

The current implementation is already efficient. The `<context_status>` is a transient per-turn injection (similar to how tool definitions are injected), not a persisted history message. There is no accumulation of multiple context status entries in the conversation context.
