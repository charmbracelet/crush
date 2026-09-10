package notebook

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/notebook"
)

// SearchToolName is the name of the notebook_search tool.
const SearchToolName = "notebook_search"

//go:embed search.md
var searchDescription []byte

// SearchParams holds the parameters for the notebook_search tool.
type SearchParams struct {
	Query string `json:"query,omitempty" description:"Optional query to filter entries by tag, event type, or text. Omit to list all entries."`
}

// NewSearchTool creates a tool that lists and searches notebook entries.
// With no query: returns all entries (titles + tags only). With a query:
// returns matching entries (titles + tags only). Use recall for full
// content.
func NewSearchTool(svc notebook.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		SearchToolName,
		string(searchDescription),
		func(ctx context.Context, params SearchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := getSessionID(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("session ID is required for notebook_search"), nil
			}

			var entries []notebook.Entry
			var err error
			if params.Query == "" {
				entries, err = svc.GetEntries(ctx, sessionID)
			} else {
				entries, err = searchNotebook(ctx, svc, sessionID, params.Query)
			}
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to search notebook: %v", err)), nil
			}

			if len(entries) == 0 {
				return fantasy.NewTextResponse("No notebook entries found."), nil
			}

			var sb strings.Builder
			for _, e := range entries {
				sb.WriteString(fmt.Sprintf("Turn %d.%d — %s", e.TurnNumber, e.EventNumber, e.Title))
				if len(e.Tags) > 0 {
					sb.WriteString(" ")
					sb.WriteString(strings.Join(e.Tags, " "))
				}
				sb.WriteString("\n")
			}
			return fantasy.NewTextResponse(sb.String()), nil
		},
	)
}
