package acp

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"github.com/charmbracelet/crush/internal/app"
)

func TestEventBridge_StartAndStop(t *testing.T) {
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()

	app := &app.App{Sessions: &stubSessionService{}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(app, log, r, w)

	// Start event bridge
	ctx, cancel := context.WithCancel(context.Background())
	srv.StartEventBridge(ctx)

	// Give it time to start
	time.Sleep(10 * time.Millisecond)

	// Stop it
	cancel()
	time.Sleep(10 * time.Millisecond)

	// Should not panic
}

func TestEventBridge_ClientDisconnect(t *testing.T) {
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()

	app := &app.App{Sessions: &stubSessionService{}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(app, log, r, w)

	// Start event bridge
	ctx, cancel := context.WithCancel(context.Background())
	srv.StartEventBridge(ctx)
	defer cancel()

	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		c := acp.NewClientSideConnection(&testClient{}, w, r)
		_, _ = c.Initialize(ctx, acp.InitializeRequest{
			ProtocolVersion:    acp.ProtocolVersionNumber,
			ClientCapabilities: acp.ClientCapabilities{},
		})
		w.Close()
	}()

	select {
	case <-clientDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for client")
	}
}
