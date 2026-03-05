# Update UI handler to call SetCompactionMethod()

Status: COMPLETED

## Sub tasks

1. [x] Replace direct field assignment with `cfg.SetCompactionMethod()` call in `ui.go`
2. [x] Add error handling for persistence failure

## NOTES

Changed `internal/ui/model/ui.go:1372` from:
```go
cfg.Options.CompactionMethod = config.CompactionMethod(msg.Method)
```
To:
```go
if err := cfg.SetCompactionMethod(config.CompactionMethod(msg.Method)); err != nil {
    cmds = append(cmds, util.ReportError(fmt.Errorf("failed to persist compaction method: %w", err)))
    break
}
```
