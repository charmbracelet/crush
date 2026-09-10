package notebook

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/db"
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

// DeleteEntries removes all notebook entries for a session.
func (s *service) DeleteEntries(ctx context.Context, sessionID string) error {
	if err := s.q.DeleteNotebookEntriesBySession(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to delete notebook entries: %w", err)
	}
	return nil
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
			EventNumber:      row.EventNumber,
			EventType:        row.EventType,
			Title:            row.Title,
			EntryText:        row.EntryText,
			EntryTextFull:    row.EntryTextFull.String,
			TokenCount:       row.TokenCount,
			CompressionLevel: row.CompressionLevel,
			CreatedAt:        row.CreatedAt,
			Tags:             tags,
		})
	}
	return entries, nil
}

// Compact compresses the oldest entries when the notebook exceeds the
// max token limit. It incrementally compresses entries oldest-first,
// using GetOldestNotebookEntries, until the total is under the limit.
// Compression progresses through levels 0 → 1 → 2.
func (s *service) Compact(ctx context.Context, sessionID string) error {
	total, err := s.GetTokenCount(ctx, sessionID)
	if err != nil {
		return err
	}
	if total <= s.opts.MaxNotebookTokens {
		return nil
	}

	// Phase 1: Compress oldest level-0 entries to level 1 (tags + 1
	// sentence), one batch at a time, until under the limit.
	if err := s.compactOldestToLevel(ctx, sessionID, CompressionFull, CompressionSummary); err != nil {
		return err
	}

	total, err = s.GetTokenCount(ctx, sessionID)
	if err != nil {
		return err
	}
	if total <= s.opts.MaxNotebookTokens {
		return nil
	}

	// Phase 2: Compress oldest level-1 entries to level 2 (tags only).
	if err := s.compactOldestToLevel(ctx, sessionID, CompressionSummary, CompressionTagsOnly); err != nil {
		return err
	}

	return nil
}

// compactOldestToLevel compresses the oldest entries at fromLevel to
// toLevel, one entry at a time, checking the token count after each
// compression. This ensures earlier turns are always compressed before
// later ones, and we stop as soon as we're under the limit.
func (s *service) compactOldestToLevel(ctx context.Context, sessionID string, fromLevel, toLevel int64) error {
	for {
		total, err := s.GetTokenCount(ctx, sessionID)
		if err != nil {
			return err
		}
		if total <= s.opts.MaxNotebookTokens {
			return nil
		}

		// Get the single oldest entry at the source compression
		// level. This guarantees earlier turns are compressed
		// before later ones.
		entries, err := s.q.GetOldestNotebookEntries(ctx, db.GetOldestNotebookEntriesParams{
			SessionID:        sessionID,
			CompressionLevel: fromLevel,
			Limit:            1,
		})
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			return nil // No more entries at this level.
		}

		entry := entries[0]
		tags, err := s.q.GetNotebookTagsByEntry(ctx, entry.ID)
		if err != nil {
			tags = nil
		}
		compressed := compressEntry(entry.EntryText, entry.Title, tags, toLevel)
		newTokens := estimateTokens(compressed)
		if err := s.q.UpdateNotebookCompression(ctx, db.UpdateNotebookCompressionParams{
			EntryText:        compressed,
			TokenCount:       newTokens,
			CompressionLevel: toLevel,
			ID:               entry.ID,
		}); err != nil {
			return fmt.Errorf("failed to update notebook compression: %w", err)
		}
	}
}

// compressEntry produces a compressed version of an entry at the given
// level.
func compressEntry(text, title string, tags []string, level int64) string {
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
		return sb.String()

	case CompressionTagsOnly:
		// Tags only.
		if len(tags) > 0 {
			return strings.Join(tags, " ")
		}
		return title

	default:
		return text
	}
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
			text = compressEntry(e.EntryText, e.Title, e.Tags, e.CompressionLevel)
		}
		sb.WriteString(text)
		sb.WriteString("\n\n")
	}
	return sb.String()
}
