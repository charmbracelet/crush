# Current Design: Copilot Review Fixes

## Error Handling in `agent.Run()`

**File**: `internal/agent/agent.go:441-540`

When `agent.Run()` encounters an error, it dispatches through a chain of type checks:

1. `context.Canceled` → `FinishReasonCanceled`
2. `permission.ErrorPermissionDenied` → `FinishReasonPermissionDenied`
3. `hyper.ErrNoCredits` → `FinishReasonError` with "No credits"
4. `*fantasy.ProviderError` → `FinishReasonError` with provider message
5. `*fantasy.Error` → `FinishReasonError` with fantasy message
6. **Fallback** → `FinishReasonError` with "Provider Error" title

`NewSessionError` (from `internal/agent/tools/new_session.go`) currently falls
through to the generic fallback (case 6), persisting a spurious "Provider
Error" finish entry in the old session's message history.

The `tools` package is already imported at `agent.go:35`.

## `contextStatusMessage` Implementation

**File**: `internal/agent/agent.go:938-953`

The JSON payload includes three fields:
- `used_pct`
- `remaining_tokens`
- `context_window`

## Tool Documentation

**File**: `internal/agent/tools/new_session.md:1-22`

The `<when_to_use>` section only mentions `used_pct` and `remaining_tokens`,
omitting the `context_window` field that is present in the actual payload.
