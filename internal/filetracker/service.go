package filetracker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/session"
)

type Service interface {
	RecordRead(ctx context.Context, sessionID, path string, snapshot Snapshot) error
	RecordIncludedInContext(ctx context.Context, sessionID, path string, snapshot Snapshot) error
	ChangedSinceRead(ctx context.Context, sessionID, path string) (bool, error)
	InCurrentContext(ctx context.Context, sessionID, path string) (bool, error)
	ListReadFiles(ctx context.Context, sessionID string) ([]string, error)
}

type service struct {
	q          *db.Queries
	sessions   session.Service
	workingDir string
}

func NewService(q *db.Queries, sessions session.Service, workingDir string) Service {
	absWorkingDir := workingDir
	if absWorkingDir == "" {
		absWorkingDir, _ = os.Getwd()
	}
	absWorkingDir = filepath.Clean(absWorkingDir)
	return &service{q: q, sessions: sessions, workingDir: absWorkingDir}
}

func (s *service) RecordRead(ctx context.Context, sessionID, path string, snapshot Snapshot) error {
	canonicalPath := s.canonicalPath(path)
	return s.q.RecordFileRead(ctx, db.RecordFileReadParams{
		SessionID:       sessionID,
		Path:            canonicalPath,
		LastReadMtimeNs: snapshot.ModTimeNanos,
		LastReadSize:    snapshot.SizeBytes,
	})
}

func (s *service) RecordIncludedInContext(ctx context.Context, sessionID, path string, snapshot Snapshot) error {
	sess, err := s.sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("getting session for inclusion: %w", err)
	}
	canonicalPath := s.canonicalPath(path)
	return s.q.RecordFileIncludedInContext(ctx, db.RecordFileIncludedInContextParams{
		SessionID:           sessionID,
		Path:                canonicalPath,
		LastIncludedMtimeNs: snapshot.ModTimeNanos,
		LastIncludedSize:    snapshot.SizeBytes,
		LastIncludedEpoch:   sess.ContextEpoch,
	})
}

func (s *service) ChangedSinceRead(ctx context.Context, sessionID, path string) (bool, error) {
	record, ok, err := s.getRecord(ctx, sessionID, path)
	if err != nil {
		return false, err
	}
	if !ok || record.LastReadAt == 0 {
		return true, nil
	}

	current, err := SnapshotFromPath(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, err
	}

	changed := current.ModTimeNanos != record.LastReadMtimeNs ||
		current.SizeBytes != record.LastReadSize
	return changed, nil
}

func (s *service) InCurrentContext(ctx context.Context, sessionID, path string) (bool, error) {
	record, ok, err := s.getRecord(ctx, sessionID, path)
	if err != nil {
		return false, err
	}
	if !ok || record.LastIncludedAt == 0 {
		return false, nil
	}

	sess, err := s.sessions.Get(ctx, sessionID)
	if err != nil {
		return false, fmt.Errorf("getting session for context check: %w", err)
	}
	if record.LastIncludedEpoch != sess.ContextEpoch {
		return false, nil
	}

	current, err := SnapshotFromPath(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	fresh := current.ModTimeNanos == record.LastIncludedMtimeNs &&
		current.SizeBytes == record.LastIncludedSize
	return fresh, nil
}

func (s *service) ListReadFiles(ctx context.Context, sessionID string) ([]string, error) {
	readFiles, err := s.q.ListSessionReadFiles(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("listing read files: %w", err)
	}

	paths := make([]string, 0, len(readFiles))
	for _, rf := range readFiles {
		paths = append(paths, s.expandPath(rf.Path))
	}
	return paths, nil
}

func (s *service) getRecord(ctx context.Context, sessionID, path string) (db.ReadFile, bool, error) {
	record, err := s.q.GetFileRead(ctx, db.GetFileReadParams{
		SessionID: sessionID,
		Path:      s.canonicalPath(path),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.ReadFile{}, false, nil
		}
		return db.ReadFile{}, false, err
	}
	return record, true, nil
}

func (s *service) canonicalPath(path string) string {
	absPath := path
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(s.workingDir, absPath)
	}
	absPath = filepath.Clean(absPath)

	relPath, err := filepath.Rel(s.workingDir, absPath)
	if err != nil {
		return absPath
	}
	if relPath == "." {
		return relPath
	}
	if relPath == ".." {
		return absPath
	}
	if strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return absPath
	}
	return relPath
}

func (s *service) expandPath(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(s.workingDir, path))
}
