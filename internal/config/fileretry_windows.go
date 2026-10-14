//go:build windows

package config

import (
	"errors"

	"golang.org/x/sys/windows"
)

// isTransientFileError reports whether err is a Windows file failure
// that clears once another handle (a concurrent reader or writer,
// antivirus, or the search indexer) briefly open on the file closes.
func isTransientFileError(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) ||
		errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
