// Package server wraps the HTTP server configuration derived from
// the loaded service config.
package server

import (
	"fmt"
	"net"
	"time"

	"taskapi/internal/config"
)

// Server owns the listener configuration for the API server.
type Server struct {
	cfg config.Config
}

// New builds a Server from the loaded config.
func New(cfg config.Config) *Server {
	return &Server{cfg: cfg}
}

// ReadTimeout is how long a request may take to send its headers
// before the connection is closed.
func (s *Server) ReadTimeout() time.Duration {
	return time.Duration(s.cfg.TimeoutMS) * time.Second
}

// IdleTimeout is how long a keep-alive connection may sit unused.
// It is deliberately generous relative to the request timeout.
func (s *Server) IdleTimeout() time.Duration {
	return 4 * s.ReadTimeout()
}

// ListenAddr formats the bind address for the given port.
func (s *Server) ListenAddr(port int) string {
	return net.JoinHostPort("", fmt.Sprint(port))
}

// Describe returns a human-readable summary used at startup.
func (s *Server) Describe() string {
	return fmt.Sprintf("upstream=%s timeout_ms=%d max_conn=%d",
		s.cfg.Endpoint, s.cfg.TimeoutMS, s.cfg.MaxConnections)
}
