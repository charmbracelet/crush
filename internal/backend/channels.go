package backend

import (
	"context"
	"log/slog"
	"sort"

	mcptools "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/session"
)

// channelSessionStore is the slice of session.Service the channel router
// needs to pick or create a target session. Narrowed so tests can fake it
// without implementing the full service.
type channelSessionStore interface {
	Create(ctx context.Context, title string) (session.Session, error)
	Get(ctx context.Context, id string) (session.Session, error)
	List(ctx context.Context) ([]session.Session, error)
}

// startChannelRouter subscribes to MCP events for the backend's lifetime
// and injects each channel push into exactly one session per opted-in
// workspace. In client/server mode the server owns channel routing:
// frontends do not inject (see workspace.ClientWorkspace.RoutesChannelEvents),
// so a push is processed once no matter how many clients are attached —
// including zero, which keeps the "a pushed event is never dropped"
// contract for a headless `crush serve`.
//
// Events are processed sequentially on one goroutine, so pushes keep
// their arrival order and the init/inject sequence per event does not
// race with itself.
func (b *Backend) startChannelRouter() {
	events := mcptools.SubscribeEvents(b.ctx)
	go func() {
		for ev := range events {
			if ev.Payload.Type != mcptools.EventChannelMessage || ev.Payload.ChannelMessage == "" {
				continue
			}
			b.routeChannelMessage(ev.Payload)
		}
	}()
}

// routeChannelMessage delivers one channel push to every hosted workspace
// that both declares the originating MCP server in its config and opted it
// in via --channels. The MCP event broker is process-global, so the
// config check is what scopes a push to the workspace(s) that actually
// enabled the channel.
func (b *Backend) routeChannelMessage(ev mcptools.Event) {
	for _, ws := range b.workspaces.Seq2() {
		cfg := ws.Cfg.Config()
		if _, declared := cfg.MCP[ev.Name]; !declared {
			continue
		}
		if !mcptools.ChannelEnabled(ws.Cfg.Overrides().EnabledChannels, ev.Name) {
			continue
		}
		b.injectChannelMessage(ws, ev.Name, ev.ChannelMessage)
	}
}

// injectChannelMessage runs one rendered <channel> element as an agent
// turn in ws. The coder agent is initialized on demand so a headless
// server (where no client has called InitCoderAgent yet) can still
// process pushes. Failures are logged and the push is dropped for this
// workspace; there is no caller to return an error to.
func (b *Backend) injectChannelMessage(ws *Workspace, serverName, content string) {
	if ws.AgentCoordinator == nil {
		if err := ws.InitCoderAgent(ws.ctx); err != nil {
			slog.Warn("Channel message dropped: coder agent init failed",
				"workspace", ws.ID, "server", serverName, "error", err)
			return
		}
	}
	sessionID, err := channelTargetSession(ws.ctx, ws.viewedSessions(), ws.Sessions)
	if err != nil {
		slog.Warn("Channel message dropped: no target session",
			"workspace", ws.ID, "server", serverName, "error", err)
		return
	}
	if err := b.SendMessage(ws.ID, proto.AgentMessage{
		SessionID: sessionID,
		Channel:   serverName,
		Prompt:    content,
	}); err != nil {
		slog.Warn("Channel message dropped: dispatch failed",
			"workspace", ws.ID, "server", serverName, "session", sessionID, "error", err)
	}
}

// channelTargetSession picks the session a channel push should land in,
// mirroring the in-process TUI semantics ("route into the session you
// have open; start one if none is open") as closely as a multi-client
// server can:
//
//   - exactly one session is being viewed by attached clients: use it;
//   - several distinct sessions are viewed: use the most recently
//     updated of them (ties broken by smallest ID for determinism);
//   - none viewed (all clients on the landing screen, or no clients):
//     use the most recently updated top-level session so repeated
//     pushes coalesce into one conversation, creating a session only
//     when the workspace has none.
func channelTargetSession(ctx context.Context, viewed []string, sessions channelSessionStore) (string, error) {
	switch len(viewed) {
	case 0:
		existing, err := sessions.List(ctx)
		if err != nil {
			return "", err
		}
		if len(existing) > 0 {
			return existing[0].ID, nil
		}
		sess, err := sessions.Create(ctx, "New Session")
		if err != nil {
			return "", err
		}
		return sess.ID, nil
	case 1:
		return viewed[0], nil
	}

	best := ""
	var bestUpdated int64 = -1
	for _, id := range viewed {
		sess, err := sessions.Get(ctx, id)
		if err != nil {
			// A viewed session that cannot be loaded (e.g. deleted
			// while still selected client-side) is skipped rather
			// than failing the whole push.
			continue
		}
		if sess.UpdatedAt > bestUpdated || (sess.UpdatedAt == bestUpdated && sess.ID < best) {
			best = sess.ID
			bestUpdated = sess.UpdatedAt
		}
	}
	if best != "" {
		return best, nil
	}
	// Every viewed session failed to load; fall back to the no-viewed
	// path.
	return channelTargetSession(ctx, nil, sessions)
}

// viewedSessions returns the distinct sessions currently being viewed by
// clients with at least one live SSE stream, sorted for determinism.
// Hold-only clients and clients on the landing screen do not contribute.
func (w *Workspace) viewedSessions() []string {
	w.clientsMu.Lock()
	set := make(map[string]struct{})
	for _, cs := range w.clients {
		if cs.streams > 0 && cs.currentSessionID != "" {
			set[cs.currentSessionID] = struct{}{}
		}
	}
	w.clientsMu.Unlock()

	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
