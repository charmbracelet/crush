//go:build windows

package fsext

import (
	"cmp"
	"os"
	"path/filepath"
)

// WindowsWorkingDirDrive returns the drive letter of the current working
// directory, e.g. "C:".
// Falls back to the system drive if the current working directory cannot be
// determined.
func WindowsWorkingDirDrive() string {
	if cwd, err := os.Getwd(); err == nil {
		return filepath.VolumeName(cwd)
	}
	return WindowsSystemDrive()
}

// WindowsSystemDrive returns the drive letter of the system drive, e.g. "C:".
func WindowsSystemDrive() string {
	systemRoot := cmp.Or(os.Getenv("SYSTEMROOT"), os.Getenv("WINDIR"))
	return filepath.VolumeName(systemRoot)
}
