# Crush Development Guide

## Build/Test/Lint Commands

- **Build**: `go build ./...` or `go build .` (for main binary)
- **Test**: `task test` or `go test ./...`
- **Single test**: `go test ./internal/path/to/package -run TestName`
- **Lint**: `task lint` or `golangci-lint run`
- **Format**: `task fmt` or `gofumpt -w .`

## Code Style Guidelines

- **Imports**: Standard library first, then third-party, then internal packages (separated by blank lines)
- **Types**: Use `any` instead of `interface{}`, prefer concrete types over interfaces when possible
- **Naming**: Use camelCase for private, PascalCase for public, descriptive names (e.g., `messageListCmp`, `handleNewUserMessage`)
- **Constants**: Use `const` blocks with descriptive names (e.g., `NotFound = -1`)
- **Error handling**: Always check errors, use `require.NoError()` in tests, return errors up the stack
- **Documentation**: Add comments for all public types/methods, explain complex logic in private methods
- **Testing**: Use testify/assert and testify/require, table-driven tests with `t.Run()`, mark helpers with `t.Helper()`
- **File organization**: Group related functionality, extract helper methods for complex logic, use meaningful method names
- **TUI components**: Implement interfaces (util.Model, layout.Sizeable), document component purpose and behavior
- **Message handling**: Use pubsub events, handle different message roles (User/Assistant/Tool), manage tool calls separately
