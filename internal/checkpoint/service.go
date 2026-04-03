package checkpoint

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/google/uuid"
)

type Checkpoint struct {
	ID        string
	SessionID string
	MessageID string
	CreatedAt int64
}

type FileChange struct {
	Path       string
	OldContent string
	NewContent string
}

type Service interface {
	CreateCheckpoint(ctx context.Context, sessionID, messageID string) (Checkpoint, error)
	GetLatest(ctx context.Context, sessionID string) (Checkpoint, error)
	ListBySession(ctx context.Context, sessionID string) ([]Checkpoint, error)
	DiffFromCheckpoint(ctx context.Context, checkpointID string) ([]FileChange, error)
	RewindToCheckpoint(ctx context.Context, checkpointID string) ([]string, error)
}

type service struct {
	q          *db.Queries
	dbConn     *sql.DB
	history    history.Service
	workingDir string
}

func NewService(q *db.Queries, dbConn *sql.DB, hist history.Service, workingDir string) Service {
	return &service{q: q, dbConn: dbConn, history: hist, workingDir: workingDir}
}

func (s *service) CreateCheckpoint(ctx context.Context, sessionID, messageID string) (Checkpoint, error) {
	latestFiles, err := s.history.ListLatestSessionFiles(ctx, sessionID)
	if err != nil {
		return Checkpoint{}, fmt.Errorf("listing latest files: %w", err)
	}
	if len(latestFiles) == 0 {
		return Checkpoint{}, nil
	}

	cpID := uuid.New().String()
	dbCP, err := s.q.CreateCheckpoint(ctx, db.CreateCheckpointParams{
		ID: cpID, SessionID: sessionID, MessageID: messageID,
	})
	if err != nil {
		return Checkpoint{}, fmt.Errorf("creating checkpoint: %w", err)
	}

	for _, f := range latestFiles {
		if err := s.q.AddCheckpointFile(ctx, db.AddCheckpointFileParams{
			CheckpointID: cpID, FileID: f.ID,
		}); err != nil {
			slog.Warn("Failed to add file to checkpoint", "checkpoint_id", cpID, "file_id", f.ID, "error", err)
		}
	}

	slog.Info("Created checkpoint", "id", cpID, "session_id", sessionID, "files", len(latestFiles))
	return fromDB(dbCP), nil
}

func (s *service) GetLatest(ctx context.Context, sessionID string) (Checkpoint, error) {
	dbCP, err := s.q.GetLatestSessionCheckpoint(ctx, sessionID)
	if err != nil {
		return Checkpoint{}, err
	}
	return fromDB(dbCP), nil
}

func (s *service) ListBySession(ctx context.Context, sessionID string) ([]Checkpoint, error) {
	dbCPs, err := s.q.ListSessionCheckpoints(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	cps := make([]Checkpoint, len(dbCPs))
	for i, cp := range dbCPs {
		cps[i] = fromDB(cp)
	}
	return cps, nil
}

func (s *service) DiffFromCheckpoint(ctx context.Context, checkpointID string) ([]FileChange, error) {
	files, err := s.q.ListCheckpointFiles(ctx, checkpointID)
	if err != nil {
		return nil, fmt.Errorf("listing checkpoint files: %w", err)
	}
	var changes []FileChange
	for _, f := range files {
		absPath := f.Path
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(s.workingDir, absPath)
		}
		currentContent, err := os.ReadFile(absPath)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading file %s: %w", absPath, err)
		}
		current := string(currentContent)
		if current != f.Content {
			changes = append(changes, FileChange{Path: f.Path, OldContent: f.Content, NewContent: current})
		}
	}
	return changes, nil
}

func (s *service) RewindToCheckpoint(ctx context.Context, checkpointID string) ([]string, error) {
	files, err := s.q.ListCheckpointFiles(ctx, checkpointID)
	if err != nil {
		return nil, fmt.Errorf("listing checkpoint files: %w", err)
	}
	var restored []string
	for _, f := range files {
		absPath := f.Path
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(s.workingDir, absPath)
		}
		currentContent, readErr := os.ReadFile(absPath)
		if readErr == nil && string(currentContent) == f.Content {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return restored, fmt.Errorf("creating directory for %s: %w", absPath, err)
		}
		if err := os.WriteFile(absPath, []byte(f.Content), 0o644); err != nil {
			return restored, fmt.Errorf("restoring file %s: %w", absPath, err)
		}
		restored = append(restored, f.Path)
	}
	slog.Info("Rewound to checkpoint", "checkpoint_id", checkpointID, "files_restored", len(restored))
	return restored, nil
}

func fromDB(cp db.Checkpoint) Checkpoint {
	return Checkpoint{ID: cp.ID, SessionID: cp.SessionID, MessageID: cp.MessageID, CreatedAt: cp.CreatedAt}
}
