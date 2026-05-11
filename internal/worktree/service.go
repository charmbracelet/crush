// Package worktree provides management of Crush-controlled git worktrees.
// Worktrees are stored in .crush/worktrees/ and allow parallel development
// with automatic dependency management.
package worktree

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/checkpoint"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// Common errors.
var (
	ErrWorktreeNotFound  = errors.New("worktree not found")
	ErrWorktreeExists    = errors.New("worktree already exists")
	ErrInvalidName       = errors.New("invalid worktree name")
	ErrNoActiveWorktree  = errors.New("no active worktree")
	ErrWorktreesDisabled = errors.New("worktrees not enabled")
)

// Service manages Crush worktrees.
type Service interface {
	pubsub.Subscriber[Worktree]

	// Create creates a new worktree, optionally from a snapshot.
	// If name is empty, generates one using conventional-commit style.
	Create(ctx context.Context, sessionID string, name string, fromSnapshotID string) (*Worktree, error)

	// Switch switches to a worktree, making it active.
	Switch(ctx context.Context, sessionID string, worktreeID string) error

	// Delete deletes a worktree and its files.
	Delete(ctx context.Context, worktreeID string) error

	// Get retrieves a worktree by ID.
	Get(ctx context.Context, worktreeID string) (*Worktree, error)

	// GetByName retrieves a worktree by session and name.
	GetByName(ctx context.Context, sessionID string, name string) (*Worktree, error)

	// GetActive returns the active worktree for a session.
	GetActive(ctx context.Context, sessionID string) (*Worktree, error)

	// List lists all worktrees for a session.
	List(ctx context.Context, sessionID string) ([]*Worktree, error)

	// ListAll lists all worktrees across all sessions.
	ListAll(ctx context.Context) ([]*Worktree, error)

	// GenerateName generates a worktree name from a description.
	GenerateName(description string) string

	// RunPostCreateHooks runs configured post-create commands.
	RunPostCreateHooks(ctx context.Context, worktreePath string) error

	// ValidateState checks for external modifications on startup.
	ValidateState(ctx context.Context) error

	// IsEnabled returns whether worktrees are enabled.
	IsEnabled() bool

	// WorktreesDir returns the directory where worktrees are stored.
	WorktreesDir() string
}

// Worktree represents a managed worktree.
type Worktree struct {
	ID             string    `json:"id"`
	SessionID      string    `json:"session_id"`
	Name           string    `json:"name"`
	Path           string    `json:"path"`
	BaseSnapshotID string    `json:"base_snapshot_id,omitempty"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
}

// service implements the Service interface.
type service struct {
	*pubsub.Broker[Worktree]

	queries     *db.Queries
	conn        *sql.DB
	checkpoints checkpoint.Service
	projectDir  string
	worktreeDir string
	hooks       []config.PostCreateHook
	enabled     bool
}

// ServiceConfig holds configuration for the worktree service.
type ServiceConfig struct {
	ProjectDir      string
	Enabled         bool
	PostCreateHooks []config.PostCreateHook
}

// NewService creates a new worktree service.
func NewService(cfg ServiceConfig, queries *db.Queries, conn *sql.DB, checkpoints checkpoint.Service) (Service, error) {
	worktreeDir := filepath.Join(cfg.ProjectDir, ".crush", "worktrees")

	if cfg.Enabled {
		if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
			return nil, fmt.Errorf("create worktrees dir: %w", err)
		}
	}

	return &service{
		Broker:      pubsub.NewBroker[Worktree](),
		queries:     queries,
		conn:        conn,
		checkpoints: checkpoints,
		projectDir:  cfg.ProjectDir,
		worktreeDir: worktreeDir,
		hooks:       cfg.PostCreateHooks,
		enabled:     cfg.Enabled,
	}, nil
}

func (s *service) IsEnabled() bool {
	return s.enabled
}

func (s *service) WorktreesDir() string {
	return s.worktreeDir
}

func (s *service) Create(ctx context.Context, sessionID string, name string, fromSnapshotID string) (*Worktree, error) {
	if !s.enabled {
		return nil, ErrWorktreesDisabled
	}

	// Generate name if not provided.
	if name == "" {
		name = s.GenerateName("")
	}

	// Validate name.
	if !isValidWorktreeName(name) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidName, name)
	}

	// Check if worktree already exists.
	_, err := s.GetByName(ctx, sessionID, name)
	if err == nil {
		return nil, ErrWorktreeExists
	}
	if !errors.Is(err, ErrWorktreeNotFound) {
		return nil, err
	}

	// Create worktree directory.
	worktreePath := filepath.Join(s.worktreeDir, name)
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		return nil, fmt.Errorf("create worktree dir: %w", err)
	}

	// If we have a snapshot, restore it to the worktree.
	if fromSnapshotID != "" && s.checkpoints != nil && s.checkpoints.IsEnabled() {
		if err := s.checkpoints.RestoreSnapshot(ctx, fromSnapshotID, worktreePath); err != nil {
			// Clean up on failure.
			os.RemoveAll(worktreePath)
			return nil, fmt.Errorf("restore snapshot to worktree: %w", err)
		}
	} else {
		// Copy current project state to worktree.
		if err := copyDir(s.projectDir, worktreePath); err != nil {
			os.RemoveAll(worktreePath)
			return nil, fmt.Errorf("copy project to worktree: %w", err)
		}
	}

	// Deactivate other worktrees for this session.
	if err := s.queries.DeactivateSessionWorktrees(ctx, sessionID); err != nil {
		slog.Debug("Failed to deactivate session worktrees", "error", err)
	}

	// Create database record.
	id := uuid.New().String()
	now := time.Now()

	worktree := &Worktree{
		ID:             id,
		SessionID:      sessionID,
		Name:           name,
		Path:           worktreePath,
		BaseSnapshotID: fromSnapshotID,
		IsActive:       true,
		CreatedAt:      now,
	}

	if err := s.queries.CreateWorktree(ctx, db.CreateWorktreeParams{
		ID:             id,
		SessionID:      sessionID,
		Name:           name,
		Path:           worktreePath,
		BaseSnapshotID: toNullString(fromSnapshotID),
		IsActive:       1,
		CreatedAt:      now.UnixMilli(),
	}); err != nil {
		os.RemoveAll(worktreePath)
		return nil, fmt.Errorf("create worktree record: %w", err)
	}

	// Run post-create hooks.
	if err := s.RunPostCreateHooks(ctx, worktreePath); err != nil {
		slog.Debug("Post-create hooks failed", "error", err)
	}

	s.Publish(pubsub.CreatedEvent, *worktree)

	return worktree, nil
}

func (s *service) Switch(ctx context.Context, sessionID string, worktreeID string) error {
	if !s.enabled {
		return ErrWorktreesDisabled
	}

	worktree, err := s.Get(ctx, worktreeID)
	if err != nil {
		return err
	}

	// Verify worktree belongs to session.
	if worktree.SessionID != sessionID {
		return ErrWorktreeNotFound
	}

	// Deactivate all worktrees for session.
	if err := s.queries.DeactivateSessionWorktrees(ctx, sessionID); err != nil {
		return fmt.Errorf("deactivate worktrees: %w", err)
	}

	// Activate this worktree.
	if err := s.queries.SetWorktreeActive(ctx, db.SetWorktreeActiveParams{
		IsActive: 1,
		ID:       worktreeID,
	}); err != nil {
		return fmt.Errorf("activate worktree: %w", err)
	}

	worktree.IsActive = true
	s.Publish(pubsub.UpdatedEvent, *worktree)

	return nil
}

func (s *service) Delete(ctx context.Context, worktreeID string) error {
	if !s.enabled {
		return ErrWorktreesDisabled
	}

	worktree, err := s.Get(ctx, worktreeID)
	if err != nil {
		return err
	}

	// Remove filesystem.
	if err := os.RemoveAll(worktree.Path); err != nil {
		slog.Debug("Failed to remove worktree directory", "error", err, "path", worktree.Path)
	}

	// Delete database record.
	if err := s.queries.DeleteWorktree(ctx, worktreeID); err != nil {
		return fmt.Errorf("delete worktree record: %w", err)
	}

	s.Publish(pubsub.DeletedEvent, *worktree)

	return nil
}

func (s *service) Get(ctx context.Context, worktreeID string) (*Worktree, error) {
	if !s.enabled {
		return nil, ErrWorktreesDisabled
	}

	row, err := s.queries.GetWorktree(ctx, worktreeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWorktreeNotFound
		}
		return nil, err
	}

	return dbRowToWorktree(row), nil
}

func (s *service) GetByName(ctx context.Context, sessionID string, name string) (*Worktree, error) {
	if !s.enabled {
		return nil, ErrWorktreesDisabled
	}

	row, err := s.queries.GetWorktreeByName(ctx, db.GetWorktreeByNameParams{
		SessionID: sessionID,
		Name:      name,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWorktreeNotFound
		}
		return nil, err
	}

	return dbRowToWorktree(row), nil
}

func (s *service) GetActive(ctx context.Context, sessionID string) (*Worktree, error) {
	if !s.enabled {
		return nil, ErrWorktreesDisabled
	}

	row, err := s.queries.GetActiveWorktree(ctx, sessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoActiveWorktree
		}
		return nil, err
	}

	return dbRowToWorktree(row), nil
}

func (s *service) List(ctx context.Context, sessionID string) ([]*Worktree, error) {
	if !s.enabled {
		return nil, nil
	}

	// If no sessionID provided, list all worktrees.
	if sessionID == "" {
		return s.ListAll(ctx)
	}

	rows, err := s.queries.ListWorktrees(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	worktrees := make([]*Worktree, len(rows))
	for i, row := range rows {
		worktrees[i] = dbRowToWorktree(row)
	}

	return worktrees, nil
}

func (s *service) ListAll(ctx context.Context) ([]*Worktree, error) {
	if !s.enabled {
		return nil, nil
	}

	rows, err := s.queries.ListAllWorktrees(ctx)
	if err != nil {
		return nil, err
	}

	worktrees := make([]*Worktree, len(rows))
	for i, row := range rows {
		worktrees[i] = dbRowToWorktree(row)
	}

	return worktrees, nil
}

// GenerateName generates a worktree name from a description.
// Uses conventional-commit style: feat/add-something, fix/issue-123
func (s *service) GenerateName(description string) string {
	if description == "" {
		// Generate a timestamp-based name.
		return fmt.Sprintf("worktree-%d", time.Now().Unix())
	}

	// Clean and convert to slug.
	name := strings.ToLower(description)
	name = regexp.MustCompile(`[^a-z0-9\s-]`).ReplaceAllString(name, "")
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, "-")
	name = regexp.MustCompile(`-+`).ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")

	// Limit length.
	if len(name) > 40 {
		name = name[:40]
		name = strings.TrimSuffix(name, "-")
	}

	if name == "" {
		name = fmt.Sprintf("worktree-%d", time.Now().Unix())
	}

	return name
}

func (s *service) RunPostCreateHooks(ctx context.Context, worktreePath string) error {
	if len(s.hooks) == 0 {
		return nil
	}

	var lastErr error
	for _, hook := range s.hooks {
		// Check if the trigger file exists.
		triggerPath := filepath.Join(worktreePath, hook.IfExists)
		if _, err := os.Stat(triggerPath); os.IsNotExist(err) {
			continue
		}

		// Run the command.
		slog.Debug("Running post-create hook", "command", hook.Run, "trigger", hook.IfExists)

		cmd := exec.CommandContext(ctx, "sh", "-c", hook.Run)
		cmd.Dir = worktreePath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			slog.Warn("Post-create hook failed", "command", hook.Run, "error", err)
			lastErr = err
		}
	}

	return lastErr
}

func (s *service) ValidateState(ctx context.Context) error {
	if !s.enabled {
		return nil
	}

	// Get all worktrees from database.
	dbWorktrees, err := s.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}

	dbPaths := make(map[string]*Worktree)
	for _, wt := range dbWorktrees {
		dbPaths[wt.Path] = wt
	}

	// Scan filesystem for worktrees.
	entries, err := os.ReadDir(s.worktreeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read worktrees dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		fsPath := filepath.Join(s.worktreeDir, entry.Name())

		if _, inDB := dbPaths[fsPath]; !inDB {
			slog.Warn("Orphan worktree found (exists on disk but not in database)",
				"path", fsPath,
				"name", entry.Name())
		}
	}

	// Check for stale DB records.
	for path, wt := range dbPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			slog.Warn("Stale worktree record (exists in database but not on disk)",
				"id", wt.ID,
				"name", wt.Name,
				"path", path)
		}
	}

	return nil
}

func dbRowToWorktree(row any) *Worktree {
	switch r := row.(type) {
	case db.Worktree:
		return &Worktree{
			ID:             r.ID,
			SessionID:      r.SessionID,
			Name:           r.Name,
			Path:           r.Path,
			BaseSnapshotID: fromNullString(r.BaseSnapshotID),
			IsActive:       r.IsActive != 0,
			CreatedAt:      time.UnixMilli(r.CreatedAt),
		}
	case *db.Worktree:
		return &Worktree{
			ID:             r.ID,
			SessionID:      r.SessionID,
			Name:           r.Name,
			Path:           r.Path,
			BaseSnapshotID: fromNullString(r.BaseSnapshotID),
			IsActive:       r.IsActive != 0,
			CreatedAt:      time.UnixMilli(r.CreatedAt),
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

// isValidWorktreeName checks if a worktree name is valid.
func isValidWorktreeName(name string) bool {
	if name == "" {
		return false
	}
	if len(name) > 100 {
		return false
	}
	// Only allow alphanumeric, hyphens, underscores, and forward slashes (for feat/xxx style).
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9][a-zA-Z0-9/_-]*$`, name)
	return matched
}

// copyDir copies a directory recursively, excluding .git and .crush.
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()

		// Skip .git and .crush.
		if name == ".git" || name == ".crush" {
			continue
		}

		// Skip common large directories.
		if name == "node_modules" || name == "vendor" || name == ".venv" || name == "venv" {
			continue
		}

		srcPath := filepath.Join(src, name)
		dstPath := filepath.Join(dst, name)

		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				return err
			}
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file.
func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Handle symlinks.
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	}

	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, content, info.Mode())
}
