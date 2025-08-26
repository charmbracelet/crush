//go:build windows

package fsext

// Owner retrieves the user ID of the owner of the file or directory at the
// specified path.
func Owner(path string) (int, error) {
	return -1, nil
}
