package notebook

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
)

// Mem0Sync provides cross-session memory sync via an MCP server
// (typically mem0). It is optional and only active when
// NotebookSyncMem0 is enabled in config.
type Mem0Sync struct {
	cfg        *config.ConfigStore
	serverName string
	sessionID  string
}

// NewMem0Sync creates a mem0 sync helper for the given session.
func NewMem0Sync(cfg *config.ConfigStore, serverName, sessionID string) *Mem0Sync {
	return &Mem0Sync{
		cfg:        cfg,
		serverName: serverName,
		sessionID:  sessionID,
	}
}

// SyncEntries syncs notebook entries to mem0 by calling the MCP
// server's add_memory tool. Each entry is stored with its tags as
// metadata so cross-session search can filter by tag.
func (m *Mem0Sync) SyncEntries(ctx context.Context, entries []Entry) {
	if m == nil || m.cfg == nil || m.serverName == "" || len(entries) == 0 {
		return
	}
	for _, entry := range entries {
		text := entry.EntryTextFull
		if text == "" {
			text = entry.EntryText
		}
		metadata := map[string]any{
			"session_id":   entry.SessionID,
			"turn_number":  entry.TurnNumber,
			"event_number": entry.EventNumber,
			"event_type":   entry.EventType,
			"tags":         entry.Tags,
			"compression":  entry.CompressionLevel,
		}
		args := map[string]any{
			"text":     fmt.Sprintf("## %s\n%s", entry.Title, text),
			"agent_id": "crush",
			"metadata": metadata,
			"infer":    false,
		}
		input, _ := json.Marshal(args)
		_, err := mcp.RunTool(ctx, m.cfg, m.serverName, "add_memory", string(input))
		if err != nil {
			slog.Warn("Failed to sync entry to mem0",
				"error", err,
				"server", m.serverName,
				"turn", entry.TurnNumber,
			)
		}
	}
}

// mem0SearchMaxTokens is the maximum token count for mem0 search
// results returned to the model. Results beyond this are truncated
// to avoid prompt inflation.
const mem0SearchMaxTokens = 4000

// SearchMem0 searches cross-session memories via the MCP server's
// search_memories tool. Returns the raw text results, truncated to
// mem0SearchMaxTokens to avoid prompt inflation.
func SearchMem0(ctx context.Context, cfg *config.ConfigStore, serverName, query string) (string, error) {
	if cfg == nil || serverName == "" || strings.TrimSpace(query) == "" {
		return "", nil
	}
	args := map[string]any{
		"query":    query,
		"agent_id": "crush",
		"top_k":    10,
	}
	input, _ := json.Marshal(args)
	result, err := mcp.RunTool(ctx, cfg, serverName, "search_memories", string(input))
	if err != nil {
		return "", fmt.Errorf("mem0 search failed: %w", err)
	}
	return truncateTextToTokens(result.Content, mem0SearchMaxTokens), nil
}

// truncateTextToTokens truncates text to approximately maxTokens by
// using a rough 4-chars-per-token estimate.
func truncateTextToTokens(text string, maxTokens int) string {
	maxChars := maxTokens * 4
	if len(text) <= maxChars {
		return text
	}
	return text[:maxChars] + "\n\n[Results truncated to stay within token budget]"
}
