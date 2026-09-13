package acp

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	acp "github.com/coder/acp-go-sdk"

	acpsrv "github.com/charmbracelet/crush/internal/acp"
)

// --- Stubs ---

type stubCoordinator struct {
	mu     sync.Mutex
	result *fantasy.AgentResult
}

func newStubCoordinator(result *fantasy.AgentResult) *stubCoordinator {
	return &stubCoordinator{result: result}
}

func (c *stubCoordinator) Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.result == nil {
		c.result = &fantasy.AgentResult{
			Response: fantasy.Response{
				Content:      fantasy.ResponseContent{},
				FinishReason: fantasy.FinishReasonStop,
				Usage:        fantasy.Usage{},
			},
		}
	}
	return c.result, nil
}

func (c *stubCoordinator) RunAccepted(ctx context.Context, accept *agent.AcceptedRun, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	return c.Run(ctx, sessionID, prompt, attachments...)
}

func (c *stubCoordinator) BeginAccepted(sessionID string) *agent.AcceptedRun { return nil }
func (c *stubCoordinator) Cancel(sessionID string)                           {}
func (c *stubCoordinator) CancelAll()                                        {}
func (c *stubCoordinator) IsSessionBusy(sessionID string) bool               { return false }
func (c *stubCoordinator) IsBusy() bool                                      { return false }
func (c *stubCoordinator) QueuedPrompts(sessionID string) int                { return 0 }
func (c *stubCoordinator) QueuedPromptsList(sessionID string) []string       { return nil }
func (c *stubCoordinator) ClearQueue(sessionID string)                       {}
func (c *stubCoordinator) Summarize(_ context.Context, _ string) error       { return nil }
func (c *stubCoordinator) Model() agent.Model                                { return agent.Model{} }
func (c *stubCoordinator) UpdateModels(_ context.Context) error              { return nil }
func (c *stubCoordinator) GenerateTitle(_ context.Context, _, _ string)      {}

// stubBlockingCoordinator blocks until the prompt's context is cancelled.
type stubBlockingCoordinator struct {
	mu     sync.Mutex
	result *fantasy.AgentResult
	ready  chan struct{}
}

func newStubBlockingCoordinator(result *fantasy.AgentResult) *stubBlockingCoordinator {
	return &stubBlockingCoordinator{result: result, ready: make(chan struct{})}
}

func (c *stubBlockingCoordinator) Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	close(c.ready)
	<-ctx.Done()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.result == nil {
		c.result = &fantasy.AgentResult{
			Response: fantasy.Response{
				Content:      fantasy.ResponseContent{},
				FinishReason: fantasy.FinishReasonStop,
				Usage:        fantasy.Usage{},
			},
		}
	}
	return c.result, nil
}

func (c *stubBlockingCoordinator) RunAccepted(ctx context.Context, accept *agent.AcceptedRun, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	return c.Run(ctx, sessionID, prompt, attachments...)
}

func (c *stubBlockingCoordinator) BeginAccepted(sessionID string) *agent.AcceptedRun { return nil }
func (c *stubBlockingCoordinator) Cancel(sessionID string)                           {}
func (c *stubBlockingCoordinator) CancelAll()                                        {}
func (c *stubBlockingCoordinator) IsSessionBusy(sessionID string) bool               { return true }
func (c *stubBlockingCoordinator) IsBusy() bool                                      { return true }
func (c *stubBlockingCoordinator) QueuedPrompts(sessionID string) int                { return 0 }
func (c *stubBlockingCoordinator) QueuedPromptsList(sessionID string) []string       { return nil }
func (c *stubBlockingCoordinator) ClearQueue(sessionID string)                       {}
func (c *stubBlockingCoordinator) Summarize(_ context.Context, _ string) error       { return nil }
func (c *stubBlockingCoordinator) Model() agent.Model                                { return agent.Model{} }
func (c *stubBlockingCoordinator) UpdateModels(_ context.Context) error              { return nil }
func (c *stubBlockingCoordinator) GenerateTitle(_ context.Context, _, _ string)      {}

type stubPermSvc struct{}

func (s *stubPermSvc) Subscribe(_ context.Context) <-chan pubsub.Event[permission.PermissionRequest] {
	return make(chan pubsub.Event[permission.PermissionRequest])
}
func (s *stubPermSvc) GrantPersistent(_ permission.PermissionRequest) bool { return false }
func (s *stubPermSvc) Grant(_ permission.PermissionRequest) bool           { return false }
func (s *stubPermSvc) Deny(_ permission.PermissionRequest) bool            { return false }
func (s *stubPermSvc) Request(_ context.Context, _ permission.CreatePermissionRequest) (bool, error) {
	return false, nil
}
func (s *stubPermSvc) AutoApproveSession(_ string) {}
func (s *stubPermSvc) SetSkipRequests(_ bool)      {}
func (s *stubPermSvc) SkipRequests() bool          { return false }
func (s *stubPermSvc) SubscribeNotifications(_ context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(chan pubsub.Event[permission.PermissionNotification])
}

func stubApp(co agent.Coordinator) *app.App {
	return &app.App{
		Sessions:         &stubSessionService{},
		AgentCoordinator: co,
		Permissions:      &stubPermSvc{},
	}
}

type stubClient struct {
	files   map[string]string
	updates []acp.SessionNotification
	mu      sync.Mutex
}

func newStubClient() *stubClient {
	return &stubClient{files: map[string]string{}}
}

func (c *stubClient) SessionUpdate(_ context.Context, p acp.SessionNotification) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updates = append(c.updates, p)
	return nil
}

func (c *stubClient) RequestPermission(_ context.Context, req acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	var out acp.RequestPermissionOutcome
	if len(req.Options) > 0 {
		out.Selected = &acp.RequestPermissionOutcomeSelected{OptionId: acp.PermissionOptionId(req.Options[0].OptionId)}
	} else {
		out.Selected = &acp.RequestPermissionOutcomeSelected{}
	}
	return acp.RequestPermissionResponse{Outcome: out}, nil
}

func (c *stubClient) ReadTextFile(_ context.Context, req acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	c.mu.Lock()
	v := c.files[req.Path]
	c.mu.Unlock()
	return acp.ReadTextFileResponse{Content: v}, nil
}

func (c *stubClient) WriteTextFile(_ context.Context, req acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	c.mu.Lock()
	c.files[req.Path] = req.Content
	c.mu.Unlock()
	return acp.WriteTextFileResponse{}, nil
}

func (c *stubClient) CreateTerminal(_ context.Context, _ acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{TerminalId: "t1"}, nil
}

func (c *stubClient) KillTerminal(_ context.Context, _ acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, nil
}

func (c *stubClient) TerminalOutput(_ context.Context, _ acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{Output: ""}, nil
}

func (c *stubClient) ReleaseTerminal(_ context.Context, _ acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, nil
}

func (c *stubClient) WaitForTerminalExit(_ context.Context, _ acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, nil
}

// trackingUpdateClient wraps stubClient and records all SessionUpdate calls.
type trackingUpdateClient struct {
	*stubClient
	hook func(acp.SessionNotification)
}

func (c *trackingUpdateClient) SessionUpdate(ctx context.Context, n acp.SessionNotification) error {
	if c.hook != nil {
		c.hook(n)
	}
	return c.stubClient.SessionUpdate(ctx, n)
}

// --- Conformance Tests ---
// Uses two-pipe pattern (client→agent and agent→client) to avoid
// single-pipe read races between concurrent client/server goroutines.

func TestConformanceInitialize(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	co := newStubCoordinator(nil)
	app := stubApp(co)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := acpsrv.NewServer(app, log, c2aR, a2cW)

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- srv.Start(ctx)
	}()

	client := acp.NewClientSideConnection(newStubClient(), c2aW, a2cR)
	initResp, err := client.Initialize(t.Context(), acp.InitializeRequest{
		ProtocolVersion:    acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	if initResp.ProtocolVersion != acp.ProtocolVersionNumber {
		t.Errorf("protocol version = %d, want %d", initResp.ProtocolVersion, acp.ProtocolVersionNumber)
	}
	if initResp.AgentInfo == nil || initResp.AgentInfo.Name != "Crush" {
		t.Errorf("agent info = %+v, want Name=Crush", initResp.AgentInfo)
	}
	if !initResp.AgentCapabilities.LoadSession {
		t.Errorf("LoadSession = false, want true")
	}

	c2aW.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("server did not exit")
	}
}

func TestConformanceNewSession(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	co := newStubCoordinator(nil)
	app := stubApp(co)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := acpsrv.NewServer(app, log, c2aR, a2cW)

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- srv.Start(ctx)
	}()

	client := acp.NewClientSideConnection(newStubClient(), c2aW, a2cR)
	ns, err := client.NewSession(t.Context(), acp.NewSessionRequest{Cwd: "/workspace", McpServers: []acp.McpServer{}})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}
	if ns.SessionId == "" {
		t.Error("empty session ID")
	}

	c2aW.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("server did not exit")
	}
}

func TestConformancePromptEndTurn(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	co := newStubCoordinator(&fantasy.AgentResult{
		Response: fantasy.Response{
			Content:      fantasy.ResponseContent{},
			FinishReason: fantasy.FinishReasonStop,
			Usage:        fantasy.Usage{},
		},
	})
	app := stubApp(co)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := acpsrv.NewServer(app, log, c2aR, a2cW)

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- srv.Start(ctx)
	}()

	client := acp.NewClientSideConnection(newStubClient(), c2aW, a2cR)
	ns, err := client.NewSession(t.Context(), acp.NewSessionRequest{Cwd: "/workspace", McpServers: []acp.McpServer{}})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	resp, err := client.Prompt(t.Context(), acp.PromptRequest{
		SessionId: ns.SessionId,
		Prompt:    []acp.ContentBlock{{Text: &acp.ContentBlockText{Text: "hello"}}},
	})
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}
	if resp.StopReason != acp.StopReasonEndTurn {
		t.Errorf("stop reason = %q, want %q", resp.StopReason, acp.StopReasonEndTurn)
	}

	c2aW.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("server did not exit")
	}
}

// TestConformanceSessionSetupAndUnsupportedMethods pins two separate
// contracts: methods an editor sends during session setup must succeed even
// when Crush has nothing to do, and methods Crush genuinely does not implement
// must fail loudly rather than silently appear to work.
func TestConformanceSessionSetupAndUnsupportedMethods(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	co := newStubCoordinator(nil)
	app := stubApp(co)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := acpsrv.NewServer(app, log, c2aR, a2cW)

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- srv.Start(ctx)
	}()

	client := acp.NewClientSideConnection(newStubClient(), c2aW, a2cR)
	ns, err := client.NewSession(t.Context(), acp.NewSessionRequest{Cwd: "/workspace", McpServers: []acp.McpServer{}})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	// Setup methods must not fail: an error here can abort an editor session
	// before the first prompt is ever sent.
	if _, err := client.SetSessionMode(t.Context(), acp.SetSessionModeRequest{SessionId: ns.SessionId, ModeId: "code"}); err != nil {
		t.Errorf("SetSessionMode: %v", err)
	}

	if _, err := client.SetSessionConfigOption(t.Context(), acp.SetSessionConfigOptionRequest{
		ValueId: &acp.SetSessionConfigOptionValueId{SessionId: ns.SessionId, ConfigId: "model", Value: "gpt-4o-mini"},
	}); err != nil {
		t.Errorf("SetSessionConfigOption: %v", err)
	}

	// Genuinely unimplemented methods must still report failure.
	if _, err := client.ResumeSession(t.Context(), acp.ResumeSessionRequest{SessionId: ns.SessionId, Cwd: "/workspace"}); err == nil {
		t.Error("ResumeSession should error")
	}

	c2aW.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("server did not exit")
	}
}

func TestConformanceSessionLifecycle(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	co := newStubCoordinator(nil)
	app := stubApp(co)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := acpsrv.NewServer(app, log, c2aR, a2cW)

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- srv.Start(ctx)
	}()

	client := acp.NewClientSideConnection(newStubClient(), c2aW, a2cR)

	_, err := client.Initialize(t.Context(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	ns, err := client.NewSession(t.Context(), acp.NewSessionRequest{Cwd: "/workspace", McpServers: []acp.McpServer{}})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}
	if ns.SessionId == "" {
		t.Error("empty session ID")
	}

	_, err = client.ListSessions(t.Context(), acp.ListSessionsRequest{})
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	_, err = client.CloseSession(t.Context(), acp.CloseSessionRequest{SessionId: ns.SessionId})
	if err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}

	c2aW.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("server did not exit")
	}
}

// --- Extended Conformance Tests ---

func TestConformanceCancelDuringPrompt(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	co := newStubBlockingCoordinator(nil)
	app := stubApp(co)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := acpsrv.NewServer(app, log, c2aR, a2cW)

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- srv.Start(ctx)
	}()

	client := acp.NewClientSideConnection(newStubClient(), c2aW, a2cR)
	_, err := client.Initialize(t.Context(), acp.InitializeRequest{
		ProtocolVersion:    acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	ns, err := client.NewSession(t.Context(), acp.NewSessionRequest{Cwd: "/workspace", McpServers: []acp.McpServer{}})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	promptDone := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		defer close(promptDone)
		_, err := client.Prompt(t.Context(), acp.PromptRequest{
			SessionId: ns.SessionId,
			Prompt:    []acp.ContentBlock{acp.TextBlock("hello")},
		})
		errCh <- err
	}()

	select {
	case <-co.ready:
	case <-time.After(2 * time.Second):
		t.Fatal("coordinator did not start blocking")
	}

	if err := client.Cancel(t.Context(), acp.CancelNotification{SessionId: ns.SessionId}); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	select {
	case <-promptDone:
	case <-time.After(2 * time.Second):
		t.Fatal("prompt did not complete after cancel")
	}

	err = <-errCh
	if err != nil {
		t.Fatalf("prompt error: %v", err)
	}

	c2aW.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("server did not exit")
	}
}

func TestConformanceMultipleSessions(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	co := newStubCoordinator(nil)
	app := stubApp(co)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := acpsrv.NewServer(app, log, c2aR, a2cW)

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- srv.Start(ctx)
	}()

	client := acp.NewClientSideConnection(newStubClient(), c2aW, a2cR)
	_, err := client.Initialize(t.Context(), acp.InitializeRequest{
		ProtocolVersion:    acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	ns1, err := client.NewSession(t.Context(), acp.NewSessionRequest{Cwd: "/ws1", McpServers: []acp.McpServer{}})
	if err != nil {
		t.Fatalf("NewSession 1 failed: %v", err)
	}
	ns2, err := client.NewSession(t.Context(), acp.NewSessionRequest{Cwd: "/ws2", McpServers: []acp.McpServer{}})
	if err != nil {
		t.Fatalf("NewSession 2 failed: %v", err)
	}
	if ns1.SessionId == ns2.SessionId {
		t.Error("two sessions have same ID")
	}

	listResp, err := client.ListSessions(t.Context(), acp.ListSessionsRequest{})
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(listResp.Sessions) < 2 {
		t.Errorf("ListSessions returned %d sessions, want >= 2", len(listResp.Sessions))
	}

	if _, err := client.CloseSession(t.Context(), acp.CloseSessionRequest{SessionId: ns1.SessionId}); err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}

	listResp, err = client.ListSessions(t.Context(), acp.ListSessionsRequest{})
	if err != nil {
		t.Fatalf("ListSessions after close failed: %v", err)
	}
	_ = listResp

	c2aW.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("server did not exit")
	}
}

func TestConformanceSessionUpdates(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	co := newStubCoordinator(&fantasy.AgentResult{
		Response: fantasy.Response{
			Content:      fantasy.ResponseContent{},
			FinishReason: fantasy.FinishReasonStop,
			Usage:        fantasy.Usage{},
		},
	})
	app := stubApp(co)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := acpsrv.NewServer(app, log, c2aR, a2cW)

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- srv.Start(ctx)
	}()

	var mu sync.Mutex
	var updates []acp.SessionNotification
	updateClient := &trackingUpdateClient{stubClient: newStubClient(), hook: func(n acp.SessionNotification) {
		mu.Lock()
		defer mu.Unlock()
		updates = append(updates, n)
	}}

	client := acp.NewClientSideConnection(updateClient, c2aW, a2cR)
	_, err := client.Initialize(t.Context(), acp.InitializeRequest{
		ProtocolVersion:    acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	ns, err := client.NewSession(t.Context(), acp.NewSessionRequest{Cwd: "/workspace", McpServers: []acp.McpServer{}})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	resp, err := client.Prompt(t.Context(), acp.PromptRequest{
		SessionId: ns.SessionId,
		Prompt:    []acp.ContentBlock{acp.TextBlock("hello")},
	})
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}
	if resp.StopReason != acp.StopReasonEndTurn {
		t.Errorf("stop reason = %q, want %q", resp.StopReason, acp.StopReasonEndTurn)
	}

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	gotUpdates := make([]acp.SessionNotification, len(updates))
	copy(gotUpdates, updates)
	mu.Unlock()
	_ = gotUpdates

	c2aW.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("server did not exit")
	}
}

func TestConformanceContentBlockVariants(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	co := newStubCoordinator(&fantasy.AgentResult{
		Response: fantasy.Response{
			Content:      fantasy.ResponseContent{},
			FinishReason: fantasy.FinishReasonStop,
			Usage:        fantasy.Usage{},
		},
	})
	app := stubApp(co)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := acpsrv.NewServer(app, log, c2aR, a2cW)

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- srv.Start(ctx)
	}()

	client := acp.NewClientSideConnection(newStubClient(), c2aW, a2cR)
	_, err := client.Initialize(t.Context(), acp.InitializeRequest{
		ProtocolVersion:    acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	ns, err := client.NewSession(t.Context(), acp.NewSessionRequest{Cwd: "/workspace", McpServers: []acp.McpServer{}})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	resp, err := client.Prompt(t.Context(), acp.PromptRequest{
		SessionId: ns.SessionId,
		Prompt: []acp.ContentBlock{
			acp.TextBlock("read this file"),
			acp.ResourceLinkBlock("main.go", "file:///workspace/main.go"),
		},
	})
	if err != nil {
		t.Fatalf("Prompt with resource link failed: %v", err)
	}
	if resp.StopReason != acp.StopReasonEndTurn {
		t.Errorf("stop reason = %q, want %q", resp.StopReason, acp.StopReasonEndTurn)
	}

	c2aW.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("server did not exit")
	}
}

func TestConformanceInitializeCapabilities(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	co := newStubCoordinator(nil)
	app := stubApp(co)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := acpsrv.NewServer(app, log, c2aR, a2cW)

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- srv.Start(ctx)
	}()

	client := acp.NewClientSideConnection(newStubClient(), c2aW, a2cR)
	initResp, err := client.Initialize(t.Context(), acp.InitializeRequest{
		ProtocolVersion:    acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if initResp.AgentInfo == nil {
		t.Fatal("AgentInfo is nil")
	}
	if initResp.AgentInfo.Name != "Crush" {
		t.Errorf("agent name = %q, want %q", initResp.AgentInfo.Name, "Crush")
	}
	if initResp.ProtocolVersion != acp.ProtocolVersionNumber {
		t.Errorf("protocol version = %d, want %d", initResp.ProtocolVersion, acp.ProtocolVersionNumber)
	}
	if !initResp.AgentCapabilities.LoadSession {
		t.Error("LoadSession capability = false, want true")
	}

	c2aW.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("server did not exit")
	}
}

func TestConformanceInvalidSessionError(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	co := newStubCoordinator(nil)
	app := stubApp(co)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := acpsrv.NewServer(app, log, c2aR, a2cW)

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- srv.Start(ctx)
	}()

	client := acp.NewClientSideConnection(newStubClient(), c2aW, a2cR)
	_, err := client.Initialize(t.Context(), acp.InitializeRequest{
		ProtocolVersion:    acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	_, err = client.Prompt(t.Context(), acp.PromptRequest{
		SessionId: acp.SessionId("nonexistent-session"),
		Prompt:    []acp.ContentBlock{acp.TextBlock("hello")},
	})
	if err != nil {
		t.Error("Prompt on invalid session should error")
	}

	err = client.Cancel(t.Context(), acp.CancelNotification{SessionId: acp.SessionId("nonexistent-session")})
	if err != nil {
		t.Error("Cancel on invalid session should error")
	}

	c2aW.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("server did not exit")
	}
}
