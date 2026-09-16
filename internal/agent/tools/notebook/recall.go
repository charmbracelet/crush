package notebook

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/notebook"
)

// RecallToolName is the name of the recall tool.
const RecallToolName = "recall"

//go:embed recall.md
var recallDescription []byte

// RecallParams holds the parameters for the recall tool.
type RecallParams struct {
	Query string `json:"query" description:"Search query: a tag (file:auth.go), event type (command, decision), turn number (turn:5), segment (segment:5.2), an original tool result (result:<tool_call_id>), or text to search for. Use cross: prefix to search across sessions in this project via mem0."`
}

// recallContext holds dependencies for the recall tool.
type recallContext struct {
	svc        notebook.Service
	messages   message.Service
	cfg        *config.ConfigStore
	mem0Server string
	syncMem0   bool
	// stats is the shared per-session sufficiency counter map; nil
	// disables counting. The split by query type is deliberate:
	// entry recalls measure this layer, result: recalls measure the
	// stubbing track.
	stats *csync.Map[string, notebook.Stats]
}

// bump increments one counter on the session's stats record.
func (rc *recallContext) bump(sessionID string, f func(*notebook.Stats)) {
	if rc.stats == nil || sessionID == "" {
		return
	}
	s, _ := rc.stats.Get(sessionID)
	f(&s)
	rc.stats.Set(sessionID, s)
}

// NewRecallTool creates a tool that retrieves full notebook entries by
// tag, event type, turn/segment number, or text search, and original
// tool results via the "result:" prefix. When mem0 sync is enabled,
// cross-session search is available via the "cross:" prefix. stats, when
// non-nil, accumulates per-session recall counts for sufficiency
// telemetry.
func NewRecallTool(svc notebook.Service, messages message.Service, cfg *config.ConfigStore, mem0Server string, syncMem0 bool, stats *csync.Map[string, notebook.Stats]) fantasy.AgentTool {
	rc := &recallContext{
		svc:        svc,
		messages:   messages,
		cfg:        cfg,
		mem0Server: mem0Server,
		syncMem0:   syncMem0,
		stats:      stats,
	}
	return fantasy.NewAgentTool(
		RecallToolName,
		string(recallDescription),
		func(ctx context.Context, params RecallParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Query == "" {
				return fantasy.NewTextErrorResponse("query parameter is required"), nil
			}

			sessionID := getSessionID(ctx)
			slog.Debug("Notebook recall",
				"session_id", sessionID,
				"query", params.Query,
			)

			// Cross-session search via mem0.
			if strings.HasPrefix(params.Query, "cross:") {
				rc.bump(sessionID, func(s *notebook.Stats) { s.CrossRecalls++ })
				if !rc.syncMem0 || rc.cfg == nil || rc.mem0Server == "" {
					return fantasy.NewTextErrorResponse("cross-session search requires mem0 sync to be enabled"), nil
				}
				query := strings.TrimPrefix(params.Query, "cross:")
				result, err := notebook.SearchMem0(ctx, rc.cfg, rc.mem0Server, query)
				if err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("mem0 search failed: %v", err)), nil
				}
				if result == "" {
					rc.bump(sessionID, func(s *notebook.Stats) { s.EmptyRecalls++ })
					return fantasy.NewTextResponse("No cross-session memories found."), nil
				}
				return fantasy.NewTextResponse(result), nil
			}

			if sessionID == "" {
				return fantasy.NewTextErrorResponse("session ID is required for recall"), nil
			}

			// Original tool result recall — used by superseded-result
			// stubs and error digests to fetch pre-edit snapshots and
			// full failure output.
			if strings.HasPrefix(params.Query, "result:") {
				rc.bump(sessionID, func(s *notebook.Stats) { s.ResultRecalls++ })
				return rc.recallToolResult(ctx, sessionID, strings.TrimPrefix(params.Query, "result:"))
			}

			rc.bump(sessionID, func(s *notebook.Stats) { s.EntryRecalls++ })
			entries, err := searchNotebook(ctx, rc.svc, sessionID, params.Query)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to search notebook: %v", err)), nil
			}

			if len(entries) == 0 {
				rc.bump(sessionID, func(s *notebook.Stats) { s.EmptyRecalls++ })
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

// recallToolResult returns the original persisted content of the tool
// result identified by tool call ID. The result is never rewritten by
// the supersession stubber, so this always yields the pre-stub full
// content — including failed results kept for diagnosis.
func (rc *recallContext) recallToolResult(ctx context.Context, sessionID, toolCallID string) (fantasy.ToolResponse, error) {
	if toolCallID == "" {
		return fantasy.NewTextErrorResponse("result: query requires a tool call ID, e.g. result:toolu_01ABC"), nil
	}
	if rc.messages == nil {
		return fantasy.NewTextErrorResponse("result recall requires the message service"), nil
	}
	msgs, err := rc.messages.List(ctx, sessionID)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to list messages: %v", err)), nil
	}
	for _, m := range msgs {
		for _, tr := range m.ToolResults() {
			if tr.ToolCallID != toolCallID {
				continue
			}
			var sb strings.Builder
			fmt.Fprintf(&sb, "## Tool result %s — %s", tr.ToolCallID, tr.Name)
			if tr.IsError {
				sb.WriteString(" (error)")
			}
			sb.WriteString("\n")
			// Cap like bash output: a stubbed 200KB result shouldn't
			// re-import wholesale what stubbing removed. The marker
			// labels this a recall-time cut, distinct from any cap the
			// stored content already carries.
			sb.WriteString(tools.TruncateHeadTail(tr.Content, tools.MaxOutputLength, tools.TruncatedAtRecall))
			return fantasy.NewTextResponse(sb.String()), nil
		}
	}
	rc.bump(sessionID, func(s *notebook.Stats) { s.EmptyRecalls++ })
	return fantasy.NewTextResponse(fmt.Sprintf("No tool result found for %q in this session.", toolCallID)), nil
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
	case strings.HasPrefix(query, "segment:"):
		// Segment-grained lookup matching coverage granularity:
		// segment:<turn>.<segment>.
		var turn, seg int64
		if n, _ := fmt.Sscanf(strings.TrimPrefix(query, "segment:"), "%d.%d", &turn, &seg); n != 2 {
			return nil, fmt.Errorf("invalid segment: query %q: expected segment:<turn>.<segment>", query)
		}
		return svc.GetByTurnSegment(ctx, sessionID, turn, seg)
	case query == "command" || query == "decision" || query == "file_read" || query == "file_edit" || query == "exploration":
		return svc.SearchByEventType(ctx, sessionID, query)
	default:
		return svc.SearchByText(ctx, sessionID, query)
	}
}
