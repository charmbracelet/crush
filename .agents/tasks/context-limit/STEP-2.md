# Inject <context_status> system message in PrepareStep callback

Status: COMPLETED

## Sub tasks

1. [x] Add `contextStatusMessage` method to `sessionAgent`
2. [x] Call it in `PrepareStep` callback, gated by `!a.isSubAgent`
3. [x] Verify compilation

## NOTES

Added `contextStatusMessage()` method at `internal/agent/agent.go:928-947`. It:
- Returns `(fantasy.Message, bool)` — false when context window is 0 or unknown
- Computes `used_pct` as integer percentage, `remaining_tokens`, and `context_window`
- Outputs a compact `<context_status>` JSON block as a user message (same pattern as `<system_reminder>`)
- Clamps `remaining` to 0 if negative

Injection point: `internal/agent/agent.go:291-296`, inside `PrepareStep`, after prompt prefix prepend, before assistant message creation. Gated by `!a.isSubAgent`.
