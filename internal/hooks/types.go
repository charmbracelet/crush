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
	Data any

	// ToolName is the tool name (for tool hooks only).
	ToolName string

	// ToolCallID is the tool call ID (for tool hooks only).
	ToolCallID string

	// Environment contains additional environment variables to pass to the hook.
	Environment map[string]string
}

// HookResult contains the result of hook execution.
type HookResult struct {
	// HookType the hook type
	HookType HookType `json:"hook_type"`
	// Name the name of the hook (usually the file name)
	Name string `json:"name"`
	// Path hook path
	Path string `json:"path"`
	// AllResults stores all results for this event
	AllResults []HookResult `json:"all_results,omitempty"`
	// Continue indicates whether to continue execution.
	// If false, execution stops.
	Continue bool `json:"continue"`

	// Permission decision (for PreToolUse hooks only).
	// Values: "ask" (default), "approve", "deny"
	Permission string `json:"permission"`

	// ModifiedPrompt is the modified user prompt (for UserPromptSubmit).
	ModifiedPrompt *string `json:"modified_prompt"`

	// ModifiedInput is the modified tool input parameters (for PreToolUse).
	// This is a map that can be merged with the original tool input.
	ModifiedInput map[string]any `json:"modified_input"`

	// ModifiedOutput is the modified tool output (for PostToolUse).
	ModifiedOutput map[string]any `json:"modified_output"`

	// ContextContent is raw text content to add to LLM context.
	ContextContent string `json:"context_content"`

	// ContextFiles is a list of file paths to load and add to LLM context.
	ContextFiles []string `json:"context_files"`

	// Message is a user-facing message (logged and potentially displayed).
	Message string `json:"message"`
}

// Manager coordinates hook discovery and execution.
type Manager interface {
	// ListHooks returns all discovered hooks for a given type.
	ListHooks(hookType HookType) []string

	// ExecuteUserPromptSubmit executes the UserPromptSubmit event
	ExecuteUserPromptSubmit(ctx context.Context, sessionID, workingDir string, data UserPromptSubmitData) (HookResult, error)

	// ExecutePreToolUse executes the PreToolUse event
	ExecutePreToolUse(ctx context.Context, sessionID, workingDir string, data PreToolUseData) (HookResult, error)

	// ExecutePostToolUse executes the PostToolUse event
	ExecutePostToolUse(ctx context.Context, sessionID, workingDir string, data PostToolUseData) (HookResult, error)

	// ExecuteStop executes the Stop event
	ExecuteStop(ctx context.Context, sessionID, workingDir string, data StopData) (HookResult, error)
}

type UserPromptSubmitData struct {
	Prompt         string   `json:"prompt"`
	Attachments    []string `json:"attachments"`
	Model          string   `json:"model"`
	Provider       string   `json:"provider"`
	IsFirstMessage bool     `json:"is_first_message"`
}

type PreToolUseData struct {
	ToolName   string         `json:"tool_name"`
	ToolCallID string         `json:"tool_call_id"`
	ToolInput  map[string]any `json:"tool_input"`
}

type PostToolUseData struct {
	ToolName        string         `json:"tool_name"`
	ToolCallID      string         `json:"tool_call_id"`
	ToolInput       map[string]any `json:"tool_input"`
	ToolOutput      map[string]any `json:"tool_output"`
	ExecutionTimeMs int64          `json:"execution_time_ms"`
}

type StopData struct {
	Reason string `json:"reason"`
}
