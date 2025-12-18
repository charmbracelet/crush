# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Crush is a terminal-based AI coding assistant written in Go. The project is also known as "karigor" internally (package name: `github.com/charmbracelet/crush`). It provides an interactive TUI for AI-assisted software development with support for multiple LLM providers, LSP integration, and Model Context Protocol (MCP) servers.

## Build/Test/Lint Commands

- **Build**: `go build .` or `go run .`
- **Run**: `task run` or `go run . {{.CLI_ARGS}}`
- **Test**: `task test` or `go test ./...`
  - Run single test: `go test ./internal/llm/prompt -run TestGetContextFromPaths`
  - Update golden files: `go test ./... -update`
  - Update specific package: `go test ./internal/tui/components/core -update`
  - Record VCR cassettes: `task test:record` (re-records all test fixtures)
- **Lint**: `task lint:fix` (runs golangci-lint with auto-fix)
- **Format**: `task fmt` (runs gofumpt -w .)
- **Dev with profiling**: `task dev` (sets CRUSH_PROFILE=true)
- **Generate schema**: `task schema` (generates schema.json from config structs)
- **Install**: `task install` (installs to GOPATH with version info)

## Architecture Overview

### Core Application Layers

1. **CLI Layer** (`internal/cmd/`)
   - Entry point and command handling via cobra
   - `root.go`: Main command setup, app initialization, metrics toggle
   - `run.go`: Non-interactive mode for single prompts
   - `projects.go`: Multi-project management
   - `logs.go`: Log viewing commands

2. **App Layer** (`internal/app/`)
   - Wires together all services and coordinates lifecycle
   - Initializes agents, sessions, messages, permissions, LSP clients
   - Manages event pub/sub system for TUI communication
   - Creates and coordinates the `AgentCoordinator`

3. **Agent Layer** (`internal/agent/`)
   - Core AI orchestration using Fantasy library (charm.land/fantasy)
   - `SessionAgent`: Manages conversations, tool execution, auto-summarization
   - `Coordinator`: Routes requests to appropriate agents, handles model switching
   - `tools/`: Built-in tools (bash, edit, fetch, grep, glob, ls, view, write, etc.)
   - `tools/mcp/`: MCP server integration (stdio, http, sse transports)
   - Supports queuing, cancellation, and concurrent session handling

4. **TUI Layer** (`internal/tui/`)
   - Built with Bubble Tea v2 (charm.land/bubbletea/v2)
   - Page-based navigation system (chat, settings, etc.)
   - Components: chat, dialogs, status bar, file picker, model selector
   - Uses lipgloss v2 for styling and layout
   - Mouse event throttling to prevent trackpad spam

5. **Data Layer** (`internal/db/`)
   - SQLite database via ncruces/go-sqlite3
   - Managed with sqlc for type-safe queries
   - Goose for migrations (`internal/db/migrations/`)
   - Stores sessions, messages, files, and conversation history

6. **LSP Integration** (`internal/lsp/`)
   - Language Server Protocol clients for code context
   - Dynamically initialized based on config
   - Provides diagnostics, hover info, and references to agents

### Key Services

- **Sessions** (`internal/session/`): Session lifecycle management, CRUD operations
- **Messages** (`internal/message/`): Message storage, attachments, provider tracking
- **History** (`internal/history/`): File context tracking per session
- **Permissions** (`internal/permission/`): Tool execution permission system
- **Pub/Sub** (`internal/pubsub/`): Event broadcasting for UI updates

### Configuration System (`internal/config/`)

- Multi-layer config loading (`.crush.json`, `crush.json`, `~/.config/crush/crush.json`)
- Auto-updates provider list from Catwalk (charmbracelet/catwalk) unless disabled
- Supports custom providers (OpenAI-compatible, Anthropic-compatible)
- LSP, MCP, permissions, and tool configuration
- Default context paths: `.github/copilot-instructions.md`, `.cursorrules`, `CLAUDE.md`, `AGENTS.md`, etc.

### Fantasy Library Integration

Crush uses the Fantasy library (`charm.land/fantasy`) as its LLM abstraction layer:
- Unified interface for Anthropic, OpenAI, Gemini, Bedrock, Azure, etc.
- Handles streaming, tool calls, function calling
- Provider-agnostic `LanguageModel` and `AgentTool` interfaces

### Database Schema

Core tables (see `internal/db/migrations/`):
- `sessions`: Chat sessions with model info, created_at, updated_at
- `messages`: Individual messages with role, content, attachments, provider
- `files`: File context history per session
- Indexes on created_at for efficient sorting

## Code Style Guidelines

- **Imports**: Use goimports formatting, group stdlib, external, internal packages
- **Formatting**: Use gofumpt (stricter than gofmt), enabled in golangci-lint
- **Naming**: Standard Go conventions - PascalCase for exported, camelCase for unexported
- **Types**: Prefer explicit types, use type aliases for clarity (e.g., `type AgentName string`)
- **Error handling**: Return errors explicitly, use `fmt.Errorf` for wrapping
- **Context**: Always pass context.Context as first parameter for operations
- **Interfaces**: Define interfaces in consuming packages, keep them small and focused
- **Structs**: Use struct embedding for composition, group related fields
- **Constants**: Use typed constants with iota for enums, group in const blocks
- **Testing**: Use testify's `require` package, parallel tests with `t.Parallel()`,
  `t.SetEnv()` to set environment variables. Always use `t.Tempdir()` when in
  need of a temporary directory. This directory does not need to be removed.
- **JSON tags**: Use snake_case for JSON field names
- **File permissions**: Use octal notation (0o755, 0o644) for file permissions
- **Comments**: End comments in periods unless comments are at the end of the line. Wrap at 78 columns.

## Testing with Mock Providers

When writing tests that involve provider configurations, use the mock providers to avoid API calls:

```go
func TestYourFunction(t *testing.T) {
    // Enable mock providers for testing
    originalUseMock := config.UseMockProviders
    config.UseMockProviders = true
    defer func() {
        config.UseMockProviders = originalUseMock
        config.ResetProviders()
    }()

    // Reset providers to ensure fresh mock data
    config.ResetProviders()

    // Your test code here - providers will now return mock data
    providers := config.Providers()
    // ... test logic
}
```

## Formatting

- ALWAYS format any Go code you write.
  - First, try `gofumpt -w .`.
  - If `gofumpt` is not available, use `goimports`.
  - If `goimports` is not available, use `gofmt`.
  - You can also use `task fmt` to run `gofumpt -w .` on the entire project,
    as long as `gofumpt` is on the `PATH`.

## Committing

- ALWAYS use semantic commits (`fix:`, `feat:`, `chore:`, `refactor:`, `docs:`, `sec:`, etc).
- Try to keep commits to one line, not including your attribution. Only use
  multi-line commits when additional context is truly necessary.

## Important Patterns

### Event System

The app uses a pub/sub event system (`internal/pubsub/`) for decoupling services from the TUI:
- Services emit events (e.g., MessageCreated, SessionUpdated)
- TUI subscribes and updates UI accordingly
- Events flow through channels managed by `App.Subscribe()`

### Concurrent Safety

Use `internal/csync` package for concurrent data structures:
- `csync.Map[K, V]`: Thread-safe map with generics
- `csync.Slice[T]`: Thread-safe slice
- `csync.VersionedMap[K, V]`: Map with version tracking for optimistic updates

### Tool System

Built-in tools are in `internal/agent/tools/`:
- Each tool implements `fantasy.AgentTool` interface
- Tools can be allowed/disabled via config
- MCP tools are dynamically loaded from external servers

### LSP Client Management

LSP clients are lazily initialized per language:
- Config specifies command, args, env per language
- Clients stored in `App.LSPClients` (csync.Map)
- Used by agents to fetch diagnostics, symbols, references

## Key Dependencies

- **TUI**: charm.land/bubbletea/v2, charm.land/lipgloss/v2
- **LLM**: charm.land/fantasy, charmbracelet/catwalk
- **Database**: ncruces/go-sqlite3, pressly/goose/v3
- **CLI**: spf13/cobra, charmbracelet/fang
- **HTTP**: openai/openai-go/v2, charmbracelet/anthropic-sdk-go
- **Cloud**: AWS SDK v2, Azure SDK, Google Cloud (Vertex AI, Gemini)
- **Utils**: tidwall/gjson, tidwall/sjson, google/uuid

## Environment Variables

Common env vars (see README.md for full list):
- `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GEMINI_API_KEY`, etc.
- `CRUSH_DISABLE_METRICS=1`: Opt out of telemetry
- `CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1`: Disable Catwalk auto-updates
- `CRUSH_PROFILE=true`: Enable pprof server on :6060

## Data Directories

- Project-local: `./.crush/` (logs, database, cache)
- Global config: `~/.config/crush/crush.json`
- Global state: `~/.local/share/crush/crush.json` (Unix) or `%LOCALAPPDATA%\crush\crush.json` (Windows)
