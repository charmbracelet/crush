//go:build !windows

package fsext

import (
	"os"
	"syscall"
)

// Owner retrieves the user ID of the owner of the file or directory at the
// specified path.
func Owner(path string) (int, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	var uid int
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		uid = int(stat.Uid)
	} else {
		uid = os.Getuid()
	}
	return uid, nil
}
