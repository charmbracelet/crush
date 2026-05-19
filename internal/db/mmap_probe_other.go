//go:build !unix

package db

import (
	"os"
	"path/filepath"
)

// mmapAvailable always returns true on non-Unix platforms where
// sandbox-restricted mmap is not a concern.
func mmapAvailable(_ string) bool {
	return true
}

func probeMmapDir(dbPath string) string {
	dir := filepath.Dir(dbPath)
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	return os.TempDir()
}
