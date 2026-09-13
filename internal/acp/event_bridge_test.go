package acp

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/app"
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
