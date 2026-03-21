// Package plugin provides an extensible hooks system for Crush.
// It allows users to intercept and customize core behaviors like tool execution,
// LLM requests, permission decisions, and shell environment injection.
package plugin

import (
	"context"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
)

// PluginInput provides context for plugin initialization.
type PluginInput struct {
	Config     *config.ConfigStore
	Sessions   session.Service
	Messages   message.Service
	WorkingDir string
}

// Hooks defines all available extension points.
// Plugins can implement any subset of these hooks.
type Hooks struct {
	// Tool execution lifecycle hooks.
	// ToolBeforeExecute is called before a tool executes. It can modify args
	// or skip execution entirely by returning Skip=true.
	ToolBeforeExecute func(ctx context.Context, input ToolBeforeExecuteInput) (*ToolBeforeExecuteOutput, error)
	// ToolAfterExecute is called after a tool executes. It can modify the result.
	ToolAfterExecute func(ctx context.Context, input ToolAfterExecuteInput) (*ToolAfterExecuteOutput, error)

	// Chat lifecycle hooks.
	// ChatBeforeRequest is called before sending a request to the LLM.
	ChatBeforeRequest func(ctx context.Context, input ChatBeforeRequestInput) error
	// ChatAfterResponse is called after receiving a response from the LLM.
	ChatAfterResponse func(ctx context.Context, input ChatAfterResponseInput) error
	// ChatMessagesTransform is called before prompt construction so plugins can
	// transform the outgoing session history for the current request purpose.
	ChatMessagesTransform func(ctx context.Context, input ChatMessagesTransformInput, output *ChatMessagesTransformOutput) error
	// ChatSystemTransform is called before sending a request so plugins can
	// adjust system prompt sections and provider-specific prefixes.
	ChatSystemTransform func(ctx context.Context, input ChatSystemTransformInput, output *ChatSystemTransformOutput) error
	// SessionCompacting is called before summarization to customize the summary
	// prompt or inject additional continuation context.
	SessionCompacting func(ctx context.Context, input SessionCompactingInput, output *SessionCompactingOutput) error

	// Permission handling.
	// PermissionAsk is called when a permission decision is needed.
	// If it returns an action other than PermissionAsk, that decision is used
	// instead of prompting the user.
	PermissionAsk func(input PermissionAskInput) PermissionAskOutput

	// Shell environment injection.
	// ShellEnv is called when building the environment for shell execution.
	ShellEnv func(ctx context.Context, input ShellEnvInput) map[string]string

	// Message lifecycle.
	// MessageCreated is called after a new message is created.
	MessageCreated func(ctx context.Context, msg message.Message) error

	// Custom tool definitions.
	// Tools is a map of tool name to tool definition.
	Tools map[string]ToolDefinition
}

// ToolDefinition describes a custom tool that can be registered by plugins.
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  any // JSON Schema
	Execute     func(ctx context.Context, args map[string]any, execCtx ToolContext) (string, error)
}

// ToolContext provides execution context for custom tools.
type ToolContext struct {
	SessionID string
	MessageID string
	Agent     string
	Directory string
	Worktree  string
}

// ToolBeforeExecuteInput is the input for ToolBeforeExecute hook.
type ToolBeforeExecuteInput struct {
	Tool      string
	SessionID string
	CallID    string
	Args      map[string]any
}

// ToolBeforeExecuteOutput is the output for ToolBeforeExecute hook.
type ToolBeforeExecuteOutput struct {
	Args      map[string]any // Modified args (optional)
	Skip      bool           // If true, skip execution
	PreResult string         // Result to use if Skip is true
}

// ToolAfterExecuteInput is the input for ToolAfterExecute hook.
type ToolAfterExecuteInput struct {
	Tool      string
	SessionID string
	CallID    string
	Args      map[string]any
	Result    string
	Metadata  map[string]any
}

// ToolAfterExecuteOutput is the output for ToolAfterExecute hook.
type ToolAfterExecuteOutput struct {
	Result        string         // Modified result.
	ResultChanged bool           // True when Result should replace current result.
	Metadata      map[string]any // Modified metadata (optional).
}

// ChatBeforeRequestInput is the input for ChatBeforeRequest hook.
type ChatBeforeRequestInput struct {
	SessionID string
	Agent     string
	Model     ModelInfo
	Provider  ProviderContext
	Message   message.Message
}

// ChatAfterResponseInput is the input for ChatAfterResponse hook.
type ChatAfterResponseInput struct {
	SessionID string
	Agent     string
	Model     ModelInfo
	Purpose   ChatTransformPurpose
	Result    *fantasy.AgentResult
	Error     error
}

// ChatTransformPurpose represents the purpose of a chat transform.
type ChatTransformPurpose string

const (
	// ChatTransformPurposeRequest is the purpose for a request.
	ChatTransformPurposeRequest ChatTransformPurpose = "request"
	// ChatTransformPurposePreflightEstimate is the purpose for a preflight estimate.
	ChatTransformPurposePreflightEstimate ChatTransformPurpose = "preflight_estimate"
	// ChatTransformPurposeNextStepEstimate is the purpose for a next step estimate.
	ChatTransformPurposeNextStepEstimate ChatTransformPurpose = "next_step_estimate"
	// ChatTransformPurposeSummarize is the purpose for a summarize.
	ChatTransformPurposeSummarize ChatTransformPurpose = "summarize"
	// ChatTransformPurposeRecover is the purpose for a recover.
	ChatTransformPurposeRecover ChatTransformPurpose = "recover"
)

// ChatMessagesTransformInput is the input for ChatMessagesTransform hook.
type ChatMessagesTransformInput struct {
	SessionID string
	Agent     string
	Model     ModelInfo
	Provider  ProviderContext
	Purpose   ChatTransformPurpose
	Message   message.Message
}

// ChatMessagesTransformOutput is the output for ChatMessagesTransform hook.
type ChatMessagesTransformOutput struct {
	Messages []message.Message
}

// ChatSystemTransformInput is the input for ChatSystemTransform hook.
type ChatSystemTransformInput struct {
	SessionID string
	Agent     string
	Model     ModelInfo
	Provider  ProviderContext
	Purpose   ChatTransformPurpose
	Message   message.Message
}

// ChatSystemTransformOutput is the output for ChatSystemTransform hook.
type ChatSystemTransformOutput struct {
	System []string
	Prefix string
}

// SessionCompactingInput is the input for SessionCompacting hook.
type SessionCompactingInput struct {
	SessionID string
	Agent     string
	Model     ModelInfo
	Purpose   ChatTransformPurpose
}

// SessionCompactingOutput is the output for SessionCompacting hook.
type SessionCompactingOutput struct {
	Context []string
	Prompt  string
}

// PermissionAction represents a permission decision.
type PermissionAction int

const (
	// PermissionAsk means no override, use default behavior.
	PermissionAsk PermissionAction = iota
	// PermissionAllow means allow the permission.
	PermissionAllow
	// PermissionDeny means deny the permission.
	PermissionDeny
)

// PermissionRequest is the normalized permission request passed to plugins.
type PermissionRequest struct {
	ID          string
	SessionID   string
	ToolCallID  string
	ToolName    string
	Description string
	Action      string
	Params      any
	Path        string
}

// PermissionAskInput is the input for PermissionAsk hook.
type PermissionAskInput struct {
	Permission PermissionRequest
}

// PermissionAskOutput is the output for PermissionAsk hook.
type PermissionAskOutput struct {
	Action PermissionAction
}

// ShellEnvInput is the input for ShellEnv hook.
type ShellEnvInput struct {
	CWD       string
	SessionID string
	CallID    string
}

// ModelInfo contains model identification information.
type ModelInfo struct {
	ProviderID    string
	ModelID       string
	ContextWindow int64 `json:"context_window,omitempty"`
}

// ProviderContext contains provider context information.
type ProviderContext struct {
	Source  string         // "env", "config", "custom", "api"
	Options map[string]any // Provider-specific options
}

// Plugin is the interface that all plugins must implement.
type Plugin interface {
	// Name returns the plugin identifier.
	Name() string

	// Init initializes the plugin and returns hooks.
	// The plugin should return a Hooks struct with any hooks it wants to register.
	// Returning an empty Hooks struct is valid if the plugin only wants to
	// perform initialization logic.
	Init(ctx context.Context, input PluginInput) (Hooks, error)

	// Close shuts down the plugin, releasing any resources (e.g. persistent processes).
	// Plugins that don't hold resources can return nil.
	Close(ctx context.Context) error
}
