# Implement signal mechanism for new session creation in agent coordinator and register the tool

Status: COMPLETED

## Sub tasks

1. [x] Add `NewNewSessionTool()` to `buildTools()` in `internal/agent/coordinator.go`.
2. [x] Define `NewSessionError` sentinel error in `new_session.go` so the coordinator can detect it.
3. [x] Intercept `NewSessionError` in the UI's `sendMessage()` goroutine in `internal/ui/model/ui.go`.
4. [x] Define `newSessionMsg` Bubble Tea message type and handle it in `Update()` to call `m.newSession()` + send the summary as a new message.

## NOTES

The signal mechanism uses a sentinel error pattern:
- `NewSessionError` is returned from the tool's execute function.
- The coordinator's `Run()` propagates this error up.
- In `ui.go:sendMessage()`, `errors.As(err, &nse)` catches it and returns a `newSessionMsg`.
- The `Update()` handler calls `m.newSession()` (which clears the session) and then sends `sendMessageMsg{Content: nse.Summary}` to auto-start the new session with the summary.
