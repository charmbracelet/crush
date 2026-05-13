package backend

import (
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/taigrr/crush/internal/worktree"
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

// ListAllWorktrees returns all worktrees for a workspace.
func (b *Backend) ListAllWorktrees(ctx context.Context, workspaceID string) ([]*worktree.Worktree, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	if ws.Worktrees == nil || !ws.Worktrees.IsEnabled() {
		return nil, ErrWorktreesDisabled
	}
	return ws.Worktrees.ListAll(ctx)
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

// MergeWorktree merges or rebases a worktree onto a target branch.
func (b *Backend) MergeWorktree(ctx context.Context, workspaceID, worktreeID, targetBranch string, rebase bool) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	if ws.Worktrees == nil || !ws.Worktrees.IsEnabled() {
		return ErrWorktreesDisabled
	}
	return ws.Worktrees.Merge(ctx, worktreeID, targetBranch, rebase)
}

// ListGitBranches returns all git branches in the workspace.
func (b *Backend) ListGitBranches(_ context.Context, workspaceID string) ([]string, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	// Run git branch to list all branches.
	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	cmd.Dir = ws.Cfg.WorkingDir()
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var branches []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}
