// Package lock provides cross-process advisory file locking.
//
// File acquires an exclusive lock on the file at path, blocking until
// the context is cancelled (or its deadline elapses). TryFile does the
// same but returns ErrContended immediately if the lock is already
// held. In both cases the returned release function drops the lock and
// closes the underlying file descriptor.
//
// The lock is released automatically by the kernel on process
// termination (including crash), so no stale-lock recovery is needed.
//
// The lock file at path is created if it does not exist. It is never
// unlinked — flock is keyed by inode, not path, and unlinking could
// create a window where two processes lock different inodes at the
// same path.
//
// This is the canonical file-locking helper for Crush. Callers should
// prefer it over rolling their own platform-specific code.
package lock

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// ErrContended is returned by TryFile when the lock is already held by
// another process.
var ErrContended = errors.New("file lock is held by another process")

// File acquires an exclusive advisory lock on the file at path, blocking
// until the lock is acquired or ctx is cancelled. It returns a release
// function that drops the lock and closes the underlying file descriptor.
//
// Pass a context with a deadline (e.g. context.WithTimeout) to bound the
// wait. Pass context.Background() to block indefinitely.
func File(ctx context.Context, path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file %q: %w", path, err)
	}

	release, err := lockFile(ctx, f)
	if err != nil {
		f.Close()
		return nil, err
	}

	return func() {
		release()
		f.Close()
	}, nil
}

// TryFile is like File but returns ErrContended immediately if the lock
// is already held by another process. Use this when you want to fail
// fast rather than wait.
func TryFile(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file %q: %w", path, err)
	}

	release, err := tryLockFile(f)
	if err != nil {
		f.Close()
		return nil, err
	}

	return func() {
		release()
		f.Close()
	}, nil
}
