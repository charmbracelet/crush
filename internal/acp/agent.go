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
	"github.com/charmbracelet/crush/internal/message"
	"github.com/coder/acp-go-sdk"
	"log/slog"
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

	//TODO: should https://agentclientprotocol.com/protocol/slash-commands#availablecommand after creating a session

	models := a.app.Config().Models
	print(models)

	return acp.NewSessionResponse{
		Models:    nil,
		Modes:     nil,
		SessionId: acp.SessionId(s.ID),
	}, nil
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
	_, err := a.app.Sessions.Get(ctx, string(params.SessionId))
	if err != nil {
		return acp.PromptResponse{}, fmt.Errorf("session %s not found", string(params.SessionId))
	}

	content := ""
	for _, cont := range params.Prompt {
		if cont.Text != nil {
			content += cont.Text.Text + " "
		}
	}

	done, err := a.app.CoderAgent.Run(context.Background(), string(params.SessionId), content)
	if err != nil {
		return acp.PromptResponse{}, err
	}

	messageEvents := a.app.Messages.Subscribe(ctx)

	// Track sent content to only send deltas
	var lastTextSent string
	var lastThinkingSent string

	for {
		select {
		case result := <-done:
			if result.Error != nil {
				if errors.Is(result.Error, context.Canceled) || errors.Is(result.Error, agent.ErrRequestCancelled) {
					slog.Info("agent processing cancelled", "session_id", params.SessionId)
					return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
				}
				return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
			}
		case event, ok := <-messageEvents:
			if !ok {
				// Stream closed, agent finished
				return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
			}

			switch event.Payload.Role {
			case message.Assistant:
				for _, part := range event.Payload.Parts {
					switch part := part.(type) {
					case message.ReasoningContent:
						// Only send the delta (new thinking content)
						if len(part.Thinking) > len(lastThinkingSent) {
							delta := part.Thinking[len(lastThinkingSent):]
							if a.conn != nil && delta != "" {
								if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
									SessionId: params.SessionId,
									Update: acp.SessionUpdate{
										AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{
											SessionUpdate: "agent_thought_chunk",
											Content: acp.ContentBlock{
												Text: &acp.ContentBlockText{
													Text: delta,
													Type: "text",
												},
											},
										},
									},
								}); err != nil {
									slog.Error("error sending agent thought chunk", err)
									continue
								}
							}
							lastThinkingSent = part.Thinking
						}
					case message.BinaryContent:
					case message.ImageURLContent:
					case message.Finish:
					case message.TextContent:
						// Only send the delta (new text content)
						if len(part.Text) > len(lastTextSent) {
							delta := part.Text[len(lastTextSent):]
							if a.conn != nil && delta != "" {
								if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
									SessionId: params.SessionId,
									Update: acp.SessionUpdate{
										AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{
											SessionUpdate: "agent_message_chunk",
											Content: acp.ContentBlock{
												Text: &acp.ContentBlockText{
													Text: delta,
													Type: "text",
												},
											},
										},
									},
								}); err != nil {
									slog.Error("error sending agent text chunk", err)
									continue
								}
							}
							lastTextSent = part.Text
						}
					case message.ToolCall:
					case message.ToolResult:
					}
				}
			case message.System:
			case message.Tool:
			case message.User:
			}
		}
	}
}

func (a *Agent) setupApp(ctx context.Context, params acp.NewSessionRequest) (*app.App, error) {
	cwDir, err := cwd.ResolveCwd(params.Cwd)
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
