//go:build !go1.21

package fmtdirent

import "io/fs"

// Backport fs.FormatDirEntry from go1.21

// FormatDirEntry returns a formatted version of dir for human readability.
// Implementations of [DirEntry] can call this from a String method.
// The outputs for a directory named subdir and a file named hello.go are:
//
//	d subdir/
//	- hello.go
func FormatDirEntry(dir fs.DirEntry) string {
	name := dir.Name()
	b := make([]byte, 0, 5+len(name))

	// The Type method does not return any permission bits,
	// so strip them from the string.
	mode := dir.Type().String()
	mode = mode[:len(mode)-9]

	b = append(b, mode...)
	b = append(b, ' ')
	b = append(b, name...)
	if dir.IsDir() {
		b = append(b, '/')
	}
	return string(b)
}
