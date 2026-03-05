# Change AgentTask to use small model and update buildAgent() to respect agent.Model

Status: COMPLETED

## Sub tasks

1. [x] Change `AgentTask.Model` from `SelectedModelTypeLarge` to `SelectedModelTypeSmall` in `config.go:772`
2. [x] Update `buildAgent()` in `coordinator.go:363` to select primary model based on `agent.Model` field
3. [x] Build project successfully
4. [x] Run all tests — all pass
5. [x] Format with gofumpt

## NOTES

### Changes Made

**`internal/config/config.go:772`** — Changed `AgentTask.Model` from `SelectedModelTypeLarge` to `SelectedModelTypeSmall`. This makes the task sub-agent default to the user's configured small model.

**`internal/agent/coordinator.go:363-405`** — Updated `buildAgent()` to:
- Determine the `primary` model based on `agent.Model` (large or small)
- Pass `primary` as `LargeModel` to `NewSessionAgent` (since `Model()` returns `largeModel`)
- Use `primary`'s provider config for `SystemPromptPrefix`
- Build the system prompt using `primary`'s provider/model info

This follows the same pattern already used by `agentic_fetch_tool.go:176` which passes `small` as `LargeModel`.
