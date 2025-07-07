// Package vfs wraps the C SQLite VFS API.
package vfs

import (
	"context"
	"io"

	"github.com/tetratelabs/wazero/api"
)

// A VFS defines the interface between the SQLite core and the underlying operating system.
//
// Use sqlite3.ErrorCode or sqlite3.ExtendedErrorCode to return specific error codes to SQLite.
//
// https://sqlite.org/c3ref/vfs.html
type VFS interface {
	Open(name string, flags OpenFlag) (File, OpenFlag, error)
	Delete(name string, syncDir bool) error
	Access(name string, flags AccessFlag) (bool, error)
	FullPathname(name string) (string, error)
}

// VFSFilename extends VFS with the ability to use Filename
// objects for opening files.
//
// https://sqlite.org/c3ref/filename.html
type VFSFilename interface {
	VFS
	OpenFilename(name *Filename, flags OpenFlag) (File, OpenFlag, error)
}

// A File represents an open file in the OS interface layer.
//
// Use sqlite3.ErrorCode or sqlite3.ExtendedErrorCode to return specific error codes to SQLite.
// In particular, sqlite3.BUSY is necessary to correctly implement lock methods.
//
// https://sqlite.org/c3ref/io_methods.html
type File interface {
	Close() error
	ReadAt(p []byte, off int64) (n int, err error)
	WriteAt(p []byte, off int64) (n int, err error)
	Truncate(size int64) error
	Sync(flags SyncFlag) error
	Size() (int64, error)
	Lock(lock LockLevel) error
	Unlock(lock LockLevel) error
	CheckReservedLock() (bool, error)
	SectorSize() int
	DeviceCharacteristics() DeviceCharacteristic
}

// FileUnwrap should be implemented by a File
// that wraps another File implementation.
type FileUnwrap interface {
	File
	Unwrap() File
}

// FileLockState extends File to implement the
// SQLITE_FCNTL_LOCKSTATE file control opcode.
//
// https://sqlite.org/c3ref/c_fcntl_begin_atomic_write.html#sqlitefcntllockstate
type FileLockState interface {
	File
	LockState() LockLevel
}

// FilePersistWAL extends File to implement the
// SQLITE_FCNTL_PERSIST_WAL file control opcode.
//
// https://sqlite.org/c3ref/c_fcntl_begin_atomic_write.html#sqlitefcntlpersistwal
type FilePersistWAL interface {
	File
	PersistWAL() bool
	SetPersistWAL(bool)
}

// FilePowersafeOverwrite extends File to implement the
// SQLITE_FCNTL_POWERSAFE_OVERWRITE file control opcode.
//
// https://sqlite.org/c3ref/c_fcntl_begin_atomic_write.html#sqlitefcntlpowersafeoverwrite
type FilePowersafeOverwrite interface {
	File
	PowersafeOverwrite() bool
	SetPowersafeOverwrite(bool)
}

// FileChunkSize extends File to implement the
// SQLITE_FCNTL_CHUNK_SIZE file control opcode.
//
// https://sqlite.org/c3ref/c_fcntl_begin_atomic_write.html#sqlitefcntlchunksize
type FileChunkSize interface {
	File
	ChunkSize(size int)
}

// FileSizeHint extends File to implement the
// SQLITE_FCNTL_SIZE_HINT file control opcode.
//
// https://sqlite.org/c3ref/c_fcntl_begin_atomic_write.html#sqlitefcntlsizehint
type FileSizeHint interface {
	File
	SizeHint(size int64) error
}

// FileHasMoved extends File to implement the
// SQLITE_FCNTL_HAS_MOVED file control opcode.
//
// https://sqlite.org/c3ref/c_fcntl_begin_atomic_write.html#sqlitefcntlhasmoved
type FileHasMoved interface {
	File
	HasMoved() (bool, error)
}

// FileOverwrite extends File to implement the
// SQLITE_FCNTL_OVERWRITE file control opcode.
//
// https://sqlite.org/c3ref/c_fcntl_begin_atomic_write.html#sqlitefcntloverwrite
type FileOverwrite interface {
	File
	Overwrite() error
}

// FileSync extends File to implement the
// SQLITE_FCNTL_SYNC file control opcode.
//
// https://sqlite.org/c3ref/c_fcntl_begin_atomic_write.html#sqlitefcntlsync
type FileSync interface {
	File
	SyncSuper(super string) error
}

// FileCommitPhaseTwo extends File to implement the
// SQLITE_FCNTL_COMMIT_PHASETWO file control opcode.
//
// https://sqlite.org/c3ref/c_fcntl_begin_atomic_write.html#sqlitefcntlcommitphasetwo
type FileCommitPhaseTwo interface {
	File
	CommitPhaseTwo() error
}

// FileBatchAtomicWrite extends File to implement the
// SQLITE_FCNTL_BEGIN_ATOMIC_WRITE, SQLITE_FCNTL_COMMIT_ATOMIC_WRITE
// and SQLITE_FCNTL_ROLLBACK_ATOMIC_WRITE file control opcodes.
//
// https://sqlite.org/c3ref/c_fcntl_begin_atomic_write.html#sqlitefcntlbeginatomicwrite
type FileBatchAtomicWrite interface {
	File
	BeginAtomicWrite() error
	CommitAtomicWrite() error
	RollbackAtomicWrite() error
}

// FileCheckpoint extends File to implement the
// SQLITE_FCNTL_CKPT_START and SQLITE_FCNTL_CKPT_DONE
// file control opcodes.
//
// https://sqlite.org/c3ref/c_fcntl_begin_atomic_write.html#sqlitefcntlckptstart
type FileCheckpoint interface {
	File
	CheckpointStart()
	CheckpointDone()
}

// FilePragma extends File to implement the
// SQLITE_FCNTL_PRAGMA file control opcode.
//
// https://sqlite.org/c3ref/c_fcntl_begin_atomic_write.html#sqlitefcntlpragma
type FilePragma interface {
	File
	Pragma(name, value string) (string, error)
}

// FileBusyHandler extends File to implement the
// SQLITE_FCNTL_BUSYHANDLER file control opcode.
//
// https://sqlite.org/c3ref/c_fcntl_begin_atomic_write.html#sqlitefcntlbusyhandler
type FileBusyHandler interface {
	File
	BusyHandler(func() bool)
}

// FileSharedMemory extends File to possibly implement
// shared-memory for the WAL-index.
// The same shared-memory instance must be returned
// for the entire life of the file.
// It's OK for SharedMemory to return nil.
type FileSharedMemory interface {
	File
	SharedMemory() SharedMemory
}

// SharedMemory is a shared-memory WAL-index implementation.
// Use [NewSharedMemory] to create a shared-memory.
type SharedMemory interface {
	shmMap(context.Context, api.Module, int32, int32, bool) (ptr_t, _ErrorCode)
	shmLock(int32, int32, _ShmFlag) _ErrorCode
	shmUnmap(bool)
	shmBarrier()
	io.Closer
}

type blockingSharedMemory interface {
	SharedMemory
	shmEnableBlocking(block bool)
}

type fileControl interface {
	File
	fileControl(ctx context.Context, mod api.Module, op _FcntlOpcode, pArg ptr_t) _ErrorCode
}

type filePDB interface {
	File
	SetDB(any)
}
