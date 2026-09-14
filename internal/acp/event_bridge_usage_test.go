package acp

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/session"
	acp "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/require"
)

// usageStubSession returns fixed token usage for a session id.
type usageStubSession struct {
	*stubSessionService
	promptTokens     int64
	completionTokens int64
	cost             float64
}

func (s *usageStubSession) Get(_ context.Context, id string) (session.Session, error) {
	return session.Session{
		ID:               id,
		PromptTokens:     s.promptTokens,
		CompletionTokens: s.completionTokens,
		Cost:             s.cost,
	}, nil
}

// captureClient records every session update the agent streamed to it.
type captureClient struct {
	*testClient
	ch chan acp.SessionUpdate
}

func (c *captureClient) SessionUpdate(_ context.Context, n acp.SessionNotification) error {
	c.ch <- n.Update
	return nil
}

// TestEventBridge_UsageUpdate verifies the bridge emits a usage_update carrying
// the session's token usage and cost after a run completes (O13).
func TestEventBridge_UsageUpdate(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	app := &app.App{Sessions: &usageStubSession{
		stubSessionService: &stubSessionService{},
		promptTokens:       10,
		completionTokens:   32,
		cost:               0.42,
	}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(app, log, c2aR, a2cW)
	b := newEventBridge(srv.conn, app, log)

	cc := &captureClient{ch: make(chan acp.SessionUpdate, 1)}
	client := acp.NewClientSideConnection(cc, c2aW, a2cR)
	go func() {
		_, _ = client.Initialize(t.Context(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	}()

	b.usageUpdate(t.Context(), "sid")

	select {
	case u := <-cc.ch:
		require.NotNil(t, u.UsageUpdate)
		require.Equal(t, 42, u.UsageUpdate.Used)
		require.NotNil(t, u.UsageUpdate.Cost)
		require.Equal(t, 0.42, u.UsageUpdate.Cost.Amount)
	case <-time.After(2 * time.Second):
		t.Fatal("no usage update received")
	}
}
