// Package embed embeds SQLite into your application.
//
// Importing package embed initializes the [sqlite3.Binary] variable
// with an appropriate build of SQLite:
//
//	import _ "github.com/ncruces/go-sqlite3/embed"
package embed

import (
	_ "embed"
	"unsafe"

	"github.com/ncruces/go-sqlite3"
)

//go:embed sqlite3.wasm
var binary string

func init() {
	sqlite3.Binary = unsafe.Slice(unsafe.StringData(binary), len(binary))
}
