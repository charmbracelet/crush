//go:build windows

package projects

// lockFile is a no-op on Windows: only the in-process mutex and the atomic
// rename protect the registry there. shortcut: concurrent processes can lose
// an entry on Windows, upgrade with LockFileEx if that platform matters.
func lockFile(string) (func(), error) { return func() {}, nil }
