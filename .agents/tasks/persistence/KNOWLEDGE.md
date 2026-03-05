# Knowledge

## Codebase Search

- Use the `agent` tool when querying questions about the codebase. Also use
  the `tldr` command for quickly searching for function references (you can
  pass the output of this tool to subagents).
  - Whenever you use the `agent` tool, always warn the agent not to perform
    expensive operations like calling `grep` for something in the entire home
    directory, but to call these tools in a way that restricts their scope (so
    that they don't take forever to run).

## Persistence Pattern

- All runtime setting persistence uses `Config.SetConfigField(key, value)` in
  `internal/config/config.go:520`. This patches
  `~/.local/share/crush/crush.json` (the "data config") via `sjson.Set`.
- Follow the compact mode pattern as a reference:
  - `Config.SetCompactMode()` at `config.go:486` mutates in-memory + calls
    `SetConfigField`.
  - UI handler at `ui.go:1360` calls the config method.
- The `CompactionMethod` field lives in `Options` struct at `config.go:275`
  with JSON tag `compaction_method`.
- Currently the UI handler at `ui.go:1372` only sets
  `cfg.Options.CompactionMethod` in memory — it does NOT call
  `SetConfigField`, so the setting is lost on restart.
