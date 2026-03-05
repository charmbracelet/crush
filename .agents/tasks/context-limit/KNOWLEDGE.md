Use the `agent` tool when querying questions about the codebase. Also use the `tldr` command for quickly searching for function references (you can pass the output of this tool to subagents).

Wheenver you use the `agent` tool, always warn the agent not to perform expensive operations like calling `grep` for something in the entire home directory, but to call these tools in a way that restricts their scope (so that they don't take forever to run).

## Key Codebase Facts (from new-session task)

- The tool allowlist lives in `allToolNames()` at `internal/config/config.go:703-727`. Any new tool must be added here or it gets silently dropped by `buildTools()` filtering against `agent.AllowedTools`.
- Tools are registered in `coordinator.buildTools()` at `internal/agent/coordinator.go:438+`.
- The `new_session` tool (`internal/agent/tools/new_session.go`) returns a sentinel `NewSessionError` which propagates up through the agent loop. The UI catches it in `sendMessage` (`internal/ui/model/ui.go:2744`) and converts it to a `newSessionMsg`.
- `PrepareStep` callback (`internal/agent/agent.go:253-306`) fires **before each LLM call** in the agentic loop — this is the injection point for per-turn metadata like context usage info.
- The todo reminder is injected as a `<system_reminder>` user message in `preparePrompt()` at `internal/agent/agent.go:714-720`. This is a one-time injection at prompt build time, not per-turn.
- Token counts come exclusively from LLM provider responses (`fantasy.Usage`). There is no local tokenizer.
- `session.PromptTokens` and `session.CompletionTokens` are accumulated in the DB via `UpdateTitleAndUsage` (SQL uses `prompt_tokens = prompt_tokens + ?`).
- The `StopWhen` condition at `internal/agent/agent.go:406-422` already computes `remaining = contextWindow - (promptTokens + completionTokens)` and triggers auto-summarize. This is the same calculation we need for the context usage info.
- The UI header already shows context percentage at `internal/ui/model/header.go:130`.
- `catwalk.Model.ContextWindow` (int64) is the model's max context size in tokens.
- `contextStatusMessage()` uses `fantasy.NewUserMessage` (not system message) to match the `<system_reminder>` pattern. This is intentional — some providers handle system messages differently.
- `updateSessionUsage` (called in `OnStepFinish`) overwrites `session.PromptTokens` and `session.CompletionTokens` with the latest values from the provider response (not cumulative). So `contextStatusMessage` sees the most recent turn's token counts, which reflect the full conversation size.
- Line numbers shifted after edits: `PrepareStep` injection is now around line 291-296, `contextStatusMessage` method is around line 928-947.
