// Package lsp provides a manager for Language Server Protocol (LSP) clients.
package lsp

import (
	"cmp"
	"context"
	"errors"
	"io"
	"log/slog"
	"maps"
	"os"
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
	"github.com/sourcegraph/jsonrpc2"
)

const unavailableRetryDelay = 30 * time.Second

// Manager handles lazy initialization of LSP clients based on file types.
type Manager struct {
	clients     *csync.Map[string, *Client]
	unavailable *csync.Map[string, time.Time]
	cfg         *config.ConfigStore
	manager     *powernapconfig.Manager
	callback    func(name string, client *Client)
	now         func() time.Time
	lookPath    func(string) (string, error)

	// mu serializes replacing the client of a server in clients with the
	// checks that decide it.
	mu sync.Mutex

	// defaults is the server catalog as it was before the user's
	// configuration was merged into manager. Never modified.
	defaults map[string]*powernapconfig.ServerConfig
	// sharedHost is the application that shares its language servers, or
	// nil. See [Manager.SetHost].
	sharedHost atomic.Pointer[Host]
	// hostSkipLogged holds the servers already reported as not shared
	// with the host because of their configuration.
	hostSkipLogged sync.Map
	// crushExecutable returns the executable hosted clients run to reach
	// the host. Nil means os.Executable.
	crushExecutable func() (string, error)
	// supersededCloseDelay overrides [supersededCloseDelay] when positive.
	supersededCloseDelay time.Duration
	// rehosting holds the servers [Manager.rehost] is moving to the host,
	// so that the starts of one server make one attempt between them.
	rehosting sync.Map
	// superseded holds the clients that were replaced by hosted ones and
	// are still running, with the timers that close them. They are no
	// longer in clients, and still have to be stopped with them. Guarded
	// by mu.
	superseded map[*Client]*time.Timer
}

// NewManager creates a new LSP manager service.
func NewManager(cfg *config.ConfigStore) *Manager {
	manager := powernapconfig.NewManager()
	manager.LoadDefaults()
	defaults := maps.Clone(manager.GetServers())

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
		clients:     csync.NewMap[string, *Client](),
		unavailable: csync.NewMap[string, time.Time](),
		cfg:         cfg,
		manager:     manager,
		defaults:    defaults,
		callback:    func(string, *Client) {}, // default no-op callback
		now:         time.Now,
		lookPath:    exec.LookPath,
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

	var wg sync.WaitGroup
	for name, server := range s.manager.GetServers() {
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

	if client, ok := s.running(name); ok {
		s.callback(name, client)
		// Already done, unless the host can take the server over now.
		s.rehost(name, client, server)
		return
	}

	// A server the host application offers is reached on the host, so it
	// does not have to be installed here. Whether the server is wanted at
	// all is decided as for a local one.
	target := s.hostTarget(name, server)
	switch {
	case target != nil:
		if !isUserConfigured && skipAutoStartCommands[server.Command] {
			return
		}
		if !handles(server, filepath, s.cfg.WorkingDir()) {
			return
		}
	case isUserConfigured:
		if !handles(server, filepath, s.cfg.WorkingDir()) {
			return
		}
	default:
		if !s.canAutoStart(name, filepath, s.cfg.WorkingDir(), server) {
			return
		}
	}

	// Check again in case another goroutine started it in the meantime.
	if client, ok := s.running(name); ok {
		s.callback(name, client)
		s.rehost(name, client, server)
		return
	}

	if target != nil {
		if s.launch(name, cfg, target) {
			return
		}
		// The host did not serve it: leave the host alone for a while,
		// and run the server here like any other.
		s.markUnavailable(hostKeyPrefix + name)
		if !isUserConfigured && !s.commandAvailable(name, server) {
			return
		}
		if client, ok := s.running(name); ok {
			s.callback(name, client)
			return
		}
	}
	s.launch(name, cfg, nil)
}

// running returns the client of the server name when there is nothing to
// start: the client is ready, starting or disabled, and its connection is
// not known to be gone.
func (s *Manager) running(name string) (*Client, bool) {
	client, ok := s.clients.Get(name)
	if !ok || !reusable(client) {
		return nil, false
	}
	return client, true
}

func reusable(client *Client) bool {
	switch client.GetServerState() {
	case StateReady, StateStarting, StateDisabled:
		return client.alive()
	}
	return false
}

// create makes the client of the server name: of the host's server when
// target is not nil, of a server of its own otherwise.
func (s *Manager) create(name string, cfg config.LSPConfig, target *hostTarget) (*Client, error) {
	client, err := newClient(
		name,
		cfg,
		s.cfg.Resolver(),
		s.cfg.WorkingDir(),
		s.cfg.Config().Options.DebugLSP,
		target,
	)
	if err != nil {
		return nil, err
	}
	client.onLost = func(c *Client) { s.clientLost(name, c) }
	return client, nil
}

// initialize runs the LSP handshake of a new client. A client that fails
// it is shut down. A client that was stopped on purpose meanwhile, by
// [Manager.StopAll] or [Manager.KillAll], is left to whoever stopped it,
// and the error is [errStopped]: the server did not fail.
func (s *Manager) initialize(name string, client *Client, cfg config.LSPConfig) error {
	client.serverState.Store(StateStarting)

	// Use an independent context for initialization so that the LSP server
	// startup is not tied to the caller's request context. The caller's
	// context may have a short timeout or be canceled when the tool call
	// completes, but LSP initialization can take several seconds and the
	// server must persist beyond any single request.
	initCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cmp.Or(cfg.Timeout, 30))*time.Second)
	defer cancel()

	if _, err := client.Initialize(initCtx, s.cfg.WorkingDir()); err != nil {
		// A restart closed the connection under the handshake, and goes
		// on with the client.
		if client.restarting.Load() {
			return errRestarted
		}
		// Asked before the client is shut down here, which marks it as
		// stopped too.
		if errors.Is(client.interrupted(), errStopped) {
			return errStopped
		}
		client.Shutdown()
		return err
	}

	err := client.WaitForServerReady(initCtx)
	switch {
	case err == nil:
		client.SetServerState(StateReady)
	case errors.Is(err, errStopped), errors.Is(err, errConnectionLost):
		// Handled below.
	default:
		slog.Warn("LSP server not fully ready, continuing anyway", "name", name, "error", err)
		client.SetServerState(StateError)
	}

	// The client may have been stopped, or its connection may have gone
	// away, while it was starting, which the states set above would hide.
	switch client.interrupted() {
	case errStopped:
		client.SetServerState(StateStopped)
		return errStopped
	case errConnectionLost:
		client.SetServerState(StateError)
		client.Shutdown()
		return errConnectionLost
	}
	return nil
}

// connect returns an initialized client of the host's server, without
// making it the client of the server name.
func (s *Manager) connect(name string, cfg config.LSPConfig, target *hostTarget) (*Client, error) {
	client, err := s.create(name, cfg, target)
	if err != nil {
		return nil, err
	}
	if err := s.initialize(name, client, cfg); err != nil {
		return nil, err
	}
	return client, nil
}

// launch starts the server name, on the host when target is not nil, and
// makes it the server's client. It reports whether the server is taken
// care of; false means it could not be started.
func (s *Manager) launch(name string, cfg config.LSPConfig, target *hostTarget) bool {
	client, err := s.create(name, cfg, target)
	if err != nil {
		if target != nil {
			slog.Warn("Failed to create LSP client for the host's server", "name", name, "host", target.name, "error", err)
		} else {
			slog.Error("Failed to create LSP client", "name", name, "error", err)
		}
		return false
	}

	// Only store non-nil clients. If another goroutine raced us,
	// prefer the already-stored client.
	s.mu.Lock()
	if existing, ok := s.running(name); ok {
		s.mu.Unlock()
		client.Shutdown()
		s.callback(name, existing)
		return true
	}
	s.clients.Set(name, client)
	s.mu.Unlock()

	if err := s.initialize(name, client, cfg); err != nil {
		if errors.Is(err, errRestarted) {
			// The client stays: the restart has it now.
			return true
		}
		s.clients.CompareAndDelete(name, client)
		if errors.Is(err, errStopped) {
			// Stopped on purpose: there is nothing to fall back to,
			// and nothing to hold against the host.
			slog.Debug("LSP client stopped while starting", "name", name)
			return true
		}
		if target != nil {
			// Not an error: the server is started locally instead.
			slog.Warn("LSP host did not serve the server, starting it locally", "name", name, "host", target.name, "error", err)
			return false
		}
		slog.Error("LSP client initialization failed", "name", name, "error", err)
		s.callback(name, client)
		return false
	}

	// The client may have been replaced while it was starting, by one of
	// a host that announced itself meanwhile.
	if current, ok := s.clients.Get(name); ok {
		client = current
	}
	s.callback(name, client)

	// A host may also have announced itself after this start looked for
	// one and before its client was there for [Manager.reconcile] to see.
	// SetHost stores the host and then looks at the clients; this stored
	// the client and now looks at the host: one of them sees the other.
	if server, ok := s.manager.GetServer(name); ok {
		s.rehost(name, client, server)
	}

	if host, ok := client.Host(); ok {
		slog.Debug("LSP client started", "name", name, "host", host)
	} else {
		slog.Debug("LSP client started", "name", name)
	}
	return true
}

// clientLost is called when the connection of a client to its server went
// away: the server exited, or the host application closed the connection.
// The client is dropped, so that the next start connects or spawns again
// instead of finding a client that looks ready.
func (s *Manager) clientLost(name string, client *Client) {
	stored := s.clients.CompareAndDelete(name, client)
	client.Shutdown()
	if stored {
		s.callback(name, client)
	}
}

// executable returns the crush executable hosted clients run.
func (s *Manager) executable() (string, error) {
	if s.crushExecutable != nil {
		return s.crushExecutable()
	}
	return os.Executable()
}

func (s *Manager) canAutoStart(
	name, filePath, workDir string,
	server *powernapconfig.ServerConfig,
) bool {
	if skipAutoStartCommands[server.Command] {
		slog.Debug("LSP command too generic for auto-start, skipping", "name", name, "command", server.Command)
		return false
	}

	// Filtering by file type is cheap and usually rejects a server before the
	// root marker check. Do both before searching PATH, which can require a stat
	// for every directory in PATH for every bundled server.
	if !handles(server, filePath, workDir) {
		return false
	}

	return s.commandAvailable(name, server)
}

// commandAvailable reports whether the server's command is installed. A
// missing command is remembered for a while, so PATH is not searched on
// every start.
func (s *Manager) commandAvailable(name string, server *powernapconfig.ServerConfig) bool {
	if s.recentlyUnavailable(name) {
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
	lastUnavailableAt, exists := s.unavailable.Get(name)
	if !exists {
		return false
	}
	if s.now().Sub(lastUnavailableAt) < unavailableRetryDelay {
		return true
	}
	s.unavailable.Del(name)
	return false
}

func (s *Manager) markUnavailable(name string) {
	s.unavailable.Set(name, s.now())
}

func (s *Manager) clearUnavailable(name string) {
	s.unavailable.Del(name)
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
	clients, superseded := s.stopping()
	var wg sync.WaitGroup
	for _, client := range superseded {
		wg.Go(client.Shutdown)
	}
	for name, client := range clients {
		wg.Go(func() {
			defer func() { s.callback(name, client) }()
			client.Shutdown()
			client.SetServerState(StateStopped)
			s.clients.CompareAndDelete(name, client)
			slog.Debug("Killed LSP client", "name", name)
		})
	}
	wg.Wait()
}

// stopping returns the clients to stop: the client of every server, and
// the clients that were replaced by hosted ones and are not closed yet,
// which are the caller's to close from here on.
//
// The clients are marked as being stopped under the lock that replacing a
// client takes. A replacement is therefore either in what is returned, or
// finds its client marked and gives up: nothing is left running behind the
// stop.
func (s *Manager) stopping() (clients map[string]*Client, superseded []*Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	clients = s.clients.Copy()
	for _, client := range clients {
		client.closing.Store(true)
	}
	for client, timer := range s.superseded {
		timer.Stop()
		superseded = append(superseded, client)
	}
	clear(s.superseded)
	return clients, superseded
}

// StopAll stops all running LSP clients and clears the client map.
func (s *Manager) StopAll(ctx context.Context) {
	clients, superseded := s.stopping()
	var wg sync.WaitGroup
	for _, client := range superseded {
		wg.Go(func() {
			_ = client.Close(ctx)
			client.cancelCtx()
		})
	}
	for name, client := range clients {
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
			s.clients.CompareAndDelete(name, client)
			slog.Debug("Stopped LSP client", "name", name)
		})
	}
	wg.Wait()
}
