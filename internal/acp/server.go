// Package acp provides ACP (Agent Client Protocol) server support for Crush.
//
// Crush acts as an ACP server over stdio using github.com/coder/acp-go-sdk.
package acp

import (
	"context"
	"errors"
	"io"
	"log/slog"

	acp "github.com/coder/acp-go-sdk"
)

// Server is the ACP server adapter for Crush.
type Server struct {
	conn *acp.AgentSideConnection
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

// stubAgent implements a no-op agent for testing.
type stubAgent struct{}

func (s *stubAgent) Initialize(_ context.Context, _ acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersionNumber,
		AgentCapabilities: acp.AgentCapabilities{
			LoadSession: false,
		},
	}, nil
}
func (s *stubAgent) NewSession(_ context.Context, req acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	return acp.NewSessionResponse{SessionId: acp.SessionId("test-" + req.Cwd)}, nil
}
func (s *stubAgent) ListSessions(_ context.Context, _ acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	return acp.ListSessionsResponse{}, nil
}
func (s *stubAgent) CloseSession(_ context.Context, req acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	return acp.CloseSessionResponse{}, nil
}
func (s *stubAgent) ResumeSession(_ context.Context, req acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	return acp.ResumeSessionResponse{}, nil
}
func (s *stubAgent) Cancel(_ context.Context, _ acp.CancelNotification) error { return nil }
func (s *stubAgent) Prompt(_ context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}
func (s *stubAgent) SetSessionMode(_ context.Context, _ acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, nil
}
func (s *stubAgent) SetSessionConfigOption(_ context.Context, _ acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, nil
}

// stubAuthAgent wraps a stubAgent with auth methods required by acp.Agent.
type stubAuthAgent struct {
	stubAgent
}

// Authenticate is required by acp.Agent.
func (s *stubAuthAgent) Authenticate(_ context.Context, _ acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, errors.New("auth not supported")
}

// Logout is required by acp.Agent.
func (s *stubAuthAgent) Logout(_ context.Context, _ acp.LogoutRequest) (acp.LogoutResponse, error) {
	return acp.LogoutResponse{}, errors.New("logout not supported")
}

// NewServer creates a new ACP server that connects to the given peer.
func NewServer(agent Agent, stdin io.Reader, stdout io.Writer) *Server {
	d := &dispatcher{agent}
	conn := acp.NewAgentSideConnection(d, stdout, stdin)
	return &Server{conn: conn}
}

// dispatcher forwards SDK calls to our Agent interface.
type dispatcher struct {
	Agent
}

// Authenticate is required by acp.Agent; Crush does not support auth yet.
func (d *dispatcher) Authenticate(_ context.Context, _ acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, errors.New("auth not supported")
}

// Logout is required by acp.Agent; Crush does not support auth yet.
func (d *dispatcher) Logout(_ context.Context, _ acp.LogoutRequest) (acp.LogoutResponse, error) {
	return acp.LogoutResponse{}, errors.New("logout not supported")
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
