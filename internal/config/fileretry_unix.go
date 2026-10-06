//go:build !windows

package config

// isTransientFileError reports whether err is a file failure that can
// resolve on its own. Only Windows has such failures.
func isTransientFileError(error) bool { return false }
