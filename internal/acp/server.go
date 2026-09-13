// Package acp provides ACP (Agent Client Protocol) server support for Crush.
package acp

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/charmbracelet/crush/internal/app"
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
	agent := newCrushAgent(a, log)
	d := &dispatcher{agent: agent, app: a}
	conn := acp.NewAgentSideConnection(d, stdout, stdin)

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
