// Package filetracker provides functionality to track file reads in sessions.
package filetracker

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/crush/internal/db"
)

// Service defines the interface for tracking file reads in sessions.
type Service interface {
	// RecordRead records when a file was read.
	RecordRead(ctx context.Context, sessionID, path string)

	// LastReadTime returns when a file was last read.
	// Returns zero time if never read.
	LastReadTime(ctx context.Context, sessionID, path string) time.Time
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
