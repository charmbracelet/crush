# UI Development Instructions

## General Guidelines

- Never use commands to send messages when you can directly mutate children
  or state.
- Keep things simple; do not overcomplicate.
- Create files if needed to separate logic; do not nest models.
- Never do IO or expensive work in `Update`; always use a `tea.Cmd`.
- Never change the model state inside of a command. Use messages and update
  the state in the main `Update` loop.
- Use the `github.com/charmbracelet/x/ansi` package for any string
  manipulation that might involve ANSI codes. Do not manipulate ANSI strings
  at byte level! Some useful functions:
  - `ansi.Cut`
  - `ansi.StringWidth`
  - `ansi.Strip`
  - `ansi.Truncate`

## Architecture

### Rendering Pipeline

The UI uses a **hybrid rendering** approach:

1. **Screen-based (Ultraviolet)**: The top-level `UI` model creates a
   `uv.ScreenBuffer`, and components draw into sub-regions using
   `uv.NewStyledString(str).Draw(scr, rect)`. Layout is rectangle-based via
   a `uiLayout` struct with fields like `layout.header`, `layout.main`,
   `layout.editor`, `layout.sidebar`, `layout.pills`, `layout.status`.
2. **String-based**: Sub-components like `list.List` and `completions` render
   to strings, which are painted onto the screen buffer.
3. **`View()`** creates the screen buffer, calls `Draw()`, then
   `canvas.Render()` flattens it to a string for Bubble Tea.

### Main Model (`model/ui.go`)

The `UI` struct is the top-level Bubble Tea model. Key fields:

- `width`, `height` — terminal dimensions
- `layout uiLayout` — computed layout rectangles
- `state uiState` — `uiOnboarding | uiInitialize | uiLanding | uiChat`
- `focus uiFocusState` — `uiFocusNone | uiFocusEditor | uiFocusMain`
- `chat *Chat` — wraps `list.List` for the message view
- `textarea textarea.Model` — the input editor
- `dialog *dialog.Overlay` — stacked dialog system
- `completions`, `attachments` — sub-components

Keep most logic and state here. This is where:

- Message routing happens (giant `switch msg.(type)` in `Update`)
- Focus and UI state is managed
- Layout calculations are performed
- Dialogs are orchestrated

### Centralized Message Handling

The `UI` model is the **sole Bubble Tea model**. Sub-components (`Chat`,
`List`, `Attachments`, `Completions`, etc.) do not participate in the
standard Elm architecture message loop. They are stateful structs with
imperative methods that the main model calls directly:

- **`Chat`** and **`List`** have no `Update` method at all. The main model
  calls targeted methods like `HandleMouseDown()`, `ScrollBy()`,
  `SetMessages()`, `Animate()`.
- **`Attachments`** and **`Completions`** have non-standard `Update`
  signatures (e.g., returning `bool` for "consumed") that act as guards, not
  as full Bubble Tea models.
- **Sidebar** is not its own model: it's a `drawSidebar()` method on `UI`.

When writing new components, follow this pattern:

- Expose imperative methods for state changes (not `Update(tea.Msg)`).
- Return `tea.Cmd` from methods when side effects are needed.
- Handle rendering via `Render(width int) string` or
  `Draw(scr uv.Screen, area uv.Rectangle)`.
- Let the main `UI.Update()` decide when and how to call into the component.

### Chat View (`model/chat.go`)

The `Chat` struct wraps a `list.List` with an ID-to-index map, mouse
tracking (drag, double/triple click), animation management, and a `follow`
flag for auto-scroll. It bridges screen-based and string-based rendering:

```go
func (m *Chat) Draw(scr uv.Screen, area uv.Rectangle) {
    uv.NewStyledString(m.list.Render()).Draw(scr, area)
}
```

Individual chat items in `chat/` should be simple renderers that cache their
output and invalidate when data changes (see `cachedMessageItem` in
`chat/messages.go`).

## Key Patterns

### Composition Over Inheritance

Use struct embedding for shared behaviors. See `chat/messages.go` for
examples of reusable embedded structs for highlighting, caching, and focus.

### Interface Hierarchy

The chat message system uses layered interface composition:

- **`list.Item`** — base: `Render(width int) string`
- **`MessageItem`** — extends `list.Item` + `list.RawRenderable` +
  `Identifiable`
- **`ToolMessageItem`** — extends `MessageItem` with tool call/result/status
  methods
- **Opt-in capabilities**: `Focusable`, `Highlightable`, `Expandable`,
  `Animatable`, `Compactable`, `KeyEventHandler`

Key interface locations:

- List item interfaces: `list/item.go`
- Chat message interfaces: `chat/messages.go`
- Tool message interfaces: `chat/tools.go`
- Dialog interface: `dialog/dialog.go`

### Tool Renderers

Each tool has a dedicated renderer in `chat/`. The `ToolRenderer` interface
requires:

```go
RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string
```

`NewToolMessageItem` in `chat/tools.go` is the central factory that routes
tool names to specific types:

| File                  | Tools rendered                                 |
| --------------------- | ---------------------------------------------- |
| `chat/bash.go`        | Bash, JobOutput, JobKill                       |
| `chat/file.go`        | View, Write, Edit, MultiEdit, Download         |
| `chat/search.go`      | Glob, Grep, LS, Sourcegraph                    |
| `chat/fetch.go`       | Fetch, WebFetch, WebSearch                     |
| `chat/agent.go`       | Agent, AgenticFetch                            |
| `chat/diagnostics.go` | Diagnostics                                    |
| `chat/references.go`  | References                                     |
| `chat/lsp_restart.go` | LSPRestart                                     |
| `chat/todos.go`       | Todos                                          |
| `chat/mcp.go`         | MCP tools (`mcp_` prefix)                      |
| `chat/generic.go`     | Fallback for unrecognized tools                |
| `chat/assistant.go`   | Assistant messages (thinking, content, errors) |
| `chat/user.go`        | User messages (input + attachments)            |

### Styling

- All styles are defined in `styles/styles.go` (massive `Styles` struct with
  nested groups for Header, Pills, Dialog, Help, etc.).
- Access styles via `*common.Common` passed to components.
- Use semantic color fields rather than hardcoded colors.

### Dialogs

- Implement the `Dialog` interface in `dialog/dialog.go`:
  `ID()`, `HandleMsg()` returning an `Action`, `Draw()` onto `uv.Screen`.
- `Overlay` manages a stack of dialogs with push/pop/contains operations.
- Dialogs draw last and overlay everything else.
- Use `RenderContext` from `dialog/common.go` for consistent layout (title
  gradients, width, gap, cursor offset helpers).

### Shared Context

The `common.Common` struct holds `*app.App` and `*styles.Styles`. Thread it
through all components that need access to app state or styles.

## File Organization

- `model/` — Main UI model and major sub-models (chat, sidebar, header,
  status, pills, session, onboarding, keys, etc.)
- `chat/` — Chat message item types and tool renderers
- `dialog/` — Dialog implementations (models, sessions, commands,
  permissions, API key, OAuth, filepicker, reasoning, quit)
- `list/` — Generic lazy-rendered scrollable list with viewport tracking
- `common/` — Shared `Common` struct, layout helpers, markdown rendering,
  diff rendering, scrollbar
- `completions/` — Autocomplete popup with filterable list
- `attachments/` — File attachment management
- `styles/` — All style definitions, color tokens, icons
- `diffview/` — Unified and split diff rendering with syntax highlighting
- `anim/` — Animated spinnner
- `image/` — Terminal image rendering (Kitty graphics)
- `logo/` — Logo rendering
- `util/` — Small shared utilities and message types

## Common Gotchas

- Always account for padding/borders in width calculations.
- Use `tea.Batch()` when returning multiple commands.
- Pass `*common.Common` to components that need styles or app access.
- When writing tea.Cmd's prefer creating methods in the model instead of writing inline functions.
- The `list.List` only renders visible items (lazy). No render cache exists
  at the list level — items should cache internally if rendering is
  expensive.
- Dialog messages are intercepted first in `Update` before other routing.
- Focus state determines key event routing: `uiFocusEditor` sends keys to
  the textarea, `uiFocusMain` sends them to the chat list.
