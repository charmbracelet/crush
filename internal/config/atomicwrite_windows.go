//go:build windows

package config

import (
	"errors"

	"golang.org/x/sys/windows"
)

// isTransientRenameError reports whether err is a Windows rename
// failure that can resolve on its own: replacing the destination fails
// while another handle (a concurrent reader, antivirus, or the search
// indexer) is briefly open on it without FILE_SHARE_DELETE.
func isTransientRenameError(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) ||
		errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
