package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/history"
)

//go:embed history_search.md
var historySearchDescription []byte

const HistorySearchToolName = "history_search"

type HistorySearchParams struct {
	Query     string `json:"query" description:"Text to search for in message content"`
	SessionID string `json:"session_id,omitempty" description:"Optional session ID to scope search"`
	Limit     int    `json:"limit,omitempty" description:"Maximum number of results to return (default 10, max 100)"`
}

func NewHistorySearchTool(historySvc history.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		HistorySearchToolName,
		string(historySearchDescription),
		func(ctx context.Context, params HistorySearchParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			query := strings.TrimSpace(params.Query)
			if query == "" {
				return fantasy.NewTextErrorResponse("query is required"), nil
			}

			limit := params.Limit
			if limit == 0 {
				limit = 10
			}

			results, err := historySvc.SearchMessages(ctx, history.SearchParams{
				Query:     query,
				SessionID: strings.TrimSpace(params.SessionID),
				Limit:     limit,
			})
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if len(results) == 0 {
				return fantasy.NewTextResponse("No matching messages found."), nil
			}

			var b strings.Builder
			fmt.Fprintf(&b, "Found %d matching messages:\n\n", len(results))
			for i, result := range results {
				text := strings.ReplaceAll(result.Text, "\n", " ")
				text = strings.TrimSpace(text)
				if len([]rune(text)) > 120 {
					text = string([]rune(text)[:120]) + "…"
				}
				timestamp := time.Unix(result.CreatedAt, 0).Format(time.RFC3339)
				fmt.Fprintf(&b, "%d. [%s] session=%s role=%s\n   %s\n", i+1, timestamp, result.SessionID, result.Role, text)
			}

			return fantasy.NewTextResponse(strings.TrimSpace(b.String())), nil
		},
	)
}
