//go:build !unix

package watcher

// MaximizeOpenFileLimit is a no-op on non-Unix systems.
// Returns a high value to indicate no practical limit.
func MaximizeOpenFileLimit() (int, error) {
	// Windows and other non-Unix systems don't have file descriptor limits
	// in the same way Unix systems do. Return a very high value to indicate
	// there's no practical limit to worry about.
	return 10000000, nil // 10M handles - way more than any process would use
}