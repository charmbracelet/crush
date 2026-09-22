package acp

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	acp "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/require"
)

// stubMessages serves a fixed message list for LoadSession replay tests.
type stubMessages struct {
	msgs []message.Message
}

func (s *stubMessages) Subscribe(_ context.Context) <-chan pubsub.Event[message.Message] {
	return make(chan pubsub.Event[message.Message])
}

func (s *stubMessages) Create(_ context.Context, _ string, _ message.CreateMessageParams) (message.Message, error) {
	return message.Message{}, nil
}

func (s *stubMessages) Update(_ context.Context, _ message.Message) error { return nil }

func (s *stubMessages) Get(_ context.Context, _ string) (message.Message, error) {
	return message.Message{}, nil
}

func (s *stubMessages) List(_ context.Context, _ string) ([]message.Message, error) {
	return s.msgs, nil
}

func (s *stubMessages) ListFromSummary(_ context.Context, _, _ string) ([]message.Message, error) {
	return s.msgs, nil
}

func (s *stubMessages) ListUserMessages(_ context.Context, _ string) ([]message.Message, error) {
	return nil, nil
}

func (s *stubMessages) ListAllUserMessages(_ context.Context) ([]message.Message, error) {
	return nil, nil
}

func (s *stubMessages) GetLastAssistantMessage(_ context.Context, _ string) (message.Message, error) {
	return message.Message{}, nil
}

func (s *stubMessages) Delete(_ context.Context, _ string) error { return nil }

func (s *stubMessages) DeleteSessionMessages(_ context.Context, _ string) error { return nil }

func (s *stubMessages) Flush(_ context.Context, _ string) error { return nil }

func (s *stubMessages) FlushAll(_ context.Context) error { return nil }

// TestLoadSessionReplaysFullHistory verifies session/load streams the entire
// conversation: user text, agent thought, tool calls with kinds and rawInput,
// agent text, and tool results with terminal status and content (R7).
func TestLoadSessionReplaysFullHistory(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	app := &app.App{
		Sessions: &stubSessionService{},
		Messages: &stubMessages{msgs: []message.Message{
			{
				ID: "m1", SessionID: "sess-1", Role: message.User,
				Parts: []message.ContentPart{message.TextContent{Text: "question"}},
			},
			{
				ID: "m2", SessionID: "sess-1", Role: message.Assistant,
				Parts: []message.ContentPart{
					message.ReasoningContent{Thinking: "pondering", FinishedAt: 1},
					message.ToolCall{ID: "call-1", Name: "view", Input: `{"path":"/a/b.go"}`},
					message.TextContent{Text: "answer"},
				},
			},
			{
				ID: "m3", SessionID: "sess-1", Role: message.Tool,
				Parts: []message.ContentPart{
					message.ToolResult{ToolCallID: "call-1", Name: "view", Content: "file body"},
				},
			},
		}},
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(app, log, c2aR, a2cW)
	_ = srv

	cc := &captureClient{ch: make(chan acp.SessionUpdate, 16)}
	client := acp.NewClientSideConnection(cc, c2aW, a2cR)
	_, err := client.Initialize(t.Context(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, err := client.LoadSession(t.Context(), acp.LoadSessionRequest{
			SessionId:  "sess-1",
			Cwd:        "/workspace",
			McpServers: []acp.McpServer{},
		})
		done <- err
	}()

	var updates []acp.SessionUpdate
	for {
		select {
		case u := <-cc.ch:
			updates = append(updates, u)
		case err := <-done:
			require.NoError(t, err)
			require.NotEmpty(t, updates)

			var sawUserChunk, sawThought, sawAgentChunk bool
			var start, result acp.SessionUpdate
			for _, u := range updates {
				switch {
				case u.UserMessageChunk != nil:
					require.Equal(t, "question", u.UserMessageChunk.Content.Text.Text)
					sawUserChunk = true
				case u.AgentThoughtChunk != nil:
					require.Equal(t, "pondering", u.AgentThoughtChunk.Content.Text.Text)
					sawThought = true
				case u.AgentMessageChunk != nil:
					require.Equal(t, "answer", u.AgentMessageChunk.Content.Text.Text)
					sawAgentChunk = true
				case u.ToolCall != nil:
					start = u
				case u.ToolCallUpdate != nil:
					result = u
				}
			}
			require.True(t, sawUserChunk, "user_message_chunk replayed")
			require.True(t, sawThought, "agent_thought_chunk replayed")
			require.True(t, sawAgentChunk, "agent_message_chunk replayed")
			require.NotNil(t, start.ToolCall)
			require.Equal(t, acp.ToolCallId("call-1"), start.ToolCall.ToolCallId)
			require.Equal(t, acp.ToolKindRead, start.ToolCall.Kind)
			require.Contains(t, start.ToolCall.Title, "/a/b.go")
			require.NotNil(t, result.ToolCallUpdate)
			require.NotNil(t, result.ToolCallUpdate.Status)
			require.Equal(t, acp.ToolCallStatusCompleted, *result.ToolCallUpdate.Status)
			require.Len(t, result.ToolCallUpdate.Content, 1)
			require.Equal(t, "file body", result.ToolCallUpdate.Content[0].Content.Content.Text.Text)
			return
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for LoadSession replay")
		}
	}
}

// TestLoadSessionReplayToolResultWithoutStart verifies a tool result whose
// start update is missing from history (e.g. compacted sessions) still renders.
func TestLoadSessionReplayToolResultWithoutStart(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	app := &app.App{
		Sessions: &stubSessionService{},
		Messages: &stubMessages{msgs: []message.Message{
			{
				ID: "m1", SessionID: "sess-2", Role: message.Tool,
				Parts: []message.ContentPart{
					message.ToolResult{ToolCallID: "call-x", Name: "bash", Content: "out", IsError: true},
				},
			},
		}},
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(app, log, c2aR, a2cW)
	_ = srv

	cc := &captureClient{ch: make(chan acp.SessionUpdate, 8)}
	client := acp.NewClientSideConnection(cc, c2aW, a2cR)
	_, err := client.Initialize(t.Context(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, err := client.LoadSession(t.Context(), acp.LoadSessionRequest{
			SessionId:  "sess-2",
			Cwd:        "/workspace",
			McpServers: []acp.McpServer{},
		})
		done <- err
	}()

	var updates []acp.SessionUpdate
	for {
		select {
		case u := <-cc.ch:
			updates = append(updates, u)
		case err := <-done:
			require.NoError(t, err)
			var sawStart, sawResult bool
			for _, u := range updates {
				if u.ToolCall != nil && u.ToolCall.ToolCallId == "call-x" {
					sawStart = true
				}
				if u.ToolCallUpdate != nil && u.ToolCallUpdate.ToolCallId == "call-x" {
					sawResult = true
					require.NotNil(t, u.ToolCallUpdate.Status)
					require.Equal(t, acp.ToolCallStatusFailed, *u.ToolCallUpdate.Status)
				}
			}
			require.True(t, sawStart, "synthesized tool_call start")
			require.True(t, sawResult, "tool_call_update with failed status")
			return
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for LoadSession replay")
		}
	}
}
