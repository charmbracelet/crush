package config

// HookEventType represents the lifecycle event when a hook should run.
type HookEventType string

const (
	// PreToolUse runs before tool calls and can block them.
	PreToolUse HookEventType = "pre_tool_use"
	// PostToolUse runs after tool calls complete.
	PostToolUse HookEventType = "post_tool_use"
	// UserPromptSubmit runs when the user submits a prompt, before processing.
	UserPromptSubmit HookEventType = "user_prompt_submit"
	// Stop runs when Crush finishes responding.
	Stop HookEventType = "stop"
	// SubagentStop runs when subagent tasks complete.
	SubagentStop HookEventType = "subagent_stop"
	// PreCompact runs before running a compact operation.
	PreCompact HookEventType = "pre_compact"
	// PermissionRequested runs when a permission is requested from the user.
	PermissionRequested HookEventType = "permission_requested"
)

// Hook represents a single hook command configuration.
type Hook struct {
	// Type is the hook type, currently only "command" is supported.
	Type string `json:"type" jsonschema:"description=Hook type,enum=command,default=command"`
	// Command is the shell command to execute.
	// WARNING: Hook commands execute with Crush's full permissions. Only use trusted commands.
	Command string `json:"command" jsonschema:"required,description=Shell command to execute for this hook (executes with Crush's permissions),example=echo 'Hook executed'"`
	// Timeout is the maximum time in seconds to wait for the hook to complete.
	// Default is 30 seconds.
	Timeout *int `json:"timeout,omitempty" jsonschema:"description=Maximum time in seconds to wait for hook completion,default=30,minimum=1,maximum=300"`
}

// HookMatcher represents a matcher for a specific event type.
type HookMatcher struct {
	// Matcher is the tool name or pattern to match (for tool events).
	// For non-tool events, this can be empty or "*" to match all.
	// Supports pipe-separated tool names like "edit|write|multiedit".
	Matcher string `json:"matcher,omitempty" jsonschema:"description=Tool name or pattern to match (e.g. 'bash' 'edit|write' for multiple or '*' for all),example=bash,example=edit|write|multiedit,example=*"`
	// Hooks is the list of hooks to execute when the matcher matches.
	Hooks []Hook `json:"hooks" jsonschema:"required,description=List of hooks to execute when matcher matches"`
}

// HookConfig holds the complete hook configuration.
type HookConfig map[HookEventType][]HookMatcher
