package sqlite3

import "github.com/ncruces/go-sqlite3/internal/util"

// JSON returns a value that can be used as an argument to
// [database/sql.DB.Exec], [database/sql.Row.Scan] and similar methods to
// store value as JSON, or decode JSON into value.
// JSON should NOT be used with [Stmt.BindJSON], [Stmt.ColumnJSON],
// [Value.JSON], or [Context.ResultJSON].
func JSON(value any) any {
	return util.JSON{Value: value}
}
