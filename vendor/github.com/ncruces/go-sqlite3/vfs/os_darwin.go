//go:build !sqlite3_flock

package vfs

import (
	"io"
	"os"
	"runtime"
	"time"

	"golang.org/x/sys/unix"
)

const (
	// https://github.com/apple/darwin-xnu/blob/main/bsd/sys/fcntl.h
	_F_OFD_SETLK         = 90
	_F_OFD_SETLKW        = 91
	_F_OFD_SETLKWTIMEOUT = 93
)

type flocktimeout_t struct {
	fl      unix.Flock_t
	timeout unix.Timespec
}

func osSync(file *os.File, fullsync, _ /*dataonly*/ bool) error {
	if fullsync {
		return file.Sync()
	}
	for {
		err := unix.Fsync(int(file.Fd()))
		if err != unix.EINTR {
			return err
		}
	}
}

func osAllocate(file *os.File, size int64) error {
	off, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	if size <= off {
		return nil
	}

	store := unix.Fstore_t{
		Flags:   unix.F_ALLOCATEALL | unix.F_ALLOCATECONTIG,
		Posmode: unix.F_PEOFPOSMODE,
		Offset:  0,
		Length:  size - off,
	}

	// Try to get a continuous chunk of disk space.
	err = unix.FcntlFstore(file.Fd(), unix.F_PREALLOCATE, &store)
	if err != nil {
		// OK, perhaps we are too fragmented, allocate non-continuous.
		store.Flags = unix.F_ALLOCATEALL
		unix.FcntlFstore(file.Fd(), unix.F_PREALLOCATE, &store)
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
	lock := &flocktimeout_t{fl: unix.Flock_t{
		Type:  typ,
		Start: start,
		Len:   len,
	}}
	var err error
	switch {
	case timeout == 0:
		err = unix.FcntlFlock(file.Fd(), _F_OFD_SETLK, &lock.fl)
	case timeout < 0:
		err = unix.FcntlFlock(file.Fd(), _F_OFD_SETLKW, &lock.fl)
	default:
		lock.timeout = unix.NsecToTimespec(int64(timeout / time.Nanosecond))
		err = unix.FcntlFlock(file.Fd(), _F_OFD_SETLKWTIMEOUT, &lock.fl)
		runtime.KeepAlive(lock)
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
		err := unix.FcntlFlock(file.Fd(), _F_OFD_SETLK, &lock)
		if err == nil {
			return _OK
		}
		if err != unix.EINTR {
			return _IOERR_UNLOCK
		}
	}
}
