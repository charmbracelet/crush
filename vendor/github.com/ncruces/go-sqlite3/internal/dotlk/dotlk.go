package dotlk

import (
	"errors"
	"io/fs"
	"os"
)

// LockShm creates a directory on disk to prevent SQLite
// from using this path for a shared memory file.
func LockShm(name string) error {
	err := os.Mkdir(name, 0777)
	if errors.Is(err, fs.ErrExist) {
		s, err := os.Lstat(name)
		if err == nil && s.IsDir() {
			return nil
		}
	}
	return err
}

// Unlock removes the lock or shared memory file.
func Unlock(name string) error {
	err := os.Remove(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
