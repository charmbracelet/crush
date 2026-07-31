//go:build !windows

package config

// isTransientRenameError reports whether err is a rename failure that
// can resolve on its own. Only Windows has such failures.
func isTransientRenameError(error) bool { return false }
