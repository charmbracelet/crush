// Package honcho provides a client for the Honcho memory API and the
// glue that makes Crush a stateful agent on top of it.
//
// Honcho stores conversation turns as messages against peers inside
// sessions, reasons over them in the background, and exposes the
// derived knowledge as representations and conclusions. Crush writes
// its turns out asynchronously and reads context back in two tiers: a
// session-stable snapshot sealed into the system prompt, and a
// volatile per-turn recall block appended at the tail of the
// conversation.
//
// Every exported method tolerates a nil receiver so a disabled
// integration costs one nil check rather than a branch at every call
// site.
package honcho

import "time"

// Workspace is a top-level container isolating one application's
// peers and sessions.
type Workspace struct {
	ID       string         `json:"id"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Peer is any entity that persists but changes over time: a user, an
// agent, a document. Crush models the human and itself as peers.
type Peer struct {
	ID       string         `json:"id"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// PeerConfig controls whether Honcho derives knowledge about a peer
// (ObserveMe) and whether that peer derives knowledge about the others
// it shares a session with (ObserveOthers).
//
// Both fields are pointers because the API distinguishes "leave as
// configured" from an explicit false.
type PeerConfig struct {
	ObserveMe     *bool `json:"observe_me,omitempty"`
	ObserveOthers *bool `json:"observe_others,omitempty"`
}

// Session is an interaction thread between peers with temporal
// boundaries. It is the unit Honcho summarizes and scopes recall to.
type Session struct {
	ID       string         `json:"id"`
	Metadata map[string]any `json:"metadata,omitempty"`
	// Peers maps peer ID to that peer's configuration within the
	// session. Peers named here are created if they do not exist.
	Peers map[string]PeerConfig `json:"peers,omitempty"`
}

// Message is a single unit of data written to a session. Writing a
// message is what triggers Honcho's background reasoning.
type Message struct {
	ID          string         `json:"id,omitempty"`
	PeerID      string         `json:"peer_id"`
	SessionID   string         `json:"session_id,omitempty"`
	WorkspaceID string         `json:"workspace_id,omitempty"`
	Content     string         `json:"content"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	TokenCount  int            `json:"token_count,omitempty"`
	// CreatedAt backdates a message. The importer sets it so
	// historical transcripts reason in their original order; live
	// writes leave it zero and let the server stamp it.
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

// Summary is a condensed view of a session's messages up to a point.
type Summary struct {
	Content     string `json:"content"`
	MessageID   string `json:"message_id"`
	SummaryType string `json:"summary_type"`
	TokenCount  int    `json:"token_count"`
	CreatedAt   string `json:"created_at"`
}

// Summaries holds the two summary tiers Honcho maintains per session.
// Short covers recent turns; long covers the whole session.
type Summaries struct {
	ID    string   `json:"id"`
	Short *Summary `json:"short_summary,omitempty"`
	Long  *Summary `json:"long_summary,omitempty"`
}

// SessionContext is the result of asking Honcho to assemble context
// for a session: recent messages, an optional summary, and the
// representation of whichever peer perspective was requested.
type SessionContext struct {
	ID             string    `json:"id"`
	Messages       []Message `json:"messages"`
	Summary        *Summary  `json:"summary,omitempty"`
	Representation string    `json:"peer_representation,omitempty"`
	PeerCard       []string  `json:"peer_card,omitempty"`
}

// PeerContext is a peer's standing representation plus its peer card,
// independent of any one session.
type PeerContext struct {
	PeerID         string   `json:"peer_id"`
	TargetID       string   `json:"target_id"`
	Representation string   `json:"representation,omitempty"`
	PeerCard       []string `json:"peer_card,omitempty"`
}

// Conclusion is a durable statement one peer holds about another.
// Honcho derives these in the background; Crush can also write them
// directly when the model decides something is worth remembering.
type Conclusion struct {
	ID           string         `json:"id,omitempty"`
	Content      string         `json:"content"`
	ObserverID   string         `json:"observer_id"`
	ObservedID   string         `json:"observed_id"`
	SessionID    string         `json:"session_id,omitempty"`
	Level        string         `json:"level,omitempty"`
	TimesDerived int            `json:"times_derived,omitempty"`
	CreatedAt    string         `json:"created_at,omitempty"`
	SourceIDs    []string       `json:"source_ids,omitempty"`
	Distance     *float64       `json:"distance,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// ReasoningLevel selects how much work the dialectic endpoint does
// before answering. Higher levels cost more latency and tokens.
type ReasoningLevel string

// Reasoning levels accepted by the chat endpoint.
const (
	ReasoningLow  ReasoningLevel = "low"
	ReasoningHigh ReasoningLevel = "high"
)

// ChatQuery asks a peer's representation a natural-language question.
//
// Target selects directional mode: the observer is the peer named in
// the request path and the observed is Target. Leaving Target empty
// queries the peer's own self-collection, which is what unified
// observation mode wants.
type ChatQuery struct {
	Query           string         `json:"query"`
	Target          string         `json:"target,omitempty"`
	SessionID       string         `json:"session_id,omitempty"`
	ReasoningLevel  ReasoningLevel `json:"reasoning_level,omitempty"`
	IncludeEvidence bool           `json:"include_evidence,omitempty"`
	Stream          bool           `json:"stream,omitempty"`
}

// ChatResponse is the dialectic answer. Content is nil-able on the
// wire when Honcho has nothing to say.
type ChatResponse struct {
	Content string `json:"content"`
}

// SearchQuery performs lexical/semantic search over stored messages.
type SearchQuery struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

// ConclusionQuery performs semantic search over stored conclusions.
type ConclusionQuery struct {
	Query    string   `json:"query"`
	TopK     int      `json:"top_k,omitempty"`
	Distance *float64 `json:"distance,omitempty"`
}

// ContextOptions tunes how much context Honcho assembles and how it
// selects conclusions to include.
//
// The zero value is meaningful: it asks for an exhaustive context
// within the server's configured maximum.
type ContextOptions struct {
	// Tokens caps the whole assembled context. Honcho allocates 40%
	// to the summary and 60% to recent messages.
	Tokens int
	// SearchQuery biases conclusion selection toward a topic,
	// typically the user's current prompt.
	SearchQuery string
	// Summary requests the session summary. Defaults to true
	// server-side, so IncludeSummary is a pointer to allow opting out.
	IncludeSummary *bool
	// PeerPerspective and PeerTarget select a directional
	// representation: what PeerPerspective believes about PeerTarget.
	PeerPerspective string
	PeerTarget      string
	// LimitToSession restricts recall to this session instead of
	// drawing on everything the peer knows.
	LimitToSession bool
	// SearchTopK and SearchMaxDistance tune semantic selection.
	SearchTopK        int
	SearchMaxDistance float64
	// IncludeMostFrequent adds the peer's most frequently derived
	// conclusions regardless of the search query.
	IncludeMostFrequent bool
	// MaxConclusions caps how many conclusions enter the context.
	MaxConclusions int
}

// PeerContextOptions tunes a standing peer representation lookup.
type PeerContextOptions struct {
	Target              string
	SearchQuery         string
	SearchTopK          int
	SearchMaxDistance   float64
	IncludeMostFrequent bool
	MaxConclusions      int
}

// QueueStatus reports Honcho's background reasoning backlog. A
// non-empty queue means recently written messages have not yet
// influenced representations.
type QueueStatus struct {
	Total      int  `json:"total_work_units,omitempty"`
	Completed  int  `json:"completed_work_units,omitempty"`
	InProgress int  `json:"in_progress_work_units,omitempty"`
	Pending    int  `json:"pending_work_units,omitempty"`
	Empty      bool `json:"empty,omitempty"`
}
