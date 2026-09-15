package acp

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/message"
	acp "github.com/coder/acp-go-sdk"
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
