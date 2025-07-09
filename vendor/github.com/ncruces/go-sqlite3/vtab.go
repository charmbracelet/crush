package sqlite3

import (
	"context"
	"errors"
	"reflect"

	"github.com/tetratelabs/wazero/api"

	"github.com/ncruces/go-sqlite3/internal/util"
)

// CreateModule registers a new virtual table module name.
// If create is nil, the virtual table is eponymous.
//
// https://sqlite.org/c3ref/create_module.html
func CreateModule[T VTab](db *Conn, name string, create, connect VTabConstructor[T]) error {
	var flags int

	const (
		VTAB_CREATOR     = 0x001
		VTAB_DESTROYER   = 0x002
		VTAB_UPDATER     = 0x004
		VTAB_RENAMER     = 0x008
		VTAB_OVERLOADER  = 0x010
		VTAB_CHECKER     = 0x020
		VTAB_TXN         = 0x040
		VTAB_SAVEPOINTER = 0x080
		VTAB_SHADOWTABS  = 0x100
	)

	if create != nil {
		flags |= VTAB_CREATOR
	}

	vtab := reflect.TypeOf(connect).Out(0)
	if implements[VTabDestroyer](vtab) {
		flags |= VTAB_DESTROYER
	}
	if implements[VTabUpdater](vtab) {
		flags |= VTAB_UPDATER
	}
	if implements[VTabRenamer](vtab) {
		flags |= VTAB_RENAMER
	}
	if implements[VTabOverloader](vtab) {
		flags |= VTAB_OVERLOADER
	}
	if implements[VTabChecker](vtab) {
		flags |= VTAB_CHECKER
	}
	if implements[VTabTxn](vtab) {
		flags |= VTAB_TXN
	}
	if implements[VTabSavepointer](vtab) {
		flags |= VTAB_SAVEPOINTER
	}
	if implements[VTabShadowTabler](vtab) {
		flags |= VTAB_SHADOWTABS
	}

	var modulePtr ptr_t
	defer db.arena.mark()()
	namePtr := db.arena.string(name)
	if connect != nil {
		modulePtr = util.AddHandle(db.ctx, module[T]{create, connect})
	}
	rc := res_t(db.call("sqlite3_create_module_go", stk_t(db.handle),
		stk_t(namePtr), stk_t(flags), stk_t(modulePtr)))
	return db.error(rc)
}

func implements[T any](typ reflect.Type) bool {
	var ptr *T
	return typ.Implements(reflect.TypeOf(ptr).Elem())
}

// DeclareVTab declares the schema of a virtual table.
//
// https://sqlite.org/c3ref/declare_vtab.html
func (c *Conn) DeclareVTab(sql string) error {
	if c.interrupt.Err() != nil {
		return INTERRUPT
	}
	defer c.arena.mark()()
	textPtr := c.arena.string(sql)
	rc := res_t(c.call("sqlite3_declare_vtab", stk_t(c.handle), stk_t(textPtr)))
	return c.error(rc)
}

// VTabConflictMode is a virtual table conflict resolution mode.
//
// https://sqlite.org/c3ref/c_fail.html
type VTabConflictMode uint8

const (
	VTAB_ROLLBACK VTabConflictMode = 1
	VTAB_IGNORE   VTabConflictMode = 2
	VTAB_FAIL     VTabConflictMode = 3
	VTAB_ABORT    VTabConflictMode = 4
	VTAB_REPLACE  VTabConflictMode = 5
)

// VTabOnConflict determines the virtual table conflict policy.
//
// https://sqlite.org/c3ref/vtab_on_conflict.html
func (c *Conn) VTabOnConflict() VTabConflictMode {
	return VTabConflictMode(c.call("sqlite3_vtab_on_conflict", stk_t(c.handle)))
}

// VTabConfigOption is a virtual table configuration option.
//
// https://sqlite.org/c3ref/c_vtab_constraint_support.html
type VTabConfigOption uint8

const (
	VTAB_CONSTRAINT_SUPPORT VTabConfigOption = 1
	VTAB_INNOCUOUS          VTabConfigOption = 2
	VTAB_DIRECTONLY         VTabConfigOption = 3
	VTAB_USES_ALL_SCHEMAS   VTabConfigOption = 4
)

// VTabConfig configures various facets of the virtual table interface.
//
// https://sqlite.org/c3ref/vtab_config.html
func (c *Conn) VTabConfig(op VTabConfigOption, args ...any) error {
	var i int32
	if op == VTAB_CONSTRAINT_SUPPORT && len(args) > 0 {
		if b, ok := args[0].(bool); ok && b {
			i = 1
		}
	}
	rc := res_t(c.call("sqlite3_vtab_config_go", stk_t(c.handle), stk_t(op), stk_t(i)))
	return c.error(rc)
}

// VTabConstructor is a virtual table constructor function.
type VTabConstructor[T VTab] func(db *Conn, module, schema, table string, arg ...string) (T, error)

type module[T VTab] [2]VTabConstructor[T]

type vtabConstructor int

const (
	xCreate  vtabConstructor = 0
	xConnect vtabConstructor = 1
)

// A VTab describes a particular instance of the virtual table.
// A VTab may optionally implement [io.Closer] to free resources.
//
// https://sqlite.org/c3ref/vtab.html
type VTab interface {
	// https://sqlite.org/vtab.html#xbestindex
	BestIndex(*IndexInfo) error
	// https://sqlite.org/vtab.html#xopen
	Open() (VTabCursor, error)
}

// A VTabDestroyer allows a virtual table to drop persistent state.
type VTabDestroyer interface {
	VTab
	// https://sqlite.org/vtab.html#sqlite3_module.xDestroy
	Destroy() error
}

// A VTabUpdater allows a virtual table to be updated.
// Implementations must not retain arg.
type VTabUpdater interface {
	VTab
	// https://sqlite.org/vtab.html#xupdate
	Update(arg ...Value) (rowid int64, err error)
}

// A VTabRenamer allows a virtual table to be renamed.
type VTabRenamer interface {
	VTab
	// https://sqlite.org/vtab.html#xrename
	Rename(new string) error
}

// A VTabOverloader allows a virtual table to overload SQL functions.
type VTabOverloader interface {
	VTab
	// https://sqlite.org/vtab.html#xfindfunction
	FindFunction(arg int, name string) (ScalarFunction, IndexConstraintOp)
}

// A VTabShadowTabler allows a virtual table to protect the content
// of shadow tables from being corrupted by hostile SQL.
//
// Implementing this interface signals that a virtual table named
// "mumble" reserves all table names starting with "mumble_".
type VTabShadowTabler interface {
	VTab
	// https://sqlite.org/vtab.html#the_xshadowname_method
	ShadowTables()
}

// A VTabChecker allows a virtual table to report errors
// to the PRAGMA integrity_check and PRAGMA quick_check commands.
//
// Integrity should return an error if it finds problems in the content of the virtual table,
// but should avoid returning a (wrapped) [Error], [ErrorCode] or [ExtendedErrorCode],
// as those indicate the Integrity method itself encountered problems
// while trying to evaluate the virtual table content.
type VTabChecker interface {
	VTab
	// https://sqlite.org/vtab.html#xintegrity
	Integrity(schema, table string, flags int) error
}

// A VTabTxn allows a virtual table to implement
// transactions with two-phase commit.
//
// Anything that is required as part of a commit that may fail
// should be performed in the Sync() callback.
// Current versions of SQLite ignore any errors
// returned by Commit() and Rollback().
type VTabTxn interface {
	VTab
	// https://sqlite.org/vtab.html#xBegin
	Begin() error
	// https://sqlite.org/vtab.html#xsync
	Sync() error
	// https://sqlite.org/vtab.html#xcommit
	Commit() error
	// https://sqlite.org/vtab.html#xrollback
	Rollback() error
}

// A VTabSavepointer allows a virtual table to implement
// nested transactions.
//
// https://sqlite.org/vtab.html#xsavepoint
type VTabSavepointer interface {
	VTabTxn
	Savepoint(id int) error
	Release(id int) error
	RollbackTo(id int) error
}

// A VTabCursor describes cursors that point
// into the virtual table and are used
// to loop through the virtual table.
// A VTabCursor may optionally implement
// [io.Closer] to free resources.
// Implementations of Filter must not retain arg.
//
// https://sqlite.org/c3ref/vtab_cursor.html
type VTabCursor interface {
	// https://sqlite.org/vtab.html#xfilter
	Filter(idxNum int, idxStr string, arg ...Value) error
	// https://sqlite.org/vtab.html#xnext
	Next() error
	// https://sqlite.org/vtab.html#xeof
	EOF() bool
	// https://sqlite.org/vtab.html#xcolumn
	Column(ctx Context, n int) error
	// https://sqlite.org/vtab.html#xrowid
	RowID() (int64, error)
}

// An IndexInfo describes virtual table indexing information.
//
// https://sqlite.org/c3ref/index_info.html
type IndexInfo struct {
	// Inputs
	Constraint  []IndexConstraint
	OrderBy     []IndexOrderBy
	ColumnsUsed uint64
	// Outputs
	ConstraintUsage []IndexConstraintUsage
	IdxNum          int
	IdxStr          string
	IdxFlags        IndexScanFlag
	OrderByConsumed bool
	EstimatedCost   float64
	EstimatedRows   int64
	// Internal
	c      *Conn
	handle ptr_t
}

// An IndexConstraint describes virtual table indexing constraint information.
//
// https://sqlite.org/c3ref/index_info.html
type IndexConstraint struct {
	Column int
	Op     IndexConstraintOp
	Usable bool
}

// An IndexOrderBy describes virtual table indexing order by information.
//
// https://sqlite.org/c3ref/index_info.html
type IndexOrderBy struct {
	Column int
	Desc   bool
}

// An IndexConstraintUsage describes how virtual table indexing constraints will be used.
//
// https://sqlite.org/c3ref/index_info.html
type IndexConstraintUsage struct {
	ArgvIndex int
	Omit      bool
}

// RHSValue returns the value of the right-hand operand of a constraint
// if the right-hand operand is known.
//
// https://sqlite.org/c3ref/vtab_rhs_value.html
func (idx *IndexInfo) RHSValue(column int) (Value, error) {
	defer idx.c.arena.mark()()
	valPtr := idx.c.arena.new(ptrlen)
	rc := res_t(idx.c.call("sqlite3_vtab_rhs_value", stk_t(idx.handle),
		stk_t(column), stk_t(valPtr)))
	if err := idx.c.error(rc); err != nil {
		return Value{}, err
	}
	return Value{
		c:      idx.c,
		handle: util.Read32[ptr_t](idx.c.mod, valPtr),
	}, nil
}

// Collation returns the name of the collation for a virtual table constraint.
//
// https://sqlite.org/c3ref/vtab_collation.html
func (idx *IndexInfo) Collation(column int) string {
	ptr := ptr_t(idx.c.call("sqlite3_vtab_collation", stk_t(idx.handle),
		stk_t(column)))
	return util.ReadString(idx.c.mod, ptr, _MAX_NAME)
}

// Distinct determines if a virtual table query is DISTINCT.
//
// https://sqlite.org/c3ref/vtab_distinct.html
func (idx *IndexInfo) Distinct() int {
	i := int32(idx.c.call("sqlite3_vtab_distinct", stk_t(idx.handle)))
	return int(i)
}

// In identifies and handles IN constraints.
//
// https://sqlite.org/c3ref/vtab_in.html
func (idx *IndexInfo) In(column, handle int) bool {
	b := int32(idx.c.call("sqlite3_vtab_in", stk_t(idx.handle),
		stk_t(column), stk_t(handle)))
	return b != 0
}

func (idx *IndexInfo) load() {
	// https://sqlite.org/c3ref/index_info.html
	mod := idx.c.mod
	ptr := idx.handle

	nConstraint := util.Read32[int32](mod, ptr+0)
	idx.Constraint = make([]IndexConstraint, nConstraint)
	idx.ConstraintUsage = make([]IndexConstraintUsage, nConstraint)
	idx.OrderBy = make([]IndexOrderBy, util.Read32[int32](mod, ptr+8))

	constraintPtr := util.Read32[ptr_t](mod, ptr+4)
	constraint := idx.Constraint
	for i := range idx.Constraint {
		constraint[i] = IndexConstraint{
			Column: int(util.Read32[int32](mod, constraintPtr+0)),
			Op:     util.Read[IndexConstraintOp](mod, constraintPtr+4),
			Usable: util.Read[byte](mod, constraintPtr+5) != 0,
		}
		constraintPtr += 12
	}

	orderByPtr := util.Read32[ptr_t](mod, ptr+12)
	orderBy := idx.OrderBy
	for i := range orderBy {
		orderBy[i] = IndexOrderBy{
			Column: int(util.Read32[int32](mod, orderByPtr+0)),
			Desc:   util.Read[byte](mod, orderByPtr+4) != 0,
		}
		orderByPtr += 8
	}

	idx.EstimatedCost = util.ReadFloat64(mod, ptr+40)
	idx.EstimatedRows = util.Read64[int64](mod, ptr+48)
	idx.ColumnsUsed = util.Read64[uint64](mod, ptr+64)
}

func (idx *IndexInfo) save() {
	// https://sqlite.org/c3ref/index_info.html
	mod := idx.c.mod
	ptr := idx.handle

	usagePtr := util.Read32[ptr_t](mod, ptr+16)
	for _, usage := range idx.ConstraintUsage {
		util.Write32(mod, usagePtr+0, int32(usage.ArgvIndex))
		if usage.Omit {
			util.Write(mod, usagePtr+4, int8(1))
		}
		usagePtr += 8
	}

	util.Write32(mod, ptr+20, int32(idx.IdxNum))
	if idx.IdxStr != "" {
		util.Write32(mod, ptr+24, idx.c.newString(idx.IdxStr))
		util.WriteBool(mod, ptr+28, true) // needToFreeIdxStr
	}
	if idx.OrderByConsumed {
		util.WriteBool(mod, ptr+32, true)
	}
	util.WriteFloat64(mod, ptr+40, idx.EstimatedCost)
	util.Write64(mod, ptr+48, idx.EstimatedRows)
	util.Write32(mod, ptr+56, idx.IdxFlags)
}

// IndexConstraintOp is a virtual table constraint operator code.
//
// https://sqlite.org/c3ref/c_index_constraint_eq.html
type IndexConstraintOp uint8

const (
	INDEX_CONSTRAINT_EQ        IndexConstraintOp = 2
	INDEX_CONSTRAINT_GT        IndexConstraintOp = 4
	INDEX_CONSTRAINT_LE        IndexConstraintOp = 8
	INDEX_CONSTRAINT_LT        IndexConstraintOp = 16
	INDEX_CONSTRAINT_GE        IndexConstraintOp = 32
	INDEX_CONSTRAINT_MATCH     IndexConstraintOp = 64
	INDEX_CONSTRAINT_LIKE      IndexConstraintOp = 65
	INDEX_CONSTRAINT_GLOB      IndexConstraintOp = 66
	INDEX_CONSTRAINT_REGEXP    IndexConstraintOp = 67
	INDEX_CONSTRAINT_NE        IndexConstraintOp = 68
	INDEX_CONSTRAINT_ISNOT     IndexConstraintOp = 69
	INDEX_CONSTRAINT_ISNOTNULL IndexConstraintOp = 70
	INDEX_CONSTRAINT_ISNULL    IndexConstraintOp = 71
	INDEX_CONSTRAINT_IS        IndexConstraintOp = 72
	INDEX_CONSTRAINT_LIMIT     IndexConstraintOp = 73
	INDEX_CONSTRAINT_OFFSET    IndexConstraintOp = 74
	INDEX_CONSTRAINT_FUNCTION  IndexConstraintOp = 150
)

// IndexScanFlag is a virtual table scan flag.
//
// https://sqlite.org/c3ref/c_index_scan_unique.html
type IndexScanFlag uint32

const (
	INDEX_SCAN_UNIQUE IndexScanFlag = 1
)

func vtabModuleCallback(i vtabConstructor) func(_ context.Context, _ api.Module, _ ptr_t, _ int32, _, _, _ ptr_t) res_t {
	return func(ctx context.Context, mod api.Module, pMod ptr_t, nArg int32, pArg, ppVTab, pzErr ptr_t) res_t {
		arg := make([]reflect.Value, 1+nArg)
		arg[0] = reflect.ValueOf(ctx.Value(connKey{}))

		for i := range nArg {
			ptr := util.Read32[ptr_t](mod, pArg+ptr_t(i)*ptrlen)
			arg[i+1] = reflect.ValueOf(util.ReadString(mod, ptr, _MAX_SQL_LENGTH))
		}

		module := vtabGetHandle(ctx, mod, pMod)
		val := reflect.ValueOf(module).Index(int(i)).Call(arg)
		err, _ := val[1].Interface().(error)
		if err == nil {
			vtabPutHandle(ctx, mod, ppVTab, val[0].Interface())
		}

		return vtabError(ctx, mod, pzErr, _PTR_ERROR, err)
	}
}

func vtabDisconnectCallback(ctx context.Context, mod api.Module, pVTab ptr_t) res_t {
	err := vtabDelHandle(ctx, mod, pVTab)
	return vtabError(ctx, mod, 0, _PTR_ERROR, err)
}

func vtabDestroyCallback(ctx context.Context, mod api.Module, pVTab ptr_t) res_t {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabDestroyer)
	err := errors.Join(vtab.Destroy(), vtabDelHandle(ctx, mod, pVTab))
	return vtabError(ctx, mod, 0, _PTR_ERROR, err)
}

func vtabBestIndexCallback(ctx context.Context, mod api.Module, pVTab, pIdxInfo ptr_t) res_t {
	var info IndexInfo
	info.handle = pIdxInfo
	info.c = ctx.Value(connKey{}).(*Conn)
	info.load()

	vtab := vtabGetHandle(ctx, mod, pVTab).(VTab)
	err := vtab.BestIndex(&info)

	info.save()
	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func vtabUpdateCallback(ctx context.Context, mod api.Module, pVTab ptr_t, nArg int32, pArg, pRowID ptr_t) res_t {
	db := ctx.Value(connKey{}).(*Conn)
	args := callbackArgs(db, nArg, pArg)
	defer returnArgs(args)

	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabUpdater)
	rowID, err := vtab.Update(*args...)
	if err == nil {
		util.Write64(mod, pRowID, rowID)
	}

	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func vtabRenameCallback(ctx context.Context, mod api.Module, pVTab, zNew ptr_t) res_t {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabRenamer)
	err := vtab.Rename(util.ReadString(mod, zNew, _MAX_NAME))
	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func vtabFindFuncCallback(ctx context.Context, mod api.Module, pVTab ptr_t, nArg int32, zName, pxFunc ptr_t) int32 {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabOverloader)
	f, op := vtab.FindFunction(int(nArg), util.ReadString(mod, zName, _MAX_NAME))
	if op != 0 {
		var wrapper ptr_t
		wrapper = util.AddHandle(ctx, func(c Context, arg ...Value) {
			defer util.DelHandle(ctx, wrapper)
			f(c, arg...)
		})
		util.Write32(mod, pxFunc, wrapper)
	}
	return int32(op)
}

func vtabIntegrityCallback(ctx context.Context, mod api.Module, pVTab, zSchema, zTabName ptr_t, mFlags uint32, pzErr ptr_t) res_t {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabChecker)
	schema := util.ReadString(mod, zSchema, _MAX_NAME)
	table := util.ReadString(mod, zTabName, _MAX_NAME)
	err := vtab.Integrity(schema, table, int(mFlags))
	// xIntegrity should return OK - even if it finds problems in the content of the virtual table.
	// https://sqlite.org/vtab.html#xintegrity
	vtabError(ctx, mod, pzErr, _PTR_ERROR, err)
	_, code := errorCode(err, _OK)
	return code
}

func vtabBeginCallback(ctx context.Context, mod api.Module, pVTab ptr_t) res_t {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabTxn)
	err := vtab.Begin()
	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func vtabSyncCallback(ctx context.Context, mod api.Module, pVTab ptr_t) res_t {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabTxn)
	err := vtab.Sync()
	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func vtabCommitCallback(ctx context.Context, mod api.Module, pVTab ptr_t) res_t {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabTxn)
	err := vtab.Commit()
	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func vtabRollbackCallback(ctx context.Context, mod api.Module, pVTab ptr_t) res_t {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabTxn)
	err := vtab.Rollback()
	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func vtabSavepointCallback(ctx context.Context, mod api.Module, pVTab ptr_t, id int32) res_t {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabSavepointer)
	err := vtab.Savepoint(int(id))
	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func vtabReleaseCallback(ctx context.Context, mod api.Module, pVTab ptr_t, id int32) res_t {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabSavepointer)
	err := vtab.Release(int(id))
	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func vtabRollbackToCallback(ctx context.Context, mod api.Module, pVTab ptr_t, id int32) res_t {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabSavepointer)
	err := vtab.RollbackTo(int(id))
	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func cursorOpenCallback(ctx context.Context, mod api.Module, pVTab, ppCur ptr_t) res_t {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTab)

	cursor, err := vtab.Open()
	if err == nil {
		vtabPutHandle(ctx, mod, ppCur, cursor)
	}

	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func cursorCloseCallback(ctx context.Context, mod api.Module, pCur ptr_t) res_t {
	err := vtabDelHandle(ctx, mod, pCur)
	return vtabError(ctx, mod, 0, _VTAB_ERROR, err)
}

func cursorFilterCallback(ctx context.Context, mod api.Module, pCur ptr_t, idxNum int32, idxStr ptr_t, nArg int32, pArg ptr_t) res_t {
	db := ctx.Value(connKey{}).(*Conn)
	args := callbackArgs(db, nArg, pArg)
	defer returnArgs(args)

	var idxName string
	if idxStr != 0 {
		idxName = util.ReadString(mod, idxStr, _MAX_LENGTH)
	}

	cursor := vtabGetHandle(ctx, mod, pCur).(VTabCursor)
	err := cursor.Filter(int(idxNum), idxName, *args...)
	return vtabError(ctx, mod, pCur, _CURSOR_ERROR, err)
}

func cursorEOFCallback(ctx context.Context, mod api.Module, pCur ptr_t) int32 {
	cursor := vtabGetHandle(ctx, mod, pCur).(VTabCursor)
	if cursor.EOF() {
		return 1
	}
	return 0
}

func cursorNextCallback(ctx context.Context, mod api.Module, pCur ptr_t) res_t {
	cursor := vtabGetHandle(ctx, mod, pCur).(VTabCursor)
	err := cursor.Next()
	return vtabError(ctx, mod, pCur, _CURSOR_ERROR, err)
}

func cursorColumnCallback(ctx context.Context, mod api.Module, pCur, pCtx ptr_t, n int32) res_t {
	cursor := vtabGetHandle(ctx, mod, pCur).(VTabCursor)
	db := ctx.Value(connKey{}).(*Conn)
	err := cursor.Column(Context{db, pCtx}, int(n))
	return vtabError(ctx, mod, pCur, _CURSOR_ERROR, err)
}

func cursorRowIDCallback(ctx context.Context, mod api.Module, pCur, pRowID ptr_t) res_t {
	cursor := vtabGetHandle(ctx, mod, pCur).(VTabCursor)

	rowID, err := cursor.RowID()
	if err == nil {
		util.Write64(mod, pRowID, rowID)
	}

	return vtabError(ctx, mod, pCur, _CURSOR_ERROR, err)
}

const (
	_PTR_ERROR = iota
	_VTAB_ERROR
	_CURSOR_ERROR
)

func vtabError(ctx context.Context, mod api.Module, ptr ptr_t, kind uint32, err error) res_t {
	const zErrMsgOffset = 8
	msg, code := errorCode(err, ERROR)
	if msg != "" && ptr != 0 {
		switch kind {
		case _VTAB_ERROR:
			ptr = ptr + zErrMsgOffset // zErrMsg
		case _CURSOR_ERROR:
			ptr = util.Read32[ptr_t](mod, ptr) + zErrMsgOffset // pVTab->zErrMsg
		}
		db := ctx.Value(connKey{}).(*Conn)
		if ptr := util.Read32[ptr_t](mod, ptr); ptr != 0 {
			db.free(ptr)
		}
		util.Write32(mod, ptr, db.newString(msg))
	}
	return code
}

func vtabGetHandle(ctx context.Context, mod api.Module, ptr ptr_t) any {
	const handleOffset = 4
	handle := util.Read32[ptr_t](mod, ptr-handleOffset)
	return util.GetHandle(ctx, handle)
}

func vtabDelHandle(ctx context.Context, mod api.Module, ptr ptr_t) error {
	const handleOffset = 4
	handle := util.Read32[ptr_t](mod, ptr-handleOffset)
	return util.DelHandle(ctx, handle)
}

func vtabPutHandle(ctx context.Context, mod api.Module, pptr ptr_t, val any) {
	const handleOffset = 4
	handle := util.AddHandle(ctx, val)
	ptr := util.Read32[ptr_t](mod, pptr)
	util.Write32(mod, ptr-handleOffset, handle)
}
