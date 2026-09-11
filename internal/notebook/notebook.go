// Package notebook provides per-event context summarization for agent
// sessions. It classifies tool calls as significant or trivial, generates
// structured notebook entries using a small model, stores them in SQLite,
// and supports retrieval via tag/text search and compaction.
package notebook

import (
	"context"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/message"
)

// Event types for notebook entries.
const (
	EventFileRead    = "file_read"
	EventFileEdit    = "file_edit"
	EventCommand     = "command"
	EventDecision    = "decision"
	EventExploration = "exploration"
	EventGeneral     = "general"
)

// Compression levels for notebook entries.
const (
	CompressionFull     = 0 // Full entry (≤ maxEntryTokens).
	CompressionSummary  = 1 // Tags + 1 sentence (~100 tokens).
	CompressionTagsOnly = 2 // Tags only (~20 tokens).
)

// Defaults for notebook configuration.
const (
	DefaultRawTokenBudget    = 25_000
	DefaultMaxNotebookTokens = 100_000
	DefaultMaxEntryTokens    = 1_000
)

// Entry is a notebook entry with its associated tags.
type Entry struct {
	ID               string
	SessionID        string
	TurnNumber       int64
	EventNumber      int64
	EventType        string
	Title            string
	EntryText        string
	EntryTextFull    string // Original uncompressed text; empty if same as EntryText.
	TokenCount       int64
	CompressionLevel int64
	CreatedAt        int64
	Tags             []string
	// Succeeded reports whether the underlying tool event completed
	// without error. Only successful events may supersede earlier
	// same-file entries: a failed edit leaves the file — and every
	// prior read of it — untouched.
	Succeeded bool
	// ErrorHeadline is a one-line digest of the underlying failure for
	// failed tool events. It survives compaction so later turns can
	// compare repeated failures against it.
	ErrorHeadline string
}

// EntryInput is the input for generating a notebook entry from a
// significant event. It is produced by classifying the tool calls in a
// turn and sent to the small model for structured entry generation.
type EntryInput struct {
	EventType   string
	Title       string
	Description string // Human-readable description of the event.
	ToolCall    *message.ToolCall
	ToolResult  *message.ToolResult
	// Succeeded mirrors !ToolResult.IsError for tool events. Entries
	// without a tool result (decisions) are always successful.
	Succeeded bool
	// ErrorHeadline carries a one-line digest of the failure for
	// entries whose tool result is an error.
	ErrorHeadline string
}

// Service is the interface for notebook operations.
type Service interface {
	// GenerateEntries classifies events in a turn and generates notebook
	// entries asynchronously using the small model. It is non-blocking;
	// the caller should invoke it in a goroutine.
	GenerateEntries(ctx context.Context, sessionID string, turnNumber int64, msgs []message.Message) error

	// GetEntries retrieves all notebook entries for a session, ordered
	// by turn and event number.
	GetEntries(ctx context.Context, sessionID string) ([]Entry, error)

	// SearchByTag retrieves entries matching a tag.
	SearchByTag(ctx context.Context, sessionID, tag string) ([]Entry, error)

	// SearchByText retrieves entries whose text contains the query.
	SearchByText(ctx context.Context, sessionID, query string) ([]Entry, error)

	// SearchByEventType retrieves entries of a specific event type.
	SearchByEventType(ctx context.Context, sessionID, eventType string) ([]Entry, error)

	// GetByTurn retrieves entries for a specific turn number.
	GetByTurn(ctx context.Context, sessionID string, turnNumber int64) ([]Entry, error)

	// GetTokenCount returns the total token count of all entries for a
	// session.
	GetTokenCount(ctx context.Context, sessionID string) (int64, error)

	// Compact compresses the oldest entries when the notebook exceeds
	// the max token limit.
	Compact(ctx context.Context, sessionID string) error

	// DeleteEntries removes all notebook entries for a session.
	DeleteEntries(ctx context.Context, sessionID string) error
}

// service implements the notebook Service interface.
type service struct {
	q         *db.Queries
	generator Generator
	opts      Options
}

// Options configures the notebook service.
type Options struct {
	MaxEntryTokens    int64
	MaxNotebookTokens int64
	// PreCompactRunner, when set, fires PreCompact hooks before
	// Compact compresses entries. A deny or halt decision skips
	// compaction for that round.
	PreCompactRunner *hooks.Runner
}

// Generator generates notebook entries from classified events using an
// LLM. It is abstracted so tests can provide a mock.
type Generator interface {
	// Generate takes classified events and returns structured entry
	// texts. Each returned string is the full entry text for one event.
	Generate(ctx context.Context, sessionID string, events []EntryInput) ([]GeneratedEntry, error)
}

// GeneratedEntry is the output of the Generator for one event.
type GeneratedEntry struct {
	EventType string
	Title     string
	Text      string
	Tags      []string
}

// NewService creates a new notebook service.
func NewService(q *db.Queries, generator Generator, opts Options) Service {
	if opts.MaxEntryTokens == 0 {
		opts.MaxEntryTokens = DefaultMaxEntryTokens
	}
	if opts.MaxNotebookTokens == 0 {
		opts.MaxNotebookTokens = DefaultMaxNotebookTokens
	}
	// Propagate MaxEntryTokens to the generator so it can scale its
	// max output token budget based on event count.
	if llmGen, ok := generator.(*llmGenerator); ok {
		llmGen.maxEntryTokens = opts.MaxEntryTokens
	}
	return &service{
		q:         q,
		generator: generator,
		opts:      opts,
	}
}
