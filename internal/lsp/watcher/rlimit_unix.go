//go:build unix

// This file contains code inspired by Syncthing's rlimit implementation
// Syncthing is licensed under the Mozilla Public License Version 2.0
// See: https://github.com/syncthing/syncthing/blob/main/LICENSE

package watcher

import (
	"runtime"
	"syscall"
)

const (
	// macOS has a specific limit for RLIMIT_NOFILE
	darwinOpenMax = 10240
)

// maximizeOpenFileLimit tries to set the resource limit RLIMIT_NOFILE (number
// of open file descriptors) to the max (hard limit), if the current (soft
// limit) is below the max. Returns the new (though possibly unchanged) limit,
// or an error if it could not be changed.
func maximizeOpenFileLimit() (int, error) {
	// Get the current limit on number of open files.
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		return 0, err
	}

	// If we're already at max, there's no need to try to raise the limit.
	if lim.Cur >= lim.Max {
		return int(lim.Cur), nil
	}

	// macOS doesn't like a soft limit greater than OPEN_MAX
	if runtime.GOOS == "darwin" && lim.Max > darwinOpenMax {
		lim.Max = darwinOpenMax
	}

	// Try to increase the limit to the max.
	oldLimit := lim.Cur
	lim.Cur = lim.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		return int(oldLimit), err
	}

	// If the set succeeded, perform a new get to see what happened. We might
	// have gotten a value lower than the one in lim.Max, if lim.Max was
	// something that indicated "unlimited" (i.e. intmax).
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		// We don't really know the correct value here since Getrlimit
		// mysteriously failed after working once... Shouldn't ever happen.
		return 0, err
	}

	return int(lim.Cur), nil
}
