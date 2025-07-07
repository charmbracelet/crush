//go:build !sqlite3_flock

package vfs

import (
	"io"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func osSync(file *os.File, _ /*fullsync*/, _ /*dataonly*/ bool) error {
	// SQLite trusts Linux's fdatasync for all fsync's.
	for {
		err := unix.Fdatasync(int(file.Fd()))
		if err != unix.EINTR {
			return err
		}
	}
}

func osAllocate(file *os.File, size int64) error {
	if size == 0 {
		return nil
	}
	for {
		err := unix.Fallocate(int(file.Fd()), 0, 0, size)
		if err == unix.EOPNOTSUPP {
			break
		}
		if err != unix.EINTR {
			return err
		}
	}
	off, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	if size <= off {
		return nil
	}
	return file.Truncate(size)

}

func osReadLock(file *os.File, start, len int64, timeout time.Duration) _ErrorCode {
	return osLock(file, unix.F_RDLCK, start, len, timeout, _IOERR_RDLOCK)
}

func osWriteLock(file *os.File, start, len int64, timeout time.Duration) _ErrorCode {
	return osLock(file, unix.F_WRLCK, start, len, timeout, _IOERR_LOCK)
}

func osLock(file *os.File, typ int16, start, len int64, timeout time.Duration, def _ErrorCode) _ErrorCode {
	lock := unix.Flock_t{
		Type:  typ,
		Start: start,
		Len:   len,
	}
	var err error
	switch {
	default:
		err = unix.FcntlFlock(file.Fd(), unix.F_OFD_SETLK, &lock)
	case timeout < 0:
		err = unix.FcntlFlock(file.Fd(), unix.F_OFD_SETLKW, &lock)
	}
	return osLockErrorCode(err, def)
}

func osUnlock(file *os.File, start, len int64) _ErrorCode {
	lock := unix.Flock_t{
		Type:  unix.F_UNLCK,
		Start: start,
		Len:   len,
	}
	for {
		err := unix.FcntlFlock(file.Fd(), unix.F_OFD_SETLK, &lock)
		if err == nil {
			return _OK
		}
		if err != unix.EINTR {
			return _IOERR_UNLOCK
		}
	}
}
