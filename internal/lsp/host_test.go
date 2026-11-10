//go:build !windows

package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"maps"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/charmbracelet/crush/internal/lsphost"
	"github.com/charmbracelet/crush/internal/version"
	powernapconfig "github.com/charmbracelet/x/powernap/pkg/config"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/stretchr/testify/require"
)

// fakeServerName is the name under which the test binary acts as a
// language server. A server shared with a host has no arguments, so the
// test binary is linked under this name instead of being given one.
const fakeServerName = "fakels"

// fakeSlowShutdown is how long a fake server that is slow to shut down
// takes to answer the shutdown request.
const fakeSlowShutdown = 1500 * time.Millisecond

// TestMain lets the test binary stand in for the two processes a manager
// starts: `crush __lsp-dial`, which hosted clients run and which is
// [Manager.executable] in tests, and a local language server.
func TestMain(m *testing.M) {
	switch {
	case len(os.Args) > 1 && os.Args[1] == lsphost.Command:
		os.Exit(lsphost.Main(context.Background(), os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
	case filepath.Base(os.Args[0]) == fakeServerName:
		os.Exit(runFakeServer())
	}
	os.Exit(m.Run())
}

// fakeLSPHandler answers the few requests a client needs to start and stop,
// and reports every message it receives: the method, followed by the
// document's URI for document notifications.
func fakeLSPHandler(record func(*jsonrpc2.Conn, string), failShutdown bool, onExit func()) jsonrpc2.Handler {
	return jsonrpc2.HandlerWithError(func(_ context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (any, error) {
		line := req.Method
		if strings.HasPrefix(req.Method, "textDocument/did") && req.Params != nil {
			var p struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(*req.Params, &p)
			line += " " + p.TextDocument.URI
		}
		record(conn, line)
		switch req.Method {
		case "initialize":
			return map[string]any{"capabilities": map[string]any{}}, nil
		case "shutdown":
			if failShutdown {
				return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInternalError, Message: "not shutting down"}
			}
		case "exit":
			onExit()
		}
		return nil, nil
	})
}

// runFakeServer is a language server on stdio. It writes its PID to
// "<its path>.pid" and appends what it receives to "<its path>.log". Given
// --fail-shutdown, it answers shutdown with an error. When the file
// "<its path>.slow-shutdown" exists, it takes a while to answer shutdown;
// that is a file because a server with arguments is not shared with a host.
func runFakeServer() int {
	self := os.Args[0]
	if err := os.WriteFile(self+".pid", []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		return 3
	}
	logFile, err := os.OpenFile(self+".log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 3
	}
	exit := make(chan struct{}, 1)
	_, statErr := os.Stat(self + ".slow-shutdown")
	slowShutdown := statErr == nil
	handler := fakeLSPHandler(
		func(_ *jsonrpc2.Conn, line string) {
			_, _ = logFile.WriteString(line + "\n")
			if line == "shutdown" && slowShutdown {
				time.Sleep(fakeSlowShutdown)
			}
		},
		slices.Contains(os.Args[1:], "--fail-shutdown"),
		func() { exit <- struct{}{} },
	)
	stdio := struct {
		io.Reader
		io.Writer
		io.Closer
	}{os.Stdin, os.Stdout, os.Stdin}
	conn := jsonrpc2.NewConn(context.Background(), jsonrpc2.NewBufferedStream(stdio, jsonrpc2.VSCodeObjectCodec{}), handler)
	select {
	case <-exit:
	case <-conn.DisconnectNotify():
	}
	return 0
}

// fakeServer is a local language server for a test: the test binary under
// the name [fakeServerName].
type fakeServer struct {
	path string
}

func newFakeServer(t *testing.T) *fakeServer {
	t.Helper()
	self, err := os.Executable()
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), fakeServerName)
	require.NoError(t, os.Symlink(self, path))
	return &fakeServer{path: path}
}

// received returns what every process of the server received so far,
// except for exit notifications: a client kills its server right after
// sending one, so whether the server still gets to read it is a race.
func (f *fakeServer) received() []string {
	data, _ := os.ReadFile(f.path + ".log")
	lines := strings.Fields(strings.ReplaceAll(strings.TrimSpace(string(data)), " ", "="))
	return slices.DeleteFunc(lines, func(line string) bool { return line == "exit" })
}

// exited reports whether a process of the server received an exit
// notification.
func (f *fakeServer) exited() bool {
	data, _ := os.ReadFile(f.path + ".log")
	return slices.Contains(strings.Fields(string(data)), "exit")
}

// slowShutdown makes the server's processes slow to shut down.
func (f *fakeServer) slowShutdown(t *testing.T) {
	t.Helper()
	require.NoError(t, os.WriteFile(f.path+".slow-shutdown", nil, 0o644))
}

// pid returns the PID of the server's latest process, or 0.
func (f *fakeServer) pid() int {
	data, _ := os.ReadFile(f.path + ".pid")
	pid, _ := strconv.Atoi(string(data))
	return pid
}

// processGone reports whether no process has the given PID. A killed
// process still counts until its parent reaped it.
func processGone(pid int) bool {
	return syscall.Kill(pid, 0) != nil
}

// fakeHub is an LSP host in the test's process: it listens on a unix
// socket, answers each hello, and then serves a minimal language server on
// the connection.
type fakeHub struct {
	t      *testing.T
	socket string
	ln     net.Listener

	mu     sync.Mutex
	refuse string // When set, every hello is refused with this error.
	// initializeDelay is how long the hub takes to answer initialize, as
	// when it has to start its server first.
	initializeDelay time.Duration
	// dropInitialized makes the hub close a session shortly after the
	// client finished the handshake, as when its server crashes then.
	dropInitialized bool
	// publishOnOpen makes the hub publish one diagnostic for every
	// document a client opens.
	publishOnOpen bool
	// frozen, once set, makes the hub stop: it answers no hello and
	// reads nothing more from its sessions, as a host that is suspended.
	// It goes on when the channel is closed.
	frozen   chan struct{}
	hellos   []lsphost.Hello
	conns    map[net.Conn]struct{}
	received []string
}

func newFakeHub(t *testing.T) *fakeHub {
	t.Helper()
	// A unix socket path has to be short, which the test's temporary
	// directory may not be.
	socket := filepath.Join(t.TempDir(), "hub.sock")
	if len(socket) >= 100 {
		dir, err := os.MkdirTemp("/tmp", "crush-lsp-")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		socket = filepath.Join(dir, "hub.sock")
	}
	return newFakeHubAt(t, socket)
}

// newFakeHubAt returns a hub listening on the given socket, as another
// process of the host application that takes over the socket of one that
// exited.
func newFakeHubAt(t *testing.T, socket string) *fakeHub {
	t.Helper()
	ln, err := net.Listen("unix", socket) //nolint:noctx
	require.NoError(t, err)
	h := &fakeHub{t: t, socket: socket, ln: ln, conns: make(map[net.Conn]struct{})}
	t.Cleanup(h.close)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go h.serve(conn)
		}
	}()
	return h
}

func (h *fakeHub) serve(conn net.Conn) {
	br := bufio.NewReader(conn)
	line, err := br.ReadBytes('\n')
	if err != nil {
		_ = conn.Close()
		return
	}
	var hello lsphost.Hello
	if err := json.Unmarshal(line, &hello); err != nil {
		_ = conn.Close()
		return
	}

	h.mu.Lock()
	frozen := h.frozen
	h.mu.Unlock()
	if frozen != nil {
		<-frozen
		_ = conn.Close()
		return
	}

	h.mu.Lock()
	h.hellos = append(h.hellos, hello)
	refuse := h.refuse
	if refuse == "" {
		h.conns[conn] = struct{}{}
	}
	h.mu.Unlock()

	if refuse != "" {
		ack, _ := json.Marshal(lsphost.Ack{Error: refuse})
		_, _ = conn.Write(append(ack, '\n'))
		_ = conn.Close()
		return
	}
	ack, _ := json.Marshal(lsphost.Ack{OK: true, V: lsphost.Protocol, Host: "joy", PID: os.Getpid()})
	if _, err := conn.Write(append(ack, '\n')); err != nil {
		_ = conn.Close()
		return
	}

	record := func(rpc *jsonrpc2.Conn, line string) {
		h.mu.Lock()
		frozen := h.frozen
		h.mu.Unlock()
		if frozen != nil {
			// The read loop waits here: nothing more is read.
			<-frozen
		}

		h.mu.Lock()
		h.received = append(h.received, strings.ReplaceAll(line, " ", "="))
		delay, drop, publish := h.initializeDelay, h.dropInitialized, h.publishOnOpen
		h.mu.Unlock()
		// This runs before the message is answered.
		switch {
		case line == "initialize":
			time.Sleep(delay)
		case line == "initialized":
			if drop {
				time.AfterFunc(100*time.Millisecond, func() { _ = conn.Close() })
			}
		case strings.HasPrefix(line, "textDocument/didOpen ") && publish && rpc != nil:
			uri := strings.TrimPrefix(line, "textDocument/didOpen ")
			go func() {
				_ = rpc.Notify(context.Background(), "textDocument/publishDiagnostics", map[string]any{
					"uri": uri,
					"diagnostics": []map[string]any{{
						"range":    map[string]any{"start": map[string]int{"line": 0, "character": 0}, "end": map[string]int{"line": 0, "character": 1}},
						"severity": 1,
						"message":  "reported by the host's server",
					}},
				})
			}()
		}
	}
	stream := struct {
		io.Reader
		io.WriteCloser
	}{br, conn}
	// Like the real hub, exit is a detach: the hub closes the connection
	// and keeps its server.
	rpc := jsonrpc2.NewConn(context.Background(), jsonrpc2.NewBufferedStream(stream, jsonrpc2.VSCodeObjectCodec{}),
		fakeLSPHandler(record, false, func() { go conn.Close() }),
		jsonrpc2.SetLogger(log.New(io.Discard, "", 0)))
	<-rpc.DisconnectNotify()
	h.mu.Lock()
	h.received = append(h.received, "disconnected")
	h.mu.Unlock()
	h.mu.Lock()
	delete(h.conns, conn)
	h.mu.Unlock()
}

// host returns the announcement of the hub, offering the given servers as
// name and command pairs.
func (h *fakeHub) host(servers ...HostServer) *Host {
	return &Host{Name: "joy", Version: "1.2.3", PID: os.Getpid(), Socket: h.socket, Workspace: "ws-1", Servers: servers}
}

func (h *fakeHub) setRefuse(reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.refuse = reason
}

func (h *fakeHub) setInitializeDelay(d time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.initializeDelay = d
}

func (h *fakeHub) setDropInitialized(drop bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dropInitialized = drop
}

func (h *fakeHub) setPublishOnOpen(publish bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.publishOnOpen = publish
}

// freeze makes the hub stop answering and reading, until the test ends.
func (h *fakeHub) freeze() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.frozen == nil {
		frozen := make(chan struct{})
		h.frozen = frozen
		h.t.Cleanup(func() { close(frozen) })
	}
}

func (h *fakeHub) helloCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.hellos)
}

func (h *fakeHub) lastHello() lsphost.Hello {
	h.mu.Lock()
	defer h.mu.Unlock()
	require.NotEmpty(h.t, h.hellos)
	return h.hellos[len(h.hellos)-1]
}

func (h *fakeHub) sessions() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.conns)
}

func (h *fakeHub) messages() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.received)
}

// dropSessions closes every connection, as a host that crashes does, and
// keeps listening.
func (h *fakeHub) dropSessions() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.conns {
		_ = conn.Close()
	}
}

// close stops the hub, as a host that exits does: its socket is gone.
func (h *fakeHub) close() {
	_ = h.ln.Close()
	h.dropSessions()
}

// callbacks records the clients a manager reported through its callback.
type callbacks struct {
	mu   sync.Mutex
	seen []callbackEvent
}

type callbackEvent struct {
	client *Client
	state  ServerState
}

func (c *callbacks) record(_ string, client *Client) {
	if client == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seen = append(c.seen, callbackEvent{client: client, state: client.GetServerState()})
}

// reported reports whether the callback was called for client while it was
// in the given state.
func (c *callbacks) reported(client *Client, state ServerState) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Contains(c.seen, callbackEvent{client: client, state: state})
}

// newTestManager returns a manager for a project in workDir that knows the
// given servers, none of them configured by the user.
func newTestManager(t *testing.T, workDir string, servers map[string]*powernapconfig.ServerConfig) (*Manager, *callbacks) {
	t.Helper()
	catalog := powernapconfig.NewManager()
	for name, server := range servers {
		catalog.AddServer(name, server)
	}
	cb := &callbacks{}
	m := &Manager{
		clients:              csync.NewMap[string, *Client](),
		unavailable:          csync.NewMap[string, time.Time](),
		cfg:                  config.NewTestStoreIn(workDir, &config.Config{Options: &config.Options{}}),
		manager:              catalog,
		defaults:             maps.Clone(catalog.GetServers()),
		callback:             cb.record,
		now:                  time.Now,
		lookPath:             exec.LookPath,
		supersededCloseDelay: 50 * time.Millisecond,
	}
	t.Cleanup(func() { m.KillAll(context.Background()) })
	return m, cb
}

// projectDir returns a project directory with a Go file in it.
func projectDir(t *testing.T) (dir, file string) {
	t.Helper()
	// The manager compares paths, so the directory is the one the
	// operating system reports (on macOS /var is a link to /private/var).
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	file = filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(file, []byte("package main\n"), 0o644))
	return dir, file
}

func requireClient(t *testing.T, m *Manager, name string) *Client {
	t.Helper()
	client, ok := m.Clients().Get(name)
	require.True(t, ok, "no client for %s", name)
	return client
}

const waitFor = 10 * time.Second

func TestHostedStartNeedsNoLocalCommand(t *testing.T) {
	t.Parallel()
	dir, file := projectDir(t)
	hub := newFakeHub(t)

	m, cb := newTestManager(t, dir, map[string]*powernapconfig.ServerConfig{
		"gopls": {Command: "crush-test-server-that-is-not-installed", FileTypes: []string{"go"}},
	})
	lookups := 0
	m.lookPath = func(string) (string, error) {
		lookups++
		return "", exec.ErrNotFound
	}
	m.SetHost(hub.host(HostServer{Name: "gopls-of-joy", Command: "crush-test-server-that-is-not-installed"}))

	m.Start(t.Context(), file)

	client := requireClient(t, m, "gopls")
	require.Equal(t, StateReady, client.GetServerState())
	host, hosted := client.Host()
	require.True(t, hosted)
	require.Equal(t, "joy", host)
	require.Zero(t, lookups, "a hosted server is not looked up on PATH")
	require.True(t, cb.reported(client, StateReady))

	require.Equal(t, lsphost.Hello{
		LSPHost:       lsphost.Protocol,
		Server:        "gopls-of-joy",
		Client:        "crush",
		ClientVersion: version.Version,
		Workspace:     "ws-1",
		Root:          dir,
		PID:           os.Getpid(),
	}, hub.lastHello())
	require.Equal(t, 1, hub.helloCount())
	require.Contains(t, hub.messages(), "initialize")
	require.Contains(t, hub.messages(), "initialized")

	// The client works like any other.
	require.NoError(t, client.OpenFile(t.Context(), file))
	require.Eventually(t, func() bool {
		return slices.Contains(hub.messages(), "textDocument/didOpen="+string(protocol.URIFromPath(file)))
	}, waitFor, 10*time.Millisecond)

	// Starting again finds it running.
	m.Start(t.Context(), file)
	require.Same(t, client, requireClient(t, m, "gopls"))
	require.Equal(t, 1, hub.helloCount())
}

func TestHostedStartKeepsTheFiletypeFilter(t *testing.T) {
	t.Parallel()
	dir, _ := projectDir(t)
	other := filepath.Join(dir, "notes.txt")
	require.NoError(t, os.WriteFile(other, []byte("hi\n"), 0o644))
	hub := newFakeHub(t)

	m, _ := newTestManager(t, dir, map[string]*powernapconfig.ServerConfig{
		"gopls": {Command: "gopls", FileTypes: []string{"go"}},
	})
	m.SetHost(hub.host(HostServer{Name: "gopls", Command: "gopls"}))

	m.Start(t.Context(), other)

	require.Zero(t, m.Clients().Len())
	require.Zero(t, hub.helloCount())
}

func TestHostTarget(t *testing.T) {
	t.Parallel()
	hub := &fakeHub{socket: "/run/joy-501/lsp.sock"}
	offer := hub.host(
		HostServer{Name: "gopls", Command: "gopls"},
		HostServer{Name: "rust-analyzer", Command: "/usr/local/bin/rust-analyzer"},
	)
	catalog := map[string]*powernapconfig.ServerConfig{
		"gopls":         {Command: "gopls", Args: []string{"serve"}, Settings: map[string]any{}},
		"rust_analyzer": {Command: "rust-analyzer"},
		"pyright":       {Command: "pyright-langserver", Args: []string{"--stdio"}},
	}

	newManager := func(t *testing.T) *Manager {
		m, _ := newTestManager(t, t.TempDir(), nil)
		m.defaults = catalog
		m.crushExecutable = func() (string, error) { return "/opt/crush", nil }
		m.SetHost(offer)
		return m
	}

	t.Run("matches by command name", func(t *testing.T) {
		t.Parallel()
		m := newManager(t)

		target := m.hostTarget("gopls", catalog["gopls"])
		require.Equal(t, &hostTarget{name: "joy", socket: hub.socket, server: "gopls", workspace: "ws-1", executable: "/opt/crush"}, target)

		// Crush's name for the server does not matter, nor where either
		// side finds the command.
		target = m.hostTarget("rust_analyzer", &powernapconfig.ServerConfig{Command: "/opt/rust/bin/rust-analyzer"})
		require.NotNil(t, target)
		require.Equal(t, "rust-analyzer", target.server)

		require.Nil(t, m.hostTarget("pyright", catalog["pyright"]), "not offered")
	})

	t.Run("customized servers stay local", func(t *testing.T) {
		t.Parallel()
		m := newManager(t)

		for name, server := range map[string]*powernapconfig.ServerConfig{
			"args":         {Command: "gopls", Args: []string{"serve", "-rpc.trace"}},
			"env":          {Command: "gopls", Args: []string{"serve"}, Environment: map[string]string{"GOFLAGS": "-tags=x"}},
			"init options": {Command: "gopls", Args: []string{"serve"}, InitOptions: map[string]any{"a": 1}},
			"settings":     {Command: "gopls", Args: []string{"serve"}, Settings: map[string]any{"gopls": map[string]any{"staticcheck": true}}},
		} {
			require.Nil(t, m.hostTarget("gopls", server), name)
		}
		// The catalog's own values are not a customization, however they
		// are spelled.
		require.NotNil(t, m.hostTarget("gopls", &powernapconfig.ServerConfig{Command: "gopls", Args: []string{"serve"}, Environment: map[string]string{}}))
	})

	t.Run("no host", func(t *testing.T) {
		t.Parallel()
		m := newManager(t)
		m.SetHost(nil)
		require.Nil(t, m.hostTarget("gopls", catalog["gopls"]))
	})

	t.Run("backoff ends with a new announcement", func(t *testing.T) {
		t.Parallel()
		m := newManager(t)
		m.markUnavailable(hostKeyPrefix + "gopls")
		m.markUnavailable("pyright")
		require.Nil(t, m.hostTarget("gopls", catalog["gopls"]))

		m.SetHost(offer)
		require.NotNil(t, m.hostTarget("gopls", catalog["gopls"]))
		require.True(t, m.recentlyUnavailable("pyright"), "a missing local command stays missing")
	})
}

// TestHostFailureFallsBackToLocalServer covers a host that refuses the
// server and one that cannot be reached: the server is started locally at
// once, and the host is left alone until the backoff ends or it announces
// itself again.
func TestHostFailureFallsBackToLocalServer(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		prepare func(hub *fakeHub)
		dials   int
	}{
		{name: "refusal", prepare: func(hub *fakeHub) { hub.setRefuse("no server fakels") }, dials: 1},
		{name: "host gone", prepare: func(hub *fakeHub) { hub.close() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir, file := projectDir(t)
			hub := newFakeHub(t)
			server := newFakeServer(t)
			tc.prepare(hub)

			m, _ := newTestManager(t, dir, map[string]*powernapconfig.ServerConfig{
				"fake": {Command: server.path, FileTypes: []string{"go"}},
			})
			m.SetHost(hub.host(HostServer{Name: "fake", Command: fakeServerName}))

			m.Start(t.Context(), file)

			client := requireClient(t, m, "fake")
			require.Equal(t, StateReady, client.GetServerState())
			_, hosted := client.Host()
			require.False(t, hosted, "the server runs locally")
			require.Contains(t, server.received(), "initialize")
			require.Equal(t, tc.dials, hub.helloCount())
			require.True(t, m.recentlyUnavailable(hostKeyPrefix+"fake"))

			// While the backoff lasts the host is not tried again, not
			// even after the local server stopped.
			m.StopAll(t.Context())
			m.Start(t.Context(), file)
			client = requireClient(t, m, "fake")
			_, hosted = client.Host()
			require.False(t, hosted)
			require.Equal(t, tc.dials, hub.helloCount())
		})
	}
}

// TestStopAllDetachesFromTheHost covers Crush's new-session teardown: the
// hosted client says goodbye and its connection ends, nothing else of the
// host's is touched, and the next start connects again.
func TestStopAllDetachesFromTheHost(t *testing.T) {
	t.Parallel()
	dir, file := projectDir(t)
	hub := newFakeHub(t)
	server := newFakeServer(t)

	m, cb := newTestManager(t, dir, map[string]*powernapconfig.ServerConfig{
		"fake": {Command: server.path, FileTypes: []string{"go"}},
	})
	m.SetHost(hub.host(HostServer{Name: "fake", Command: fakeServerName}))

	m.Start(t.Context(), file)
	first := requireClient(t, m, "fake")
	require.NoError(t, first.OpenFile(t.Context(), file))
	require.Equal(t, 1, hub.sessions())

	m.StopAll(t.Context())

	require.Zero(t, m.Clients().Len())
	require.Equal(t, StateStopped, first.GetServerState(), "stopped, not failed")
	require.True(t, cb.reported(first, StateStopped))
	require.Eventually(t, func() bool { return hub.sessions() == 0 }, waitFor, 10*time.Millisecond)
	uri := string(protocol.URIFromPath(file))
	messages := hub.messages()
	closed := slices.Index(messages, "textDocument/didClose="+uri)
	shutdown := slices.Index(messages, "shutdown")
	require.GreaterOrEqual(t, closed, 0, "%v", messages)
	require.Greater(t, shutdown, closed, "%v", messages)
	require.Zero(t, server.pid(), "no local server was ever started")

	// The hub is still there: the next start is a new session on it.
	m.Start(t.Context(), file)
	second := requireClient(t, m, "fake")
	require.NotSame(t, first, second)
	_, hosted := second.Host()
	require.True(t, hosted)
	require.Equal(t, StateReady, second.GetServerState())
	require.Equal(t, 2, hub.helloCount())
	require.Equal(t, 1, hub.sessions())
}

// TestLostHostConnection covers the host going away under a running
// client: the client stops being ready on its own, and the next start
// connects again, or starts the server locally when the host is gone.
func TestLostHostConnection(t *testing.T) {
	t.Parallel()
	dir, file := projectDir(t)
	hub := newFakeHub(t)
	server := newFakeServer(t)

	m, cb := newTestManager(t, dir, map[string]*powernapconfig.ServerConfig{
		"fake": {Command: server.path, FileTypes: []string{"go"}},
	})
	m.SetHost(hub.host(HostServer{Name: "fake", Command: fakeServerName}))

	m.Start(t.Context(), file)
	first := requireClient(t, m, "fake")
	require.Equal(t, StateReady, first.GetServerState())

	// The host drops the connection but keeps listening, as when its
	// server crashed.
	hub.dropSessions()
	require.Eventually(t, func() bool {
		_, ok := m.Clients().Get("fake")
		return !ok
	}, waitFor, 10*time.Millisecond, "a client whose host is gone is dropped")
	require.Equal(t, StateError, first.GetServerState())
	require.False(t, first.alive())
	require.Eventually(t, func() bool { return cb.reported(first, StateError) }, waitFor, 10*time.Millisecond)

	m.Start(t.Context(), file)
	second := requireClient(t, m, "fake")
	require.NotSame(t, first, second)
	_, hosted := second.Host()
	require.True(t, hosted, "the next start connects again")
	require.Equal(t, StateReady, second.GetServerState())
	require.Equal(t, 2, hub.helloCount())
	require.Zero(t, server.pid())

	// The host exits: its socket is gone as well.
	hub.close()
	require.Eventually(t, func() bool {
		_, ok := m.Clients().Get("fake")
		return !ok
	}, waitFor, 10*time.Millisecond)

	m.Start(t.Context(), file)
	third := requireClient(t, m, "fake")
	_, hosted = third.Host()
	require.False(t, hosted, "without the host the server runs locally")
	require.Equal(t, StateReady, third.GetServerState())
	require.NotZero(t, server.pid())
}

// TestLostLocalServer covers a local server that dies: the client must not
// stay ready forever. Nothing reports the exit, so it is noticed on the
// next message to the server, and the start after that spawns a new one.
func TestLostLocalServer(t *testing.T) {
	t.Parallel()
	dir, file := projectDir(t)
	server := newFakeServer(t)

	m, cb := newTestManager(t, dir, map[string]*powernapconfig.ServerConfig{
		"fake": {Command: server.path, FileTypes: []string{"go"}},
	})

	m.Start(t.Context(), file)
	first := requireClient(t, m, "fake")
	require.Equal(t, StateReady, first.GetServerState())
	require.NoError(t, first.OpenFile(t.Context(), file))
	pid := server.pid()
	require.NotZero(t, pid)

	require.NoError(t, syscall.Kill(pid, syscall.SIGKILL))
	require.Eventually(t, func() bool {
		return first.NotifyChange(t.Context(), file) != nil
	}, waitFor, 10*time.Millisecond, "writing to a dead server fails")
	require.Eventually(t, func() bool {
		_, ok := m.Clients().Get("fake")
		return !ok
	}, waitFor, 10*time.Millisecond, "a client whose server is gone is dropped")
	require.Equal(t, StateError, first.GetServerState())
	require.Eventually(t, func() bool { return cb.reported(first, StateError) }, waitFor, 10*time.Millisecond)

	m.Start(t.Context(), file)
	second := requireClient(t, m, "fake")
	require.NotSame(t, first, second)
	require.Equal(t, StateReady, second.GetServerState())
	require.NotEqual(t, pid, server.pid(), "a new server was spawned")
}

// TestHostSupersedesLocalServer covers a host that announces itself after
// its server was already started locally, as on a session restore: the
// running client is replaced by a hosted one without a gap, and the local
// server is closed afterwards.
func TestHostSupersedesLocalServer(t *testing.T) {
	t.Parallel()
	dir, file := projectDir(t)
	hub := newFakeHub(t)
	server := newFakeServer(t)

	m, cb := newTestManager(t, dir, map[string]*powernapconfig.ServerConfig{
		"fake": {Command: server.path, FileTypes: []string{"go"}},
	})

	m.Start(t.Context(), file)
	local := requireClient(t, m, "fake")
	_, hosted := local.Host()
	require.False(t, hosted)
	pid := server.pid()
	require.NotZero(t, pid)

	m.SetHost(hub.host(HostServer{Name: "fake", Command: fakeServerName}))

	var replacement *Client
	require.Eventually(t, func() bool {
		client, ok := m.Clients().Get("fake")
		require.True(t, ok, "there is a client at every moment")
		if _, hosted := client.Host(); !hosted {
			return false
		}
		replacement = client
		return true
	}, waitFor, time.Millisecond)
	require.Equal(t, StateReady, replacement.GetServerState())
	require.Eventually(t, func() bool { return cb.reported(replacement, StateReady) }, waitFor, 10*time.Millisecond)
	require.Equal(t, 1, hub.helloCount())

	// The local server is closed properly, a little later.
	require.Eventually(t, func() bool { return processGone(pid) }, waitFor, 10*time.Millisecond)
	require.Contains(t, server.received(), "shutdown")

	// The hosted client stays: it is not disturbed by the old one going.
	require.Same(t, replacement, requireClient(t, m, "fake"))
	require.Equal(t, StateReady, replacement.GetServerState())
	require.Equal(t, 1, hub.sessions())
}

// TestHostThatRefusesKeepsLocalServer covers a host that announces a
// server and then does not serve it: the local server keeps running.
func TestHostThatRefusesKeepsLocalServer(t *testing.T) {
	t.Parallel()
	dir, file := projectDir(t)
	hub := newFakeHub(t)
	hub.setRefuse("outside the host's project")
	server := newFakeServer(t)

	m, _ := newTestManager(t, dir, map[string]*powernapconfig.ServerConfig{
		"fake": {Command: server.path, FileTypes: []string{"go"}},
	})
	m.Start(t.Context(), file)
	local := requireClient(t, m, "fake")
	pid := server.pid()

	m.SetHost(hub.host(HostServer{Name: "fake", Command: fakeServerName}))

	require.Eventually(t, func() bool { return m.recentlyUnavailable(hostKeyPrefix + "fake") }, waitFor, 10*time.Millisecond)
	require.Equal(t, 1, hub.helloCount())
	require.Same(t, local, requireClient(t, m, "fake"))
	require.Equal(t, StateReady, local.GetServerState())
	require.False(t, processGone(pid))
}

// testClock is a clock for a manager that tests can move ahead, past a
// backoff for example.
type testClock struct {
	ahead atomic.Int64
}

func (c *testClock) now() time.Time {
	return time.Now().Add(time.Duration(c.ahead.Load()))
}

func (c *testClock) advance(d time.Duration) {
	c.ahead.Add(int64(d))
}

func isHosted(m *Manager, name string) bool {
	client, ok := m.Clients().Get(name)
	if !ok {
		return false
	}
	_, hosted := client.Host()
	return hosted
}

// TestHostIsTriedAgainAfterTheBackoff covers a server that was started
// locally because its host failed once. The host does not announce itself
// again when it recovers, so the starts that find the local server running
// have to try: once the backoff is over the server moves to the host, and
// a host that still fails is left alone for another backoff.
func TestHostIsTriedAgainAfterTheBackoff(t *testing.T) {
	t.Parallel()
	dir, file := projectDir(t)
	hub := newFakeHub(t)
	hub.setRefuse("too many sessions")
	server := newFakeServer(t)

	m, _ := newTestManager(t, dir, map[string]*powernapconfig.ServerConfig{
		"fake": {Command: server.path, FileTypes: []string{"go"}},
	})
	clock := &testClock{}
	m.now = clock.now
	m.SetHost(hub.host(HostServer{Name: "fake", Command: fakeServerName}))

	m.Start(t.Context(), file)
	local := requireClient(t, m, "fake")
	_, hosted := local.Host()
	require.False(t, hosted)
	require.Equal(t, 1, hub.helloCount())
	pid := server.pid()
	require.NotZero(t, pid)

	// During the backoff the starts leave the host alone.
	for range 3 {
		m.Start(t.Context(), file)
	}
	require.Equal(t, 1, hub.helloCount())

	// The backoff is over and the host still refuses: one more attempt,
	// however many starts there are, and another backoff. The local
	// server is untouched.
	clock.advance(unavailableRetryDelay + time.Second)
	require.False(t, m.recentlyUnavailable(hostKeyPrefix+"fake"))
	var starts sync.WaitGroup
	for range 5 {
		starts.Go(func() { m.Start(t.Context(), file) })
	}
	starts.Wait()
	require.Eventually(t, func() bool { return m.recentlyUnavailable(hostKeyPrefix + "fake") }, waitFor, 10*time.Millisecond)
	require.Equal(t, 2, hub.helloCount())
	for range 3 {
		m.Start(t.Context(), file)
	}
	require.Equal(t, 2, hub.helloCount())
	require.Same(t, local, requireClient(t, m, "fake"))
	require.Equal(t, StateReady, local.GetServerState())
	require.False(t, processGone(pid))

	// The host recovers without saying so. The first start after the
	// backoff moves the server over, without blocking and without a gap.
	hub.setRefuse("")
	clock.advance(unavailableRetryDelay + time.Second)
	m.Start(t.Context(), file)
	require.Eventually(t, func() bool {
		_, ok := m.Clients().Get("fake")
		require.True(t, ok, "there is a client at every moment")
		return isHosted(m, "fake")
	}, waitFor, time.Millisecond)
	require.Equal(t, 3, hub.helloCount())
	require.Equal(t, StateReady, requireClient(t, m, "fake").GetServerState())
	require.Eventually(t, func() bool { return processGone(pid) }, waitFor, 10*time.Millisecond,
		"the local server is closed once the host took over")

	// Starts leave a hosted server alone.
	for range 3 {
		m.Start(t.Context(), file)
	}
	require.Equal(t, 3, hub.helloCount())
	require.Equal(t, 1, hub.sessions())
}

// TestHostTakenOverOnTheSameSocket covers the host's socket changing
// hands: the process serving it exits, a start falls back to a local
// server before another process of the host application listens on the
// same path, and nothing is announced because nothing changed. The server
// has to find its way back to the host.
func TestHostTakenOverOnTheSameSocket(t *testing.T) {
	t.Parallel()
	dir, file := projectDir(t)
	first := newFakeHub(t)
	server := newFakeServer(t)

	m, _ := newTestManager(t, dir, map[string]*powernapconfig.ServerConfig{
		"fake": {Command: server.path, FileTypes: []string{"go"}},
	})
	clock := &testClock{}
	m.now = clock.now
	m.SetHost(first.host(HostServer{Name: "fake", Command: fakeServerName}))

	m.Start(t.Context(), file)
	require.True(t, isHosted(m, "fake"))

	first.close()
	require.Eventually(t, func() bool {
		_, ok := m.Clients().Get("fake")
		return !ok
	}, waitFor, 10*time.Millisecond)
	m.Start(t.Context(), file)
	require.False(t, isHosted(m, "fake"), "nobody listens: the server runs locally")
	require.Equal(t, StateReady, requireClient(t, m, "fake").GetServerState())

	second := newFakeHubAt(t, first.socket)
	clock.advance(unavailableRetryDelay + time.Second)
	m.Start(t.Context(), file)

	require.Eventually(t, func() bool { return isHosted(m, "fake") }, waitFor, 10*time.Millisecond)
	require.Equal(t, 1, second.helloCount())
	require.Equal(t, StateReady, requireClient(t, m, "fake").GetServerState())
}

// TestHostAnnouncedDuringLocalStart covers a host that announces itself
// while its server is being started locally: after the start looked for a
// host, and before its client is there for the announcement to move. The
// start has to look again when it is done.
func TestHostAnnouncedDuringLocalStart(t *testing.T) {
	t.Parallel()
	dir, file := projectDir(t)
	hub := newFakeHub(t)
	server := newFakeServer(t)

	m, _ := newTestManager(t, dir, map[string]*powernapconfig.ServerConfig{
		"fake": {Command: server.path, FileTypes: []string{"go"}},
	})
	// The command is looked up after the start decided there is no host
	// and before it creates its client.
	var announce sync.Once
	m.lookPath = func(command string) (string, error) {
		announce.Do(func() {
			m.SetHost(hub.host(HostServer{Name: "fake", Command: fakeServerName}))
			// Long enough for the announcement to look at the clients
			// and find none.
			time.Sleep(200 * time.Millisecond)
		})
		return exec.LookPath(command)
	}

	m.Start(t.Context(), file)

	// The start itself was local.
	pid := server.pid()
	require.NotZero(t, pid)
	// No other start is needed for the server to end up on the host.
	require.Eventually(t, func() bool {
		_, ok := m.Clients().Get("fake")
		require.True(t, ok, "there is a client at every moment")
		return isHosted(m, "fake")
	}, waitFor, time.Millisecond)
	require.Equal(t, 1, hub.helloCount())
	require.Eventually(t, func() bool { return processGone(pid) }, waitFor, 10*time.Millisecond)
}

// TestStopDuringHostedStart covers a hosted start that is cut short by
// StopAll or KillAll, when a new session begins while the host is still
// starting its server for example. That is not a failure of the host: no
// local server may be started behind the stop, and the host is not put in
// backoff.
func TestStopDuringHostedStart(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		stop func(m *Manager)
	}{
		// The client is killed while it waits for the answer to
		// initialize.
		{name: "KillAll", stop: func(m *Manager) { m.KillAll(context.Background()) }},
		// The client is closed gracefully: the answer still arrives, and
		// the stop is noticed while waiting for the server to be ready.
		{name: "StopAll", stop: func(m *Manager) { m.StopAll(context.Background()) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir, file := projectDir(t)
			hub := newFakeHub(t)
			hub.setInitializeDelay(700 * time.Millisecond)
			server := newFakeServer(t)

			m, _ := newTestManager(t, dir, map[string]*powernapconfig.ServerConfig{
				"fake": {Command: server.path, FileTypes: []string{"go"}},
			})
			m.SetHost(hub.host(HostServer{Name: "fake", Command: fakeServerName}))

			started := make(chan struct{})
			go func() {
				defer close(started)
				m.Start(context.Background(), file)
			}()
			require.Eventually(t, func() bool {
				return slices.Contains(hub.messages(), "initialize")
			}, waitFor, 5*time.Millisecond)
			client := requireClient(t, m, "fake")
			require.Equal(t, StateStarting, client.GetServerState())

			tc.stop(m)
			require.Zero(t, m.Clients().Len())

			select {
			case <-started:
			case <-time.After(waitFor):
				require.FailNow(t, "the start did not return")
			}
			require.Zero(t, m.Clients().Len(), "nothing was started behind the stop")
			require.Zero(t, server.pid(), "no local server was started")
			require.False(t, m.recentlyUnavailable(hostKeyPrefix+"fake"), "the host did not fail")
			require.Equal(t, StateStopped, client.GetServerState())

			// The host is still used by the next start.
			hub.setInitializeDelay(0)
			m.Start(t.Context(), file)
			require.True(t, isHosted(m, "fake"))
			require.Equal(t, StateReady, requireClient(t, m, "fake").GetServerState())
		})
	}
}

// TestLostWhileBecomingReady covers a connection that goes away between
// the handshake and the client being ready. The start must notice at once
// rather than wait for a server that is gone until its timeout, and then
// falls back like for any other host failure.
func TestLostWhileBecomingReady(t *testing.T) {
	t.Parallel()
	dir, file := projectDir(t)
	hub := newFakeHub(t)
	hub.setDropInitialized(true)
	server := newFakeServer(t)

	m, _ := newTestManager(t, dir, map[string]*powernapconfig.ServerConfig{
		"fake": {Command: server.path, FileTypes: []string{"go"}},
	})
	m.SetHost(hub.host(HostServer{Name: "fake", Command: fakeServerName}))

	begin := time.Now()
	m.Start(t.Context(), file)
	// The start includes the local fallback, which takes half a second
	// to be ready. The initialization timeout is 30 seconds.
	require.Less(t, time.Since(begin), 10*time.Second, "the start waited for a server that was gone")

	require.Equal(t, 1, hub.helloCount())
	client := requireClient(t, m, "fake")
	_, hosted := client.Host()
	require.False(t, hosted, "the server runs locally")
	require.Equal(t, StateReady, client.GetServerState())
	require.True(t, m.recentlyUnavailable(hostKeyPrefix+"fake"))
}

// TestWaitForServerReadyGivesUpOnALostClient is the same for the wait
// itself, with a local server that dies.
func TestWaitForServerReadyGivesUpOnALostClient(t *testing.T) {
	t.Parallel()
	dir, _ := projectDir(t)
	client, server := newFakeClient(t, dir)

	require.NoError(t, syscall.Kill(server.pid(), syscall.SIGKILL))
	client.markLost()

	begin := time.Now()
	err := client.WaitForServerReady(t.Context())
	require.ErrorIs(t, err, errConnectionLost)
	require.Less(t, time.Since(begin), 5*time.Second)
	require.Equal(t, StateError, client.GetServerState())
}

// TestRestartDuringStart covers a restart of a client that a start is
// still waiting for, as when lsp_restart runs while servers are started in
// the background. The restart closes the client on its way, which the
// start must not take for the client being stopped: the client stays the
// server's client, and the server the restart started is the one the
// manager knows and stops.
func TestRestartDuringStart(t *testing.T) {
	t.Parallel()
	dir, file := projectDir(t)
	server := newFakeServer(t)
	// The restart is still closing the old server when the start looks
	// at the client again.
	server.slowShutdown(t)

	m, _ := newTestManager(t, dir, map[string]*powernapconfig.ServerConfig{
		"fake": {Command: server.path, FileTypes: []string{"go"}},
	})

	started := make(chan struct{})
	go func() {
		defer close(started)
		m.Start(context.Background(), file)
	}()
	// The handshake is done; the start waits for the server to be ready.
	require.Eventually(t, func() bool {
		return slices.Contains(server.received(), "initialized")
	}, waitFor, 5*time.Millisecond)
	client := requireClient(t, m, "fake")
	first := server.pid()

	require.NoError(t, client.Restart())
	restarted := server.pid()
	require.NotEqual(t, first, restarted)

	select {
	case <-started:
	case <-time.After(waitFor):
		require.FailNow(t, "the start did not return")
	}
	require.Same(t, client, requireClient(t, m, "fake"), "the restarted client is still the server's client")
	require.Equal(t, StateReady, client.GetServerState())

	// The next start finds it, and a stop reaches the restarted server.
	m.Start(t.Context(), file)
	require.Same(t, client, requireClient(t, m, "fake"))
	require.Equal(t, restarted, server.pid(), "no other server was started")
	m.KillAll(t.Context())
	require.Eventually(t, func() bool { return processGone(restarted) }, waitFor, 10*time.Millisecond)
}

// TestStopDuringSwap covers a stop that arrives while a local server is
// being moved to the host. The old client is still the server's client
// while it is being closed; a swap that completes then would leave a
// hosted client running behind the stop.
func TestStopDuringSwap(t *testing.T) {
	t.Parallel()
	dir, file := projectDir(t)
	hub := newFakeHub(t)
	// The swap is under way when the stop begins, and done before the
	// stop is: the local server takes longer to shut down.
	hub.setInitializeDelay(300 * time.Millisecond)
	server := newFakeServer(t)
	server.slowShutdown(t)

	m, cb := newTestManager(t, dir, map[string]*powernapconfig.ServerConfig{
		"fake": {Command: server.path, FileTypes: []string{"go"}},
	})
	m.Start(t.Context(), file)
	local := requireClient(t, m, "fake")
	pid := server.pid()

	m.SetHost(hub.host(HostServer{Name: "fake", Command: fakeServerName}))
	require.Eventually(t, func() bool { return hub.helloCount() == 1 }, waitFor, 5*time.Millisecond)

	begin := time.Now()
	m.StopAll(t.Context())
	require.Greater(t, time.Since(begin), time.Second, "the swap had the time to complete during the stop")

	require.Zero(t, m.Clients().Len(), "nothing runs behind the stop")
	require.Eventually(t, func() bool { return hub.sessions() == 0 }, waitFor, 10*time.Millisecond,
		"the hosted client that came too late is closed")
	require.Zero(t, m.Clients().Len())
	require.Equal(t, StateStopped, local.GetServerState())
	require.True(t, cb.reported(local, StateStopped))
	require.Eventually(t, func() bool { return processGone(pid) }, waitFor, 10*time.Millisecond)
	require.False(t, m.recentlyUnavailable(hostKeyPrefix+"fake"), "the host did not fail")
}

// TestStopReachesSupersededClients covers a stop shortly after a server
// moved to the host. The local server it ran on is kept for a while and is
// no longer the server's client; the stop has to stop it all the same,
// rather than leave it to a timer that may never fire.
func TestStopReachesSupersededClients(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		stop     func(m *Manager)
		graceful bool
	}{
		{name: "KillAll", stop: func(m *Manager) { m.KillAll(context.Background()) }},
		{name: "StopAll", stop: func(m *Manager) { m.StopAll(context.Background()) }, graceful: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir, file := projectDir(t)
			hub := newFakeHub(t)
			server := newFakeServer(t)

			m, _ := newTestManager(t, dir, map[string]*powernapconfig.ServerConfig{
				"fake": {Command: server.path, FileTypes: []string{"go"}},
			})
			// Far longer than the test: only the stop can close it.
			m.supersededCloseDelay = time.Hour

			m.Start(t.Context(), file)
			pid := server.pid()
			require.NotZero(t, pid)
			m.SetHost(hub.host(HostServer{Name: "fake", Command: fakeServerName}))
			require.Eventually(t, func() bool { return isHosted(m, "fake") }, waitFor, 5*time.Millisecond)
			require.False(t, processGone(pid), "the local server is kept for a while")

			tc.stop(m)

			require.Zero(t, m.Clients().Len())
			require.Eventually(t, func() bool { return processGone(pid) }, waitFor, 10*time.Millisecond,
				"the superseded local server is stopped too")
			if tc.graceful {
				require.Contains(t, server.received(), "shutdown")
			}
			require.Eventually(t, func() bool { return hub.sessions() == 0 }, waitFor, 10*time.Millisecond)
		})
	}
}

// TestSwapCarriesOpenFiles covers what a tool finds right after a server
// moved to the host: the files that were open on the local server are open
// on the host's as well, with its diagnostics, from the moment the hosted
// client is the server's client.
func TestSwapCarriesOpenFiles(t *testing.T) {
	t.Parallel()
	dir, file := projectDir(t)
	other := filepath.Join(dir, "other.go")
	require.NoError(t, os.WriteFile(other, []byte("package main\n"), 0o644))
	hub := newFakeHub(t)
	hub.setPublishOnOpen(true)
	server := newFakeServer(t)

	m, _ := newTestManager(t, dir, map[string]*powernapconfig.ServerConfig{
		"fake": {Command: server.path, FileTypes: []string{"go"}},
	})
	m.Start(t.Context(), file)
	local := requireClient(t, m, "fake")
	require.NoError(t, local.OpenFile(t.Context(), file))
	require.NoError(t, local.OpenFile(t.Context(), other))

	m.SetHost(hub.host(HostServer{Name: "fake", Command: fakeServerName}))

	// What the first look at the hosted client sees, with no waiting.
	var (
		hosted      *Client
		open        [2]bool
		diagnostics [2]int
	)
	require.Eventually(t, func() bool {
		client, ok := m.Clients().Get("fake")
		require.True(t, ok)
		if _, isHosted := client.Host(); !isHosted {
			return false
		}
		hosted = client
		for i, path := range []string{file, other} {
			open[i] = client.IsFileOpen(path)
			diagnostics[i] = len(client.GetFileDiagnostics(protocol.URIFromPath(path)))
		}
		return true
	}, waitFor, time.Millisecond)

	require.Equal(t, [2]bool{true, true}, open, "the open files were carried over before the swap")
	require.Equal(t, [2]int{1, 1}, diagnostics, "with the diagnostics of the host's server")
	for _, path := range []string{file, other} {
		require.Contains(t, hub.messages(), "textDocument/didOpen="+string(protocol.URIFromPath(path)))
	}

	// The hosted client is then brought up to date with the disk, for a
	// change that went to the old client meanwhile.
	require.Eventually(t, func() bool {
		return slices.Contains(hub.messages(), "textDocument/didChange="+string(protocol.URIFromPath(file)))
	}, waitFor, 10*time.Millisecond)
	require.Same(t, hosted, requireClient(t, m, "fake"))
}

// TestHostThatStopsReading covers a host that is suspended in the middle
// of a session: it keeps its connections and reads nothing. Writing to it
// must not block for as long as that lasts. The command that connects the
// client gives up, the client is dropped like one whose host is gone, and
// the next start runs the server locally.
func TestHostThatStopsReading(t *testing.T) {
	t.Parallel()
	dir, file := projectDir(t)
	// More than the socket buffers on the way to the host take.
	require.NoError(t, os.WriteFile(file, []byte("package main\n\n// "+strings.Repeat("x", 4<<20)+"\n"), 0o644))
	hub := newFakeHub(t)
	server := newFakeServer(t)

	m, _ := newTestManager(t, dir, map[string]*powernapconfig.ServerConfig{
		"fake": {Command: server.path, FileTypes: []string{"go"}},
	})
	m.SetHost(hub.host(HostServer{Name: "fake", Command: fakeServerName}))
	m.Start(t.Context(), file)
	client := requireClient(t, m, "fake")
	require.True(t, isHosted(m, "fake"))

	hub.freeze()
	begin := time.Now()
	sent := make(chan struct{})
	go func() {
		defer close(sent)
		// Blocks once the buffers are full, or fails once the command
		// gave up; either way it has to return.
		_ = client.OpenFile(context.Background(), file)
		_ = client.NotifyChange(context.Background(), file)
	}()

	select {
	case <-sent:
	case <-time.After(lsphost.WriteTimeout + waitFor):
		require.FailNow(t, "writing to a host that stopped reading blocked")
	}
	require.Eventually(t, func() bool {
		_, ok := m.Clients().Get("fake")
		return !ok
	}, lsphost.WriteTimeout+waitFor, 10*time.Millisecond, "the client is dropped")
	require.GreaterOrEqual(t, time.Since(begin), lsphost.WriteTimeout-time.Second, "the host was given its time")
	require.Equal(t, StateError, client.GetServerState())

	// The host does not answer a new hello either: the server runs
	// locally, and the host is left alone for a while.
	m.Start(t.Context(), file)
	require.False(t, isHosted(m, "fake"))
	require.Equal(t, StateReady, requireClient(t, m, "fake").GetServerState())
	require.True(t, m.recentlyUnavailable(hostKeyPrefix+"fake"))
}

// newFakeClient returns an initialized client of a local fake server.
func newFakeClient(t *testing.T, dir string, args ...string) (*Client, *fakeServer) {
	t.Helper()
	server := newFakeServer(t)
	client, err := New("fake", config.LSPConfig{
		Command:   server.path,
		Args:      args,
		FileTypes: []string{"go"},
	}, config.NewShellVariableResolver(env.New()), dir, false)
	require.NoError(t, err)
	t.Cleanup(client.Shutdown)
	_, err = client.Initialize(t.Context(), dir)
	require.NoError(t, err)
	require.NoError(t, client.WaitForServerReady(t.Context()))
	return client, server
}

func TestCloseExitsOnlyAfterShutdown(t *testing.T) {
	t.Parallel()

	t.Run("shutdown answered", func(t *testing.T) {
		t.Parallel()
		dir, _ := projectDir(t)
		client, server := newFakeClient(t, dir)
		pid := server.pid()

		require.NoError(t, client.Close(t.Context()))

		require.Eventually(t, func() bool { return processGone(pid) }, waitFor, 10*time.Millisecond)
		require.Equal(t, []string{"initialize", "initialized", "shutdown"}, server.received())
	})

	t.Run("shutdown refused", func(t *testing.T) {
		t.Parallel()
		dir, _ := projectDir(t)
		client, server := newFakeClient(t, dir, "--fail-shutdown")
		pid := server.pid()

		require.Error(t, client.Close(t.Context()))

		require.Eventually(t, func() bool { return processGone(pid) }, waitFor, 10*time.Millisecond,
			"a server that does not shut down is killed")
		require.Equal(t, []string{"initialize", "initialized", "shutdown"}, server.received())
		require.False(t, server.exited(), "exit is not sent to a server that did not shut down")
	})
}

func TestRestartReopensFiles(t *testing.T) {
	t.Parallel()
	dir, file := projectDir(t)
	client, server := newFakeClient(t, dir)
	uri := string(protocol.URIFromPath(file))

	require.NoError(t, client.OpenFile(t.Context(), file))
	require.True(t, client.IsFileOpen(file))
	first := server.pid()

	require.NoError(t, client.Restart())

	require.NotEqual(t, first, server.pid(), "a new server was started")
	require.Equal(t, StateReady, client.GetServerState())
	require.True(t, client.IsFileOpen(file))
	require.Eventually(t, func() bool {
		return slices.Equal(server.received(), []string{
			"initialize", "initialized", "textDocument/didOpen=" + uri,
			"textDocument/didClose=" + uri, "shutdown",
			"initialize", "initialized", "textDocument/didOpen=" + uri,
		})
	}, waitFor, 10*time.Millisecond, "%v", server.received())
}
