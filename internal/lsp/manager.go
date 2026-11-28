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
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/fsext"
	powernapconfig "github.com/charmbracelet/x/powernap/pkg/config"
	powernap "github.com/charmbracelet/x/powernap/pkg/lsp"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
	"github.com/sourcegraph/jsonrpc2"
)

const (
	// unavailableRetryDelay is how long a server whose command is missing,
	// or could not be spawned, is skipped before Crush looks for it again.
	unavailableRetryDelay = 30 * time.Second
	// maxStartFailureRetryDelay caps how long a server that keeps failing to
	// initialize is skipped. The first failure waits unavailableRetryDelay,
	// so a server that was only slow once comes back soon, and each further
	// failure doubles the wait, because each retry spawns a process and may
	// block for the full init timeout.
	maxStartFailureRetryDelay = 5 * time.Minute
)

// Manager handles lazy initialization of LSP clients based on file types.
type Manager struct {
	clients *csync.Map[string, *Client]
	// unavailable maps server names to the time they may be retried.
	unavailable *csync.Map[string, time.Time]
	// startFailures counts initialize failures in a row per server.
	startFailures *csync.Map[string, *atomic.Int32]
	cfg           *config.ConfigStore
	manager       *powernapconfig.Manager
	callback      func(name string, client *Client)
	now           func() time.Time
	lookPath      func(string) (string, error)
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
		clients:       csync.NewMap[string, *Client](),
		unavailable:   csync.NewMap[string, time.Time](),
		startFailures: csync.NewMap[string, *atomic.Int32](),
		cfg:           cfg,
		manager:       manager,
		callback:      func(string, *Client) {}, // default no-op callback
		now:           time.Now,
		lookPath:      exec.LookPath,
	}
}

// Clients returns the map of LSP clients.
func (s *Manager) Clients() *csync.Map[string, *Client] {
	return s.clients
}

// BestClientFor returns the LSP client best suited to handle the given
// path, preferring clients whose FileTypes explicitly match over
// clients that act as catch-alls (no FileTypes set). Returns nil when
// no client is willing to handle the path.
//
// Use this from tools that must pick a single client (lsp_references).
// Tools that broadcast to every willing client (lsp_diagnostics,
// notifyLSPs) should iterate Clients() directly with HandlesFile.
//
// Ties are broken by client name so the choice is deterministic across
// runs — Go's map iteration order is randomized, and without a tiebreak
// the same workspace would route to different clients on different
// invocations.
func (s *Manager) BestClientFor(path string) *Client {
	var (
		best     *Client
		bestName string
	)
	bestScore := 0
	for name, c := range s.clients.Seq2() {
		score := c.MatchScore(path)
		if score == 0 {
			continue
		}
		if score > bestScore || (score == bestScore && (best == nil || name < bestName)) {
			best = c
			bestScore = score
			bestName = name
		}
	}
	return best
}

// SetCallback sets a callback that is invoked when a new LSP
// client is successfully started. This allows the coordinator to add LSP tools.
func (s *Manager) SetCallback(cb func(name string, client *Client)) {
	s.callback = cb
}

// TrackConfigured notifies the UI about user-configured LSP servers without
// starting them. Servers start on demand via Start().
func (s *Manager) TrackConfigured(ctx context.Context) {
	var wg sync.WaitGroup
	servers := s.manager.GetServers()
	for name := range servers {
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
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if !fsext.HasPrefix(path, s.cfg.WorkingDir()) {
		return
	}

	// Most of the bundled servers do not handle this file. Filter them out
	// here so only servers that already have a client, or whose file types
	// match, get a goroutine and the full start checks.
	var (
		wg   sync.WaitGroup
		base = strings.ToLower(filepath.Base(path))
		kind = powernap.DetectLanguage(path)
	)
	for name, server := range s.manager.GetServers() {
		if _, ok := s.clients.Get(name); !ok && !matchesFiletype(server.FileTypes, base, kind) {
			continue
		}
		wg.Go(func() {
			s.startServer(name, path, server)
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

func (s *Manager) startServer(name, filepath string, server *powernapconfig.ServerConfig) {
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

	if client, ok := s.clients.Get(name); ok {
		switch client.GetServerState() {
		case StateReady, StateStarting, StateDisabled:
			s.callback(name, client)
			// already done, return
			return
		}
	}

	if isUserConfigured {
		if !handles(server, filepath, s.cfg.WorkingDir()) {
			return
		}
		if s.recentlyUnavailable(name) {
			return
		}
	} else if !s.canAutoStart(name, filepath, s.cfg.WorkingDir(), server) {
		return
	}

	// Check again in case another goroutine started it in the meantime.
	if client, ok := s.clients.Get(name); ok {
		switch client.GetServerState() {
		case StateReady, StateStarting, StateDisabled:
			s.callback(name, client)
			return
		}
	}

	client, err := New(
		name,
		cfg,
		s.cfg.Resolver(),
		s.cfg.WorkingDir(),
		s.cfg.Config().Options.DebugLSP,
	)
	if err != nil {
		slog.Error("Failed to create LSP client", "name", name, "error", err)
		// Usually the command is missing. Retrying that is cheap, so a
		// server installed mid-session is picked up soon.
		s.markUnavailable(name)
		return
	}
	// Only store non-nil clients. If another goroutine raced us,
	// prefer the already-stored client.
	if existing, ok := s.clients.Get(name); ok {
		switch existing.GetServerState() {
		case StateReady, StateStarting, StateDisabled:
			client.Shutdown()
			s.callback(name, existing)
			return
		case StateError:
			// A client left in the error state (for example by a failed
			// restart) may still have a live server process. Shut it down
			// before replacing it so the process is not orphaned.
			existing.Shutdown()
		}
	}
	s.clients.Set(name, client)
	defer func() {
		s.callback(name, client)
	}()

	switch client.GetServerState() {
	case StateReady, StateStarting, StateDisabled:
		// already done, return
		return
	}

	client.serverState.Store(StateStarting)

	// Use an independent context for initialization so that the LSP server
	// startup is not tied to the caller's request context. The caller's
	// context may have a short timeout or be canceled when the tool call
	// completes, but LSP initialization can take several seconds and the
	// server must persist beyond any single request.
	initCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cmp.Or(cfg.Timeout, 30))*time.Second)
	defer cancel()

	if _, err := client.Initialize(initCtx, s.cfg.WorkingDir()); err != nil {
		slog.Error("LSP client initialization failed", "name", name, "error", err)
		client.Shutdown()
		s.clients.Del(name)
		s.markStartFailure(name)
		return
	}
	s.clearUnavailable(name)
	s.startFailures.Del(name)

	if err := client.WaitForServerReady(initCtx); err != nil {
		slog.Warn("LSP server not fully ready, continuing anyway", "name", name, "error", err)
		client.SetServerState(StateError)
	} else {
		client.SetServerState(StateReady)
	}

	slog.Debug("LSP client started", "name", name)
}

func (s *Manager) canAutoStart(
	name, filePath, workDir string,
	server *powernapconfig.ServerConfig,
) bool {
	if skipAutoStartCommands[server.Command] {
		slog.Debug("LSP command too generic for auto-start, skipping", "name", name, "command", server.Command)
		return false
	}

	// The unavailable check is a map lookup. Filtering by file type is cheap
	// and usually rejects a server before the root marker check, which may
	// read the working directory. Do all of them before searching PATH,
	// which can require a stat for every directory in PATH.
	if s.recentlyUnavailable(name) {
		return false
	}
	if !handles(server, filePath, workDir) {
		return false
	}
	if _, err := s.lookPath(server.Command); err != nil {
		slog.Debug("LSP server not installed, skipping", "name", name, "command", server.Command)
		s.markUnavailable(name)
		return false
	}
	s.clearUnavailable(name)
	return true
}

func (s *Manager) isUserConfigured(name string) bool {
	cfg, ok := s.cfg.Config().LSP[name]
	return ok && !cfg.Disabled
}

func (s *Manager) recentlyUnavailable(name string) bool {
	retryAt, exists := s.unavailable.Get(name)
	if !exists {
		return false
	}
	if s.now().Before(retryAt) {
		return true
	}
	s.unavailable.Del(name)
	return false
}

func (s *Manager) markUnavailable(name string) {
	s.markUnavailableFor(name, unavailableRetryDelay)
}

// markUnavailableFor skips the server for the given duration.
func (s *Manager) markUnavailableFor(name string, delay time.Duration) {
	s.unavailable.Set(name, s.now().Add(delay))
}

func (s *Manager) clearUnavailable(name string) {
	s.unavailable.Del(name)
}

// markStartFailure skips a server that failed to initialize, doubling the
// wait with each failure in a row.
func (s *Manager) markStartFailure(name string) {
	failures := s.startFailures.GetOrSet(name, func() *atomic.Int32 {
		return new(atomic.Int32)
	}).Add(1)
	delay := unavailableRetryDelay << min(failures-1, 4)
	s.markUnavailableFor(name, min(delay, maxStartFailureRetryDelay))
}

// ResetUnavailable lets a server that was skipped after failing to start be
// tried again on its next use. An empty name resets every server.
func (s *Manager) ResetUnavailable(name string) {
	if name != "" {
		s.unavailable.Del(name)
		s.startFailures.Del(name)
		return
	}
	s.unavailable.Reset(map[string]time.Time{})
	s.startFailures.Reset(map[string]*atomic.Int32{})
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
	if matchesFiletype(fileTypes, name, kind) {
		slog.Debug("Handles file", "name", sname, "file", name, "kind", kind)
		return true
	}

	slog.Debug("Doesn't handle file", "name", sname, "file", name)
	return false
}

// matchesFiletype reports whether a file with the given lowercased base name
// and detected language matches any of fileTypes. An empty fileTypes matches
// every file.
func matchesFiletype(fileTypes []string, base string, kind protocol.LanguageKind) bool {
	if len(fileTypes) == 0 {
		return true
	}
	for _, filetype := range fileTypes {
		if filetype == string(kind) {
			return true
		}
		// Match base against "." + filetype without allocating.
		ext := strings.TrimPrefix(strings.ToLower(filetype), ".")
		if len(base) > len(ext) &&
			base[len(base)-len(ext)-1] == '.' &&
			strings.HasSuffix(base, ext) {
			return true
		}
	}
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
			client.Shutdown()
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
			client.cancelCtx()
			client.SetServerState(StateStopped)
			s.clients.Del(name)
			slog.Debug("Stopped LSP client", "name", name)
		})
	}
	wg.Wait()
}
