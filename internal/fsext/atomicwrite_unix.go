//go:build !windows

package fsext

import "os"

// isTransientRenameError reports whether err is a rename failure that
// can resolve on its own. Only Windows has such failures.
func isTransientRenameError(error) bool { return false }

// syncDir flushes the directory entry of a completed rename to stable
// storage. On POSIX the directory is a file, so this is an open + fsync.
func syncDir(dir string) error {
	// dir is filepath.Dir of a trusted internal path (see AtomicWriteFile).
	// codeql[go/path-injection]
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
