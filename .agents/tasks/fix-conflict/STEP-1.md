# Define a configuration/state setting to track the current compaction method

Status: COMPLETED

## Sub tasks

1. [x] Add `CompactionMethod` string constant type and values to `internal/config/config.go`
2. [x] Add `CompactionMethod` field to `Options` struct
3. [x] Update `coordinator.go` `buildAgent` to derive `DisableAutoSummarize`/`DisableContextStatus` from `CompactionMethod`
4. [x] Fix positional `SessionAgentOptions` in coordinator.go and common_test.go to use named fields
5. [x] Run tests

## NOTES

- Added `CompactionMethod` type with `CompactionAuto` ("auto") and `CompactionLLM` ("llm") constants at `internal/config/config.go:54-63`.
- Added `CompactionMethod` field to `Options` struct at `internal/config/config.go:276`.
- Added `compactionFlags()` helper in `coordinator.go` that maps method to the two bool flags.
- `buildAgent` now calls `compactionFlags(c.cfg.Options.CompactionMethod, c.cfg.Options.DisableAutoSummarize)`.
- Converted positional `SessionAgentOptions` to named fields in `coordinator.go:373` and `common_test.go:156`.
- `agentic_fetch_tool.go:175` already used named fields.
- All tests pass (`go test ./internal/config/ ./internal/agent/`).
