# Plan: Fix Sub-Agent Model Selection (Issue #2358)

## Problem

The `agent` tool (sub-agent) always uses the large model for inference, ignoring the `Agent.Model` field in the config. The `AgentTask` config specifies `Model: SelectedModelTypeLarge`, but even if changed to `SelectedModelTypeSmall`, the `buildAgent()` function doesn't respect it.

## Approach

Modify `buildAgent()` in `coordinator.go` to respect the `agent.Model` field when constructing the `SessionAgent`. When `agent.Model == SelectedModelTypeSmall`, pass the small model as the `LargeModel` to `NewSessionAgent` (matching the pattern used by `agentic_fetch_tool.go`).

Additionally, change `AgentTask` in `SetupAgents()` to use `SelectedModelTypeSmall` so the task sub-agent defaults to the user's configured small model.

## Changes

1. **`internal/config/config.go` — `SetupAgents()`**: Change `AgentTask.Model` from `SelectedModelTypeLarge` to `SelectedModelTypeSmall`.

2. **`internal/agent/coordinator.go` — `buildAgent()`**: Use the `agent.Model` field to determine which model to pass as the primary model. When `agent.Model == SelectedModelTypeSmall`, use the small model as `LargeModel` in `SessionAgentOptions` (and use the small provider's `SystemPromptPrefix`).

3. **Tests**: Verify existing tests pass; add test coverage if appropriate test infrastructure exists.
