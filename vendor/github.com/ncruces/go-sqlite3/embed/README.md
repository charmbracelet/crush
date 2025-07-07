# Embeddable Wasm build of SQLite

This folder includes an embeddable Wasm build of SQLite 3.49.1 for use with
[`github.com/ncruces/go-sqlite3`](https://pkg.go.dev/github.com/ncruces/go-sqlite3).

The following optional features are compiled in:
- [math functions](https://sqlite.org/lang_mathfunc.html)
- [FTS5](https://sqlite.org/fts5.html)
- [JSON](https://sqlite.org/json1.html)
- [R*Tree](https://sqlite.org/rtree.html)
- [GeoPoly](https://sqlite.org/geopoly.html)
- [Spellfix1](https://sqlite.org/spellfix1.html)
- [soundex](https://sqlite.org/lang_corefunc.html#soundex)
- [stat4](https://sqlite.org/compile.html#enable_stat4)
- [base64](https://github.com/sqlite/sqlite/blob/master/ext/misc/base64.c)
- [decimal](https://github.com/sqlite/sqlite/blob/master/ext/misc/decimal.c)
- [ieee754](https://github.com/sqlite/sqlite/blob/master/ext/misc/ieee754.c)
- [regexp](https://github.com/sqlite/sqlite/blob/master/ext/misc/regexp.c)
- [series](https://github.com/sqlite/sqlite/blob/master/ext/misc/series.c)
- [uint](https://github.com/sqlite/sqlite/blob/master/ext/misc/uint.c)
- [time](../sqlite3/time.c)

See the [configuration options](../sqlite3/sqlite_opt.h),
and [patches](../sqlite3) applied.

Built using [`wasi-sdk`](https://github.com/WebAssembly/wasi-sdk),
and [`binaryen`](https://github.com/WebAssembly/binaryen).

The build is easily reproducible, and verifiable, using
[Artifact Attestations](https://github.com/ncruces/go-sqlite3/attestations).

### Customizing the build

You can use your own custom build of SQLite.

Examples of custom builds of SQLite are:
- [`github.com/ncruces/go-sqlite3/embed/bcw2`](https://github.com/ncruces/go-sqlite3/tree/main/embed/bcw2)
  built from a branch supporting [`BEGIN CONCURRENT`](https://sqlite.org/src/doc/begin-concurrent/doc/begin_concurrent.md)
  and [Wal2](https://sqlite.org/cgi/src/doc/wal2/doc/wal2.md).
- [`github.com/asg017/sqlite-vec-go-bindings/ncruces`](https://github.com/asg017/sqlite-vec-go-bindings)
  which includes the [`sqlite-vec`](https://github.com/asg017/sqlite-vec) vector search extension.