package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/charmbracelet/crush/internal/checkpoint"
	"github.com/charmbracelet/crush/internal/fork"
	"github.com/charmbracelet/crush/internal/worktree"
)

// SnapshotsEnabled returns whether snapshots are enabled for a workspace.
func (c *Client) SnapshotsEnabled(ctx context.Context, workspaceID string) (bool, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/snapshots/enabled", workspaceID), nil, nil)
	if err != nil {
		return false, fmt.Errorf("failed to check snapshots enabled: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to check snapshots enabled: status code %d", rsp.StatusCode)
	}
	var result struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(rsp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}
	return result.Enabled, nil
}

// ListSnapshots returns all snapshots for a session.
func (c *Client) ListSnapshots(ctx context.Context, workspaceID, sessionID string) ([]*checkpoint.Snapshot, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/sessions/%s/snapshots", workspaceID, sessionID), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list snapshots: status code %d", rsp.StatusCode)
	}
	var snapshots []*checkpoint.Snapshot
	if err := json.NewDecoder(rsp.Body).Decode(&snapshots); err != nil {
		return nil, fmt.Errorf("failed to decode snapshots: %w", err)
	}
	return snapshots, nil
}

// GetSnapshot retrieves a snapshot by ID.
func (c *Client) GetSnapshot(ctx context.Context, workspaceID, snapshotID string) (*checkpoint.Snapshot, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/snapshots/%s", workspaceID, snapshotID), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get snapshot: status code %d", rsp.StatusCode)
	}
	var snapshot checkpoint.Snapshot
	if err := json.NewDecoder(rsp.Body).Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("failed to decode snapshot: %w", err)
	}
	return &snapshot, nil
}

// GetSnapshotByMessage retrieves a snapshot by message ID.
func (c *Client) GetSnapshotByMessage(ctx context.Context, workspaceID, messageID string) (*checkpoint.Snapshot, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/messages/%s/snapshot", workspaceID, messageID), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot by message: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get snapshot by message: status code %d", rsp.StatusCode)
	}
	var snapshot checkpoint.Snapshot
	if err := json.NewDecoder(rsp.Body).Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("failed to decode snapshot: %w", err)
	}
	return &snapshot, nil
}

// RestoreSnapshot restores a workspace to a snapshot.
func (c *Client) RestoreSnapshot(ctx context.Context, workspaceID, snapshotID string) error {
	rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/snapshots/%s/restore", workspaceID, snapshotID), nil, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to restore snapshot: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to restore snapshot: status code %d", rsp.StatusCode)
	}
	return nil
}

// DiffFromCurrentSnapshot returns the diff from current filesystem to a snapshot.
func (c *Client) DiffFromCurrentSnapshot(ctx context.Context, workspaceID, snapshotID string) (string, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/snapshots/%s/diff", workspaceID, snapshotID), nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get snapshot diff: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get snapshot diff: status code %d", rsp.StatusCode)
	}
	var result struct {
		Diff string `json:"diff"`
	}
	if err := json.NewDecoder(rsp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode diff: %w", err)
	}
	return result.Diff, nil
}

// SnapshotGC runs garbage collection on snapshots.
func (c *Client) SnapshotGC(ctx context.Context, workspaceID string) (int64, error) {
	rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/snapshots/gc", workspaceID), nil, nil, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to run snapshot GC: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to run snapshot GC: status code %d", rsp.StatusCode)
	}
	var result struct {
		BytesFreed int64 `json:"bytes_freed"`
	}
	if err := json.NewDecoder(rsp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode GC result: %w", err)
	}
	return result.BytesFreed, nil
}

// SnapshotStats returns snapshot storage statistics.
func (c *Client) SnapshotStats(ctx context.Context, workspaceID string) (*checkpoint.Stats, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/snapshots/stats", workspaceID), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot stats: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get snapshot stats: status code %d", rsp.StatusCode)
	}
	var stats checkpoint.Stats
	if err := json.NewDecoder(rsp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to decode stats: %w", err)
	}
	return &stats, nil
}

// WorktreesEnabled returns whether worktrees are enabled for a workspace.
func (c *Client) WorktreesEnabled(ctx context.Context, workspaceID string) (bool, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/worktrees/enabled", workspaceID), nil, nil)
	if err != nil {
		return false, fmt.Errorf("failed to check worktrees enabled: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to check worktrees enabled: status code %d", rsp.StatusCode)
	}
	var result struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(rsp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}
	return result.Enabled, nil
}

// ListWorktrees returns all worktrees for a session.
func (c *Client) ListWorktrees(ctx context.Context, workspaceID, sessionID string) ([]*worktree.Worktree, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/sessions/%s/worktrees", workspaceID, sessionID), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list worktrees: status code %d", rsp.StatusCode)
	}
	var worktrees []*worktree.Worktree
	if err := json.NewDecoder(rsp.Body).Decode(&worktrees); err != nil {
		return nil, fmt.Errorf("failed to decode worktrees: %w", err)
	}
	return worktrees, nil
}

// ListAllWorktrees returns all worktrees for a workspace.
func (c *Client) ListAllWorktrees(ctx context.Context, workspaceID string) ([]*worktree.Worktree, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/worktrees", workspaceID), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list all worktrees: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list all worktrees: status code %d", rsp.StatusCode)
	}
	var worktrees []*worktree.Worktree
	if err := json.NewDecoder(rsp.Body).Decode(&worktrees); err != nil {
		return nil, fmt.Errorf("failed to decode worktrees: %w", err)
	}
	return worktrees, nil
}

// GetWorktree retrieves a worktree by ID.
func (c *Client) GetWorktree(ctx context.Context, workspaceID, worktreeID string) (*worktree.Worktree, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/worktrees/%s", workspaceID, worktreeID), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get worktree: status code %d", rsp.StatusCode)
	}
	var wt worktree.Worktree
	if err := json.NewDecoder(rsp.Body).Decode(&wt); err != nil {
		return nil, fmt.Errorf("failed to decode worktree: %w", err)
	}
	return &wt, nil
}

// GetActiveWorktree retrieves the active worktree for a session.
func (c *Client) GetActiveWorktree(ctx context.Context, workspaceID, sessionID string) (*worktree.Worktree, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/sessions/%s/worktrees/active", workspaceID, sessionID), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get active worktree: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get active worktree: status code %d", rsp.StatusCode)
	}
	var wt worktree.Worktree
	if err := json.NewDecoder(rsp.Body).Decode(&wt); err != nil {
		return nil, fmt.Errorf("failed to decode worktree: %w", err)
	}
	return &wt, nil
}

// CreateWorktree creates a new worktree.
func (c *Client) CreateWorktree(ctx context.Context, workspaceID, sessionID, name, fromSnapshotID string) (*worktree.Worktree, error) {
	body := struct {
		Name           string `json:"name"`
		FromSnapshotID string `json:"from_snapshot_id"`
	}{
		Name:           name,
		FromSnapshotID: fromSnapshotID,
	}
	rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/sessions/%s/worktrees", workspaceID, sessionID), nil, jsonBody(body), http.Header{"Content-Type": []string{"application/json"}})
	if err != nil {
		return nil, fmt.Errorf("failed to create worktree: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create worktree: status code %d", rsp.StatusCode)
	}
	var wt worktree.Worktree
	if err := json.NewDecoder(rsp.Body).Decode(&wt); err != nil {
		return nil, fmt.Errorf("failed to decode worktree: %w", err)
	}
	return &wt, nil
}

// SwitchWorktree switches to a different worktree.
func (c *Client) SwitchWorktree(ctx context.Context, workspaceID, sessionID, worktreeID string) error {
	rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/sessions/%s/worktrees/%s/switch", workspaceID, sessionID, worktreeID), nil, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to switch worktree: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to switch worktree: status code %d", rsp.StatusCode)
	}
	return nil
}

// DeleteWorktree deletes a worktree.
func (c *Client) DeleteWorktree(ctx context.Context, workspaceID, worktreeID string) error {
	rsp, err := c.delete(ctx, fmt.Sprintf("/workspaces/%s/worktrees/%s", workspaceID, worktreeID), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete worktree: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete worktree: status code %d", rsp.StatusCode)
	}
	return nil
}

// ForkConversation creates a fork of a conversation.
func (c *Client) ForkConversation(ctx context.Context, workspaceID string, params fork.ForkParams) (*fork.ForkResult, error) {
	rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/fork", workspaceID), nil, jsonBody(params), http.Header{"Content-Type": []string{"application/json"}})
	if err != nil {
		return nil, fmt.Errorf("failed to fork conversation: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fork conversation: status code %d", rsp.StatusCode)
	}
	var result fork.ForkResult
	if err := json.NewDecoder(rsp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode fork result: %w", err)
	}
	return &result, nil
}
