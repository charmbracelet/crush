package fsext

import (
	"errors"
	"os"
	"path/filepath"
)

// SearchParent searches for a target file or directory starting from dir
// and walking up the directory tree until found or root or home is reached.
// Returns the full path to the target if found, empty string and false otherwise.
// The search includes the starting directory itself.
func SearchParent(dir, target string) (string, bool) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}

	path := filepath.Join(absDir, target)
	if _, err := os.Stat(path); err == nil {
		return path, true
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", false
	}

	previousParent := absDir

	for {
		parent := filepath.Dir(previousParent)
		if parent == previousParent || parent == HomeDir() {
			return "", false
		}

		path := filepath.Join(parent, target)
		if _, err := os.Stat(path); err == nil {
			return path, true
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", false
		}

		previousParent = parent
	}
}
