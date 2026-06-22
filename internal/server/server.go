package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/config"
	_ "github.com/charmbracelet/crush/internal/swagger"
	httpswagger "github.com/swaggo/http-swagger/v2"
)

// maxUnixSocketPathLen is the maximum length of a Unix domain socket
// path. The macOS sun_path field is 104 bytes; Linux allows 108. We
// use 104 so the resulting path is portable across both platforms.
const maxUnixSocketPathLen = 104

// socketDir returns the directory used for the Crush Unix socket.
// It prefers $XDG_RUNTIME_DIR when set (systemd's per-user runtime
// directory on Linux), and otherwise falls back to [os.TempDir],
// which resolves to the per-user private $TMPDIR on macOS and to
// /tmp on Linux.
func socketDir() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir
	}
	return os.TempDir()
}

// ErrServerClosed is returned when the server is closed.
var ErrServerClosed = http.ErrServerClosed

// ParseHostURL parses a host URL into a [url.URL].
func ParseHostURL(host string) (*url.URL, error) {
	proto, addr, ok := strings.Cut(host, "://")
	if !ok {
		return nil, fmt.Errorf("invalid host format: %s", host)
	}

	var basePath string
	if proto == "tcp" {
		parsed, err := url.Parse("tcp://" + addr)
		if err != nil {
			return nil, fmt.Errorf("invalid tcp address: %v", err)
		}
		addr = parsed.Host
		basePath = parsed.Path
	}
	return &url.URL{
		Scheme: proto,
		Host:   addr,
		Path:   basePath,
	}, nil
}

// DefaultHost returns the default server host.
//
// On Windows the address is a named pipe under \\.\pipe\. On Unix
// platforms the socket lives in the per-user runtime directory
// returned by [socketDir] and is named crush-<uid>.sock, falling
// back to crush.sock when the current uid cannot be determined. If
// the composed path would exceed [maxUnixSocketPathLen] bytes (the
// macOS sun_path limit), we fall back to /tmp/crush-<uid>.sock so
// the socket remains bindable.
func DefaultHost() string {
	sock := "crush.sock"
	usr, err := user.Current()
	if err == nil && usr.Uid != "" {
		sock = fmt.Sprintf("crush-%s.sock", usr.Uid)
	}
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("npipe:////./pipe/%s", sock)
	}
	path := filepath.Join(socketDir(), sock)
	if len(path) > maxUnixSocketPathLen {
		path = filepath.Join("/tmp", sock)
	}
	return "unix://" + path
}

// Server represents a Crush server bound to a specific address.
type Server struct {
	// Addr can be a TCP address, a Unix socket path, or a Windows named pipe.
	Addr    string
	network string

	h  *http.Server
	ln net.Listener

	backend *backend.Backend
	logger  *slog.Logger
}

// SetLogger sets the logger for the server.
func (s *Server) SetLogger(logger *slog.Logger) {
	s.logger = logger
}

// DefaultServer returns a new [Server] with the default address.
func DefaultServer(cfg *config.ConfigStore) *Server {
	hostURL, err := ParseHostURL(DefaultHost())
	if err != nil {
		panic("invalid default host")
	}
	return NewServer(cfg, hostURL.Scheme, hostURL.Host)
}

// NewServer creates a new [Server] with the given network and address.
func NewServer(cfg *config.ConfigStore, network, address string) *Server {
	s := new(Server)
	s.Addr = address
	s.network = network

	// The backend is created with a shutdown callback that triggers
	// a graceful server shutdown (e.g. when the last workspace is
	// removed).
	s.backend = backend.New(context.Background(), cfg, func() {
		go func() {
			slog.Info("Shutting down server...")
			if err := s.Shutdown(context.Background()); err != nil {
				slog.Error("Failed to shutdown server", "error", err)
			}
		}()
	})
	s.installHandler()
	if network == "tcp" {
		s.h.Addr = address
	}
	return s
}

// installHandler builds the protocol/router around s.backend and
// assigns the resulting http.Server to s.h. Extracted from
// [NewServer] so test harnesses can wire a Server around a
// pre-constructed backend.
func (s *Server) installHandler() {
	var p http.Protocols
	p.SetHTTP1(true)
	p.SetUnencryptedHTTP2(true)
	c := &controllerV1{backend: s.backend, server: s}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", c.handleGetHealth)
	mux.HandleFunc("GET /v1/version", c.handleGetVersion)
	mux.HandleFunc("GET /v1/config", c.handleGetConfig)
	mux.HandleFunc("POST /v1/control", c.handlePostControl)
	mux.HandleFunc("GET /v1/workspaces", c.handleGetWorkspaces)
	mux.HandleFunc("POST /v1/workspaces", c.handlePostWorkspaces)
	mux.HandleFunc("DELETE /v1/workspaces/{id}", c.handleDeleteWorkspaces)
	mux.HandleFunc("POST /v1/workspaces/{id}/current-session", c.handlePostWorkspaceCurrentSession)
	mux.HandleFunc("GET /v1/workspaces/{id}/clients", c.handleGetWorkspaceClients)
	mux.HandleFunc("GET /v1/workspaces/{id}", c.handleGetWorkspace)
	mux.HandleFunc("GET /v1/workspaces/{id}/config", c.handleGetWorkspaceConfig)
	mux.HandleFunc("GET /v1/workspaces/{id}/events", c.handleGetWorkspaceEvents)
	mux.HandleFunc("GET /v1/workspaces/{id}/providers", c.handleGetWorkspaceProviders)
	mux.HandleFunc("GET /v1/workspaces/{id}/sessions", c.handleGetWorkspaceSessions)
	mux.HandleFunc("POST /v1/workspaces/{id}/sessions", c.handlePostWorkspaceSessions)
	mux.HandleFunc("GET /v1/workspaces/{id}/sessions/{sid}", c.handleGetWorkspaceSession)
	mux.HandleFunc("PUT /v1/workspaces/{id}/sessions/{sid}", c.handlePutWorkspaceSession)
	mux.HandleFunc("DELETE /v1/workspaces/{id}/sessions/{sid}", c.handleDeleteWorkspaceSession)
	mux.HandleFunc("GET /v1/workspaces/{id}/sessions/{sid}/history", c.handleGetWorkspaceSessionHistory)
	mux.HandleFunc("GET /v1/workspaces/{id}/sessions/{sid}/messages", c.handleGetWorkspaceSessionMessages)
	mux.HandleFunc("GET /v1/workspaces/{id}/sessions/{sid}/messages/user", c.handleGetWorkspaceSessionUserMessages)
	mux.HandleFunc("GET /v1/workspaces/{id}/messages/user", c.handleGetWorkspaceAllUserMessages)
	mux.HandleFunc("GET /v1/workspaces/{id}/sessions/{sid}/filetracker/files", c.handleGetWorkspaceSessionFileTrackerFiles)
	mux.HandleFunc("POST /v1/workspaces/{id}/filetracker/read", c.handlePostWorkspaceFileTrackerRead)
	mux.HandleFunc("GET /v1/workspaces/{id}/filetracker/lastread", c.handleGetWorkspaceFileTrackerLastRead)
	mux.HandleFunc("GET /v1/workspaces/{id}/lsps", c.handleGetWorkspaceLSPs)
	mux.HandleFunc("GET /v1/workspaces/{id}/lsps/{lsp}/diagnostics", c.handleGetWorkspaceLSPDiagnostics)
	mux.HandleFunc("POST /v1/workspaces/{id}/lsps/start", c.handlePostWorkspaceLSPStart)
	mux.HandleFunc("POST /v1/workspaces/{id}/lsps/stop", c.handlePostWorkspaceLSPStopAll)
	mux.HandleFunc("GET /v1/workspaces/{id}/permissions/skip", c.handleGetWorkspacePermissionsSkip)
	mux.HandleFunc("POST /v1/workspaces/{id}/permissions/skip", c.handlePostWorkspacePermissionsSkip)
	mux.HandleFunc("POST /v1/workspaces/{id}/permissions/grant", c.handlePostWorkspacePermissionsGrant)
	mux.HandleFunc("GET /v1/workspaces/{id}/agent", c.handleGetWorkspaceAgent)
	mux.HandleFunc("POST /v1/workspaces/{id}/agent", c.handlePostWorkspaceAgent)
	mux.HandleFunc("POST /v1/workspaces/{id}/agent/init", c.handlePostWorkspaceAgentInit)
	mux.HandleFunc("POST /v1/workspaces/{id}/agent/update", c.handlePostWorkspaceAgentUpdate)
	mux.HandleFunc("GET /v1/workspaces/{id}/agent/sessions/{sid}", c.handleGetWorkspaceAgentSession)
	mux.HandleFunc("POST /v1/workspaces/{id}/agent/sessions/{sid}/cancel", c.handlePostWorkspaceAgentSessionCancel)
	mux.HandleFunc("GET /v1/workspaces/{id}/agent/sessions/{sid}/prompts/queued", c.handleGetWorkspaceAgentSessionPromptQueued)
	mux.HandleFunc("GET /v1/workspaces/{id}/agent/sessions/{sid}/prompts/list", c.handleGetWorkspaceAgentSessionPromptList)
	mux.HandleFunc("POST /v1/workspaces/{id}/agent/sessions/{sid}/prompts/clear", c.handlePostWorkspaceAgentSessionPromptClear)
	mux.HandleFunc("POST /v1/workspaces/{id}/agent/sessions/{sid}/summarize", c.handlePostWorkspaceAgentSessionSummarize)
	mux.HandleFunc("POST /v1/workspaces/{id}/agent/sessions/{sid}/shell", c.handlePostWorkspaceAgentSessionShell)
	mux.HandleFunc("GET /v1/workspaces/{id}/agent/default-small-model", c.handleGetWorkspaceAgentDefaultSmallModel)
	mux.HandleFunc("POST /v1/workspaces/{id}/config/set", c.handlePostWorkspaceConfigSet)
	mux.HandleFunc("POST /v1/workspaces/{id}/config/remove", c.handlePostWorkspaceConfigRemove)
	mux.HandleFunc("POST /v1/workspaces/{id}/config/model", c.handlePostWorkspaceConfigModel)
	mux.HandleFunc("POST /v1/workspaces/{id}/config/compact", c.handlePostWorkspaceConfigCompact)
	mux.HandleFunc("POST /v1/workspaces/{id}/config/provider-key", c.handlePostWorkspaceConfigProviderKey)
	mux.HandleFunc("POST /v1/workspaces/{id}/config/import-copilot", c.handlePostWorkspaceConfigImportCopilot)
	mux.HandleFunc("POST /v1/workspaces/{id}/config/refresh-oauth", c.handlePostWorkspaceConfigRefreshOAuth)
	mux.HandleFunc("GET /v1/workspaces/{id}/project/needs-init", c.handleGetWorkspaceProjectNeedsInit)
	mux.HandleFunc("POST /v1/workspaces/{id}/project/init", c.handlePostWorkspaceProjectInit)
	mux.HandleFunc("GET /v1/workspaces/{id}/project/init-prompt", c.handleGetWorkspaceProjectInitPrompt)
	mux.HandleFunc("GET /v1/workspaces/{id}/skills", c.handleGetWorkspaceSkills)
	mux.HandleFunc("POST /v1/workspaces/{id}/skills/read", c.handlePostWorkspaceSkillRead)
	mux.HandleFunc("POST /v1/workspaces/{id}/mcp/refresh-tools", c.handlePostWorkspaceMCPRefreshTools)
	mux.HandleFunc("POST /v1/workspaces/{id}/mcp/read-resource", c.handlePostWorkspaceMCPReadResource)
	mux.HandleFunc("POST /v1/workspaces/{id}/mcp/get-prompt", c.handlePostWorkspaceMCPGetPrompt)
	mux.HandleFunc("GET /v1/workspaces/{id}/mcp/states", c.handleGetWorkspaceMCPStates)
	mux.HandleFunc("POST /v1/workspaces/{id}/mcp/refresh-prompts", c.handlePostWorkspaceMCPRefreshPrompts)
	mux.HandleFunc("POST /v1/workspaces/{id}/mcp/refresh-resources", c.handlePostWorkspaceMCPRefreshResources)
	mux.HandleFunc("POST /v1/workspaces/{id}/mcp/docker/enable", c.handlePostWorkspaceMCPEnableDocker)
	mux.HandleFunc("POST /v1/workspaces/{id}/mcp/docker/disable", c.handlePostWorkspaceMCPDisableDocker)
	mux.Handle("/v1/docs/", httpswagger.WrapHandler)
	s.h = &http.Server{
		Protocols: &p,
		Handler:   s.recoverHandler(s.loggingHandler(mux)),
	}
}

// Handler returns the server's HTTP handler. Exposed so test harnesses
// can wrap it in an httptest.Server without going through the
// production listener setup.
func (s *Server) Handler() http.Handler {
	return s.h.Handler
}

// Serve accepts incoming connections on the listener.
func (s *Server) Serve(ln net.Listener) error {
	return s.h.Serve(ln)
}

// ListenAndServe starts the server and begins accepting connections.
func (s *Server) ListenAndServe() error {
	if s.ln != nil {
		return fmt.Errorf("server already started")
	}
	ln, removedStale, err := listen(s.network, s.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.Addr, err)
	}
	if removedStale && s.logger != nil {
		s.logger.Warn("Removed stale socket before binding", "address", s.Addr)
	}
	return s.Serve(ln)
}

func (s *Server) closeListener() {
	if s.ln != nil {
		s.ln.Close()
		s.ln = nil
	}
}

// Close force closes all listeners and connections.
func (s *Server) Close() error {
	defer func() { s.closeListener() }()
	return s.h.Close()
}

// Shutdown gracefully shuts down the server without interrupting active
// connections.
func (s *Server) Shutdown(ctx context.Context) error {
	defer func() { s.closeListener() }()
	return s.h.Shutdown(ctx)
}

func (s *Server) logDebug(r *http.Request, msg string, args ...any) {
	if s.logger != nil {
		s.logger.With(
			slog.String("method", r.Method),
			slog.String("url", r.URL.String()),
			slog.String("remote_addr", r.RemoteAddr),
		).Debug(msg, args...)
	}
}

func (s *Server) logError(r *http.Request, msg string, args ...any) {
	if s.logger != nil {
		s.logger.With(
			slog.String("method", r.Method),
			slog.String("url", r.URL.String()),
			slog.String("remote_addr", r.RemoteAddr),
		).Error(msg, args...)
	}
}
