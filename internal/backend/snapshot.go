package backend

import (
	"context"
	"errors"

	"github.com/taigrr/crush/internal/checkpoint"
)

// ErrSnapshotsDisabled is returned when snapshots are not enabled.
var ErrSnapshotsDisabled = errors.New("snapshots not enabled")

// SnapshotsEnabled returns whether snapshots are enabled for a workspace.
func (b *Backend) SnapshotsEnabled(workspaceID string) (bool, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return false, err
	}
	return ws.Checkpoints != nil && ws.Checkpoints.IsEnabled(), nil
}

// ListSnapshots returns all snapshots for a session.
func (b *Backend) ListSnapshots(ctx context.Context, workspaceID, sessionID string) ([]*checkpoint.Snapshot, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	if ws.Checkpoints == nil || !ws.Checkpoints.IsEnabled() {
		return nil, ErrSnapshotsDisabled
	}
	return ws.Checkpoints.ListSnapshots(ctx, sessionID)
}

// GetSnapshot retrieves a snapshot by ID.
func (b *Backend) GetSnapshot(ctx context.Context, workspaceID, snapshotID string) (*checkpoint.Snapshot, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	if ws.Checkpoints == nil || !ws.Checkpoints.IsEnabled() {
		return nil, ErrSnapshotsDisabled
	}
	return ws.Checkpoints.GetSnapshot(ctx, snapshotID)
}

// GetSnapshotByMessage retrieves a snapshot by message ID.
func (b *Backend) GetSnapshotByMessage(ctx context.Context, workspaceID, messageID string) (*checkpoint.Snapshot, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	if ws.Checkpoints == nil || !ws.Checkpoints.IsEnabled() {
		return nil, ErrSnapshotsDisabled
	}
	return ws.Checkpoints.GetSnapshotByMessage(ctx, messageID)
}

// RestoreSnapshot restores a workspace to a snapshot.
func (b *Backend) RestoreSnapshot(ctx context.Context, workspaceID, snapshotID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	if ws.Checkpoints == nil || !ws.Checkpoints.IsEnabled() {
		return ErrSnapshotsDisabled
	}
	return ws.Checkpoints.RestoreSnapshot(ctx, snapshotID, ws.Path)
}

// DiffFromCurrentSnapshot returns the diff from current filesystem to a snapshot.
func (b *Backend) DiffFromCurrentSnapshot(ctx context.Context, workspaceID, snapshotID string) (string, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return "", err
	}
	if ws.Checkpoints == nil || !ws.Checkpoints.IsEnabled() {
		return "", ErrSnapshotsDisabled
	}
	return ws.Checkpoints.DiffFromCurrent(ctx, snapshotID)
}

// SnapshotGC runs garbage collection on snapshots.
func (b *Backend) SnapshotGC(ctx context.Context, workspaceID string) (int64, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return 0, err
	}
	if ws.Checkpoints == nil || !ws.Checkpoints.IsEnabled() {
		return 0, ErrSnapshotsDisabled
	}
	return ws.Checkpoints.GC(ctx)
}

// SnapshotStats returns statistics about snapshot storage.
func (b *Backend) SnapshotStats(ctx context.Context, workspaceID string) (*checkpoint.Stats, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	if ws.Checkpoints == nil || !ws.Checkpoints.IsEnabled() {
		return nil, ErrSnapshotsDisabled
	}
	return ws.Checkpoints.GetStats(ctx)
}
