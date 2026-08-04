//go:build !windows

package fsext

import "os"

// isTransientRenameError reports whether err is a rename failure that
// can resolve on its own. Only Windows has such failures.
func isTransientRenameError(error) bool { return false }

// syncDir flushes the completed rename's directory entry to stable storage.
func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
