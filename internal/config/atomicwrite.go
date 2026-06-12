package config

import (
	"os"
	"path/filepath"
	"time"
)

const (
	renameRetryDeadline       = 500 * time.Millisecond
	renameRetryInitialBackoff = time.Millisecond
	renameRetryMaxBackoff     = 50 * time.Millisecond
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

func renameFile(tmp, path string) error {
	deadline := time.Now().Add(renameRetryDeadline)
	backoff := renameRetryInitialBackoff

	for {
		err := os.Rename(tmp, path)
		if err == nil || !isRetryableRenameError(err) {
			return err
		}

		sleep := min(backoff, time.Until(deadline))
		if sleep <= 0 {
			return err
		}

		time.Sleep(sleep)
		backoff = min(backoff*2, renameRetryMaxBackoff)
	}
}
