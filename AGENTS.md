# Crush Development Guide

Crush is a terminal-based AI coding assistant built on the Charm ecosystem. It provides session-based AI agent functionality for managing conversations, tool execution, and LLM interactions.

## Build/Test/Lint Commands

- **Build**: `go build .` or `go run .`
- **Test**: `task test` or `go test ./...` (run single test: `go test ./internal/agent -run TestCoderAgent`)
- **Test with race detection**: `go test -race -failfast ./...`
- **Update Golden Files**: `go test ./... -update` (regenerates .golden files when test output changes)
  - Update specific package: `go test ./internal/ui -update`
- **Re-record VCR cassettes**: `task test:record` (re-records API interactions for tests)
- **Lint**: `task lint` or `task lint:fix` (auto-fix)
- **Format**: `task fmt` (`gofumpt -w .`)
- **Modernize**: `task modernize` (runs `modernize` for code simplifications)
- **Dev**: `task dev` (runs with profiling enabled at localhost:6060)
- **Install**: `task install` (installs to GOPATH)
- **Generate schema**: `task schema` (generates schema.json from config)
- **Release**: `task release` (creates and pushes semver tag)

## Code Style Guidelines

- **Imports**: Use `goimports` formatting, group stdlib, external, internal packages
- **Formatting**: Use gofumpt (stricter than gofmt), enabled in golangci-lint
- **Naming**: Standard Go conventions - PascalCase for exported, camelCase for unexported
- **Types**: Prefer explicit types, use type aliases for clarity (e.g., `type AgentName string`)
- **Error handling**: Return errors explicitly, use `fmt.Errorf` for wrapping
- **Context**: Always pass `context.Context` as first parameter for operations
- **Interfaces**: Define interfaces in consuming packages, keep them small and focused
- **Structs**: Use struct embedding for composition, group related fields
- **Constants**: Use typed constants with iota for enums, group in const blocks
- **Testing**: Use testify's `require` package, parallel tests with `t.Parallel()`,
  `t.SetEnv()` to set environment variables. Always use `t.TempDir()` when in
  need of a temporary directory. This directory does not need to be removed.
- **JSON tags**: Use snake_case for JSON field names
- **File permissions**: Use octal notation (0o755, 0o644) for file permissions
- **Log messages**: Log messages must start with a capital letter (e.g., "Failed to save session" not "failed to save session")
  - This is enforced by `task lint:log` which runs as part of `task lint`
- **Comments**: End comments in periods unless comments are at the end of the line.

## Architecture Overview

```
internal/
├── agent/          # Core AI agent orchestration, tool definitions, prompts
│   ├── tools/      # Individual tool implementations (bash, edit, view, etc.)
│   │   └── mcp/    # MCP (Model Context Protocol) tool integration
│   └── hyper/      # Hyper provider integration
├── app/            # Application wiring - coordinates all services
├── cmd/            # CLI commands (cobra-based)
├── config/         # Configuration loading, provider management
├── csync/          # Concurrent-safe data structures (Map, Slice, Value)
├── db/             # Database layer (sqlc-generated, SQLite)
│   ├── migrations/ # Goose migrations
│   └── sql/        # SQL queries for sqlc
├── format/         # Output formatting utilities
├── fsext/          # Filesystem extensions and utilities
├── history/        # File history tracking
├── log/            # Logging configuration
├── lsp/            # LSP client management (via powernap)
├── message/        # Message service for chat history
├── oauth/          # OAuth flows (Copilot, Hyper)
├── orchestra/      # Multi-agent orchestration
├── permission/     # Permission request handling
├── pubsub/         # Publish-subscribe event system
├── session/        # Session management
├── shell/          # Shell command execution
├── ui/             # Terminal UI (Bubble Tea)
└── version/        # Version information
```

### Key Components

- **Agent** (`internal/agent/`): Core orchestration layer for AI agents. Manages conversations, tool execution, message handling, and coordinates between LLMs, messages, sessions, and tools.

- **App** (`internal/app/`): Wires together all services and manages application lifecycle. Holds references to Sessions, Messages, History, Permissions, FileTracker, AgentCoordinator, and LSPManager.

- **Config** (`internal/config/`): Loads configuration from `crush.json` files, manages provider credentials, model selection, LSP settings, and MCP configurations.

- **PubSub** (`internal/pubsub/`): Generic publish-subscribe system used by Session, Message, and Permission services for real-time updates.

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

### VCR Testing

Tests that make API calls use VCR (Video Cassette Recorder) to record and replay HTTP interactions. Cassettes are stored in `internal/agent/testdata/`. To re-record:

```bash
task test:record  # Re-records all agent tests
```

## Database (sqlc + SQLite + Goose)

- **Migrations**: Located in `internal/db/migrations/`, managed by Goose
- **Queries**: SQL queries in `internal/db/sql/` are compiled to Go by sqlc
- **Generated code**: `internal/db/*.sql.go` files are auto-generated
- **Run migrations**: Automatically handled on app startup

To add a new query:
1. Add SQL to `internal/db/sql/*.sql`
2. Run `go generate ./internal/db/...` (or it auto-generates on build)

## LSP Integration

LSP clients are managed lazily via `internal/lsp/manager.go`. Configuration is merged from:
1. Default LSPs from powernap
2. User-configured LSPs in `crush.json`

LSPs provide diagnostics, references, and code intelligence. The manager handles:
- Lazy initialization per file type
- Client lifecycle management
- Callback registration for new clients

## MCP (Model Context Protocol)

MCP tools extend Crush's capabilities. Located in `internal/agent/tools/mcp/`:
- Supports `http`, `stdio`, and `sse` transports
- Tools are discovered and registered dynamically
- Configuration in `crush.json` under `mcp` key

## Tool Implementation Pattern

Tools in `internal/agent/tools/` follow a consistent pattern:
- Each tool has a `.go` file and a `.md` file (documentation for the LLM)
- Tools implement `fantasy.AgentTool` interface
- Context carries session/message IDs via `SessionIDContextKey` and `MessageIDContextKey`

Example tool structure:
```go
func NewMyTool(cfg *config.Config) fantasy.AgentTool {
    return fantasy.AgentTool{
        Name:        "my_tool",
       	Description: "Does something useful",
       	// ... rest of implementation
    }
}
```

## UI Development (Bubble Tea)

See `internal/ui/AGENTS.md` for detailed TUI development guidelines.

Key points:
- Never do I/O or expensive work in `Update`; always use a `tea.Cmd`
- Components should be "dumb" - expose methods, don't handle messages directly
- Use `github.com/charmbracelet/x/ansi` for ANSI string manipulation
- Styles are centralized in `internal/ui/styles/styles.go`

## Formatting

- ALWAYS format any Go code you write.
  - First, try `gofumpt -w .`.
  - If `gofumpt` is not available, use `goimports`.
  - If `goimports` is not available, use `gofmt`.
  - You can also use `task fmt` to run `gofumpt -w .` on the entire project,
    as long as `gofumpt` is on the `PATH`.

## Comments

- Comments that live on their own lines should start with capital letters and
  end with periods. Wrap comments at 78 columns.

## Committing

- ALWAYS use semantic commits (`fix:`, `feat:`, `chore:`, `refactor:`, `docs:`, `sec:`, etc).
- Try to keep commits to one line, not including your attribution. Only use
  multi-line commits when additional context is truly necessary.

## Context Files

Crush automatically loads context from these files in the working directory:
- `.github/copilot-instructions.md`
- `.cursorrules`, `.cursor/rules/`
- `CLAUDE.md`, `CLAUDE.local.md`
- `GEMINI.md`, `gemini.md`
- `crush.md`, `crush.local.md`, `Crush.md`, `Crush.local.md`, `CRUSH.md`, `CRUSH.local.md`
- `AGENTS.md`, `agents.md`, `Agents.md`

## Concurrent-Safe Data Structures

The `internal/csync/` package provides thread-safe collections:
- `Map[K, V]`: Concurrent map with RWMutex
- `Slice[T]`: Concurrent slice with mutex
- `Value[T]`: Atomic value holder with notifications
- `VersionedMap[K, V]`: Map with version tracking

Use these instead of standard Go maps/slices when access is shared across goroutines.

## CI/CD

GitHub Actions workflows:
- `build.yml`: Builds and tests on Ubuntu, macOS, Windows
- `lint.yml`: Runs golangci-lint
- `release.yml`: Creates releases via goreleaser
- `snapshot.yml`: Creates snapshot builds
- `security.yml`: Security scanning

## Working on the TUI (UI)

Anytime you need to work on the TUI, before starting work read the `internal/ui/AGENTS.md` file.

## Working on Stats Page

The stats page has HTML/CSS/JS files that need prettier formatting:
```bash
task fmt:html
```

## Profiling

When running with `task dev` or `CRUSH_PROFILE=true`:
- CPU profile: `task profile:cpu`
- Heap profile: `task profile:heap`
- Allocations: `task profile:allocs`
