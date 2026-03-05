# Current Design: Sub-Agent Model Selection

## Issue Summary

GitHub issue #2358: When a user sets the main model to Opus 4.6 (via Copilot) and the "small task" model to Gemini Flash 3.1 Preview (via OpenRouter), sub-agents spawned by the `agent` tool still use the large model (Opus) instead of the configured small model. The sub-agent identifies as "Claude by Anthropic" because it's actually calling Opus, not Gemini.

## Key Files and Functions

### Config Layer

- **`internal/config/config.go:54-63`** — `SelectedModelType` with constants `SelectedModelTypeLarge = "large"` and `SelectedModelTypeSmall = "small"`.
- **`internal/config/config.go:329-340`** — `Agent` struct has a `Model SelectedModelType` field that specifies which model type (`"large"` or `"small"`) the agent should use.
- **`internal/config/config.go:755-780`** — `SetupAgents()` configures both `AgentCoder` and `AgentTask` with `Model: SelectedModelTypeLarge`. The `AgentTask` agent (used by the `agent` tool) is **hardcoded to use the large model type**.

### Agent Tool (Sub-Agent Spawning)

- **`internal/agent/agent_tool.go:26-66`** — `agentTool()` fetches `config.AgentTask` agent config and calls `c.buildAgent(ctx, prompt, agentCfg, true)`.

### Agent Builder

- **`internal/agent/coordinator.go:363-401`** — `buildAgent()` calls `buildAgentModels()` to get both large and small `Model` instances, then creates `NewSessionAgent` passing **both** as `LargeModel` and `SmallModel`. It **ignores** the `agent.Model` field for actual model selection — it's only used at line 424 to get the model name for the bash tool.
- **`internal/agent/coordinator.go:497-578`** — `buildAgentModels()` correctly builds both large and small models from config. This function works correctly.

### Sub-Agent Execution

- **`internal/agent/coordinator.go:960-996`** — `runSubAgent()` calls `params.Agent.Model()` (line 974) which returns the `largeModel` (see below), then passes it to `params.Agent.Run()`.
- **`internal/agent/agent.go:1019-1021`** — `Model()` method always returns `a.largeModel.Get()`.
- **`internal/agent/agent.go:193-194`** — `Run()` creates `fantasy.NewAgent(largeModel.Model, ...)` — **always uses the large model for LLM inference**.

### Correct Pattern: Agentic Fetch

- **`internal/agent/agentic_fetch_tool.go:149-185`** — The `agentic_fetch` tool correctly uses the small model by passing it as `LargeModel: small` when creating its `SessionAgent`. This is the workaround pattern that already exists.

## Root Cause

The `AgentTask` config has `Model: SelectedModelTypeLarge`, but `buildAgent()` ignores this field for model selection. It always passes the actual large model as `LargeModel` to `NewSessionAgent`. The `Model()` method on `sessionAgent` always returns `largeModel`. Therefore, the sub-agent always calls the large model (Opus), never the configured small model (Gemini Flash).

The `agent.Model` field is only used in `buildTools()` (coordinator.go:424) to get the display name for the bash tool — it has no effect on which model is used for inference.

## Expected Fix

The `buildAgent()` function should respect the `agent.Model` field. When `agent.Model == SelectedModelTypeSmall`, the sub-agent should use the small model for inference. The cleanest approach: pass the appropriate model based on `agent.Model` as the `LargeModel` to `NewSessionAgent` (similar to how `agentic_fetch_tool.go` passes `small` as `LargeModel`).
