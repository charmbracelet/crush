//go:build windows

package model

// lockDraftsFile is a no-op on Windows. shortcut: two TUIs quitting at the
// same instant can lose a draft there, upgrade with LockFileEx if it matters.
func lockDraftsFile(string) (func(), error) { return func() {}, nil }
