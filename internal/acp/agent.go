package acp

import (
	"context"
	"errors"
	"fmt"
	"github.com/charmbracelet/crush/internal/acp/terminal"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/cwd"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/llm/agent"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/coder/acp-go-sdk"
	"log/slog"
	"strings"
	"time"
)

type Agent struct {
	app        *app.App
	conn       *acp.AgentSideConnection
	terminals  *terminal.Service
	sink       *agentEventSink
	promptDone chan any
	client     acp.ClientCapabilities
	debug      bool
	yolo       bool
	dataDir    string
}

var (
	_ acp.Agent             = (*Agent)(nil)
	_ acp.AgentLoader       = (*Agent)(nil)
	_ acp.AgentExperimental = (*Agent)(nil)
)

func NewAgent(debug bool, yolo bool, dataDir string) (*Agent, error) {
	return &Agent{
		debug:   debug,
		yolo:    yolo,
		dataDir: dataDir,
	}, nil
}

func (a *Agent) SetSessionMode(ctx context.Context, params acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	slog.Info("SetSessionMode")
	return acp.SetSessionModeResponse{}, nil
}

func (a *Agent) SetSessionModel(ctx context.Context, params acp.SetSessionModelRequest) (acp.SetSessionModelResponse, error) {
	slog.Info("SetSessionModel")
	return acp.SetSessionModelResponse{}, nil
}

func (a *Agent) SetAgentConnection(conn *acp.AgentSideConnection) { a.conn = conn }

func (a *Agent) Initialize(ctx context.Context, params acp.InitializeRequest) (acp.InitializeResponse, error) {
	slog.Debug("Initialize", "params", params)
	a.client = params.ClientCapabilities
	a.terminals = terminal.NewService(a.conn, a.client.Terminal)

	return acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersionNumber,
		AgentCapabilities: acp.AgentCapabilities{
			LoadSession: false,
			McpCapabilities: acp.McpCapabilities{
				Http: false,
				Sse:  false,
			},
			PromptCapabilities: acp.PromptCapabilities{
				EmbeddedContext: true,
				Audio:           false,
				Image:           false,
			},
		},
	}, nil
}

func (a *Agent) NewSession(ctx context.Context, params acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	slog.Info("New session requested...")
	appInstance, err := a.setupApp(ctx, params)
	if err != nil {
		return acp.NewSessionResponse{}, err
	}
	a.app = appInstance
	a.sink = newAgentSink(ctx, a)
	a.promptDone = make(chan any)
	close(a.promptDone) // first prompt may run straight away

	go app.Subscribe[any](appInstance, a.sink)

	s, err := a.app.Sessions.Create(ctx, "New ACP Session")
	if err != nil {
		return acp.NewSessionResponse{}, err
	}

	// TODO: send models/modes
	//models := a.app.Config().Models
	resp := acp.NewSessionResponse{
		Models:    nil,
		Modes:     nil,
		SessionId: acp.SessionId(s.ID),
	}

	go func() {
		_ = a.NotifySlashCommands(ctx, resp.SessionId, defaultSlashCommands)
	}()

	// E.g. we can read remote file like this
	//go func() {
	//	r, _ := a.ReadTextFile(ctx, resp.SessionId, "/Users/andrei/Projects/cache-decorator/src/cache_decorator/storages/memory.py", 0, 0)
	//}()

	// E.g. we can write remote file like this
	//go func() {
	//	_ = a.WriteTextFile(ctx, resp.SessionId, "/Users/andrei/Projects/cache-decorator/src/cache_decorator/storages/memory1.py", "Hello here")
	//}()

	// E.g. we can call terminal command on client side like this
	//go func() {
	//	if t, err := a.terminals.Create(ctx, resp.SessionId, "ls", terminal.WithArgs("-la")); err == nil {
	//		_ = t.EmbedInToolCalls(ctx, a.conn)
	//	}
	//
	//}()

	return resp, nil
}

func (a *Agent) NotifySlashCommands(ctx context.Context, sessionId acp.SessionId, commands SlashCommandRegistry) error {
	notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := a.conn.SessionUpdate(notifyCtx, acp.SessionNotification{
		SessionId: sessionId,
		Update: acp.SessionUpdate{
			AvailableCommandsUpdate: &acp.SessionUpdateAvailableCommandsUpdate{
				AvailableCommands: AvailableCommands(commands),
			},
		},
	}); err != nil {
		slog.Error("failed to send available-commands update", "error", err)
		return err
	}

	return nil
}

func (a *Agent) Authenticate(ctx context.Context, _ acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	slog.Info("Authenticate")
	return acp.AuthenticateResponse{}, nil
}

func (a *Agent) LoadSession(ctx context.Context, _ acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	slog.Info("LoadSession")
	return acp.LoadSessionResponse{}, nil
}

func (a *Agent) Cancel(ctx context.Context, params acp.CancelNotification) error {
	slog.Info("Cancel")
	_, err := a.app.Sessions.Get(ctx, string(params.SessionId))
	if err != nil {
		return err
	}

	if a.app.CoderAgent != nil {
		a.app.CoderAgent.Cancel(string(params.SessionId))
	}

	return nil
}

func (a *Agent) RunPrompt(ctx context.Context, prompt string, params acp.PromptRequest) error {
	sid := string(params.SessionId)
	if a.app.CoderAgent.IsSessionBusy(sid) {
		slog.Info("Cancel previous prompt.")
		a.app.CoderAgent.Cancel(sid)
		<-a.promptDone // wait until previous turn canceled
	}

	slog.Info("Process a new prompt.")
	done, err := a.app.CoderAgent.Run(ctx, string(params.SessionId), prompt)
	if err != nil {
		slog.Error("Cant run coder agent", "err", err)
		return err
	}

	a.promptDone = make(chan any)
	defer close(a.promptDone)
	for {
		select {
		case result := <-done:
			// nil, context.Canceled, or agent.ErrRequestCancelled
			return result.Error
		}
	}
}

func (a *Agent) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	var err error

	sid := string(params.SessionId)
	if _, err = a.app.Sessions.Get(ctx, sid); err != nil {
		err = fmt.Errorf("session %s not found", params.SessionId)
	} else {
		prompt := Prompt(params.Prompt).String()
		a.sink.LastUserPrompt(prompt)

		// FIXME: Add support for different types of content (image, audio and etc)
		name, text := parseSlash(prompt)
		if name != "" { // slash-command
			if cmd := defaultSlashCommands.Get(name); cmd != nil {
				slog.Info("Slash command requested", "cmd", name)
				err = cmd.Exec(ctx, a, text, params)
			}
		} else { // normal LLM turn
			err = a.RunPrompt(ctx, prompt, params)
		}
	}

	switch {
	case err == nil:
		return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
	case errors.Is(err, context.Canceled), errors.Is(err, agent.ErrRequestCancelled):
		return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
	default:
		return acp.PromptResponse{}, err // real failure
	}
}

// ReadTextFile allows Agents to read text file contents from the Client’s filesystem, including unsaved changes in the editor.
func (a *Agent) ReadTextFile(ctx context.Context, sessionId acp.SessionId, path string, line int, limit int) (string, error) {
	if !a.client.Fs.ReadTextFile {
		return "", errors.New("client does not support reading of text files")
	}

	var pLine, pLimit *int
	if line > 0 {
		pLine = acp.Ptr(line)
	}

	if limit > 0 {
		pLimit = acp.Ptr(limit)
	}

	if resp, err := a.conn.ReadTextFile(ctx, acp.ReadTextFileRequest{
		SessionId: sessionId,
		Path:      path,
		Line:      pLine,
		Limit:     pLimit,
	}); err != nil {
		slog.Error("could not read remote file", "error", err)
		return "", err
	} else {
		return resp.Content, nil
	}
}

// WriteTextFile allows Agents to write or update text files in the Client’s filesystem.
func (a *Agent) WriteTextFile(ctx context.Context, sessionId acp.SessionId, path string, content string) error {
	if !a.client.Fs.WriteTextFile {
		return errors.New("client does not support writing of text files")
	}

	if _, err := a.conn.WriteTextFile(ctx, acp.WriteTextFileRequest{
		SessionId: sessionId,
		Path:      path,
		Content:   content,
	}); err != nil {
		slog.Error("could not write to remote file", "error", err)
		return err
	}

	return nil
}

func (a *Agent) setupApp(ctx context.Context, params acp.NewSessionRequest) (*app.App, error) {
	cwDir, err := cwd.Resolve(params.Cwd)
	if err != nil {
		return nil, err
	}

	cfg, err := config.Init(cwDir, a.dataDir, a.debug)
	if err != nil {
		return nil, err
	}

	if cfg.Permissions == nil {
		cfg.Permissions = &config.Permissions{}
	}
	cfg.Permissions.SkipRequests = a.yolo

	if err := cwd.CreateDotCrushDir(cfg.Options.DataDirectory); err != nil {
		return nil, err
	}

	// Connect to DB; this will also run migrations.
	conn, err := db.Connect(ctx, cfg.Options.DataDirectory)
	if err != nil {
		return nil, err
	}

	appInstance, err := app.New(ctx, conn, cfg)
	if err != nil {
		slog.Error("Failed to create app instance", "error", err)
		return nil, err
	}

	return appInstance, nil
}

func (a *Agent) RequestPermission(ctx context.Context, req permission.PermissionRequest) {
	slog.Info("RequestPermission", "req", req)
	payload := acp.RequestPermissionRequest{
		SessionId: acp.SessionId(req.SessionID),
		ToolCall: acp.ToolCallUpdate{
			ToolCallId: acp.ToolCallId(req.ToolCallID),
			Title:      acp.Ptr(req.Description),
			Kind:       acp.Ptr(acp.ToolKindEdit),
			Status:     acp.Ptr(acp.ToolCallStatusPending),
			Locations:  []acp.ToolCallLocation{{Path: req.Path}},
			RawInput:   req.Params,
		}, Options: []acp.PermissionOption{
			{Kind: acp.PermissionOptionKindAllowOnce, Name: "Allow this change", OptionId: acp.PermissionOptionId("allow")},
			{Kind: acp.PermissionOptionKindRejectOnce, Name: "Skip this change", OptionId: acp.PermissionOptionId("reject")},
		}}

	result, err := a.conn.RequestPermission(ctx, payload)
	if err != nil {
		slog.Error("error sending permission request", err)
		return
	}

	if result.Outcome.Selected != nil {
		a.app.Permissions.Grant(req)
	} else {
		a.app.Permissions.Deny(req)
	}
}

// parseSlash parses "input" and returns:
//
//	("", input)            – not a slash command
//	("cmd", "rest")        – "/cmd rest"
func parseSlash(input string) (cmd, rest string) {
	input = strings.TrimSpace(input)
	if input == "" || input[0] != '/' {
		return "", input
	}
	after := input[1:]
	if i := strings.IndexByte(after, ' '); i == -1 {
		return after, ""
	} else {
		return after[:i], strings.TrimSpace(after[i:])
	}
}

type Prompt []acp.ContentBlock

func (p Prompt) String() string {
	var sb strings.Builder
	for _, b := range p {
		if b.Text != nil {
			sb.WriteString(b.Text.Text)
		}
	}
	return sb.String()
}
