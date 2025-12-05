# Crush Development Guide

## Build/Test/Lint Commands

- **Build**: `go build .` or `go run .`
- **Test**: `task test` or `go test ./...` (run single test: `go test ./internal/llm/prompt -run TestGetContextFromPaths`)
- **Update Golden Files**: `go test ./... -update` (regenerates .golden files when test output changes)
  - Update specific package: `go test ./internal/tui/components/core -update` (in this case, we're updating "core")
- **Lint**: `task lint:fix`
- **Format**: `task fmt` (gofumpt -w .)
- **Dev**: `task dev` (runs with profiling enabled)

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
- **Comments**: End comments in periods unless comments are at the end of the line.

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

## Comments

- Comments that live on their own lines should start with capital letters and
  end with periods. Wrap comments at 78 columns.

## Committing

- ALWAYS use semantic commits (`fix:`, `feat:`, `chore:`, `refactor:`, `docs:`, `sec:`, etc).
- Try to keep commits to one line, not including your attribution. Only use
  multi-line commits when additional context is truly necessary.

## Project Tracking

Crush maintains a centralized list of projects at `~/.local/share/crush/projects.json`.
This enables external tools (like [Splitrail](https://github.com/Piebald-AI/splitrail)) to analyze Crush usage across projects
without scanning the entire filesystem for `.crush` folders.

### How It Works

- Projects are registered automatically when Crush starts in a directory.
- Each entry tracks the working directory, data directory path, and last accessed time.
- The `data_dir` field points to the actual `.crush` folder location (handles custom
  `-D` paths and inherited `.crush` folders from parent directories).

### Commands

```bash
# List all tracked projects (table view in TTY, plain text otherwise)
crush projects

# Output as JSON (for external tools like Splitrail)
crush projects --json
```

### JSON Schema

```json
{
  "projects": [
    {
      "path": "/home/user/myproject",
      "data_dir": "/home/user/myproject/.crush",
      "last_accessed": "2025-12-04T14:30:00Z"
    }
  ]
}
```

### Key Functions

- `projects.Register(workingDir, dataDir)` - Add or update a project in the list.
- `projects.List()` - Get all tracked projects sorted by last accessed.
- `projects.Load()` / `projects.Save()` - Low-level file operations.
