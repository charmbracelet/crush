package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/crush/internal/lock"
)

// AcquireSessionWriter claims a transcript until the pooled connection closes.
// Ownership persists between turns, including while a local TUI is idle.
func AcquireSessionWriter(ctx context.Context, conn *sql.DB, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	poolMu.Lock()
	defer poolMu.Unlock()
	for path, entry := range pool {
		if entry.db != conn {
			continue
		}
		if _, ok := entry.writers[id]; ok {
			return nil
		}
		directory := filepath.Join(filepath.Dir(path), "session-writers")
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return err
		}
		name := fmt.Sprintf("%x.lock", sha256.Sum256([]byte(id)))
		release, err := lock.TryFile(filepath.Join(directory, name))
		if err != nil {
			if errors.Is(err, lock.ErrContended) {
				return fmt.Errorf("session is owned by another Crush process; send input to the owning session: %w", err)
			}
			return fmt.Errorf("acquire session writer: %w", err)
		}
		if entry.writers == nil {
			entry.writers = make(map[string]func())
		}
		entry.writers[id] = release
		return nil
	}
	return errors.New("session database connection is not pooled")
}

func (e *connEntry) releaseWriters() {
	for id, release := range e.writers {
		release()
		delete(e.writers, id)
	}
}
