# Go bindings to SQLite using wazero

[![Go Reference](https://pkg.go.dev/badge/image)](https://pkg.go.dev/github.com/ncruces/go-sqlite3)
[![Go Report](https://goreportcard.com/badge/github.com/ncruces/go-sqlite3)](https://goreportcard.com/report/github.com/ncruces/go-sqlite3)
[![Go Coverage](https://github.com/ncruces/go-sqlite3/wiki/coverage.svg)](https://github.com/ncruces/go-sqlite3/wiki/Test-coverage-report)

Go module `github.com/ncruces/go-sqlite3` is a `cgo`-free [SQLite](https://sqlite.org/) wrapper.\
It provides a [`database/sql`](https://pkg.go.dev/database/sql) compatible driver,
as well as direct access to most of the [C SQLite API](https://sqlite.org/cintro.html).

It wraps a [Wasm](https://webassembly.org/) [build](embed/) of SQLite,
and uses [wazero](https://wazero.io/) as the runtime.\
Go, wazero and [`x/sys`](https://pkg.go.dev/golang.org/x/sys) are the _only_ direct dependencies.

### Getting started

Using the [`database/sql`](https://pkg.go.dev/database/sql) driver:
```go

import "database/sql"
import _ "github.com/ncruces/go-sqlite3/driver"
import _ "github.com/ncruces/go-sqlite3/embed"

var version string
db, _ := sql.Open("sqlite3", "file:demo.db")
db.QueryRow(`SELECT sqlite_version()`).Scan(&version)
```

### Packages

- [`github.com/ncruces/go-sqlite3`](https://pkg.go.dev/github.com/ncruces/go-sqlite3)
  wraps the [C SQLite API](https://sqlite.org/cintro.html)
  ([example usage](https://pkg.go.dev/github.com/ncruces/go-sqlite3#example-package)).
- [`github.com/ncruces/go-sqlite3/driver`](https://pkg.go.dev/github.com/ncruces/go-sqlite3/driver)
  provides a [`database/sql`](https://pkg.go.dev/database/sql) driver
  ([example usage](https://pkg.go.dev/github.com/ncruces/go-sqlite3/driver#example-package)).
- [`github.com/ncruces/go-sqlite3/embed`](https://pkg.go.dev/github.com/ncruces/go-sqlite3/embed)
  embeds a build of SQLite into your application.
- [`github.com/ncruces/go-sqlite3/vfs`](https://pkg.go.dev/github.com/ncruces/go-sqlite3/vfs)
  wraps the [C SQLite VFS API](https://sqlite.org/vfs.html) and provides a pure Go implementation.
- [`github.com/ncruces/go-sqlite3/gormlite`](https://pkg.go.dev/github.com/ncruces/go-sqlite3/gormlite)
  provides a [GORM](https://gorm.io) driver.

### Advanced features

- [incremental BLOB I/O](https://sqlite.org/c3ref/blob_open.html)
- [nested transactions](https://sqlite.org/lang_savepoint.html)
- [custom functions](https://sqlite.org/c3ref/create_function.html)
- [virtual tables](https://sqlite.org/vtab.html)
- [custom VFSes](https://sqlite.org/vfs.html)
- [online backup](https://sqlite.org/backup.html)
- [JSON support](https://sqlite.org/json1.html)
- [math functions](https://sqlite.org/lang_mathfunc.html)
- [full-text search](https://sqlite.org/fts5.html)
- [geospatial search](https://sqlite.org/geopoly.html)
- [Unicode support](https://pkg.go.dev/github.com/ncruces/go-sqlite3/ext/unicode)
- [statistics functions](https://pkg.go.dev/github.com/ncruces/go-sqlite3/ext/stats)
- [encryption at rest](vfs/adiantum/README.md)
- [many extensions](ext/README.md)
- [custom VFSes](vfs/README.md#custom-vfses)
- [and more…](embed/README.md)

### Caveats

This module replaces the SQLite [OS Interface](https://sqlite.org/vfs.html)
(aka VFS) with a [pure Go](vfs/) implementation,
which has advantages and disadvantages.
Read more about the Go VFS design [here](vfs/README.md).

Because each database connection executes within a Wasm sandboxed environment,
memory usage will be higher than alternatives.

### Testing

This project aims for [high test coverage](https://github.com/ncruces/go-sqlite3/wiki/Test-coverage-report).
It also benefits greatly from [SQLite's](https://sqlite.org/testing.html) and
[wazero's](https://tetrate.io/blog/introducing-wazero-from-tetrate/#:~:text=Rock%2Dsolid%20test%20approach)
thorough testing.

Every commit is [tested](https://github.com/ncruces/go-sqlite3/wiki/Support-matrix) on
Linux (amd64/arm64/386/riscv64/ppc64le/s390x), macOS (arm64/amd64),
Windows (amd64), FreeBSD (amd64/arm64), OpenBSD (amd64), NetBSD (amd64/arm64),
DragonFly BSD (amd64), illumos (amd64), and Solaris (amd64).

The Go VFS is tested by running SQLite's
[mptest](https://github.com/sqlite/sqlite/blob/master/mptest/mptest.c).

### Performance

Performance of the [`database/sql`](https://pkg.go.dev/database/sql) driver is
[competitive](https://github.com/cvilsmeier/go-sqlite-bench) with alternatives.

The Wasm and VFS layers are also benchmarked by running SQLite's
[speedtest1](https://github.com/sqlite/sqlite/blob/master/test/speedtest1.c).

### Concurrency

This module behaves similarly to SQLite in [multi-thread](https://sqlite.org/threadsafe.html) mode:
it is goroutine-safe, provided that no single database connection, or object derived from it,
is used concurrently by multiple goroutines.

The [`database/sql`](https://pkg.go.dev/database/sql) API is safe to use concurrently,
according to its documentation.

### FAQ, issues, new features

For questions, please see [Discussions](https://github.com/ncruces/go-sqlite3/discussions/categories/q-a).

Also, post there if you used this driver for something interesting
([_"Show and tell"_](https://github.com/ncruces/go-sqlite3/discussions/categories/show-and-tell)),
have an [idea](https://github.com/ncruces/go-sqlite3/discussions/categories/ideas)…

The [Issue](https://github.com/ncruces/go-sqlite3/issues) tracker is for bugs,
and features we're working on, planning to work on, or asking for help with.

### Alternatives

- [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite)
- [`crawshaw.io/sqlite`](https://pkg.go.dev/crawshaw.io/sqlite)
- [`github.com/mattn/go-sqlite3`](https://pkg.go.dev/github.com/mattn/go-sqlite3)
- [`github.com/zombiezen/go-sqlite`](https://pkg.go.dev/github.com/zombiezen/go-sqlite)