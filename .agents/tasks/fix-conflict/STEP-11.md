# Wire SetCompactionFlags into coordinator.UpdateModels

Status: COMPLETED

## Sub tasks

1. [x] Add `compactionFlags()` call + `SetCompactionFlags()` in `coordinator.UpdateModels`
2. [x] Run tests to confirm no regressions
3. [x] Run `gofumpt` to format

## NOTES

Added two lines to `coordinator.go:911-912` in `UpdateModels`:
```go
disableAutoSummarize, disableContextStatus := compactionFlags(c.cfg.Options.CompactionMethod, c.cfg.Options.DisableAutoSummarize)
c.currentAgent.SetCompactionFlags(disableAutoSummarize, disableContextStatus)
```

This re-derives the compaction flags from the current config every time `UpdateModels` is called (which happens when the user switches compaction mode via the UI command palette). The flags are applied to the live agent via `SetCompactionFlags`, which uses `.Set()` on the thread-safe `*csync.Value[bool]` fields.
