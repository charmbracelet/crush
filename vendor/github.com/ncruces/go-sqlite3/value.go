package sqlite3

import (
	"encoding/json"
	"math"
	"strconv"
	"time"

	"github.com/ncruces/go-sqlite3/internal/util"
)

// Value is any value that can be stored in a database table.
//
// https://sqlite.org/c3ref/value.html
type Value struct {
	c      *Conn
	handle ptr_t
	unprot bool
	copied bool
}

func (v Value) protected() stk_t {
	if v.unprot {
		panic(util.ValueErr)
	}
	return stk_t(v.handle)
}

// Dup makes a copy of the SQL value and returns a pointer to that copy.
//
// https://sqlite.org/c3ref/value_dup.html
func (v Value) Dup() *Value {
	ptr := ptr_t(v.c.call("sqlite3_value_dup", stk_t(v.handle)))
	return &Value{
		c:      v.c,
		copied: true,
		handle: ptr,
	}
}

// Close frees an SQL value previously obtained by [Value.Dup].
//
// https://sqlite.org/c3ref/value_dup.html
func (dup *Value) Close() error {
	if !dup.copied {
		panic(util.ValueErr)
	}
	dup.c.call("sqlite3_value_free", stk_t(dup.handle))
	dup.handle = 0
	return nil
}

// Type returns the initial datatype of the value.
//
// https://sqlite.org/c3ref/value_blob.html
func (v Value) Type() Datatype {
	return Datatype(v.c.call("sqlite3_value_type", v.protected()))
}

// Type returns the numeric datatype of the value.
//
// https://sqlite.org/c3ref/value_blob.html
func (v Value) NumericType() Datatype {
	return Datatype(v.c.call("sqlite3_value_numeric_type", v.protected()))
}

// Bool returns the value as a bool.
// SQLite does not have a separate boolean storage class.
// Instead, boolean values are retrieved as numbers,
// with 0 converted to false and any other value to true.
//
// https://sqlite.org/c3ref/value_blob.html
func (v Value) Bool() bool {
	return v.Float() != 0
}

// Int returns the value as an int.
//
// https://sqlite.org/c3ref/value_blob.html
func (v Value) Int() int {
	return int(v.Int64())
}

// Int64 returns the value as an int64.
//
// https://sqlite.org/c3ref/value_blob.html
func (v Value) Int64() int64 {
	return int64(v.c.call("sqlite3_value_int64", v.protected()))
}

// Float returns the value as a float64.
//
// https://sqlite.org/c3ref/value_blob.html
func (v Value) Float() float64 {
	f := uint64(v.c.call("sqlite3_value_double", v.protected()))
	return math.Float64frombits(f)
}

// Time returns the value as a [time.Time].
//
// https://sqlite.org/c3ref/value_blob.html
func (v Value) Time(format TimeFormat) time.Time {
	var a any
	switch v.Type() {
	case INTEGER:
		a = v.Int64()
	case FLOAT:
		a = v.Float()
	case TEXT, BLOB:
		a = v.Text()
	case NULL:
		return time.Time{}
	default:
		panic(util.AssertErr())
	}
	t, _ := format.Decode(a)
	return t
}

// Text returns the value as a string.
//
// https://sqlite.org/c3ref/value_blob.html
func (v Value) Text() string {
	return string(v.RawText())
}

// Blob appends to buf and returns
// the value as a []byte.
//
// https://sqlite.org/c3ref/value_blob.html
func (v Value) Blob(buf []byte) []byte {
	return append(buf, v.RawBlob()...)
}

// RawText returns the value as a []byte.
// The []byte is owned by SQLite and may be invalidated by
// subsequent calls to [Value] methods.
//
// https://sqlite.org/c3ref/value_blob.html
func (v Value) RawText() []byte {
	ptr := ptr_t(v.c.call("sqlite3_value_text", v.protected()))
	return v.rawBytes(ptr, 1)
}

// RawBlob returns the value as a []byte.
// The []byte is owned by SQLite and may be invalidated by
// subsequent calls to [Value] methods.
//
// https://sqlite.org/c3ref/value_blob.html
func (v Value) RawBlob() []byte {
	ptr := ptr_t(v.c.call("sqlite3_value_blob", v.protected()))
	return v.rawBytes(ptr, 0)
}

func (v Value) rawBytes(ptr ptr_t, nul int32) []byte {
	if ptr == 0 {
		return nil
	}

	n := int32(v.c.call("sqlite3_value_bytes", v.protected()))
	return util.View(v.c.mod, ptr, int64(n+nul))[:n]
}

// Pointer gets the pointer associated with this value,
// or nil if it has no associated pointer.
func (v Value) Pointer() any {
	ptr := ptr_t(v.c.call("sqlite3_value_pointer_go", v.protected()))
	return util.GetHandle(v.c.ctx, ptr)
}

// JSON parses a JSON-encoded value
// and stores the result in the value pointed to by ptr.
func (v Value) JSON(ptr any) error {
	var data []byte
	switch v.Type() {
	case NULL:
		data = []byte("null")
	case TEXT:
		data = v.RawText()
	case BLOB:
		data = v.RawBlob()
	case INTEGER:
		data = strconv.AppendInt(nil, v.Int64(), 10)
	case FLOAT:
		data = util.AppendNumber(nil, v.Float())
	default:
		panic(util.AssertErr())
	}
	return json.Unmarshal(data, ptr)
}

// NoChange returns true if and only if the value is unchanged
// in a virtual table update operatiom.
//
// https://sqlite.org/c3ref/value_blob.html
func (v Value) NoChange() bool {
	b := int32(v.c.call("sqlite3_value_nochange", v.protected()))
	return b != 0
}

// FromBind returns true if value originated from a bound parameter.
//
// https://sqlite.org/c3ref/value_blob.html
func (v Value) FromBind() bool {
	b := int32(v.c.call("sqlite3_value_frombind", v.protected()))
	return b != 0
}

// InFirst returns the first element
// on the right-hand side of an IN constraint.
//
// https://sqlite.org/c3ref/vtab_in_first.html
func (v Value) InFirst() (Value, error) {
	defer v.c.arena.mark()()
	valPtr := v.c.arena.new(ptrlen)
	rc := res_t(v.c.call("sqlite3_vtab_in_first", stk_t(v.handle), stk_t(valPtr)))
	if err := v.c.error(rc); err != nil {
		return Value{}, err
	}
	return Value{
		c:      v.c,
		handle: util.Read32[ptr_t](v.c.mod, valPtr),
	}, nil
}

// InNext returns the next element
// on the right-hand side of an IN constraint.
//
// https://sqlite.org/c3ref/vtab_in_first.html
func (v Value) InNext() (Value, error) {
	defer v.c.arena.mark()()
	valPtr := v.c.arena.new(ptrlen)
	rc := res_t(v.c.call("sqlite3_vtab_in_next", stk_t(v.handle), stk_t(valPtr)))
	if err := v.c.error(rc); err != nil {
		return Value{}, err
	}
	return Value{
		c:      v.c,
		handle: util.Read32[ptr_t](v.c.mod, valPtr),
	}, nil
}
