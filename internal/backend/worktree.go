package backend

import (
	"context"
	"errors"

	"github.com/charmbracelet/crush/internal/worktree"
)

// ErrWorktreesDisabled is returned when worktrees are not enabled.
var ErrWorktreesDisabled = errors.New("worktrees not enabled")

// WorktreesEnabled returns whether worktrees are enabled for a workspace.
func (b *Backend) WorktreesEnabled(workspaceID string) (bool, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return false, err
	}
	return ws.Worktrees != nil && ws.Worktrees.IsEnabled(), nil
}

// ListWorktrees returns all worktrees for a session.
func (b *Backend) ListWorktrees(ctx context.Context, workspaceID, sessionID string) ([]*worktree.Worktree, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	if ws.Worktrees == nil || !ws.Worktrees.IsEnabled() {
		return nil, ErrWorktreesDisabled
	}
	return ws.Worktrees.List(ctx, sessionID)
}

// GetWorktree retrieves a worktree by ID.
func (b *Backend) GetWorktree(ctx context.Context, workspaceID, worktreeID string) (*worktree.Worktree, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	if ws.Worktrees == nil || !ws.Worktrees.IsEnabled() {
		return nil, ErrWorktreesDisabled
	}
	return ws.Worktrees.Get(ctx, worktreeID)
}

// GetActiveWorktree retrieves the active worktree for a session.
func (b *Backend) GetActiveWorktree(ctx context.Context, workspaceID, sessionID string) (*worktree.Worktree, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	if ws.Worktrees == nil || !ws.Worktrees.IsEnabled() {
		return nil, ErrWorktreesDisabled
	}
	return ws.Worktrees.GetActive(ctx, sessionID)
}

// CreateWorktree creates a new worktree.
func (b *Backend) CreateWorktree(ctx context.Context, workspaceID, sessionID, name, fromSnapshotID string) (*worktree.Worktree, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	if ws.Worktrees == nil || !ws.Worktrees.IsEnabled() {
		return nil, ErrWorktreesDisabled
	}
	return ws.Worktrees.Create(ctx, sessionID, name, fromSnapshotID)
}

// SwitchWorktree switches to a different worktree.
func (b *Backend) SwitchWorktree(ctx context.Context, workspaceID, sessionID, worktreeID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	if ws.Worktrees == nil || !ws.Worktrees.IsEnabled() {
		return ErrWorktreesDisabled
	}
	return ws.Worktrees.Switch(ctx, sessionID, worktreeID)
}

// DeleteWorktree deletes a worktree.
func (b *Backend) DeleteWorktree(ctx context.Context, workspaceID, worktreeID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	if ws.Worktrees == nil || !ws.Worktrees.IsEnabled() {
		return ErrWorktreesDisabled
	}
	return ws.Worktrees.Delete(ctx, worktreeID)
}
