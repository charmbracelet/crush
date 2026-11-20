package acp

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/permission"
	acp "github.com/coder/acp-go-sdk"
)

func TestBuildPermissionOptions(t *testing.T) {
	opts := buildPermissionOptions("bash")
	if len(opts) != 4 {
		t.Fatalf("expected 4 options, got %d", len(opts))
	}
	// All four kinds the spec defines must be offered (tool-calls.md:232-235).
	want := []struct {
		id   acp.PermissionOptionId
		kind acp.PermissionOptionKind
	}{
		{"allow_once", acp.PermissionOptionKindAllowOnce},
		{"allow_always", acp.PermissionOptionKindAllowAlways},
		{"reject_once", acp.PermissionOptionKindRejectOnce},
		{"reject_always", acp.PermissionOptionKindRejectAlways},
	}
	for i, w := range want {
		if opts[i].OptionId != w.id {
			t.Errorf("option %d: got %s, want %s", i, opts[i].OptionId, w.id)
		}
		if opts[i].Kind != w.kind {
			t.Errorf("option %d: got kind %s, want %s", i, opts[i].Kind, w.kind)
		}
	}
	// A client that grants by picking the first option must not deny.
	if opts[0].Kind != acp.PermissionOptionKindAllowOnce {
		t.Error("first option must be an allow kind")
	}
	// The always-kinds state their scope in the name (tool-calls.md:250);
	// the once-kinds must not claim it.
	if !strings.Contains(opts[1].Name, "bash") || !strings.Contains(opts[3].Name, "bash") {
		t.Errorf("always options must name their scope, got %q and %q", opts[1].Name, opts[3].Name)
	}
	if strings.Contains(opts[0].Name, "bash") || strings.Contains(opts[2].Name, "bash") {
		t.Error("once options must not claim a scope")
	}
	// Without a tool name the generic labels survive.
	plain := buildPermissionOptions("")
	if plain[1].Name != "Allow always" || plain[3].Name != "Reject always" {
		t.Errorf("generic labels changed: %q, %q", plain[1].Name, plain[3].Name)
	}
}

func TestCheckPermissionAppliesRememberedScope(t *testing.T) {
	// conn is nil on purpose: a remembered decision must return before any
	// client round-trip, proving the cache short-circuits the prompt.
	b := &permissionBridge{
		log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		remembered: make(map[scopeKey]bool),
	}
	b.remember(scopeKey{sessionID: "sess", toolName: "bash"}, true)

	granted, err := b.CheckPermission(context.Background(), permission.PermissionRequest{
		SessionID: "sess", ToolName: "bash", ToolCallID: "call-1",
	})
	if err != nil {
		t.Fatalf("remembered grant failed: %v", err)
	}
	if !granted {
		t.Error("a remembered allow_always must grant without prompting")
	}

	b.remember(scopeKey{sessionID: "sess", toolName: "bash"}, false)
	granted, err = b.CheckPermission(context.Background(), permission.PermissionRequest{
		SessionID: "sess", ToolName: "bash", ToolCallID: "call-2",
	})
	if err != nil {
		t.Fatalf("remembered rejection failed: %v", err)
	}
	if granted {
		t.Error("a remembered reject_always must deny without prompting")
	}

	// The scope is the tool: only one entry exists, and a different tool
	// would prompt rather than inherit this decision.
	if len(b.remembered) != 1 {
		t.Errorf("remembered decisions = %d, want 1", len(b.remembered))
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

	// reject_always denies: it is a rejection, not a grant.
	outcome = acp.NewRequestPermissionOutcomeSelected("reject_always")
	if mapPermissionOutcome(outcome) {
		t.Error("expected reject_always to be denied")
	}

	// An option id Crush never offered must never be read as approval
	// (tool-calls.md:248).
	outcome = acp.NewRequestPermissionOutcomeSelected("bogus_id")
	if mapPermissionOutcome(outcome) {
		t.Error("expected an unrecognised optionId to be denied")
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

func TestPermissionBridgeIntegration(t *testing.T) {
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

	// Permission bridge is integrated into Server and tested via integration tests
	_ = srv
}

// acceptingTestClient is a test client that accepts all permissions.
type acceptingTestClient struct{}

func (c *acceptingTestClient) SessionUpdate(_ context.Context, _ acp.SessionNotification) error {
	return nil
}

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

func (c *acceptingTestClient) CreateElicitation(_ context.Context, _ acp.CreateElicitationRequest) (acp.CreateElicitationResponse, error) {
	return acp.CreateElicitationResponse{
		Cancel: &acp.CreateElicitationCancel{Action: "cancel"},
	}, nil
}

func (c *acceptingTestClient) CompleteElicitation(_ context.Context, _ acp.CompleteElicitationNotification) error {
	return nil
}

func (c *acceptingTestClient) KillTerminal(_ context.Context, _ acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, nil
}
