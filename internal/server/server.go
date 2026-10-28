package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/config"
)

// maxUnixSocketPathLen is the maximum length of a Unix domain socket
// path. The macOS sun_path field is 104 bytes; Linux allows 108. We
// use 104 so the resulting path is portable across both platforms.
const maxUnixSocketPathLen = 104

// selfShutdownTimeout bounds a shutdown the server starts on its own. It
// is the budget `crush server` gives the shutdown a signal starts.
const selfShutdownTimeout = 15 * time.Second

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
//
// A relative unix socket path is made absolute against the current
// directory, so the address keeps naming the same socket after the process
// changes directory, as the client does for --cwd.
func ParseHostURL(host string) (*url.URL, error) {
	proto, addr, ok := strings.Cut(host, "://")
	if !ok {
		return nil, fmt.Errorf("invalid host format: %s", host)
	}

	// On Linux a leading '@' names an abstract socket, which has no path.
	if proto == "unix" && addr != "" && !filepath.IsAbs(addr) && !strings.HasPrefix(addr, "@") {
		abs, err := filepath.Abs(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid unix socket path: %w", err)
		}
		addr = abs
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
// On Windows the address is the named pipe \\.\pipe\crush-<sid>.sock,
// where <sid> is the current user's SID. On Unix platforms the socket
// lives in the per-user runtime directory returned by [socketDir] and
// is named crush-<uid>.sock. If the composed path would exceed
// [maxUnixSocketPathLen] bytes (the macOS sun_path limit), we fall back
// to /tmp/crush-<uid>.sock so the socket remains bindable.
//
// The user's id keeps users from sharing a server, since the pipe
// namespace and /tmp are machine-wide. If it cannot be determined,
// DefaultHost returns "", which callers reject as an invalid host,
// rather than a name every user would share.
func DefaultHost() string {
	id := currentUserID()
	if id == "" {
		return ""
	}
	sock := fmt.Sprintf("crush-%s.sock", id)
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
	// cancelRequests cancels the context every request derives from.
	cancelRequests context.CancelFunc

	backend *backend.Backend
	logger  *slog.Logger

	// selfShutdown runs the shutdown the backend asks for at most once.
	// selfShutdownStarted is set before that shutdown begins and
	// selfShutdownDone is closed once it has finished, so [Server.Serve]
	// can wait for it.
	selfShutdown        sync.Once
	selfShutdownStarted atomic.Bool
	selfShutdownDone    chan struct{}
}

// SetLogger sets the logger for the server.
func (s *Server) SetLogger(logger *slog.Logger) {
	s.logger = logger
}

// Backend returns the server's backend. Intended for integration tests
// that drive lifecycle transitions (detach, grace tuning) against a live
// HTTP surface.
func (s *Server) Backend() *backend.Backend {
	return s.backend
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
	s.selfShutdownDone = make(chan struct{})

	// The backend is created with a shutdown callback that triggers
	// a graceful server shutdown (e.g. when the last workspace is
	// removed).
	s.backend = backend.New(context.Background(), cfg, func() {
		s.shutdownSelf(selfShutdownTimeout)
	})
	s.installHandler()
	if network == "tcp" {
		s.h.Addr = address
	}
	return s
}

// shutdownSelf starts the graceful shutdown the backend asks for when the
// server has gone idle. The backend can ask more than once (a client's
// request and the idle timer, say), so only the first call does anything.
// [Server.Serve] waits for this shutdown, so it gives up after timeout and
// closes what is left: a request that never finishes must not keep the
// server up for good.
func (s *Server) shutdownSelf(timeout time.Duration) {
	s.selfShutdown.Do(func() {
		s.selfShutdownStarted.Store(true)
		go func() {
			defer close(s.selfShutdownDone)
			slog.Info("Shutting down server...")
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			if err := s.Shutdown(ctx); err != nil {
				slog.Error("Failed to shutdown server", "error", err)
				_ = s.Close()
			}
		}()
	})
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
	for _, e := range c.endpoints() {
		mux.HandleFunc(e.Method()+" "+e.Path(), e.Handler())
	}
	mux.HandleFunc("GET /v1/docs/", c.handleDocsIndex)
	mux.HandleFunc("GET /v1/docs/openapi.json", c.handleDocsSpec)
	// http.Server.Shutdown waits for active requests without ever
	// cancelling them, so requests derive from a context Shutdown can
	// cancel itself.
	baseCtx, cancel := context.WithCancel(context.Background())
	s.cancelRequests = cancel
	s.h = &http.Server{
		Protocols:   &p,
		Handler:     s.recoverHandler(s.loggingHandler(s.crossOriginHandler(mux))),
		BaseContext: func(net.Listener) context.Context { return baseCtx },
	}
}

// Handler returns the server's HTTP handler. Exposed so test harnesses
// can wrap it in an httptest.Server without going through the
// production listener setup.
func (s *Server) Handler() http.Handler {
	return s.h.Handler
}

// Serve accepts incoming connections on the listener. When the server
// stops itself, Serve returns once that shutdown has finished rather than
// as soon as the listener closes, so a caller that exits on return does
// not cut off the requests the shutdown is still draining.
func (s *Server) Serve(ln net.Listener) error {
	err := s.h.Serve(ln)
	if errors.Is(err, ErrServerClosed) && s.selfShutdownStarted.Load() {
		<-s.selfShutdownDone
	}
	return err
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
	// Start the idle clock now that clients can reach us: a server that
	// never gets a workspace must not outlive the client that spawned it.
	s.backend.ArmIdleShutdown()
	return s.Serve(ln)
}

func (s *Server) closeListener() {
	if s.ln != nil {
		s.ln.Close()
		s.ln = nil
	}
}

// Close force closes all listeners and connections. Unlike [Server.Shutdown]
// it leaves hosted workspaces running.
func (s *Server) Close() error {
	defer func() { s.closeListener() }()
	return s.h.Close()
}

// Shutdown tears down hosted workspaces, then cancels in-flight requests
// and gracefully shuts down the server, waiting for their handlers to
// return. ctx bounds both. The idle-shutdown path relies on the teardown
// being a no-op with none left.
func (s *Server) Shutdown(ctx context.Context) error {
	defer func() { s.closeListener() }()
	err := s.backend.ShutdownWorkspaces(ctx)
	// With the workspaces gone no request has work left to do, and one
	// blocked on its context, such as a running shell command, would
	// otherwise hold the shutdown until ctx expires and then be
	// abandoned. Cancelling only now keeps event streams open to deliver
	// the teardown's final events.
	s.cancelRequests()
	return errors.Join(err, s.h.Shutdown(ctx))
}

// ReloadConfig re-reads the server and hosted workspace configs from disk.
func (s *Server) ReloadConfig(ctx context.Context) error {
	return s.backend.ReloadConfig(ctx)
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
