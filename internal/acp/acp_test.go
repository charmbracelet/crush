package acp_test

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/acp"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/toolruntime"
	"github.com/stretchr/testify/require"
)

// ---- Minimal fakes ----

type fakeSessionService struct {
	*pubsub.Broker[session.Session]
	sessions map[string]session.Session
	cwds     map[string]string
}

func newFakeSessionService() *fakeSessionService {
	return &fakeSessionService{
		Broker:   pubsub.NewBroker[session.Session](),
		sessions: make(map[string]session.Session),
		cwds:     make(map[string]string),
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

func (f *fakeSessionService) CreateHandoffSession(_ context.Context, sourceSessionID, title, goal, draftPrompt string, files []string) (session.Session, error) {
	s := session.Session{
		ID:                     "handoff-" + title,
		Kind:                   session.KindHandoff,
		Title:                  title,
		HandoffSourceSessionID: sourceSessionID,
		HandoffGoal:            goal,
		HandoffDraftPrompt:     draftPrompt,
		HandoffRelevantFiles:   files,
	}
	f.sessions[s.ID] = s
	return s, nil
}

func (f *fakeSessionService) Get(_ context.Context, id string) (session.Session, error) {
	s, ok := f.sessions[id]
	if !ok {
		return session.Session{}, sql.ErrNoRows
	}
	return s, nil
}

func (f *fakeSessionService) GetLast(_ context.Context) (session.Session, error) {
	for _, s := range f.sessions {
		return s, nil
	}
	return session.Session{}, fmt.Errorf("no sessions found")
}

func (f *fakeSessionService) List(_ context.Context) ([]session.Session, error) {
	list := make([]session.Session, 0, len(f.sessions))
	for _, s := range f.sessions {
		list = append(list, s)
	}
	return list, nil
}

func (f *fakeSessionService) Save(_ context.Context, s session.Session) (session.Session, error) {
	f.sessions[s.ID] = s
	return s, nil
}

func (f *fakeSessionService) UpdateCollaborationMode(_ context.Context, id string, mode session.CollaborationMode) (session.Session, error) {
	return session.Session{ID: id}, nil
}

func (f *fakeSessionService) UpdatePermissionMode(_ context.Context, id string, mode session.PermissionMode) (session.Session, error) {
	s := f.sessions[id]
	s.ID = id
	s.PermissionMode = mode
	f.sessions[id] = s
	return s, nil
}

func (f *fakeSessionService) SetDefaultPermissionMode(session.PermissionMode) {}

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
	lists   map[string][]message.Message
	created []createdMessageCall
}

type createdMessageCall struct {
	sessionID string
	params    message.CreateMessageParams
}

func newFakeMessageService() *fakeMessageService {
	return &fakeMessageService{
		Broker: pubsub.NewBroker[message.Message](),
		lists:  make(map[string][]message.Message),
	}
}

func (f *fakeMessageService) Create(_ context.Context, sessionID string, params message.CreateMessageParams) (message.Message, error) {
	msg := message.Message{
		ID:        fmt.Sprintf("msg-%d", len(f.created)+1),
		Role:      params.Role,
		SessionID: sessionID,
		Parts:     append([]message.ContentPart(nil), params.Parts...),
	}
	f.created = append(f.created, createdMessageCall{
		sessionID: sessionID,
		params:    params,
	})
	f.lists[sessionID] = append(f.lists[sessionID], msg)
	return msg, nil
}
func (f *fakeMessageService) Update(_ context.Context, _ message.Message) error { return nil }
func (f *fakeMessageService) Get(_ context.Context, _ string) (message.Message, error) {
	return message.Message{}, nil
}

func (f *fakeMessageService) List(_ context.Context, sessionID string) ([]message.Message, error) {
	return f.lists[sessionID], nil
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
func (f *fakeCoordinator) Cancel(_ string)                         {}
func (f *fakeCoordinator) CancelAll()                              {}
func (f *fakeCoordinator) IsSessionBusy(_ string) bool             { return false }
func (f *fakeCoordinator) IsBusy() bool                            { return false }
func (f *fakeCoordinator) QueuedPrompts(_ string) int              { return 0 }
func (f *fakeCoordinator) QueuedPromptsList(_ string) []string     { return nil }
func (f *fakeCoordinator) RemoveQueuedPrompt(_ string, _ int) bool { return false }
func (f *fakeCoordinator) ClearQueue(_ string)                     {}
func (f *fakeCoordinator) PauseQueue(_ string)                     {}
func (f *fakeCoordinator) ResumeQueue(_ string)                    {}
func (f *fakeCoordinator) IsQueuePaused(_ string) bool             { return false }
func (f *fakeCoordinator) Summarize(_ context.Context, _ string, _ fantasy.ProviderOptions) error {
	return nil
}
func (f *fakeCoordinator) GenerateHandoff(_ context.Context, _ string, _ string) (agent.HandoffDraft, error) {
	return agent.HandoffDraft{}, nil
}
func (f *fakeCoordinator) ClassifyPermission(_ context.Context, _ permission.PermissionRequest) (permission.AutoClassification, error) {
	return permission.AutoClassification{}, nil
}
func (f *fakeCoordinator) Model() agent.Model { return agent.Model{} }
func (f *fakeCoordinator) PrepareModelSwitch(_ context.Context, _ string, _ config.SelectedModelType, _ config.SelectedModel) error {
	return nil
}
func (f *fakeCoordinator) UpdateModels(_ context.Context) error        { return nil }
func (f *fakeCoordinator) RefreshTools(_ context.Context) error        { return nil }
func (f *fakeCoordinator) PrioritizeQueuedPrompt(_ string, _ int) bool { return false }

type fakeApp struct {
	sessions    *fakeSessionService
	messages    *fakeMessageService
	coordinator *fakeCoordinator
	permissions permission.Service
	cfg         *config.ConfigStore
	runtime     toolruntime.Service
}

func (a *fakeApp) GetSessions() session.Service      { return a.sessions }
func (a *fakeApp) GetMessages() message.Service      { return a.messages }
func (a *fakeApp) GetCoordinator() agent.Coordinator { return a.coordinator }
func (a *fakeApp) GetConfig() *config.ConfigStore    { return a.cfg }
func (a *fakeApp) GetPermissions() permission.Service {
	if a.permissions == nil {
		a.permissions = permission.NewPermissionService(".", false, nil)
	}
	return a.permissions
}
func (a *fakeApp) GetToolRuntime() toolruntime.Service {
	if a.runtime == nil {
		a.runtime = toolruntime.NewService()
	}
	return a.runtime
}

func TestSessionListIncludesCWD(t *testing.T) {
	t.Parallel()

	cwd := "/tmp/project"
	reqLine := buildRequest(t, 1, "session/list", acp.SessionListParams{CWD: cwd})

	app := &fakeApp{
		sessions:    newFakeSessionService(),
		messages:    newFakeMessageService(),
		coordinator: &fakeCoordinator{runResult: &fantasy.AgentResult{}},
	}
	app.sessions.sessions["sess-1"] = session.Session{ID: "sess-1", Title: "test"}

	resp := runSingleRequest(t, app, reqLine)
	require.Nil(t, resp.Error)

	var result acp.SessionListResult
	require.NoError(t, json.Unmarshal(resp.Result, &result))
	require.Len(t, result.Sessions, 1)
	require.Equal(t, "sess-1", result.Sessions[0].SessionID)
	require.Equal(t, "test", result.Sessions[0].Title)

	// Expected CWD is the absolute path of the input cwd.
	expectedCWD, err := filepath.Abs(filepath.FromSlash(cwd))
	require.NoError(t, err)
	require.Equal(t, expectedCWD, result.Sessions[0].CWD)
}

func TestSessionListPrefersPersistedCWD(t *testing.T) {
	t.Parallel()

	reqLine := buildRequest(t, 1, "session/list", acp.SessionListParams{CWD: "/fallback"})

	app := &fakeApp{
		sessions:    newFakeSessionService(),
		messages:    newFakeMessageService(),
		coordinator: &fakeCoordinator{runResult: &fantasy.AgentResult{}},
	}
	savedCWD, err := filepath.Abs(filepath.FromSlash("/persisted/workspace"))
	require.NoError(t, err)
	app.sessions.sessions["sess-1"] = session.Session{ID: "sess-1", Title: "test", WorkspaceCWD: savedCWD}

	resp := runSingleRequest(t, app, reqLine)
	require.Nil(t, resp.Error)

	var result acp.SessionListResult
	require.NoError(t, json.Unmarshal(resp.Result, &result))
	require.Len(t, result.Sessions, 1)
	require.Equal(t, savedCWD, result.Sessions[0].CWD)
}

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
	for scanner.Scan() {
		var resp acp.Response
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &resp))
		if resp.ID != nil {
			return resp
		}
	}
	require.FailNow(t, "expected a response line")
	return acp.Response{}
}

func runSingleRequest(t *testing.T, app *fakeApp, reqLine string) acp.Response {
	t.Helper()

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()

	handler := acp.NewHandler(app)
	server := acp.NewServerWithIO(handler, inReader, outWriter)
	handler.SetServer(server)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() { _ = server.Serve(ctx) }()

	_, err := fmt.Fprint(inWriter, reqLine)
	require.NoError(t, err)
	require.NoError(t, inWriter.Close())

	scanner := bufio.NewScanner(outReader)
	return readResponse(t, scanner)
}

// ---- Tests ----

func TestInitialize(t *testing.T) {
	t.Parallel()

	reqLine := buildRequest(t, 1, "initialize", acp.InitializeParams{
		ProtocolVersion: 1,
		ClientInfo:      acp.ClientInfo{Name: "test-client", Version: "1.0"},
	})

	app := &fakeApp{
		sessions:    newFakeSessionService(),
		messages:    newFakeMessageService(),
		coordinator: &fakeCoordinator{runResult: &fantasy.AgentResult{}},
	}

	resp := runSingleRequest(t, app, reqLine)

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

	reqLine := buildRequest(t, 1, "session/new", acp.SessionNewParams{CWD: "/tmp"})

	app := &fakeApp{
		sessions:    newFakeSessionService(),
		messages:    newFakeMessageService(),
		coordinator: &fakeCoordinator{runResult: &fantasy.AgentResult{}},
	}

	resp := runSingleRequest(t, app, reqLine)

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

func TestSessionLoadReplaysHistoryBeforeResponse(t *testing.T) {
	t.Parallel()

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()

	sessionID := "test-load-sess"

	app := &fakeApp{
		sessions:    newFakeSessionService(),
		messages:    newFakeMessageService(),
		coordinator: &fakeCoordinator{runResult: &fantasy.AgentResult{}},
	}
	app.sessions.sessions[sessionID] = session.Session{ID: sessionID, Title: "loaded-session"}
	app.messages.lists[sessionID] = []message.Message{
		{
			ID:        "user-1",
			SessionID: sessionID,
			Role:      message.User,
			Parts: []message.ContentPart{
				message.TextContent{Text: "history question"},
			},
		},
		{
			ID:        "assistant-1",
			SessionID: sessionID,
			Role:      message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: "history answer"},
			},
		},
	}

	handler := acp.NewHandler(app)
	server := acp.NewServerWithIO(handler, inReader, outWriter)
	handler.SetServer(server)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer inWriter.Close()

	go func() { _ = server.Serve(ctx) }()

	outScanner := bufio.NewScanner(outReader)

	_, err := fmt.Fprint(inWriter, buildRequest(t, 1, "session/load", acp.SessionLoadParams{SessionID: sessionID}))
	require.NoError(t, err)

	require.True(t, outScanner.Scan(), "expected first line")
	firstLine := outScanner.Bytes()
	var firstMsg map[string]any
	require.NoError(t, json.Unmarshal(firstLine, &firstMsg))
	require.Equal(t, "session/update", firstMsg["method"])

	require.True(t, outScanner.Scan(), "expected second line")
	secondLine := outScanner.Bytes()
	var secondMsg map[string]any
	require.NoError(t, json.Unmarshal(secondLine, &secondMsg))
	require.Equal(t, "session/update", secondMsg["method"])

	require.True(t, outScanner.Scan(), "expected response line")
	thirdLine := outScanner.Bytes()
	var resp acp.Response
	require.NoError(t, json.Unmarshal(thirdLine, &resp))
	require.NotNil(t, resp.ID)
	require.EqualValues(t, 1, *resp.ID)
	require.Nil(t, resp.Error)
}

func TestSessionLoadPersistsCWD(t *testing.T) {
	t.Parallel()

	sessionID := "test-load-save-cwd"
	reqLine := buildRequest(t, 1, "session/load", acp.SessionLoadParams{
		SessionID: sessionID,
		CWD:       "/tmp/acp-workspace",
	})

	app := &fakeApp{
		sessions:    newFakeSessionService(),
		messages:    newFakeMessageService(),
		coordinator: &fakeCoordinator{runResult: &fantasy.AgentResult{}},
	}
	app.sessions.sessions[sessionID] = session.Session{ID: sessionID, Title: "loaded-session"}

	resp := runSingleRequest(t, app, reqLine)
	require.Nil(t, resp.Error)

	saved, ok := app.sessions.sessions[sessionID]
	require.True(t, ok)
	expectedCWD, err := filepath.Abs(filepath.FromSlash("/tmp/acp-workspace"))
	require.NoError(t, err)
	require.Equal(t, expectedCWD, saved.WorkspaceCWD)
}

func TestSessionLoadNotFound(t *testing.T) {
	t.Parallel()

	reqLine := buildRequest(t, 1, "session/load", acp.SessionLoadParams{
		SessionID: "nonexistent-session",
	})

	app := &fakeApp{
		sessions:    newFakeSessionService(),
		messages:    newFakeMessageService(),
		coordinator: &fakeCoordinator{runResult: &fantasy.AgentResult{}},
	}

	resp := runSingleRequest(t, app, reqLine)

	require.NotNil(t, resp.Error)
	require.Equal(t, acp.CodeResourceNotFound, resp.Error.Code)
	require.Contains(t, resp.Error.Message, "session not found")
	require.Contains(t, resp.Error.Message, "nonexistent-session")
}

func TestUnknownMethod(t *testing.T) {
	t.Parallel()

	reqLine := buildRequest(t, 1, "unknown/method", nil)

	app := &fakeApp{
		sessions:    newFakeSessionService(),
		messages:    newFakeMessageService(),
		coordinator: &fakeCoordinator{runResult: &fantasy.AgentResult{}},
	}

	resp := runSingleRequest(t, app, reqLine)

	require.NotNil(t, resp.Error)
	require.Equal(t, acp.CodeMethodNotFound, resp.Error.Code)
}

func TestSetConfigOptionMethodIsRouted(t *testing.T) {
	t.Parallel()

	reqLine := buildRequest(t, 1, "session/set_config_option", acp.SetConfigOptionParams{
		SessionID: "sess-1",
		ConfigID:  "model_large",
		Value:     "bad-format",
	})

	app := &fakeApp{
		sessions:    newFakeSessionService(),
		messages:    newFakeMessageService(),
		coordinator: &fakeCoordinator{runResult: &fantasy.AgentResult{}},
	}

	resp := runSingleRequest(t, app, reqLine)

	require.NotNil(t, resp.Error)
	// Should be handled by set_config_option and fail params, not method not found.
	require.NotEqual(t, acp.CodeMethodNotFound, resp.Error.Code)
}

func TestSessionSetModePersistsPermissionMode(t *testing.T) {
	t.Parallel()

	sessionID := "sess-mode"
	reqLine := buildRequest(t, 1, "session/set_mode", acp.SetModeParams{
		SessionID: sessionID,
		ModeID:    "yolo",
	})

	app := &fakeApp{
		sessions:    newFakeSessionService(),
		messages:    newFakeMessageService(),
		coordinator: &fakeCoordinator{runResult: &fantasy.AgentResult{}},
	}
	app.sessions.sessions[sessionID] = session.Session{
		ID:             sessionID,
		Title:          "mode-test",
		PermissionMode: session.PermissionModeDefault,
	}

	resp := runSingleRequest(t, app, reqLine)
	require.Nil(t, resp.Error)

	saved := app.sessions.sessions[sessionID]
	require.Equal(t, session.PermissionModeYolo, saved.PermissionMode)
}

func TestSessionSetModeAutoExitCreatesReminder(t *testing.T) {
	t.Parallel()

	sessionID := "sess-mode-auto-exit"
	reqLine := buildRequest(t, 1, "session/set_mode", acp.SetModeParams{
		SessionID: sessionID,
		ModeID:    "default",
	})

	app := &fakeApp{
		sessions:    newFakeSessionService(),
		messages:    newFakeMessageService(),
		coordinator: &fakeCoordinator{runResult: &fantasy.AgentResult{}},
	}
	app.sessions.sessions[sessionID] = session.Session{
		ID:             sessionID,
		Title:          "mode-test",
		PermissionMode: session.PermissionModeAuto,
	}

	resp := runSingleRequest(t, app, reqLine)
	require.Nil(t, resp.Error)

	require.Len(t, app.messages.created, 1)
	created := app.messages.created[0]
	require.Equal(t, sessionID, created.sessionID)
	msg := message.Message{
		Role:      created.params.Role,
		SessionID: created.sessionID,
		Parts:     created.params.Parts,
	}
	promptType, ok := message.ParseAutoModePrompt(msg)
	require.True(t, ok)
	require.Equal(t, message.AutoModePromptTypeExit, promptType)
}

func TestSessionSetConfigOptionModePersistsPermissionMode(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	workingDir := filepath.Join(baseDir, "workspace")
	require.NoError(t, os.MkdirAll(workingDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(workingDir, "crush.json"),
		[]byte(`{"options":{"disable_provider_auto_update":true},"tools":{}}`),
		0o644,
	))

	store, err := config.Init(workingDir, filepath.Join(baseDir, "state"), false)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, log.ResetForTesting())
	})

	sessionID := "sess-config-mode"
	reqLine := buildRequest(t, 1, "session/set_config_option", acp.SetConfigOptionParams{
		SessionID: sessionID,
		ConfigID:  "mode",
		Value:     "auto",
	})

	app := &fakeApp{
		sessions:    newFakeSessionService(),
		messages:    newFakeMessageService(),
		coordinator: &fakeCoordinator{runResult: &fantasy.AgentResult{}},
		cfg:         store,
	}
	app.sessions.sessions[sessionID] = session.Session{
		ID:             sessionID,
		Title:          "config-mode-test",
		PermissionMode: session.PermissionModeDefault,
	}

	resp := runSingleRequest(t, app, reqLine)
	require.Nil(t, resp.Error)

	saved := app.sessions.sessions[sessionID]
	require.Equal(t, session.PermissionModeAuto, saved.PermissionMode)

	var result acp.SetConfigOptionResult
	require.NoError(t, json.Unmarshal(resp.Result, &result))
}

func TestSessionSetConfigOptionModeAutoExitCreatesReminder(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	workingDir := filepath.Join(baseDir, "workspace")
	require.NoError(t, os.MkdirAll(workingDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(workingDir, "crush.json"),
		[]byte(`{"options":{"disable_provider_auto_update":true},"tools":{}}`),
		0o644,
	))

	store, err := config.Init(workingDir, filepath.Join(baseDir, "state"), false)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, log.ResetForTesting())
	})

	sessionID := "sess-config-mode-auto-exit"
	reqLine := buildRequest(t, 1, "session/set_config_option", acp.SetConfigOptionParams{
		SessionID: sessionID,
		ConfigID:  "mode",
		Value:     "default",
	})

	app := &fakeApp{
		sessions:    newFakeSessionService(),
		messages:    newFakeMessageService(),
		coordinator: &fakeCoordinator{runResult: &fantasy.AgentResult{}},
		cfg:         store,
	}
	app.sessions.sessions[sessionID] = session.Session{
		ID:             sessionID,
		Title:          "config-mode-test",
		PermissionMode: session.PermissionModeAuto,
	}

	resp := runSingleRequest(t, app, reqLine)
	require.Nil(t, resp.Error)

	require.Len(t, app.messages.created, 1)
	created := app.messages.created[0]
	require.Equal(t, sessionID, created.sessionID)
	msg := message.Message{
		Role:      created.params.Role,
		SessionID: created.sessionID,
		Parts:     created.params.Parts,
	}
	promptType, ok := message.ParseAutoModePrompt(msg)
	require.True(t, ok)
	require.Equal(t, message.AutoModePromptTypeExit, promptType)
}
