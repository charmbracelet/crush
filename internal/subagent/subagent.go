package subagent

// SubagentSource defines where the subagent was loaded from.
type SubagentSource string

const (
	SubagentSourceCLI     SubagentSource = "cli"
	SubagentSourceProject SubagentSource = "project"
	SubagentSourceUser    SubagentSource = "user"
	SubagentSourcePlugin  SubagentSource = "plugin"
)

// Hook defines a lifecycle hook for a subagent.
type Hook struct {
	Type    string `yaml:"type"`    // e.g., "command"
	Command string `yaml:"command"` // The command to run
}

// HookMatcher defines a set of hooks that match a specific tool.
type HookMatcher struct {
	Matcher string `yaml:"matcher"` // Regex to match tool names
	Hooks   []Hook `yaml:"hooks"`
}

// SubagentHooks defines the available lifecycle hooks for a subagent.
type SubagentHooks struct {
	PreToolUse  []HookMatcher `yaml:"PreToolUse,omitempty"`
	PostToolUse []HookMatcher `yaml:"PostToolUse,omitempty"`
	Stop        []Hook        `yaml:"Stop,omitempty"`
}

// Subagent defines the structure of a custom subagent.
type Subagent struct {
	// From frontmatter
	Name            string         `yaml:"name"`
	Description     string         `yaml:"description"`
	Tools           []string       `yaml:"tools,omitempty"`
	DisallowedTools []string       `yaml:"disallowedTools,omitempty"`
	Model           string         `yaml:"model,omitempty"` // sonnet|opus|haiku|inherit
	PermissionMode  string         `yaml:"permissionMode,omitempty"`
	Color           string         `yaml:"color,omitempty"`
	Skills          []string       `yaml:"skills,omitempty"`
	Hooks           *SubagentHooks `yaml:"hooks,omitempty"`

	// Parsed from body
	SystemPrompt string `yaml:"-"`

	// Metadata
	Source   SubagentSource `yaml:"-"`
	FilePath string         `yaml:"-"`
	Disabled bool           `yaml:"-"`
}
