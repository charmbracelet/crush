package lsp

import (
	"cmp"
	"context"
	"log/slog"
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
)

// Starter handles lazy initialization of LSP clients based on file types.
type Starter struct {
	clients             *csync.Map[string, *Client]
	cfg                 *config.Config
	manager             *powernapconfig.Manager
	onClientStarted     func(name string, client *Client)
	onClientStartedOnce sync.Once
	mu                  sync.Mutex
}

// NewStarter creates a new LSP starter service.
func NewStarter(
	clients *csync.Map[string, *Client],
	cfg *config.Config,
) *Starter {
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

	return &Starter{
		clients: clients,
		cfg:     cfg,
		manager: manager,
	}
}

// SetClientStartedCallback sets a callback that is invoked when a new LSP
// client is successfully started. This allows the coordinator to add LSP tools.
func (s *Starter) SetClientStartedCallback(cb func(name string, client *Client)) {
	s.onClientStartedOnce.Do(func() {
		s.onClientStarted = cb
	})
}

// StartForFile starts an LSP server that can handle the given file path.
// If an appropriate LSP is already running, this is a no-op.
func (s *Starter) StartForFile(ctx context.Context, filePath string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	language := lsp.DetectLanguage(filePath)
	ext := filepath.Ext(filePath)

	for name, server := range s.manager.GetServers() {
		if !handles(server, ext, language) {
			continue
		}
		if _, exists := s.clients.Get(name); exists {
			return
		}
		go s.startServer(ctx, name, server)
	}
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

func (s *Starter) startServer(ctx context.Context, name string, server *powernapconfig.ServerConfig) {
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

	slog.Debug("Starting LSP client on demand", "name", name, "command", server.Command)
	cfg := s.buildConfig(name, server)

	client, err := New(ctx, name, cfg, s.cfg.Resolver())
	if err != nil {
		slog.Error("Failed to create LSP client", "name", name, "error", err)
		return
	}

	initCtx, cancel := context.WithTimeout(ctx, time.Duration(cmp.Or(cfg.Timeout, 30))*time.Second)
	defer cancel()

	if _, err = client.Initialize(initCtx, s.cfg.WorkingDir()); err != nil {
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

	s.clients.Set(name, client)
	s.onClientStarted(name, client)
	slog.Info("LSP client started", "name", name)
}

func (s *Starter) isUserConfigured(name string) bool {
	cfg, ok := s.cfg.LSP[name]
	return ok && !cfg.Disabled
}

func (s *Starter) buildConfig(name string, server *powernapconfig.ServerConfig) config.LSPConfig {
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

func handles(server *powernapconfig.ServerConfig, ext string, language protocol.LanguageKind) bool {
	for _, ft := range server.FileTypes {
		if protocol.LanguageKind(ft) == language ||
			ft == strings.TrimPrefix(ext, ".") ||
			"."+ft == ext {
			return true
		}
	}
	return false
}
