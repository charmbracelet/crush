// Package commands provides a registry system for loading and managing slash commands
// in the Crush editor. Commands are defined as Markdown files with YAML frontmatter
// and can be loaded from multiple locations (project, user home, XDG config).
//
// # Overview
//
// The commands package implements a registry pattern for managing slash commands
// similar to Claude Code's command system. Commands are Markdown files containing
// prompts/content that can be executed by the AI agent with optional tool restrictions.
//
// # Command Loading Locations
//
// Commands are loaded from three locations in order of precedence (highest to lowest):
//
//  1. Project directory: `.crush/commands/**/*.md`
//     - Project-specific commands that are version-controlled with the codebase
//     - Highest precedence - overwrites user/XDG commands with same name
//
//  2. User home directory: `~/.crush/commands/**/*.md`
//     - User-specific commands available across all projects
//     - Medium precedence
//
//  3. XDG config directory: `$XDG_CONFIG_HOME/crush/commands/**/*.md` or `~/.config/crush/commands/**/*.md`
//     - System-wide user commands following XDG Base Directory specification
//     - Lowest precedence
//
// Commands from all locations are merged into a single registry. If multiple commands
// have the same name, project commands take precedence over user commands, which take
// precedence over XDG commands.
//
// # Command File Format
//
// Commands are Markdown files with optional YAML frontmatter:
//
//	---
//	description: Brief description of the command
//	argument-hint: "[arg1] [arg2]"
//	allowed-tools:
//	  - view
//	  - edit
//	  - grep
//	---
//	# Command Content
//
//	This is the actual prompt/content sent to the AI agent when the command is executed.
//
// The frontmatter fields are:
//
//   - description: A brief description shown in command completions and help output
//   - argument-hint: Optional hint about expected arguments (e.g., "[pr-number] [priority]")
//   - allowed-tools: Optional list of Crush tool names that are allowed when executing this command.
//     If not specified, all tools are available. Valid tool names include: agent, bash, download,
//     edit, multiedit, lsp_diagnostics, lsp_references, fetch, glob, grep, ls, sourcegraph, view, write.
//
// # Namespacing Strategy
//
// Commands are automatically namespaced based on their directory structure. This prevents
// naming conflicts and allows organizing commands into logical groups.
//
// Examples:
//
//   - `.crush/commands/review-pr.md` → Command name: `review-pr` (no namespace)
//   - `.crush/commands/frontend/review-pr.md` → Command name: `frontend:review-pr`
//   - `.crush/commands/frontend/components/button.md` → Command name: `frontend:components:button`
//
// Directory separators (`/` or `\`) are converted to colons (`:`) in command names.
// Namespaces prevent conflicts: `frontend/review-pr.md` and `backend/review-pr.md` can coexist
// as `frontend:review-pr` and `backend:review-pr`.
//
// # Usage Examples
//
// Create a new registry and load commands:
//
//	registry := commands.NewRegistry("/path/to/project")
//	commands, err := registry.LoadCommands()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Find a specific command:
//
//	cmd, err := registry.FindCommand("frontend:review-pr")
//	if err != nil {
//	    log.Printf("Command not found: %v", err)
//	} else {
//	    fmt.Printf("Found command: %s\n", cmd.Description)
//	}
//
// List all available commands:
//
//	allCommands := registry.ListCommands()
//	for _, cmd := range allCommands {
//	    fmt.Printf("%s: %s\n", cmd.Name, cmd.Description)
//	}
//
// Reload commands after files are added/modified:
//
//	err := registry.Reload()
//	if err != nil {
//	    log.Printf("Failed to reload: %v", err)
//	}
//
// # Error Handling
//
// The package uses a "partial success" approach to error handling:
//
//   - Individual file parsing errors are logged but don't prevent other commands from loading
//   - Missing directories are handled gracefully (return empty slice, no error)
//   - Invalid YAML frontmatter is logged as a warning, and the file is treated as having no frontmatter
//   - Invalid tool names in `allowed-tools` are logged as warnings and filtered out
//   - Only critical errors (e.g., all loaders failing) cause LoadCommands() to return an error
//
// This ensures that the registry can continue operating even if some command files are malformed,
// missing, or have configuration issues.
//
// # Thread Safety
//
// The Registry interface is thread-safe. All methods use appropriate locking to ensure
// safe concurrent access. LoadCommands() and Reload() should be called from a single
// goroutine or synchronized externally to avoid race conditions during loading.
//
// # Command Execution
//
// Commands are executed through the Executor interface, which handles the complete
// execution flow from command lookup to agent invocation.
//
// ## Execution Flow
//
// When a command is executed, the executor performs the following steps:
//
//   1. Look up the command in the registry by name
//   2. Validate arguments match command requirements (based on placeholders in content)
//   3. Substitute arguments into command content ($ARGS, $ARGUMENTS, $1, $2, etc.)
//   4. Parse file references (@filename) from the processed content
//   5. Resolve file paths relative to the working directory
//   6. Read file contents and build attachments
//   7. Filter tools based on command's `allowed-tools` frontmatter
//   8. Invoke the agent coordinator with processed content and attachments
//
// ## Argument Substitution Syntax
//
// Commands support two types of argument placeholders:
//
//   - $ARGS or $ARGUMENTS: Replaced with all arguments joined by a single space
//     Example: Command "review $ARGS" with args ["123", "high"] → "review 123 high"
//     Example: Command "review $ARGUMENTS" with args ["123", "high"] → "review 123 high"
//
//   - $1, $2, $3, etc.: Positional arguments replaced with the corresponding argument
//     Example: Command "Review PR $1 with priority $2" with args ["123", "high"] → "Review PR 123 with priority high"
//
// Both placeholder types can be used together. When $ARGS or $ARGUMENTS is present, it's replaced
// first, then positional arguments are substituted.
//
// Missing arguments are replaced with empty strings. For example, "$3" with only 2 args
// becomes an empty string.
//
// ## File Reference Syntax
//
// Commands can reference files using the `@filename` syntax. File references are parsed
// from the command content and their contents are attached to the agent execution.
//
// Examples:
//
//   - `@file.txt` - References a file in the working directory
//   - `@src/main.go` - References a file in a subdirectory
//   - `@../parent/file.txt` - References a file in a parent directory
//
// File references are resolved relative to the executor's working directory. Absolute
// paths are preserved as-is. The file contents are read and attached to the agent
// execution with automatic MIME type detection.
//
// If a referenced file cannot be read (not found, permission denied, etc.), command
// execution fails with an error indicating which files could not be read.
//
// File references remain in the command content after processing - they are not removed
// from the prompt sent to the agent.
//
// ## Tool Filtering Behavior
//
// Commands can restrict which tools are available during execution using the
// `allowed-tools` frontmatter field. Tool filtering works as follows:
//
//   - If `allowed-tools` is empty or not specified: All available tools are allowed
//   - If `allowed-tools` contains tool names: Only those tools are allowed
//   - Invalid tool names are logged as warnings and filtered out
//   - Tool filtering is case-sensitive
//
// Note: Currently, tool restrictions are noted but full enforcement requires
// coordinator extension to support per-command agent configs. The executor passes
// the command to the coordinator with all tools available, but the structure is
// in place for future tool restriction enforcement.
//
// ## Executor Usage Examples
//
// Create an executor and execute a command:
//
//	registry := commands.NewRegistry("/path/to/project")
//	_, err := registry.LoadCommands()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	coordinator := agent.NewCoordinator(...) // Your agent coordinator
//	messages := message.NewService(...) // Your message service
//	executor := commands.NewExecutor(registry, coordinator, messages, "/path/to/project")
//
//	ctx := context.Background()
//	err = executor.Execute(ctx, "session-123", "frontend:review-pr", []string{"123", "high"})
//	if err != nil {
//	    log.Printf("Command execution failed: %v", err)
//	}
//
// Execute a command with file references:
//
//	// Command content: "Review @file1.txt and @file2.go"
//	err = executor.Execute(ctx, "session-123", "review-files", []string{})
//	// Files file1.txt and file2.go are automatically attached
//
// Execute a command with argument substitution:
//
//	// Command content: "Review PR $1 with priority $2. All args: $ARGS"
//	err = executor.Execute(ctx, "session-123", "review-pr", []string{"123", "high"})
//	// Prompt sent to agent: "Review PR 123 with priority high. All args: 123 high"
//
// # Integration with Crush
//
// The commands package integrates with Crush's existing systems:
//
//   - Uses Crush's logging infrastructure (log/slog) for error reporting
//   - Validates `allowed-tools` against Crush's available tools (internal/config/config.go)
//   - Commands can be executed through Crush's agent coordinator with tool restrictions
//   - File attachments use Crush's message.Attachment format
//
// # Help Command
//
// The `\help` command lists all available commands with their descriptions and usage hints.
//
// ## Usage
//
// Type `\help` in the chat editor and press Enter to see all available commands:
//
//	\help
//
// The help command displays commands grouped by namespace, making it easy to discover
// available commands and understand their organization.
//
// ## Help Output Format
//
// The help output is organized into sections:
//
//   - Root Commands: Commands without a namespace (e.g., `\help`, `\review-pr`)
//   - Namespace Commands: Commands organized by namespace (e.g., `Frontend Commands:`, `Backend Commands:`)
//
// Each command entry includes:
//
//   - Command name: The full command name including namespace (e.g., `\frontend:review-pr`)
//   - Description: Brief description from the command's frontmatter
//   - Argument hints: Optional hints about expected arguments (e.g., `[pr-number] [priority]`)
//   - Source indicator: Where the command was loaded from (e.g., `(project:frontend)`, `(user)`)
//
// Example help output:
//
//	Available Commands:
//
//	Root Commands:
//	  \help - Show help
//	  \review-pr [pr-number] (project)
//
//	Frontend Commands:
//	  \frontend:review-pr [pr-number] [priority] (project:frontend)
//	  \frontend:components:button (project:frontend)
//
//	Backend Commands:
//	  \backend:deploy [environment] (user)
//
// Commands within each section are sorted alphabetically for easy scanning.
//
// # Reload Commands
//
// Commands can be reloaded without restarting Crush using the "Reload Commands" option
// in the `Ctrl+P` command dialog.
//
// ## When to Use Reload
//
// Reload commands when:
//
//   - You've added new command files to `.crush/commands/`, `~/.crush/commands/`, or XDG config
//   - You've modified existing command files (changed frontmatter, content, etc.)
//   - You've removed command files and want them to disappear from completions
//   - You've moved commands between directories (changing namespaces)
//
// After reloading, the next time you:
//
//   - Open command completions (type `\`): New commands appear, removed commands disappear
//   - Execute `\help`: Updated command list is displayed
//   - Execute a command: Uses the latest version of the command file
//
// ## How to Reload
//
// 1. Press `Ctrl+P` to open the command dialog
// 2. Type "reload" or navigate to "Reload Commands"
// 3. Press Enter to execute
//
// A success message confirms that commands were reloaded. If reload fails, an error
// message explains what went wrong (errors are also logged to Crush's log system).
//
// ## Reload Behavior
//
// Reloading:
//
//   - Loads commands from all configured locations (project, user home, XDG config)
//   - Merges commands with proper precedence (project > user > XDG)
//   - Updates the command registry immediately
//   - Does not affect currently executing commands
//   - Does not require restarting Crush
//
// Commands are loaded fresh each time completions are opened, so reload ensures
// that newly added or modified commands are immediately available in the completion
// popup and help output.
//
// ## Reload Errors
//
// If reload encounters errors (e.g., invalid YAML, permission issues), the errors
// are logged but don't prevent other commands from loading. You'll see an error
// message in the status bar, and details are available in Crush's logs.
//
// Existing commands remain available even if reload fails, so you can continue
// using commands while fixing issues with new command files.
//
// # See Also
//
// Related packages:
//
//   - internal/agent: Agent coordinator for executing commands
//   - internal/config: Configuration and tool definitions
//   - internal/tui/components/completions: Command completion UI
//   - internal/message: Message and attachment types
package commands

