package acp

import (
	"context"
	"errors"
	"fmt"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/cwd"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/llm/agent"
	"github.com/coder/acp-go-sdk"
	"log/slog"
	"strings"
	"time"
)

type Agent struct {
	app     *app.App
	conn    *acp.AgentSideConnection
	client  acp.ClientCapabilities
	debug   bool
	yolo    bool
	dataDir string
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
	slog.Info("Initialize")
	a.client = params.ClientCapabilities
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
		a.NotifySlashCommands(ctx, resp.SessionId)
	}()
	return resp, nil
}

func (a *Agent) NotifySlashCommands(ctx context.Context, sessionId acp.SessionId) {
	notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := a.conn.SessionUpdate(notifyCtx, acp.SessionNotification{
		SessionId: sessionId,
		Update: acp.SessionUpdate{
			AvailableCommandsUpdate: &acp.SessionUpdateAvailableCommandsUpdate{
				AvailableCommands: a.availableCommands(),
			},
		},
	}); err != nil {
		slog.Error("failed to send available-commands update", "error", err)
	}
}

func (a *Agent) availableCommands() []acp.AvailableCommand {
	out := make([]acp.AvailableCommand, 0, len(slashRegistry))
	for name := range slashRegistry {
		out = append(out, acp.AvailableCommand{
			Name: name,
			Input: &acp.AvailableCommandInput{
				&acp.UnstructuredCommandInput{
					Hint: slashRegistry[name].Help(),
				},
			},
		})
	}

	return out
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

func (a *Agent) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	slog.Info("Prompt")
	var err error

	if _, err = a.app.Sessions.Get(ctx, string(params.SessionId)); err != nil {
		err = fmt.Errorf("session %s not found", params.SessionId)
	} else {
		// FIXME: Add support for different types of content (image, audio and etc)
		cmd, txt := parseTextPrompt(params.Prompt)
		if cmd != "" { // slash-command
			err = a.handleSlash(ctx, cmd, txt, params)
		} else { // normal LLM turn
			err = a.streamAgentRun(ctx, params.SessionId, txt, false)
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

func (a *Agent) handleSlash(ctx context.Context, name, text string, params acp.PromptRequest) error {
	slog.Info("Handle slash command", "name", name)

	cmd, ok := slashRegistry[name]
	if !ok {
		return fmt.Errorf("unknown command: %s", cmd)
	}

	return cmd.Exec(ctx, a, text, params)
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

// streamAgentRun streams whatever CoderAgent produces for the given prompt
// and returns only when the run finishes (or context is cancelled).
func (a *Agent) streamAgentRun(ctx context.Context, sessionId acp.SessionId, prompt string, modified bool) error {
	slog.Debug("Stream agent's run.")
	done, err := a.app.CoderAgent.Run(ctx, string(sessionId), prompt)
	if err != nil {
		return err
	}

	events := a.app.Messages.Subscribe(ctx)
	if modified {
		// in case if prompt was modified (e.g. slash command), we do not need to worry about duplicates
		prompt = ""
	}
	updatesIter := newUpdatesIterator(prompt)

	for {
		select {
		case result := <-done: // agent finished
			// nil, context.Canceled, or agent.ErrRequestCancelled
			return result.Error
		case ev, ok := <-events:
			if !ok {
				// channel closed → agent finished cleanly
				return nil
			}

			for update := range updatesIter.next(&ev.Payload) {
				if err = a.conn.SessionUpdate(ctx, acp.SessionNotification{
					SessionId: sessionId,
					Update:    *update,
				}); err != nil {
					slog.Error("error sending update", err)
					continue
				}
			}
		}
	}
}

// supposedly only one Text block from user?
func parseTextPrompt(prompt []acp.ContentBlock) (cmd, rest string) {
	for _, b := range prompt {
		if b.Text == nil {
			continue
		}
		txt := strings.TrimSpace(b.Text.Text)
		if txt == "" || txt[0] != '/' {
			return "", txt // normal message
		}

		// slash command
		afterSlash := txt[1:]
		i := strings.IndexByte(afterSlash, ' ')
		if i == -1 {
			return afterSlash, "" // "/cmd"
		}

		return afterSlash[:i], strings.TrimSpace(afterSlash[i:])
	}

	return "", ""
}
