package fsext

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// renameRetryBudget bounds how long renameFile retries transient failures.
const renameRetryBudget = 2 * time.Second

// AtomicWriteFile writes data to path atomically and durably.
//
// Temp file in the same directory, fsync, rename, then fsync the directory.
// The rename keeps readers from seeing a torn file (the prompt layer reads
// MEMORY.md while the agent rewrites it). Both fsyncs are needed: on ext4
// data=ordered the rename can land while the data blocks are still in page
// cache, leaving a zero-length file after power loss, and the directory fsync
// covers the opposite case where the crash loses the rename. A directory
// fsync failure is only logged -- the data is already visible.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	path = filepath.Clean(path)
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Chmod(perm); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := renameFile(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := syncDir(dir); err != nil {
		slog.Debug(
			"AtomicWriteFile: fsync of parent directory failed; write is already visible",
			"dir", dir, "err", err,
		)
	}
	return nil
}

// renameFile renames tmp over path. On Windows this fails with
// ERROR_ACCESS_DENIED while another handle holds the destination -- the
// prompt layer reading MEMORY.md at launch -- surfacing as a spurious
// memory_write "Access is denied" without a retry. Elsewhere
// isTransientRenameError is always false and this is a plain os.Rename.
// Bounded by renameRetryBudget so contention cannot hang the write.
func renameFile(tmp, path string) error {
	var slept time.Duration
	delay := time.Millisecond
	for {
		err := os.Rename(tmp, path)
		if err == nil || !isTransientRenameError(err) || slept >= renameRetryBudget {
			return err
		}
		time.Sleep(delay)
		slept += delay
		delay = min(delay*2, 50*time.Millisecond)
	}
}
