# Crush Public API (`pkg/crush`)

> **Experimental — no stability guarantees**
>
> `pkg/crush` is an unstable, experimental API. Types, functions, and method
> signatures may change or be removed in any release without notice. There is
> no semantic-versioning or backward-compatibility promise. Pin to a specific
> Crush version and plan for breakage when upgrading.

This package exposes a public Go API for [Crush](https://github.com/charmbracelet/crush) — a terminal-based AI coding assistant by [Charm](https://charm.sh). Use it to configure, initialize, and interact with Crush from external Go code.

Do not treat this API as production-stable. It exists for early integration and feedback while the surface matures.

## Usage

```go
import "github.com/charmbracelet/crush/pkg/crush"
```

## Overview

The public API is organized around these key areas:

| Area | Types | Description |
|------|-------|-------------|
| **Configuration** | `Config`, `ConfigStore`, `SelectedModel`, `ProviderConfig`, `MCPConfig`, `LSPConfig`, `Agent`, `Options` | Crush configuration and settings |
| **Sessions** | `Session`, `Todo`, `SessionService` | Conversation session management |
| **Messages** | `Message`, `ContentPart`, `TextContent`, `ToolCall`, `ToolResult`, `MessageService` | Message creation and streaming |
| **Permissions** | `PermissionRequest`, `PermissionService` | Tool execution permission management |
| **History** | `File`, `HistoryService` | Versioned file history |
| **Events** | `Event`, `EventType`, `Subscriber`, `Publisher` | Pub/sub event system |
| **App** | `App`, `NewApp()` | Top-level application orchestrator |

## Quick Start

### One-Shot Execution

The simplest way to run a single prompt and exit:

```go
ctx := context.Background()

// Initialize the application (loads config, opens DB, sets up agents)
app, err := crush.NewAppWithConfig(ctx, ".", ".crush", false, nil)
if err != nil {
    log.Fatalf("failed to initialize app: %v", err)
}
defer app.Shutdown()

// Run a single prompt with sensible defaults
err = crush.RunPrompt(app, ctx, "Write a hello world program in Go")
if err != nil {
    log.Fatal(err)
}
```

See [`example/oneshot`](./example/oneshot/main.go) for a complete runnable example.

### Server-Style Execution

For long-running processes that need session management and event streaming:

```go
ctx := context.Background()

app, err := crush.NewAppWithConfig(ctx, ".", ".crush", false, nil)
if err != nil {
    log.Fatalf("failed to initialize app: %v", err)
}
defer app.Shutdown()

// 1. Create a session
sess, err := app.Sessions.Create(ctx, "My coding session")
if err != nil {
    log.Fatal(err)
}

// 2. Subscribe to live message events BEFORE running the prompt
eventsCh := crush.SubscribeSessionMessages(ctx, app, sess.ID)

// 3. Consume events in a background goroutine
go func() {
    for ev := range eventsCh {
        switch ev.Type {
        case crush.EventCreated:
            fmt.Printf("[CREATED] %s role=%s\n", ev.Payload.ID, ev.Payload.Role)
        case crush.EventUpdated:
            msg := ev.Payload
            if rc := msg.ReasoningContent(); rc.Thinking != "" {
                fmt.Printf("[THINKING] %s\n", rc.Thinking)
            }
            if txt := msg.Content(); txt.Text != "" {
                fmt.Printf("[TEXT] %s\n", txt.Text)
            }
            for _, tc := range msg.ToolCalls() {
                fmt.Printf("[TOOL_CALL] %s %s\n", tc.Name, tc.ID)
            }
            for _, tr := range msg.ToolResults() {
                fmt.Printf("[TOOL_RESULT] %s\n", tr.ToolCallID)
            }
            if msg.IsFinished() {
                fmt.Printf("[FINISHED] reason=%s\n", msg.FinishReason())
            }
        }
    }
}()

// 4. Run the prompt in the existing session
var out bytes.Buffer
err = crush.RunPromptInSession(app, ctx, &out, sess.ID, "Write a hello world program in Go")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Final response:\n%s\n", out.String())
```

See [`example/server`](./example/server/main.go) for a complete runnable example.

### Loading Configuration

```go
import (
    "context"
    "database/sql"

    "github.com/charmbracelet/crush/pkg/crush"
)

// Load configuration from default paths (crush.json, etc.)
store, err := crush.Load("/path/to/project", "", false)
if err != nil {
    // handle error
}

cfg := store.Config()
```

### Defining Configuration Programmatically

Instead of loading `crush.json` from disk, you can build the configuration entirely in code and wrap it in a `ConfigStore` for use with `NewApp`:

```go
import (
    "os"

    "github.com/charmbracelet/crush/pkg/crush"
    "charm.land/catwalk/pkg/catwalk"
)

// Build the pure-data configuration
cfg := &crush.Config{
    Models: map[crush.SelectedModelType]crush.SelectedModel{
        crush.SelectedModelTypeLarge: {
            Provider: "openai",
            Model:    "gpt-5",
        },
        crush.SelectedModelTypeSmall: {
            Provider: "openai",
            Model:    "gpt-5-mini",
        },
    },
    Providers: crush.NewMapFrom(map[string]crush.ProviderConfig{
        "openai": {
            ID:     "openai",
            Name:   "OpenAI",
            Type:   catwalk.TypeOpenAI,
            APIKey: os.Getenv("OPENAI_API_KEY"),
            Models: []catwalk.Model{
                {ID: "gpt-5", Name: "GPT-5"},
                {ID: "gpt-5-mini", Name: "GPT-5 Mini"},
            },
        },
    }),
    MCP: map[string]crush.MCPConfig{
        "filesystem": {
            Command: "npx",
            Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
            Type:    crush.MCPStdio,
        },
    },
    Options: &crush.Options{
        DataDirectory: ".crush",
        Debug:         true,
    },
}

// Derive agents from the configuration
cfg.SetupAgents()

// Wrap in a store so the app can consume it
store := crush.NewTestStore(cfg)

// Now pass it to NewApp exactly as if it had been loaded from file
app, err := crush.NewApp(ctx, conn, store, nil)
```

`NewTestStore` is the helper exported for this use-case. It takes a `*Config` (and optional loaded-path hints) and returns a `*ConfigStore` that satisfies everything `App` needs without ever touching the filesystem.

```go
// Open the SQLite database
conn, err := sql.Open("sqlite", "path/to/db")
if err != nil {
    // handle error
}
defer conn.Close()

// Create the application instance
app, err := crush.NewApp(ctx, conn, store, nil)
if err != nil {
    // handle error
}
defer app.Shutdown()
```

## Convenience Functions

### `NewAppWithConfig`

Loads configuration from default paths, opens the SQLite database (with migrations), and creates a fully initialized `App`:

```go
func NewAppWithConfig(ctx context.Context, workingDir, dataDir string, debug bool, skillsMgr *SkillsManager) (*App, error)
```

- `workingDir`: project working directory (config lookup starts here)
- `dataDir`: directory for SQLite DB and other runtime data (e.g. `".crush"`)
- `debug`: enable debug logging
- `skillsMgr`: optional skills manager (pass `nil` for default behavior)

### `RunPrompt`

Run a single prompt with sensible defaults — output to `os.Stdout`, models from config, spinner enabled, new session:

```go
func RunPrompt(app *App, ctx context.Context, prompt string) error
```

### `RunPromptAndCreateSession`

Create a named session, run a prompt, and return the session ID for later retrieval:

```go
func RunPromptAndCreateSession(app *App, ctx context.Context, output io.Writer, title, prompt string) (string, error)
```

### `RunPromptInSession`

Run a prompt in an existing session:

```go
func RunPromptInSession(app *App, ctx context.Context, output io.Writer, sessionID, prompt string) error
```

### `SubscribeSessionMessages`

Subscribe to live message events for a specific session. Useful for streaming thinking text, tool calls, and results to a server client:

```go
func SubscribeSessionMessages(ctx context.Context, app *App, sessionID string) <-chan Event[Message]
```

Returns a channel that emits events with types `EventCreated`, `EventUpdated`, `EventDeleted`. Use the helpers on `Message` to extract content:

| What you want | Method |
|---------------|--------|
| Assistant text | `msg.Content().Text` |
| Thinking/reasoning text | `msg.ReasoningContent().Thinking` |
| Tool calls | `msg.ToolCalls()` |
| Tool results | `msg.ToolResults()` |
| Turn finished? | `msg.IsFinished()` |
| Finish reason | `msg.FinishReason()` |

## Working with Sessions

```go
// Create a new session
sess, err := app.Sessions.Create(ctx, "My coding session")

// List all sessions
sessions, err := app.Sessions.List(ctx)

// Get a session by ID
sess, err := app.Sessions.Get(ctx, sessionID)

// Delete a session
err = app.Sessions.Delete(ctx, sessionID)
```

## Working with Messages

```go
// Create a user message
msg, err := app.Messages.Create(ctx, sessionID, crush.CreateMessageParams{
    Role:  crush.RoleUser,
    Parts: []crush.ContentPart{crush.TextContent{Text: "Hello!"}},
})

// List messages in a session
messages, err := app.Messages.List(ctx, sessionID)
```

### Non-Interactive Agent Execution (Low-Level)

For full control over output, model overrides, session continuation, and spinner:

```go
err = app.RunNonInteractive(
    ctx,
    os.Stdout,
    "Write a hello world program in Go",
    "",    // large model override (empty = use config)
    "",    // small model override
    false, // hide spinner
    "",    // continue session ID
    false, // use last session
)
```

## Configuration Types

### SelectedModel

Represents a selected LLM model:

```go
type SelectedModel struct {
    Model           string
    Provider        string
    ReasoningEffort string
    Think           bool
    MaxTokens       int64
    Temperature     *float64
    TopP            *float64
    TopK            *int64
    // ... and more
}
```

### ProviderConfig

Configures an LLM provider:

```go
type ProviderConfig struct {
    ID       string
    Name     string
    BaseURL  string
    Type     string // openai, anthropic, gemini, etc.
    APIKey   string
    Models   []catwalk.Model
    // ... and more
}
```

### MCPConfig

Configures a Model Context Protocol server:

```go
type MCPConfig struct {
    Command string
    Args    []string
    Env     map[string]string
    Type    MCPType // stdio, sse, http
    URL     string
    // ... and more
}
```

## Session Types

### Session

A conversation session:

```go
type Session struct {
    ID               string
    ParentSessionID  string
    Title            string
    MessageCount     int64
    PromptTokens     int64
    CompletionTokens int64
    Cost             float64
    Todos            []Todo
    CreatedAt        int64
    UpdatedAt        int64
}
```

### Todo

A task item within a session:

```go
type Todo struct {
    Content    string
    Status     TodoStatus // pending, in_progress, completed
    ActiveForm string
}
```

## Message Types

### Message

A single message in a conversation:

```go
type Message struct {
    ID               string
    Role             MessageRole // assistant, user, system, tool
    SessionID        string
    Parts            []ContentPart
    Model            string
    Provider         string
    CreatedAt        int64
    UpdatedAt        int64
    IsSummaryMessage bool
}
```

### Content Parts

Messages are composed of content parts:

- **`TextContent`** — Plain text
- **`ReasoningContent`** — Model reasoning/thinking
- **`ImageURLContent`** — Image referenced by URL
- **`BinaryContent`** — Binary data (e.g. uploaded files)
- **`ToolCall`** — Tool invocation request
- **`ToolResult`** — Tool invocation result
- **`Finish`** — End-of-turn marker

## Event System

Crush uses an internal pub/sub system for decoupled communication. Each service (sessions, messages, permissions, etc.) has its own broker.

### Per-Session Message Events

For real-time streaming of a single session, use the convenience wrapper:

```go
eventsCh := crush.SubscribeSessionMessages(ctx, app, sessionID)
for ev := range eventsCh {
    msg := ev.Payload
    if rc := msg.ReasoningContent(); rc.Thinking != "" {
        fmt.Printf("Thinking: %s\n", rc.Thinking)
    }
    if txt := msg.Content(); txt.Text != "" {
        fmt.Printf("Text: %s\n", txt.Text)
    }
    for _, tc := range msg.ToolCalls() {
        fmt.Printf("Tool call: %s\n", tc.Name)
    }
}
```

### General Service Events

```go
// Subscribe to all session events
events := app.Sessions.Subscribe(ctx)
for event := range events {
    switch event.Type {
    case crush.EventCreated:
        // handle new session
    case crush.EventUpdated:
        // handle updated session
    case crush.EventDeleted:
        // handle deleted session
    }
}
```

## Experimental API

- **Unstable** — breaking changes can land in any release (including patch releases).
- **No guarantees** — exported symbols are not covered by semantic versioning or a compatibility policy.
- **Pin versions** — depend on an explicit Crush module version; do not assume `latest` stays compatible.
- **Internal code** — packages under `internal/` are not importable and may change without notice.

A formal stability policy may be adopted later. Until then, treat every upgrade as a potential breaking change and read the release notes before bumping your dependency.

## See Also

- [Crush GitHub Repository](https://github.com/charmbracelet/crush)
- [Crush Documentation](https://charm.sh/crush)
