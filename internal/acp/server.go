package acp

import (
	"context"
	"fmt"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/coder/acp-go-sdk"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
)

type Server struct {
	ctx    context.Context
	cancel context.CancelFunc

	//TODO: Only stdio as transport is part of standard, http is still a draft, so only one agent until that
	agent   *Agent
	debug   bool
	yolo    bool
	dataDir string
}

func NewServer(ctx context.Context, debug bool, yolo bool, dataDir string) (*Server, error) {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, os.Kill, syscall.SIGTERM)
	log.Setup(
		filepath.Join(LogsDir(), "logs", fmt.Sprintf("%s.log", config.AppName)),
		debug,
	)

	return &Server{
		ctx:     ctx,
		cancel:  cancel,
		debug:   debug,
		yolo:    yolo,
		dataDir: dataDir,
	}, nil
}

func (s *Server) Run() error {
	agent, err := NewAgent(s.debug, s.yolo, s.dataDir)
	if err != nil {
		return err
	}
	s.agent = agent
	slog.Info("Running in ACP mode")

	conn := acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin)
	agent.SetAgentConnection(conn)
	conn.SetLogger(slog.Default())

	select {
	case <-conn.Done():
		slog.Debug("peer disconnected, shutting down")
	case <-s.ctx.Done():
		slog.Debug("received termination signal, shutting down", "signal", s.ctx.Err())
	}

	return nil
}

func (s *Server) Shutdown() {
	// Graceful shutdown
	s.agent = nil
}

var LogsDir = sync.OnceValue(func() string {
	tmp := filepath.Join(os.TempDir(), config.AppName)
	if err := os.MkdirAll(tmp, 0755); err != nil {
		slog.Error("could not create temp dir", "tmp", tmp)
		os.Exit(-1)
	}

	return tmp
})
