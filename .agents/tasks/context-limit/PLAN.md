# Plan: Context Limit Awareness

## Overview

Inject a compact context-usage status into every LLM turn so the agent knows when to invoke `new_session`. The info must include remaining tokens (absolute) and remaining context (percentage). Default threshold for auto-invoking `new_session`: 75% full.

## Steps

### 1. Update `new_session.md` tool description

Add a `<when_to_use>` section that tells the LLM:
- A `<context_status>` block is injected on every turn with `used_percent` and `remaining_tokens`.
- By default, invoke `new_session` when `used_percent >= 75` (i.e. context is 75% full).
- The user may override this threshold with explicit instructions (e.g. "start a new session when there's only 5000 tokens remaining").
- When approaching the threshold, the LLM should proactively wrap up and call `new_session` with a comprehensive summary.

### 2. Inject context status in `PrepareStep`

In `internal/agent/agent.go`, inside the `PrepareStep` callback (line 253-306), after the existing logic but before returning, prepend a system message with context usage info. The message should be very compact, e.g.:

```
<context_status>
{"used_pct":42,"remaining_tokens":116000,"context_window":200000}
</context_status>
```

This uses JSON for easy parsing and minimal tokens. The data comes from:
- `currentSession.PromptTokens + currentSession.CompletionTokens` = used tokens
- `largeModel.CatwalkCfg.ContextWindow` = total context window
- `remaining = contextWindow - used`
- `used_pct = (used / contextWindow) * 100` (integer)

Important: this should only be injected for non-subagent sessions (check `a.isSubAgent`), and only when the context window is known (> 0).

### 3. Write tests

Add a test in `internal/agent/` that verifies:
- The `<context_status>` block is present in `PrepareStep` output messages.
- The values are computed correctly from session tokens and model context window.
- The block is not injected for sub-agents.
- The block is not injected when context window is 0 (unknown).

### 4. Verify end-to-end

- Run the full test suite.
- Build and manually verify the context status appears in the LLM's message stream (via debug logging or `crush logs`).
