package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/taigrr/fantasy"

	"github.com/taigrr/crush/internal/message"
	"github.com/taigrr/crush/internal/session"
)

const (
	SearchHistoryToolName = "search_history"
	maxHistoryMatches     = 50
	historySnippetWindow  = 80 // chars on each side of the match
)

//go:embed search_history.md
var searchHistoryDescription string

// SearchHistoryParams scopes the search. SessionID is optional; when set
// it broadens the role filter (we list ALL messages in that session) so
// the agent can recover prior assistant reasoning, not just user input.
type SearchHistoryParams struct {
	Query     string `json:"query" description:"Substring to search for (case-insensitive)"`
	SessionID string `json:"session_id,omitempty" description:"Optional: limit to one session and include assistant messages too"`
	Limit     int    `json:"limit,omitempty" description:"Max matches to return (default 20, max 50)"`
}

// SearchHistoryHit is one row in the response. Snippet is the matched
// region with surrounding context, ready to render.
type SearchHistoryHit struct {
	SessionID    string `json:"session_id"`
	SessionTitle string `json:"session_title"`
	MessageID    string `json:"message_id"`
	Role         string `json:"role"`
	CreatedAt    string `json:"created_at"`
	Snippet      string `json:"snippet"`
}

// NewSearchHistoryTool returns the search_history tool.
func NewSearchHistoryTool(messages message.Service, sessions session.Service) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		SearchHistoryToolName,
		searchHistoryDescription,
		func(ctx context.Context, params SearchHistoryParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			query := strings.TrimSpace(params.Query)
			if query == "" {
				return fantasy.NewTextErrorResponse("query is required"), nil
			}

			limit := params.Limit
			if limit <= 0 {
				limit = 20
			}
			if limit > maxHistoryMatches {
				limit = maxHistoryMatches
			}

			titles, err := sessionTitles(ctx, sessions)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to load sessions: %s", err)), nil
			}

			var msgs []message.Message
			if params.SessionID != "" {
				msgs, err = messages.List(ctx, params.SessionID)
			} else {
				msgs, err = messages.ListAllUserMessages(ctx)
			}
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to load messages: %s", err)), nil
			}

			needle := strings.ToLower(query)
			hits := make([]SearchHistoryHit, 0, limit)
			for _, m := range msgs {
				body := messageBody(m)
				if body == "" {
					continue
				}
				idx := strings.Index(strings.ToLower(body), needle)
				if idx < 0 {
					continue
				}
				hits = append(hits, SearchHistoryHit{
					SessionID:    m.SessionID,
					SessionTitle: titles[m.SessionID],
					MessageID:    m.ID,
					Role:         string(m.Role),
					CreatedAt:    time.Unix(m.CreatedAt, 0).Format(time.RFC3339),
					Snippet:      snippetAround(body, idx, len(query)),
				})
				if len(hits) >= limit {
					break
				}
			}

			if len(hits) == 0 {
				return fantasy.NewTextResponse(fmt.Sprintf("No matches for %q", query)), nil
			}
			return fantasy.NewTextResponse(formatHistoryHits(query, hits)), nil
		},
	)
}

// sessionTitles loads every session and returns id -> title. Cheap: the
// session table is small (one row per chat) and we read all of it once.
func sessionTitles(ctx context.Context, sessions session.Service) (map[string]string, error) {
	all, err := sessions.List(ctx)
	if err != nil {
		return nil, err
	}
	titles := make(map[string]string, len(all))
	for _, s := range all {
		titles[s.ID] = s.Title
	}
	return titles, nil
}

// messageBody concatenates the human-readable parts of a message into a
// single search target. We skip binary/image parts: those don't have
// useful text to match against.
func messageBody(m message.Message) string {
	var b strings.Builder
	if t := m.Content().Text; t != "" {
		b.WriteString(t)
	}
	if r := m.ReasoningContent().Thinking; r != "" {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(r)
	}
	return b.String()
}

// snippetAround extracts a window around the match index, with leading
// and trailing ellipsis when we trimmed.
func snippetAround(text string, idx, queryLen int) string {
	start := idx - historySnippetWindow
	if start < 0 {
		start = 0
	}
	end := idx + queryLen + historySnippetWindow
	if end > len(text) {
		end = len(text)
	}
	prefix := ""
	suffix := ""
	if start > 0 {
		prefix = "…"
	}
	if end < len(text) {
		suffix = "…"
	}
	// Replace newlines so the snippet stays single-line in the output.
	body := strings.ReplaceAll(text[start:end], "\n", " ")
	return prefix + body + suffix
}

// formatHistoryHits renders the hits in chronological-feeling order:
// most recent first within each session group is fine since the
// underlying queries return DESC by created_at.
func formatHistoryHits(query string, hits []SearchHistoryHit) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Found %d match(es) for %q:\n\n", len(hits), query)
	for _, h := range hits {
		title := h.SessionTitle
		if title == "" {
			title = "(untitled)"
		}
		fmt.Fprintf(&b, "[%s] %s — %s (session %s)\n  %s\n\n",
			h.CreatedAt, h.Role, title, shortID(h.SessionID), h.Snippet)
	}
	return b.String()
}

// shortID truncates a session ID for terse display in the listing.
func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
