package critic

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
)

// fileSnapshot holds the original content and permissions of a file.
type fileSnapshot struct {
	Content []byte
	Mode    fs.FileMode
	Exists  bool
}

// SnapshotStore captures file contents before mutation so that Rollback can
// restore pristine state. It also records file permissions and handles files
// that did not exist before capture (deleted on rollback).
type SnapshotStore struct {
	stash       map[string]fileSnapshot
	maxFileSize int64
}

// NewSnapshotStore creates an empty snapshot store.
func NewSnapshotStore() *SnapshotStore {
	return &SnapshotStore{
		stash: make(map[string]fileSnapshot),
	}
}

// SetMaxFileSize configures the maximum file size to snapshot.
// Files larger than this are skipped. A value of 0 disables the limit.
func (ss *SnapshotStore) SetMaxFileSize(size int64) {
	ss.maxFileSize = size
}

// Capture reads the current content and mode of each path into memory.
// If a path does not exist, it stores a sentinel so that Rollback will remove
// the file if it is created later.
func (ss *SnapshotStore) Capture(paths []string) error {
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				ss.stash[p] = fileSnapshot{Exists: false}
				continue
			}
			return fmt.Errorf("snapshot stat %s: %w", p, err)
		}

		if ss.maxFileSize > 0 && info.Size() > ss.maxFileSize {
			slog.Warn("Skipping large file in critic snapshot", "path", p, "size", info.Size(), "max", ss.maxFileSize)
			ss.stash[p] = fileSnapshot{Exists: true, Mode: info.Mode()}
			continue
		}

		b, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("snapshot read %s: %w", p, err)
		}

		ss.stash[p] = fileSnapshot{
			Content: b,
			Mode:    info.Mode(),
			Exists:  true,
		}
	}
	return nil
}

// Rollback restores all captured files to their original state.
// Files that did not exist at capture time are removed.
// Files that were only marked as pre-existing (no content captured) are skipped
// so we don't accidentally overwrite them with empty data.
func (ss *SnapshotStore) Rollback() error {
	for p, snap := range ss.stash {
		if !snap.Exists {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("rollback remove %s: %w", p, err)
			}
			continue
		}

		// Skip files that were only marked pre-existing but never captured.
		// Their Content is nil/empty and we don't know their original state.
		if len(snap.Content) == 0 && snap.Mode == 0 {
			continue
		}

		if err := os.WriteFile(p, snap.Content, snap.Mode); err != nil {
			return fmt.Errorf("rollback write %s: %w", p, err)
		}
		if err := os.Chmod(p, snap.Mode); err != nil {
			return fmt.Errorf("rollback chmod %s: %w", p, err)
		}
	}
	return nil
}

// Clear drops all captured data to free memory.
func (ss *SnapshotStore) Clear() {
	ss.stash = make(map[string]fileSnapshot)
}

// Paths returns the list of captured paths.
func (ss *SnapshotStore) Paths() []string {
	paths := make([]string, 0, len(ss.stash))
	for p := range ss.stash {
		paths = append(paths, p)
	}
	return paths
}

// Changed returns the paths whose current content differs from the snapshot.
// It also returns a map of path -> current content for changed files.
func (ss *SnapshotStore) Changed() (changed []string, after map[string][]byte, err error) {
	after = make(map[string][]byte)
	for p, snap := range ss.stash {
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			if os.IsNotExist(readErr) && snap.Exists {
				// File was deleted.
				changed = append(changed, p)
				after[p] = nil
				continue
			}
			return nil, nil, fmt.Errorf("changed check %s: %w", p, readErr)
		}

		if !snap.Exists {
			// File was created.
			changed = append(changed, p)
			after[p] = b
			continue
		}

		if string(b) != string(snap.Content) {
			changed = append(changed, p)
			after[p] = b
		}
	}
	return changed, after, nil
}
