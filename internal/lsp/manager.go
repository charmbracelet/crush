// Package lsp provides a manager for Language Server Protocol (LSP) clients.
package lsp

import (
	"cmp"
	"context"
	"errors"
	"io"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/fsext"
	powernapconfig "github.com/charmbracelet/x/powernap/pkg/config"
	powernap "github.com/charmbracelet/x/powernap/pkg/lsp"
	"github.com/sourcegraph/jsonrpc2"
	"golang.org/x/sync/singleflight"
)

var unavailable = csync.NewMap[string, struct{}]()

// Manager handles lazy initialization of LSP clients based on file types.
type Manager struct {
	clients  *csync.Map[string, *Client]
	cfg      *config.ConfigStore
	manager  *powernapconfig.Manager
	callback func(name string, client *Client)
	// requestGroup coalesces concurrent startup requests per server.
	requestGroup singleflight.Group
}

// NewManager creates a new LSP manager service.
func NewManager(cfg *config.ConfigStore) *Manager {
	manager := powernapconfig.NewManager()
	manager.LoadDefaults()

	// Merge user-configured LSPs into the manager.
	for name, clientConfig := range cfg.Config().LSP {
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
		clients:  csync.NewMap[string, *Client](),
		cfg:      cfg,
		manager:  manager,
		callback: func(string, *Client) {}, // default no-op callback
	}
}

// Clients returns the map of LSP clients.
func (s *Manager) Clients() *csync.Map[string, *Client] {
	return s.clients
}

// SetCallback sets a callback that is invoked when a new LSP
// client is successfully started. This allows the coordinator to add LSP tools.
func (s *Manager) SetCallback(cb func(name string, client *Client)) {
	s.callback = cb
}

// TrackConfigured will callback the user-configured LSPs, but will not create
// any clients.
func (s *Manager) TrackConfigured() {
	var wg sync.WaitGroup
	for name := range s.manager.GetServers() {
		if !s.isUserConfigured(name) {
			continue
		}
		wg.Go(func() {
			s.callback(name, nil)
		})
	}
	wg.Wait()
}

// Start starts an LSP server that can handle the given file path.
// If an appropriate LSP is already running, this is a no-op.
func (s *Manager) Start(ctx context.Context, path string) {
	if !fsext.HasPrefix(path, s.cfg.WorkingDir()) {
		return
	}

	var wg sync.WaitGroup
	for name, server := range s.manager.GetServers() {
		wg.Go(func() {
			s.startServer(ctx, name, path, server)
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

func (s *Manager) startServer(ctx context.Context, name, filepath string, server *powernapconfig.ServerConfig) {
	var (
		isUserConfigured = s.isUserConfigured(name)
		autoLSP          = s.cfg.Config().Options.AutoLSP
	)
	if !isUserConfigured && autoLSP != nil && !*autoLSP {
		slog.Debug("Auto-start LSP disabled", "name", name)
		return
	}

	cfg := s.buildConfig(name, server)
	if cfg.Disabled {
		return
	}

	if _, exists := unavailable.Get(name); exists {
		return
	}

	if client, ok := s.getActiveClient(name); ok {
		s.callback(name, client)
		return
	}

	if !isUserConfigured {
		if _, err := exec.LookPath(server.Command); err != nil {
			slog.Debug("LSP server not installed, skipping", "name", name, "command", server.Command)
			unavailable.Set(name, struct{}{})
			return
		}
		if skipAutoStartCommands[server.Command] {
			slog.Debug("LSP command too generic for auto-start, skipping", "name", name, "command", server.Command)
			return
		}
	}

	// this is the slowest bit, so we do it last.
	if !handles(server, filepath, s.cfg.WorkingDir()) {
		// nothing to do
		return
	}

	ch := s.requestGroup.DoChan(name, func() (any, error) {
		// Double-check in case another goroutine started it in the meantime.
		if client, ok := s.getActiveClient(name); ok {
			return client, nil
		}

		// Keep shared startup alive even if one caller's context is canceled.
		baseCtx := context.WithoutCancel(ctx)

		client, err := New(
			baseCtx,
			name,
			cfg,
			s.cfg.Resolver(),
			s.cfg.WorkingDir(),
			s.cfg.Config().Options.DebugLSP,
		)
		if err != nil {
			return nil, err
		}

		// If another goroutine won the race, prefer the existing active client.
		if existing, ok := s.getActiveClient(name); ok {
			_ = client.Close(baseCtx)
			return existing, nil
		}

		client.SetServerState(StateStarting)
		s.clients.Set(name, client)

		timeout := time.Duration(cmp.Or(cfg.Timeout, 30)) * time.Second
		initCtx, cancel := context.WithTimeout(baseCtx, timeout)
		defer cancel()

		if _, err := client.Initialize(initCtx, s.cfg.WorkingDir()); err != nil {
			client.SetServerState(StateError)
			_ = client.Close(baseCtx)
			s.clients.Del(name)
			return nil, err
		}

		if err := client.WaitForServerReady(initCtx); err != nil {
			slog.Warn("LSP server not fully ready, continuing anyway", "name", name, "error", err)
			client.SetServerState(StateError)
		} else {
			client.SetServerState(StateReady)
		}

		return client, nil
	})

	select {
	case <-ctx.Done():
		slog.Debug("Context canceled while waiting for LSP start", "name", name)
		return
	case res := <-ch:
		if res.Err != nil {
			if !res.Shared {
				slog.Error("Failed to start LSP client", "name", name, "error", res.Err)
			} else {
				slog.Debug("Failed to start LSP client (shared result)", "name", name, "error", res.Err)
			}
			return
		}

		client, ok := res.Val.(*Client)
		if !ok || client == nil {
			slog.Error("Invalid LSP client result type", "name", name)
			return
		}

		s.callback(name, client)
	}
}

func isActiveClient(client *Client) bool {
	if client == nil {
		return false
	}
	switch client.GetServerState() {
	case StateReady, StateStarting, StateDisabled:
		return true
	default:
		return false
	}
}

func (s *Manager) getActiveClient(name string) (*Client, bool) {
	client, ok := s.clients.Get(name)
	if !ok || !isActiveClient(client) {
		return nil, false
	}
	return client, true
}

func (s *Manager) isUserConfigured(name string) bool {
	cfg, ok := s.cfg.Config().LSP[name]
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
	if userCfg, ok := s.cfg.Config().LSP[name]; ok {
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

func handlesFiletype(sname string, fileTypes []string, filePath string) bool {
	if len(fileTypes) == 0 {
		return true
	}

	kind := powernap.DetectLanguage(filePath)
	name := strings.ToLower(filepath.Base(filePath))
	for _, filetype := range fileTypes {
		suffix := strings.ToLower(filetype)
		if !strings.HasPrefix(suffix, ".") {
			suffix = "." + suffix
		}
		if strings.HasSuffix(name, suffix) || filetype == string(kind) {
			slog.Debug("Handles file", "name", sname, "file", name, "filetype", filetype, "kind", kind)
			return true
		}
	}

	slog.Debug("Doesn't handle file", "name", sname, "file", name)
	return false
}

func hasRootMarkers(dir string, markers []string) bool {
	if len(markers) == 0 {
		return true
	}
	for _, pattern := range markers {
		// Use filepath.Glob for a non-recursive check in the root
		// directory. This avoids walking the entire tree (which is
		// catastrophic in large monorepos with node_modules, etc.).
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err == nil && len(matches) > 0 {
			return true
		}
	}
	return false
}

func handles(server *powernapconfig.ServerConfig, filePath, workDir string) bool {
	return handlesFiletype(server.Command, server.FileTypes, filePath) &&
		hasRootMarkers(workDir, server.RootMarkers)
}

// KillAll force-kills all the LSP clients.
//
// This is generally faster than [Manager.StopAll] because it doesn't wait for
// the server to exit gracefully, but it can lead to data loss if the server is
// in the middle of writing something.
// Generally it doesn't matter when shutting down Crush, though.
func (s *Manager) KillAll(context.Context) {
	var wg sync.WaitGroup
	for name, client := range s.clients.Seq2() {
		wg.Go(func() {
			defer func() { s.callback(name, client) }()
			client.client.Kill()
			client.SetServerState(StateStopped)
			s.clients.Del(name)
			slog.Debug("Killed LSP client", "name", name)
		})
	}
	wg.Wait()
}

// StopAll stops all running LSP clients and clears the client map.
func (s *Manager) StopAll(ctx context.Context) {
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
			s.clients.Del(name)
			slog.Debug("Stopped LSP client", "name", name)
		})
	}
	wg.Wait()
}
