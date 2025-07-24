//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly && !windows

package watcher

func Ulimit() (uint64, error) {
	// Fallback for exotic systems - return a reasonable default
	return 2048, nil
}
