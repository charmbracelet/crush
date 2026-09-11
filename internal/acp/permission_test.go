package acp

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/permission"
)

func TestBuildPermissionOptions(t *testing.T) {
	opts := buildPermissionOptions("execute")
	if len(opts) != 3 {
		t.Fatalf("expected 3 options, got %d", len(opts))
	}
	if opts[0].OptionId != "allow_once" {
		t.Errorf("expected first option allow_once, got %s", opts[0].OptionId)
	}
	if opts[1].OptionId != "allow_always" {
		t.Errorf("expected second option allow_always, got %s", opts[1].OptionId)
	}
	if opts[2].OptionId != "reject_once" {
		t.Errorf("expected third option reject_once, got %s", opts[2].OptionId)
	}
}

func TestMapPermissionOutcome(t *testing.T) {
	// Test selected allow_once
	outcome := acp.NewRequestPermissionOutcomeSelected("allow_once")
	if !mapPermissionOutcome(outcome) {
		t.Error("expected allow_once to be granted")
	}

	// Test selected allow_always
	outcome = acp.NewRequestPermissionOutcomeSelected("allow_always")
	if !mapPermissionOutcome(outcome) {
		t.Error("expected allow_always to be granted")
	}

	// Test selected reject_once
	outcome = acp.NewRequestPermissionOutcomeSelected("reject_once")
	if mapPermissionOutcome(outcome) {
		t.Error("expected reject_once to be denied")
	}

	// Test cancelled
	outcome = acp.NewRequestPermissionOutcomeCancelled()
	if mapPermissionOutcome(outcome) {
		t.Error("expected cancelled to be denied")
	}
}

func TestIsAllowAlways(t *testing.T) {
	// allow_always should be persistent
	outcome := acp.NewRequestPermissionOutcomeSelected("allow_always")
	if !isAllowAlways(outcome) {
		t.Error("expected allow_always to be persistent")
	}

	// allow_once should not be persistent
	outcome = acp.NewRequestPermissionOutcomeSelected("allow_once")
	if isAllowAlways(outcome) {
		t.Error("expected allow_once to not be persistent")
	}
}

func TestPermissionBridge_CheckPermission(t *testing.T) {
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()

	app := &app.App{Sessions: &stubSessionService{}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(app, log, r, w)

	// Create a client that accepts all permissions
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		c := acp.NewClientSideConnection(&acceptingTestClient{}, w, r)
		ctx := context.Background()

		_, err := c.Initialize(ctx, acp.InitializeRequest{
			ProtocolVersion:    acp.ProtocolVersionNumber,
			ClientCapabilities: acp.ClientCapabilities{},
		})
		if err != nil {
			t.Logf("initialize error: %v", err)
			return
		}
		w.Close()
	}()

	// Give client time to connect
	time.Sleep(50 * time.Millisecond)

	// Now test the permission bridge directly
	req := permission.PermissionRequest{
		ID:          "test-id",
		SessionID:   "session-1",
		ToolCallID:  "tool-call-1",
		ToolName:    "bash",
		Description: "run command",
		Action:      "execute",
	}

	// The bridge uses the connection which will be closed by client
	// This tests the flow even with early disconnect
	srv.permb.CheckPermission(context.Background(), req)
}

// acceptingTestClient is a test client that accepts all permissions.
type acceptingTestClient struct{}

func (c *acceptingTestClient) SessionUpdate(_ context.Context, _ acp.SessionNotification) error { return nil }
func (c *acceptingTestClient) RequestPermission(_ context.Context, _ acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	return acp.RequestPermissionResponse{
		Outcome: acp.NewRequestPermissionOutcomeSelected("allow_once"),
	}, nil
}
func (c *acceptingTestClient) ReadTextFile(_ context.Context, _ acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, nil
}
func (c *acceptingTestClient) WriteTextFile(_ context.Context, _ acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return acp.WriteTextFileResponse{}, nil
}
func (c *acceptingTestClient) CreateTerminal(_ context.Context, _ acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, nil
}
func (c *acceptingTestClient) TerminalOutput(_ context.Context, _ acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, nil
}
func (c *acceptingTestClient) WaitForTerminalExit(_ context.Context, _ acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, nil
}
func (c *acceptingTestClient) ReleaseTerminal(_ context.Context, _ acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, nil
}
func (c *acceptingTestClient) KillTerminal(_ context.Context, _ acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, nil
}
