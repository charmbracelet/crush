package notebook

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/hooks"
)

// GetEntries retrieves all notebook entries for a session.
func (s *service) GetEntries(ctx context.Context, sessionID string) ([]Entry, error) {
	rows, err := s.q.GetNotebookEntries(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get notebook entries: %w", err)
	}
	return s.enrichEntries(ctx, rows)
}

// SearchByTag retrieves entries matching a tag.
func (s *service) SearchByTag(ctx context.Context, sessionID, tag string) ([]Entry, error) {
	rows, err := s.q.SearchNotebookByTag(ctx, db.SearchNotebookByTagParams{
		SessionID: sessionID,
		Tag:       tag,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search notebook by tag: %w", err)
	}
	return s.enrichEntries(ctx, rows)
}

// SearchByText retrieves entries whose text contains the query.
func (s *service) SearchByText(ctx context.Context, sessionID, query string) ([]Entry, error) {
	rows, err := s.q.SearchNotebookByText(ctx, db.SearchNotebookByTextParams{
		SessionID:  sessionID,
		SearchTerm: "%" + query + "%",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search notebook by text: %w", err)
	}
	return s.enrichEntries(ctx, rows)
}

// SearchByEventType retrieves entries of a specific event type.
func (s *service) SearchByEventType(ctx context.Context, sessionID, eventType string) ([]Entry, error) {
	rows, err := s.q.GetNotebookEntriesByEventType(ctx, db.GetNotebookEntriesByEventTypeParams{
		SessionID: sessionID,
		EventType: eventType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search notebook by event type: %w", err)
	}
	return s.enrichEntries(ctx, rows)
}

// GetByTurn retrieves entries for a specific turn number.
func (s *service) GetByTurn(ctx context.Context, sessionID string, turnNumber int64) ([]Entry, error) {
	rows, err := s.q.GetNotebookEntriesByTurn(ctx, db.GetNotebookEntriesByTurnParams{
		SessionID:  sessionID,
		TurnNumber: turnNumber,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get notebook entries by turn: %w", err)
	}
	return s.enrichEntries(ctx, rows)
}

// GetTokenCount returns the total token count of all entries for a
// session.
func (s *service) GetTokenCount(ctx context.Context, sessionID string) (int64, error) {
	count, err := s.q.GetNotebookTokenCount(ctx, sessionID)
	if err != nil {
		return 0, fmt.Errorf("failed to get notebook token count: %w", err)
	}
	return count, nil
}

// DeleteEntries removes all notebook entries and segment coverage
// rows for a session.
func (s *service) DeleteEntries(ctx context.Context, sessionID string) error {
	s.ForgetSession(sessionID)
	return s.withTx(ctx, func(q *db.Queries) error {
		if err := q.DeleteNotebookEntriesBySession(ctx, sessionID); err != nil {
			return fmt.Errorf("failed to delete notebook entries: %w", err)
		}
		if err := q.DeleteProcessedSegmentsBySession(ctx, sessionID); err != nil {
			return fmt.Errorf("failed to delete processed segments: %w", err)
		}
		return nil
	})
}

// enrichEntries converts DB rows to Entry structs and loads their tags.
func (s *service) enrichEntries(ctx context.Context, rows []db.NotebookEntry) ([]Entry, error) {
	entries := make([]Entry, 0, len(rows))
	for _, row := range rows {
		tags, err := s.q.GetNotebookTagsByEntry(ctx, row.ID)
		if err != nil {
			tags = nil
		}
		entries = append(entries, Entry{
			ID:               row.ID,
			SessionID:        row.SessionID,
			TurnNumber:       row.TurnNumber,
			SegmentNumber:    row.SegmentNumber,
			EventNumber:      row.EventNumber,
			EventType:        row.EventType,
			Title:            row.Title,
			EntryText:        row.EntryText,
			EntryTextFull:    row.EntryTextFull.String,
			TokenCount:       row.TokenCount,
			CompressionLevel: row.CompressionLevel,
			CreatedAt:        row.CreatedAt,
			Tags:             tags,
			Succeeded:        row.Succeeded != 0,
			ErrorHeadline:    row.ErrorHeadline,
			Verified:         row.Verified,
		})
	}
	return entries, nil
}

// PinnedFileTags returns the file: tags whose files had an edit entry
// in the last two turns present in entries — successful or not. A file
// stuck in an edit-fail-retry loop is just as "under active edit" as
// one whose edits landed. Entries carrying a pinned tag are prioritized
// during selection and never compressed by compaction.
func PinnedFileTags(entries []Entry) map[string]bool {
	if len(entries) == 0 {
		return nil
	}
	maxTurn := entries[len(entries)-1].TurnNumber
	for _, e := range entries {
		if e.TurnNumber > maxTurn {
			maxTurn = e.TurnNumber
		}
	}
	pinned := make(map[string]bool)
	for _, e := range entries {
		if e.EventType != EventFileEdit || e.TurnNumber < maxTurn-1 {
			continue
		}
		for _, tag := range e.Tags {
			if strings.HasPrefix(tag, "file:") {
				pinned[tag] = true
			}
		}
	}
	return pinned
}

// PinnedFileTagsSince is the segment-grained variant of
// PinnedFileTags: an edit entry pins its file when its (turn, segment)
// key is at or after (sinceTurn, sinceSegment). Selection uses this so
// a single long turn does not pin every file it ever edited — only
// the segments near the tail count as "under active edit".
func PinnedFileTagsSince(entries []Entry, sinceTurn, sinceSegment int64) map[string]bool {
	pinned := make(map[string]bool)
	for _, e := range entries {
		if e.EventType != EventFileEdit {
			continue
		}
		if e.TurnNumber < sinceTurn || (e.TurnNumber == sinceTurn && e.SegmentNumber < sinceSegment) {
			continue
		}
		for _, tag := range e.Tags {
			if strings.HasPrefix(tag, "file:") {
				pinned[tag] = true
			}
		}
	}
	return pinned
}

// Compact compresses the oldest entries when the notebook exceeds the
// max token limit. It incrementally compresses entries oldest-first,
// using GetOldestNotebookEntries, until the total is under the limit.
// Compression progresses through levels 0 → 1 → 2. Entries pinned to
// files under active edit are skipped.
//
// A round that makes no progress — a PreCompact denial, or every
// remaining entry pinned — increments a per-session stall counter;
// consecutive stalls past compactionStallThreshold surface a
// user-visible warning via Options.OnCompactionStall. The counter
// resets on any round that makes progress.
func (s *service) Compact(ctx context.Context, sessionID string) error {
	total, err := s.GetTokenCount(ctx, sessionID)
	if err != nil {
		return err
	}
	if total <= s.opts.MaxNotebookTokens {
		s.noteCompactProgress(sessionID)
		return nil
	}

	// Fire PreCompact hooks; a deny or halt skips this round.
	if s.opts.PreCompactRunner != nil {
		input := fmt.Sprintf(`{"token_count":%d,"max_tokens":%d}`, total, s.opts.MaxNotebookTokens)
		result, err := s.opts.PreCompactRunner.Run(ctx, hooks.EventPreCompact, sessionID, "compact", input)
		if err != nil {
			slog.Warn("PreCompact hook failed; proceeding", "session_id", sessionID, "error", err)
		} else if result.Decision == hooks.DecisionDeny || result.Halt {
			slog.Info("PreCompact hook blocked compaction", "session_id", sessionID, "reason", result.Reason)
			s.noteCompactStall(sessionID,
				"PreCompact hook permanently blocking compaction; notebook DB grows unbounded — prompt unaffected")
			return nil
		}
	}

	pinned, err := s.pinnedEntryIDs(ctx, sessionID)
	if err != nil {
		return err
	}

	// Phase 1: Compress oldest level-0 entries to level 1 (tags + 1
	// sentence), one batch at a time, until under the limit.
	compressed, err := s.compactOldestToLevel(ctx, sessionID, CompressionFull, CompressionSummary, pinned)
	if err != nil {
		return err
	}

	total, err = s.GetTokenCount(ctx, sessionID)
	if err != nil {
		return err
	}
	if total <= s.opts.MaxNotebookTokens {
		s.noteCompactProgress(sessionID)
		return nil
	}

	// Phase 2: Compress oldest level-1 entries to level 2 (tags only).
	n, err := s.compactOldestToLevel(ctx, sessionID, CompressionSummary, CompressionTagsOnly, pinned)
	if err != nil {
		return err
	}
	compressed += n

	total, err = s.GetTokenCount(ctx, sessionID)
	if err != nil {
		return err
	}
	switch {
	case total <= s.opts.MaxNotebookTokens || compressed > 0:
		// Under budget, or at least moving: a session alternating
		// stall and progress must not accumulate warnings.
		s.noteCompactProgress(sessionID)
	default:
		s.noteCompactStall(sessionID,
			"Notebook compaction stalled: no compressible entries remain (all pinned or already at maximum compression); notebook DB grows unbounded — prompt unaffected")
	}
	return nil
}

// compactionStallThreshold is the number of consecutive no-progress
// Compact rounds before the stall callback fires. Compact runs per
// segment commit, so a streak this long can accrue within one turn.
const compactionStallThreshold = 3

// ForgetSession drops the session's in-memory stall counter.
func (s *service) ForgetSession(sessionID string) {
	s.stallMu.Lock()
	defer s.stallMu.Unlock()
	s.stallCounts.Del(sessionID)
}

// noteCompactStall counts a no-progress compaction round and fires the
// stall callback once the streak reaches the threshold — on every
// further round too, so the warning stays live while the condition
// persists.
func (s *service) noteCompactStall(sessionID, reason string) {
	if sessionID == "" {
		return
	}
	s.stallMu.Lock()
	defer s.stallMu.Unlock()
	n, _ := s.stallCounts.Get(sessionID)
	n++
	s.stallCounts.Set(sessionID, n)
	slog.Warn("Notebook compaction made no progress",
		"session_id", sessionID, "consecutive", n, "reason", reason)
	if n >= compactionStallThreshold && s.opts.OnCompactionStall != nil {
		s.opts.OnCompactionStall(sessionID, reason)
	}
}

// noteCompactProgress resets the stall counter for a round that
// compressed something or ended under budget. When the reset streak
// had already warned, the callback fires once more with an empty
// reason — the resolution signal that lets the UI clear the warning
// instead of waiting out a TTL.
func (s *service) noteCompactProgress(sessionID string) {
	if sessionID == "" {
		return
	}
	s.stallMu.Lock()
	defer s.stallMu.Unlock()
	n, _ := s.stallCounts.Get(sessionID)
	s.stallCounts.Del(sessionID)
	if n >= compactionStallThreshold && s.opts.OnCompactionStall != nil {
		s.opts.OnCompactionStall(sessionID, "")
	}
}

// pinnedEntryIDs returns the IDs of entries pinned to files under
// active edit — entries carrying a file: tag that had a successful
// edit entry in the last two turns.
func (s *service) pinnedEntryIDs(ctx context.Context, sessionID string) (map[string]bool, error) {
	entries, err := s.GetEntries(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	tags := PinnedFileTags(entries)
	if len(tags) == 0 {
		return nil, nil
	}
	ids := make(map[string]bool)
	for _, e := range entries {
		for _, tag := range e.Tags {
			if tags[tag] {
				ids[e.ID] = true
				break
			}
		}
	}
	return ids, nil
}

// compactOldestToLevel compresses the oldest entries at fromLevel to
// toLevel, one entry at a time, checking the token count after each
// compression. This ensures earlier turns are always compressed before
// later ones, and we stop as soon as we're under the limit. Entries in
// the pinned ID set are skipped. pinned is keyed by entry ID. Returns
// the number of entries compressed so the caller can distinguish
// "under budget" from "stalled on pins".
func (s *service) compactOldestToLevel(ctx context.Context, sessionID string, fromLevel, toLevel int64, pinned map[string]bool) (int, error) {
	compressed := 0
	for {
		total, err := s.GetTokenCount(ctx, sessionID)
		if err != nil {
			return compressed, err
		}
		if total <= s.opts.MaxNotebookTokens {
			return compressed, nil
		}

		// Fetch all entries at the source compression level and pick
		// the oldest unpinned one. Scanning the full level avoids
		// stalling when a leading run of pinned entries fills a
		// fixed-size batch.
		entries, err := s.q.GetOldestNotebookEntries(ctx, db.GetOldestNotebookEntriesParams{
			SessionID:        sessionID,
			CompressionLevel: fromLevel,
			Limit:            math.MaxInt32,
		})
		if err != nil {
			return compressed, err
		}
		var entry *db.NotebookEntry
		for i := range entries {
			if pinned[entries[i].ID] {
				continue
			}
			entry = &entries[i]
			break
		}
		if entry == nil {
			// No unpinned entries at this level (or none at all).
			return compressed, nil
		}
		tags, err := s.q.GetNotebookTagsByEntry(ctx, entry.ID)
		if err != nil {
			tags = nil
		}
		text := compressEntry(entry.EntryText, entry.Title, tags, entry.ErrorHeadline, toLevel)
		newTokens := estimateTokens(text)
		if err := s.q.UpdateNotebookCompression(ctx, db.UpdateNotebookCompressionParams{
			EntryText:        text,
			TokenCount:       newTokens,
			CompressionLevel: toLevel,
			ID:               entry.ID,
		}); err != nil {
			return compressed, fmt.Errorf("failed to update notebook compression: %w", err)
		}
		compressed++
	}
}

// compressEntry produces a compressed version of an entry at the given
// level. A stored error headline survives every level — the digest is
// the part of a failure that stays useful after the body is gone.
func compressEntry(text, title string, tags []string, headline string, level int64) string {
	var out string
	switch level {
	case CompressionSummary:
		// Tags + first sentence of the entry.
		firstSentence := text
		if idx := strings.Index(text, "\n"); idx > 0 {
			firstSentence = text[:idx]
		}
		if len(firstSentence) > 200 {
			firstSentence = firstSentence[:200] + "…"
		}
		var sb strings.Builder
		sb.WriteString(firstSentence)
		if len(tags) > 0 {
			sb.WriteString("\n")
			sb.WriteString(strings.Join(tags, " "))
		}
		out = sb.String()

	case CompressionTagsOnly:
		// Tags only.
		if len(tags) > 0 {
			out = strings.Join(tags, " ")
		} else {
			out = title
		}

	default:
		out = text
	}
	if headline != "" && level >= CompressionSummary {
		out += "\nError: " + headline
	}
	return out
}

// RenderEntries renders a list of entries into a single text block for
// injection into the conversation as a system message.
func RenderEntries(entries []Entry) string {
	if len(entries) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, e := range entries {
		text := e.EntryText
		if e.CompressionLevel > 0 {
			text = compressEntry(e.EntryText, e.Title, e.Tags, e.ErrorHeadline, e.CompressionLevel)
		}
		sb.WriteString(text)
		sb.WriteString("\n\n")
	}
	return sb.String()
}
