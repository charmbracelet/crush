package config

import (
	"os"
	"path/filepath"
	"time"
)

// atomicWriteFile writes data to a file atomically by writing to a unique
// temporary file in the same directory and renaming it into place. This
// prevents concurrent readers from observing a partially-written file.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
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
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := renameFile(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// renameRetryBudget bounds how long renameFile keeps retrying transient
// failures before giving up and returning the error.
const renameRetryBudget = 2 * time.Second

// renameFile renames tmp over path. On Windows the rename fails with
// ERROR_ACCESS_DENIED or ERROR_SHARING_VIOLATION while another process
// (antivirus, search indexer) or a concurrent reader briefly holds a
// handle on the destination, so transient failures are retried with
// backoff. On other platforms isTransientRenameError is always false
// and this is a plain os.Rename.
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
