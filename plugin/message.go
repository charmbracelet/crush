package plugin

import "context"

// MessageEventType identifies the type of message event.
type MessageEventType string

const (
	// MessageCreated is published when a new message is created.
	MessageCreated MessageEventType = "created"
	// MessageUpdated is published when a message is updated.
	MessageUpdated MessageEventType = "updated"
	// MessageDeleted is published when a message is deleted.
	MessageDeleted MessageEventType = "deleted"
)

// MessageRole identifies who sent the message.
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleSystem    MessageRole = "system"
	MessageRoleTool      MessageRole = "tool"
)

// Message represents a chat message exposed to plugins.
type Message struct {
	ID        string
	SessionID string
	Role      MessageRole
	Content   string // Text content of the message

	// ToolCalls contains tool invocations in assistant messages.
	ToolCalls []ToolCallInfo
	// ToolResults contains tool results in tool messages.
	ToolResults []ToolResultInfo

	CreatedAt int64
	UpdatedAt int64
}

// ToolCallInfo contains information about a tool invocation.
type ToolCallInfo struct {
	ID       string
	Name     string
	Input    string // JSON input to the tool
	Finished bool
}

// ToolResultInfo contains information about a tool result.
type ToolResultInfo struct {
	ToolCallID string
	Name       string
	Content    string
	IsError    bool
}

// MessageEvent represents a message lifecycle event.
type MessageEvent struct {
	Type    MessageEventType
	Message Message
}

// MessageSubscriber allows plugins to subscribe to message events.
type MessageSubscriber interface {
	// SubscribeMessages returns a channel that receives message events.
	// The channel is closed when ctx is canceled.
	SubscribeMessages(ctx context.Context) <-chan MessageEvent
}

// SessionInfo contains metadata about the current session.
type SessionInfo struct {
	// Model is the AI model identifier (e.g., "claude-3-opus-20240229").
	Model string
	// Provider is the API provider (e.g., "anthropic", "bedrock", "openrouter").
	Provider string
	// Tokens contains token usage counters.
	Tokens TokenUsage
	// CostUSD is the estimated cost in USD.
	CostUSD float64
}

// TokenUsage contains token usage counters.
type TokenUsage struct {
	Input      int64 // Total input tokens consumed.
	Output     int64 // Total output tokens generated.
	CacheRead  int64 // Tokens read from cache (Anthropic).
	CacheWrite int64 // Tokens written to cache (Anthropic).
}

// SessionInfoProvider allows plugins to get session metadata.
type SessionInfoProvider interface {
	// SessionInfo returns the current session info.
	// Returns nil if no session is active.
	SessionInfo() *SessionInfo
}

// PromptSubmitter allows plugins to submit prompts to the agent.
type PromptSubmitter interface {
	// SubmitPrompt sends a prompt to the agent as if it were typed by the user.
	// If the agent is busy, the prompt will be queued and executed when ready.
	// The sessionID is automatically determined from the current session.
	SubmitPrompt(ctx context.Context, prompt string) error

	// CurrentSessionID returns the ID of the currently active session.
	// Returns empty string if no session is active.
	CurrentSessionID() string

	// IsSessionBusy returns true if the current session is busy processing.
	IsSessionBusy() bool
}
