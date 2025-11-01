package commands

// Registry provides methods for loading, querying, and managing slash commands.
// Commands are loaded from multiple locations (project, user home, XDG config)
// and can be queried by name or listed for completion/help functionality.
type Registry interface {
	// LoadCommands loads all commands from configured locations.
	// Returns a slice of all available commands and any error encountered during loading.
	// Errors from individual files are logged but don't prevent other commands from loading.
	LoadCommands() ([]Command, error)

	// FindCommand looks up a command by its full name (including namespace if applicable).
	// Examples: FindCommand("review-pr"), FindCommand("frontend:review-pr")
	// Returns the command if found, or an error if not found.
	FindCommand(name string) (*Command, error)

	// ListCommands returns all loaded commands.
	// Useful for \help command and command completions.
	// Returns all commands from all sources in a consistent order.
	ListCommands() []Command

	// Reload refreshes commands from all configured locations.
	// Useful for reloading commands without restarting Crush.
	// Clears existing commands and reloads from all sources.
	// Returns an error if reload fails completely, but partial failures are logged.
	Reload() error
}

