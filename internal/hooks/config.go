package hooks

import "time"

// HookConfig defines a user-configured shell command that fires on a hook
// event (e.g. PreToolUse). This is a pure-data struct: matcher compilation
// is owned by Runner so a JSON round-trip, merge, or reload can't silently
// drop compiled state.
type HookConfig struct {
	// Friendly display name shown in the TUI. Falls back to Command when empty.
	Name string `json:"name,omitempty" jsonschema:"description=Friendly display name shown in the TUI for this hook"`
	// Regex pattern tested against the tool name. Empty means match all.
	Matcher string `json:"matcher,omitempty" jsonschema:"description=Regex pattern tested against the tool name. Empty means match all tools."`
	// Shell command to execute.
	Command string `json:"command" jsonschema:"required,description=Shell command to execute when the hook fires"`
	// Timeout in seconds. Default 30.
	Timeout int `json:"timeout,omitempty" jsonschema:"description=Timeout in seconds for the hook command,default=30"`
	// IncludeSubAgents controls whether this hook also fires on sub-agent
	// tool calls (agent task tool, agentic_fetch, etc.). When false (the
	// default), hooks only run on the top-level agent's calls.
	IncludeSubAgents bool `json:"include_sub_agents,omitempty" jsonschema:"description=Also fire this hook on sub-agent tool calls,default=false"`
}

// DisplayName returns the hook name for display purposes. It returns Name
// when set, otherwise falls back to Command.
func (h *HookConfig) DisplayName() string {
	if h.Name != "" {
		return h.Name
	}
	return h.Command
}

// TimeoutDuration returns the hook timeout as a time.Duration, defaulting
// to 30s.
func (h *HookConfig) TimeoutDuration() time.Duration {
	if h.Timeout <= 0 {
		return 30 * time.Second
	}
	return time.Duration(h.Timeout) * time.Second
}
