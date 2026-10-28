//go:build !windows

package dirid

import "os"

// identity holds the stat result, whose device and inode numbers
// os.SameFile compares without further IO.
type identity = os.FileInfo

func of(path string) (identity, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	return info, true
}

func same(a, b identity) bool {
	return os.SameFile(a, b)
}
