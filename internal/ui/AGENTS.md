# UI Development Instructions

## General Guidelines
- Never use commands to send messages when you can directly mutate children or state.
- Keep things simple; do not overcomplicate.
- Create files if needed to separate logic; do not nest models.

## Architecture

### Main Model (`model/ui.go`)
Keep most of the logic and state in the main model. This is where:
- Message routing happens
- Focus and UI state is managed
- Layout calculations are performed
- Dialogs are orchestrated

### Components Should Be Dumb
Components should not handle bubbletea messages directly. Instead:
- Expose methods for state changes
- Return `tea.Cmd` from methods when side effects are needed
- Handle their own rendering via `Render(width int) string`

### Chat Logic (`model/chat.go`)
Most chat-related logic belongs here. Individual chat items in `chat/` should be simple renderers that cache their output and invalidate when data changes (see `cachedMessageItem` in `chat/messages.go`).

## Key Patterns

### Composition Over Inheritance
Use struct embedding for shared behaviors. See `chat/messages.go` for examples of reusable embedded structs for highlighting, caching, and focus.

### Interfaces
- List item interfaces are in `list/item.go`
- Chat message interfaces are in `chat/messages.go`
- Dialog interface is in `dialog/dialog.go`

### Styling
- All styles are defined in `styles/styles.go`
- Access styles via `*common.Common` passed to components
- Use semantic color fields rather than hardcoded colors

### Dialogs
- Implement the dialog interface in `dialog/dialog.go`
- Return message types from `Update()` to signal actions to the main model
- Use the overlay system for managing dialog lifecycle

## File Organization
- `model/` - Main UI model and major components (chat, sidebar, etc.)
- `chat/` - Chat message item types and renderers
- `dialog/` - Dialog implementations
- `list/` - Generic list component with lazy rendering
- `common/` - Shared utilities and the Common struct
- `styles/` - All style definitions
- `anim/` - Animation system
- `logo/` - Logo rendering

## Common Gotchas
- Always account for padding/borders in width calculations
- Use `tea.Batch()` when returning multiple commands
- Pass `*common.Common` to components that need styles or app access
