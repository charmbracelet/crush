package config

import (
	"fmt"
	"log/slog"
)

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
	// Type is the hook type: "command" or "prompt".
	Type string `json:"type" jsonschema:"description=Hook type,enum=command,enum=prompt,default=command"`
	// Command is the shell command to execute (for type: "command").
	// WARNING: Hook commands execute with Crush's full permissions. Only use trusted commands.
	Command string `json:"command,omitempty" jsonschema:"description=Shell command to execute for this hook (executes with Crush's permissions),example=echo 'Hook executed'"`
	// Prompt is the LLM prompt to execute (for type: "prompt").
	// Use $ARGUMENTS placeholder to include hook context JSON.
	Prompt string `json:"prompt,omitempty" jsonschema:"description=LLM prompt for intelligent decision making,example=Analyze if all tasks are complete. Context: $ARGUMENTS. Return JSON with decision and reason."`
	// Timeout is the maximum time in seconds to wait for the hook to complete.
	// Default is 30 seconds.
	Timeout *int `json:"timeout,omitempty" jsonschema:"description=Maximum time in seconds to wait for hook completion,default=30,minimum=1,maximum=300"`
}

// Validate checks hook configuration invariants.
func (h *Hook) Validate() error {
	switch h.Type {
	case "prompt":
		if h.Prompt == "" {
			return fmt.Errorf("prompt-based hook missing 'prompt' field")
		}
	case "", "command":
		if h.Command == "" {
			return fmt.Errorf("command-based hook missing 'command' field")
		}
	default:
		return fmt.Errorf("unsupported hook type: %s", h.Type)
	}
	if h.Timeout != nil {
		if *h.Timeout < 1 {
			slog.Warn("Hook timeout too low, using minimum",
				"configured", *h.Timeout, "minimum", 1)
			v := 1
			h.Timeout = &v
		}
		if *h.Timeout > 300 {
			slog.Warn("Hook timeout too high, using maximum",
				"configured", *h.Timeout, "maximum", 300)
			v := 300
			h.Timeout = &v
		}
	}
	return nil
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

// Validate validates the entire hook configuration.
func (c HookConfig) Validate() error {
	for eventType, matchers := range c {
		for i, matcher := range matchers {
			for j := range matcher.Hooks {
				if err := matcher.Hooks[j].Validate(); err != nil {
					return fmt.Errorf("invalid hook config for %s matcher %d hook %d: %w",
						eventType, i, j, err)
				}
			}
		}
	}
	return nil
}
