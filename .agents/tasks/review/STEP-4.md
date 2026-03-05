# Conditionally register new_session tool only for LLM compaction

Status: COMPLETED

## Sub tasks

1. [x] Move `tools.NewNewSessionTool()` out of the unconditional `allTools` append in `buildTools()`
2. [x] Add conditional: `if c.cfg.Options.CompactionMethod == config.CompactionLLM { allTools = append(allTools, tools.NewNewSessionTool()) }`
3. [x] Verify build passes

## NOTES

- Changed `coordinator.go:446-465`: Removed `NewNewSessionTool()` from the main tools list, added it conditionally after.
- Build passes. No test changes needed — the tool is filtered by `AllowedTools` anyway, and existing tests don't test the conditional registration logic in `buildTools`.
