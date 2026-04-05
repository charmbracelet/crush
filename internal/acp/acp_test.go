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
	"github.com/charmbracelet/crush/internal/timeline"
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

func (f *fakeSessionService) ListChildren(_ context.Context, _ string) ([]session.Session, error) {
	return nil, nil
}

func (f *fakeSessionService) Save(_ context.Context, s session.Session) (session.Session, error) {
	f.sessions[s.ID] = s
	return s, nil
}

func (f *fakeSessionService) UpdateCollaborationMode(_ context.Context, id string, mode session.CollaborationMode) (session.Session, error) {
	s := f.sessions[id]
	s.ID = id
	s.CollaborationMode = mode
	f.sessions[id] = s
	return s, nil
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
	runResult   *fantasy.AgentResult
	runErr      error
	sessionSvc  *fakeSessionService
	messageSvc  *fakeMessageService
	runtimeSvc  toolruntime.Service
	timelineSvc timeline.Service
	sessions    map[string]session.Session
}

func (f *fakeCoordinator) Run(_ context.Context, sessionID, prompt string, _ ...message.Attachment) (*fantasy.AgentResult, error) {
	if strings.Contains(prompt, "spawn-agent-subsession") {
		subSessionID := "sub-session-1"
		if subSession, ok := f.sessions[subSessionID]; ok {
			f.sessionSvc.Publish(pubsub.CreatedEvent, subSession)
		}
		f.messageSvc.Publish(pubsub.CreatedEvent, message.Message{
			ID:        "sub-tool-result-1",
			SessionID: subSessionID,
			Role:      message.Tool,
			Parts: []message.ContentPart{
				message.ToolResult{
					ToolCallID: "tool-agent-1",
					Name:       "agent",
					Content:    "sub-session tool result",
				}.WithSubtaskResult(message.ToolResultSubtaskResult{
					ChildSessionID:   subSessionID,
					ParentToolCallID: "tool-agent-1",
					Status:           message.ToolResultSubtaskStatusCompleted,
				}),
			},
		})
	}
	if strings.Contains(prompt, "publish-runtime-complete") {
		f.runtimeSvc.Publish(toolruntime.State{SessionID: sessionID, ToolCallID: "tool-fetch-1", ToolName: "fetch", Status: toolruntime.StatusCompleted})
	}
	if strings.Contains(prompt, "publish-reducer-update") {
		f.messageSvc.Publish(pubsub.CreatedEvent, message.Message{
			ID:        "reducer-tool-result-1",
			SessionID: sessionID,
			Role:      message.Tool,
			Parts: []message.ContentPart{
				message.ToolResult{
					ToolCallID: "tool-reducer-1",
					Name:       "agent",
					Content:    "done",
				}.WithReducer(message.ToolResultReducer{
					Summary:     "Execution finished",
					Artifacts:   []string{"dist/app"},
					Risks:       []string{"network flakiness"},
					NextActions: []string{"monitor"},
					Confidence:  "high",
				}),
			},
		})
	}
	if strings.Contains(prompt, "publish-timeline-events") && f.timelineSvc != nil {
		f.timelineSvc.Publish(timeline.ModeChangedEvent(sessionID, session.ModeTransition{
			Previous: session.ModeState{CollaborationMode: session.CollaborationModeDefault, PermissionMode: session.PermissionModeAuto},
			Current:  session.ModeState{CollaborationMode: session.CollaborationModePlan, PermissionMode: session.PermissionModeYolo},
		}))
		f.timelineSvc.Publish(timeline.ChildSessionStartedEvent(sessionID, "child-1", "Child Session"))
		f.timelineSvc.Publish(timeline.ChildSessionFinishedEvent(sessionID, "child-1", "Child Session", "completed", "done"))
		f.timelineSvc.Publish(timeline.Event{SessionID: sessionID, Type: timeline.EventToolFinished, ToolCallID: "tool-1", ToolName: "bash", Title: "bash", Status: "completed"})
	}
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
	timeline    timeline.Service
}

type recordingPermissionService struct {
	permission.Service
	requests              *pubsub.Broker[permission.PermissionRequest]
	lastGrantedPersistent permission.PermissionRequest
	lastGranted           permission.PermissionRequest
	lastDenied            permission.PermissionRequest
}

func (r *recordingPermissionService) Subscribe(ctx context.Context) <-chan pubsub.Event[permission.PermissionRequest] {
	return r.requests.Subscribe(ctx)
}

func (r *recordingPermissionService) PublishRequest(req permission.PermissionRequest) {
	r.requests.Publish(pubsub.CreatedEvent, req)
}

func (r *recordingPermissionService) GrantPersistent(req permission.PermissionRequest) {
	r.lastGrantedPersistent = req
	r.Service.GrantPersistent(req)
}

func (r *recordingPermissionService) Grant(req permission.PermissionRequest) {
	r.lastGranted = req
	r.Service.Grant(req)
}

func (r *recordingPermissionService) Deny(req permission.PermissionRequest) {
	r.lastDenied = req
	r.Service.Deny(req)
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

func (a *fakeApp) GetTimeline() timeline.Service {
	if a.timeline == nil {
		a.timeline = timeline.NewService()
	}
	return a.timeline
}

func newFakeApp() *fakeApp {
	app := &fakeApp{
		sessions: newFakeSessionService(),
		messages: newFakeMessageService(),
	}
	app.coordinator = &fakeCoordinator{
		runResult:   &fantasy.AgentResult{},
		sessionSvc:  app.sessions,
		messageSvc:  app.messages,
		runtimeSvc:  app.GetToolRuntime(),
		timelineSvc: app.GetTimeline(),
		sessions:    app.sessions.sessions,
	}
	return app
}

func newFakeAppWithConfig(cfg *config.ConfigStore) *fakeApp {
	app := newFakeApp()
	app.cfg = cfg
	return app
}

func TestSessionListIncludesCWD(t *testing.T) {
	t.Parallel()

	cwd := "/tmp/project"
	reqLine := buildRequest(t, 1, "session/list", acp.SessionListParams{CWD: cwd})

	app := newFakeApp()
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

	app := newFakeApp()
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

	app := newFakeApp()

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

	app := newFakeApp()

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

	app := newFakeApp()
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

	app := newFakeApp()
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

	var updates []map[string]any
	for range 10 {
		require.True(t, outScanner.Scan(), "expected more output")
		line := outScanner.Bytes()
		var msg map[string]any
		require.NoError(t, json.Unmarshal(line, &msg))
		if _, ok := msg["result"]; ok {
			var resp acp.Response
			require.NoError(t, json.Unmarshal(line, &resp))
			require.NotNil(t, resp.ID)
			require.EqualValues(t, 1, *resp.ID)
			require.Nil(t, resp.Error)
			break
		}
		require.Equal(t, "session/update", msg["method"])
		updates = append(updates, msg)
	}

	require.Len(t, updates, 2)
}

func TestSessionLoadPersistsCWD(t *testing.T) {
	t.Parallel()

	sessionID := "test-load-save-cwd"
	reqLine := buildRequest(t, 1, "session/load", acp.SessionLoadParams{
		SessionID: sessionID,
		CWD:       "/tmp/acp-workspace",
	})

	app := newFakeApp()
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

	app := newFakeApp()

	resp := runSingleRequest(t, app, reqLine)

	require.NotNil(t, resp.Error)
	require.Equal(t, acp.CodeResourceNotFound, resp.Error.Code)
	require.Contains(t, resp.Error.Message, "session not found")
	require.Contains(t, resp.Error.Message, "nonexistent-session")
}

func TestUnknownMethod(t *testing.T) {
	t.Parallel()

	reqLine := buildRequest(t, 1, "unknown/method", nil)

	app := newFakeApp()

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

	app := newFakeApp()

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

	app := newFakeApp()
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

	app := newFakeApp()
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

	app := newFakeAppWithConfig(store)
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

	app := newFakeAppWithConfig(store)
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

func TestSessionPrompt_ForwardsChildSessionToolUpdates(t *testing.T) {
	t.Parallel()

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()

	sessionID := "parent-session"
	subSessionID := "sub-session-1"

	app := newFakeApp()
	app.sessions.sessions[sessionID] = session.Session{ID: sessionID, Title: "parent"}
	app.sessions.sessions[subSessionID] = session.Session{ID: subSessionID, ParentSessionID: sessionID, Title: "child"}

	handler := acp.NewHandler(app)
	server := acp.NewServerWithIO(handler, inReader, outWriter)
	handler.SetServer(server)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer inWriter.Close()

	go func() { _ = server.Serve(ctx) }()

	outScanner := bufio.NewScanner(outReader)

	_, err := fmt.Fprint(inWriter, buildRequest(t, 1, "session/prompt", acp.PromptParams{
		SessionID: sessionID,
		Prompt:    []acp.ContentBlock{{Type: "text", Text: "spawn-agent-subsession"}},
	}))
	require.NoError(t, err)

	seenSubToolUpdate := false
	for outScanner.Scan() {
		line := outScanner.Bytes()
		var envelope struct {
			ID     *int64                        `json:"id"`
			Method string                        `json:"method"`
			Params acp.SessionUpdateNotification `json:"params"`
		}
		require.NoError(t, json.Unmarshal(line, &envelope))
		if envelope.ID != nil {
			require.EqualValues(t, 1, *envelope.ID)
			break
		}
		if envelope.Method != "session/update" {
			continue
		}
		if envelope.Params.Update.SessionUpdate == acp.SessionUpdateToolCallUpdate && envelope.Params.Update.ToolCallID == "tool-agent-1" {
			seenSubToolUpdate = true
			require.Equal(t, sessionID, envelope.Params.SessionID)
			require.Equal(t, "agent", envelope.Params.Update.Title)
			require.Equal(t, subSessionID, envelope.Params.Update.ChildSessionID)
			require.Equal(t, "tool-agent-1", envelope.Params.Update.ParentToolCallID)
			require.NotNil(t, envelope.Params.Update.SubtaskResult)
			require.Equal(t, "completed", envelope.Params.Update.SubtaskResult.Status)
		}
	}
	require.True(t, seenSubToolUpdate)
}

func TestSessionPrompt_ForwardsReducerFromToolResult(t *testing.T) {
	t.Parallel()

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()

	sessionID := "reducer-session"

	app := newFakeApp()
	app.sessions.sessions[sessionID] = session.Session{ID: sessionID, Title: "reducer"}

	handler := acp.NewHandler(app)
	server := acp.NewServerWithIO(handler, inReader, outWriter)
	handler.SetServer(server)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer inWriter.Close()

	go func() { _ = server.Serve(ctx) }()

	outScanner := bufio.NewScanner(outReader)

	_, err := fmt.Fprint(inWriter, buildRequest(t, 1, "session/prompt", acp.PromptParams{
		SessionID: sessionID,
		Prompt:    []acp.ContentBlock{{Type: "text", Text: "publish-reducer-update"}},
	}))
	require.NoError(t, err)

	seenReducerUpdate := false
	for outScanner.Scan() {
		line := outScanner.Bytes()
		var envelope struct {
			ID     *int64                        `json:"id"`
			Method string                        `json:"method"`
			Params acp.SessionUpdateNotification `json:"params"`
		}
		require.NoError(t, json.Unmarshal(line, &envelope))
		if envelope.ID != nil {
			require.EqualValues(t, 1, *envelope.ID)
			break
		}
		if envelope.Method != "session/update" {
			continue
		}
		update := envelope.Params.Update
		if update.SessionUpdate == acp.SessionUpdateToolCallUpdate && update.ToolCallID == "tool-reducer-1" {
			seenReducerUpdate = true
			require.NotNil(t, update.Reducer)
			require.Equal(t, "Execution finished", update.Reducer.Summary)
			require.Equal(t, []string{"dist/app"}, update.Reducer.Artifacts)
			require.Equal(t, []string{"network flakiness"}, update.Reducer.Risks)
			require.Equal(t, []string{"monitor"}, update.Reducer.NextActions)
			require.Equal(t, "high", update.Reducer.Confidence)
		}
	}
	require.True(t, seenReducerUpdate)
}

func TestSessionPrompt_ForwardsNonBashRuntimeCompletion(t *testing.T) {
	t.Parallel()

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()

	sessionID := "runtime-session"

	app := newFakeApp()
	app.sessions.sessions[sessionID] = session.Session{ID: sessionID, Title: "runtime"}

	handler := acp.NewHandler(app)
	server := acp.NewServerWithIO(handler, inReader, outWriter)
	handler.SetServer(server)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer inWriter.Close()

	go func() { _ = server.Serve(ctx) }()

	outScanner := bufio.NewScanner(outReader)

	_, err := fmt.Fprint(inWriter, buildRequest(t, 1, "session/prompt", acp.PromptParams{
		SessionID: sessionID,
		Prompt:    []acp.ContentBlock{{Type: "text", Text: "publish-runtime-complete"}},
	}))
	require.NoError(t, err)

	seenRuntimeUpdate := false
	for outScanner.Scan() {
		line := outScanner.Bytes()
		var envelope struct {
			ID     *int64                        `json:"id"`
			Method string                        `json:"method"`
			Params acp.SessionUpdateNotification `json:"params"`
		}
		require.NoError(t, json.Unmarshal(line, &envelope))
		if envelope.ID != nil {
			require.EqualValues(t, 1, *envelope.ID)
			break
		}
		if envelope.Method != "session/update" {
			continue
		}
		update := envelope.Params.Update
		if update.SessionUpdate == acp.SessionUpdateToolCallUpdate && update.ToolCallID == "tool-fetch-1" {
			seenRuntimeUpdate = true
			require.Equal(t, acp.ToolCallStatusCompleted, update.Status)
			require.Equal(t, "fetch", update.Title)
		}
	}
	require.True(t, seenRuntimeUpdate)
}

func TestSessionPrompt_ForwardsTimelineEvents(t *testing.T) {
	t.Parallel()

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()

	sessionID := "timeline-session"

	app := newFakeApp()
	app.sessions.sessions[sessionID] = session.Session{ID: sessionID, Title: "timeline", PermissionMode: session.PermissionModeAuto}

	handler := acp.NewHandler(app)
	server := acp.NewServerWithIO(handler, inReader, outWriter)
	handler.SetServer(server)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer inWriter.Close()

	go func() { _ = server.Serve(ctx) }()

	outScanner := bufio.NewScanner(outReader)
	_, err := fmt.Fprint(inWriter, buildRequest(t, 1, "session/prompt", acp.PromptParams{
		SessionID: sessionID,
		Prompt:    []acp.ContentBlock{{Type: "text", Text: "publish-timeline-events"}},
	}))
	require.NoError(t, err)

	seenMode := false
	seenChildEvent := false
	seenTimeline := false
	seenResponse := false
	for outScanner.Scan() {
		line := outScanner.Bytes()
		var envelope struct {
			ID     *int64                        `json:"id"`
			Method string                        `json:"method"`
			Params acp.SessionUpdateNotification `json:"params"`
		}
		require.NoError(t, json.Unmarshal(line, &envelope))
		if envelope.ID != nil {
			require.EqualValues(t, 1, *envelope.ID)
			seenResponse = true
			if seenTimeline && seenMode && seenChildEvent {
				break
			}
			continue
		}
		if envelope.Method != "session/update" {
			continue
		}
		update := envelope.Params.Update
		if update.SessionUpdate != acp.SessionUpdateTimelineEvent || update.TimelineEvent == nil {
			continue
		}
		seenTimeline = true
		switch update.TimelineEvent.Type {
		case "mode_changed":
			seenMode = true
			require.Equal(t, "plan", update.TimelineEvent.CollaborationMode)
			require.Equal(t, "yolo", update.TimelineEvent.PermissionMode)
		case "child_session_started":
			seenChildEvent = true
			require.Equal(t, "child-1", update.TimelineEvent.ChildSessionID)
		case "child_session_finished":
			seenChildEvent = true
			require.Equal(t, "child-1", update.TimelineEvent.ChildSessionID)
			require.Equal(t, "completed", update.TimelineEvent.Status)
		}
		if seenResponse && seenTimeline && seenMode && seenChildEvent {
			break
		}
	}
	require.True(t, seenResponse)
	require.True(t, seenTimeline)
	require.True(t, seenMode)
	require.True(t, seenChildEvent)
}

func TestPermissionBridgeForwardsAuthoritySessionID(t *testing.T) {
	t.Parallel()

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()

	handler := acp.NewHandler(newFakeApp())
	server := acp.NewServerWithIO(handler, inReader, outWriter)
	handler.SetServer(server)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer inWriter.Close()

	go func() { _ = server.Serve(ctx) }()

	base := permission.NewPermissionService(".", false, nil)
	perms := &recordingPermissionService{Service: base, requests: pubsub.NewBroker[permission.PermissionRequest]()}
	go acp.RunPermissionBridge(ctx, perms, server)
	require.Eventually(t, func() bool {
		return perms.requests.GetSubscriberCount() > 0
	}, time.Second, 10*time.Millisecond)

	req := permission.PermissionRequest{
		ID:                 "perm-1",
		SessionID:          "child-session",
		AuthoritySessionID: "parent-session",
		ToolCallID:         "tool-1",
		ToolName:           "write",
		Action:             "write",
		Params:             map[string]any{"file_path": "a.txt"},
		Path:               ".",
	}

	perms.PublishRequest(req)

	scanner := bufio.NewScanner(outReader)
	require.True(t, scanner.Scan(), "expected request_permission call")
	line := scanner.Bytes()

	var outbound struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      *int64          `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	require.NoError(t, json.Unmarshal(line, &outbound))
	require.Equal(t, "session/request_permission", outbound.Method)
	require.NotNil(t, outbound.ID)

	var params acp.RequestPermissionParams
	require.NoError(t, json.Unmarshal(outbound.Params, &params))
	require.Equal(t, "child-session", params.SessionID)
	require.Equal(t, "parent-session", params.AuthoritySessionID)

	response, err := json.Marshal(acp.Response{
		JSONRPC: "2.0",
		ID:      outbound.ID,
		Result:  mustJSONRaw(t, acp.RequestPermissionResult{Outcome: acp.RequestPermissionOutcome{Outcome: "selected", OptionID: params.Options[0].OptionID}}),
	})
	require.NoError(t, err)
	_, err = fmt.Fprintln(inWriter, string(response))
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return perms.lastGranted.ToolCallID == "tool-1"
	}, time.Second, 10*time.Millisecond)
}

func mustJSONRaw(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
