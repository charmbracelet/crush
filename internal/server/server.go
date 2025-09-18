package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/rpc"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc/v2"
)

// ErrServerClosed is returned when the server is closed.
var ErrServerClosed = fmt.Errorf("server closed")

// InstanceState represents the state of a running [app.App] instance.
type InstanceState uint8

const (
	// InstanceStateCreated indicates that the instance has been created but not yet started.
	InstanceStateCreated InstanceState = iota
	// InstanceStateStarted indicates that the instance is currently running.
	InstanceStateStarted
	// InstanceStateStopped indicates that the instance has been stopped.
	InstanceStateStopped
)

// Instance represents a running [app.App] instance with its associated
// resources and state.
type Instance struct {
	*app.App
	State InstanceState
	id    string
	path  string
}

// ID returns the unique identifier of the instance.
func (i *Instance) ID() string {
	return i.id
}

// Path returns the filesystem path associated with the instance.
func (i *Instance) Path() string {
	return i.path
}

// DefaultAddr returns the default address path for the Crush server based on
// the operating system.
func DefaultAddr() string {
	sock := "crush.sock"
	user, err := user.Current()
	if err == nil && user.Uid != "" {
		sock = fmt.Sprintf("crush-%s.sock", user.Uid)
	}
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(`\\.\pipe\%s`, sock)
	}
	return filepath.Join(os.TempDir(), sock)
}

// Server represents a Crush server instance bound to a specific address.
type Server struct {
	// Addr can be a TCP address, a Unix socket path, or a Windows named pipe.
	Addr string

	// instances is a map of running applications managed by the server.
	instances *csync.Map[string, *Instance]
	// listeners is the network listener for the server.
	listeners *csync.Map[*net.Listener, struct{}]
	cfg       *config.Config
	logger    *slog.Logger

	shutdown atomic.Bool
}

// DefaultServer returns a new [Server] instance with the default address.
func DefaultServer(cfg *config.Config) *Server {
	return NewServer(cfg, "unix", DefaultAddr())
}

// NewServer is a helper to create a new [Server] instance with the given
// address. On Windows, if the address is not a "tcp" address, it will be
// converted to a named pipe format.
func NewServer(cfg *config.Config, network, address string) *Server {
	if runtime.GOOS == "windows" && !strings.HasPrefix(address, "tcp") &&
		!strings.HasPrefix(address, `\\.\pipe\`) {
		// On Windows, convert to named pipe format if not TCP
		// (e.g., "mypipe" -> "\\.\pipe\mypipe")
		address = fmt.Sprintf(`\\.\pipe\%s`, address)
	}

	s := new(Server)
	s.Addr = address
	s.cfg = cfg
	s.instances = csync.NewMap[string, *Instance]()
	rpc.Register(&ServerProto{s})
	return s
}

// Serve accepts incoming connections on the listener.
func (s *Server) Serve(ln net.Listener) error {
	if s.listeners == nil {
		s.listeners = csync.NewMap[*net.Listener, struct{}]()
	}
	s.listeners.Set(&ln, struct{}{})

	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		conn, err := ln.Accept()
		if err != nil {
			if s.shuttingDown() {
				return ErrServerClosed
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				time.Sleep(tempDelay)
				continue
			}
			return fmt.Errorf("failed to accept connection: %w", err)
		}
		go s.handleConn(conn)
	}
}

// ListenAndServe starts the server and begins accepting connections.
func (s *Server) ListenAndServe() error {
	ln, err := listen("unix", s.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.Addr, err)
	}
	return s.Serve(ln)
}

// Close force close all listeners and connections.
func (s *Server) Close() error {
	s.shutdown.Store(true)
	var firstErr error
	for k := range s.listeners.Seq2() {
		if err := (*k).Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		s.listeners.Del(k)
	}
	return firstErr
}

// Shutdown gracefully shuts down the server without interrupting active
// connections. It stops accepting new connections and waits for existing
// connections to finish.
func (s *Server) Shutdown(ctx context.Context) error {
	// TODO: implement graceful shutdown
	return s.Close()
}

func (s *Server) handleConn(conn net.Conn) {
	s.info("accepted connection from %s", conn.RemoteAddr())
	msgpackrpc.ServeConn(conn)
	// var req rpc.Request
	// codec := msgpackrpc.NewServerCodec(conn)
	// if err := codec.ReadRequestHeader(&req); err != nil {
	// 	s.error("failed to read request header: %v", err)
	// }
	// rpc.ServeCodec(codec)
}

func (s *Server) shuttingDown() bool {
	return s.shutdown.Load()
}

func (s *Server) info(msg string, args ...any) {
	if s.logger != nil {
		s.logger.Info(msg, args...)
	}
}

func (s *Server) debug(msg string, args ...any) {
	if s.logger != nil {
		s.logger.Debug(msg, args...)
	}
}

func (s *Server) error(msg string, args ...any) {
	if s.logger != nil {
		s.logger.Error(msg, args...)
	}
}

func (s *Server) warn(msg string, args ...any) {
	if s.logger != nil {
		s.logger.Warn(msg, args...)
	}
}
