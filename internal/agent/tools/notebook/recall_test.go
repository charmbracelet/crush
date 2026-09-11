package notebook

import (
	"context"
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// newRecallTestEnv builds a message service on a scratch DB and returns
// it with a session ID.
func newRecallTestEnv(t *testing.T) (message.Service, string) {
	t.Helper()
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	sess, err := sessions.Create(t.Context(), "test")
	require.NoError(t, err)
	return message.NewService(q), sess.ID
}

func runRecall(t *testing.T, tool fantasy.AgentTool, ctx context.Context, query string) fantasy.ToolResponse {
	t.Helper()
	input, err := json.Marshal(RecallParams{Query: query})
	require.NoError(t, err)
	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call",
		Name:  RecallToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	return resp
}

func TestRecallToolResult(t *testing.T) {
	t.Parallel()

	svc, sessionID := newRecallTestEnv(t)
	tool := NewRecallTool(nil, svc, nil, "", false)
	ctx := context.WithValue(t.Context(), tools.SessionIDContextKey, sessionID)

	original := "compile failed:\nmain.go:12: undefined: foo\nmain.go:13: missing return"
	_, err := svc.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "tc-1", Name: "bash", Input: `{"command":"go build ."}`, Finished: true},
		},
	})
	require.NoError(t, err)
	_, err = svc.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "tc-1", Name: "bash", Content: original, IsError: true},
		},
	})
	require.NoError(t, err)

	t.Run("returns original full content", func(t *testing.T) {
		t.Parallel()
		resp := runRecall(t, tool, ctx, "result:tc-1")
		require.False(t, resp.IsError)
		require.Contains(t, resp.Content, "(error)")
		require.Contains(t, resp.Content, original)
	})

	t.Run("unknown call id is a clean miss", func(t *testing.T) {
		t.Parallel()
		resp := runRecall(t, tool, ctx, "result:nope")
		require.False(t, resp.IsError)
		require.Contains(t, resp.Content, "No tool result found")
	})

	t.Run("empty id is an error", func(t *testing.T) {
		t.Parallel()
		resp := runRecall(t, tool, ctx, "result:")
		require.True(t, resp.IsError)
	})
}
