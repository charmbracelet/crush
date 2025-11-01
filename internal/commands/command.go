package commands

// Command represents a slash command that can be executed in the Crush editor.
// Commands are loaded from markdown files in various locations (project, user home, XDG config)
// and can include YAML frontmatter for metadata.
type Command struct {
	// Name is the full command name, including namespace if applicable.
	// Examples: "review-pr", "frontend:review-pr", "frontend:components:button"
	Name string

	// Namespace is the namespace derived from subdirectory structure.
	// Empty for root-level commands, e.g., "frontend" or "frontend:components"
	Namespace string

	// Description is a brief description of the command, parsed from frontmatter.
	// Shown in command completions and \help output.
	Description string

	// ArgumentHint provides hints about expected arguments, parsed from frontmatter.
	// Example: "[pr-number] [priority]"
	ArgumentHint string

	// AllowedTools is a list of Crush tool names that are allowed when executing this command.
	// Parsed from frontmatter. If empty, all tools are available.
	// Example: []string{"View", "Edit", "Grep"}
	AllowedTools []string

	// Content is the full command content (markdown) after frontmatter is removed.
	// This is the actual prompt/content sent to the agent.
	Content string

	// Path is the file path where this command was loaded from.
	// Example: ".crush/commands/frontend/review-pr.md"
	Path string

	// Source indicates where the command was loaded from.
	// Examples: "project:frontend", "user", "user:frontend"
	Source string
}

