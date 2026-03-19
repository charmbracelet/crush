package plugin

import "context"

// SessionStore provides session and message persistence for plugins.
// This enables session export/import and cross-process session continuity.
type SessionStore interface {
	// GetSession returns a session by ID. Returns nil if not found.
	GetSession(ctx context.Context, id string) (*SessionData, error)

	// CreateSession creates a new session with the given title.
	// Returns the created session with its generated ID.
	CreateSession(ctx context.Context, title string) (*SessionData, error)

	// ListSessionMessages returns all messages for a session, ordered by creation time.
	ListSessionMessages(ctx context.Context, sessionID string) ([]SessionMessage, error)

	// ImportSession imports a full session snapshot (session + messages).
	// If a session with the same ID already exists, it is replaced.
	ImportSession(ctx context.Context, snapshot SessionSnapshot) error

	// ExportSession exports a full session snapshot (session + messages).
	ExportSession(ctx context.Context, sessionID string) (*SessionSnapshot, error)
}

// SessionData represents a chat session's metadata.
type SessionData struct {
	ID               string  `json:"id"`
	Title            string  `json:"title"`
	SummaryMessageID string  `json:"summary_message_id,omitempty"`
	MessageCount     int64   `json:"message_count"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	Cost             float64 `json:"cost"`
	CreatedAt        int64   `json:"created_at"`
	UpdatedAt        int64   `json:"updated_at"`
}

// SessionMessage represents a single message in a session's conversation history.
// Parts is the raw JSON-encoded parts array from the database, preserving
// the full fidelity of the internal message format (text, reasoning, tool calls,
// tool results, finish markers, images, binary content).
type SessionMessage struct {
	ID               string `json:"id"`
	SessionID        string `json:"session_id"`
	Role             string `json:"role"`
	Parts            string `json:"parts"`
	Model            string `json:"model,omitempty"`
	Provider         string `json:"provider,omitempty"`
	IsSummaryMessage bool   `json:"is_summary_message,omitempty"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

// SessionSnapshot is a portable representation of a complete session
// including all conversation history. It can be serialized to JSON
// and transferred between Crush instances for session migration.
type SessionSnapshot struct {
	// Version is the snapshot format version for forward compatibility.
	Version int `json:"version"`

	// Session contains the session metadata.
	Session SessionData `json:"session"`

	// Messages contains the full ordered conversation history.
	// Parts are stored as raw JSON to preserve internal format fidelity.
	Messages []SessionMessage `json:"messages"`
}

// CurrentSnapshotVersion is the current session snapshot format version.
const CurrentSnapshotVersion = 1
