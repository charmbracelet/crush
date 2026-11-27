//go:build !darwin && !linux && !freebsd && !netbsd && !openbsd

package pinentry

// procExeSupported reports whether /proc/<pid>/exe can be read to resolve
// a process executable when the process table does not provide a path.
const procExeSupported = false

// BeginHandover is a no-op without a POSIX terminal to hand over.
func BeginHandover(bool) {}

// EndHandover is a no-op without a POSIX terminal to hand over.
func EndHandover() {}
