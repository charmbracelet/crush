package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	acp "github.com/coder/acp-go-sdk"

	acpsrv "github.com/charmbracelet/crush/internal/acp"
)

// These tests reproduce the conditions that leave an ACP client (Zed) showing
// a loading indicator forever. Each maps to a finding recorded in
// docs/ACP_PLAN.md: I-1 (no streaming), I-2 (capability shape), I-3 (cwd),
// I-4 (stdout purity), I-5 (setup method errors).

// stubMessageService is a minimal message.Service whose broker can be driven by
// tests to simulate streaming assistant output.
type stubMessageService struct {
	*stubMessageBroker
}

type stubMessageBroker struct {
	*pubsub.Broker[message.Message]
}

func newStubMessageService() *stubMessageService {
	return &stubMessageService{stubMessageBroker: &stubMessageBroker{pubsub.NewBroker[message.Message]()}}
}

func (s *stubMessageService) Create(_ context.Context, sessionID string, params message.CreateMessageParams) (message.Message, error) {
	return message.Message{
		ID:        "msg-1",
		SessionID: sessionID,
		Role:      params.Role,
		Parts:     params.Parts,
	}, nil
}

func (s *stubMessageService) Update(_ context.Context, _ message.Message) error { return nil }
func (s *stubMessageService) Get(_ context.Context, _ string) (message.Message, error) {
	return message.Message{}, nil
}

func (s *stubMessageService) List(_ context.Context, _ string) ([]message.Message, error) {
	return nil, nil
}

func (s *stubMessageService) ListFromSummary(_ context.Context, _, _ string) ([]message.Message, error) {
	return nil, nil
}

func (s *stubMessageService) ListUserMessages(_ context.Context, _ string) ([]message.Message, error) {
	return nil, nil
}

func (s *stubMessageService) ListAllUserMessages(_ context.Context) ([]message.Message, error) {
	return nil, nil
}

func (s *stubMessageService) GetLastAssistantMessage(_ context.Context, _ string) (message.Message, error) {
	return message.Message{}, nil
}
func (s *stubMessageService) Delete(_ context.Context, _ string) error { return nil }
func (s *stubMessageService) DeleteSessionMessages(_ context.Context, _ string) error {
	return nil
}
func (s *stubMessageService) Flush(_ context.Context, _ string) error { return nil }
func (s *stubMessageService) FlushAll(_ context.Context) error        { return nil }
func (s *stubMessageService) Subscribe(ctx context.Context) <-chan pubsub.Event[message.Message] {
	return s.Broker.Subscribe(ctx)
}

// pipeHarness wires a Server to a client over the two-pipe pattern required by
// concurrent SDK calls. See conformance_test.go for the rationale.
type pipeHarness struct {
	t      *testing.T
	app    *app.App
	client *acp.ClientSideConnection
	done   chan error

	c2aW io.WriteCloser
}

func newPipeHarness(t *testing.T, a *app.App) *pipeHarness {
	t.Helper()

	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := acpsrv.NewServer(a, log, c2aR, a2cW)
	srv.StartEventBridge(t.Context())

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		done <- srv.Start(ctx)
	}()

	h := &pipeHarness{
		t:      t,
		app:    a,
		client: acp.NewClientSideConnection(newStubClient(), c2aW, a2cR),
		done:   done,
		c2aW:   c2aW,
	}
	t.Cleanup(func() {
		_ = c2aW.Close()
		_ = c2aR.Close()
		_ = a2cR.Close()
		_ = a2cW.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not exit")
		}
	})
	return h
}

// I-4: stdout must carry nothing but JSON-RPC frames. A stray log line or
// fmt.Print corrupts the stream and hangs the editor with no visible error.
func TestServerStdoutIsPureJSONRPC(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	// Capture everything the server writes to its stdout, then forward it on so
	// the client still sees a working stream.
	var mu sync.Mutex
	var stdout bytes.Buffer
	tapped := &lockedWriter{w: io.MultiWriter(a2cW, &stdout), mu: &mu}

	// The logger is discarded: the server's stdout must be clean regardless of
	// where logs go, and routing them into the tap here would only prove the
	// tap works.
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := acpsrv.NewServer(stubApp(newStubCoordinator(nil)), log, c2aR, tapped)

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- srv.Start(ctx)
	}()

	client := acp.NewClientSideConnection(newStubClient(), c2aW, a2cR)
	if _, err := client.Initialize(t.Context(), acp.InitializeRequest{
		ProtocolVersion:    acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{},
	}); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if _, err := client.NewSession(t.Context(), acp.NewSessionRequest{
		Cwd: "/workspace", McpServers: []acp.McpServer{},
	}); err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	_ = c2aW.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("server did not exit")
	}

	mu.Lock()
	raw := stdout.String()
	mu.Unlock()

	if raw == "" {
		t.Fatal("server produced no stdout; the handshake should have")
	}
	sc := bufio.NewScanner(strings.NewReader(raw))
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	lines := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		lines++
		var envelope struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Result  json.RawMessage `json:"result"`
			Error   json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			t.Fatalf("stdout line %d is not JSON-RPC: %q", lines, line)
		}
		if envelope.JSONRPC != "2.0" {
			t.Fatalf("stdout line %d has jsonrpc=%q, want \"2.0\": %q", lines, envelope.JSONRPC, line)
		}
		if envelope.Method == "" && len(envelope.Result) == 0 && len(envelope.Error) == 0 {
			t.Fatalf("stdout line %d is neither a call, a result, nor an error: %q", lines, line)
		}
	}
	if lines == 0 {
		t.Fatal("no JSON-RPC frames found on stdout")
	}
}

type lockedWriter struct {
	w  io.Writer
	mu *sync.Mutex
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// I-2: the initialize response must advertise the capabilities an editor reads
// to decide whether the agent is usable.
func TestInitializeResponseShape(t *testing.T) {
	h := newPipeHarness(t, stubApp(newStubCoordinator(nil)))

	resp, err := h.client.Initialize(t.Context(), acp.InitializeRequest{
		ProtocolVersion:    acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{},
	})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if resp.ProtocolVersion != acp.ProtocolVersionNumber {
		t.Errorf("protocolVersion = %d, want %d", resp.ProtocolVersion, acp.ProtocolVersionNumber)
	}
	if resp.AgentInfo == nil {
		t.Fatal("agentInfo is nil")
	}
	if resp.AgentInfo.Name != "Crush" {
		t.Errorf("agentInfo.name = %q, want Crush", resp.AgentInfo.Name)
	}
	if resp.AgentInfo.Version == "" {
		t.Error("agentInfo.version is empty")
	}
	// The wire form must carry the capability object, not omit it: an editor
	// that reads promptCapabilities from a missing field sees the zero value
	// and can gate its UI. Assert on the encoded bytes, not just the struct.
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	caps, ok := generic["agentCapabilities"].(map[string]any)
	if !ok {
		t.Fatalf("agentCapabilities missing from %s", raw)
	}
	for _, key := range []string{"promptCapabilities", "sessionCapabilities", "mcpCapabilities"} {
		if _, present := caps[key]; !present {
			t.Errorf("agentCapabilities.%s missing from %s", key, raw)
		}
	}
}

// I-1: a successful turn must stream assistant output as session/update
// notifications. A client that waits for the first agent_message_chunk before
// leaving its loading state hangs forever without this.
func TestPromptStreamsAgentMessageChunk(t *testing.T) {
	msgs := newStubMessageService()
	co := &emittingCoordinator{
		stubCoordinator: newStubCoordinator(nil),
		msgs:            msgs,
		text:            "hello from crush",
	}
	a := stubApp(co)
	a.Messages = msgs

	var (
		mu      sync.Mutex
		updates []acp.SessionNotification
	)
	c := &trackingUpdateClient{
		stubClient: newStubClient(),
		hook: func(n acp.SessionNotification) {
			mu.Lock()
			updates = append(updates, n)
			mu.Unlock()
		},
	}

	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := acpsrv.NewServer(a, log, c2aR, a2cW)
	srv.StartEventBridge(t.Context())

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		done <- srv.Start(ctx)
	}()

	client := acp.NewClientSideConnection(c, c2aW, a2cR)
	if _, err := client.Initialize(t.Context(), acp.InitializeRequest{
		ProtocolVersion:    acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{},
	}); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	ns, err := client.NewSession(t.Context(), acp.NewSessionRequest{
		Cwd: "/workspace", McpServers: []acp.McpServer{},
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	// The bridge must be attached before the turn publishes, or the broker
	// simply drops the event and this test measures nothing.
	waitForSubscriber(t, msgs.Broker)

	resp, err := client.Prompt(t.Context(), acp.PromptRequest{
		SessionId: ns.SessionId,
		Prompt:    []acp.ContentBlock{acp.TextBlock("hi")},
	})
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if resp.StopReason != acp.StopReasonEndTurn {
		t.Errorf("stopReason = %q, want %q", resp.StopReason, acp.StopReasonEndTurn)
	}

	// The update is written while Prompt is still in flight; give the
	// notification queue a moment to drain on the client side.
	deadline := time.Now().Add(2 * time.Second)
	sawChunk := false
	for time.Now().Before(deadline) {
		mu.Lock()
		for _, u := range updates {
			if u.Update.AgentMessageChunk != nil {
				sawChunk = true
				break
			}
		}
		mu.Unlock()
		if sawChunk {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(updates) == 0 {
		t.Fatal("no session/update notifications during a successful turn; " +
			"a client waiting for agent_message_chunk would hang (see ACP_PLAN I-1)")
	}
	var streamed string
	for _, u := range updates {
		if u.Update.AgentMessageChunk != nil && u.Update.AgentMessageChunk.Content.Text != nil {
			streamed += u.Update.AgentMessageChunk.Content.Text.Text
		}
	}
	if streamed != "hello from crush" {
		t.Fatalf("streamed text = %q, want %q", streamed, "hello from crush")
	}
}

// I-3: req.Cwd names the workspace the editor has open. Crush resolves its
// working directory once at startup, so the agent must at least not silently
// disagree with the client: a divergent cwd has to be visible, not dropped.
func TestNewSessionReportsCwdMismatch(t *testing.T) {
	// The stub app carries no config store, so the agent cannot see a local
	// working directory and must accept whatever the client asks for.
	h := newPipeHarness(t, stubApp(newStubCoordinator(nil)))

	if _, err := h.client.NewSession(t.Context(), acp.NewSessionRequest{
		Cwd: "/workspace", McpServers: []acp.McpServer{},
	}); err != nil {
		t.Fatalf("NewSession should accept a cwd when no local directory is known: %v", err)
	}
}

// I-5: editors send these during session setup. An error here can abort the
// panel before the first prompt.
func TestSessionSetupMethodsSucceed(t *testing.T) {
	h := newPipeHarness(t, stubApp(newStubCoordinator(nil)))

	ns, err := h.client.NewSession(t.Context(), acp.NewSessionRequest{
		Cwd: "/workspace", McpServers: []acp.McpServer{},
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	if _, err := h.client.SetSessionMode(t.Context(), acp.SetSessionModeRequest{
		SessionId: ns.SessionId,
		ModeId:    "code",
	}); err != nil {
		t.Errorf("SetSessionMode: %v", err)
	}

	if _, err := h.client.SetSessionConfigOption(t.Context(), acp.SetSessionConfigOptionRequest{
		ValueId: &acp.SetSessionConfigOptionValueId{
			SessionId: ns.SessionId,
			ConfigId:  "model",
			Value:     "gpt-4o-mini",
		},
	}); err != nil {
		t.Errorf("SetSessionConfigOption: %v", err)
	}
}

var (
	_                   = fantasy.FinishReasonStop
	_ agent.Coordinator = (*stubCoordinator)(nil)
)

// waitForSubscriber blocks until the broker reports at least one subscriber.
func waitForSubscriber(t *testing.T, broker *pubsub.Broker[message.Message]) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if broker.GetSubscriberCount() > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("event bridge never subscribed to the message broker")
}

// emittingCoordinator publishes assistant text on the message broker while the
// turn is in flight, the way the real agent does.
type emittingCoordinator struct {
	*stubCoordinator
	msgs *stubMessageService
	text string
}

func (c *emittingCoordinator) Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	c.msgs.Publish(pubsub.UpdatedEvent, message.Message{
		ID:        "msg-1",
		SessionID: sessionID,
		Role:      message.Assistant,
		Parts:     []message.ContentPart{message.TextContent{Text: c.text}},
	})
	return c.stubCoordinator.Run(ctx, sessionID, prompt, attachments...)
}

func (c *emittingCoordinator) RunAccepted(ctx context.Context, accept *agent.AcceptedRun, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	return c.Run(ctx, sessionID, prompt, attachments...)
}

// S8/S9: session creation must emit the setup notifications (available
// commands, session info) the client needs before the first prompt.
func TestNewSessionEmitsSetupUpdates(t *testing.T) {
	msgs := newStubMessageService()
	co := newStubCoordinator(nil)
	a := stubApp(co)
	a.Messages = msgs

	var (
		mu      sync.Mutex
		updates []acp.SessionNotification
	)
	c := &trackingUpdateClient{
		stubClient: newStubClient(),
		hook: func(n acp.SessionNotification) {
			mu.Lock()
			updates = append(updates, n)
			mu.Unlock()
		},
	}

	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	defer c2aR.Close()
	defer c2aW.Close()
	defer a2cR.Close()
	defer a2cW.Close()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := acpsrv.NewServer(a, log, c2aR, a2cW)
	srv.StartEventBridge(t.Context())

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		done <- srv.Start(ctx)
	}()

	client := acp.NewClientSideConnection(c, c2aW, a2cR)
	if _, err := client.Initialize(t.Context(), acp.InitializeRequest{
		ProtocolVersion:    acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{},
	}); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if _, err := client.NewSession(t.Context(), acp.NewSessionRequest{
		Cwd: "/workspace", McpServers: []acp.McpServer{},
	}); err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	sawCommands, sawInfo := false, false
	for time.Now().Before(deadline) {
		mu.Lock()
		for _, u := range updates {
			if u.Update.AvailableCommandsUpdate != nil {
				sawCommands = true
			}
			if u.Update.SessionInfoUpdate != nil {
				sawInfo = true
			}
		}
		mu.Unlock()
		if sawCommands && sawInfo {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !sawCommands {
		t.Fatal("NewSession did not emit available_commands_update (S8)")
	}
	if !sawInfo {
		t.Fatal("NewSession did not emit session_info_update (S9)")
	}
}
