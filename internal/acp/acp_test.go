package acp_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/acp"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// ---- Minimal fakes ----

type fakeSessionService struct {
	*pubsub.Broker[session.Session]
	sessions map[string]session.Session
}

func newFakeSessionService() *fakeSessionService {
	return &fakeSessionService{
		Broker:   pubsub.NewBroker[session.Session](),
		sessions: make(map[string]session.Session),
	}
}

func (f *fakeSessionService) Create(_ context.Context, title string) (session.Session, error) {
	s := session.Session{ID: "sess-" + title, Title: title}
	f.sessions[s.ID] = s
	return s, nil
}
func (f *fakeSessionService) CreateTitleSession(_ context.Context, parentID string) (session.Session, error) {
	return session.Session{ID: "title-" + parentID}, nil
}
func (f *fakeSessionService) CreateTaskSession(_ context.Context, toolCallID, parentID, title string) (session.Session, error) {
	return session.Session{ID: toolCallID}, nil
}
func (f *fakeSessionService) Get(_ context.Context, id string) (session.Session, error) {
	s, ok := f.sessions[id]
	if !ok {
		return session.Session{ID: id, Title: "loaded"}, nil
	}
	return s, nil
}
func (f *fakeSessionService) List(_ context.Context) ([]session.Session, error) { return nil, nil }
func (f *fakeSessionService) Save(_ context.Context, s session.Session) (session.Session, error) {
	return s, nil
}
func (f *fakeSessionService) UpdateCollaborationMode(_ context.Context, id string, mode session.CollaborationMode) (session.Session, error) {
	return session.Session{ID: id}, nil
}
func (f *fakeSessionService) UpdateTitleAndUsage(_ context.Context, id, title string, p, c int64, cost float64) error {
	return nil
}
func (f *fakeSessionService) Rename(_ context.Context, id, title string) error { return nil }
func (f *fakeSessionService) Delete(_ context.Context, id string) error        { return nil }
func (f *fakeSessionService) CreateAgentToolSessionID(msgID, tcID string) string {
	return msgID + ":" + tcID
}
func (f *fakeSessionService) ParseAgentToolSessionID(id string) (string, string, bool) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1], true
	}
	return "", "", false
}
func (f *fakeSessionService) IsAgentToolSession(id string) bool { return strings.Contains(id, ":") }

type fakeMessageService struct {
	*pubsub.Broker[message.Message]
}

func newFakeMessageService() *fakeMessageService {
	return &fakeMessageService{Broker: pubsub.NewBroker[message.Message]()}
}

func (f *fakeMessageService) Create(_ context.Context, _ string, _ message.CreateMessageParams) (message.Message, error) {
	return message.Message{}, nil
}
func (f *fakeMessageService) Update(_ context.Context, _ message.Message) error { return nil }
func (f *fakeMessageService) Get(_ context.Context, _ string) (message.Message, error) {
	return message.Message{}, nil
}
func (f *fakeMessageService) List(_ context.Context, _ string) ([]message.Message, error) {
	return nil, nil
}
func (f *fakeMessageService) ListUserMessages(_ context.Context, _ string) ([]message.Message, error) {
	return nil, nil
}
func (f *fakeMessageService) ListAllUserMessages(_ context.Context) ([]message.Message, error) {
	return nil, nil
}
func (f *fakeMessageService) Delete(_ context.Context, _ string) error                { return nil }
func (f *fakeMessageService) DeleteSessionMessages(_ context.Context, _ string) error { return nil }

type fakeCoordinator struct {
	runResult *fantasy.AgentResult
	runErr    error
}

func (f *fakeCoordinator) Run(_ context.Context, sessionID, prompt string, _ ...message.Attachment) (*fantasy.AgentResult, error) {
	return f.runResult, f.runErr
}
func (f *fakeCoordinator) Cancel(_ string)                             {}
func (f *fakeCoordinator) CancelAll()                                  {}
func (f *fakeCoordinator) IsSessionBusy(_ string) bool                 { return false }
func (f *fakeCoordinator) IsBusy() bool                                { return false }
func (f *fakeCoordinator) QueuedPrompts(_ string) int                  { return 0 }
func (f *fakeCoordinator) QueuedPromptsList(_ string) []string         { return nil }
func (f *fakeCoordinator) RemoveQueuedPrompt(_ string, _ int) bool     { return false }
func (f *fakeCoordinator) ClearQueue(_ string)                         {}
func (f *fakeCoordinator) Summarize(_ context.Context, _ string) error { return nil }
func (f *fakeCoordinator) Model() agent.Model                          { return agent.Model{} }
func (f *fakeCoordinator) UpdateModels(_ context.Context) error        { return nil }

type fakeApp struct {
	sessions    *fakeSessionService
	messages    *fakeMessageService
	coordinator *fakeCoordinator
}

func (a *fakeApp) GetSessions() session.Service      { return a.sessions }
func (a *fakeApp) GetMessages() message.Service      { return a.messages }
func (a *fakeApp) GetCoordinator() agent.Coordinator { return a.coordinator }

// ---- Helpers ----

func buildRequest(t *testing.T, id int64, method string, params any) string {
	t.Helper()
	p, err := json.Marshal(params)
	require.NoError(t, err)
	req := acp.Request{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  p,
	}
	b, err := json.Marshal(req)
	require.NoError(t, err)
	return string(b) + "\n"
}

func readResponse(t *testing.T, scanner *bufio.Scanner) acp.Response {
	t.Helper()
	require.True(t, scanner.Scan(), "expected a response line")
	var resp acp.Response
	require.NoError(t, json.Unmarshal(scanner.Bytes(), &resp))
	return resp
}

// ---- Tests ----

func TestInitialize(t *testing.T) {
	t.Parallel()

	var outBuf bytes.Buffer
	reqLine := buildRequest(t, 1, "initialize", acp.InitializeParams{
		ProtocolVersion: 1,
		ClientInfo:      acp.ClientInfo{Name: "test-client", Version: "1.0"},
	})

	app := &fakeApp{
		sessions:    newFakeSessionService(),
		messages:    newFakeMessageService(),
		coordinator: &fakeCoordinator{runResult: &fantasy.AgentResult{}},
	}

	handler := acp.NewHandler(app)
	server := acp.NewServerWithIO(handler, strings.NewReader(reqLine), &outBuf)
	handler.SetServer(server)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() { _ = server.Serve(ctx) }()
	time.Sleep(100 * time.Millisecond) // let it process

	scanner := bufio.NewScanner(&outBuf)
	resp := readResponse(t, scanner)

	require.Nil(t, resp.Error, "unexpected error: %v", resp.Error)
	require.NotNil(t, resp.Result)

	var result acp.InitializeResult
	require.NoError(t, json.Unmarshal(resp.Result, &result))
	require.Equal(t, acp.ProtocolVersion, result.ProtocolVersion)
	require.Equal(t, "crush", result.AgentInfo.Name)
	require.True(t, result.AgentCapabilities.LoadSession)
}

func TestSessionNew(t *testing.T) {
	t.Parallel()

	var outBuf bytes.Buffer
	reqLine := buildRequest(t, 1, "session/new", acp.SessionNewParams{CWD: "/tmp"})

	app := &fakeApp{
		sessions:    newFakeSessionService(),
		messages:    newFakeMessageService(),
		coordinator: &fakeCoordinator{runResult: &fantasy.AgentResult{}},
	}

	handler := acp.NewHandler(app)
	server := acp.NewServerWithIO(handler, strings.NewReader(reqLine), &outBuf)
	handler.SetServer(server)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() { _ = server.Serve(ctx) }()
	time.Sleep(100 * time.Millisecond)

	scanner := bufio.NewScanner(&outBuf)
	resp := readResponse(t, scanner)

	require.Nil(t, resp.Error)
	var result acp.SessionNewResult
	require.NoError(t, json.Unmarshal(resp.Result, &result))
	require.NotEmpty(t, result.SessionID)
}

func TestSessionPrompt(t *testing.T) {
	t.Parallel()

	// Use pipes to write one request at a time and read each response before
	// sending the next, ensuring predictable ordering.
	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()

	sessionID := "test-sess-123"

	app := &fakeApp{
		sessions:    newFakeSessionService(),
		messages:    newFakeMessageService(),
		coordinator: &fakeCoordinator{runResult: &fantasy.AgentResult{}},
	}
	app.sessions.sessions[sessionID] = session.Session{ID: sessionID, Title: "test"}

	handler := acp.NewHandler(app)
	server := acp.NewServerWithIO(handler, inReader, outWriter)
	handler.SetServer(server)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer inWriter.Close()

	go func() { _ = server.Serve(ctx) }()

	outScanner := bufio.NewScanner(outReader)

	// Send session/new and wait for its response.
	_, err := fmt.Fprint(inWriter, buildRequest(t, 1, "session/new", acp.SessionNewParams{}))
	require.NoError(t, err)
	resp1 := readResponse(t, outScanner)
	require.Nil(t, resp1.Error)

	// Send session/prompt and wait for its response.
	_, err = fmt.Fprint(inWriter, buildRequest(t, 2, "session/prompt", acp.PromptParams{
		SessionID: sessionID,
		Prompt:    []acp.ContentBlock{{Type: "text", Text: "hello"}},
	}))
	require.NoError(t, err)
	resp2 := readResponse(t, outScanner)
	require.Nil(t, resp2.Error)

	var result acp.PromptResult
	require.NoError(t, json.Unmarshal(resp2.Result, &result))
	require.Equal(t, acp.StopReasonEndTurn, result.StopReason)
}

func TestUnknownMethod(t *testing.T) {
	t.Parallel()

	var outBuf bytes.Buffer
	reqLine := buildRequest(t, 1, "unknown/method", nil)

	app := &fakeApp{
		sessions:    newFakeSessionService(),
		messages:    newFakeMessageService(),
		coordinator: &fakeCoordinator{runResult: &fantasy.AgentResult{}},
	}

	handler := acp.NewHandler(app)
	server := acp.NewServerWithIO(handler, strings.NewReader(reqLine), &outBuf)
	handler.SetServer(server)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() { _ = server.Serve(ctx) }()
	time.Sleep(100 * time.Millisecond)

	scanner := bufio.NewScanner(&outBuf)
	resp := readResponse(t, scanner)

	require.NotNil(t, resp.Error)
	require.Equal(t, acp.CodeMethodNotFound, resp.Error.Code)
}
