//go:build !(linux || darwin || windows || freebsd || openbsd || netbsd || dragonfly || illumos || sqlite3_flock || sqlite3_dotlk)

package vfs

// SupportsFileLocking is false on platforms that do not support file locking.
// To open a database file on those platforms,
// you need to use the [nolock] or [immutable] URI parameters.
//
// [nolock]: https://sqlite.org/uri.html#urinolock
// [immutable]: https://sqlite.org/uri.html#uriimmutable
const SupportsFileLocking = false

func (f *vfsFile) Lock(LockLevel) error {
	return _IOERR_LOCK
}

func (f *vfsFile) Unlock(LockLevel) error {
	return _IOERR_UNLOCK
}

func (f *vfsFile) CheckReservedLock() (bool, error) {
	return false, _IOERR_CHECKRESERVEDLOCK
}
