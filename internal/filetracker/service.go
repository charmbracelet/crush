// Package filetracker provides functionality to track file reads in sessions.
package filetracker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/crush/internal/db"
)

// Service defines the interface for tracking file reads and writes in sessions.
type Service interface {
	// RecordRead records when a file was read.
	RecordRead(ctx context.Context, sessionID, path string)

	// LastReadTime returns when a file was last read.
	// Returns zero time if never read.
	LastReadTime(ctx context.Context, sessionID, path string) time.Time

	// ListReadFiles returns the paths of all files read in a session.
	ListReadFiles(ctx context.Context, sessionID string) ([]string, error)

	// RecordWrite records when a file was written.
	RecordWrite(ctx context.Context, sessionID, path string)

	// ListWrittenFiles returns the paths of all files written in a session.
	ListWrittenFiles(ctx context.Context, sessionID string) ([]string, error)
}

type service struct {
	q *db.Queries
}

// NewService creates a new file tracker service.
func NewService(q *db.Queries) Service {
	return &service{q: q}
}

// RecordRead records when a file was read.
func (s *service) RecordRead(ctx context.Context, sessionID, path string) {
	if err := s.q.RecordFileRead(ctx, db.RecordFileReadParams{
		SessionID: sessionID,
		Path:      relpath(path),
	}); err != nil {
		slog.Error("Error recording file read", "error", err, "file", path)
	}
}

// LastReadTime returns when a file was last read.
// Returns zero time if never read.
func (s *service) LastReadTime(ctx context.Context, sessionID, path string) time.Time {
	readFile, err := s.q.GetFileRead(ctx, db.GetFileReadParams{
		SessionID: sessionID,
		Path:      relpath(path),
	})
	if err != nil {
		return time.Time{}
	}

	return time.Unix(readFile.ReadAt, 0)
}

func relpath(path string) string {
	path = filepath.Clean(path)
	basepath, err := os.Getwd()
	if err != nil {
		slog.Warn("Error getting basepath", "error", err)
		return path
	}
	relpath, err := filepath.Rel(basepath, path)
	if err != nil {
		slog.Warn("Error getting relpath", "error", err)
		return path
	}
	return relpath
}

// ListReadFiles returns the paths of all files read in a session.
func (s *service) ListReadFiles(ctx context.Context, sessionID string) ([]string, error) {
	readFiles, err := s.q.ListSessionReadFiles(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("listing read files: %w", err)
	}

	basepath, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}

	paths := make([]string, 0, len(readFiles))
	for _, rf := range readFiles {
		paths = append(paths, filepath.Join(basepath, rf.Path))
	}
	return paths, nil
}

// RecordWrite records when a file was written.
func (s *service) RecordWrite(ctx context.Context, sessionID, path string) {
	if err := s.q.RecordFileWrite(ctx, db.RecordFileWriteParams{
		SessionID: sessionID,
		Path:      relpath(path),
	}); err != nil {
		slog.Error("Error recording file write", "error", err, "file", path)
	}
}

// ListWrittenFiles returns the paths of all files written in a session.
func (s *service) ListWrittenFiles(ctx context.Context, sessionID string) ([]string, error) {
	writtenFiles, err := s.q.ListSessionWrittenFiles(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("listing written files: %w", err)
	}

	basepath, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}

	paths := make([]string, 0, len(writtenFiles))
	for _, wf := range writtenFiles {
		paths = append(paths, filepath.Join(basepath, wf.Path))
	}
	return paths, nil
}
