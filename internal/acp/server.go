// Package acp provides ACP (Agent Client Protocol) server support for Crush.
package acp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

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

func (d *dispatcher) Logout(_ context.Context, _ acp.LogoutRequest) (acp.LogoutResponse, error) {
	return acp.LogoutResponse{}, errors.New("logout not supported")
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
// message history as session/update notifications (user_message_chunk /
// agent_message_chunk) before returning, satisfying the spec requirement that
// session/load MUST stream the full history before its response.
func (d *dispatcher) LoadSession(ctx context.Context, req acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	sessionID := string(req.SessionId)
	msgs, err := d.app.Messages.List(ctx, sessionID)
	if err != nil {
		return acp.LoadSessionResponse{}, fmt.Errorf("list messages: %w", err)
	}
	for _, m := range msgs {
		var sb strings.Builder
		for _, part := range m.Parts {
			if tc, ok := part.(message.TextContent); ok {
				sb.WriteString(tc.Text)
			}
		}
		text := sb.String()
		if text == "" {
			continue
		}
		var update acp.SessionUpdate
		switch m.Role {
		case message.User:
			update = acp.UpdateUserMessageText(text)
		case message.Assistant:
			update = acp.UpdateAgentMessageText(text)
		default:
			continue
		}
		note := acp.SessionNotification{SessionId: req.SessionId, Update: update}
		if err := d.conn.SessionUpdate(ctx, note); err != nil {
			return acp.LoadSessionResponse{}, fmt.Errorf("stream loaded message: %w", err)
		}
	}
	return acp.LoadSessionResponse{}, nil
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
