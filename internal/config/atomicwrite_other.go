//go:build !windows

package config

func isRetryableRenameError(error) bool {
	return false
}
