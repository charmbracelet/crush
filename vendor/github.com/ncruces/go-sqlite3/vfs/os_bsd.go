//go:build ((freebsd || openbsd || netbsd || dragonfly || illumos) && !sqlite3_dotlk) || sqlite3_flock

package vfs

import (
	"os"

	"golang.org/x/sys/unix"
)

func osGetSharedLock(file *os.File) _ErrorCode {
	return osFlock(file, unix.LOCK_SH|unix.LOCK_NB, _IOERR_RDLOCK)
}

func osGetReservedLock(file *os.File) _ErrorCode {
	rc := osFlock(file, unix.LOCK_EX|unix.LOCK_NB, _IOERR_LOCK)
	if rc == _BUSY {
		// The documentation states that a lock is upgraded by
		// releasing the previous lock, then acquiring the new lock.
		// Going over the source code of various BSDs, though,
		// with LOCK_NB, the lock is not released,
		// and EAGAIN is returned holding the shared lock.
		// Still, if we're already in a transaction, we want to abort it,
		// so return BUSY_SNAPSHOT here. If there's no transaction active,
		// SQLite will change this back to SQLITE_BUSY,
		// and invoke the busy handler if appropriate.
		return _BUSY_SNAPSHOT
	}
	return rc
}

func osGetExclusiveLock(file *os.File, state *LockLevel) _ErrorCode {
	if *state >= LOCK_RESERVED {
		return _OK
	}
	return osGetReservedLock(file)
}

func osDowngradeLock(file *os.File, _ LockLevel) _ErrorCode {
	rc := osFlock(file, unix.LOCK_SH|unix.LOCK_NB, _IOERR_RDLOCK)
	if rc == _BUSY {
		// The documentation states that a lock is downgraded by
		// releasing the previous lock then acquiring the new lock.
		// Going over the source code of various BSDs, though,
		// with LOCK_SH|LOCK_NB this should never happen.
		// Return IOERR_RDLOCK, as BUSY would cause an assert to fail.
		return _IOERR_RDLOCK
	}
	return _OK
}

func osReleaseLock(file *os.File, _ LockLevel) _ErrorCode {
	for {
		err := unix.Flock(int(file.Fd()), unix.LOCK_UN)
		if err == nil {
			return _OK
		}
		if err != unix.EINTR {
			return _IOERR_UNLOCK
		}
	}
}

func osCheckReservedLock(file *os.File) (bool, _ErrorCode) {
	// Test the RESERVED lock with fcntl(F_GETLK).
	// This only works on systems where fcntl and flock are compatible.
	// However, SQLite only calls this while holding a shared lock,
	// so the difference is immaterial.
	lock, rc := osTestLock(file, _RESERVED_BYTE, 1)
	return lock == unix.F_WRLCK, rc
}

func osFlock(file *os.File, how int, def _ErrorCode) _ErrorCode {
	err := unix.Flock(int(file.Fd()), how)
	return osLockErrorCode(err, def)
}

func osReadLock(file *os.File, start, len int64) _ErrorCode {
	return osLock(file, unix.F_RDLCK, start, len, _IOERR_RDLOCK)
}

func osWriteLock(file *os.File, start, len int64) _ErrorCode {
	return osLock(file, unix.F_WRLCK, start, len, _IOERR_LOCK)
}

func osLock(file *os.File, typ int16, start, len int64, def _ErrorCode) _ErrorCode {
	err := unix.FcntlFlock(file.Fd(), unix.F_SETLK, &unix.Flock_t{
		Type:  typ,
		Start: start,
		Len:   len,
	})
	return osLockErrorCode(err, def)
}

func osUnlock(file *os.File, start, len int64) _ErrorCode {
	lock := unix.Flock_t{
		Type:  unix.F_UNLCK,
		Start: start,
		Len:   len,
	}
	for {
		err := unix.FcntlFlock(file.Fd(), unix.F_SETLK, &lock)
		if err == nil {
			return _OK
		}
		if err != unix.EINTR {
			return _IOERR_UNLOCK
		}
	}
}
