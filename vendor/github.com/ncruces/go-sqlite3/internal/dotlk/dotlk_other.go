//go:build !unix

package dotlk

import "os"

// TryLock returns nil if it acquired the lock,
// fs.ErrExist if another process has the lock.
func TryLock(name string) error {
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	f.Close()
	return err
}
