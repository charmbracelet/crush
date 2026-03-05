# Current Design: Context Limit Awareness

## Goal

Inform the LLM of its remaining context budget on every turn so it can decide when to invoke `new_session`. Default behavior: create a new session when context is 75% full.

---

## Relevant Code

### 1. Token Tracking — `internal/agent/agent.go`

Token usage is tracked per-session via `session.PromptTokens` and `session.CompletionTokens`. These are accumulated in the DB (SQL uses `prompt_tokens = prompt_tokens + ?`). The values are updated in `updateSessionUsage()` (line 903-920), called from the `OnStepFinish` callback (line 380-405).

**Context window exhaustion check** (line 406-422):
```go
StopWhen: []fantasy.StopCondition{
    func(_ []fantasy.StepResult) bool {
        cw := int64(largeModel.CatwalkCfg.ContextWindow)
        tokens := currentSession.CompletionTokens + currentSession.PromptTokens
        remaining := cw - tokens
        var threshold int64
        if cw > largeContextWindowThreshold {     // 200,000
            threshold = largeContextWindowBuffer   // 20,000
        } else {
            threshold = int64(float64(cw) * smallContextWindowRatio) // 0.2
        }
        if (remaining <= threshold) && !a.disableAutoSummarize {
            shouldSummarize = true
            return true
        }
        return false
    },
```

This computes `remaining = contextWindow - (promptTokens + completionTokens)`. We need the same calculation for the context info we inject.

### 2. Per-Turn Injection Point — `PrepareStep` callback (line 253-306)

`PrepareStep` fires **before each LLM call** in the agentic loop. It currently:
- Drains queued user messages and appends them (lines 259-267)
- Applies media workarounds (line 269)
- Sets cache control on system + last 2 messages (lines 271-285)
- Prepends `systemPromptPrefix` as a system message (lines 287-289)
- Creates the assistant message record (lines 292-304)

This is where we can inject a context usage system message.

### 3. One-Time Injection — `preparePrompt()` (line 711-747)

The todo reminder is injected as a `<system_reminder>` user message at line 714-720. This runs once at the start of a generation, not per-turn. We could use a similar pattern, but `PrepareStep` is better because it reflects updated token counts after each tool call round-trip.

### 4. Model Context Window

`catwalk.Model.ContextWindow` (int64) holds the model's max context in tokens. Accessed via `largeModel.CatwalkCfg.ContextWindow`. Already in scope inside `PrepareStep` via the `largeModel` closure variable.

### 5. Session Type — `internal/session/session.go:41-53`

```go
type Session struct {
    ID               string
    ParentSessionID  string
    Title            string
    MessageCount     int64
    PromptTokens     int64
    CompletionTokens int64
    SummaryMessageID string
    Cost             float64
    Todos            []Todo
    CreatedAt        int64
    UpdatedAt        int64
}
```

### 6. Fantasy Usage Type

```go
type Usage struct {
    InputTokens         int64
    OutputTokens        int64
    TotalTokens         int64
    ReasoningTokens     int64
    CacheCreationTokens int64
    CacheReadTokens     int64
}
```

### 7. UI Header — Already Shows Percentage

`internal/ui/model/header.go:130`:
```go
percentage := (float64(session.CompletionTokens+session.PromptTokens) / float64(model.ContextWindow)) * 100
```

### 8. new_session Tool — `internal/agent/tools/new_session.go`

The tool returns `NewSessionError{Summary}`. The UI intercepts this in `sendMessage` at `internal/ui/model/ui.go:2744` and triggers `newSessionMsg`.

Tool description is in `internal/agent/tools/new_session.md` — needs a `<when_to_use>` section referencing the context info.

### 9. Tool Registration

- Tools registered in `coordinator.buildTools()` at `internal/agent/coordinator.go:438+`
- Tool allowlist in `allToolNames()` at `internal/config/config.go:703-727`
- `new_session` is already in both places.
