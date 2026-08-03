package config

import (
	"os"

	"github.com/charmbracelet/crush/internal/fsext"
)

// atomicWriteFile writes data to a file atomically and durably: temp file in
// the same directory, fsync, rename into place, and a parent-directory fsync.
// Concurrent readers never observe a partially-written file. See
// fsext.AtomicWriteFile for the full rationale, including the Windows
// rename-retry loop.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	return fsext.AtomicWriteFile(path, data, perm)
}
