package acp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/app"
	acp "github.com/coder/acp-go-sdk"
)

func TestServerStartEndToEnd(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	app := &app.App{Sessions: &stubSessionService{}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(app, log, c2aR, a2cW)

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- srv.Start(ctx)
	}()

	clientDone := make(chan struct{})
	var clientErr error
	go func() {
		defer close(clientDone)
		c := acp.NewClientSideConnection(&testClient{}, c2aW, a2cR)
		ctx := context.Background()

		// Initialize
		_, err := c.Initialize(ctx, acp.InitializeRequest{
			ProtocolVersion:    acp.ProtocolVersionNumber,
			ClientCapabilities: acp.ClientCapabilities{},
		})
		if err != nil {
			clientErr = err
			return
		}
		// Create a session
		ns, err := c.NewSession(ctx, acp.NewSessionRequest{Cwd: "/tmp", McpServers: []acp.McpServer{}})
		if err != nil {
			clientErr = err
			return
		}
		if ns.SessionId == "" {
			clientErr = errors.New("empty session id")
			return
		}
		// Cancel shouldn't hang.
		if err := c.Cancel(ctx, acp.CancelNotification{SessionId: ns.SessionId}); err != nil {
			clientErr = err
		}
		// Close the write end to signal disconnect.
		if cerr := c2aW.Close(); cerr != nil {
			clientErr = cerr
		}
	}()

	select {
	case <-clientDone:
	case <-time.After(3 * time.Second):
		t.Fatal("client goroutine did not finish within timeout")
	}
	if clientErr != nil {
		t.Fatalf("client error: %v", clientErr)
	}

	srvDone := make(chan error, 1)
	go func() { srvDone <- <-done }()

	select {
	case err := <-srvDone:
		if err != nil && !strings.Contains(err.Error(), "client disconnected") {
			t.Fatalf("server start error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not exit after client disconnect")
	}
}

// testClient implements acp.Client for testing.
type testClient struct{}

func (c *testClient) SessionUpdate(_ context.Context, _ acp.SessionNotification) error { return nil }

func (c *testClient) RequestPermission(_ context.Context, _ acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	return acp.RequestPermissionResponse{}, nil
}

func (c *testClient) ReadTextFile(_ context.Context, _ acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, nil
}

func (c *testClient) WriteTextFile(_ context.Context, _ acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return acp.WriteTextFileResponse{}, nil
}

func (c *testClient) CreateTerminal(_ context.Context, _ acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, nil
}

func (c *testClient) TerminalOutput(_ context.Context, _ acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, nil
}

func (c *testClient) WaitForTerminalExit(_ context.Context, _ acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, nil
}

func (c *testClient) ReleaseTerminal(_ context.Context, _ acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, nil
}

func (c *testClient) KillTerminal(_ context.Context, _ acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, nil
}
