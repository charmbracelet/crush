//go:build !unix

package vfs

import (
	"io/fs"
	"os"
)

const (
	isUnix      = false
	_O_NOFOLLOW = 0
)

func osAccess(path string, flags AccessFlag) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if flags == ACCESS_EXISTS {
		return nil
	}

	const (
		S_IREAD  = 0400
		S_IWRITE = 0200
		S_IEXEC  = 0100
	)

	var want fs.FileMode = S_IREAD
	if flags == ACCESS_READWRITE {
		want |= S_IWRITE
	}
	if fi.IsDir() {
		want |= S_IEXEC
	}
	if fi.Mode()&want != want {
		return fs.ErrPermission
	}
	return nil
}

func osSetMode(file *os.File, modeof string) error {
	fi, err := os.Stat(modeof)
	if err != nil {
		return err
	}
	file.Chmod(fi.Mode())
	return nil
}
