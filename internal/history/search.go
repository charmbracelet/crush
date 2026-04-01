package history

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
)

const (
	DefaultSearchLimit = 20
	MaxSearchLimit     = 100
)

type SearchParams struct {
	Query     string
	SessionID string
	Limit     int
}

type MessageSearchResult struct {
	ID        string
	SessionID string
	Role      message.MessageRole
	Text      string
	CreatedAt int64
}

func (s *service) SearchMessages(ctx context.Context, params SearchParams) ([]MessageSearchResult, error) {
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	limit := cmp.Or(params.Limit, DefaultSearchLimit)
	if limit < 1 {
		limit = DefaultSearchLimit
	}
	if limit > MaxSearchLimit {
		limit = MaxSearchLimit
	}

	dbMessages, err := s.q.SearchMessages(ctx, db.SearchMessagesParams{
		SessionID: params.SessionID,
		Query:     sql.NullString{String: query, Valid: true},
		Limit:     int64(limit),
	})
	if err != nil {
		return nil, err
	}

	results := make([]MessageSearchResult, 0, len(dbMessages))
	for _, item := range dbMessages {
		text, err := extractTextFromParts(item.Parts)
		if err != nil {
			return nil, err
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		results = append(results, MessageSearchResult{
			ID:        item.ID,
			SessionID: item.SessionID,
			Role:      message.MessageRole(item.Role),
			Text:      text,
			CreatedAt: item.CreatedAt,
		})
	}

	return results, nil
}

func extractTextFromParts(partsJSON string) (string, error) {
	var wrapped []struct {
		Type string `json:"type"`
		Data struct {
			Text string `json:"text"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(partsJSON), &wrapped); err != nil {
		return "", err
	}
	for _, part := range wrapped {
		if part.Type == "text" {
			return part.Data.Text, nil
		}
	}
	return "", nil
}
