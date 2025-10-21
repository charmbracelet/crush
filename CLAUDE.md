# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Cliffy** is a fast, focused AI coding assistant for one-off tasks. It's a headless fork of [Crush](https://github.com/charmbracelet/crush) designed for direct execution without TUI, database, or sessions. Key goals:
- **Speed**: 200ms cold start target (vs. Crush's 2.3s)
- **Zero Persistence**: No database writes, no session management
- **Transparency**: Optional thinking/reasoning output (hidden in Crush)
- **Volley Mode**: Parallel execution of multiple tasks

## Build and Test Commands

```bash
# Build the binary
go build -o bin/cliffy ./cmd/cliffy

# Run tests
go test ./...

# Run tests with coverage
go test ./... -cover

# Run linter (requires golangci-lint)
golangci-lint run --path-mode=abs --config=".golangci.yml" --timeout=5m

# Using Task runner (alternative)
task build    # Build the project
task test     # Run tests
task lint     # Run linter
```

## Running Cliffy

```bash
# Single task
./bin/cliffy "list all Go files"

# Multiple tasks (parallel execution via volley)
./bin/cliffy "analyze auth.go" "analyze db.go"

# With shared context
./bin/cliffy --context "You are a security expert" "review auth.go" "review db.go"

# Show progress and stats
./bin/cliffy --verbose "task1" "task2"

# Model selection
./bin/cliffy --fast "quick task"    # small model
./bin/cliffy --smart "complex task" # large model
```

## Architecture

### Execution Paths

All tasks route through the **volley scheduler** (`internal/volley/scheduler.go`), even single tasks. This unified approach provides:
- Parallel execution for multiple tasks
- Automatic retry with exponential backoff
- Progress tracking and token usage stats
- Rate limiting and concurrency control

Key flow:
1. `cmd/cliffy/main.go` → `executeVolley()` creates tasks
2. `volley.Scheduler.Execute()` manages worker pool
3. Workers call `agent.Run()` which streams events
4. Results collected and output in order

### Core Components

**Agent System** (`internal/llm/agent/`)
- `agent.go`: Main agent implementation using LLM providers
- `agent-tool.go`: Nested agent tool for complex tasks
- Supports "coder" and "task" agent types via prompt selection
- Streams events: `AgentEventTypeResponse`, `AgentEventTypeError`

**Volley Scheduler** (`internal/volley/`)
- `scheduler.go`: Worker pool with configurable concurrency (default: 3)
- `task.go`: Task definition and result types
- `progress.go`: Live progress tracking (when `--verbose`)
- Retry logic: 3 attempts with exponential backoff (1s, 2s, 4s...)
- Smart retry: retries rate limits (429), timeouts, network errors; skips auth (401/403) and validation (400) errors

**LLM Tools** (`internal/llm/tools/`)
- File operations: `view.go`, `edit.go`, `write.go`, `multiedit.go`
- Search: `glob.go`, `grep.go`, `rg.go`
- Shell: `bash.go` (persistent shell via `internal/shell/persistent.go`)
- LSP integration: `diagnostics.go` (lazy initialization)
- MCP tools: Dynamically loaded from MCP servers

**Provider Abstraction** (`internal/llm/provider/`)
- Unified interface across OpenAI, Anthropic, Gemini, Azure, Bedrock, Vertex AI
- OpenRouter support via OpenAI-compatible provider
- Token usage tracking in `TokenUsage` struct

**Configuration** (`internal/config/`)
- Loads from `~/.config/cliffy/cliffy.json` (global) or `.cliffy.json`/`cliffy.json` (local)
- Provider configs with model definitions
- Agent configs map to prompts via `agentPromptMap` in `agent.go:79`
- Context paths: Auto-loads from `.cursorrules`, `CLAUDE.md`, `.github/copilot-instructions.md`, etc.

### Message Flow

```
User Input
  ↓
Task[] (volley.Task)
  ↓
Scheduler (worker pool)
  ↓
Agent.Run(ctx, sessionID, prompt)
  ↓
Provider (OpenAI/Anthropic/etc.)
  ↓
Event Stream (<-chan AgentEvent)
  ↓
Results (TaskResult[])
```

### Key Design Patterns

1. **Lazy Initialization**: LSP clients, tools loaded on-demand
2. **Context Cancellation**: Each volley has cancelable context for fail-fast
3. **In-Memory Store**: `message.Store` holds conversation without DB
4. **Event Streaming**: Provider responses stream via channels
5. **Concurrent Maps**: `csync.Map[K,V]` for thread-safe state

## Important Implementation Details

### Provider Configuration

Default uses OpenRouter with free models (DeepSeek R1):
```json
{
  "models": {
    "large": {
      "provider": "openrouter",
      "model": "deepseek/deepseek-r1:free"
    },
    "small": {
      "provider": "openrouter",
      "model": "deepseek/deepseek-r1-distill-qwen-14b:free"
    }
  },
  "providers": {
    "openrouter": {
      "base_url": "https://openrouter.ai/api/v1",
      "api_key": "${CLIFFY_OPENROUTER_API_KEY}"
    }
  }
}
```

Other available free models include `google/gemini-2.0-flash-exp:free`.

### Environment Variables

- `CLIFFY_OPENROUTER_API_KEY`: Primary API key (required)
- Provider-specific keys: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, etc.
- Variables expand via `${VAR}` syntax in config

### Session Management

Each task gets unique `sessionID` (format: `volley-{timestamp}`). Messages stored in-memory via `message.Store`, never persisted.

### Error Handling

- Retryable errors: Rate limits (429), timeouts, network issues
- Fatal errors: Auth failures (401/403), bad requests (400)
- Retry delay: Exponential backoff (1s → 2s → 4s → 8s, capped at 60s)

### Tool Execution

Tools receive context with `SessionIDContextKey` and `MessageIDContextKey`. The `BaseTool` interface requires:
```go
Info() ToolInfo
Name() string
Run(ctx context.Context, params ToolCall) (ToolResponse, error)
```

### Shell Commands

Persistent shell via `internal/shell/persistent.go` maintains working directory across calls. Uses `mvdan.cc/sh` for shell interpretation.

## Testing

The benchmark suite in `benchmark/` compares Cliffy vs Crush performance:
- `bench.sh`: Runs comparative benchmarks
- `tasks.json`: Defines test tasks
- `config/`: Separate configs for each tool
- Results show 1.11x-1.54x speedup over Crush

Run benchmarks:
```bash
cd benchmark
./bench.sh
./report.sh  # Generate summary
```

## File Naming Conventions

- `*_test.go`: Unit tests
- `*.md` in `internal/llm/prompt/`: System prompts
- Executables in `cmd/*/main.go`

## Dependencies

- **Go 1.25.0** (required)
- `github.com/charmbracelet/catwalk`: Model abstraction
- `github.com/spf13/cobra`: CLI framework
- `github.com/mark3labs/mcp-go`: MCP protocol
- `mvdan.cc/sh`: Shell interpreter

## Common Patterns

### Adding a New Tool

1. Create `internal/llm/tools/newtool.go` implementing `BaseTool`
2. Add to allowed tools list in agent config
3. Update tool registration in `tools.go`

### Adding a New Provider

1. Implement `provider.Provider` interface in `internal/llm/provider/`
2. Add to provider factory in `provider.go`
3. Update config schema with provider type

### Modifying Prompts

Prompts in `internal/llm/prompt/*.md` are embedded at compile time. Changes require rebuild. Prompt selection via `agentPromptMap` in `agent.go`.
