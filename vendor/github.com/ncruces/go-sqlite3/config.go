package sqlite3

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"

	"github.com/tetratelabs/wazero/api"

	"github.com/ncruces/go-sqlite3/internal/util"
	"github.com/ncruces/go-sqlite3/vfs"
)

// Config makes configuration changes to a database connection.
// Only boolean configuration options are supported.
// Called with no arg reads the current configuration value,
// called with one arg sets and returns the new value.
//
// https://sqlite.org/c3ref/db_config.html
func (c *Conn) Config(op DBConfig, arg ...bool) (bool, error) {
	if op < DBCONFIG_ENABLE_FKEY || op > DBCONFIG_REVERSE_SCANORDER {
		return false, MISUSE
	}

	// We need to call sqlite3_db_config, a variadic function.
	// We only support the `int int*` variants.
	// The int is a three-valued bool: -1 queries, 0/1 sets false/true.
	// The int* points to where new state will be written to.
	// The vararg is a pointer to an array containing these arguments:
	// an int and an int* pointing to that int.

	defer c.arena.mark()()
	argsPtr := c.arena.new(intlen + ptrlen)

	var flag int32
	switch {
	case len(arg) == 0:
		flag = -1
	case arg[0]:
		flag = 1
	}

	util.Write32(c.mod, argsPtr+0*ptrlen, flag)
	util.Write32(c.mod, argsPtr+1*ptrlen, argsPtr)

	rc := res_t(c.call("sqlite3_db_config", stk_t(c.handle),
		stk_t(op), stk_t(argsPtr)))
	return util.ReadBool(c.mod, argsPtr), c.error(rc)
}

var defaultLogger atomic.Pointer[func(code ExtendedErrorCode, msg string)]

// ConfigLog sets up the default error logging callback for new connections.
//
// https://sqlite.org/errlog.html
func ConfigLog(cb func(code ExtendedErrorCode, msg string)) {
	defaultLogger.Store(&cb)
}

// ConfigLog sets up the error logging callback for the connection.
//
// https://sqlite.org/errlog.html
func (c *Conn) ConfigLog(cb func(code ExtendedErrorCode, msg string)) error {
	var enable int32
	if cb != nil {
		enable = 1
	}
	rc := res_t(c.call("sqlite3_config_log_go", stk_t(enable)))
	if err := c.error(rc); err != nil {
		return err
	}
	c.log = cb
	return nil
}

func logCallback(ctx context.Context, mod api.Module, _ ptr_t, iCode res_t, zMsg ptr_t) {
	if c, ok := ctx.Value(connKey{}).(*Conn); ok && c.log != nil {
		msg := util.ReadString(mod, zMsg, _MAX_LENGTH)
		c.log(xErrorCode(iCode), msg)
	}
}

// Log writes a message into the error log established by [Conn.ConfigLog].
//
// https://sqlite.org/c3ref/log.html
func (c *Conn) Log(code ExtendedErrorCode, format string, a ...any) {
	if c.log != nil {
		c.log(code, fmt.Sprintf(format, a...))
	}
}

// FileControl allows low-level control of database files.
// Only a subset of opcodes are supported.
//
// https://sqlite.org/c3ref/file_control.html
func (c *Conn) FileControl(schema string, op FcntlOpcode, arg ...any) (any, error) {
	defer c.arena.mark()()
	ptr := c.arena.new(max(ptrlen, intlen))

	var schemaPtr ptr_t
	if schema != "" {
		schemaPtr = c.arena.string(schema)
	}

	var rc res_t
	var ret any
	switch op {
	default:
		return nil, MISUSE

	case FCNTL_RESET_CACHE:
		rc = res_t(c.call("sqlite3_file_control",
			stk_t(c.handle), stk_t(schemaPtr),
			stk_t(op), 0))

	case FCNTL_PERSIST_WAL, FCNTL_POWERSAFE_OVERWRITE:
		var flag int32
		switch {
		case len(arg) == 0:
			flag = -1
		case arg[0]:
			flag = 1
		}
		util.Write32(c.mod, ptr, flag)
		rc = res_t(c.call("sqlite3_file_control",
			stk_t(c.handle), stk_t(schemaPtr),
			stk_t(op), stk_t(ptr)))
		ret = util.ReadBool(c.mod, ptr)

	case FCNTL_CHUNK_SIZE:
		util.Write32(c.mod, ptr, int32(arg[0].(int)))
		rc = res_t(c.call("sqlite3_file_control",
			stk_t(c.handle), stk_t(schemaPtr),
			stk_t(op), stk_t(ptr)))

	case FCNTL_RESERVE_BYTES:
		bytes := -1
		if len(arg) > 0 {
			bytes = arg[0].(int)
		}
		util.Write32(c.mod, ptr, int32(bytes))
		rc = res_t(c.call("sqlite3_file_control",
			stk_t(c.handle), stk_t(schemaPtr),
			stk_t(op), stk_t(ptr)))
		ret = int(util.Read32[int32](c.mod, ptr))

	case FCNTL_DATA_VERSION:
		rc = res_t(c.call("sqlite3_file_control",
			stk_t(c.handle), stk_t(schemaPtr),
			stk_t(op), stk_t(ptr)))
		ret = util.Read32[uint32](c.mod, ptr)

	case FCNTL_LOCKSTATE:
		rc = res_t(c.call("sqlite3_file_control",
			stk_t(c.handle), stk_t(schemaPtr),
			stk_t(op), stk_t(ptr)))
		ret = util.Read32[vfs.LockLevel](c.mod, ptr)

	case FCNTL_VFS_POINTER:
		rc = res_t(c.call("sqlite3_file_control",
			stk_t(c.handle), stk_t(schemaPtr),
			stk_t(op), stk_t(ptr)))
		if rc == _OK {
			const zNameOffset = 16
			ptr = util.Read32[ptr_t](c.mod, ptr)
			ptr = util.Read32[ptr_t](c.mod, ptr+zNameOffset)
			name := util.ReadString(c.mod, ptr, _MAX_NAME)
			ret = vfs.Find(name)
		}

	case FCNTL_FILE_POINTER, FCNTL_JOURNAL_POINTER:
		rc = res_t(c.call("sqlite3_file_control",
			stk_t(c.handle), stk_t(schemaPtr),
			stk_t(op), stk_t(ptr)))
		if rc == _OK {
			const fileHandleOffset = 4
			ptr = util.Read32[ptr_t](c.mod, ptr)
			ptr = util.Read32[ptr_t](c.mod, ptr+fileHandleOffset)
			ret = util.GetHandle(c.ctx, ptr)
		}
	}

	if err := c.error(rc); err != nil {
		return nil, err
	}
	return ret, nil
}

// Limit allows the size of various constructs to be
// limited on a connection by connection basis.
//
// https://sqlite.org/c3ref/limit.html
func (c *Conn) Limit(id LimitCategory, value int) int {
	v := int32(c.call("sqlite3_limit", stk_t(c.handle), stk_t(id), stk_t(value)))
	return int(v)
}

// SetAuthorizer registers an authorizer callback with the database connection.
//
// https://sqlite.org/c3ref/set_authorizer.html
func (c *Conn) SetAuthorizer(cb func(action AuthorizerActionCode, name3rd, name4th, schema, inner string) AuthorizerReturnCode) error {
	var enable int32
	if cb != nil {
		enable = 1
	}
	rc := res_t(c.call("sqlite3_set_authorizer_go", stk_t(c.handle), stk_t(enable)))
	if err := c.error(rc); err != nil {
		return err
	}
	c.authorizer = cb
	return nil

}

func authorizerCallback(ctx context.Context, mod api.Module, pDB ptr_t, action AuthorizerActionCode, zName3rd, zName4th, zSchema, zInner ptr_t) (rc AuthorizerReturnCode) {
	if c, ok := ctx.Value(connKey{}).(*Conn); ok && c.handle == pDB && c.authorizer != nil {
		var name3rd, name4th, schema, inner string
		if zName3rd != 0 {
			name3rd = util.ReadString(mod, zName3rd, _MAX_NAME)
		}
		if zName4th != 0 {
			name4th = util.ReadString(mod, zName4th, _MAX_NAME)
		}
		if zSchema != 0 {
			schema = util.ReadString(mod, zSchema, _MAX_NAME)
		}
		if zInner != 0 {
			inner = util.ReadString(mod, zInner, _MAX_NAME)
		}
		rc = c.authorizer(action, name3rd, name4th, schema, inner)
	}
	return rc
}

// Trace registers a trace callback function against the database connection.
//
// https://sqlite.org/c3ref/trace_v2.html
func (c *Conn) Trace(mask TraceEvent, cb func(evt TraceEvent, arg1 any, arg2 any) error) error {
	rc := res_t(c.call("sqlite3_trace_go", stk_t(c.handle), stk_t(mask)))
	if err := c.error(rc); err != nil {
		return err
	}
	c.trace = cb
	return nil
}

func traceCallback(ctx context.Context, mod api.Module, evt TraceEvent, pDB, pArg1, pArg2 ptr_t) (rc res_t) {
	if c, ok := ctx.Value(connKey{}).(*Conn); ok && c.handle == pDB && c.trace != nil {
		var arg1, arg2 any
		if evt == TRACE_CLOSE {
			arg1 = c
		} else {
			for _, s := range c.stmts {
				if pArg1 == s.handle {
					arg1 = s
					switch evt {
					case TRACE_STMT:
						arg2 = s.SQL()
					case TRACE_PROFILE:
						arg2 = util.Read64[int64](mod, pArg2)
					}
					break
				}
			}
		}
		if arg1 != nil {
			_, rc = errorCode(c.trace(evt, arg1, arg2), ERROR)
		}
	}
	return rc
}

// WALCheckpoint checkpoints a WAL database.
//
// https://sqlite.org/c3ref/wal_checkpoint_v2.html
func (c *Conn) WALCheckpoint(schema string, mode CheckpointMode) (nLog, nCkpt int, err error) {
	if c.interrupt.Err() != nil {
		return 0, 0, INTERRUPT
	}

	defer c.arena.mark()()
	nLogPtr := c.arena.new(ptrlen)
	nCkptPtr := c.arena.new(ptrlen)
	schemaPtr := c.arena.string(schema)
	rc := res_t(c.call("sqlite3_wal_checkpoint_v2",
		stk_t(c.handle), stk_t(schemaPtr), stk_t(mode),
		stk_t(nLogPtr), stk_t(nCkptPtr)))
	nLog = int(util.Read32[int32](c.mod, nLogPtr))
	nCkpt = int(util.Read32[int32](c.mod, nCkptPtr))
	return nLog, nCkpt, c.error(rc)
}

// WALAutoCheckpoint configures WAL auto-checkpoints.
//
// https://sqlite.org/c3ref/wal_autocheckpoint.html
func (c *Conn) WALAutoCheckpoint(pages int) error {
	rc := res_t(c.call("sqlite3_wal_autocheckpoint", stk_t(c.handle), stk_t(pages)))
	return c.error(rc)
}

// WALHook registers a callback function to be invoked
// each time data is committed to a database in WAL mode.
//
// https://sqlite.org/c3ref/wal_hook.html
func (c *Conn) WALHook(cb func(db *Conn, schema string, pages int) error) {
	var enable int32
	if cb != nil {
		enable = 1
	}
	c.call("sqlite3_wal_hook_go", stk_t(c.handle), stk_t(enable))
	c.wal = cb
}

func walCallback(ctx context.Context, mod api.Module, _, pDB, zSchema ptr_t, pages int32) (rc res_t) {
	if c, ok := ctx.Value(connKey{}).(*Conn); ok && c.handle == pDB && c.wal != nil {
		schema := util.ReadString(mod, zSchema, _MAX_NAME)
		err := c.wal(c, schema, int(pages))
		_, rc = errorCode(err, ERROR)
	}
	return rc
}

// AutoVacuumPages registers a autovacuum compaction amount callback.
//
// https://sqlite.org/c3ref/autovacuum_pages.html
func (c *Conn) AutoVacuumPages(cb func(schema string, dbPages, freePages, bytesPerPage uint) uint) error {
	var funcPtr ptr_t
	if cb != nil {
		funcPtr = util.AddHandle(c.ctx, cb)
	}
	rc := res_t(c.call("sqlite3_autovacuum_pages_go", stk_t(c.handle), stk_t(funcPtr)))
	return c.error(rc)
}

func autoVacuumCallback(ctx context.Context, mod api.Module, pApp, zSchema ptr_t, nDbPage, nFreePage, nBytePerPage uint32) uint32 {
	fn := util.GetHandle(ctx, pApp).(func(schema string, dbPages, freePages, bytesPerPage uint) uint)
	schema := util.ReadString(mod, zSchema, _MAX_NAME)
	return uint32(fn(schema, uint(nDbPage), uint(nFreePage), uint(nBytePerPage)))
}

// SoftHeapLimit imposes a soft limit on heap size.
//
// https://sqlite.org/c3ref/hard_heap_limit64.html
func (c *Conn) SoftHeapLimit(n int64) int64 {
	return int64(c.call("sqlite3_soft_heap_limit64", stk_t(n)))
}

// HardHeapLimit imposes a hard limit on heap size.
//
// https://sqlite.org/c3ref/hard_heap_limit64.html
func (c *Conn) HardHeapLimit(n int64) int64 {
	return int64(c.call("sqlite3_hard_heap_limit64", stk_t(n)))
}

// EnableChecksums enables checksums on a database.
//
// https://sqlite.org/cksumvfs.html
func (c *Conn) EnableChecksums(schema string) error {
	r, err := c.FileControl(schema, FCNTL_RESERVE_BYTES)
	if err != nil {
		return err
	}
	if r == 8 {
		// Correct value, enabled.
		return nil
	}
	if r == 0 {
		// Default value, enable.
		_, err = c.FileControl(schema, FCNTL_RESERVE_BYTES, 8)
		if err != nil {
			return err
		}
		r, err = c.FileControl(schema, FCNTL_RESERVE_BYTES)
		if err != nil {
			return err
		}
	}
	if r != 8 {
		// Invalid value.
		return util.ErrorString("sqlite3: reserve bytes must be 8, is: " + strconv.Itoa(r.(int)))
	}

	// VACUUM the database.
	if schema != "" {
		err = c.Exec(`VACUUM ` + QuoteIdentifier(schema))
	} else {
		err = c.Exec(`VACUUM`)
	}
	if err != nil {
		return err
	}

	// Checkpoint the WAL.
	_, _, err = c.WALCheckpoint(schema, CHECKPOINT_FULL)
	return err
}
