# Convert disableAutoSummarize and disableContextStatus to *csync.Value[bool]

Status: COMPLETED

## Sub tasks

1. [x] Change field types on `sessionAgent` struct from `bool` to `*csync.Value[bool]`
2. [x] Update `NewSessionAgent` constructor to wrap values with `csync.NewValue()`
3. [x] Update read site in `StopWhen` (agent.go:426) to use `.Get()`
4. [x] Update read site in `PrepareStep` (agent.go:294) to use `.Get()`
5. [x] Verify build passes
6. [x] Run existing tests in `internal/agent/`

## Notes

All changes in `internal/agent/agent.go`:
- Struct fields at lines 109-110: `bool` → `*csync.Value[bool]`
- Constructor at lines 142-143: `opts.DisableAutoSummarize` → `csync.NewValue(opts.DisableAutoSummarize)`
- PrepareStep at line 294: `!a.disableContextStatus` → `!a.disableContextStatus.Get()`
- StopWhen at line 426: `!a.disableAutoSummarize` → `!a.disableAutoSummarize.Get()`

The `csync` import was already present. Test files that create bare `&sessionAgent{}` still compile because the zero-value of `*csync.Value[bool]` is `nil`, and those tests don't touch the flag fields.
