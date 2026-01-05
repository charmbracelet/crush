// Package acp implements the Agent-Client Protocol server for Crush.
//
// ACP allows external clients (web, desktop, mobile) to drive Crush as an
// agent server over stdio using JSON-RPC.
package acp

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/coder/acp-go-sdk"
)

// Server manages the ACP connection lifecycle.
type Server struct {
	ctx    context.Context
	cancel context.CancelFunc
	agent  *Agent
}

// NewServer creates a new ACP server.
func NewServer(ctx context.Context) *Server {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, os.Kill, syscall.SIGTERM)
	return &Server{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Run starts the ACP server and blocks until the connection closes.
func (s *Server) Run(agent *Agent) error {
	s.agent = agent
	slog.Info("Starting ACP server")

	conn := acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin)
	conn.SetLogger(slog.Default())
	agent.SetAgentConnection(conn)

	select {
	case <-conn.Done():
		slog.Debug("ACP client disconnected")
	case <-s.ctx.Done():
		slog.Debug("ACP server received shutdown signal")
	}

	return nil
}

// Shutdown performs graceful shutdown.
func (s *Server) Shutdown() {
	s.cancel()
}
