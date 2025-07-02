package sqlite3

import (
	"encoding/json"
	"math"
	"strconv"
	"time"

	"github.com/ncruces/go-sqlite3/internal/util"
)

// Stmt is a prepared statement object.
//
// https://sqlite.org/c3ref/stmt.html
type Stmt struct {
	c      *Conn
	err    error
	sql    string
	handle ptr_t
}

// Close destroys the prepared statement object.
//
// It is safe to close a nil, zero or closed Stmt.
//
// https://sqlite.org/c3ref/finalize.html
func (s *Stmt) Close() error {
	if s == nil || s.handle == 0 {
		return nil
	}

	rc := res_t(s.c.call("sqlite3_finalize", stk_t(s.handle)))
	stmts := s.c.stmts
	for i := range stmts {
		if s == stmts[i] {
			l := len(stmts) - 1
			stmts[i] = stmts[l]
			stmts[l] = nil
			s.c.stmts = stmts[:l]
			break
		}
	}

	s.handle = 0
	return s.c.error(rc)
}

// Conn returns the database connection to which the prepared statement belongs.
//
// https://sqlite.org/c3ref/db_handle.html
func (s *Stmt) Conn() *Conn {
	return s.c
}

// SQL returns the SQL text used to create the prepared statement.
//
// https://sqlite.org/c3ref/expanded_sql.html
func (s *Stmt) SQL() string {
	return s.sql
}

// ExpandedSQL returns the SQL text of the prepared statement
// with bound parameters expanded.
//
// https://sqlite.org/c3ref/expanded_sql.html
func (s *Stmt) ExpandedSQL() string {
	ptr := ptr_t(s.c.call("sqlite3_expanded_sql", stk_t(s.handle)))
	sql := util.ReadString(s.c.mod, ptr, _MAX_SQL_LENGTH)
	s.c.free(ptr)
	return sql
}

// ReadOnly returns true if and only if the statement
// makes no direct changes to the content of the database file.
//
// https://sqlite.org/c3ref/stmt_readonly.html
func (s *Stmt) ReadOnly() bool {
	b := int32(s.c.call("sqlite3_stmt_readonly", stk_t(s.handle)))
	return b != 0
}

// Reset resets the prepared statement object.
//
// https://sqlite.org/c3ref/reset.html
func (s *Stmt) Reset() error {
	rc := res_t(s.c.call("sqlite3_reset", stk_t(s.handle)))
	s.err = nil
	return s.c.error(rc)
}

// Busy determines if a prepared statement has been reset.
//
// https://sqlite.org/c3ref/stmt_busy.html
func (s *Stmt) Busy() bool {
	rc := res_t(s.c.call("sqlite3_stmt_busy", stk_t(s.handle)))
	return rc != 0
}

// Step evaluates the SQL statement.
// If the SQL statement being executed returns any data,
// then true is returned each time a new row of data is ready for processing by the caller.
// The values may be accessed using the Column access functions.
// Step is called again to retrieve the next row of data.
// If an error has occurred, Step returns false;
// call [Stmt.Err] or [Stmt.Reset] to get the error.
//
// https://sqlite.org/c3ref/step.html
func (s *Stmt) Step() bool {
	if s.c.interrupt.Err() != nil {
		s.err = INTERRUPT
		return false
	}
	return s.step()
}

func (s *Stmt) step() bool {
	rc := res_t(s.c.call("sqlite3_step", stk_t(s.handle)))
	switch rc {
	case _ROW:
		s.err = nil
		return true
	case _DONE:
		s.err = nil
	default:
		s.err = s.c.error(rc)
	}
	return false
}

// Err gets the last error occurred during [Stmt.Step].
// Err returns nil after [Stmt.Reset] is called.
//
// https://sqlite.org/c3ref/step.html
func (s *Stmt) Err() error {
	return s.err
}

// Exec is a convenience function that repeatedly calls [Stmt.Step] until it returns false,
// then calls [Stmt.Reset] to reset the statement and get any error that occurred.
func (s *Stmt) Exec() error {
	if s.c.interrupt.Err() != nil {
		return INTERRUPT
	}
	// TODO: implement this in C.
	for s.step() {
	}
	return s.Reset()
}

// Status monitors the performance characteristics of prepared statements.
//
// https://sqlite.org/c3ref/stmt_status.html
func (s *Stmt) Status(op StmtStatus, reset bool) int {
	if op > STMTSTATUS_FILTER_HIT && op != STMTSTATUS_MEMUSED {
		return 0
	}
	var i int32
	if reset {
		i = 1
	}
	n := int32(s.c.call("sqlite3_stmt_status", stk_t(s.handle),
		stk_t(op), stk_t(i)))
	return int(n)
}

// ClearBindings resets all bindings on the prepared statement.
//
// https://sqlite.org/c3ref/clear_bindings.html
func (s *Stmt) ClearBindings() error {
	rc := res_t(s.c.call("sqlite3_clear_bindings", stk_t(s.handle)))
	return s.c.error(rc)
}

// BindCount returns the number of SQL parameters in the prepared statement.
//
// https://sqlite.org/c3ref/bind_parameter_count.html
func (s *Stmt) BindCount() int {
	n := int32(s.c.call("sqlite3_bind_parameter_count",
		stk_t(s.handle)))
	return int(n)
}

// BindIndex returns the index of a parameter in the prepared statement
// given its name.
//
// https://sqlite.org/c3ref/bind_parameter_index.html
func (s *Stmt) BindIndex(name string) int {
	defer s.c.arena.mark()()
	namePtr := s.c.arena.string(name)
	i := int32(s.c.call("sqlite3_bind_parameter_index",
		stk_t(s.handle), stk_t(namePtr)))
	return int(i)
}

// BindName returns the name of a parameter in the prepared statement.
// The leftmost SQL parameter has an index of 1.
//
// https://sqlite.org/c3ref/bind_parameter_name.html
func (s *Stmt) BindName(param int) string {
	ptr := ptr_t(s.c.call("sqlite3_bind_parameter_name",
		stk_t(s.handle), stk_t(param)))
	if ptr == 0 {
		return ""
	}
	return util.ReadString(s.c.mod, ptr, _MAX_NAME)
}

// BindBool binds a bool to the prepared statement.
// The leftmost SQL parameter has an index of 1.
// SQLite does not have a separate boolean storage class.
// Instead, boolean values are stored as integers 0 (false) and 1 (true).
//
// https://sqlite.org/c3ref/bind_blob.html
func (s *Stmt) BindBool(param int, value bool) error {
	var i int64
	if value {
		i = 1
	}
	return s.BindInt64(param, i)
}

// BindInt binds an int to the prepared statement.
// The leftmost SQL parameter has an index of 1.
//
// https://sqlite.org/c3ref/bind_blob.html
func (s *Stmt) BindInt(param int, value int) error {
	return s.BindInt64(param, int64(value))
}

// BindInt64 binds an int64 to the prepared statement.
// The leftmost SQL parameter has an index of 1.
//
// https://sqlite.org/c3ref/bind_blob.html
func (s *Stmt) BindInt64(param int, value int64) error {
	rc := res_t(s.c.call("sqlite3_bind_int64",
		stk_t(s.handle), stk_t(param), stk_t(value)))
	return s.c.error(rc)
}

// BindFloat binds a float64 to the prepared statement.
// The leftmost SQL parameter has an index of 1.
//
// https://sqlite.org/c3ref/bind_blob.html
func (s *Stmt) BindFloat(param int, value float64) error {
	rc := res_t(s.c.call("sqlite3_bind_double",
		stk_t(s.handle), stk_t(param),
		stk_t(math.Float64bits(value))))
	return s.c.error(rc)
}

// BindText binds a string to the prepared statement.
// The leftmost SQL parameter has an index of 1.
//
// https://sqlite.org/c3ref/bind_blob.html
func (s *Stmt) BindText(param int, value string) error {
	if len(value) > _MAX_LENGTH {
		return TOOBIG
	}
	ptr := s.c.newString(value)
	rc := res_t(s.c.call("sqlite3_bind_text_go",
		stk_t(s.handle), stk_t(param),
		stk_t(ptr), stk_t(len(value))))
	return s.c.error(rc)
}

// BindRawText binds a []byte to the prepared statement as text.
// The leftmost SQL parameter has an index of 1.
//
// https://sqlite.org/c3ref/bind_blob.html
func (s *Stmt) BindRawText(param int, value []byte) error {
	if len(value) > _MAX_LENGTH {
		return TOOBIG
	}
	if len(value) == 0 {
		return s.BindText(param, "")
	}
	ptr := s.c.newBytes(value)
	rc := res_t(s.c.call("sqlite3_bind_text_go",
		stk_t(s.handle), stk_t(param),
		stk_t(ptr), stk_t(len(value))))
	return s.c.error(rc)
}

// BindBlob binds a []byte to the prepared statement.
// The leftmost SQL parameter has an index of 1.
//
// https://sqlite.org/c3ref/bind_blob.html
func (s *Stmt) BindBlob(param int, value []byte) error {
	if len(value) > _MAX_LENGTH {
		return TOOBIG
	}
	if len(value) == 0 {
		return s.BindZeroBlob(param, 0)
	}
	ptr := s.c.newBytes(value)
	rc := res_t(s.c.call("sqlite3_bind_blob_go",
		stk_t(s.handle), stk_t(param),
		stk_t(ptr), stk_t(len(value))))
	return s.c.error(rc)
}

// BindZeroBlob binds a zero-filled, length n BLOB to the prepared statement.
// The leftmost SQL parameter has an index of 1.
//
// https://sqlite.org/c3ref/bind_blob.html
func (s *Stmt) BindZeroBlob(param int, n int64) error {
	rc := res_t(s.c.call("sqlite3_bind_zeroblob64",
		stk_t(s.handle), stk_t(param), stk_t(n)))
	return s.c.error(rc)
}

// BindNull binds a NULL to the prepared statement.
// The leftmost SQL parameter has an index of 1.
//
// https://sqlite.org/c3ref/bind_blob.html
func (s *Stmt) BindNull(param int) error {
	rc := res_t(s.c.call("sqlite3_bind_null",
		stk_t(s.handle), stk_t(param)))
	return s.c.error(rc)
}

// BindTime binds a [time.Time] to the prepared statement.
// The leftmost SQL parameter has an index of 1.
//
// https://sqlite.org/c3ref/bind_blob.html
func (s *Stmt) BindTime(param int, value time.Time, format TimeFormat) error {
	switch format {
	case TimeFormatDefault, TimeFormatAuto, time.RFC3339Nano:
		return s.bindRFC3339Nano(param, value)
	}
	switch v := format.Encode(value).(type) {
	case string:
		return s.BindText(param, v)
	case int64:
		return s.BindInt64(param, v)
	case float64:
		return s.BindFloat(param, v)
	default:
		panic(util.AssertErr())
	}
}

func (s *Stmt) bindRFC3339Nano(param int, value time.Time) error {
	const maxlen = int64(len(time.RFC3339Nano)) + 5

	ptr := s.c.new(maxlen)
	buf := util.View(s.c.mod, ptr, maxlen)
	buf = value.AppendFormat(buf[:0], time.RFC3339Nano)

	rc := res_t(s.c.call("sqlite3_bind_text_go",
		stk_t(s.handle), stk_t(param),
		stk_t(ptr), stk_t(len(buf))))
	return s.c.error(rc)
}

// BindPointer binds a NULL to the prepared statement, just like [Stmt.BindNull],
// but it also associates ptr with that NULL value such that it can be retrieved
// within an application-defined SQL function using [Value.Pointer].
// The leftmost SQL parameter has an index of 1.
//
// https://sqlite.org/c3ref/bind_blob.html
func (s *Stmt) BindPointer(param int, ptr any) error {
	valPtr := util.AddHandle(s.c.ctx, ptr)
	rc := res_t(s.c.call("sqlite3_bind_pointer_go",
		stk_t(s.handle), stk_t(param), stk_t(valPtr)))
	return s.c.error(rc)
}

// BindJSON binds the JSON encoding of value to the prepared statement.
// The leftmost SQL parameter has an index of 1.
//
// https://sqlite.org/c3ref/bind_blob.html
func (s *Stmt) BindJSON(param int, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.BindRawText(param, data)
}

// BindValue binds a copy of value to the prepared statement.
// The leftmost SQL parameter has an index of 1.
//
// https://sqlite.org/c3ref/bind_blob.html
func (s *Stmt) BindValue(param int, value Value) error {
	if value.c != s.c {
		return MISUSE
	}
	rc := res_t(s.c.call("sqlite3_bind_value",
		stk_t(s.handle), stk_t(param), stk_t(value.handle)))
	return s.c.error(rc)
}

// DataCount resets the number of columns in a result set.
//
// https://sqlite.org/c3ref/data_count.html
func (s *Stmt) DataCount() int {
	n := int32(s.c.call("sqlite3_data_count",
		stk_t(s.handle)))
	return int(n)
}

// ColumnCount returns the number of columns in a result set.
//
// https://sqlite.org/c3ref/column_count.html
func (s *Stmt) ColumnCount() int {
	n := int32(s.c.call("sqlite3_column_count",
		stk_t(s.handle)))
	return int(n)
}

// ColumnName returns the name of the result column.
// The leftmost column of the result set has the index 0.
//
// https://sqlite.org/c3ref/column_name.html
func (s *Stmt) ColumnName(col int) string {
	ptr := ptr_t(s.c.call("sqlite3_column_name",
		stk_t(s.handle), stk_t(col)))
	if ptr == 0 {
		panic(util.OOMErr)
	}
	return util.ReadString(s.c.mod, ptr, _MAX_NAME)
}

// ColumnType returns the initial [Datatype] of the result column.
// The leftmost column of the result set has the index 0.
//
// https://sqlite.org/c3ref/column_blob.html
func (s *Stmt) ColumnType(col int) Datatype {
	return Datatype(s.c.call("sqlite3_column_type",
		stk_t(s.handle), stk_t(col)))
}

// ColumnDeclType returns the declared datatype of the result column.
// The leftmost column of the result set has the index 0.
//
// https://sqlite.org/c3ref/column_decltype.html
func (s *Stmt) ColumnDeclType(col int) string {
	ptr := ptr_t(s.c.call("sqlite3_column_decltype",
		stk_t(s.handle), stk_t(col)))
	if ptr == 0 {
		return ""
	}
	return util.ReadString(s.c.mod, ptr, _MAX_NAME)
}

// ColumnDatabaseName returns the name of the database
// that is the origin of a particular result column.
// The leftmost column of the result set has the index 0.
//
// https://sqlite.org/c3ref/column_database_name.html
func (s *Stmt) ColumnDatabaseName(col int) string {
	ptr := ptr_t(s.c.call("sqlite3_column_database_name",
		stk_t(s.handle), stk_t(col)))
	if ptr == 0 {
		return ""
	}
	return util.ReadString(s.c.mod, ptr, _MAX_NAME)
}

// ColumnTableName returns the name of the table
// that is the origin of a particular result column.
// The leftmost column of the result set has the index 0.
//
// https://sqlite.org/c3ref/column_database_name.html
func (s *Stmt) ColumnTableName(col int) string {
	ptr := ptr_t(s.c.call("sqlite3_column_table_name",
		stk_t(s.handle), stk_t(col)))
	if ptr == 0 {
		return ""
	}
	return util.ReadString(s.c.mod, ptr, _MAX_NAME)
}

// ColumnOriginName returns the name of the table column
// that is the origin of a particular result column.
// The leftmost column of the result set has the index 0.
//
// https://sqlite.org/c3ref/column_database_name.html
func (s *Stmt) ColumnOriginName(col int) string {
	ptr := ptr_t(s.c.call("sqlite3_column_origin_name",
		stk_t(s.handle), stk_t(col)))
	if ptr == 0 {
		return ""
	}
	return util.ReadString(s.c.mod, ptr, _MAX_NAME)
}

// ColumnBool returns the value of the result column as a bool.
// The leftmost column of the result set has the index 0.
// SQLite does not have a separate boolean storage class.
// Instead, boolean values are retrieved as numbers,
// with 0 converted to false and any other value to true.
//
// https://sqlite.org/c3ref/column_blob.html
func (s *Stmt) ColumnBool(col int) bool {
	return s.ColumnFloat(col) != 0
}

// ColumnInt returns the value of the result column as an int.
// The leftmost column of the result set has the index 0.
//
// https://sqlite.org/c3ref/column_blob.html
func (s *Stmt) ColumnInt(col int) int {
	return int(s.ColumnInt64(col))
}

// ColumnInt64 returns the value of the result column as an int64.
// The leftmost column of the result set has the index 0.
//
// https://sqlite.org/c3ref/column_blob.html
func (s *Stmt) ColumnInt64(col int) int64 {
	return int64(s.c.call("sqlite3_column_int64",
		stk_t(s.handle), stk_t(col)))
}

// ColumnFloat returns the value of the result column as a float64.
// The leftmost column of the result set has the index 0.
//
// https://sqlite.org/c3ref/column_blob.html
func (s *Stmt) ColumnFloat(col int) float64 {
	f := uint64(s.c.call("sqlite3_column_double",
		stk_t(s.handle), stk_t(col)))
	return math.Float64frombits(f)
}

// ColumnTime returns the value of the result column as a [time.Time].
// The leftmost column of the result set has the index 0.
//
// https://sqlite.org/c3ref/column_blob.html
func (s *Stmt) ColumnTime(col int, format TimeFormat) time.Time {
	var v any
	switch s.ColumnType(col) {
	case INTEGER:
		v = s.ColumnInt64(col)
	case FLOAT:
		v = s.ColumnFloat(col)
	case TEXT, BLOB:
		v = s.ColumnText(col)
	case NULL:
		return time.Time{}
	default:
		panic(util.AssertErr())
	}
	t, err := format.Decode(v)
	if err != nil {
		s.err = err
	}
	return t
}

// ColumnText returns the value of the result column as a string.
// The leftmost column of the result set has the index 0.
//
// https://sqlite.org/c3ref/column_blob.html
func (s *Stmt) ColumnText(col int) string {
	return string(s.ColumnRawText(col))
}

// ColumnBlob appends to buf and returns
// the value of the result column as a []byte.
// The leftmost column of the result set has the index 0.
//
// https://sqlite.org/c3ref/column_blob.html
func (s *Stmt) ColumnBlob(col int, buf []byte) []byte {
	return append(buf, s.ColumnRawBlob(col)...)
}

// ColumnRawText returns the value of the result column as a []byte.
// The []byte is owned by SQLite and may be invalidated by
// subsequent calls to [Stmt] methods.
// The leftmost column of the result set has the index 0.
//
// https://sqlite.org/c3ref/column_blob.html
func (s *Stmt) ColumnRawText(col int) []byte {
	ptr := ptr_t(s.c.call("sqlite3_column_text",
		stk_t(s.handle), stk_t(col)))
	return s.columnRawBytes(col, ptr, 1)
}

// ColumnRawBlob returns the value of the result column as a []byte.
// The []byte is owned by SQLite and may be invalidated by
// subsequent calls to [Stmt] methods.
// The leftmost column of the result set has the index 0.
//
// https://sqlite.org/c3ref/column_blob.html
func (s *Stmt) ColumnRawBlob(col int) []byte {
	ptr := ptr_t(s.c.call("sqlite3_column_blob",
		stk_t(s.handle), stk_t(col)))
	return s.columnRawBytes(col, ptr, 0)
}

func (s *Stmt) columnRawBytes(col int, ptr ptr_t, nul int32) []byte {
	if ptr == 0 {
		rc := res_t(s.c.call("sqlite3_errcode", stk_t(s.c.handle)))
		if rc != _ROW && rc != _DONE {
			s.err = s.c.error(rc)
		}
		return nil
	}

	n := int32(s.c.call("sqlite3_column_bytes",
		stk_t(s.handle), stk_t(col)))
	return util.View(s.c.mod, ptr, int64(n+nul))[:n]
}

// ColumnJSON parses the JSON-encoded value of the result column
// and stores it in the value pointed to by ptr.
// The leftmost column of the result set has the index 0.
//
// https://sqlite.org/c3ref/column_blob.html
func (s *Stmt) ColumnJSON(col int, ptr any) error {
	var data []byte
	switch s.ColumnType(col) {
	case NULL:
		data = []byte("null")
	case TEXT:
		data = s.ColumnRawText(col)
	case BLOB:
		data = s.ColumnRawBlob(col)
	case INTEGER:
		data = strconv.AppendInt(nil, s.ColumnInt64(col), 10)
	case FLOAT:
		data = util.AppendNumber(nil, s.ColumnFloat(col))
	default:
		panic(util.AssertErr())
	}
	return json.Unmarshal(data, ptr)
}

// ColumnValue returns the unprotected value of the result column.
// The leftmost column of the result set has the index 0.
//
// https://sqlite.org/c3ref/column_blob.html
func (s *Stmt) ColumnValue(col int) Value {
	ptr := ptr_t(s.c.call("sqlite3_column_value",
		stk_t(s.handle), stk_t(col)))
	return Value{
		c:      s.c,
		unprot: true,
		handle: ptr,
	}
}

// Columns populates result columns into the provided slice.
// The slice must have [Stmt.ColumnCount] length.
//
// [INTEGER] columns will be retrieved as int64 values,
// [FLOAT] as float64, [NULL] as nil,
// [TEXT] as string, and [BLOB] as []byte.
func (s *Stmt) Columns(dest ...any) error {
	types, ptr, err := s.columns(int64(len(dest)))
	if err != nil {
		return err
	}

	// Avoid bounds checks on types below.
	if len(types) != len(dest) {
		panic(util.AssertErr())
	}

	for i := range dest {
		switch types[i] {
		case byte(INTEGER):
			dest[i] = util.Read64[int64](s.c.mod, ptr)
		case byte(FLOAT):
			dest[i] = util.ReadFloat64(s.c.mod, ptr)
		case byte(NULL):
			dest[i] = nil
		case byte(TEXT):
			len := util.Read32[int32](s.c.mod, ptr+4)
			if len != 0 {
				ptr := util.Read32[ptr_t](s.c.mod, ptr)
				buf := util.View(s.c.mod, ptr, int64(len))
				dest[i] = string(buf)
			} else {
				dest[i] = ""
			}
		case byte(BLOB):
			len := util.Read32[int32](s.c.mod, ptr+4)
			if len != 0 {
				ptr := util.Read32[ptr_t](s.c.mod, ptr)
				buf := util.View(s.c.mod, ptr, int64(len))
				tmp, _ := dest[i].([]byte)
				dest[i] = append(tmp[:0], buf...)
			} else {
				dest[i], _ = dest[i].([]byte)
			}
		}
		ptr += 8
	}
	return nil
}

// ColumnsRaw populates result columns into the provided slice.
// The slice must have [Stmt.ColumnCount] length.
//
// [INTEGER] columns will be retrieved as int64 values,
// [FLOAT] as float64, [NULL] as nil,
// [TEXT] and [BLOB] as []byte.
// Any []byte are owned by SQLite and may be invalidated by
// subsequent calls to [Stmt] methods.
func (s *Stmt) ColumnsRaw(dest ...any) error {
	types, ptr, err := s.columns(int64(len(dest)))
	if err != nil {
		return err
	}

	// Avoid bounds checks on types below.
	if len(types) != len(dest) {
		panic(util.AssertErr())
	}

	for i := range dest {
		switch types[i] {
		case byte(INTEGER):
			dest[i] = util.Read64[int64](s.c.mod, ptr)
		case byte(FLOAT):
			dest[i] = util.ReadFloat64(s.c.mod, ptr)
		case byte(NULL):
			dest[i] = nil
		default:
			len := util.Read32[int32](s.c.mod, ptr+4)
			if len == 0 && types[i] == byte(BLOB) {
				dest[i] = []byte{}
			} else {
				cap := len
				if types[i] == byte(TEXT) {
					cap++
				}
				ptr := util.Read32[ptr_t](s.c.mod, ptr)
				buf := util.View(s.c.mod, ptr, int64(cap))[:len]
				dest[i] = buf
			}
		}
		ptr += 8
	}
	return nil
}

func (s *Stmt) columns(count int64) ([]byte, ptr_t, error) {
	defer s.c.arena.mark()()
	typePtr := s.c.arena.new(count)
	dataPtr := s.c.arena.new(count * 8)

	rc := res_t(s.c.call("sqlite3_columns_go",
		stk_t(s.handle), stk_t(count), stk_t(typePtr), stk_t(dataPtr)))
	if rc == res_t(MISUSE) {
		return nil, 0, MISUSE
	}
	if err := s.c.error(rc); err != nil {
		return nil, 0, err
	}

	return util.View(s.c.mod, typePtr, count), dataPtr, nil
}
