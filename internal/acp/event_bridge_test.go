package acp

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	notify "github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/message"
	acp "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/require"
)

func TestEventBridge_StartAndStop(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	app := &app.App{Sessions: &stubSessionService{}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(app, log, c2aR, a2cW)

	ctx, cancel := context.WithCancel(context.Background())
	srv.StartEventBridge(ctx)
	time.Sleep(10 * time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond)
}

func TestEventBridge_ClientDisconnect(t *testing.T) {
	// Two pipes, not one. A single io.Pipe used for both directions makes the
	// client and server read each other's writes, which races their scanners and
	// hangs the test intermittently.
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	app := &app.App{Sessions: &stubSessionService{}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(app, log, c2aR, a2cW)

	ctx, cancel := context.WithCancel(context.Background())
	srv.StartEventBridge(ctx)
	defer cancel()

	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		c := acp.NewClientSideConnection(&testClient{}, c2aW, a2cR)
		_, _ = c.Initialize(ctx, acp.InitializeRequest{
			ProtocolVersion:    acp.ProtocolVersionNumber,
			ClientCapabilities: acp.ClientCapabilities{},
		})
		_ = c2aW.Close()
	}()

	select {
	case <-clientDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for client")
	}
}

// TestEventBridge_ToolResultStreaming verifies a tool-role message's results
// are surfaced on the matching tool_call_update with output content (O9), so a
// tool-heavy turn no longer appears to stall after the first batch.
func TestEventBridge_ToolResultStreaming(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	app := &app.App{Sessions: &stubSessionService{}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(app, log, c2aR, a2cW)
	b := newEventBridge(srv.conn, app, log)

	cc := &captureClient{ch: make(chan acp.SessionUpdate, 4)}
	client := acp.NewClientSideConnection(cc, c2aW, a2cR)
	go func() {
		_, _ = client.Initialize(context.Background(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	}()

	toolMsg := message.Message{
		ID:        "msg-tool",
		SessionID: "sess-1",
		Role:      message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{
				ToolCallID: "call-1",
				Name:       "bash",
				Content:    "hello world",
			},
			message.ToolResult{
				ToolCallID: "call-2",
				Name:       "ls",
				Content:    "boom",
				IsError:    true,
			},
		},
	}

	b.handleMessage(context.Background(), toolMsg)

	var sawContent map[string]string
	for i := 0; i < 2; i++ {
		var u acp.SessionUpdate
		select {
		case u = <-cc.ch:
		case <-time.After(2 * time.Second):
			t.Fatalf("no tool_call_update %d received", i)
		}
		if u.ToolCallUpdate == nil {
			t.Fatalf("expected tool_call_update, got %+v", u)
		}
		if sawContent == nil {
			sawContent = make(map[string]string)
		}
		for _, c := range u.ToolCallUpdate.Content {
			if c.Content != nil && c.Content.Content.Text != nil {
				sawContent[string(u.ToolCallUpdate.ToolCallId)] = c.Content.Content.Text.Text
			}
		}
	}
	if got := sawContent["call-1"]; got != "hello world" {
		t.Errorf("call-1 content = %q, want %q", got, "hello world")
	}
	if got := sawContent["call-2"]; got != "Error: boom" {
		t.Errorf("call-2 content = %q, want %q", got, "Error: boom")
	}

	// Re-snapshot must not re-emit identical content.
	b.handleMessage(context.Background(), toolMsg)
	select {
	case u := <-cc.ch:
		if u.ToolCallUpdate != nil {
			t.Errorf("duplicate tool_call_update emitted on re-snapshot: %+v", u)
		}
	case <-time.After(200 * time.Millisecond):
	}
}

// TestEventBridge_ToolResultFailedStatus verifies an errored tool result is
// reported with the failed status (S10), not just error-prefixed content.
func TestEventBridge_ToolResultFailedStatus(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	app := &app.App{Sessions: &stubSessionService{}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(app, log, c2aR, a2cW)
	b := newEventBridge(srv.conn, app, log)

	cc := &captureClient{ch: make(chan acp.SessionUpdate, 2)}
	client := acp.NewClientSideConnection(cc, c2aW, a2cR)
	go func() {
		_, _ = client.Initialize(context.Background(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	}()

	toolMsg := message.Message{
		ID:        "msg-tool-err",
		SessionID: "sess-1",
		Role:      message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "call-err", Name: "bash", Content: "boom", IsError: true},
			message.ToolResult{ToolCallID: "call-ok", Name: "view", Content: "fine"},
		},
	}
	b.handleMessage(context.Background(), toolMsg)

	for i := 0; i < 2; i++ {
		var u acp.SessionUpdate
		select {
		case u = <-cc.ch:
		case <-time.After(2 * time.Second):
			t.Fatalf("no tool_call_update %d received", i)
		}
		require.NotNil(t, u.ToolCallUpdate)
		require.NotNil(t, u.ToolCallUpdate.Status, "terminal status must be reported")
		switch string(u.ToolCallUpdate.ToolCallId) {
		case "call-err":
			require.Equal(t, acp.ToolCallStatusFailed, *u.ToolCallUpdate.Status)
		case "call-ok":
			require.Equal(t, acp.ToolCallStatusCompleted, *u.ToolCallUpdate.Status)
		}
	}
}

// TestEventBridge_ToolCallEnrichment verifies a tool_call start carries a
// human-readable title, the tool kind, file locations, and rawInput (S10).
func TestEventBridge_ToolCallEnrichment(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	app := &app.App{Sessions: &stubSessionService{}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(app, log, c2aR, a2cW)
	b := newEventBridge(srv.conn, app, log)

	cc := &captureClient{ch: make(chan acp.SessionUpdate, 2)}
	client := acp.NewClientSideConnection(cc, c2aW, a2cR)
	go func() {
		_, _ = client.Initialize(context.Background(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	}()

	assistantMsg := message.Message{
		ID:        "msg-a",
		SessionID: "sess-1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "call-9", Name: "view", Input: `{"path":"/tmp/x.go","limit":10}`},
		},
	}
	b.handleMessage(context.Background(), assistantMsg)

	var u acp.SessionUpdate
	select {
	case u = <-cc.ch:
	case <-time.After(2 * time.Second):
		t.Fatal("no tool_call start received")
	}
	require.NotNil(t, u.ToolCall)
	require.Equal(t, acp.ToolCallId("call-9"), u.ToolCall.ToolCallId)
	require.Equal(t, acp.ToolKindRead, u.ToolCall.Kind)
	require.Contains(t, u.ToolCall.Title, "/tmp/x.go")
	require.Equal(t, acp.ToolCallStatusInProgress, u.ToolCall.Status)
	require.NotEmpty(t, u.ToolCall.Locations)
	require.Equal(t, "/tmp/x.go", u.ToolCall.Locations[0].Path)
	require.NotNil(t, u.ToolCall.RawInput)
	require.Contains(t, u.ToolCall.Title, "View")
}

// TestEventBridge_PromptMetaEcho verifies the _meta object captured from a
// session/prompt request is echoed on outgoing session/update notifications
// for that turn only (S7).
func TestEventBridge_PromptMetaEcho(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	app := &app.App{Sessions: &stubSessionService{}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(app, log, c2aR, a2cW)
	b := newEventBridge(srv.conn, app, log)

	b.SetPromptMeta("sess-1", map[string]any{"traceparent": "tp"})

	n := b.note("sess-1", acp.UpdateAgentMessageText("hi"))
	require.Equal(t, "tp", n.Meta["traceparent"])

	n = b.note("sess-2", acp.UpdateAgentMessageText("hi"))
	require.Empty(t, n.Meta)

	// Run completion clears the recorded meta.
	b.SetPromptMeta("sess-1", map[string]any{"k": "v"})
	cc := &captureClient{ch: make(chan acp.SessionUpdate, 8)}
	client := acp.NewClientSideConnection(cc, c2aW, a2cR)
	go func() {
		_, _ = client.Initialize(context.Background(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	}()
	b.handleRunComplete(context.Background(), notify.RunComplete{SessionID: "sess-1"})
	n = b.note("sess-1", acp.UpdateAgentMessageText("hi"))
	require.Empty(t, n.Meta)
}

func TestSplitDelta_Spaces(t *testing.T) {
	chunks := splitDelta("alpha bravo charlie delta echo foxtrot golf hotel")
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks for 8 words, got %d", len(chunks))
	}
	if chunks[0] != "alpha bravo charlie delta" {
		t.Errorf("chunk 0 = %q, want %q", chunks[0], "alpha bravo charlie delta")
	}
	if chunks[1] != " echo foxtrot golf hotel" {
		t.Errorf("chunk 1 = %q, want %q", chunks[1], " echo foxtrot golf hotel")
	}
}

func TestSplitDelta_ConcatenationPreservesSpaces(t *testing.T) {
	input := "to analyze the codebase. Let me start by exploring the project structure"
	chunks := splitDelta(input)
	joined := ""
	for _, c := range chunks {
		joined += c
	}
	if joined != input {
		t.Errorf("concatenated chunks = %q, want %q", joined, input)
	}
}

func TestSplitDelta_ContinuationDelta(t *testing.T) {
	delta := " Let me start by exploring the project structure"
	chunks := splitDelta(delta)
	joined := ""
	for _, c := range chunks {
		joined += c
	}
	if joined != delta {
		t.Errorf("concatenated chunks = %q, want %q", joined, delta)
	}
	if chunks[0] != " Let me start by" {
		t.Errorf("chunk 0 = %q, want %q", chunks[0], " Let me start by")
	}
}

func TestSplitDelta_MixedWhitespace(t *testing.T) {
	input := "hello   world\t\n  foo bar"
	chunks := splitDelta(input)
	joined := ""
	for _, c := range chunks {
		joined += c
	}
	if joined != input {
		t.Errorf("concatenated chunks = %q, want %q", joined, input)
	}
}

func TestSplitDelta_Newlines(t *testing.T) {
	chunks := splitDelta("alpha\nbravo\ncharlie\ndelta\necho\nfoxtrot\ngolf\nhotel")
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks for 8 newline-separated words, got %d", len(chunks))
	}
}

func TestSplitDelta_Empty(t *testing.T) {
	if chunks := splitDelta(""); chunks != nil {
		t.Errorf("expected nil for empty input, got %v", chunks)
	}
}

func TestSplitDelta_SingleWord(t *testing.T) {
	chunks := splitDelta("hello")
	if len(chunks) != 1 || chunks[0] != "hello" {
		t.Errorf("expected [hello], got %v", chunks)
	}
}
