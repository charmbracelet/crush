package notebook

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/notebook"
)

// RecallToolName is the name of the recall tool.
const RecallToolName = "recall"

//go:embed recall.md
var recallDescription []byte

// RecallParams holds the parameters for the recall tool.
type RecallParams struct {
	Query string `json:"query" description:"Search query: a tag (file:auth.go), event type (command, decision), turn number (turn:5), or text to search for. Use cross: prefix to search across sessions via mem0."`
}

// recallContext holds dependencies for the recall tool.
type recallContext struct {
	svc        notebook.Service
	cfg        *config.ConfigStore
	mem0Server string
	syncMem0   bool
}

// NewRecallTool creates a tool that retrieves full notebook entries by
// tag, event type, turn number, or text search. When mem0 sync is
// enabled, cross-session search is available via the "cross:" prefix.
func NewRecallTool(svc notebook.Service, cfg *config.ConfigStore, mem0Server string, syncMem0 bool) fantasy.AgentTool {
	rc := &recallContext{
		svc:        svc,
		cfg:        cfg,
		mem0Server: mem0Server,
		syncMem0:   syncMem0,
	}
	return fantasy.NewAgentTool(
		RecallToolName,
		string(recallDescription),
		func(ctx context.Context, params RecallParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Query == "" {
				return fantasy.NewTextErrorResponse("query parameter is required"), nil
			}

			// Cross-session search via mem0.
			if strings.HasPrefix(params.Query, "cross:") {
				if !rc.syncMem0 || rc.cfg == nil || rc.mem0Server == "" {
					return fantasy.NewTextErrorResponse("cross-session search requires mem0 sync to be enabled"), nil
				}
				query := strings.TrimPrefix(params.Query, "cross:")
				result, err := notebook.SearchMem0(ctx, rc.cfg, rc.mem0Server, query)
				if err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("mem0 search failed: %v", err)), nil
				}
				if result == "" {
					return fantasy.NewTextResponse("No cross-session memories found."), nil
				}
				return fantasy.NewTextResponse(result), nil
			}

			sessionID := getSessionID(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("session ID is required for recall"), nil
			}

			entries, err := searchNotebook(ctx, rc.svc, sessionID, params.Query)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to search notebook: %v", err)), nil
			}

			if len(entries) == 0 {
				return fantasy.NewTextResponse("No notebook entries found matching the query."), nil
			}

			var sb strings.Builder
			for _, e := range entries {
				sb.WriteString(fmt.Sprintf("## Turn %d.%d — %s\n", e.TurnNumber, e.EventNumber, e.Title))
				// Return the full uncompressed text when available,
				// so recall always provides the original detail even
				// after compaction has replaced entry_text.
				text := e.EntryText
				if e.EntryTextFull != "" {
					text = e.EntryTextFull
				}
				sb.WriteString(text)
				if len(e.Tags) > 0 {
					sb.WriteString("\nTags: ")
					sb.WriteString(strings.Join(e.Tags, " "))
				}
				sb.WriteString("\n\n---\n\n")
			}
			return fantasy.NewTextResponse(sb.String()), nil
		},
	)
}

// searchNotebook dispatches a query to the appropriate search method
// based on the query prefix.
func searchNotebook(ctx context.Context, svc notebook.Service, sessionID, query string) ([]notebook.Entry, error) {
	// Normalize: strip a leading # if present so both "#file:auth.go"
	// and "file:auth.go" match the same stored tags.
	query = strings.TrimPrefix(query, "#")
	switch {
	case strings.HasPrefix(query, "file:"):
		return svc.SearchByTag(ctx, sessionID, query)
	case strings.HasPrefix(query, "phase:"):
		return svc.SearchByTag(ctx, sessionID, query)
	case strings.HasPrefix(query, "turn:"):
		var turn int64
		if n, _ := fmt.Sscanf(query, "turn:%d", &turn); n != 1 {
			return nil, fmt.Errorf("invalid turn: query %q: expected turn:<number>", query)
		}
		return svc.GetByTurn(ctx, sessionID, turn)
	case query == "command" || query == "decision" || query == "file_read" || query == "file_edit" || query == "exploration":
		return svc.SearchByEventType(ctx, sessionID, query)
	default:
		return svc.SearchByText(ctx, sessionID, query)
	}
}
