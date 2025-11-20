package hooks

// Config defines hook system configuration.
type Config struct {
	// Enabled controls whether hooks are executed.
	Enabled bool

	// TimeoutSeconds is the maximum time a hook can run.
	TimeoutSeconds int

	// Directories are additional directories to search for hooks.
	// Defaults to [".crush/hooks"] if empty.
	Directories []string

	// Inline hooks defined directly in configuration.
	// Map key is the hook type (e.g., "pre-tool-use").
	Inline map[string][]InlineHook

	// Disabled is a list of hook paths to skip.
	// Paths are relative to the hooks directory.
	// Example: ["pre-tool-use/02-slow-check.sh"]
	Disabled []string

	// Environment variables to pass to hooks.
	Environment map[string]string
}

// InlineHook is a hook defined inline in the config.
type InlineHook struct {
	// Name is the name of the hook (used as filename).
	Name string

	// Script is the bash script content.
	Script string
}
