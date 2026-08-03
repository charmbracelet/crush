package fsext

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// renameRetryBudget bounds how long renameFile keeps retrying transient
// failures before giving up and returning the error.
const renameRetryBudget = 2 * time.Second

// AtomicWriteFile writes data to path atomically and durably.
//
// Atomicity: it writes to a unique temporary file in the same directory and
// renames it into place, so concurrent readers observe either the whole old
// file or the whole new one, never a partial write. This is what prevents a
// prompt layer reading MEMORY.md at launch from seeing a torn file while the
// agent rewrites it.
//
// Durability: the temp file is fsynced before close and the parent directory
// is fsynced after the rename. On ext4 with the default data=ordered mode the
// rename can become durable while the file's data blocks are still in page
// cache, leaving a zero-length file after a power loss; the file fsync closes
// that window, and the directory fsync closes the opposite one where a crash
// loses the rename itself. A directory fsync failure does not fail the write:
// the data is already renamed into place and visible, so it is only logged.
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

// renameFile renames tmp over path. On Windows the rename fails with
// ERROR_ACCESS_DENIED while another handle has the destination open, which is
// exactly the case AtomicWriteFile exists to serve: the prompt layer reads
// MEMORY.md at launch while the agent may be rewriting it, so without a retry
// the write surfaces as a failed memory_write ("Access is denied") even
// though nothing is wrong. On other platforms isTransientRenameError is
// always false and this is a plain os.Rename. Bounded by renameRetryBudget so
// a contended rename cannot become an unbounded wait.
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
