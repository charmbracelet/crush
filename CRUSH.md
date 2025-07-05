# Crush Development Guide

## Build/Test/Lint Commands

- **Build**: `go build .` or `go run .`
- **Test**: `task test` or `go test ./...` (run single test: `go test ./internal/llm/prompt -run TestGetContextFromPaths`)
- **Lint**: `task lint` (golangci-lint run) or `task lint-fix` (with --fix)
- **Format**: `task fmt` (gofumpt -w .)
- **Dev**: `task dev` (runs with profiling enabled)

## Available Tools

### VS Code Diff Tool

The `vscode_diff` tool opens VS Code with a diff view to compare two pieces of content. **VS Code diff is automatically enabled when running inside VS Code** (when `VSCODE_INJECTION=1` environment variable is set).

**Default Behavior:**
- **Automatic for file modifications**: Opens VS Code diff when using `write` or `edit` tools (only when inside VS Code)
- **Default for explicit diff requests**: When you ask to "show a diff" or "compare files", VS Code is preferred over terminal output (only when inside VS Code)
- **Smart fallback**: Uses terminal diff when not running inside VS Code or if VS Code is not available
- Only opens if there are actual changes (additions or removals)
- Requires user permission (requested once per session)

**Configuration:**
```json
{
  "options": {
    "auto_open_vscode_diff": true  // Default: true when VSCODE_INJECTION=1, false otherwise
  }
}
```

**Manual Usage Example:**
```json
{
  "left_content": "function hello() {\n  console.log('Hello');\n}",
  "right_content": "function hello() {\n  console.log('Hello World!');\n}",
  "left_title": "before.js",
  "right_title": "after.js", 
  "language": "javascript"
}
```

**Requirements:**
- VS Code must be installed
- The `code` command must be available in PATH
- Must be running inside VS Code (VSCODE_INJECTION=1 environment variable)
- User permission will be requested before opening VS Code

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
- **Testing**: Use testify/assert and testify/require, parallel tests with `t.Parallel()`
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
  - First, try `goftumpt -w .`.
  - If `gofumpt` is not available, use `goimports`.
  - If `goimports` is not available, use `gofmt`.
  - You can also use `task fmt` to run `gofumpt -w .` on the entire project,
    as long as `gofumpt` is on the `PATH`.
