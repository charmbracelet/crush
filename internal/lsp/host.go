package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	powernapconfig "github.com/charmbracelet/x/powernap/pkg/config"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

const (
	// carryTimeout bounds carrying the open files of a client over to
	// the hosted client that replaces it.
	carryTimeout = 5 * time.Second

	// supersededCloseDelay is how long a client that was replaced by a
	// hosted one keeps running, so that requests already sent to it can
	// finish. It is longer than any request timeout of [Client].
	supersededCloseDelay = 10 * time.Second

	// hostKeyPrefix marks the entries of [Manager.unavailable] that are
	// about a host's server rather than a local command: a server whose
	// host failed to serve it runs locally, and the host is left alone
	// for [unavailableRetryDelay]. The first start after that tries the
	// host again, see [Manager.rehost].
	hostKeyPrefix = "host:"
)

// Host is an application, such as an editor that runs Crush in one of its
// panes, that shares its language servers. The manager connects to a
// server the host offers instead of starting its own.
type Host struct {
	// Name and Version name the host application, for logs and labels.
	Name    string
	Version string
	// PID is the process that announced the host. It is informational.
	PID int
	// Socket is the unix socket the host listens on.
	Socket string
	// Workspace is the ID of the Crush workspace the manager belongs to.
	// The host is told, so it can tell its clients apart.
	Workspace string
	// Servers are the language servers the host offers.
	Servers []HostServer
}

// HostServer is a language server a [Host] offers.
type HostServer struct {
	// Name is the host's name for the server.
	Name string
	// Command is the server's command. The manager uses the host's
	// server for its own server with the same command name.
	Command string
}

// hostTarget is one server of a host, as a client connects to it.
type hostTarget struct {
	name       string // The host application's name.
	socket     string
	server     string // The host's name for the server.
	workspace  string
	executable string // The crush executable that pumps to the socket.
}

// SetHost tells the manager which host application shares its language
// servers, or that none does when h is nil. It never blocks: servers that
// already run locally and that the host offers are moved to the host in
// the background, and servers started later connect to it directly.
//
// A host that goes away needs no call: its clients notice the closed
// connection, and the next start falls back to a local server.
func (s *Manager) SetHost(h *Host) {
	if s == nil {
		return
	}
	s.sharedHost.Store(h)
	// An announcement is news about the host: whatever failed before may
	// work now.
	for key := range s.unavailable.Seq2() {
		if strings.HasPrefix(key, hostKeyPrefix) {
			s.unavailable.Del(key)
		}
	}
	if h == nil {
		slog.Debug("LSP host cleared")
		return
	}
	slog.Info("LSP host set", "host", h.Name, "socket", h.Socket, "servers", len(h.Servers))
	go s.reconcile(h)
}

// Host returns the host application last given to [Manager.SetHost], or
// nil.
func (s *Manager) Host() *Host {
	if s == nil {
		return nil
	}
	return s.sharedHost.Load()
}

// hostTarget returns the host's server to use in place of the server name,
// or nil when the server has to run locally: there is no host, the host
// does not offer the server's command, the user configured the server in a
// way a shared server cannot honor, or the host recently failed to serve
// it.
func (s *Manager) hostTarget(name string, server *powernapconfig.ServerConfig) *hostTarget {
	h := s.sharedHost.Load()
	if h == nil || h.Socket == "" {
		return nil
	}
	command := filepath.Base(server.Command)
	i := slices.IndexFunc(h.Servers, func(hs HostServer) bool {
		return hs.Command != "" && filepath.Base(hs.Command) == command
	})
	if i < 0 {
		return nil
	}
	if s.customized(name, server) {
		if _, logged := s.hostSkipLogged.LoadOrStore(name, true); !logged {
			slog.Info("LSP server has its own arguments, environment or options, not using the host's",
				"name", name, "host", h.Name)
		}
		return nil
	}
	if s.recentlyUnavailable(hostKeyPrefix + name) {
		return nil
	}
	executable, err := s.executable()
	if err != nil {
		slog.Warn("Cannot find the crush executable to reach the LSP host", "error", err)
		return nil
	}
	return &hostTarget{
		name:       h.Name,
		socket:     h.Socket,
		server:     h.Servers[i].Name,
		workspace:  h.Workspace,
		executable: executable,
	}
}

// customized reports whether the server runs with arguments, environment,
// initialization options or settings other than the catalog's. The host
// runs the server with its own, so such a server is not shared.
func (s *Manager) customized(name string, server *powernapconfig.ServerConfig) bool {
	base := s.defaults[name]
	if base == nil {
		base = &powernapconfig.ServerConfig{}
	}
	return !sameJSON(server.Args, base.Args) ||
		!sameJSON(server.Environment, base.Environment) ||
		!sameJSON(server.InitOptions, base.InitOptions) ||
		!sameJSON(server.Settings, base.Settings)
}

// sameJSON reports whether two slices or maps hold the same values, taking
// nil and empty ones as the same.
func sameJSON(a, b any) bool {
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(emptyAsNull(ja)) == string(emptyAsNull(jb))
}

func emptyAsNull(j []byte) []byte {
	switch string(j) {
	case "[]", "{}":
		return []byte("null")
	}
	return j
}

// reconcile moves the running clients whose servers h offers over to h.
// Each is replaced only once its hosted replacement is ready, so there is
// never a gap, and is closed a while later. A server h fails to serve
// keeps running locally until a start tries again, see [Manager.rehost].
//
// A server started before the host announced itself, when a session is
// restored for example, ends up on the host this way. One that is being
// started at that very moment may not be among the clients yet; its start
// looks for a host again when it is done.
func (s *Manager) reconcile(h *Host) {
	for name, old := range s.clients.Seq2() {
		if s.sharedHost.Load() != h {
			return // A newer announcement is being reconciled.
		}
		if old.host != nil && old.host.socket == h.Socket {
			continue
		}
		switch old.GetServerState() {
		case StateReady, StateStarting:
		default:
			continue
		}
		server, ok := s.manager.GetServer(name)
		if !ok {
			continue
		}
		target := s.hostTarget(name, server)
		if target == nil {
			continue
		}
		s.supersede(name, old, server, target)
	}
}

// rehost moves the running client of the server name to the host when the
// host offers the server and the client is not on it: the host failed to
// serve it earlier and the server was started locally, or the host
// announced itself while the server was starting. It is called from the
// starts that find the server running.
//
// It never blocks: the move happens in the background, as in
// [Manager.reconcile], and the starts of one server make one attempt
// between them. A host that fails again is left alone for another
// [unavailableRetryDelay], during which [Manager.hostTarget] has no target.
//
// Without this a single failed attempt would keep a server local for as
// long as it runs. The host does not announce itself again when it
// recovers: another process of the host application may take over the
// same socket, which is no news to anyone but the socket.
func (s *Manager) rehost(name string, old *Client, server *powernapconfig.ServerConfig) {
	if old.GetServerState() != StateReady {
		return
	}
	h := s.sharedHost.Load()
	if h == nil || (old.host != nil && old.host.socket == h.Socket) {
		return
	}
	target := s.hostTarget(name, server)
	if target == nil {
		return
	}
	if _, busy := s.rehosting.LoadOrStore(name, struct{}{}); busy {
		return
	}
	go func() {
		defer s.rehosting.Delete(name)
		s.supersede(name, old, server, target)
	}()
}

// supersede replaces old, the running client of the server name, with a
// client of the host's server.
func (s *Manager) supersede(name string, old *Client, server *powernapconfig.ServerConfig, target *hostTarget) {
	cfg := s.buildConfig(name, server)
	client, err := s.connect(name, cfg, target)
	if errors.Is(err, errStopped) {
		return
	}
	if err != nil {
		slog.Warn("LSP host did not take over the server, keeping the local one",
			"name", name, "host", target.name, "error", err)
		s.markUnavailable(hostKeyPrefix + name)
		return
	}

	// The hosted server knows none of the files the old one has open.
	// Whoever asks the new client about them right after the swap, the
	// tool call whose start led here for one, has to find them there.
	carryOpenFiles(old, client)

	s.mu.Lock()
	current, ok := s.clients.Get(name)
	if !ok || current != old || old.closing.Load() {
		// The client was stopped or replaced meanwhile, or is being
		// stopped right now, see [Manager.stopping]; whoever did that
		// decides what runs now.
		s.mu.Unlock()
		client.Shutdown()
		return
	}
	s.clients.Set(name, client)
	s.retire(old)
	s.mu.Unlock()

	slog.Info("LSP server moved to the host", "name", name, "host", target.name)
	s.callback(name, client)

	// A change sent to the old client while the files were carried over
	// did not reach the new one: bring it up to date with the disk.
	ctx, cancel := context.WithTimeout(context.Background(), carryTimeout)
	defer cancel()
	client.RefreshOpenFiles(ctx)
}

// carryOpenFiles opens the files that are open in old in client as well,
// and gives the server the time a tool gives it to report on them.
func carryOpenFiles(old, client *Client) {
	ctx, cancel := context.WithTimeout(context.Background(), carryTimeout)
	defer cancel()

	carried := false
	for uri := range old.openFiles.Seq2() {
		path, err := protocol.DocumentURI(uri).Path()
		if err != nil {
			continue
		}
		if err := client.OpenFile(ctx, path); err != nil {
			slog.Debug("Failed to carry an open file over to the host's server", "file", path, "error", err)
			continue
		}
		carried = true
	}
	if carried {
		client.WaitForDiagnostics(ctx, carryTimeout)
	}
}

// retire keeps a client that was just replaced running for a while, so
// that requests already sent to it can finish, and then closes it. Until
// then the client is in s.superseded, where [Manager.StopAll] and
// [Manager.KillAll] find it. It is called with s.mu held.
func (s *Manager) retire(old *Client) {
	if s.superseded == nil {
		s.superseded = make(map[*Client]*time.Timer)
	}
	s.superseded[old] = time.AfterFunc(s.supersededDelay(), func() {
		s.mu.Lock()
		_, pending := s.superseded[old]
		delete(s.superseded, old)
		s.mu.Unlock()
		if !pending {
			return // A stop took it.
		}
		ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
		defer cancel()
		_ = old.Close(ctx)
		old.cancelCtx()
	})
}

func (s *Manager) supersededDelay() time.Duration {
	if s.supersededCloseDelay > 0 {
		return s.supersededCloseDelay
	}
	return supersededCloseDelay
}
