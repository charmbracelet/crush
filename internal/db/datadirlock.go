package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/charmbracelet/crush/internal/version"
)

// ErrDataDirLocked is returned by Connect when the data directory is
// already in use by another crush process.
var ErrDataDirLocked = errors.New("data directory already in use by another crush process")

// dataDirLockFile is the name of the lock file inside the data
// directory. It lives next to crush.db so users can `ls` and find it.
const dataDirLockFile = "crush.lock"

// dataDirOwnerInfo is the JSON payload written into the lock file by
// the process that currently owns it. It is purely informational; the
// authoritative state of ownership is the operating system flock on
// the file descriptor.
type dataDirOwnerInfo struct {
	PID       int    `json:"pid"`
	Version   string `json:"version,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
}

// dataDirLock represents an acquired exclusive lock on a data
// directory. release closes the underlying file descriptor which the
// kernel uses to drop the OS-level lock.
type dataDirLock struct {
	release func()
}

// acquireDataDirLock takes an exclusive non-blocking lock on
// {dataDir}/crush.lock. If the lock is already held by another
// process, it returns ErrDataDirLocked wrapped with a diagnostic that
// includes whatever owner info that process wrote.
//
// Acquisition is skipped (returning a no-op lock) when
// CRUSH_SKIP_DATADIR_LOCK is set to a truthy value. This is intended
// as an escape hatch for hostile filesystems that do not implement
// advisory locking; it should not be used in normal operation.
func acquireDataDirLock(dataDir string) (*dataDirLock, error) {
	if skipDataDirLock() {
		return &dataDirLock{release: func() {}}, nil
	}

	path := filepath.Join(dataDir, dataDirLockFile)
	release, err := tryFileLock(path)
	if err != nil {
		if errors.Is(err, errLockContended) {
			return nil, contendedLockError(dataDir, path)
		}
		return nil, fmt.Errorf("failed to lock data directory %q: %w", dataDir, err)
	}

	// Record ownership metadata so a contending process can identify
	// us. Failures here are non-fatal: the OS-level lock is what
	// actually guarantees mutual exclusion, and a missing/partial JSON
	// payload only degrades diagnostics.
	if err := writeOwnerInfo(path); err != nil {
		// Best-effort; log via stderr only when running in a debug
		// context would be invasive here, so we silently swallow.
		_ = err
	}

	return &dataDirLock{release: release}, nil
}

// skipDataDirLock reports whether the data-dir lock should be bypassed.
func skipDataDirLock() bool {
	v, _ := strconv.ParseBool(os.Getenv("CRUSH_SKIP_DATADIR_LOCK"))
	return v
}

// writeOwnerInfo truncates and rewrites the lock file with the current
// process's identifying information. It is called only after the lock
// is held.
func writeOwnerInfo(path string) error {
	info := dataDirOwnerInfo{
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

// readOwnerInfo returns the lock file's recorded owner, if it parses.
// A missing or malformed file yields an empty struct and no error;
// the caller decides what to surface to the user.
func readOwnerInfo(path string) dataDirOwnerInfo {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return dataDirOwnerInfo{}
	}
	var info dataDirOwnerInfo
	_ = json.Unmarshal(raw, &info)
	return info
}

// contendedLockError builds a wrapped ErrDataDirLocked annotated with
// whatever owner metadata is currently in the lock file.
func contendedLockError(dataDir, lockPath string) error {
	info := readOwnerInfo(lockPath)
	details := ""
	switch {
	case info.PID != 0 && info.StartedAt != "":
		details = fmt.Sprintf(" (owner pid=%d version=%s started_at=%s)",
			info.PID, info.Version, info.StartedAt)
	case info.PID != 0:
		details = fmt.Sprintf(" (owner pid=%d)", info.PID)
	}
	return fmt.Errorf("%w: %s%s", ErrDataDirLocked, dataDir, details)
}
