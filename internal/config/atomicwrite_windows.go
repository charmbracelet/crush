//go:build windows

package config

import (
	"errors"

	"golang.org/x/sys/windows"
)

func isRetryableRenameError(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) ||
		errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
