package driver

import (
	"database/sql"
	"time"

	"github.com/ncruces/go-sqlite3"
)

// Savepoint establishes a new transaction savepoint.
//
// https://sqlite.org/lang_savepoint.html
func Savepoint(tx *sql.Tx) sqlite3.Savepoint {
	var ctx saveptCtx
	tx.ExecContext(&ctx, "")
	return ctx.Savepoint
}

// A saveptCtx is never canceled, has no values, and has no deadline.
type saveptCtx struct{ sqlite3.Savepoint }

func (*saveptCtx) Deadline() (deadline time.Time, ok bool) {
	// notest
	return
}

func (*saveptCtx) Done() <-chan struct{} {
	// notest
	return nil
}

func (*saveptCtx) Err() error {
	// notest
	return nil
}

func (*saveptCtx) Value(key any) any {
	// notest
	return nil
}
