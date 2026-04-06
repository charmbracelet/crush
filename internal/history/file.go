package history

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

const (
	InitialVersion = 0
)

type File struct {
	ID        string
	SessionID string
	Path      string
	Content   string
	Version   int64
	CreatedAt int64
	UpdatedAt int64
}

// Service manages file versions and history for sessions.
type Service interface {
	pubsub.Subscriber[File]
	Create(ctx context.Context, sessionID, path, content string) (File, error)

	// CreateVersion creates a new version of a file.
	CreateVersion(ctx context.Context, sessionID, path, content string) (File, error)

	Get(ctx context.Context, id string) (File, error)
	GetByPathAndSession(ctx context.Context, path, sessionID string) (File, error)
	ListBySession(ctx context.Context, sessionID string) ([]File, error)
	ListLatestSessionFiles(ctx context.Context, sessionID string) ([]File, error)
	Delete(ctx context.Context, id string) error
	DeleteSessionFiles(ctx context.Context, sessionID string) error

	// RestoreToTimestamp restores all files in the session to their state
	// strictly before the given Unix timestamp. Files that have no version
	// before that timestamp (i.e. they were created by the agent after that
	// point) are deleted from disk.
	RestoreToTimestamp(ctx context.Context, sessionID string, timestamp int64) error

	// RestoreToLatest restores all files in the session to their most recent
	// recorded version. Used when performing a redo back to the head state.
	RestoreToLatest(ctx context.Context, sessionID string) error

	// CleanupAfterTimestamp removes file version records whose created_at is
	// >= the given timestamp. Call this during revert cleanup (when the user
	// sends a new prompt) to permanently discard the undone history.
	CleanupAfterTimestamp(ctx context.Context, sessionID string, timestamp int64) error
}

type service struct {
	*pubsub.Broker[File]
	db *sql.DB
	q  *db.Queries
}

func NewService(q *db.Queries, db *sql.DB) Service {
	return &service{
		Broker: pubsub.NewBroker[File](),
		q:      q,
		db:     db,
	}
}

func (s *service) Create(ctx context.Context, sessionID, path, content string) (File, error) {
	return s.createWithVersion(ctx, sessionID, path, content, InitialVersion)
}

// CreateVersion creates a new version of a file with auto-incremented version
// number. If no previous versions exist for the path, it creates the initial
// version. The provided content is stored as the new version.
func (s *service) CreateVersion(ctx context.Context, sessionID, path, content string) (File, error) {
	// Get the latest version for this path
	files, err := s.q.ListFilesByPath(ctx, path)
	if err != nil {
		return File{}, err
	}

	if len(files) == 0 {
		// No previous versions, create initial
		return s.Create(ctx, sessionID, path, content)
	}

	// Get the latest version
	latestFile := files[0] // Files are ordered by version DESC, created_at DESC
	nextVersion := latestFile.Version + 1

	return s.createWithVersion(ctx, sessionID, path, content, nextVersion)
}

func (s *service) createWithVersion(ctx context.Context, sessionID, path, content string, version int64) (File, error) {
	// Maximum number of retries for transaction conflicts
	const maxRetries = 3
	var file File
	var err error

	// Retry loop for transaction conflicts
	for attempt := range maxRetries {
		// Start a transaction
		tx, txErr := s.db.BeginTx(ctx, nil)
		if txErr != nil {
			return File{}, fmt.Errorf("failed to begin transaction: %w", txErr)
		}

		// Create a new queries instance with the transaction
		qtx := s.q.WithTx(tx)

		// Try to create the file within the transaction
		dbFile, txErr := qtx.CreateFile(ctx, db.CreateFileParams{
			ID:        uuid.New().String(),
			SessionID: sessionID,
			Path:      path,
			Content:   content,
			Version:   version,
		})
		if txErr != nil {
			// Rollback the transaction
			tx.Rollback()

			// Check if this is a uniqueness constraint violation
			if strings.Contains(txErr.Error(), "UNIQUE constraint failed") {
				if attempt < maxRetries-1 {
					// If we have retries left, increment version and try again
					version++
					continue
				}
			}
			return File{}, txErr
		}

		// Commit the transaction
		if txErr = tx.Commit(); txErr != nil {
			return File{}, fmt.Errorf("failed to commit transaction: %w", txErr)
		}

		file = s.fromDBItem(dbFile)
		s.Publish(pubsub.CreatedEvent, file)
		return file, nil
	}

	return file, err
}

func (s *service) Get(ctx context.Context, id string) (File, error) {
	dbFile, err := s.q.GetFile(ctx, id)
	if err != nil {
		return File{}, err
	}
	return s.fromDBItem(dbFile), nil
}

func (s *service) GetByPathAndSession(ctx context.Context, path, sessionID string) (File, error) {
	dbFile, err := s.q.GetFileByPathAndSession(ctx, db.GetFileByPathAndSessionParams{
		Path:      path,
		SessionID: sessionID,
	})
	if err != nil {
		return File{}, err
	}
	return s.fromDBItem(dbFile), nil
}

func (s *service) ListBySession(ctx context.Context, sessionID string) ([]File, error) {
	dbFiles, err := s.q.ListFilesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	files := make([]File, len(dbFiles))
	for i, dbFile := range dbFiles {
		files[i] = s.fromDBItem(dbFile)
	}
	return files, nil
}

func (s *service) ListLatestSessionFiles(ctx context.Context, sessionID string) ([]File, error) {
	dbFiles, err := s.q.ListLatestSessionFiles(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	files := make([]File, len(dbFiles))
	for i, dbFile := range dbFiles {
		files[i] = s.fromDBItem(dbFile)
	}
	return files, nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	file, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	err = s.q.DeleteFile(ctx, id)
	if err != nil {
		return err
	}
	s.Publish(pubsub.DeletedEvent, file)
	return nil
}

func (s *service) DeleteSessionFiles(ctx context.Context, sessionID string) error {
	files, err := s.ListBySession(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, file := range files {
		err = s.Delete(ctx, file.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

// RestoreToTimestamp restores all session files to their state before
// timestamp. Files that did not exist before that point are deleted from disk.
func (s *service) RestoreToTimestamp(ctx context.Context, sessionID string, timestamp int64) error {
	paths, err := s.q.ListDistinctFilePathsBySession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("listing file paths: %w", err)
	}
	for _, path := range paths {
		f, err := s.q.GetFileVersionBefore(ctx, db.GetFileVersionBeforeParams{
			SessionID: sessionID,
			Path:      path,
			CreatedAt: timestamp,
		})
		if errors.Is(err, sql.ErrNoRows) {
			// File was created after the revert point — remove from disk.
			if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("removing file %s: %w", path, removeErr)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("getting file version before timestamp for %s: %w", path, err)
		}
		if writeErr := os.WriteFile(path, []byte(f.Content), 0o644); writeErr != nil {
			return fmt.Errorf("restoring file %s: %w", path, writeErr)
		}
	}
	return nil
}

// RestoreToLatest restores all session files to their most recent version.
func (s *service) RestoreToLatest(ctx context.Context, sessionID string) error {
	paths, err := s.q.ListDistinctFilePathsBySession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("listing file paths: %w", err)
	}
	for _, path := range paths {
		f, err := s.q.GetLatestFileVersion(ctx, db.GetLatestFileVersionParams{
			SessionID: sessionID,
			Path:      path,
		})
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("getting latest file version for %s: %w", path, err)
		}
		if writeErr := os.WriteFile(path, []byte(f.Content), 0o644); writeErr != nil {
			return fmt.Errorf("restoring file %s: %w", path, writeErr)
		}
	}
	return nil
}

// CleanupAfterTimestamp removes file version records at or after the given
// timestamp from the session. Called when a new prompt is sent after an undo.
func (s *service) CleanupAfterTimestamp(ctx context.Context, sessionID string, timestamp int64) error {
	return s.q.DeleteFileVersionsAfterTimestamp(ctx, db.DeleteFileVersionsAfterTimestampParams{
		SessionID: sessionID,
		CreatedAt: timestamp,
	})
}

func (s *service) fromDBItem(item db.File) File {
	return File{
		ID:        item.ID,
		SessionID: item.SessionID,
		Path:      item.Path,
		Content:   item.Content,
		Version:   item.Version,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}
