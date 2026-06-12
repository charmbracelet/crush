package compact_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/compact"
	"github.com/charmbracelet/crush/internal/message"
	_ "modernc.org/sqlite"
)

// dbPath returns the path to the Crush database. Override with
// CRUSH_DB_PATH for testing against a specific database.
func dbPath() string {
	if p := os.Getenv("CRUSH_DB_PATH"); p != "" {
		return p
	}
	return "/Users/kierank/code/charm/crush/.crush/crush.db"
}

// loadSessionMessages loads all messages for a session from the Crush DB.
func loadSessionMessages(t *testing.T, sessionID string) []message.Message {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath()+"?mode=ro")
	if err != nil {
		t.Skipf("Cannot open Crush DB (set CRUSH_DB_PATH): %v", err)
	}
	defer db.Close()

	rows, err := db.QueryContext(context.Background(),
		`SELECT id, session_id, role, parts, model, provider, created_at, updated_at, is_summary_message
		 FROM messages WHERE session_id = ? ORDER BY created_at ASC`, sessionID)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	var msgs []message.Message
	for rows.Next() {
		var m message.Message
		var partsJSON string
		var model, provider sql.NullString
		var isSummary int64
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &partsJSON, &model, &provider, &m.CreatedAt, &m.UpdatedAt, &isSummary); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		m.Model = model.String
		m.Provider = provider.String
		m.IsSummaryMessage = isSummary != 0

		var parts []json.RawMessage
		if err := json.Unmarshal([]byte(partsJSON), &parts); err != nil {
			continue // skip malformed
		}
		for _, raw := range parts {
			part, err := unmarshalPart(raw)
			if err != nil {
				continue
			}
			m.Parts = append(m.Parts, part)
		}
		msgs = append(msgs, m)
	}
	return msgs
}

func unmarshalPart(raw json.RawMessage) (message.ContentPart, error) {
	// Parts are stored as {"type":"...","data":{...}} tagged unions.
	var wrapper struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, err
	}
	switch wrapper.Type {
	case "text":
		var tc message.TextContent
		if err := json.Unmarshal(wrapper.Data, &tc); err != nil {
			return nil, err
		}
		return tc, nil
	case "tool_result":
		var tr message.ToolResult
		if err := json.Unmarshal(wrapper.Data, &tr); err != nil {
			return nil, err
		}
		return tr, nil
	case "tool_call":
		var tcall message.ToolCall
		if err := json.Unmarshal(wrapper.Data, &tcall); err != nil {
			return nil, err
		}
		return tcall, nil
	case "reasoning":
		var rc message.ReasoningContent
		if err := json.Unmarshal(wrapper.Data, &rc); err != nil {
			return nil, err
		}
		return rc, nil
	case "finish":
		var f message.Finish
		if err := json.Unmarshal(wrapper.Data, &f); err != nil {
			return nil, err
		}
		return f, nil
	case "binary":
		var bc message.BinaryContent
		if err := json.Unmarshal(wrapper.Data, &bc); err != nil {
			return nil, err
		}
		return bc, nil
	case "image_url":
		var iuc message.ImageURLContent
		if err := json.Unmarshal(wrapper.Data, &iuc); err != nil {
			return nil, err
		}
		return iuc, nil
	default:
		return nil, fmt.Errorf("unknown part type: %s", wrapper.Type)
	}
}

// TestBenchmarkRealSessions runs algorithmic compaction on real sessions
// and reports timing + output size. Run with:
//
//	go test ./internal/compact/ -run TestBenchmarkRealSessions -v -count=1
func TestBenchmarkRealSessions(t *testing.T) {
	sessions := []struct {
		id    string
		title string
	}{
		{"d9951f28-f0c5-4b3d-8999-8eb807d261df", "Question Tool (5018 msgs)"},
		{"0c0d71cc-5118-4856-a59f-2a6c643e0a42", "Embedded terminal (2186 msgs)"},
		{"e6909a55-d17a-4bb5-85f0-b51280612947", "jj mega merge (1581 msgs)"},
		{"175d8448-600c-44db-b2b2-507faafee77e", "PR Review 2562 (933 msgs)"},
		{"d2eaab5c-2314-4f18-9c45-78cbbd0d9807", "PR Review Command (794 msgs)"},
	}

	for _, s := range sessions {
		t.Run(s.title, func(t *testing.T) {
			msgs := loadSessionMessages(t, s.id)
			if len(msgs) == 0 {
				t.Skip("No messages loaded")
			}

			start := time.Now()
			result := compact.Compact(compact.Input{Messages: msgs})
			elapsed := time.Since(start)

			t.Logf("Messages:     %d", len(msgs))
			t.Logf("Duration:     %v", elapsed)
			t.Logf("Output chars: %d", len(result))
			t.Logf("Output lines: %d", lineCount(result))
			t.Logf("Chars/msg:    %.1f", float64(len(result))/float64(len(msgs)))

			// Write output to file for manual inspection.
			outFile := fmt.Sprintf("/tmp/compact_%s.txt", s.id[:8])
			if err := os.WriteFile(outFile, []byte(result), 0o644); err != nil {
				t.Logf("Failed to write output: %v", err)
			} else {
				t.Logf("Output written to: %s", outFile)
			}
		})
	}
}

// BenchmarkCompact measures throughput on a real session.
func BenchmarkCompact(b *testing.B) {
	sessionID := "d9951f28-f0c5-4b3d-8999-8eb807d261df" // largest session
	db, err := sql.Open("sqlite", dbPath()+"?mode=ro")
	if err != nil {
		b.Skipf("Cannot open Crush DB: %v", err)
	}
	defer db.Close()

	rows, err := db.QueryContext(context.Background(),
		`SELECT id, session_id, role, parts, model, provider, created_at, updated_at, is_summary_message
		 FROM messages WHERE session_id = ? ORDER BY created_at ASC`, sessionID)
	if err != nil {
		b.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	var msgs []message.Message
	for rows.Next() {
		var m message.Message
		var partsJSON string
		var model, provider sql.NullString
		var isSummary int64
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &partsJSON, &model, &provider, &m.CreatedAt, &m.UpdatedAt, &isSummary); err != nil {
			b.Fatalf("Scan failed: %v", err)
		}
		m.Model = model.String
		m.Provider = provider.String
		m.IsSummaryMessage = isSummary != 0
		var parts []json.RawMessage
		if err := json.Unmarshal([]byte(partsJSON), &parts); err != nil {
			continue
		}
		for _, raw := range parts {
			part, err := unmarshalPart(raw)
			if err != nil {
				continue
			}
			m.Parts = append(m.Parts, part)
		}
		msgs = append(msgs, m)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compact.Compact(compact.Input{Messages: msgs})
	}
	b.ReportMetric(float64(len(msgs)), "msgs")
}

func lineCount(s string) int {
	n := 1
	for _, c := range s {
		if c == '\n' {
			n++
		}
	}
	return n
}
