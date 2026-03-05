# Verify all read sites use .Get() on csync values

Status: COMPLETED

## Sub tasks

1. [x] Check `PrepareStep` at `agent.go:295` — `a.disableContextStatus.Get()` ✓
2. [x] Check `StopWhen` at `agent.go:427` — `a.disableAutoSummarize.Get()` ✓
3. [x] Check `contextStatusMessage` at `agent.go:934` — no direct flag read (caller gates) ✓

## NOTES

All read sites were already converted to `.Get()` in Step 8 when the fields were changed from `bool` to `*csync.Value[bool]`. No further code changes needed — this step is purely a verification checkpoint.

Read sites confirmed:
- `agent.go:295`: `!a.disableContextStatus.Get()` in `PrepareStep`
- `agent.go:427`: `!a.disableAutoSummarize.Get()` in `StopWhen`
- `contextStatusMessage` (agent.go:934) doesn't check flags — gating is done by the caller in PrepareStep
