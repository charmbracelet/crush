//go:build !unix && (!windows || sqlite3_dotlk)

package vfs

import "os"

func osReadAt(file *os.File, p []byte, off int64) (int, error) {
	return file.ReadAt(p, off)
}

func osWriteAt(file *os.File, p []byte, off int64) (int, error) {
	return file.WriteAt(p, off)
}
