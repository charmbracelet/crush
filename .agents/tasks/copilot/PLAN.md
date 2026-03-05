# Plan: Copilot Review Fixes

## Issue 1: Handle `NewSessionError` specially in `agent.Run()`

Add a new branch in the error dispatch chain in `agent.Run()` (around line
509) that checks for `*tools.NewSessionError` before the generic fallback.
When matched, call `AddFinish` with `FinishReasonEndTurn` instead of
`FinishReasonError` to avoid persisting a spurious "Provider Error" in the
session history.

## Issue 2: Update `new_session.md` documentation

Update line 4 of `internal/agent/tools/new_session.md` to mention all three
fields: `used_pct`, `remaining_tokens`, and `context_window`.
