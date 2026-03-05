# Add SetCompactionMethod() to Config

Status: COMPLETED

## Sub tasks

1. [x] Add `SetCompactionMethod(method CompactionMethod) error` method after `SetCompactMode()` in `config.go`

## NOTES

Added at `internal/config/config.go:493` following the exact same pattern as `SetCompactMode()`:
- Nil-guards `Options`
- Sets in-memory field
- Calls `SetConfigField("options.compaction_method", method)` to persist to data config
