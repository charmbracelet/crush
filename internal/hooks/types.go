// Package hooks provides a Git-like hooks system for Crush.
//
// Hooks are executable scripts that run at specific points in the application
// lifecycle. They can modify behavior, add context, control permissions, and
// audit activity.
package hooks

import "context"

// HookType represents the type of hook.
type HookType string

const (
	// HookUserPromptSubmit executes after user submits prompt, before sending to LLM.
	HookUserPromptSubmit HookType = "user-prompt-submit"

	// HookPreToolUse executes after LLM requests tool use, before permission check & execution.
	HookPreToolUse HookType = "pre-tool-use"

	// HookPostToolUse executes after tool executes, before result sent to LLM.
	HookPostToolUse HookType = "post-tool-use"

	// HookStop executes when agent conversation loop stops or is cancelled.
	HookStop HookType = "stop"
)

// HookContext contains the data passed to hooks.
type HookContext struct {
	// HookType is the type of hook being executed.
	HookType HookType

	// SessionID is the current session ID.
	SessionID string

	// WorkingDir is the working directory.
	WorkingDir string

	// Data is hook-specific data marshaled to JSON and passed via stdin.
	// For UserPromptSubmit: prompt, attachments, model, is_first_message
	// For PreToolUse: tool_name, tool_call_id, tool_input
	// For PostToolUse: tool_name, tool_call_id, tool_input, tool_output, execution_time_ms
	// For Stop: reason
	Data map[string]any

	// ToolName is the tool name (for tool hooks only).
	ToolName string

	// ToolCallID is the tool call ID (for tool hooks only).
	ToolCallID string

	// Environment contains additional environment variables to pass to the hook.
	Environment map[string]string
}

// HookResult contains the result of hook execution.
type HookResult struct {
	// Continue indicates whether to continue execution.
	// If false, execution stops.
	Continue bool

	// Permission decision (for PreToolUse hooks only).
	// Values: "ask" (default), "approve", "deny"
	Permission string

	// ModifiedPrompt is the modified user prompt (for UserPromptSubmit).
	ModifiedPrompt *string

	// ModifiedInput is the modified tool input parameters (for PreToolUse).
	// This is a map that can be merged with the original tool input.
	ModifiedInput map[string]any

	// ModifiedOutput is the modified tool output (for PostToolUse).
	ModifiedOutput map[string]any

	// ContextContent is raw text content to add to LLM context.
	ContextContent string

	// ContextFiles is a list of file paths to load and add to LLM context.
	ContextFiles []string

	// Message is a user-facing message (logged and potentially displayed).
	Message string
}

// Manager coordinates hook discovery and execution.
type Manager interface {
	// ExecuteHooks executes all hooks for the given type in order.
	// Returns accumulated results from all hooks.
	ExecuteHooks(ctx context.Context, hookType HookType, context HookContext) (HookResult, error)

	// ListHooks returns all discovered hooks for a given type.
	ListHooks(hookType HookType) []string
}
