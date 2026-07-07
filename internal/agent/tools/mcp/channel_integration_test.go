package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// injectTransport wraps a transport and hands the established connection back to
// the test, so a test can push a raw server-initiated notification onto it (the
// go-sdk server has no public API for a custom notification method).
type injectTransport struct {
	inner  mcp.Transport
	connCh chan mcp.Connection
}

func (t *injectTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	c, err := t.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	t.connCh <- c
	return c, nil
}

// TestChannelEndToEnd exercises the whole client-side channel path against a
// real go-sdk server over an in-memory transport: a server that declares the
// claude/channel capability and a reply tool, a client wrapped with the channel
// interceptor, real capability detection + gate opt-in (as createSession does),
// two-way reply-tool discovery, and a real server-pushed notification flowing
// through the transport wrapper into an EventChannelMessage.
func TestChannelEndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sub := broker.Subscribe(ctx)

	serverT, clientT := mcp.NewInMemoryTransports()

	server := mcp.NewServer(
		&mcp.Implementation{Name: "chan", Version: "0.0.1"},
		&mcp.ServerOptions{
			Instructions: "reply with the reply tool",
			Capabilities: &mcp.ServerCapabilities{
				Experimental: map[string]any{channelCapability: map[string]any{}},
			},
		},
	)
	type replyIn struct {
		ChatID string `json:"chat_id"`
		Text   string `json:"text"`
	}
	mcp.AddTool(
		server,
		&mcp.Tool{Name: "reply", Description: "send a message back over this channel"},
		func(context.Context, *mcp.CallToolRequest, replyIn) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "sent"}}}, nil, nil
		},
	)

	inject := &injectTransport{inner: serverT, connCh: make(chan mcp.Connection, 1)}
	serverSession, err := server.Connect(ctx, inject, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer serverSession.Close()

	gate := &atomic.Bool{}
	client := mcp.NewClient(&mcp.Implementation{Name: "crush", Version: "test"}, nil)
	session, err := client.Connect(ctx, &channelTransport{inner: clientT, name: "chan", gate: gate}, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer session.Close()

	// Capability detection + opt-in gate flip, mirroring createSession.
	if !hasChannelCapability(session.InitializeResult()) {
		t.Fatal("expected claude/channel capability to be detected from the handshake")
	}
	if channelEnabled([]string{"chan"}, "chan") && hasChannelCapability(session.InitializeResult()) {
		gate.Store(true)
	}

	// Two-way reply: the reply tool is exposed as an ordinary MCP tool.
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	hasReply := false
	for _, tl := range tools.Tools {
		if tl.Name == "reply" {
			hasReply = true
		}
	}
	if !hasReply {
		t.Error("expected the reply tool to be exposed to the agent")
	}

	// Push a real channel notification server -> client through the transport.
	serverConn := <-inject.connCh
	params, err := json.Marshal(channelParams{
		Content: "build failed",
		Meta:    map[string]string{"severity": "high"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := serverConn.Write(ctx, &jsonrpc.Request{
		Method: channelNotificationMethod,
		Params: params,
	}); err != nil {
		t.Fatalf("write notification: %v", err)
	}

	select {
	case ev := <-sub:
		if ev.Payload.Type != EventChannelMessage {
			t.Fatalf("event type = %v, want EventChannelMessage", ev.Payload.Type)
		}
		msg := ev.Payload.ChannelMessage
		if !strings.Contains(msg, `source="chan"`) ||
			!strings.Contains(msg, "build failed") ||
			!strings.Contains(msg, `severity="high"`) {
			t.Errorf("unexpected channel message: %q", msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the channel event")
	}
}
