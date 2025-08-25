package fsext

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/charmbracelet/crush/internal/home"
)

// SearchParent searches for a target file or directory starting from dir
// and walking up the directory tree until found or root or home is reached.
// It also checks the ownership of directories to ensure that the search does
// not cross ownership boundaries.
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
	previousOwner, err := Owner(previousParent)
	if err != nil {
		return "", false
	}

	for {
		parent := filepath.Dir(previousParent)
		if parent == previousParent || parent == home.Dir() {
			return "", false
		}

		parentOwner, err := Owner(parent)
		if err != nil {
			return "", false
		}
		if parentOwner != previousOwner {
			return "", false
		}

		path := filepath.Join(parent, target)
		if _, err := os.Stat(path); err == nil {
			return path, true
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", false
		}

		previousParent = parent
		previousOwner = parentOwner
	}
}
