//go:build !windows

package fsext

// isTransientRenameError reports whether err is a rename failure that
// can resolve on its own. Only Windows has such failures.
func isTransientRenameError(error) bool { return false }
