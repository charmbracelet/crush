//go:build !windows

package fsext

// isTransientError reports whether err is a file failure that can resolve
// on its own. Only Windows has such failures.
func isTransientError(error) bool { return false }
