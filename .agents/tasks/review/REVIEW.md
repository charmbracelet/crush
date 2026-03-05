# Code Review: LLM-driven compaction with `new_session` tool & context status injection

## 1. 🔴 Context status is injected as a `user` message but comment says "system message"

- [x] Addressed
- [ ] Dismissed

`internal/agent/agent.go:932-950`

The doc comment says "builds a compact context-usage **system** message" but the code creates a **user**-role message via `fantasy.NewUserMessage`:

```go
// contextStatusMessage builds a compact context-usage system message so the
// LLM knows how much of its context window has been consumed.
func (a *sessionAgent) contextStatusMessage(s session.Session, model Model) (fantasy.Message, bool) {
    ...
    msg := fantasy.NewUserMessage(
        fmt.Sprintf(`<context_status>...`),
    )
    return msg, true
}
```

This is appended as the **last message** in the conversation (`agent.go:297`). A synthetic `user` message injected after the real user turn is problematic:

1. **Provider confusion**: Some providers (especially Anthropic) enforce strict user/assistant alternation. Appending a second consecutive user message after the real user message may violate this constraint or cause the provider to merge/reject messages.
2. **LLM misdirection**: The LLM may interpret `<context_status>` as the user's actual query and attempt to respond to it directly rather than treating it as metadata.
3. **Comment is misleading**: Anyone reading the comment will assume it's a system message.

**Suggestion**: Use `fantasy.NewSystemMessage` instead, and place it alongside the existing system prompt prefix (or as a trailing system message if the provider supports it). If system messages can only appear at the start, consider embedding the context status into the existing system prompt prefix.

```go
msg := fantasy.NewSystemMessage(
    fmt.Sprintf(`<context_status>{"used_pct":%d,"remaining_tokens":%d,"context_window":%d}</context_status>`,
        usedPct, remaining, cw),
)
```

---

## 2. 🔴 Empty summary in `new_session` tool is not validated

- [x] Addressed
- [ ] Dismissed

`internal/agent/tools/new_session.go:33-34`

The tool accepts and propagates an empty summary without any validation:

```go
func(ctx context.Context, params NewSessionParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
    return fantasy.ToolResponse{}, &NewSessionError{Summary: params.Summary}
},
```

The test `TestNewSessionToolEmptySummary` confirms this behavior — an empty summary passes through. When this reaches the UI handler (`ui.go:444-447`), it will create a brand new session and send an empty message as the initial prompt:

```go
case newSessionMsg:
    cmds = append(cmds, m.newSession(), func() tea.Msg {
        return sendMessageMsg{Content: msg.Summary}
    })
```

An empty `sendMessageMsg{Content: ""}` will hit `sendMessage`, which calls `AgentCoordinator.Run` with an empty prompt. The agent's `Run` method (`agent.go:154`) returns `ErrEmptyPrompt` for empty prompts:

```go
if call.Prompt == "" && !message.ContainsTextAttachment(call.Attachments) {
    return nil, ErrEmptyPrompt
}
```

This will surface as an error in the UI. The LLM should not be able to trigger this state — either validate at the tool level and return a tool error explaining that a summary is required, or handle it gracefully in the `newSessionMsg` handler.

**Suggestion** — validate in the tool:

```go
func(ctx context.Context, params NewSessionParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
    if strings.TrimSpace(params.Summary) == "" {
        return fantasy.ToolResponse{
            Content: "A non-empty summary is required to create a new session.",
            IsError: true,
        }, nil
    }
    return fantasy.ToolResponse{}, &NewSessionError{Summary: params.Summary}
},
```

Returning a `ToolResponse` with `IsError: true` (instead of a Go error) lets the LLM see the feedback and retry, rather than aborting the agent loop.

---

## 3. 🟡 Context status uses stale token counts

- [x] Addressed
- [ ] Dismissed

`internal/agent/agent.go:295-298` and `agent.go:928-929`

The token counts used by `contextStatusMessage` come from `currentSession.PromptTokens` and `currentSession.CompletionTokens`, which are updated in `updateSessionUsage` **after** each step completes:

```go
session.CompletionTokens = usage.OutputTokens
session.PromptTokens = usage.InputTokens + usage.CacheReadTokens
```

But the context status is injected in `PrepareStep` — **before** the current step runs. This means the status reflects usage as of the *previous* step, not the current state. On the very first turn, both values are 0, so the LLM will always see `used_pct: 0` initially regardless of how large the system prompt is.

This may not be critical since it's one step behind, but it's worth noting because the LLM's compaction decisions will be based on slightly outdated information. The `new_session.md` tool description tells the LLM to trigger at `used_pct >= 75`, so a one-step lag near that threshold could cause it to overshoot.

**Suggestion**: Document this known lag in a code comment, or consider whether the token counts from the provider's previous response can be used to estimate the current step's input tokens more accurately.

---

## 4. 🟡 `used_pct` can exceed 100 and is not clamped

- [x] Addressed
- [ ] Dismissed

`internal/agent/agent.go:945`

```go
usedPct := int64(float64(used) / float64(cw) * 100)
```

The `remaining` value is clamped to 0 (line 942), but `usedPct` is not clamped. The test `TestContextStatusMessage/remaining clamped to zero when overflowed` confirms that `used_pct` can be `125`:

```go
`<context_status>{"used_pct":125,"remaining_tokens":0,"context_window":200000}</context_status>`
```

While values >100% are technically correct as a signal that the context is overflowed, it's inconsistent with `remaining_tokens` being clamped to 0. The LLM prompt says to trigger at `>= 75` so this likely won't cause harm, but it's a minor inconsistency.

**Suggestion**: Either clamp `usedPct` to 100, or don't clamp `remaining` to 0 — be consistent.

---

## 5. 🟡 `new_session` tool is always registered even in `auto` compaction mode

- [x] Addressed
- [ ] Dismissed

`internal/agent/coordinator.go:450`

```go
tools.NewNewSessionTool(),
```

The `new_session` tool is unconditionally added to the tool list in `buildTools`. In `auto` mode, context status injection is disabled (`disableContextStatus: true`), so the LLM never receives the `<context_status>` block that the tool description says will be "injected on every turn." Yet the tool is still visible to the LLM, and the tool description references `<context_status>` as a prerequisite.

This means in `auto` mode:
- The LLM sees a `new_session` tool whose description references signals it will never receive.
- The LLM could still call `new_session` unprompted, which would work but is an unintended path.
- The tool consumes token budget in the system prompt for no benefit.

**Suggestion**: Only register `new_session` when the compaction method is `CompactionLLM`, similar to how LSP tools are conditionally registered:

```go
if !disableContextStatus {
    allTools = append(allTools, tools.NewNewSessionTool())
}
```

This would require passing the compaction mode or a flag into `buildTools`.

---

## 6. 🟡 `newSessionMsg` handler doesn't preserve the old session's context for the user

- [ ] Addressed
- [ ] Dismissed

`internal/ui/model/ui.go:444-447`

```go
case newSessionMsg:
    cmds = append(cmds, m.newSession(), func() tea.Msg {
        return sendMessageMsg{Content: msg.Summary}
    })
```

When `new_session` fires, `m.newSession()` clears all state (session, chat messages, files, LSPs) and resets to the landing screen. Then the summary is sent as the first message of a brand-new session.

From the user's perspective, the entire chat history disappears instantly and a new conversation starts with an opaque LLM-generated summary. There is no indication to the user that:
- A session transition happened
- What the LLM decided to summarize (and potentially lose)
- Which session they were in before (so they can go back to it)

This could be jarring, especially if the user was reading the conversation. Consider at minimum logging an info message like "Session compacted — continuing in new session" before the transition, or showing the summary to the user rather than silently injecting it.

---

## 7. 🟡 Missing newline at end of `new_session.md`

- [x] Addressed
- [ ] Dismissed

`internal/agent/tools/new_session.md:22`

The file ends without a trailing newline. While this won't cause a bug, it's inconsistent with standard POSIX text file conventions and will show a `\ No newline at end of file` marker in diffs.

---

## 8. ⚪️ `CompactionMethod` zero value is empty string, not `"auto"`

- [x] Addressed
- [ ] Dismissed

`internal/config/config.go:53-59`

```go
type CompactionMethod string

const (
    CompactionAuto CompactionMethod = "auto"
    CompactionLLM  CompactionMethod = "llm"
)
```

The Go zero value for `CompactionMethod` is `""`, not `"auto"`. The `compactionFlags` function handles this correctly via the `default` case in its switch:

```go
func compactionFlags(method config.CompactionMethod, disableAutoSummarize bool) (bool, bool) {
    switch method {
    case config.CompactionLLM:
        return true, false
    default:
        return disableAutoSummarize, true
    }
}
```

And the compaction dialog also handles it:

```go
if current == "" {
    current = config.CompactionAuto
}
```

However, this means `""` and `"auto"` are semantically identical but represent different values, which could be confusing if serialized to JSON or compared. Consider setting the default in `setDefaults()` so the zero value is never used in practice, or define `CompactionAuto = ""`.

---

## 9. ⚪️ Compaction dialog is always available, even without a session

- [ ] Addressed
- [ ] Dismissed

`internal/ui/dialog/commands.go:422-425`

```go
commands = append(commands, NewCommandItem(c.com.Styles, "select_compaction", "Select Compaction Method", "", ActionOpenDialog{
    DialogID: CompactionID,
}))
```

The compaction command is appended unconditionally, unlike most other commands which are gated by `c.hasSession`. While changing the compaction method without a session is technically valid (it's a global config change), it's inconsistent with other settings like "Select Reasoning Effort" which is gated behind `c.hasSession` and model capability checks. Minor UX inconsistency.

---

## 10. ⚪️ `ContextWindow` type mismatch: `int` vs `int64`

- [ ] Addressed
- [ ] Dismissed

`internal/agent/agent.go:935,941,945`

```go
cw := model.CatwalkCfg.ContextWindow  // int
used := s.PromptTokens + s.CompletionTokens  // int64
remaining := int64(cw) - used
usedPct := int64(float64(used) / float64(cw) * 100)
```

`ContextWindow` is an `int` from the catwalk config, while token counts are `int64`. The code handles this with explicit `int64(cw)` casts in the `remaining` calculation, but the `usedPct` line divides `float64(used)` by `float64(cw)` — which implicitly promotes `cw` from `int` to `float64`. This works correctly but the mixed types could lead to subtle issues if the types change upstream. Not a bug today, but worth noting.
