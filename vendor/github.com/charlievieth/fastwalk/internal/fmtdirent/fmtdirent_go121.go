//go:build go1.21

package fmtdirent

import "io/fs"

// FormatDirEntry returns a formatted version of dir for human readability.
// Implementations of [DirEntry] can call this from a String method.
// The outputs for a directory named subdir and a file named hello.go are:
//
//	d subdir/
//	- hello.go
func FormatDirEntry(dir fs.DirEntry) string {
	return fs.FormatDirEntry(dir)
}
