package checkpoint

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/taigrr/crush/internal/db"
	"github.com/taigrr/crush/internal/pubsub"
	"github.com/google/uuid"
)

// Service coordinates snapshots with the database.
type Service interface {
	pubsub.Subscriber[Snapshot]

	// CreateSnapshot creates a snapshot for a user message.
	CreateSnapshot(ctx context.Context, sessionID, messageID, description string) (*Snapshot, error)

	// RestoreSnapshot restores to a snapshot.
	RestoreSnapshot(ctx context.Context, snapshotID string, targetDir string) error

	// GetSnapshot retrieves a snapshot by ID.
	GetSnapshot(ctx context.Context, snapshotID string) (*Snapshot, error)

	// GetSnapshotByMessage retrieves a snapshot by message ID.
	GetSnapshotByMessage(ctx context.Context, messageID string) (*Snapshot, error)

	// ListSnapshots returns all snapshots for a session.
	ListSnapshots(ctx context.Context, sessionID string) ([]*Snapshot, error)

	// GetSnapshotTree returns snapshots as a tree (for branched conversations).
	GetSnapshotTree(ctx context.Context, sessionID string) (*SnapshotTree, error)

	// DiffSnapshots returns the diff between two snapshots.
	DiffSnapshots(ctx context.Context, fromID, toID string) (string, error)

	// DiffFromCurrent returns the diff from current filesystem to a snapshot.
	DiffFromCurrent(ctx context.Context, snapshotID string) (string, error)

	// DeleteSnapshot deletes a snapshot and its ref.
	DeleteSnapshot(ctx context.Context, snapshotID string) error

	// DeleteSessionSnapshots deletes all snapshots for a session.
	DeleteSessionSnapshots(ctx context.Context, sessionID string) error

	// RunPostRestoreHooks runs configured post-restore commands.
	RunPostRestoreHooks(ctx context.Context, targetDir string) error

	// GC performs garbage collection, removing unreferenced objects.
	// Returns the number of bytes freed.
	GC(ctx context.Context) (int64, error)

	// GetStats returns statistics about the snapshot storage.
	GetStats(ctx context.Context) (*Stats, error)

	// IsEnabled returns whether snapshots are enabled.
	IsEnabled() bool
}

// Snapshot represents a filesystem snapshot.
type Snapshot struct {
	ID               string    `json:"id"`
	SessionID        string    `json:"session_id"`
	MessageID        string    `json:"message_id"`
	ParentSnapshotID string    `json:"parent_snapshot_id,omitempty"`
	GitCommitHash    string    `json:"git_commit_hash"`
	Description      string    `json:"description"`
	CreatedAt        time.Time `json:"created_at"`
}

// SnapshotTree represents the tree structure of snapshots.
type SnapshotTree struct {
	Root    *SnapshotNode `json:"root,omitempty"`
	Nodes   []*Snapshot   `json:"nodes"` // Flat list for iteration
	Current string        `json:"current,omitempty"`
}

// SnapshotNode represents a node in the snapshot tree.
type SnapshotNode struct {
	Snapshot *Snapshot       `json:"snapshot"`
	Children []*SnapshotNode `json:"children,omitempty"`
}

// PostRestoreHook defines a command to run after restoring a snapshot.
type PostRestoreHook struct {
	IfExists string `json:"if_exists"` // File to check for
	Run      string `json:"run"`       // Command to run
}

// Stats holds statistics about snapshot storage.
type Stats struct {
	SnapshotCount int   `json:"snapshot_count"`
	DiskUsage     int64 `json:"disk_usage"` // Bytes used by .crush/git
}

// service implements the Service interface.
type service struct {
	*pubsub.Broker[Snapshot]

	repo    *Repo
	queries *db.Queries
	conn    *sql.DB
	hooks   []PostRestoreHook
	enabled bool
}

// ServiceConfig holds configuration for the snapshot service.
type ServiceConfig struct {
	ProjectDir       string
	Enabled          bool
	Exclude          []string
	PostRestoreHooks []PostRestoreHook
}

// NewService creates a new snapshot service.
func NewService(cfg ServiceConfig, queries *db.Queries, conn *sql.DB) (Service, error) {
	if !cfg.Enabled {
		return &service{
			Broker:  pubsub.NewBroker[Snapshot](),
			enabled: false,
		}, nil
	}

	repoCfg := &Config{
		Exclude: cfg.Exclude,
	}
	if len(repoCfg.Exclude) == 0 {
		repoCfg = DefaultConfig()
	}

	repo, err := InitRepo(cfg.ProjectDir, repoCfg)
	if err != nil {
		return nil, fmt.Errorf("init checkpoint repo: %w", err)
	}

	return &service{
		Broker:  pubsub.NewBroker[Snapshot](),
		repo:    repo,
		queries: queries,
		conn:    conn,
		hooks:   cfg.PostRestoreHooks,
		enabled: true,
	}, nil
}

func (s *service) IsEnabled() bool {
	return s.enabled
}

func (s *service) CreateSnapshot(ctx context.Context, sessionID, messageID, description string) (*Snapshot, error) {
	if !s.enabled {
		return nil, errors.New("snapshots not enabled")
	}

	// Check if snapshot already exists for this message.
	existing, err := s.GetSnapshotByMessage(ctx, messageID)
	if err == nil && existing != nil {
		return existing, nil
	}

	// Find parent snapshot (most recent for this session).
	var parentSnapshotID string
	snapshots, err := s.ListSnapshots(ctx, sessionID)
	if err == nil && len(snapshots) > 0 {
		parentSnapshotID = snapshots[0].ID
	}

	// Create git snapshot.
	commitHash, err := s.repo.CreateSnapshotRef(sessionID, messageID, description)
	if err != nil {
		return nil, fmt.Errorf("create git snapshot: %w", err)
	}

	// Create database record.
	id := uuid.New().String()
	now := time.Now()

	snapshot := &Snapshot{
		ID:               id,
		SessionID:        sessionID,
		MessageID:        messageID,
		ParentSnapshotID: parentSnapshotID,
		GitCommitHash:    commitHash,
		Description:      truncateDescription(description),
		CreatedAt:        now,
	}

	if err := s.insertSnapshot(ctx, snapshot); err != nil {
		// Try to clean up git ref on failure.
		_ = s.repo.DeleteSnapshotRef(sessionID, messageID)
		return nil, fmt.Errorf("insert snapshot: %w", err)
	}

	s.Publish(pubsub.CreatedEvent, *snapshot)

	return snapshot, nil
}

func (s *service) RestoreSnapshot(ctx context.Context, snapshotID string, targetDir string) error {
	if !s.enabled {
		return errors.New("snapshots not enabled")
	}

	snapshot, err := s.GetSnapshot(ctx, snapshotID)
	if err != nil {
		return err
	}

	if targetDir == "" {
		targetDir = s.repo.ProjectDir()
	}

	// Restore filesystem.
	if err := s.repo.RestoreSnapshot(snapshot.GitCommitHash, targetDir); err != nil {
		return fmt.Errorf("restore filesystem: %w", err)
	}

	// Run post-restore hooks.
	if err := s.RunPostRestoreHooks(ctx, targetDir); err != nil {
		// Log but don't fail - hooks are best-effort.
		slog.Debug("Post-restore hooks failed", "error", err)
	}

	return nil
}

func (s *service) GetSnapshot(ctx context.Context, snapshotID string) (*Snapshot, error) {
	if !s.enabled {
		return nil, errors.New("snapshots not enabled")
	}

	row, err := s.queries.GetSnapshot(ctx, snapshotID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSnapshotNotFound
		}
		return nil, err
	}

	return dbRowToSnapshot(row), nil
}

func (s *service) GetSnapshotByMessage(ctx context.Context, messageID string) (*Snapshot, error) {
	if !s.enabled {
		return nil, errors.New("snapshots not enabled")
	}

	row, err := s.queries.GetSnapshotByMessage(ctx, messageID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSnapshotNotFound
		}
		return nil, err
	}

	return dbRowToSnapshot(row), nil
}

func (s *service) ListSnapshots(ctx context.Context, sessionID string) ([]*Snapshot, error) {
	if !s.enabled {
		return nil, nil
	}

	rows, err := s.queries.ListSnapshots(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	snapshots := make([]*Snapshot, len(rows))
	for i, row := range rows {
		snapshots[i] = dbRowToSnapshot(row)
	}

	return snapshots, nil
}

func (s *service) GetSnapshotTree(ctx context.Context, sessionID string) (*SnapshotTree, error) {
	snapshots, err := s.ListSnapshots(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if len(snapshots) == 0 {
		return &SnapshotTree{}, nil
	}

	// Build tree from flat list.
	nodeMap := make(map[string]*SnapshotNode)
	var roots []*SnapshotNode

	// First pass: create all nodes.
	for _, snap := range snapshots {
		nodeMap[snap.ID] = &SnapshotNode{Snapshot: snap}
	}

	// Second pass: link children to parents.
	for _, snap := range snapshots {
		node := nodeMap[snap.ID]
		if snap.ParentSnapshotID == "" {
			roots = append(roots, node)
		} else if parent, ok := nodeMap[snap.ParentSnapshotID]; ok {
			parent.Children = append(parent.Children, node)
		} else {
			// Parent not found, treat as root.
			roots = append(roots, node)
		}
	}

	tree := &SnapshotTree{
		Nodes: snapshots,
	}

	if len(roots) > 0 {
		tree.Root = roots[0]
	}

	// Current is the most recent snapshot.
	if len(snapshots) > 0 {
		tree.Current = snapshots[0].ID
	}

	return tree, nil
}

func (s *service) DiffSnapshots(ctx context.Context, fromID, toID string) (string, error) {
	if !s.enabled {
		return "", errors.New("snapshots not enabled")
	}

	from, err := s.GetSnapshot(ctx, fromID)
	if err != nil {
		return "", fmt.Errorf("get from snapshot: %w", err)
	}

	to, err := s.GetSnapshot(ctx, toID)
	if err != nil {
		return "", fmt.Errorf("get to snapshot: %w", err)
	}

	return s.repo.Diff(from.GitCommitHash, to.GitCommitHash)
}

func (s *service) DiffFromCurrent(ctx context.Context, snapshotID string) (string, error) {
	if !s.enabled {
		return "", errors.New("snapshots not enabled")
	}

	snapshot, err := s.GetSnapshot(ctx, snapshotID)
	if err != nil {
		return "", err
	}

	// Create a temporary snapshot of current state.
	currentHash, err := s.repo.CreateSnapshot("current state (temp)")
	if err != nil {
		return "", fmt.Errorf("snapshot current state: %w", err)
	}

	return s.repo.Diff(snapshot.GitCommitHash, currentHash)
}

func (s *service) DeleteSnapshot(ctx context.Context, snapshotID string) error {
	if !s.enabled {
		return errors.New("snapshots not enabled")
	}

	snapshot, err := s.GetSnapshot(ctx, snapshotID)
	if err != nil {
		return err
	}

	// Delete git ref.
	_ = s.repo.DeleteSnapshotRef(snapshot.SessionID, snapshot.MessageID)

	// Delete database record.
	if err := s.queries.DeleteSnapshot(ctx, snapshotID); err != nil {
		return err
	}

	s.Publish(pubsub.DeletedEvent, *snapshot)

	return nil
}

func (s *service) DeleteSessionSnapshots(ctx context.Context, sessionID string) error {
	if !s.enabled {
		return nil
	}

	snapshots, err := s.ListSnapshots(ctx, sessionID)
	if err != nil {
		return err
	}

	for _, snap := range snapshots {
		_ = s.repo.DeleteSnapshotRef(snap.SessionID, snap.MessageID)
	}

	return s.queries.DeleteSessionSnapshots(ctx, sessionID)
}

func (s *service) RunPostRestoreHooks(ctx context.Context, targetDir string) error {
	if len(s.hooks) == 0 {
		return nil
	}

	// TODO: Implement hook execution using shell package.
	// For now, this is a placeholder.
	// The hooks should check if IfExists file exists and run the command.

	return nil
}

// insertSnapshot inserts a snapshot into the database.
func (s *service) insertSnapshot(ctx context.Context, snap *Snapshot) error {
	return s.queries.CreateSnapshot(ctx, db.CreateSnapshotParams{
		ID:               snap.ID,
		SessionID:        snap.SessionID,
		MessageID:        snap.MessageID,
		ParentSnapshotID: toNullString(snap.ParentSnapshotID),
		GitCommitHash:    snap.GitCommitHash,
		Description:      toNullString(snap.Description),
		CreatedAt:        snap.CreatedAt.UnixMilli(),
	})
}

func dbRowToSnapshot(row any) *Snapshot {
	switch r := row.(type) {
	case db.Snapshot:
		return &Snapshot{
			ID:               r.ID,
			SessionID:        r.SessionID,
			MessageID:        r.MessageID,
			ParentSnapshotID: fromNullString(r.ParentSnapshotID),
			GitCommitHash:    r.GitCommitHash,
			Description:      fromNullString(r.Description),
			CreatedAt:        time.UnixMilli(r.CreatedAt),
		}
	case *db.Snapshot:
		return &Snapshot{
			ID:               r.ID,
			SessionID:        r.SessionID,
			MessageID:        r.MessageID,
			ParentSnapshotID: fromNullString(r.ParentSnapshotID),
			GitCommitHash:    r.GitCommitHash,
			Description:      fromNullString(r.Description),
			CreatedAt:        time.UnixMilli(r.CreatedAt),
		}
	default:
		return nil
	}
}

func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func fromNullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func truncateDescription(s string) string {
	const maxLen = 100
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func (s *service) GC(ctx context.Context) (int64, error) {
	if !s.enabled || s.repo == nil {
		return 0, nil
	}

	// Get size before GC.
	statsBefore, err := s.GetStats(ctx)
	if err != nil {
		return 0, fmt.Errorf("get stats before GC: %w", err)
	}

	// Run git gc.
	if err := s.repo.GC(); err != nil {
		return 0, fmt.Errorf("run GC: %w", err)
	}

	// Get size after GC.
	statsAfter, err := s.GetStats(ctx)
	if err != nil {
		return 0, fmt.Errorf("get stats after GC: %w", err)
	}

	freed := max(0, statsBefore.DiskUsage-statsAfter.DiskUsage)
	return freed, nil
}

func (s *service) GetStats(ctx context.Context) (*Stats, error) {
	stats := &Stats{}

	// Count snapshots.
	snapshots, err := s.queries.ListAllSnapshots(ctx)
	if err == nil {
		stats.SnapshotCount = len(snapshots)
	}

	// Calculate disk usage.
	if s.repo != nil {
		stats.DiskUsage = s.repo.DiskUsage()
	}

	return stats, nil
}
