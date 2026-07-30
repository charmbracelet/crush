package remotecontrol

import "encoding/json"

// ProtocolVersion is the wire version advertised on outbound frames.
const ProtocolVersion = 1

// Event type names (shared with crush-remote).
const (
	TypeRegisterSession   = "register_session"
	TypeUnregisterSession = "unregister_session"
	TypeSessionList       = "session_list"
	TypeSessionState      = "session_state"
	TypeSessionSnapshot   = "session_snapshot"
	TypeStreamChunk       = "stream_chunk"
	TypeToolRequest       = "tool_request"
	TypeToolResponse      = "tool_response"
	TypeSendPrompt        = "send_prompt"
	TypeCancelTask        = "cancel_task"
	TypeRequestSnapshot   = "request_snapshot"
	TypeError             = "error"
	TypePing              = "ping"
	TypePong              = "pong"
)

// EventMessage is the standard JSON frame over WebSockets.
type EventMessage struct {
	Type            string          `json:"type"`
	SessionID       string          `json:"session_id,omitempty"`
	Payload         json.RawMessage `json:"payload,omitempty"`
	Timestamp       int64           `json:"timestamp"`
	ProtocolVersion int             `json:"protocol_version,omitempty"`
}

// SessionInfo is the metadata advertised for a remote-controlled session.
type SessionInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title,omitempty"`
	Cwd       string `json:"cwd,omitempty"`
	Busy      bool   `json:"busy,omitempty"`
	Model     string `json:"model,omitempty"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
}

// SessionListPayload is the v1 session_list body.
type SessionListPayload struct {
	Sessions []SessionInfo `json:"sessions"`
}

// SessionStatePayload is a lightweight busy/title update.
type SessionStatePayload struct {
	Busy       bool   `json:"busy,omitempty"`
	Title      string `json:"title,omitempty"`
	QueueDepth int    `json:"queue_depth,omitempty"`
}

// SnapshotMessage is one transcript entry in a session_snapshot.
type SnapshotMessage struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"created_at,omitempty"`
}

// SessionSnapshotPayload is the full transcript for a session.
type SessionSnapshotPayload struct {
	Messages []SnapshotMessage `json:"messages"`
}

// StreamChunkPayload is live text for a session.
type StreamChunkPayload struct {
	MessageID string `json:"message_id,omitempty"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Done      bool   `json:"done,omitempty"`
}

// ToolRequestPayload asks the phone (and desktop) for tool approval.
type ToolRequestPayload struct {
	RequestID   string `json:"request_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	Action      string `json:"action,omitempty"`
	Path        string `json:"path,omitempty"`
}

// ToolResponsePayload is the phone's approve/deny answer.
type ToolResponsePayload struct {
	RequestID string `json:"request_id"`
	Approved  bool   `json:"approved"`
}

// SendPromptPayload is an inbound prompt from the phone.
type SendPromptPayload struct {
	Prompt string `json:"prompt"`
}

// ErrorPayload is a structured error for the phone.
type ErrorPayload struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}
