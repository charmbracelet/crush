package sqlite3

import "github.com/ncruces/go-sqlite3/internal/util"

// Pointer returns a pointer to a value that can be used as an argument to
// [database/sql.DB.Exec] and similar methods.
// Pointer should NOT be used with [Stmt.BindPointer],
// [Value.Pointer], or [Context.ResultPointer].
//
// https://sqlite.org/bindptr.html
func Pointer[T any](value T) any {
	return util.Pointer[T]{Value: value}
}
