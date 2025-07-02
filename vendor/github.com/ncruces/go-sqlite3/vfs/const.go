package vfs

import "github.com/ncruces/go-sqlite3/internal/util"

const (
	_MAX_NAME            = 1e6 // Self-imposed limit for most NUL terminated strings.
	_MAX_SQL_LENGTH      = 1e9
	_MAX_PATHNAME        = 1024
	_DEFAULT_SECTOR_SIZE = 4096

	ptrlen = util.PtrLen
)

type (
	stk_t = util.Stk_t
	ptr_t = util.Ptr_t
)

// https://sqlite.org/rescode.html
type _ErrorCode uint32

func (e _ErrorCode) Error() string {
	return util.ErrorCodeString(uint32(e))
}

const (
	_OK                      _ErrorCode = util.OK
	_ERROR                   _ErrorCode = util.ERROR
	_PERM                    _ErrorCode = util.PERM
	_BUSY                    _ErrorCode = util.BUSY
	_READONLY                _ErrorCode = util.READONLY
	_IOERR                   _ErrorCode = util.IOERR
	_NOTFOUND                _ErrorCode = util.NOTFOUND
	_FULL                    _ErrorCode = util.FULL
	_CANTOPEN                _ErrorCode = util.CANTOPEN
	_IOERR_READ              _ErrorCode = util.IOERR_READ
	_IOERR_SHORT_READ        _ErrorCode = util.IOERR_SHORT_READ
	_IOERR_WRITE             _ErrorCode = util.IOERR_WRITE
	_IOERR_FSYNC             _ErrorCode = util.IOERR_FSYNC
	_IOERR_DIR_FSYNC         _ErrorCode = util.IOERR_DIR_FSYNC
	_IOERR_TRUNCATE          _ErrorCode = util.IOERR_TRUNCATE
	_IOERR_FSTAT             _ErrorCode = util.IOERR_FSTAT
	_IOERR_UNLOCK            _ErrorCode = util.IOERR_UNLOCK
	_IOERR_RDLOCK            _ErrorCode = util.IOERR_RDLOCK
	_IOERR_DELETE            _ErrorCode = util.IOERR_DELETE
	_IOERR_ACCESS            _ErrorCode = util.IOERR_ACCESS
	_IOERR_CHECKRESERVEDLOCK _ErrorCode = util.IOERR_CHECKRESERVEDLOCK
	_IOERR_LOCK              _ErrorCode = util.IOERR_LOCK
	_IOERR_CLOSE             _ErrorCode = util.IOERR_CLOSE
	_IOERR_SHMOPEN           _ErrorCode = util.IOERR_SHMOPEN
	_IOERR_SHMSIZE           _ErrorCode = util.IOERR_SHMSIZE
	_IOERR_SHMLOCK           _ErrorCode = util.IOERR_SHMLOCK
	_IOERR_SHMMAP            _ErrorCode = util.IOERR_SHMMAP
	_IOERR_SEEK              _ErrorCode = util.IOERR_SEEK
	_IOERR_DELETE_NOENT      _ErrorCode = util.IOERR_DELETE_NOENT
	_IOERR_GETTEMPPATH       _ErrorCode = util.IOERR_GETTEMPPATH
	_IOERR_BEGIN_ATOMIC      _ErrorCode = util.IOERR_BEGIN_ATOMIC
	_IOERR_COMMIT_ATOMIC     _ErrorCode = util.IOERR_COMMIT_ATOMIC
	_IOERR_ROLLBACK_ATOMIC   _ErrorCode = util.IOERR_ROLLBACK_ATOMIC
	_IOERR_DATA              _ErrorCode = util.IOERR_DATA
	_IOERR_CORRUPTFS         _ErrorCode = util.IOERR_CORRUPTFS
	_BUSY_SNAPSHOT           _ErrorCode = util.BUSY_SNAPSHOT
	_CANTOPEN_FULLPATH       _ErrorCode = util.CANTOPEN_FULLPATH
	_CANTOPEN_ISDIR          _ErrorCode = util.CANTOPEN_ISDIR
	_READONLY_CANTINIT       _ErrorCode = util.READONLY_CANTINIT
	_READONLY_DIRECTORY      _ErrorCode = util.READONLY_DIRECTORY
	_OK_SYMLINK              _ErrorCode = util.OK_SYMLINK
)

// OpenFlag is a flag for the [VFS] Open method.
//
// https://sqlite.org/c3ref/c_open_autoproxy.html
type OpenFlag uint32

const (
	OPEN_READONLY      OpenFlag = 0x00000001 /* Ok for sqlite3_open_v2() */
	OPEN_READWRITE     OpenFlag = 0x00000002 /* Ok for sqlite3_open_v2() */
	OPEN_CREATE        OpenFlag = 0x00000004 /* Ok for sqlite3_open_v2() */
	OPEN_DELETEONCLOSE OpenFlag = 0x00000008 /* VFS only */
	OPEN_EXCLUSIVE     OpenFlag = 0x00000010 /* VFS only */
	OPEN_AUTOPROXY     OpenFlag = 0x00000020 /* VFS only */
	OPEN_URI           OpenFlag = 0x00000040 /* Ok for sqlite3_open_v2() */
	OPEN_MEMORY        OpenFlag = 0x00000080 /* Ok for sqlite3_open_v2() */
	OPEN_MAIN_DB       OpenFlag = 0x00000100 /* VFS only */
	OPEN_TEMP_DB       OpenFlag = 0x00000200 /* VFS only */
	OPEN_TRANSIENT_DB  OpenFlag = 0x00000400 /* VFS only */
	OPEN_MAIN_JOURNAL  OpenFlag = 0x00000800 /* VFS only */
	OPEN_TEMP_JOURNAL  OpenFlag = 0x00001000 /* VFS only */
	OPEN_SUBJOURNAL    OpenFlag = 0x00002000 /* VFS only */
	OPEN_SUPER_JOURNAL OpenFlag = 0x00004000 /* VFS only */
	OPEN_NOMUTEX       OpenFlag = 0x00008000 /* Ok for sqlite3_open_v2() */
	OPEN_FULLMUTEX     OpenFlag = 0x00010000 /* Ok for sqlite3_open_v2() */
	OPEN_SHAREDCACHE   OpenFlag = 0x00020000 /* Ok for sqlite3_open_v2() */
	OPEN_PRIVATECACHE  OpenFlag = 0x00040000 /* Ok for sqlite3_open_v2() */
	OPEN_WAL           OpenFlag = 0x00080000 /* VFS only */
	OPEN_NOFOLLOW      OpenFlag = 0x01000000 /* Ok for sqlite3_open_v2() */
)

// AccessFlag is a flag for the [VFS] Access method.
//
// https://sqlite.org/c3ref/c_access_exists.html
type AccessFlag uint32

const (
	ACCESS_EXISTS    AccessFlag = 0
	ACCESS_READWRITE AccessFlag = 1 /* Used by PRAGMA temp_store_directory */
	ACCESS_READ      AccessFlag = 2 /* Unused */
)

// SyncFlag is a flag for the [File] Sync method.
//
// https://sqlite.org/c3ref/c_sync_dataonly.html
type SyncFlag uint32

const (
	SYNC_NORMAL   SyncFlag = 0x00002
	SYNC_FULL     SyncFlag = 0x00003
	SYNC_DATAONLY SyncFlag = 0x00010
)

// LockLevel is a value used with [File] Lock and Unlock methods.
//
// https://sqlite.org/c3ref/c_lock_exclusive.html
type LockLevel uint32

const (
	// No locks are held on the database.
	// The database may be neither read nor written.
	// Any internally cached data is considered suspect and subject to
	// verification against the database file before being used.
	// Other processes can read or write the database as their own locking
	// states permit.
	// This is the default state.
	LOCK_NONE LockLevel = 0 /* xUnlock() only */

	// The database may be read but not written.
	// Any number of processes can hold SHARED locks at the same time,
	// hence there can be many simultaneous readers.
	// But no other thread or process is allowed to write to the database file
	// while one or more SHARED locks are active.
	LOCK_SHARED LockLevel = 1 /* xLock() or xUnlock() */

	// A RESERVED lock means that the process is planning on writing to the
	// database file at some point in the future but that it is currently just
	// reading from the file.
	// Only a single RESERVED lock may be active at one time,
	// though multiple SHARED locks can coexist with a single RESERVED lock.
	// RESERVED differs from PENDING in that new SHARED locks can be acquired
	// while there is a RESERVED lock.
	LOCK_RESERVED LockLevel = 2 /* xLock() only */

	// A PENDING lock means that the process holding the lock wants to write to
	// the database as soon as possible and is just waiting on all current
	// SHARED locks to clear so that it can get an EXCLUSIVE lock.
	// No new SHARED locks are permitted against the database if a PENDING lock
	// is active, though existing SHARED locks are allowed to continue.
	LOCK_PENDING LockLevel = 3 /* internal use only */

	// An EXCLUSIVE lock is needed in order to write to the database file.
	// Only one EXCLUSIVE lock is allowed on the file and no other locks of any
	// kind are allowed to coexist with an EXCLUSIVE lock.
	// In order to maximize concurrency, SQLite works to minimize the amount of
	// time that EXCLUSIVE locks are held.
	LOCK_EXCLUSIVE LockLevel = 4 /* xLock() only */
)

// DeviceCharacteristic is a flag retuned by the [File] DeviceCharacteristics method.
//
// https://sqlite.org/c3ref/c_iocap_atomic.html
type DeviceCharacteristic uint32

const (
	IOCAP_ATOMIC                DeviceCharacteristic = 0x00000001
	IOCAP_ATOMIC512             DeviceCharacteristic = 0x00000002
	IOCAP_ATOMIC1K              DeviceCharacteristic = 0x00000004
	IOCAP_ATOMIC2K              DeviceCharacteristic = 0x00000008
	IOCAP_ATOMIC4K              DeviceCharacteristic = 0x00000010
	IOCAP_ATOMIC8K              DeviceCharacteristic = 0x00000020
	IOCAP_ATOMIC16K             DeviceCharacteristic = 0x00000040
	IOCAP_ATOMIC32K             DeviceCharacteristic = 0x00000080
	IOCAP_ATOMIC64K             DeviceCharacteristic = 0x00000100
	IOCAP_SAFE_APPEND           DeviceCharacteristic = 0x00000200
	IOCAP_SEQUENTIAL            DeviceCharacteristic = 0x00000400
	IOCAP_UNDELETABLE_WHEN_OPEN DeviceCharacteristic = 0x00000800
	IOCAP_POWERSAFE_OVERWRITE   DeviceCharacteristic = 0x00001000
	IOCAP_IMMUTABLE             DeviceCharacteristic = 0x00002000
	IOCAP_BATCH_ATOMIC          DeviceCharacteristic = 0x00004000
	IOCAP_SUBPAGE_READ          DeviceCharacteristic = 0x00008000
)

// https://sqlite.org/c3ref/c_fcntl_begin_atomic_write.html
type _FcntlOpcode uint32

const (
	_FCNTL_LOCKSTATE             _FcntlOpcode = 1
	_FCNTL_GET_LOCKPROXYFILE     _FcntlOpcode = 2
	_FCNTL_SET_LOCKPROXYFILE     _FcntlOpcode = 3
	_FCNTL_LAST_ERRNO            _FcntlOpcode = 4
	_FCNTL_SIZE_HINT             _FcntlOpcode = 5
	_FCNTL_CHUNK_SIZE            _FcntlOpcode = 6
	_FCNTL_FILE_POINTER          _FcntlOpcode = 7
	_FCNTL_SYNC_OMITTED          _FcntlOpcode = 8
	_FCNTL_WIN32_AV_RETRY        _FcntlOpcode = 9
	_FCNTL_PERSIST_WAL           _FcntlOpcode = 10
	_FCNTL_OVERWRITE             _FcntlOpcode = 11
	_FCNTL_VFSNAME               _FcntlOpcode = 12
	_FCNTL_POWERSAFE_OVERWRITE   _FcntlOpcode = 13
	_FCNTL_PRAGMA                _FcntlOpcode = 14
	_FCNTL_BUSYHANDLER           _FcntlOpcode = 15
	_FCNTL_TEMPFILENAME          _FcntlOpcode = 16
	_FCNTL_MMAP_SIZE             _FcntlOpcode = 18
	_FCNTL_TRACE                 _FcntlOpcode = 19
	_FCNTL_HAS_MOVED             _FcntlOpcode = 20
	_FCNTL_SYNC                  _FcntlOpcode = 21
	_FCNTL_COMMIT_PHASETWO       _FcntlOpcode = 22
	_FCNTL_WIN32_SET_HANDLE      _FcntlOpcode = 23
	_FCNTL_WAL_BLOCK             _FcntlOpcode = 24
	_FCNTL_ZIPVFS                _FcntlOpcode = 25
	_FCNTL_RBU                   _FcntlOpcode = 26
	_FCNTL_VFS_POINTER           _FcntlOpcode = 27
	_FCNTL_JOURNAL_POINTER       _FcntlOpcode = 28
	_FCNTL_WIN32_GET_HANDLE      _FcntlOpcode = 29
	_FCNTL_PDB                   _FcntlOpcode = 30
	_FCNTL_BEGIN_ATOMIC_WRITE    _FcntlOpcode = 31
	_FCNTL_COMMIT_ATOMIC_WRITE   _FcntlOpcode = 32
	_FCNTL_ROLLBACK_ATOMIC_WRITE _FcntlOpcode = 33
	_FCNTL_LOCK_TIMEOUT          _FcntlOpcode = 34
	_FCNTL_DATA_VERSION          _FcntlOpcode = 35
	_FCNTL_SIZE_LIMIT            _FcntlOpcode = 36
	_FCNTL_CKPT_DONE             _FcntlOpcode = 37
	_FCNTL_RESERVE_BYTES         _FcntlOpcode = 38
	_FCNTL_CKPT_START            _FcntlOpcode = 39
	_FCNTL_EXTERNAL_READER       _FcntlOpcode = 40
	_FCNTL_CKSM_FILE             _FcntlOpcode = 41
	_FCNTL_RESET_CACHE           _FcntlOpcode = 42
	_FCNTL_NULL_IO               _FcntlOpcode = 43
)

// https://sqlite.org/c3ref/c_shm_exclusive.html
type _ShmFlag uint32

const (
	_SHM_UNLOCK    _ShmFlag = 1
	_SHM_LOCK      _ShmFlag = 2
	_SHM_SHARED    _ShmFlag = 4
	_SHM_EXCLUSIVE _ShmFlag = 8

	_SHM_NLOCK = 8
	_SHM_BASE  = 120
	_SHM_DMS   = _SHM_BASE + _SHM_NLOCK
)
