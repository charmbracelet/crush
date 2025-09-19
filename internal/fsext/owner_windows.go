//go:build windows

package fsext

import "os"

// Owner retrieves the user ID of the owner of the file or directory at the
// specified path.
func Owner(path string) (int, error) {
	_, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return -1, nil
}
