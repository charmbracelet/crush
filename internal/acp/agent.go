package acp

import (
	"context"
	"log/slog"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/coder/acp-go-sdk"
)

// Agent implements the acp.Agent interface to handle ACP protocol methods.
type Agent struct {
	app   *app.App
	conn  *acp.AgentSideConnection
	sinks *csync.Map[string, *Sink]
}

// Compile-time interface checks.
var (
	_ acp.Agent             = (*Agent)(nil)
	_ acp.AgentLoader       = (*Agent)(nil)
	_ acp.AgentExperimental = (*Agent)(nil)
)

// NewAgent creates a new ACP agent backed by a Crush app instance.
func NewAgent(app *app.App) *Agent {
	return &Agent{
		app:   app,
		sinks: csync.NewMap[string, *Sink](),
	}
}

// SetAgentConnection stores the connection for sending notifications.
func (a *Agent) SetAgentConnection(conn *acp.AgentSideConnection) {
	a.conn = conn
}

// Initialize handles the ACP initialize request.
func (a *Agent) Initialize(ctx context.Context, params acp.InitializeRequest) (acp.InitializeResponse, error) {
	slog.Debug("ACP Initialize", "protocol_version", params.ProtocolVersion)
	return acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersionNumber,
		AgentCapabilities: acp.AgentCapabilities{
			LoadSession: true,
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

// Authenticate handles authentication requests (stub for local stdio).
func (a *Agent) Authenticate(ctx context.Context, params acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	slog.Debug("ACP Authenticate")
	return acp.AuthenticateResponse{}, nil
}

// NewSession creates a new Crush session.
func (a *Agent) NewSession(ctx context.Context, params acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	slog.Info("ACP NewSession", "cwd", params.Cwd)

	sess, err := a.app.Sessions.Create(ctx, "ACP Session")
	if err != nil {
		return acp.NewSessionResponse{}, err
	}

	// Create and start the event sink to stream updates to this session.
	// Use a background context since the sink needs to outlive the NewSession
	// request.
	sink := NewSink(context.Background(), a.conn, sess.ID)
	sink.Start(a.app.Messages, a.app.Permissions, a.app.Sessions)
	a.sinks.Set(sess.ID, sink)

	return acp.NewSessionResponse{
		SessionId: acp.SessionId(sess.ID),
	}, nil
}

// LoadSession loads an existing session to resume a previous conversation.
func (a *Agent) LoadSession(ctx context.Context, params acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	sessionID := string(params.SessionId)
	slog.Info("ACP LoadSession", "session_id", sessionID)

	// Verify the session exists.
	session, err := a.app.Sessions.Get(ctx, sessionID)
	if err != nil {
		return acp.LoadSessionResponse{}, err
	}

	// Create and start the event sink for future updates.
	sink := NewSink(context.Background(), a.conn, session.ID)
	sink.Start(a.app.Messages, a.app.Permissions, a.app.Sessions)
	a.sinks.Set(session.ID, sink)

	// Load and replay historical messages to the client.
	messages, err := a.app.Messages.List(ctx, sessionID)
	if err != nil {
		return acp.LoadSessionResponse{}, err
	}

	for _, msg := range messages {
		if err := a.replayMessage(ctx, sessionID, msg); err != nil {
			slog.Error("Failed to replay message", "message_id", msg.ID, "error", err)
		}
	}

	return acp.LoadSessionResponse{}, nil
}

// SetSessionMode handles mode switching (stub - Crush doesn't have modes yet).
func (a *Agent) SetSessionMode(ctx context.Context, params acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	slog.Debug("ACP SetSessionMode", "mode_id", params.ModeId)
	return acp.SetSessionModeResponse{}, nil
}

// SetSessionModel handles model switching (stub - model selection not yet wired).
func (a *Agent) SetSessionModel(ctx context.Context, params acp.SetSessionModelRequest) (acp.SetSessionModelResponse, error) {
	slog.Debug("ACP SetSessionModel", "session_id", params.SessionId, "model_id", params.ModelId)
	return acp.SetSessionModelResponse{}, nil
}

// Prompt handles a prompt request by running the agent.
func (a *Agent) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	slog.Info("ACP Prompt", "session_id", params.SessionId)

	// Extract text from content blocks.
	var prompt string
	for _, block := range params.Prompt {
		if block.Text != nil {
			prompt += block.Text.Text
		}
	}

	if prompt == "" {
		return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
	}

	// Run the agent.
	_, err := a.app.AgentCoordinator.Run(ctx, string(params.SessionId), prompt)
	if err != nil {
		if ctx.Err() != nil {
			return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
		}
		return acp.PromptResponse{}, err
	}

	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

// Cancel handles cancellation of an in-flight prompt.
func (a *Agent) Cancel(ctx context.Context, params acp.CancelNotification) error {
	slog.Info("ACP Cancel", "session_id", params.SessionId)
	a.app.AgentCoordinator.Cancel(string(params.SessionId))
	return nil
}

// replayMessage sends a historical message to the client via session updates.
func (a *Agent) replayMessage(ctx context.Context, sessionID string, msg message.Message) error {
	for _, part := range msg.Parts {
		update := a.translateHistoryPart(msg.Role, part)
		if update == nil {
			continue
		}

		if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: acp.SessionId(sessionID),
			Update:    *update,
		}); err != nil {
			return err
		}
	}
	return nil
}

// translateHistoryPart converts a message part to an ACP session update for
// history replay. Unlike streaming updates, this sends full content rather
// than deltas.
func (a *Agent) translateHistoryPart(role message.MessageRole, part message.ContentPart) *acp.SessionUpdate {
	switch p := part.(type) {
	case message.TextContent:
		if p.Text == "" {
			return nil
		}
		var update acp.SessionUpdate
		if role == message.User {
			update = acp.UpdateUserMessageText(p.Text)
		} else {
			update = acp.UpdateAgentMessageText(p.Text)
		}
		return &update

	case message.ReasoningContent:
		if p.Thinking == "" {
			return nil
		}
		update := acp.UpdateAgentThoughtText(p.Thinking)
		return &update

	case message.ToolCall:
		// For history replay, send the tool call as completed with full input.
		opts := []acp.ToolCallStartOpt{
			acp.WithStartStatus(acp.ToolCallStatusCompleted),
			acp.WithStartKind(toolKind(p.Name)),
		}
		if input := parseToolInput(p.Input); input != nil {
			if input.Path != "" {
				opts = append(opts, acp.WithStartLocations([]acp.ToolCallLocation{{Path: input.Path}}))
			}
			opts = append(opts, acp.WithStartRawInput(input.Raw))
		}
		title := p.Name
		if input := parseToolInput(p.Input); input != nil && input.Title != "" {
			title = input.Title
		}
		update := acp.StartToolCall(acp.ToolCallId(p.ID), title, opts...)
		return &update

	case message.ToolResult:
		status := acp.ToolCallStatusCompleted
		if p.IsError {
			status = acp.ToolCallStatusFailed
		}
		content := []acp.ToolCallContent{acp.ToolContent(acp.TextBlock(p.Content))}
		update := acp.UpdateToolCall(
			acp.ToolCallId(p.ToolCallID),
			acp.WithUpdateStatus(status),
			acp.WithUpdateContent(content),
		)
		return &update

	default:
		return nil
	}
}
