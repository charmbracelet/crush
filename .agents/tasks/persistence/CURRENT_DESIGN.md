# Current Design: Compaction Method Persistence

## Problem

When the user selects a compaction method (Auto vs LLM/User-driven) via the
command palette, the setting only applies to the current session. On restart,
it reverts to the default (`"auto"`).

## Relevant Code

### Config layer — `internal/config/config.go`

- `CompactionMethod` type (line 54): `"auto"` or `"llm"`.
- `Options` struct (line 259): has `CompactionMethod` field with JSON tag
  `compaction_method`.
- `SetCompactMode()` (line 486): reference pattern — mutates in-memory struct
  + calls `SetConfigField("options.tui.compact_mode", ...)` to persist to
  data config on disk.
- `SetConfigField()` (line 520): reads `~/.local/share/crush/crush.json`,
  patches via `sjson.Set`, writes back.

### UI layer — `internal/ui/model/ui.go`

- `ActionSelectCompactionMethod` handler (line 1360): sets
  `cfg.Options.CompactionMethod` in memory, calls `UpdateAgentModel()`, shows
  toast. Does **not** persist to disk.

### Agent layer — `internal/agent/coordinator.go`

- `compactionFlags()` (line 411): derives `disableAutoSummarize` and
  `disableContextStatus` bools from `CompactionMethod`.
- `UpdateAgentModel()` (line 911): re-derives flags and calls
  `SetCompactionFlags()` on the live agent.
