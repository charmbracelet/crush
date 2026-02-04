// Package lsp provides a manager for Language Server Protocol (LSP) clients.
package lsp

import (
	"cmp"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	powernapconfig "github.com/charmbracelet/x/powernap/pkg/config"
	"github.com/charmbracelet/x/powernap/pkg/lsp"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
	"github.com/sourcegraph/jsonrpc2"
)

// Manager handles lazy initialization of LSP clients based on file types.
type Manager struct {
	clients  *csync.Map[string, *Client]
	cfg      *config.Config
	manager  *powernapconfig.Manager
	callback func(name string, client *Client)
	mu       sync.Mutex
}

// NewManager creates a new LSP manager service.
func NewManager(cfg *config.Config) *Manager {
	manager := powernapconfig.NewManager()
	manager.LoadDefaults()

	// Merge user-configured LSPs into the manager.
	for name, clientConfig := range cfg.LSP {
		if clientConfig.Disabled {
			slog.Debug("LSP disabled by user config", "name", name)
			manager.RemoveServer(name)
			continue
		}

		// HACK: the user might have the command name in their config instead
		// of the actual name. Find and use the correct name.
		actualName := resolveServerName(manager, name)
		manager.AddServer(actualName, &powernapconfig.ServerConfig{
			Command:     clientConfig.Command,
			Args:        clientConfig.Args,
			Environment: clientConfig.Env,
			FileTypes:   clientConfig.FileTypes,
			RootMarkers: clientConfig.RootMarkers,
			InitOptions: clientConfig.InitOptions,
			Settings:    clientConfig.Options,
		})
	}

	return &Manager{
		clients: csync.NewMap[string, *Client](),
		cfg:     cfg,
		manager: manager,
	}
}

// Clients returns the map of LSP clients.
func (m *Manager) Clients() *csync.Map[string, *Client] {
	return m.clients
}

// SetCallback sets a callback that is invoked when a new LSP
// client is successfully started. This allows the coordinator to add LSP tools.
func (s *Manager) SetCallback(cb func(name string, client *Client)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callback = cb
}

// Start starts an LSP server that can handle the given file path.
// If an appropriate LSP is already running, this is a no-op.
func (s *Manager) Start(ctx context.Context, filePath string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var wg sync.WaitGroup
	for name, server := range s.manager.GetServers() {
		if !handles(server, filePath, s.cfg.WorkingDir()) {
			continue
		}
		wg.Go(func() {
			s.startServer(ctx, name, server)
		})
	}
	wg.Wait()
}

// skipAutoStartCommands contains commands that are too generic or ambiguous to
// auto-start without explicit user configuration.
var skipAutoStartCommands = map[string]bool{
	"buck2":   true,
	"buf":     true,
	"cue":     true,
	"dart":    true,
	"deno":    true,
	"dotnet":  true,
	"dprint":  true,
	"gleam":   true,
	"java":    true,
	"julia":   true,
	"koka":    true,
	"node":    true,
	"npx":     true,
	"perl":    true,
	"plz":     true,
	"python":  true,
	"python3": true,
	"R":       true,
	"racket":  true,
	"rome":    true,
	"rubocop": true,
	"ruff":    true,
	"scarb":   true,
	"solc":    true,
	"stylua":  true,
	"swipl":   true,
	"tflint":  true,
}

func (s *Manager) startServer(ctx context.Context, name string, server *powernapconfig.ServerConfig) {
	userConfigured := s.isUserConfigured(name)

	if !userConfigured {
		if _, err := exec.LookPath(server.Command); err != nil {
			slog.Debug("LSP server not installed, skipping", "name", name, "command", server.Command)
			return
		}
		if skipAutoStartCommands[server.Command] {
			slog.Debug("LSP command too generic for auto-start, skipping", "name", name, "command", server.Command)
			return
		}
	}

	cfg := s.buildConfig(name, server)
	if client, ok := s.clients.Get(name); ok {
		switch client.GetServerState() {
		case StateReady, StateStarting:
			s.callback(name, client)
			// already done, return
			return
		}
	}
	client, err := New(ctx, name, cfg, s.cfg.Resolver())
	if err != nil {
		slog.Error("Failed to create LSP client", "name", name, "error", err)
		return
	}
	s.callback(name, client)

	defer func() {
		s.clients.Set(name, client)
		s.callback(name, client)
	}()

	initCtx, cancel := context.WithTimeout(ctx, time.Duration(cmp.Or(cfg.Timeout, 30))*time.Second)
	defer cancel()

	if _, err := client.Initialize(initCtx, s.cfg.WorkingDir()); err != nil {
		slog.Error("LSP client initialization failed", "name", name, "error", err)
		client.Close(ctx)
		return
	}

	if err := client.WaitForServerReady(initCtx); err != nil {
		slog.Warn("LSP server not fully ready, continuing anyway", "name", name, "error", err)
		client.SetServerState(StateError)
	} else {
		client.SetServerState(StateReady)
	}

	slog.Debug("LSP client started", "name", name)
}

func (s *Manager) isUserConfigured(name string) bool {
	cfg, ok := s.cfg.LSP[name]
	return ok && !cfg.Disabled
}

func (s *Manager) buildConfig(name string, server *powernapconfig.ServerConfig) config.LSPConfig {
	cfg := config.LSPConfig{
		Command:     server.Command,
		Args:        server.Args,
		Env:         server.Environment,
		FileTypes:   server.FileTypes,
		RootMarkers: server.RootMarkers,
		InitOptions: server.InitOptions,
		Options:     server.Settings,
	}
	if userCfg, ok := s.cfg.LSP[name]; ok {
		cfg.Timeout = userCfg.Timeout
	}
	return cfg
}

func resolveServerName(manager *powernapconfig.Manager, name string) string {
	if _, ok := manager.GetServer(name); ok {
		return name
	}
	for sname, server := range manager.GetServers() {
		if server.Command == name {
			return sname
		}
	}
	return name
}

func handlesFiletype(server *powernapconfig.ServerConfig, ext string, language protocol.LanguageKind) bool {
	for _, ft := range server.FileTypes {
		if protocol.LanguageKind(ft) == language ||
			ft == strings.TrimPrefix(ext, ".") ||
			"."+ft == ext {
			return true
		}
	}
	return false
}

func handles(server *powernapconfig.ServerConfig, filePath, workDir string) bool {
	language := lsp.DetectLanguage(filePath)
	ext := filepath.Ext(filePath)
	if !handlesFiletype(server, ext, language) {
		return false
	}
	for _, marker := range server.RootMarkers {
		if _, err := os.Stat(filepath.Join(workDir, marker)); err == nil {
			return true
		}
	}
	return false
}

// StopAll stops all running LSP clients and clears the client map.
func (s *Manager) StopAll(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var wg sync.WaitGroup
	for name, client := range s.clients.Seq2() {
		wg.Go(func() {
			defer func() { s.callback(name, client) }()
			if err := client.Close(ctx); err != nil &&
				!errors.Is(err, io.EOF) &&
				!errors.Is(err, context.Canceled) &&
				!errors.Is(err, jsonrpc2.ErrClosed) &&
				err.Error() != "signal: killed" {
				slog.Warn("Failed to stop LSP client", "name", name, "error", err)
			}
			client.SetServerState(StateStopped)
			slog.Debug("Stopped LSP client", "name", name)
		})
	}
	wg.Wait()
}
