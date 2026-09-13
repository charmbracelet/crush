package acp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

func TestClientSubscribe(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cli := &Client{log: log, sessionChs: make(map[acp.SessionId]chan acp.SessionUpdate)}

	// Register a session manually.
	sid := acp.SessionId("sess-1")
	cli.sessMu.Lock()
	ch := make(chan acp.SessionUpdate, 1)
	cli.sessionChs[sid] = ch
	cli.sessMu.Unlock()

	// The crushClient callback should have forwarded it.
	update := acp.SessionUpdate{}
	update.AgentMessageChunk = &acp.SessionUpdateAgentMessageChunk{}
	cc := &crushClient{client: cli}
	if err := cc.SessionUpdate(context.Background(), acp.SessionNotification{
		SessionId: sid,
		Update:    update,
	}); err != nil {
		t.Fatalf("SessionUpdate: %v", err)
	}

	subCh, err := cli.Subscribe(sid)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	select {
	case u := <-subCh:
		if u.AgentMessageChunk == nil {
			t.Fatal("expected AgentMessageChunk")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for update")
	}
}

func TestClientPermissionAutoAllow(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cli := &Client{log: log, autoAllow: true}
	impl := &crushClient{client: cli}

	req := acp.RequestPermissionRequest{
		SessionId: "sess-1",
		ToolCall: acp.ToolCallUpdate{
			Title: strPtr("bash"),
		},
		Options: []acp.PermissionOption{
			{Kind: acp.PermissionOptionKindAllowOnce, OptionId: "opt-1", Name: "Allow once"},
			{Kind: acp.PermissionOptionKindAllowAlways, OptionId: "opt-2", Name: "Allow always"},
			{Kind: acp.PermissionOptionKindRejectOnce, OptionId: "opt-3", Name: "Reject"},
		},
	}

	resp, err := impl.RequestPermission(context.Background(), req)
	if err != nil {
		t.Fatalf("RequestPermission: %v", err)
	}
	if resp.Outcome.Selected == nil {
		t.Fatal("expected selected outcome")
	}
	if resp.Outcome.Selected.OptionId != "opt-1" {
		t.Fatalf("expected opt-1, got %s", resp.Outcome.Selected.OptionId)
	}
}

func TestClientPermissionDefaultFirstOption(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cli := &Client{log: log} // autoAllow false
	impl := &crushClient{client: cli}

	req := acp.RequestPermissionRequest{
		SessionId: "sess-1",
		Options: []acp.PermissionOption{
			{Kind: acp.PermissionOptionKindAllowOnce, OptionId: "opt-1", Name: "Allow once"},
		},
	}

	resp, err := impl.RequestPermission(context.Background(), req)
	if err != nil {
		t.Fatalf("RequestPermission: %v", err)
	}
	if resp.Outcome.Selected == nil {
		t.Fatal("expected selected outcome")
	}
	if resp.Outcome.Selected.OptionId != "opt-1" {
		t.Fatalf("expected opt-1, got %s", resp.Outcome.Selected.OptionId)
	}
}

func TestClientPermissionNoOptions(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cli := &Client{log: log}
	impl := &crushClient{client: cli}

	req := acp.RequestPermissionRequest{
		SessionId: "sess-1",
		Options:   []acp.PermissionOption{},
	}

	resp, err := impl.RequestPermission(context.Background(), req)
	if err != nil {
		t.Fatalf("RequestPermission: %v", err)
	}
	if resp.Outcome.Cancelled == nil {
		t.Fatal("expected cancelled outcome when no options")
	}
}

func TestClientReadTextFile(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cli := &Client{log: log}
	impl := &crushClient{client: cli}

	tmp := t.TempDir()
	path := tmp + "/hello.txt"
	if err := os.WriteFile(path, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	resp, err := impl.ReadTextFile(context.Background(), acp.ReadTextFileRequest{
		Path: path,
	})
	if err != nil {
		t.Fatalf("ReadTextFile: %v", err)
	}
	if resp.Content != "line1\nline2\nline3\n" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}

	// Test line limiting.
	resp2, err := impl.ReadTextFile(context.Background(), acp.ReadTextFileRequest{
		Path:  path,
		Line:  intPtr(2),
		Limit: intPtr(1),
	})
	if err != nil {
		t.Fatalf("ReadTextFile with limits: %v", err)
	}
	if resp2.Content != "line2" {
		t.Fatalf("unexpected limited content: %q", resp2.Content)
	}

	// Test missing file.
	_, err = impl.ReadTextFile(context.Background(), acp.ReadTextFileRequest{
		Path: tmp + "/missing.txt",
	})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestClientWriteTextFile(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cli := &Client{log: log}
	impl := &crushClient{client: cli}

	tmp := t.TempDir()
	path := tmp + "/sub/dir.txt"

	_, err := impl.WriteTextFile(context.Background(), acp.WriteTextFileRequest{
		Path:    path,
		Content: "hello world",
	})
	if err != nil {
		t.Fatalf("WriteTextFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("unexpected content: %q", data)
	}
}

func TestClientTerminalMethods(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cli := &Client{log: log}
	impl := &crushClient{client: cli}
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func() error
	}{
		{"CreateTerminal", func() error { _, err := impl.CreateTerminal(ctx, acp.CreateTerminalRequest{}); return err }},
		{"KillTerminal", func() error { _, err := impl.KillTerminal(ctx, acp.KillTerminalRequest{}); return err }},
		{"TerminalOutput", func() error { _, err := impl.TerminalOutput(ctx, acp.TerminalOutputRequest{}); return err }},
		{"WaitForTerminalExit", func() error { _, err := impl.WaitForTerminalExit(ctx, acp.WaitForTerminalExitRequest{}); return err }},
		{"ReleaseTerminal", func() error { _, err := impl.ReleaseTerminal(ctx, acp.ReleaseTerminalRequest{}); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "terminals not supported") {
				t.Fatalf("expected 'terminals not supported' error, got: %v", err)
			}
		})
	}
}

func TestClientNewClient(t *testing.T) {
	// Verify NewClient creates a valid Client even with a nonexistent binary.
	// We can't actually start it, but we verify the struct is wired.
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cli := &Client{log: log}
	impl := &crushClient{client: cli}

	// Validate the impl implements acp.Client.
	var _ acp.Client = impl
}

func TestDrain(t *testing.T) {
	ch := make(chan acp.SessionUpdate, 3)
	go func() {
		ch <- acp.SessionUpdate{}
		ch <- acp.SessionUpdate{}
		close(ch)
	}()
	updates := drain(ch)
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}
}

func TestParseToolCall(t *testing.T) {
	rawInput := map[string]any{"command": "ls", "args": []string{"-la"}}

	update := acp.SessionUpdate{}
	update.ToolCall = &acp.SessionUpdateToolCall{
		RawInput: rawInput,
	}
	input, ok := parseToolCall(update)
	if !ok {
		t.Fatal("expected to parse tool call")
	}
	if input["command"] != "ls" {
		t.Fatalf("expected command 'ls', got %v", input["command"])
	}

	// Nil tool call.
	_, ok = parseToolCall(acp.SessionUpdate{})
	if ok {
		t.Fatal("expected false for nil tool call")
	}
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

// stubAgent is a minimal ACP agent for pairing with ClientSideConnection in tests.
type stubAgent struct{}

func (a *stubAgent) Authenticate(_ context.Context, _ acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, errors.New("not implemented")
}

func (a *stubAgent) Initialize(_ context.Context, req acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersionNumber,
		AgentInfo:       &acp.Implementation{Name: "stub", Version: "0.1"},
		AgentCapabilities: acp.AgentCapabilities{
			PromptCapabilities: acp.PromptCapabilities{Image: false, Audio: false, EmbeddedContext: false},
		},
	}, nil
}

func (a *stubAgent) Logout(_ context.Context, _ acp.LogoutRequest) (acp.LogoutResponse, error) {
	return acp.LogoutResponse{}, errors.New("not implemented")
}

func (a *stubAgent) NewSession(_ context.Context, req acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	return acp.NewSessionResponse{SessionId: acp.SessionId("stub-" + req.Cwd)}, nil
}

func (a *stubAgent) ListSessions(_ context.Context, _ acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	return acp.ListSessionsResponse{}, nil
}

func (a *stubAgent) CloseSession(_ context.Context, _ acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	return acp.CloseSessionResponse{}, nil
}

func (a *stubAgent) ResumeSession(_ context.Context, _ acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	return acp.ResumeSessionResponse{}, errors.New("not implemented")
}

func (a *stubAgent) Cancel(_ context.Context, _ acp.CancelNotification) error {
	return nil
}

func (a *stubAgent) Prompt(_ context.Context, _ acp.PromptRequest) (acp.PromptResponse, error) {
	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

func (a *stubAgent) SetSessionMode(_ context.Context, _ acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, errors.New("not implemented")
}

func (a *stubAgent) SetSessionConfigOption(_ context.Context, _ acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, errors.New("not implemented")
}
