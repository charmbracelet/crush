//go:build windows

package fsext

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

// syncDir is a documented no-op on Windows: opening a directory handle and
// calling Sync on it fails there, and there is no portable equivalent. The
// data blocks are already fsynced by AtomicWriteFile, so the only loss is the
// rename itself surviving a crash, which Windows journals at the filesystem
// level for NTFS.
func syncDir(string) error { return nil }
