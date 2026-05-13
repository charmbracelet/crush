//go:build unix

package db

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// mmapAvailable probes whether mmap with MAP_SHARED works in dir by
// creating a small temporary file, mapping it, and immediately
// unmapping. Returns true if the operation succeeds. SQLite's WAL
// mode requires mmap for the -shm shared-memory file; sandboxed
// environments (nsjail, bubblewrap, restrictive seccomp profiles)
// sometimes block this syscall.
func mmapAvailable(dir string) bool {
	f, err := os.CreateTemp(dir, ".crush-mmap-probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	defer os.Remove(name)

	// The file needs at least one page of content to map.
	if err := f.Truncate(int64(os.Getpagesize())); err != nil {
		f.Close()
		return false
	}

	fd := int(f.Fd())
	data, err := unix.Mmap(fd, 0, os.Getpagesize(), unix.PROT_READ, unix.MAP_SHARED)
	f.Close()
	if err != nil {
		return false
	}

	_ = unix.Munmap(data)
	return true
}

// probeMmapDir returns the directory to use for the mmap probe. It
// prefers the database directory itself (since that is where the -shm
// file would live), falling back to os.TempDir().
func probeMmapDir(dbPath string) string {
	dir := filepath.Dir(dbPath)
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	return os.TempDir()
}
