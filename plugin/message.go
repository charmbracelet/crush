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
