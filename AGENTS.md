# Crush Agent Guide

Crush is an AI-powered coding assistant CLI built in Go. It provides a terminal
UI for interacting with language models (Anthropic, OpenAI, OpenRouter, etc.)
with tool capabilities for file manipulation, code search, and shell execution.

## Essential Commands

| Command | Description |
|---------|-------------|
| `go build .` | Build the binary |
| `go run .` | Run without building |
| `task test` or `go test ./...` | Run all tests |
| `go test ./internal/pkg -run TestName` | Run specific test |
| `go test ./... -update` | Update golden files |
| `task lint:fix` | Run linters with auto-fix |
| `task fmt` | Format code with gofumpt |
| `task dev` | Run with profiling enabled |
| `task install` | Install to GOPATH |
| `task build` | Build with version info |

### Single Test Examples

```bash
go test ./internal/tui/exp/list -run TestList
go test ./internal/agent -run TestCoderAgent/anthropic-sonnet/simple_test
```

### Golden File Updates

When test output changes intentionally, regenerate `.golden` files:

```bash
go test ./internal/tui/components/core -update
go test ./internal/tui/exp/diffview -update
```

## Project Structure

```
crush/
├── main.go                    # Entry point
├── internal/
│   ├── cmd/                   # CLI commands (cobra)
│   ├── app/                   # Application wiring and lifecycle
│   ├── agent/                 # AI agent orchestration
│   │   ├── tools/             # Tool implementations (bash, edit, view, etc.)
│   │   ├── templates/         # System prompt templates
│   │   └── hyper/             # Hyper provider integration
│   ├── config/                # Configuration loading and providers
│   ├── tui/                   # Terminal UI (bubbletea)
│   │   ├── components/        # Reusable UI components
│   │   ├── page/              # Full-page views
│   │   ├── exp/               # Experimental components
│   │   └── styles/            # Theme and styling
│   ├── db/                    # SQLite database (sqlc-generated)
│   │   ├── migrations/        # Goose migrations
│   │   └── sql/               # SQL query definitions
│   ├── message/               # Message handling
│   ├── session/               # Session management
│   ├── permission/            # Permission system
│   ├── lsp/                   # Language Server Protocol client
│   ├── shell/                 # Shell execution (mvdan/sh)
│   └── ...                    # Other utility packages
├── Taskfile.yaml              # Task runner commands
├── sqlc.yaml                  # SQL code generation config
└── .golangci.yml              # Linter configuration
```

## Code Style

### Formatting

- **Always** run `gofumpt -w .` or `task fmt` before committing.
- Fallback: `goimports` → `gofmt` if gofumpt unavailable.
- gofumpt is stricter than gofmt and enforced by CI.

### Naming Conventions

- **Exported**: PascalCase (`SessionAgent`, `NewEditTool`)
- **Unexported**: camelCase (`sessionAgent`, `workingDir`)
- Use type aliases for clarity: `type AgentName string`
- JSON tags use snake_case: `json:"file_path"`

### Import Grouping

Group imports in this order with blank lines between:

1. Standard library
2. External packages (charm.land/, github.com/)
3. Internal packages (github.com/charmbracelet/crush/internal/)

### Comments

- Comments on their own line: start with capital, end with period.
- Inline comments: no period required.
- Wrap at 78 columns.
- Package comments are required for all packages.

### Error Handling

- Return errors explicitly, never panic for recoverable errors.
- Use `fmt.Errorf("context: %w", err)` for wrapping.
- Context should be passed as first parameter.

### File Permissions

Use octal notation: `0o755`, `0o644` (not `0755`).

## Testing

### Test Patterns

- Use `testify/require` for assertions (not `assert` for critical checks).
- Enable parallel tests: `t.Parallel()` at start of test functions.
- Use `t.SetEnv()` for environment variables.
- Use `t.TempDir()` for temporary directories (auto-cleaned).
- Use `t.Context()` for context in tests.

### Golden File Testing

Many UI tests use golden files for output comparison:

```go
import "github.com/charmbracelet/x/exp/golden"

func TestComponent(t *testing.T) {
    t.Parallel()
    // ... setup
    golden.RequireEqual(t, []byte(component.View()))
}
```

Update with `-update` flag when output changes intentionally.

### VCR Recording for API Tests

Agent tests use VCR cassettes to record/replay HTTP interactions:

```go
import "charm.land/x/vcr"

func TestAgent(t *testing.T) {
    r := vcr.NewRecorder(t)
    // Use r.GetDefaultClient() for HTTP clients
}
```

Re-record cassettes: `task test:record`

### Mock Providers

For config tests involving providers:

```go
func TestWithMockProviders(t *testing.T) {
    originalUseMock := config.UseMockProviders
    config.UseMockProviders = true
    defer func() {
        config.UseMockProviders = originalUseMock
        config.ResetProviders()
    }()
    config.ResetProviders()
    // Test with mock providers
}
```

## Key Architectural Patterns

### Service Pattern

Services wrap database queries and provide business logic:

```go
type Service interface {
    Create(ctx context.Context, name string) (*Model, error)
    Get(ctx context.Context, id string) (*Model, error)
    List(ctx context.Context) ([]Model, error)
}
```

### Tool Implementation

Tools follow the Fantasy framework pattern:

```go
//go:embed tool.md
var toolDescription []byte

type ToolParams struct {
    FilePath string `json:"file_path" description:"Path to the file"`
}

func NewTool(...) fantasy.AgentTool {
    return fantasy.NewAgentTool(
        "tool_name",
        string(toolDescription),
        func(ctx context.Context, params ToolParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
            // Implementation
        },
    )
}
```

### Bubbletea Components

UI components follow the Elm architecture:

```go
type Model struct { /* state */ }

func (m Model) Init() tea.Cmd { /* initial commands */ }
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { /* handle messages */ }
func (m Model) View() string { /* render output */ }
```

### Concurrent Maps

Use `csync.Map` for thread-safe maps:

```go
import "github.com/charmbracelet/crush/internal/csync"

clients := csync.NewMap[string, *Client]()
clients.Set(key, value)
value, ok := clients.Get(key)
```

## Database

### SQLc Code Generation

SQL queries in `internal/db/sql/` generate Go code via sqlc:

```sql
-- name: GetSession :one
SELECT * FROM sessions WHERE id = ? LIMIT 1;
```

After modifying SQL files, regenerate with: `sqlc generate`

### Migrations

Goose migrations in `internal/db/migrations/`:

- Named: `YYYYMMDDHHMMSS_description.sql`
- Applied automatically on startup.

## Configuration

Config files are loaded from (in order):

1. `$HOME/.config/crush/crush.json`
2. Project-level `crush.json`
3. Environment variables (`CRUSH_*`)

Context files read automatically:
- `CRUSH.md`, `AGENTS.md`, `CLAUDE.md`
- `.cursorrules`, `.cursor/rules/`
- `.github/copilot-instructions.md`

## Commits

- **Always** use semantic commits: `fix:`, `feat:`, `chore:`, `refactor:`,
  `docs:`, `sec:`
- Keep to one line unless additional context is truly necessary.

## CI/CD

- **Lint**: golangci-lint v2 with config in `.golangci.yml`
- **Build**: Cross-platform via goreleaser
- **Tests**: Run on push/PR via GitHub Actions

## Common Gotchas

1. **CGO disabled**: `CGO_ENABLED=0` is set in Taskfile. Some packages may
   behave differently.

2. **Go experiment**: `GOEXPERIMENT=greenteagc` is enabled for the green tea
   GC.

3. **Windows tests skipped**: Some agent tests skip on Windows due to path
   differences.

4. **Shell interpreter**: Uses `mvdan/sh` (not system shell) for cross-platform
   compatibility.

5. **Generated code**: Don't edit files marked `// Code generated`. Edit source
   files instead:
   - `internal/db/*.sql.go` → edit `internal/db/sql/*.sql`
   - `internal/agent/hyper/provider.json` → run `task hyper`

6. **Test timeouts**: Agent tests with VCR recording can take a long time. Use
   `go test -timeout=1h` for recording.

## Dependencies

Key external packages:

- `charm.land/bubbletea/v2` - Terminal UI framework
- `charm.land/fantasy` - AI agent framework
- `charm.land/lipgloss/v2` - Terminal styling
- `github.com/spf13/cobra` - CLI framework
- `github.com/ncruces/go-sqlite3` - SQLite driver
- `mvdan.cc/sh/v3` - Shell interpreter
- `github.com/stretchr/testify` - Testing assertions
