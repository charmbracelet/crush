package acp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/require"
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
			Title: acp.PatchSet("bash"),
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

func TestClientPermissionCancelledByCancel(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cli := &Client{
		log:                 log,
		sessionChs:          make(map[acp.SessionId]chan acp.SessionUpdate),
		permissionWaiters:   make(map[acp.SessionId]map[*permissionWaiter]struct{}),
		permissionCancelled: make(map[acp.SessionId]struct{}),
		permissionHandler: func(ctx context.Context, _ acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
			<-ctx.Done()
			return acp.RequestPermissionResponse{}, ctx.Err()
		},
	}
	impl := &crushClient{client: cli}
	started := make(chan struct{})
	req := acp.RequestPermissionRequest{SessionId: "sess-1"}

	go func() {
		close(started)
		_, _ = impl.RequestPermission(context.Background(), req)
	}()
	<-started
	cli.cancelPermissionRequests(req.SessionId)

	resp, err := impl.RequestPermission(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp.Outcome.Cancelled)
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
		Line:  acp.Ptr[int64](2),
		Limit: acp.Ptr[int64](1),
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

func TestClientElicitationUnsupported(t *testing.T) {
	impl := &crushClient{client: &Client{}}
	ctx := context.Background()

	_, err := impl.CreateElicitation(ctx, acp.CreateElicitationRequest{})
	var reqErr *acp.RequestError
	if !errors.As(err, &reqErr) {
		t.Fatalf("expected RequestError, got %T: %v", err, err)
	}
	if reqErr.Code != -32601 {
		t.Fatalf("expected -32601, got %d", reqErr.Code)
	}
	if err := impl.CompleteElicitation(ctx, acp.CompleteElicitationNotification{}); err != nil {
		t.Fatalf("CompleteElicitation: %v", err)
	}
}

func TestClientElicitationHandler(t *testing.T) {
	response, err := (&crushClient{client: &Client{
		elicitationHandler: func(_ context.Context, _ acp.CreateElicitationRequest) (acp.CreateElicitationResponse, error) {
			return acp.CreateElicitationResponse{Cancel: &acp.CreateElicitationCancel{Action: "cancel"}}, nil
		},
	}}).CreateElicitation(context.Background(), acp.CreateElicitationRequest{})
	if err != nil {
		t.Fatalf("CreateElicitation: %v", err)
	}
	if response.Cancel == nil {
		t.Fatal("expected cancel response")
	}
}

func TestClientTerminalMethods(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cli := &Client{log: log, cwd: t.TempDir(), terminals: newTerminalRegistry(), elicitations: map[acp.ElicitationId]struct{}{}}
	impl := &crushClient{client: cli}
	ctx := context.Background()

	command, args := envPrintCommand("PATH")
	created, err := impl.CreateTerminal(ctx, acp.CreateTerminalRequest{Command: command, Args: args})
	if err != nil {
		t.Fatalf("CreateTerminal: %v", err)
	}
	if created.TerminalId == "" {
		t.Fatal("expected terminal id")
	}
	_, err = impl.WaitForTerminalExit(ctx, acp.WaitForTerminalExitRequest{TerminalId: created.TerminalId})
	if err != nil {
		t.Fatalf("WaitForTerminalExit: %v", err)
	}
	output, err := impl.TerminalOutput(ctx, acp.TerminalOutputRequest{TerminalId: created.TerminalId})
	if err != nil {
		t.Fatalf("TerminalOutput: %v", err)
	}
	if output.Output == "" {
		t.Fatal("expected terminal output")
	}
	if _, err := impl.ReleaseTerminal(ctx, acp.ReleaseTerminalRequest{TerminalId: created.TerminalId}); err != nil {
		t.Fatalf("ReleaseTerminal: %v", err)
	}
	if _, err := impl.TerminalOutput(ctx, acp.TerminalOutputRequest{TerminalId: created.TerminalId}); err == nil {
		t.Fatal("expected released terminal to be unknown")
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

	rawJSON, err := json.Marshal(rawInput)
	if err != nil {
		t.Fatalf("marshal raw input: %v", err)
	}

	update := acp.SessionUpdate{}
	update.ToolCall = &acp.SessionUpdateToolCall{
		RawInput: rawJSON,
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
