// Package notebook provides per-event context summarization for agent
// sessions. It classifies tool calls as significant or trivial, generates
// structured notebook entries using a small model, stores them in SQLite,
// and supports retrieval via tag/text search and compaction.
package notebook

import (
	"context"
	"database/sql"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/csync"
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
	// EventCheckpoint is a consolidated session position — an
	// Established/Open digest with evidence handles, written at the
	// investigation→execution boundary and at run end. Unlike other
	// entries it does not mark segment coverage: a checkpoint-only
	// turn is not a covered turn (see TurnsWithEntries), and its
	// file: tags never supersede the reads it cites.
	EventCheckpoint = "checkpoint"
)

// Checkpoint granularity values, carried as granularity:<value> tags
// on checkpoint entries. Ordering matters: finer feeds coarser — a
// boundary checkpoint's input may include turn digests, never another
// boundary checkpoint.
const (
	GranularityTurn     = "turn"
	GranularityBoundary = "boundary"
	GranularitySession  = "session"
)

// granularityTagPrefix namespaces the checkpoint granularity tag.
const granularityTagPrefix = "granularity:"

// granularityRank orders granularities finer → coarser. Unknown or
// missing granularity ranks coarsest — an untagged checkpoint is
// treated as session-grain so it never feeds a finer consolidation.
func granularityRank(g string) int {
	switch g {
	case GranularityTurn:
		return 0
	case GranularityBoundary:
		return 1
	case GranularitySession:
		return 2
	default:
		return 3
	}
}

// CheckpointGranularity returns the entry's granularity tag value, or
// "" for non-checkpoint entries and untagged checkpoints.
func CheckpointGranularity(e Entry) string {
	if e.EventType != EventCheckpoint {
		return ""
	}
	for _, tag := range e.Tags {
		if g, ok := strings.CutPrefix(tag, granularityTagPrefix); ok {
			return g
		}
	}
	return ""
}

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
	ID         string
	SessionID  string
	TurnNumber int64
	// SegmentNumber identifies which segment of the turn produced the
	// entry. Coverage and ordering are keyed on (turn, segment,
	// event); segment_number is never displayed. Entries written
	// before segment coverage existed carry 0.
	SegmentNumber    int64
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
	// Verified records the verification outcome for mutation events:
	// "verified", "unverified", or "failed". A different axis from
	// Succeeded — an edit can land cleanly while its check fails.
	// Empty for non-mutation events.
	Verified string
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
	// Verified is the entry-level verification state aggregated from
	// the tool result's "verification" metadata — worst wins, pending
	// counts as unverified. Empty for non-mutation events.
	Verified string
}

// Stats accumulates per-session sufficiency telemetry for the
// notebook layer: how often the model reaches back for detail
// (recalls, re-views) and which selection pass is doing the work.
// Shared across the agent and the recall tool via a csync map so the
// step-composition log line can emit one picture per session.
type Stats struct {
	// EntryRecalls counts recall queries that hit the notebook
	// (tag/turn/segment/type/text). This layer's sufficiency signal.
	EntryRecalls int
	// ResultRecalls counts result: queries — raw tool-result lookups.
	// A stubbing signal, not a notebook one; interpret it against the
	// stub track's metrics.
	ResultRecalls int
	// CrossRecalls counts cross: queries — mem0 cross-session
	// lookups.
	CrossRecalls int
	// EmptyRecalls counts recall queries that returned nothing —
	// entry, cross, and result lookups alike. Distinct from attempts:
	// "recall that found nothing" is its own sufficiency signal
	// (missing entries or a broken stub pointer, not thin ones).
	EmptyRecalls int
	// StubReViews counts view/read calls on files whose earlier read
	// result was stubbed — expected pressure, cheap to satisfy.
	StubReViews int
	// CoveredReViews counts view/read calls on files whose entries
	// the last prefix render injected — the entry-sufficiency signal.
	CoveredReViews int
	// Selection pass contributions, cumulative across renders:
	// SelPassRecency is the recency floor pass, SelPassPinned the
	// active-edit pin pass, SelPassRefs the prompt/todo ref pass,
	// SelPassWorking the working-set pass, SelPassFill the
	// newest-first fill.
	SelPassRecency int
	SelPassPinned  int
	SelPassRefs    int
	SelPassWorking int
	SelPassFill    int
	// CheckpointRenders counts renders that included a checkpoint
	// entry — the "checkpoint present at render" telemetry the
	// checkpoint eval arm asserts on.
	CheckpointRenders int
	// CheckpointsWritten counts committed checkpoint entries — the
	// firing side of the same metric.
	CheckpointsWritten int
}

// Service is the interface for notebook operations.
type Service interface {
	// GenerateEntries classifies events in a turn and generates notebook
	// entries asynchronously using the small model. It is non-blocking;
	// the caller should invoke it in a goroutine.
	//
	// Deprecated: kept for tests simulating pre-segment coverage.
	// Production callers use GenerateSegmentEntries — this method
	// writes segment_number=0 entries and no registry row, so a turn
	// generated through it pins unprocessed coverage and would be
	// regenerated by segment detection, duplicating its entries.
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

	// GetByTurnSegment retrieves entries for one segment of a turn.
	GetByTurnSegment(ctx context.Context, sessionID string, turnNumber, segmentNumber int64) ([]Entry, error)

	// GenerateSegmentEntries classifies the events in one closed
	// segment, generates entries for them, and commits the entries and
	// the segment's processed marker in one transaction. Event numbers
	// continue the turn's existing sequence so concurrent segments of
	// one turn never emit duplicate (turn, event) keys. A segment with
	// no significant events still commits the marker.
	GenerateSegmentEntries(ctx context.Context, sessionID string, turnNumber, segmentNumber, startIndex, endIndex int64, msgs []message.Message) error

	// GenerateCheckpoint writes one consolidated checkpoint entry when
	// enough new context has gathered since the last same-or-coarser
	// checkpoint and no checkpoint already carries req.RunTag. The
	// entry is keyed to (req.TurnNumber, req.SegmentNumber) — the
	// caller passes the session's last closed segment so the entry
	// renders once the boundary passes that key. Reports whether a
	// checkpoint committed; a false return leaves the run free to
	// retry under a different threshold.
	GenerateCheckpoint(ctx context.Context, sessionID string, req CheckpointRequest) (bool, error)

	// RecordSegmentClose records a closed segment's extent as
	// unprocessed. Idempotent under (session, turn, segment).
	RecordSegmentClose(ctx context.Context, sessionID string, turnNumber, segmentNumber, startIndex, endIndex int64) error

	// ProcessedSegments lists every recorded segment for a session.
	ProcessedSegments(ctx context.Context, sessionID string) ([]ProcessedSegment, error)

	// MarkSegmentsProcessed inserts coverage rows already processed —
	// the backfill path for turns that already have entries.
	MarkSegmentsProcessed(ctx context.Context, sessionID string, segs []ProcessedSegment) error

	// RecordSegmentAttempt notes that a generation attempt started,
	// driving retry backoff for failed segments.
	RecordSegmentAttempt(ctx context.Context, sessionID string, turnNumber, segmentNumber int64) error

	// TurnsWithEntries returns the set of turn numbers that have at
	// least one entry.
	TurnsWithEntries(ctx context.Context, sessionID string) (map[int64]bool, error)

	// GetTokenCount returns the total token count of all entries for a
	// session.
	GetTokenCount(ctx context.Context, sessionID string) (int64, error)

	// Compact compresses the oldest entries when the notebook exceeds
	// the max token limit.
	Compact(ctx context.Context, sessionID string) error

	// DeleteEntries removes all notebook entries for a session.
	DeleteEntries(ctx context.Context, sessionID string) error
	// ForgetSession drops the service's in-memory per-session state
	// (the compaction-stall counter). Called on session deletion; the
	// DB rows are already cascade-deleted.
	ForgetSession(sessionID string)
}

// service implements the notebook Service interface.
type service struct {
	q         *db.Queries
	db        *sql.DB
	generator Generator
	opts      Options
	// stallCounts tracks consecutive compaction rounds that made no
	// progress, per session — hook denials and all-pinned stalls share
	// one counter. stallMu serializes the counter's get-modify-set
	// (and the ordering of stall vs. resolve callbacks) because
	// per-segment generation runs Compacts on concurrent goroutines.
	stallCounts *csync.Map[string, int]
	stallMu     sync.Mutex
}

// Options configures the notebook service.
type Options struct {
	MaxEntryTokens    int64
	MaxNotebookTokens int64
	// PreCompactRunner, when set, fires PreCompact hooks before
	// Compact compresses entries. A deny or halt decision skips
	// compaction for that round.
	PreCompactRunner *hooks.Runner
	// DB enables transactional writes for segment coverage: entries
	// and the processed marker commit atomically, and the event-number
	// offset is read inside the write lock. Nil falls back to
	// sequential statements.
	DB *sql.DB
	// OnCompactionStall fires when Compact makes no progress for
	// several consecutive rounds — a PreCompact hook denying forever,
	// or every remaining entry pinned. It never overrides the deny or
	// pin; it only warns. reason describes the cause; an EMPTY reason
	// signals resolution — fired once on the progress round that ends
	// a warned streak, letting the UI clear the warning instead of
	// waiting out a TTL. It runs under the service's stall mutex:
	// implementations must not block or call back into the service.
	OnCompactionStall func(sessionID, reason string)
}

// CheckpointRequest parameterizes GenerateCheckpoint. The caller
// computes the entry's coverage key and supplies the uncovered tail.
type CheckpointRequest struct {
	// TurnNumber and SegmentNumber are the coverage key the entry is
	// written under — the session's last closed segment, so the
	// checkpoint renders once the boundary passes that key. An
	// open-segment key can never render mid-run.
	TurnNumber    int64
	SegmentNumber int64
	// Granularity is the checkpoint's consolidation level
	// (GranularityBoundary or GranularitySession). Inputs exclude
	// checkpoints at the same or a coarser granularity — a boundary
	// checkpoint consolidates turn digests, never another boundary.
	Granularity string
	// RunTag is the dedup tag ("run:<stamp>"): an existing checkpoint
	// carrying it short-circuits generation so a run writes at most
	// one. Empty disables the check.
	RunTag string
	// MinExploration is the minimum count of newly gathered context —
	// committed entries plus classified non-trivial non-mutating tail
	// events, counted since the last same-or-coarser checkpoint —
	// required before a checkpoint is worth a small-model call.
	MinExploration int
	// Msgs is the uncovered tail: messages past the last segment with
	// committed coverage. Their classified events supplement the
	// committed entries as consolidation input.
	Msgs []message.Message
}

// Generator generates notebook entries from classified events using an
// LLM. It is abstracted so tests can provide a mock.
type Generator interface {
	// Generate takes classified events and returns structured entry
	// texts. Each returned string is the full entry text for one event.
	Generate(ctx context.Context, sessionID string, events []EntryInput) ([]GeneratedEntry, error)
	// GenerateCheckpoint produces one consolidated checkpoint entry
	// from a rendered input block (committed entry digests plus raw
	// tail event descriptions).
	GenerateCheckpoint(ctx context.Context, sessionID, input string) (GeneratedEntry, error)
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
		q:           q,
		db:          opts.DB,
		generator:   generator,
		opts:        opts,
		stallCounts: csync.NewMap[string, int](),
	}
}
