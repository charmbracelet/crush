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
// It writes a unique temp file in the same directory, fsyncs it, renames it
// into place so readers never see a torn file, then fsyncs the directory. A
// directory fsync failure is only logged: the data is already visible.
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

// renameFile renames tmp over path. On Windows the rename fails while another
// handle has the destination open; other platforms are a plain os.Rename.
// Retries are bounded by renameRetryBudget so contention cannot hang writes.
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
