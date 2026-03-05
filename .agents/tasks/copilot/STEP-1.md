# Handle `NewSessionError` specially in `agent.Run()`

Status: COMPLETED

## Sub tasks

1. [x] Add `isNewSessionErr` helper function near `contextStatusMessage`
2. [x] Add branch in error dispatch chain before generic fallback
3. [x] Build and test

## NOTES

Added `isNewSessionErr()` at `internal/agent/agent.go:938` and a new branch
at line 531 that calls `AddFinish(FinishReasonEndTurn, ...)` instead of
`FinishReasonError` when the error is a `*tools.NewSessionError`.
