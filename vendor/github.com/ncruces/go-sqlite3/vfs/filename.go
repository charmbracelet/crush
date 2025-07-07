package vfs

import (
	"context"
	"net/url"

	"github.com/tetratelabs/wazero/api"

	"github.com/ncruces/go-sqlite3/internal/util"
)

// Filename is used by SQLite to pass filenames
// to the Open method of a VFS.
//
// https://sqlite.org/c3ref/filename.html
type Filename struct {
	ctx   context.Context
	mod   api.Module
	zPath ptr_t
	flags OpenFlag
	stack [2]stk_t
}

// GetFilename is an internal API users should not call directly.
func GetFilename(ctx context.Context, mod api.Module, id ptr_t, flags OpenFlag) *Filename {
	if id == 0 {
		return nil
	}
	return &Filename{
		ctx:   ctx,
		mod:   mod,
		zPath: id,
		flags: flags,
	}
}

// String returns this filename as a string.
func (n *Filename) String() string {
	if n == nil || n.zPath == 0 {
		return ""
	}
	return util.ReadString(n.mod, n.zPath, _MAX_PATHNAME)
}

// Database returns the name of the corresponding database file.
//
// https://sqlite.org/c3ref/filename_database.html
func (n *Filename) Database() string {
	return n.path("sqlite3_filename_database")
}

// Journal returns the name of the corresponding rollback journal file.
//
// https://sqlite.org/c3ref/filename_database.html
func (n *Filename) Journal() string {
	return n.path("sqlite3_filename_journal")
}

// Journal returns the name of the corresponding WAL file.
//
// https://sqlite.org/c3ref/filename_database.html
func (n *Filename) WAL() string {
	return n.path("sqlite3_filename_wal")
}

func (n *Filename) path(method string) string {
	if n == nil || n.zPath == 0 {
		return ""
	}
	if n.flags&(OPEN_MAIN_DB|OPEN_MAIN_JOURNAL|OPEN_WAL) == 0 {
		return ""
	}

	n.stack[0] = stk_t(n.zPath)
	fn := n.mod.ExportedFunction(method)
	if err := fn.CallWithStack(n.ctx, n.stack[:]); err != nil {
		panic(err)
	}
	return util.ReadString(n.mod, ptr_t(n.stack[0]), _MAX_PATHNAME)
}

// DatabaseFile returns the main database [File] corresponding to a journal.
//
// https://sqlite.org/c3ref/database_file_object.html
func (n *Filename) DatabaseFile() File {
	if n == nil || n.zPath == 0 {
		return nil
	}
	if n.flags&(OPEN_MAIN_DB|OPEN_MAIN_JOURNAL|OPEN_WAL) == 0 {
		return nil
	}

	n.stack[0] = stk_t(n.zPath)
	fn := n.mod.ExportedFunction("sqlite3_database_file_object")
	if err := fn.CallWithStack(n.ctx, n.stack[:]); err != nil {
		panic(err)
	}
	file, _ := vfsFileGet(n.ctx, n.mod, ptr_t(n.stack[0])).(File)
	return file
}

// URIParameter returns the value of a URI parameter.
//
// https://sqlite.org/c3ref/uri_boolean.html
func (n *Filename) URIParameter(key string) string {
	if n == nil || n.zPath == 0 {
		return ""
	}

	uriKey := n.mod.ExportedFunction("sqlite3_uri_key")
	n.stack[0] = stk_t(n.zPath)
	n.stack[1] = stk_t(0)
	if err := uriKey.CallWithStack(n.ctx, n.stack[:]); err != nil {
		panic(err)
	}

	ptr := ptr_t(n.stack[0])
	if ptr == 0 {
		return ""
	}

	// Parse the format from:
	// https://github.com/sqlite/sqlite/blob/b74eb0/src/pager.c#L4797-L4840
	// This avoids having to alloc/free the key just to find a value.
	for {
		k := util.ReadString(n.mod, ptr, _MAX_NAME)
		if k == "" {
			return ""
		}
		ptr += ptr_t(len(k)) + 1

		v := util.ReadString(n.mod, ptr, _MAX_NAME)
		if k == key {
			return v
		}
		ptr += ptr_t(len(v)) + 1
	}
}

// URIParameters obtains values for URI parameters.
//
// https://sqlite.org/c3ref/uri_boolean.html
func (n *Filename) URIParameters() url.Values {
	if n == nil || n.zPath == 0 {
		return nil
	}

	uriKey := n.mod.ExportedFunction("sqlite3_uri_key")
	n.stack[0] = stk_t(n.zPath)
	n.stack[1] = stk_t(0)
	if err := uriKey.CallWithStack(n.ctx, n.stack[:]); err != nil {
		panic(err)
	}

	ptr := ptr_t(n.stack[0])
	if ptr == 0 {
		return nil
	}

	var params url.Values

	// Parse the format from:
	// https://github.com/sqlite/sqlite/blob/b74eb0/src/pager.c#L4797-L4840
	// This is the only way to support multiple valued keys.
	for {
		k := util.ReadString(n.mod, ptr, _MAX_NAME)
		if k == "" {
			return params
		}
		ptr += ptr_t(len(k)) + 1

		v := util.ReadString(n.mod, ptr, _MAX_NAME)
		if params == nil {
			params = url.Values{}
		}
		params.Add(k, v)
		ptr += ptr_t(len(v)) + 1
	}
}
