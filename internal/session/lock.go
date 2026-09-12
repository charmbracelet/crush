package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/crush/internal/lock"
	"github.com/charmbracelet/crush/internal/version"
)

var ErrSessionLocked = errors.New("session already in use by another crush process")

type Lock struct {
	release func()
}

func (l *Lock) Release() {
	if l == nil || l.release == nil {
		return
	}
	l.release()
}

type lockOwnerInfo struct {
	PID       int    `json:"pid"`
	Version   string `json:"version,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
}

func AcquireLock(dataDir, sessionID string) (*Lock, error) {
	dir := filepath.Join(dataDir, "locks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create session lock directory: %w", err)
	}

	path := filepath.Join(dir, "session-"+sessionID+".lock")
	release, err := lock.TryFile(path)
	if err != nil {
		if errors.Is(err, lock.ErrContended) {
			return nil, contendedError(sessionID, path)
		}
		return nil, fmt.Errorf("failed to lock session %q: %w", sessionID, err)
	}

	if err := writeLockOwnerInfo(path); err != nil {
		slog.Debug("Failed to write session lock owner info", "path", path, "error", err)
	}

	return &Lock{release: release}, nil
}

func writeLockOwnerInfo(path string) error {
	info := lockOwnerInfo{
		PID:       os.Getpid(),
		Version:   version.Version,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
	}
	payload, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(path, payload, 0o600)
}

func readLockOwnerInfo(path string) lockOwnerInfo {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return lockOwnerInfo{}
	}
	var info lockOwnerInfo
	_ = json.Unmarshal(raw, &info)
	return info
}

func contendedError(sessionID, path string) error {
	info := readLockOwnerInfo(path)
	if info.PID != 0 {
		return fmt.Errorf("%w: session %q (owner pid=%d)", ErrSessionLocked, sessionID, info.PID)
	}
	return fmt.Errorf("%w: session %q", ErrSessionLocked, sessionID)
}
