package editor

// Package editor provides the chat input editor component with support for slash commands
// and file path completions.
//
// # Slash Commands
//
// The editor supports slash commands using the backslash (`\`) prefix. When a user types
// `\` at the beginning of a line or after whitespace, command completions appear.
//
// ## Command Syntax
//
// Commands use the syntax: `\command-name [arguments]`
//
// Examples:
//   - `\help` - Execute the help command
//   - `\frontend:review-pr 123 high` - Execute namespaced command with arguments
//   - `\frontend:components:button` - Execute nested namespaced command
//
// ## Command Completions
//
// When typing `\`, a completion popup appears showing all available commands:
//   - Commands are sorted alphabetically by name (including namespace)
//   - Fuzzy matching works across namespaces (e.g., `\cbut` matches `\frontend:components:button`)
//   - Commands are displayed with their description if available
//   - Empty query (`\`) shows all commands
//
// ## Completion Keybindings
//
// While completions are open:
//   - Up/Down arrows: Navigate through completions
//   - Enter/Tab: Select and insert command name
//   - Escape: Close completions (keeps `\` prefix in editor)
//   - Ctrl+N: Insert next command without closing
//   - Ctrl+P: Insert previous command without closing
//
// ## Command Execution
//
// Commands are executed when the user presses Enter with a command input:
//   - Command name and arguments are parsed from the input
//   - Command is looked up in the registry
//   - Arguments are validated against command requirements
//   - Command content is processed (argument substitution, file references)
//   - Command is executed through the agent coordinator
//
// ## File Path Completions
//
// The editor also supports file path completions using forward slash (`/`):
//   - Typing `/` shows file completions
//   - Forward slash (`/`) is for files, backslash (`\`) is for commands
//   - Both completion types work independently
//
// ## Integration Points
//
// The editor integrates with:
//   - Command registry (internal/commands) for loading and executing commands
//   - Completion system (internal/tui/components/completions) for UI
//   - Agent coordinator for command execution
//   - TUI framework (bubbletea) for user interaction
//
// ## Usage Example
//
//	// Editor automatically handles command detection
//	// User types: \help
//	// Editor detects `\` prefix, shows completions
//	// User selects "help" from completions
//	// Editor inserts: \help
//	// User presses Enter
//	// Editor calls executeCommand("help", nil)
//	// Command is executed through executor

