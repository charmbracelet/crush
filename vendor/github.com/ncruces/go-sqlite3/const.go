package sqlite3

import (
	"strconv"

	"github.com/ncruces/go-sqlite3/internal/util"
)

const (
	_OK   = 0   /* Successful result */
	_ROW  = 100 /* sqlite3_step() has another row ready */
	_DONE = 101 /* sqlite3_step() has finished executing */

	_MAX_NAME       = 1e6 // Self-imposed limit for most NUL terminated strings.
	_MAX_LENGTH     = 1e9
	_MAX_SQL_LENGTH = 1e9

	ptrlen = util.PtrLen
	intlen = util.IntLen
)

type (
	stk_t = util.Stk_t
	ptr_t = util.Ptr_t
	res_t = util.Res_t
)

// ErrorCode is a result code that [Error.Code] might return.
//
// https://sqlite.org/rescode.html
type ErrorCode uint8

const (
	ERROR      ErrorCode = 1  /* Generic error */
	INTERNAL   ErrorCode = 2  /* Internal logic error in SQLite */
	PERM       ErrorCode = 3  /* Access permission denied */
	ABORT      ErrorCode = 4  /* Callback routine requested an abort */
	BUSY       ErrorCode = 5  /* The database file is locked */
	LOCKED     ErrorCode = 6  /* A table in the database is locked */
	NOMEM      ErrorCode = 7  /* A malloc() failed */
	READONLY   ErrorCode = 8  /* Attempt to write a readonly database */
	INTERRUPT  ErrorCode = 9  /* Operation terminated by sqlite3_interrupt() */
	IOERR      ErrorCode = 10 /* Some kind of disk I/O error occurred */
	CORRUPT    ErrorCode = 11 /* The database disk image is malformed */
	NOTFOUND   ErrorCode = 12 /* Unknown opcode in sqlite3_file_control() */
	FULL       ErrorCode = 13 /* Insertion failed because database is full */
	CANTOPEN   ErrorCode = 14 /* Unable to open the database file */
	PROTOCOL   ErrorCode = 15 /* Database lock protocol error */
	EMPTY      ErrorCode = 16 /* Internal use only */
	SCHEMA     ErrorCode = 17 /* The database schema changed */
	TOOBIG     ErrorCode = 18 /* String or BLOB exceeds size limit */
	CONSTRAINT ErrorCode = 19 /* Abort due to constraint violation */
	MISMATCH   ErrorCode = 20 /* Data type mismatch */
	MISUSE     ErrorCode = 21 /* Library used incorrectly */
	NOLFS      ErrorCode = 22 /* Uses OS features not supported on host */
	AUTH       ErrorCode = 23 /* Authorization denied */
	FORMAT     ErrorCode = 24 /* Not used */
	RANGE      ErrorCode = 25 /* 2nd parameter to sqlite3_bind out of range */
	NOTADB     ErrorCode = 26 /* File opened that is not a database file */
	NOTICE     ErrorCode = 27 /* Notifications from sqlite3_log() */
	WARNING    ErrorCode = 28 /* Warnings from sqlite3_log() */
)

// ExtendedErrorCode is a result code that [Error.ExtendedCode] might return.
//
// https://sqlite.org/rescode.html
type (
	ExtendedErrorCode uint16
	xErrorCode        = ExtendedErrorCode
)

const (
	ERROR_MISSING_COLLSEQ   ExtendedErrorCode = xErrorCode(ERROR) | (1 << 8)
	ERROR_RETRY             ExtendedErrorCode = xErrorCode(ERROR) | (2 << 8)
	ERROR_SNAPSHOT          ExtendedErrorCode = xErrorCode(ERROR) | (3 << 8)
	IOERR_READ              ExtendedErrorCode = xErrorCode(IOERR) | (1 << 8)
	IOERR_SHORT_READ        ExtendedErrorCode = xErrorCode(IOERR) | (2 << 8)
	IOERR_WRITE             ExtendedErrorCode = xErrorCode(IOERR) | (3 << 8)
	IOERR_FSYNC             ExtendedErrorCode = xErrorCode(IOERR) | (4 << 8)
	IOERR_DIR_FSYNC         ExtendedErrorCode = xErrorCode(IOERR) | (5 << 8)
	IOERR_TRUNCATE          ExtendedErrorCode = xErrorCode(IOERR) | (6 << 8)
	IOERR_FSTAT             ExtendedErrorCode = xErrorCode(IOERR) | (7 << 8)
	IOERR_UNLOCK            ExtendedErrorCode = xErrorCode(IOERR) | (8 << 8)
	IOERR_RDLOCK            ExtendedErrorCode = xErrorCode(IOERR) | (9 << 8)
	IOERR_DELETE            ExtendedErrorCode = xErrorCode(IOERR) | (10 << 8)
	IOERR_BLOCKED           ExtendedErrorCode = xErrorCode(IOERR) | (11 << 8)
	IOERR_NOMEM             ExtendedErrorCode = xErrorCode(IOERR) | (12 << 8)
	IOERR_ACCESS            ExtendedErrorCode = xErrorCode(IOERR) | (13 << 8)
	IOERR_CHECKRESERVEDLOCK ExtendedErrorCode = xErrorCode(IOERR) | (14 << 8)
	IOERR_LOCK              ExtendedErrorCode = xErrorCode(IOERR) | (15 << 8)
	IOERR_CLOSE             ExtendedErrorCode = xErrorCode(IOERR) | (16 << 8)
	IOERR_DIR_CLOSE         ExtendedErrorCode = xErrorCode(IOERR) | (17 << 8)
	IOERR_SHMOPEN           ExtendedErrorCode = xErrorCode(IOERR) | (18 << 8)
	IOERR_SHMSIZE           ExtendedErrorCode = xErrorCode(IOERR) | (19 << 8)
	IOERR_SHMLOCK           ExtendedErrorCode = xErrorCode(IOERR) | (20 << 8)
	IOERR_SHMMAP            ExtendedErrorCode = xErrorCode(IOERR) | (21 << 8)
	IOERR_SEEK              ExtendedErrorCode = xErrorCode(IOERR) | (22 << 8)
	IOERR_DELETE_NOENT      ExtendedErrorCode = xErrorCode(IOERR) | (23 << 8)
	IOERR_MMAP              ExtendedErrorCode = xErrorCode(IOERR) | (24 << 8)
	IOERR_GETTEMPPATH       ExtendedErrorCode = xErrorCode(IOERR) | (25 << 8)
	IOERR_CONVPATH          ExtendedErrorCode = xErrorCode(IOERR) | (26 << 8)
	IOERR_VNODE             ExtendedErrorCode = xErrorCode(IOERR) | (27 << 8)
	IOERR_AUTH              ExtendedErrorCode = xErrorCode(IOERR) | (28 << 8)
	IOERR_BEGIN_ATOMIC      ExtendedErrorCode = xErrorCode(IOERR) | (29 << 8)
	IOERR_COMMIT_ATOMIC     ExtendedErrorCode = xErrorCode(IOERR) | (30 << 8)
	IOERR_ROLLBACK_ATOMIC   ExtendedErrorCode = xErrorCode(IOERR) | (31 << 8)
	IOERR_DATA              ExtendedErrorCode = xErrorCode(IOERR) | (32 << 8)
	IOERR_CORRUPTFS         ExtendedErrorCode = xErrorCode(IOERR) | (33 << 8)
	IOERR_IN_PAGE           ExtendedErrorCode = xErrorCode(IOERR) | (34 << 8)
	LOCKED_SHAREDCACHE      ExtendedErrorCode = xErrorCode(LOCKED) | (1 << 8)
	LOCKED_VTAB             ExtendedErrorCode = xErrorCode(LOCKED) | (2 << 8)
	BUSY_RECOVERY           ExtendedErrorCode = xErrorCode(BUSY) | (1 << 8)
	BUSY_SNAPSHOT           ExtendedErrorCode = xErrorCode(BUSY) | (2 << 8)
	BUSY_TIMEOUT            ExtendedErrorCode = xErrorCode(BUSY) | (3 << 8)
	CANTOPEN_NOTEMPDIR      ExtendedErrorCode = xErrorCode(CANTOPEN) | (1 << 8)
	CANTOPEN_ISDIR          ExtendedErrorCode = xErrorCode(CANTOPEN) | (2 << 8)
	CANTOPEN_FULLPATH       ExtendedErrorCode = xErrorCode(CANTOPEN) | (3 << 8)
	CANTOPEN_CONVPATH       ExtendedErrorCode = xErrorCode(CANTOPEN) | (4 << 8)
	// CANTOPEN_DIRTYWAL    ExtendedErrorCode = xErrorCode(CANTOPEN) | (5 << 8) /* Not Used */
	CANTOPEN_SYMLINK        ExtendedErrorCode = xErrorCode(CANTOPEN) | (6 << 8)
	CORRUPT_VTAB            ExtendedErrorCode = xErrorCode(CORRUPT) | (1 << 8)
	CORRUPT_SEQUENCE        ExtendedErrorCode = xErrorCode(CORRUPT) | (2 << 8)
	CORRUPT_INDEX           ExtendedErrorCode = xErrorCode(CORRUPT) | (3 << 8)
	READONLY_RECOVERY       ExtendedErrorCode = xErrorCode(READONLY) | (1 << 8)
	READONLY_CANTLOCK       ExtendedErrorCode = xErrorCode(READONLY) | (2 << 8)
	READONLY_ROLLBACK       ExtendedErrorCode = xErrorCode(READONLY) | (3 << 8)
	READONLY_DBMOVED        ExtendedErrorCode = xErrorCode(READONLY) | (4 << 8)
	READONLY_CANTINIT       ExtendedErrorCode = xErrorCode(READONLY) | (5 << 8)
	READONLY_DIRECTORY      ExtendedErrorCode = xErrorCode(READONLY) | (6 << 8)
	ABORT_ROLLBACK          ExtendedErrorCode = xErrorCode(ABORT) | (2 << 8)
	CONSTRAINT_CHECK        ExtendedErrorCode = xErrorCode(CONSTRAINT) | (1 << 8)
	CONSTRAINT_COMMITHOOK   ExtendedErrorCode = xErrorCode(CONSTRAINT) | (2 << 8)
	CONSTRAINT_FOREIGNKEY   ExtendedErrorCode = xErrorCode(CONSTRAINT) | (3 << 8)
	CONSTRAINT_FUNCTION     ExtendedErrorCode = xErrorCode(CONSTRAINT) | (4 << 8)
	CONSTRAINT_NOTNULL      ExtendedErrorCode = xErrorCode(CONSTRAINT) | (5 << 8)
	CONSTRAINT_PRIMARYKEY   ExtendedErrorCode = xErrorCode(CONSTRAINT) | (6 << 8)
	CONSTRAINT_TRIGGER      ExtendedErrorCode = xErrorCode(CONSTRAINT) | (7 << 8)
	CONSTRAINT_UNIQUE       ExtendedErrorCode = xErrorCode(CONSTRAINT) | (8 << 8)
	CONSTRAINT_VTAB         ExtendedErrorCode = xErrorCode(CONSTRAINT) | (9 << 8)
	CONSTRAINT_ROWID        ExtendedErrorCode = xErrorCode(CONSTRAINT) | (10 << 8)
	CONSTRAINT_PINNED       ExtendedErrorCode = xErrorCode(CONSTRAINT) | (11 << 8)
	CONSTRAINT_DATATYPE     ExtendedErrorCode = xErrorCode(CONSTRAINT) | (12 << 8)
	NOTICE_RECOVER_WAL      ExtendedErrorCode = xErrorCode(NOTICE) | (1 << 8)
	NOTICE_RECOVER_ROLLBACK ExtendedErrorCode = xErrorCode(NOTICE) | (2 << 8)
	NOTICE_RBU              ExtendedErrorCode = xErrorCode(NOTICE) | (3 << 8)
	WARNING_AUTOINDEX       ExtendedErrorCode = xErrorCode(WARNING) | (1 << 8)
	AUTH_USER               ExtendedErrorCode = xErrorCode(AUTH) | (1 << 8)
)

// OpenFlag is a flag for the [OpenFlags] function.
//
// https://sqlite.org/c3ref/c_open_autoproxy.html
type OpenFlag uint32

const (
	OPEN_READONLY     OpenFlag = 0x00000001 /* Ok for sqlite3_open_v2() */
	OPEN_READWRITE    OpenFlag = 0x00000002 /* Ok for sqlite3_open_v2() */
	OPEN_CREATE       OpenFlag = 0x00000004 /* Ok for sqlite3_open_v2() */
	OPEN_URI          OpenFlag = 0x00000040 /* Ok for sqlite3_open_v2() */
	OPEN_MEMORY       OpenFlag = 0x00000080 /* Ok for sqlite3_open_v2() */
	OPEN_NOMUTEX      OpenFlag = 0x00008000 /* Ok for sqlite3_open_v2() */
	OPEN_FULLMUTEX    OpenFlag = 0x00010000 /* Ok for sqlite3_open_v2() */
	OPEN_SHAREDCACHE  OpenFlag = 0x00020000 /* Ok for sqlite3_open_v2() */
	OPEN_PRIVATECACHE OpenFlag = 0x00040000 /* Ok for sqlite3_open_v2() */
	OPEN_NOFOLLOW     OpenFlag = 0x01000000 /* Ok for sqlite3_open_v2() */
	OPEN_EXRESCODE    OpenFlag = 0x02000000 /* Extended result codes */
)

// PrepareFlag is a flag that can be passed to [Conn.PrepareFlags].
//
// https://sqlite.org/c3ref/c_prepare_normalize.html
type PrepareFlag uint32

const (
	PREPARE_PERSISTENT PrepareFlag = 0x01
	PREPARE_NORMALIZE  PrepareFlag = 0x02
	PREPARE_NO_VTAB    PrepareFlag = 0x04
	PREPARE_DONT_LOG   PrepareFlag = 0x10
)

// FunctionFlag is a flag that can be passed to
// [Conn.CreateFunction] and [Conn.CreateWindowFunction].
//
// https://sqlite.org/c3ref/c_deterministic.html
type FunctionFlag uint32

const (
	DETERMINISTIC FunctionFlag = 0x000000800
	DIRECTONLY    FunctionFlag = 0x000080000
	INNOCUOUS     FunctionFlag = 0x000200000
	SELFORDER1    FunctionFlag = 0x002000000
	// SUBTYPE        FunctionFlag = 0x000100000
	// RESULT_SUBTYPE FunctionFlag = 0x001000000
)

// StmtStatus name counter values associated with the [Stmt.Status] method.
//
// https://sqlite.org/c3ref/c_stmtstatus_counter.html
type StmtStatus uint32

const (
	STMTSTATUS_FULLSCAN_STEP StmtStatus = 1
	STMTSTATUS_SORT          StmtStatus = 2
	STMTSTATUS_AUTOINDEX     StmtStatus = 3
	STMTSTATUS_VM_STEP       StmtStatus = 4
	STMTSTATUS_REPREPARE     StmtStatus = 5
	STMTSTATUS_RUN           StmtStatus = 6
	STMTSTATUS_FILTER_MISS   StmtStatus = 7
	STMTSTATUS_FILTER_HIT    StmtStatus = 8
	STMTSTATUS_MEMUSED       StmtStatus = 99
)

// DBStatus are the available "verbs" that can be passed to the [Conn.Status] method.
//
// https://sqlite.org/c3ref/c_dbstatus_options.html
type DBStatus uint32

const (
	DBSTATUS_LOOKASIDE_USED      DBStatus = 0
	DBSTATUS_CACHE_USED          DBStatus = 1
	DBSTATUS_SCHEMA_USED         DBStatus = 2
	DBSTATUS_STMT_USED           DBStatus = 3
	DBSTATUS_LOOKASIDE_HIT       DBStatus = 4
	DBSTATUS_LOOKASIDE_MISS_SIZE DBStatus = 5
	DBSTATUS_LOOKASIDE_MISS_FULL DBStatus = 6
	DBSTATUS_CACHE_HIT           DBStatus = 7
	DBSTATUS_CACHE_MISS          DBStatus = 8
	DBSTATUS_CACHE_WRITE         DBStatus = 9
	DBSTATUS_DEFERRED_FKS        DBStatus = 10
	DBSTATUS_CACHE_USED_SHARED   DBStatus = 11
	DBSTATUS_CACHE_SPILL         DBStatus = 12
	// DBSTATUS_MAX              DBStatus = 12
)

// DBConfig are the available database connection configuration options.
//
// https://sqlite.org/c3ref/c_dbconfig_defensive.html
type DBConfig uint32

const (
	// DBCONFIG_MAINDBNAME         DBConfig = 1000
	// DBCONFIG_LOOKASIDE          DBConfig = 1001
	DBCONFIG_ENABLE_FKEY           DBConfig = 1002
	DBCONFIG_ENABLE_TRIGGER        DBConfig = 1003
	DBCONFIG_ENABLE_FTS3_TOKENIZER DBConfig = 1004
	DBCONFIG_ENABLE_LOAD_EXTENSION DBConfig = 1005
	DBCONFIG_NO_CKPT_ON_CLOSE      DBConfig = 1006
	DBCONFIG_ENABLE_QPSG           DBConfig = 1007
	DBCONFIG_TRIGGER_EQP           DBConfig = 1008
	DBCONFIG_RESET_DATABASE        DBConfig = 1009
	DBCONFIG_DEFENSIVE             DBConfig = 1010
	DBCONFIG_WRITABLE_SCHEMA       DBConfig = 1011
	DBCONFIG_LEGACY_ALTER_TABLE    DBConfig = 1012
	DBCONFIG_DQS_DML               DBConfig = 1013
	DBCONFIG_DQS_DDL               DBConfig = 1014
	DBCONFIG_ENABLE_VIEW           DBConfig = 1015
	DBCONFIG_LEGACY_FILE_FORMAT    DBConfig = 1016
	DBCONFIG_TRUSTED_SCHEMA        DBConfig = 1017
	DBCONFIG_STMT_SCANSTATUS       DBConfig = 1018
	DBCONFIG_REVERSE_SCANORDER     DBConfig = 1019
	DBCONFIG_ENABLE_ATTACH_CREATE  DBConfig = 1020
	DBCONFIG_ENABLE_ATTACH_WRITE   DBConfig = 1021
	DBCONFIG_ENABLE_COMMENTS       DBConfig = 1022
	// DBCONFIG_MAX                DBConfig = 1022
)

// FcntlOpcode are the available opcodes for [Conn.FileControl].
//
// https://sqlite.org/c3ref/c_fcntl_begin_atomic_write.html
type FcntlOpcode uint32

const (
	FCNTL_LOCKSTATE           FcntlOpcode = 1
	FCNTL_CHUNK_SIZE          FcntlOpcode = 6
	FCNTL_FILE_POINTER        FcntlOpcode = 7
	FCNTL_PERSIST_WAL         FcntlOpcode = 10
	FCNTL_POWERSAFE_OVERWRITE FcntlOpcode = 13
	FCNTL_VFS_POINTER         FcntlOpcode = 27
	FCNTL_JOURNAL_POINTER     FcntlOpcode = 28
	FCNTL_DATA_VERSION        FcntlOpcode = 35
	FCNTL_RESERVE_BYTES       FcntlOpcode = 38
	FCNTL_RESET_CACHE         FcntlOpcode = 42
)

// LimitCategory are the available run-time limit categories.
//
// https://sqlite.org/c3ref/c_limit_attached.html
type LimitCategory uint32

const (
	LIMIT_LENGTH              LimitCategory = 0
	LIMIT_SQL_LENGTH          LimitCategory = 1
	LIMIT_COLUMN              LimitCategory = 2
	LIMIT_EXPR_DEPTH          LimitCategory = 3
	LIMIT_COMPOUND_SELECT     LimitCategory = 4
	LIMIT_VDBE_OP             LimitCategory = 5
	LIMIT_FUNCTION_ARG        LimitCategory = 6
	LIMIT_ATTACHED            LimitCategory = 7
	LIMIT_LIKE_PATTERN_LENGTH LimitCategory = 8
	LIMIT_VARIABLE_NUMBER     LimitCategory = 9
	LIMIT_TRIGGER_DEPTH       LimitCategory = 10
	LIMIT_WORKER_THREADS      LimitCategory = 11
)

// AuthorizerActionCode are the integer action codes
// that the authorizer callback may be passed.
//
// https://sqlite.org/c3ref/c_alter_table.html
type AuthorizerActionCode uint32

const (
	/***************************************************** 3rd ************ 4th ***********/
	AUTH_CREATE_INDEX        AuthorizerActionCode = 1  /* Index Name      Table Name      */
	AUTH_CREATE_TABLE        AuthorizerActionCode = 2  /* Table Name      NULL            */
	AUTH_CREATE_TEMP_INDEX   AuthorizerActionCode = 3  /* Index Name      Table Name      */
	AUTH_CREATE_TEMP_TABLE   AuthorizerActionCode = 4  /* Table Name      NULL            */
	AUTH_CREATE_TEMP_TRIGGER AuthorizerActionCode = 5  /* Trigger Name    Table Name      */
	AUTH_CREATE_TEMP_VIEW    AuthorizerActionCode = 6  /* View Name       NULL            */
	AUTH_CREATE_TRIGGER      AuthorizerActionCode = 7  /* Trigger Name    Table Name      */
	AUTH_CREATE_VIEW         AuthorizerActionCode = 8  /* View Name       NULL            */
	AUTH_DELETE              AuthorizerActionCode = 9  /* Table Name      NULL            */
	AUTH_DROP_INDEX          AuthorizerActionCode = 10 /* Index Name      Table Name      */
	AUTH_DROP_TABLE          AuthorizerActionCode = 11 /* Table Name      NULL            */
	AUTH_DROP_TEMP_INDEX     AuthorizerActionCode = 12 /* Index Name      Table Name      */
	AUTH_DROP_TEMP_TABLE     AuthorizerActionCode = 13 /* Table Name      NULL            */
	AUTH_DROP_TEMP_TRIGGER   AuthorizerActionCode = 14 /* Trigger Name    Table Name      */
	AUTH_DROP_TEMP_VIEW      AuthorizerActionCode = 15 /* View Name       NULL            */
	AUTH_DROP_TRIGGER        AuthorizerActionCode = 16 /* Trigger Name    Table Name      */
	AUTH_DROP_VIEW           AuthorizerActionCode = 17 /* View Name       NULL            */
	AUTH_INSERT              AuthorizerActionCode = 18 /* Table Name      NULL            */
	AUTH_PRAGMA              AuthorizerActionCode = 19 /* Pragma Name     1st arg or NULL */
	AUTH_READ                AuthorizerActionCode = 20 /* Table Name      Column Name     */
	AUTH_SELECT              AuthorizerActionCode = 21 /* NULL            NULL            */
	AUTH_TRANSACTION         AuthorizerActionCode = 22 /* Operation       NULL            */
	AUTH_UPDATE              AuthorizerActionCode = 23 /* Table Name      Column Name     */
	AUTH_ATTACH              AuthorizerActionCode = 24 /* Filename        NULL            */
	AUTH_DETACH              AuthorizerActionCode = 25 /* Database Name   NULL            */
	AUTH_ALTER_TABLE         AuthorizerActionCode = 26 /* Database Name   Table Name      */
	AUTH_REINDEX             AuthorizerActionCode = 27 /* Index Name      NULL            */
	AUTH_ANALYZE             AuthorizerActionCode = 28 /* Table Name      NULL            */
	AUTH_CREATE_VTABLE       AuthorizerActionCode = 29 /* Table Name      Module Name     */
	AUTH_DROP_VTABLE         AuthorizerActionCode = 30 /* Table Name      Module Name     */
	AUTH_FUNCTION            AuthorizerActionCode = 31 /* NULL            Function Name   */
	AUTH_SAVEPOINT           AuthorizerActionCode = 32 /* Operation       Savepoint Name  */
	AUTH_RECURSIVE           AuthorizerActionCode = 33 /* NULL            NULL            */
	// AUTH_COPY             AuthorizerActionCode = 0  /* No longer used */
)

// AuthorizerReturnCode are the integer codes
// that the authorizer callback may return.
//
// https://sqlite.org/c3ref/c_deny.html
type AuthorizerReturnCode uint32

const (
	AUTH_OK     AuthorizerReturnCode = 0
	AUTH_DENY   AuthorizerReturnCode = 1 /* Abort the SQL statement with an error */
	AUTH_IGNORE AuthorizerReturnCode = 2 /* Don't allow access, but don't generate an error */
)

// CheckpointMode are all the checkpoint mode values.
//
// https://sqlite.org/c3ref/c_checkpoint_full.html
type CheckpointMode uint32

const (
	CHECKPOINT_PASSIVE  CheckpointMode = 0 /* Do as much as possible w/o blocking */
	CHECKPOINT_FULL     CheckpointMode = 1 /* Wait for writers, then checkpoint */
	CHECKPOINT_RESTART  CheckpointMode = 2 /* Like FULL but wait for readers */
	CHECKPOINT_TRUNCATE CheckpointMode = 3 /* Like RESTART but also truncate WAL */
)

// TxnState are the allowed return values from [Conn.TxnState].
//
// https://sqlite.org/c3ref/c_txn_none.html
type TxnState uint32

const (
	TXN_NONE  TxnState = 0
	TXN_READ  TxnState = 1
	TXN_WRITE TxnState = 2
)

// TraceEvent identify classes of events that can be monitored with [Conn.Trace].
//
// https://sqlite.org/c3ref/c_trace.html
type TraceEvent uint32

const (
	TRACE_STMT    TraceEvent = 0x01
	TRACE_PROFILE TraceEvent = 0x02
	TRACE_ROW     TraceEvent = 0x04
	TRACE_CLOSE   TraceEvent = 0x08
)

// Datatype is a fundamental datatype of SQLite.
//
// https://sqlite.org/c3ref/c_blob.html
type Datatype uint32

const (
	INTEGER Datatype = 1
	FLOAT   Datatype = 2
	TEXT    Datatype = 3
	BLOB    Datatype = 4
	NULL    Datatype = 5
)

// String implements the [fmt.Stringer] interface.
func (t Datatype) String() string {
	const name = "INTEGERFLOATEXTBLOBNULL"
	switch t {
	case INTEGER:
		return name[0:7]
	case FLOAT:
		return name[7:12]
	case TEXT:
		return name[11:15]
	case BLOB:
		return name[15:19]
	case NULL:
		return name[19:23]
	}
	return strconv.FormatUint(uint64(t), 10)
}
