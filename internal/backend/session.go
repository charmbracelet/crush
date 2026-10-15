package backend

import (
	"context"
	"fmt"

	"github.com/charmbracelet/crush/internal/agent"
	mcptools "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/shell"
)

// CreateSession creates a new session in the given workspace.
func (b *Backend) CreateSession(ctx context.Context, workspaceID, title string) (session.Session, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return session.Session{}, err
	}

	return ws.Sessions.Create(ctx, title)
}

// GetSession retrieves a session by workspace and session ID.
func (b *Backend) GetSession(ctx context.Context, workspaceID, sessionID string) (session.Session, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return session.Session{}, err
	}

	return ws.Sessions.Get(ctx, sessionID)
}

// ListSessions returns all sessions in the given workspace.
func (b *Backend) ListSessions(ctx context.Context, workspaceID string) ([]session.Session, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.Sessions.List(ctx)
}

// GetAgentSession returns session metadata with the agent's busy
// status.
func (b *Backend) GetAgentSession(ctx context.Context, workspaceID, sessionID string) (proto.AgentSession, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return proto.AgentSession{}, err
	}

	se, err := ws.Sessions.Get(ctx, sessionID)
	if err != nil {
		return proto.AgentSession{}, err
	}

	var isSessionBusy bool
	if ws.AgentCoordinator != nil {
		isSessionBusy = ws.AgentCoordinator.IsSessionBusy(sessionID)
	}

	return proto.AgentSession{
		Session: proto.Session{
			ID:    se.ID,
			Title: se.Title,
		},
		IsBusy:        isSessionBusy,
		PendingPrompt: b.hasPendingPrompt(ctx, ws, sessionID),
	}, nil
}

// hasPendingPrompt reports whether the workspace's current permission
// request or question belongs to sessionID or one of its sub-sessions.
// The services are nil on workspaces with a stubbed app (tests).
func (b *Backend) hasPendingPrompt(ctx context.Context, ws *Workspace, sessionID string) bool {
	if ws.Permissions != nil {
		if req, ok := ws.Permissions.ActiveRequest(); ok && b.sessionMatches(ctx, ws, req.SessionID, sessionID) {
			return true
		}
	}
	if ws.Questions != nil {
		if req, ok := ws.Questions.Pending(); ok && b.sessionMatches(ctx, ws, req.SessionID, sessionID) {
			return true
		}
	}
	return false
}

// ListSessionMessages returns all messages for a session.
func (b *Backend) ListSessionMessages(ctx context.Context, workspaceID, sessionID string) ([]message.Message, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	// Drain debounced updates so HTTP clients (and the TUI on session
	// switch) observe the latest in-memory state, and settle any calls a
	// dead process left dangling. Every remote client reads through here,
	// so skipping the settle showed interrupted calls as still running.
	var busy agent.SessionBusyFunc
	if coord := ws.AgentCoordinator; coord != nil {
		busy = agent.BusyWithAncestors(ctx, ws.Sessions, coord.IsSessionBusy)
	}
	return agent.ReadSettledMessages(ctx, ws.Messages, busy, sessionID)
}

// ListSessionHistory returns the history items for a session.
func (b *Backend) ListSessionHistory(ctx context.Context, workspaceID, sessionID string) (any, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.History.ListBySession(ctx, sessionID)
}

// SaveSession updates a session in the given workspace.
func (b *Backend) SaveSession(ctx context.Context, workspaceID string, sess session.Session) (session.Session, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return session.Session{}, err
	}

	return ws.Sessions.Save(ctx, sess)
}

// MCPServersDisabled returns the MCP servers disabled for the given
// workspace's repository.
func (b *Backend) MCPServersDisabled(ctx context.Context, workspaceID string) ([]string, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.Sessions.MCPDisabledServers(ctx)
}

// MCPServersEnabled returns the MCP servers with a repository-scoped
// enabled override (config-disabled servers the user enabled here).
func (b *Backend) MCPServersEnabled(ctx context.Context, workspaceID string) ([]string, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.Sessions.MCPServersEnabled(ctx)
}

// SetMCPServerDisabled toggles a repository-scoped MCP override for the
// given workspace and applies it to the running client.
func (b *Backend) SetMCPServerDisabled(ctx context.Context, workspaceID, name string, disabled bool) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	if err := ws.Sessions.SetMCPServerDisabled(ctx, name, disabled); err != nil {
		return err
	}
	return mcptools.SetLocalDisabled(ctx, ws.Cfg, name, disabled)
}

// SetMCPServerConfigDisabled toggles an MCP server's disabled flag in the
// global config and applies the change to the running client.
func (b *Backend) SetMCPServerConfigDisabled(ctx context.Context, workspaceID, name string, disabled bool) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	override, err := session.HasMCPOverride(ctx, ws.Sessions, name)
	if err != nil {
		return err
	}
	return mcptools.SetConfigDisabled(ctx, ws.Cfg, config.ScopeGlobal, name, disabled, override)
}

// StartMCPServer starts the named MCP server for the given workspace even
// when its config entry is disabled. Runtime-only.
func (b *Backend) StartMCPServer(ctx context.Context, workspaceID, name string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	return mcptools.InitializeSingleForced(ctx, name, ws.Cfg)
}

// ErrJobNotFound: không có job id trong workspace, hoặc output của job đã xong
// không còn được giữ (cùng sentinel với shell để lỗi chi tiết của shell vẫn
// thành 404).
var ErrJobNotFound = shell.ErrJobNotFound

// ListJobs trả các job (lệnh bash do agent chạy) trong thư mục của workspace.
func (b *Backend) ListJobs(workspaceID string) ([]shell.JobInfo, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	return shell.GetBackgroundShellManager().Jobs(ws.Path), nil
}

// jobInWorkspace cho biết job id thuộc workspace (server có thể giữ nhiều
// workspace; trình quản lý job là chung cả tiến trình).
func jobInWorkspace(ws *Workspace, jobID string) bool {
	for _, j := range shell.GetBackgroundShellManager().Jobs(ws.Path) {
		if j.ID == jobID {
			return true
		}
	}
	return false
}

// JobOutput trả output của một job trong workspace.
func (b *Backend) JobOutput(workspaceID, jobID string) (string, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return "", err
	}
	if !jobInWorkspace(ws, jobID) {
		return "", fmt.Errorf("%w in this workspace: %s", ErrJobNotFound, jobID)
	}
	return shell.GetBackgroundShellManager().JobOutput(jobID)
}

// KillJob dừng một job đang chạy trong workspace; job đã xong thì không làm
// gì. Job vẫn nằm trong bảng job để agent đọc được output và trạng thái.
func (b *Backend) KillJob(ctx context.Context, workspaceID, jobID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	if !jobInWorkspace(ws, jobID) {
		return fmt.Errorf("%w in this workspace: %s", ErrJobNotFound, jobID)
	}
	return shell.GetBackgroundShellManager().Stop(ctx, jobID)
}

// RenameSession đổi tiêu đề session, nguyên tử (một câu UPDATE chỉ cột
// title), không đè các trường khác mà run nền có thể vừa ghi.
func (b *Backend) RenameSession(ctx context.Context, workspaceID, sessionID, title string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	return ws.Sessions.Rename(ctx, sessionID, title)
}

// DeleteSession deletes a session from the given workspace.
func (b *Backend) DeleteSession(ctx context.Context, workspaceID, sessionID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	ws.runMu.Lock()
	coord := ws.AgentCoordinator
	ws.runMu.Unlock()
	if coord != nil && (coord.IsSessionBusy(sessionID) || coord.QueuedPrompts(sessionID) > 0) {
		return ErrSessionBusy
	}
	if err := ws.Sessions.Delete(ctx, sessionID); err != nil {
		return err
	}
	ws.Permissions.SetAcceptEdits(sessionID, false)
	return nil
}

// ListUserMessages returns user-role messages for a session.
func (b *Backend) ListUserMessages(ctx context.Context, workspaceID, sessionID string) ([]message.Message, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.Messages.ListUserMessages(ctx, sessionID)
}

// ListAllUserMessages returns all user-role messages across sessions.
func (b *Backend) ListAllUserMessages(ctx context.Context, workspaceID string) ([]message.Message, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.Messages.ListAllUserMessages(ctx)
}
