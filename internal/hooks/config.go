package hooks

// Config defines hook system configuration.
type Config struct {
	// Enabled controls whether hooks are executed.
	Enabled bool `json:"enabled,omitempty" jsonschema:"description=Enable or disable hook execution,default=true"`

	// TimeoutSeconds is the maximum time a hook can run.
	TimeoutSeconds int `json:"timeout_seconds,omitempty" jsonschema:"description=Maximum execution time for hooks in seconds,default=30,example=30"`

	// Directories are additional directories to search for hooks.
	// Defaults to [".crush/hooks"] if empty.
	Directories []string `json:"directories,omitempty" jsonschema:"description=Directories to search for hook scripts,example=.crush/hooks"`

	// Inline hooks defined directly in configuration.
	// Map key is the hook type (e.g., "pre-tool-use").
	Inline map[string][]InlineHook `json:"inline,omitempty" jsonschema:"description=Inline hook scripts defined in configuration"`

	// Disabled is a list of hook paths to skip.
	// Paths are relative to the hooks directory.
	// Example: ["pre-tool-use/02-slow-check.sh"]
	Disabled []string `json:"disabled,omitempty" jsonschema:"description=List of hook paths to disable,example=pre-tool-use/02-slow-check.sh"`

	// Environment variables to pass to hooks.
	Environment map[string]string `json:"environment,omitempty" jsonschema:"description=Environment variables to pass to all hooks"`
}

// InlineHook is a hook defined inline in the config.
type InlineHook struct {
	// Name is the name of the hook (used as filename).
	Name string `json:"name" jsonschema:"required,description=Name of the hook script,example=audit.sh"`

	// Script is the bash script content.
	Script string `json:"script" jsonschema:"required,description=Bash script content to execute"`
}
