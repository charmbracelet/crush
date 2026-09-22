// Package acp provides ACP (Agent Client Protocol) server support for Crush.
package acp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	acp "github.com/coder/acp-go-sdk"
)

// Server is the ACP server adapter for Crush.
type Server struct {
	conn        *acp.AgentSideConnection
	permb       *permissionBridge
	permService permission.Service
	eventBridge *eventBridge
}

// Agent is the Crush-side interface the SDK calls into.
type Agent interface {
	Initialize(ctx context.Context, req acp.InitializeRequest) (acp.InitializeResponse, error)
	NewSession(ctx context.Context, req acp.NewSessionRequest) (acp.NewSessionResponse, error)
	ListSessions(ctx context.Context, req acp.ListSessionsRequest) (acp.ListSessionsResponse, error)
	CloseSession(ctx context.Context, req acp.CloseSessionRequest) (acp.CloseSessionResponse, error)
	ResumeSession(ctx context.Context, req acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error)
	Cancel(ctx context.Context, req acp.CancelNotification) error
	Prompt(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error)
	SetSessionMode(ctx context.Context, req acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error)
	SetSessionConfigOption(ctx context.Context, req acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error)
	Logout(ctx context.Context, req acp.LogoutRequest) (acp.LogoutResponse, error)
	UnstableDeleteSession(ctx context.Context, req acp.UnstableDeleteSessionRequest) (acp.UnstableDeleteSessionResponse, error)
}

// NewServer creates a new ACP server that connects to the given peer.
func NewServer(a *app.App, log *slog.Logger, stdin io.Reader, stdout io.Writer) *Server {
	ca := newCrushAgent(a, log)
	d := &dispatcher{agent: ca, app: a}
	conn := acp.NewAgentSideConnection(d, stdout, stdin)
	d.conn = conn
	ca.conn = conn

	// Create permission bridge and wrap the real service.
	permb := newPermissionBridge(conn, log)
	permService := newACPPermissionService(a.Permissions, permb)

	// Create event bridge for streaming notifications.
	eventBridge := newEventBridge(conn, a, log)
	ca.bridge = eventBridge

	return &Server{
		conn:        conn,
		permb:       permb,
		permService: permService,
		eventBridge: eventBridge,
	}
}

// PermissionService returns the wrapped permission service for use by tools.
func (s *Server) PermissionService() permission.Service {
	return s.permService
}

// StartEventBridge starts the event bridge in a goroutine. Call this before Start.
func (s *Server) StartEventBridge(ctx context.Context) {
	go s.eventBridge.Start(ctx)
}

// dispatcher forwards SDK Agent method calls to our Agent interface.
type dispatcher struct {
	agent Agent
	app   *app.App
	conn  *acp.AgentSideConnection
}

func (d *dispatcher) Authenticate(_ context.Context, _ acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, errors.New("auth not supported")
}

func (d *dispatcher) Logout(ctx context.Context, req acp.LogoutRequest) (acp.LogoutResponse, error) {
	return d.agent.Logout(ctx, req)
}

func (d *dispatcher) Initialize(ctx context.Context, req acp.InitializeRequest) (acp.InitializeResponse, error) {
	return d.agent.Initialize(ctx, req)
}

func (d *dispatcher) NewSession(ctx context.Context, req acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	return d.agent.NewSession(ctx, req)
}

func (d *dispatcher) ListSessions(ctx context.Context, req acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	return d.agent.ListSessions(ctx, req)
}

func (d *dispatcher) CloseSession(ctx context.Context, req acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	return d.agent.CloseSession(ctx, req)
}

func (d *dispatcher) ResumeSession(ctx context.Context, req acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	return d.agent.ResumeSession(ctx, req)
}

// LoadSession implements acp.AgentLoader. It replays the session's stored
// message history as session/update notifications before returning, satisfying
// the spec requirement that session/load MUST stream the entire conversation:
// user/agent text chunks, agent thought chunks, and tool calls with their
// terminal status and result content.
func (d *dispatcher) LoadSession(ctx context.Context, req acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	sessionID := string(req.SessionId)
	msgs, err := d.app.Messages.List(ctx, sessionID)
	if err != nil {
		return acp.LoadSessionResponse{}, fmt.Errorf("list messages: %w", err)
	}

	// started tracks tool calls whose tool_call start update was already sent,
	// mapped to their raw input JSON so file-modifying results can carry diff
	// content even when the call is missing from history.
	started := map[string]string{}
	send := func(update acp.SessionUpdate) error {
		note := acp.SessionNotification{SessionId: req.SessionId, Update: update}
		if len(req.Meta) > 0 {
			note.Meta = req.Meta
		}
		return d.conn.SessionUpdate(ctx, note)
	}
	emitToolResult := func(tr message.ToolResult) error {
		id := acp.ToolCallId(tr.ToolCallID)
		input, seen := started[tr.ToolCallID]
		if !seen {
			started[tr.ToolCallID] = ""
			if err := send(acp.StartToolCall(id, toolTitle(tr.Name, ""), acp.WithStartKind(toolKindFor(tr.Name)))); err != nil {
				return err
			}
		}
		// A todos result carries the full plan state; replay it (S4).
		if plan := planUpdateFor(tr); plan != nil {
			if err := send(acp.SessionUpdate{Plan: plan}); err != nil {
				return err
			}
		}
		status := acp.ToolCallStatusCompleted
		text := tr.Content
		if tr.IsError {
			status = acp.ToolCallStatusFailed
			text = "Error: " + text
		}
		var contents []acp.ToolCallContent
		if text != "" {
			contents = append(contents, acp.ToolContent(acp.TextBlock(text)))
		}
		if diff := diffContentFor(tr.Name, input, tr); diff != nil {
			contents = append(contents, *diff)
		}
		opts := []acp.ToolCallUpdateOpt{acp.WithUpdateStatus(status)}
		if len(contents) > 0 {
			opts = append(opts, acp.WithUpdateContent(contents))
		}
		return send(acp.UpdateToolCall(id, opts...))
	}

	for _, m := range msgs {
		for _, part := range m.Parts {
			var err error
			switch p := part.(type) {
			case message.TextContent:
				if p.Text == "" {
					continue
				}
				switch m.Role {
				case message.User:
					err = send(acp.UpdateUserMessageText(p.Text))
				case message.Assistant:
					err = send(acp.UpdateAgentMessageText(p.Text))
				}
			case message.ReasoningContent:
				if m.Role == message.Assistant && p.Thinking != "" {
					err = send(acp.UpdateAgentThoughtText(p.Thinking))
				}
			case message.ToolCall:
				if m.Role != message.Assistant || p.ID == "" {
					continue
				}
				started[p.ID] = p.Input
				status := acp.ToolCallStatusInProgress
				if p.Finished {
					status = acp.ToolCallStatusCompleted
				}
				err = send(acp.StartToolCall(acp.ToolCallId(p.ID), toolTitle(p.Name, p.Input),
					acp.WithStartKind(toolKindFor(p.Name)),
					acp.WithStartStatus(status),
					acp.WithStartRawInput(toolRawInput(p.Input)),
				))
			case message.ToolResult:
				if m.Role != message.Tool || p.ToolCallID == "" {
					continue
				}
				err = emitToolResult(p)
			}
			if err != nil {
				return acp.LoadSessionResponse{}, fmt.Errorf("stream loaded message: %w", err)
			}
		}
	}
	return acp.LoadSessionResponse{}, nil
}

// UnstableDeleteSession forwards the session/delete request to the agent.
func (d *dispatcher) UnstableDeleteSession(ctx context.Context, req acp.UnstableDeleteSessionRequest) (acp.UnstableDeleteSessionResponse, error) {
	return d.agent.UnstableDeleteSession(ctx, req)
}

func (d *dispatcher) Cancel(ctx context.Context, req acp.CancelNotification) error {
	return d.agent.Cancel(ctx, req)
}

func (d *dispatcher) Prompt(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
	return d.agent.Prompt(ctx, req)
}

func (d *dispatcher) SetSessionMode(ctx context.Context, req acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return d.agent.SetSessionMode(ctx, req)
}

func (d *dispatcher) SetSessionConfigOption(ctx context.Context, req acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return d.agent.SetSessionConfigOption(ctx, req)
}

// SetLogger configures structured logging on the connection.
func (s *Server) SetLogger(l *slog.Logger) {
	s.conn.SetLogger(l)
}

// Start runs the ACP server loop. Blocks until the client disconnects.
func (s *Server) Start(ctx context.Context) error {
	done := s.conn.Done()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return errors.New("client disconnected")
	}
}
