package backend

import (
	"context"
	"log/slog"

	tea "charm.land/bubbletea/v2"

	mcptools "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
)

// SubscribeEvents returns a per-caller event channel for a workspace.
// Each caller receives all events; multiple callers do not compete.
func (b *Backend) SubscribeEvents(ctx context.Context, workspaceID string) (<-chan pubsub.Event[tea.Msg], error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.Events(ctx), nil
}

// maxSessionAncestryDepth bounds how far [Backend.RootSessionID] walks
// the parent chain when resolving a session to its top-level session.
// Sub-agents cannot spawn sub-agents today, so one hop suffices in
// practice; the extra slack guards against cycles and future nesting.
const maxSessionAncestryDepth = 4

// RootSessionID resolves sessionID to its top-level session by walking
// the parent chain. Sub-agent sessions (created by the agent tool or
// agentic fetch) carry derived IDs; their interactive prompts are
// scoped to the top-level session a user can actually view. A session
// with no parent resolves to itself. The sessionID is returned
// unchanged alongside any error, so callers can fall back to the
// unresolved ID.
func (b *Backend) RootSessionID(ctx context.Context, workspaceID, sessionID string) (string, error) {
	if sessionID == "" {
		return sessionID, nil
	}
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return sessionID, err
	}
	return rootSessionID(ctx, ws, sessionID)
}

// rootSessionID is the workspace-resolved variant of
// [Backend.RootSessionID]. Workspaces without an app (synthetic test
// workspaces) treat every session as top-level.
func rootSessionID(ctx context.Context, ws *Workspace, sessionID string) (string, error) {
	if sessionID == "" || ws.App == nil || ws.Sessions == nil {
		return sessionID, nil
	}
	id := sessionID
	for range maxSessionAncestryDepth {
		sess, err := ws.Sessions.Get(ctx, id)
		if err != nil {
			return sessionID, err
		}
		if sess.ParentSessionID == "" || sess.ParentSessionID == id {
			return id, nil
		}
		id = sess.ParentSessionID
	}
	return id, nil
}

// republishPendingPrompts re-publishes the workspace's pending
// permission request and question batch when their session matches the
// one a client just switched to. Without this, a prompt raised while
// nobody viewed the session would remain invisible — and therefore
// unanswerable — until the run was cancelled. Delivery is still scoped
// by the per-client SSE filter, so only the session's viewers (re)see
// the prompt.
func (b *Backend) republishPendingPrompts(ctx context.Context, ws *Workspace, sessionID string) {
	if ws.App == nil {
		return
	}
	// The individual services are nil on workspaces with a stubbed app
	// (tests); treat those as having no pending prompts.
	if ws.Permissions != nil {
		if req, ok := ws.Permissions.ActiveRequest(); ok && b.sessionMatches(ctx, ws, req.SessionID, sessionID) {
			ws.SendEvent(pubsub.Event[permission.PermissionRequest]{
				Type:    pubsub.CreatedEvent,
				Payload: req,
			})
		}
	}
	if ws.Questions != nil {
		if req, ok := ws.Questions.Pending(); ok && b.sessionMatches(ctx, ws, req.SessionID, sessionID) {
			ws.SendEvent(pubsub.Event[question.Request]{
				Type:    pubsub.CreatedEvent,
				Payload: req,
			})
		}
	}
}

// sessionMatches reports whether an event scoped to eventSessionID
// belongs to the given (top-level) session, either directly or through
// the event session's ancestry. Resolution failures fail closed here:
// the caller simply refrains from re-publishing.
func (b *Backend) sessionMatches(ctx context.Context, ws *Workspace, eventSessionID, sessionID string) bool {
	if eventSessionID == sessionID {
		return true
	}
	root, err := rootSessionID(ctx, ws, eventSessionID)
	if err != nil {
		slog.Debug("Failed to resolve root session", "session_id", eventSessionID, "error", err)
		return false
	}
	return root == sessionID
}

// GetLSPStates returns the state of all LSP clients.
func (b *Backend) GetLSPStates(workspaceID string) (map[string]app.LSPClientInfo, error) {
	_, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return app.GetLSPStates(), nil
}

// GetLSPDiagnostics returns diagnostics for a specific LSP client in
// the workspace.
func (b *Backend) GetLSPDiagnostics(workspaceID, lspName string) (any, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	for name, client := range ws.LSPManager.Clients().Seq2() {
		if name == lspName {
			return client.GetDiagnostics(), nil
		}
	}

	return nil, ErrLSPClientNotFound
}

// GetWorkspaceConfig returns the workspace-level configuration.
func (b *Backend) GetWorkspaceConfig(workspaceID string) (*config.Config, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.Cfg.Config(), nil
}

// GetWorkspaceProviders returns the configured providers for a
// workspace.
func (b *Backend) GetWorkspaceProviders(workspaceID string) (any, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	providers, _ := config.Providers(ws.Cfg.Config())
	return providers, nil
}

// LSPStart starts an LSP server for the given path.
func (b *Backend) LSPStart(ctx context.Context, workspaceID, path string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	ws.LSPManager.Start(ctx, path)
	return nil
}

// LSPStopAll stops all LSP servers for a workspace.
func (b *Backend) LSPStopAll(ctx context.Context, workspaceID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	ws.LSPManager.StopAll(ctx)
	return nil
}

// MCPGetStates returns the current state of all MCP clients.
func (b *Backend) MCPGetStates(_ string) map[string]mcptools.ClientInfo {
	return mcptools.GetStates()
}

// MCPRefreshPrompts refreshes prompts for a named MCP client.
func (b *Backend) MCPRefreshPrompts(ctx context.Context, _ string, name string) {
	mcptools.RefreshPrompts(ctx, name)
}

// MCPRefreshResources refreshes resources for a named MCP client.
func (b *Backend) MCPRefreshResources(ctx context.Context, _ string, name string) {
	mcptools.RefreshResources(ctx, name)
}

// MCPPendingAuth returns the MCP servers awaiting OAuth authentication,
// for clients that need to prompt the user. workspaceID selects the
// workspace whose config provides the server URLs.
func (b *Backend) MCPPendingAuth(workspaceID string) ([]mcptools.PendingAuthServer, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	return mcptools.PendingAuthMCPs(ws.Cfg), nil
}

// MCPAuthURL returns the current OAuth authorization URL for a named
// server, if a flow is in progress.
func (b *Backend) MCPAuthURL(name string) string {
	return mcptools.MCPAuthURL(name)
}

// MCPAuthenticate runs the OAuth flow for a named MCP server with the
// local browser suppressed: the authorization URL is exposed via
// MCPAuthURL/MCPPendingAuth for the calling client to open on the user's
// machine. The call blocks until the flow completes, fails, or ctx is
// cancelled. workspaceID selects the workspace whose config drives the
// flow.
func (b *Backend) MCPAuthenticate(ctx context.Context, workspaceID, name string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	finish, cancel, err := mcptools.BeginAuth(ws.Cfg, name)
	if err != nil {
		return err
	}
	defer cancel()
	return finish(ctx)
}
