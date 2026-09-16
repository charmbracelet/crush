package index

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))
}

func TestTagFile_Go(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	src := `package handler

import (
	"fmt"
	"github.com/me/proj/internal/store"
)

type Handler struct {
	store *store.Store
}

func NewHandler(s *store.Store) *Handler { return &Handler{store: s} }

func (h *Handler) get() {}
`
	writeFile(t, root, "internal/api/handler.go", src)
	writeFile(t, root, "internal/store/store.go", "package store\n\ntype Store struct{}\n")
	writeFile(t, root, "go.mod", "module github.com/me/proj\n")

	exists := func(p string) bool {
		return p == "internal/api/handler.go" || p == "internal/store/store.go" ||
			p == "internal" || p == "internal/store" || p == "internal/api"
	}
	tags, refs, err := tagFile(root, "internal/api/handler.go", "github.com/me/proj", exists)
	require.NoError(t, err)

	names := map[string]tag{}
	for _, tg := range tags {
		names[tg.name] = tg
	}
	require.Contains(t, names, "Handler")
	require.Equal(t, "type", names["Handler"].kind)
	require.True(t, names["Handler"].exported)
	require.Contains(t, names, "NewHandler")
	require.True(t, names["NewHandler"].exported)
	require.Contains(t, names, "get")
	require.False(t, names["get"].exported)

	require.Equal(t, []string{"internal/store"}, refs)
}

func TestTagFile_Python(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, "api/views.py", `import models.user
from services.auth import verify

class UserView:
	def get(self): pass

def _helper(): pass

async def list_users(): pass
`)
	writeFile(t, root, "models/user.py", "class User: pass\n")
	writeFile(t, root, "services/auth.py", "def verify(): pass\n")

	exists := func(p string) bool {
		return p == "api/views.py" || p == "models/user.py" || p == "services/auth.py"
	}
	tags, refs, err := tagFile(root, "api/views.py", "", exists)
	require.NoError(t, err)

	names := map[string]tag{}
	for _, tg := range tags {
		names[tg.name] = tg
	}
	require.Contains(t, names, "UserView")
	require.Equal(t, "class", names["UserView"].kind)
	require.Contains(t, names, "get")
	require.Equal(t, "method", names["get"].kind)
	require.Contains(t, names, "list_users")
	require.Contains(t, names, "_helper")
	require.False(t, names["_helper"].exported)

	require.ElementsMatch(t, []string{"models/user.py", "services/auth.py"}, refs)
}

func TestTagFile_TypeScript(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, "src/routes/user.ts", `import { db } from '../db/client'
import express from 'express'

export interface UserRow { id: string }

export async function getUser(id: string) {}

const timeout = 5000

class UserService {
	async find(id: string) {
		doSomething()
	}
	if (x) { return }
}
`)
	writeFile(t, root, "src/db/client.ts", "export const db = {}\n")

	exists := func(p string) bool {
		return p == "src/routes/user.ts" || p == "src/db/client.ts" ||
			p == "src" || p == "src/db" || p == "src/routes"
	}
	tags, refs, err := tagFile(root, "src/routes/user.ts", "", exists)
	require.NoError(t, err)

	names := map[string]tag{}
	for _, tg := range tags {
		names[tg.name] = tg
	}
	require.Contains(t, names, "UserRow")
	require.Equal(t, "type", names["UserRow"].kind)
	require.True(t, names["UserRow"].exported)
	require.Contains(t, names, "getUser")
	require.Contains(t, names, "timeout")
	require.False(t, names["timeout"].exported)
	require.Contains(t, names, "find")
	require.Equal(t, "method", names["find"].kind)
	// Control-flow keywords must not leak in as methods.
	require.NotContains(t, names, "if")
	require.NotContains(t, names, "return")
	// Call sites must not tag as method definitions.
	require.NotContains(t, names, "doSomething")

	require.Equal(t, []string{"src/db/client.ts"}, refs)
}

func TestIndexRoundTrip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dataDir := t.TempDir()

	writeFile(t, root, "go.mod", "module example.com/proj\n")
	writeFile(t, root, "main.go", `package main

import "example.com/proj/internal/api"

func main() {}
`)
	writeFile(t, root, "internal/api/handler.go", `package api

type Handler struct{}

func NewHandler() *Handler { return nil }
`)

	svc, err := Open(dataDir, root)
	require.NoError(t, err)
	defer svc.Close()

	ctx := context.Background()
	require.NoError(t, svc.EnsureIndexed(ctx))

	skel, err := svc.Skeleton(ctx, 500)
	require.NoError(t, err)
	require.Contains(t, skel, "internal/api")
	require.Contains(t, skel, "main.go")

	sym, err := svc.Symbol(ctx, "Handler", 500)
	require.NoError(t, err)
	require.Contains(t, sym, "internal/api/handler.go")

	sub, err := svc.Subtree(ctx, "internal/api", 500)
	require.NoError(t, err)
	require.Contains(t, sub, "NewHandler")

	// Staleness: edit the file — the next query must refresh it
	// lazily (refresh-before-render ordering), no manual refresh.
	writeFile(t, root, "internal/api/handler.go", `package api

type Handler struct{}

func NewHandler() *Handler { return nil }
func (h *Handler) ServeHTTP() {}
`)
	sub, err = svc.Subtree(ctx, "internal/api", 500)
	require.NoError(t, err)
	require.Contains(t, sub, "ServeHTTP")

	// New symbol in an already-indexed file: a Symbol miss triggers
	// the dirty-scan fallback and finds it without a re-walk.
	writeFile(t, root, "internal/api/handler.go", `package api

type Handler struct{}

func NewHandler() *Handler { return nil }
func (h *Handler) ServeHTTP() {}
func Extra() {}
`)
	sym, err = svc.Symbol(ctx, "Extra", 500)
	require.NoError(t, err)
	require.Contains(t, sym, "internal/api/handler.go")

	// Go dir refs must survive lazy re-tagging: edit main.go, let a
	// query refresh it, then Handler's referrers still include it.
	writeFile(t, root, "main.go", `package main

import "example.com/proj/internal/api"

func main() { api.NewHandler() }
`)
	_, err = svc.Symbol(ctx, "main", 500)
	require.NoError(t, err)
	sym, err = svc.Symbol(ctx, "Handler", 500)
	require.NoError(t, err)
	require.Contains(t, sym, "main.go")

	// Root listing: "." and "/" address the project root, not an
	// empty result.
	sub, err = svc.Subtree(ctx, ".", 500)
	require.NoError(t, err)
	require.Contains(t, sub, "main.go")
	sub, err = svc.Subtree(ctx, "/", 500)
	require.NoError(t, err)
	require.Contains(t, sub, "main.go")

	// Oversized files are recorded but not tagged: a fresh service
	// re-walks and big.go appears in listings without symbols.
	writeFile(t, root, "big.go", "package main\n// "+strings.Repeat("x", 300*1024))
	svc2, err := Open(dataDir, root)
	require.NoError(t, err)
	defer svc2.Close()
	require.NoError(t, svc2.EnsureIndexed(ctx))
	sub, err = svc2.Subtree(ctx, ".", 500)
	require.NoError(t, err)
	require.Contains(t, sub, "big.go")
}

func TestTagFile_Rust(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, "src/lib.rs", `use crate::a::b::Item;
mod util;

pub struct Root;

pub fn run() {}
`)
	writeFile(t, root, "src/a/b.rs", "pub struct Item;\n")
	writeFile(t, root, "src/util.rs", "pub fn help() {}\n")

	exists := func(p string) bool {
		switch p {
		case "src/lib.rs", "src/a/b.rs", "src/util.rs",
			"src", "src/a", "src/a/b":
			return true
		}
		return false
	}
	tags, refs, err := tagFile(root, "src/lib.rs", "", exists)
	require.NoError(t, err)

	names := map[string]tag{}
	for _, tg := range tags {
		names[tg.name] = tg
	}
	require.Contains(t, names, "Root")
	require.True(t, names["Root"].exported)
	require.Contains(t, names, "run")
	require.Contains(t, names, "util")

	// `use crate::a::b::Item` must resolve to the containing module
	// file — the last path segment is an item, not a file. `mod util`
	// names the file directly.
	require.ElementsMatch(t, []string{"src/a/b.rs", "src/util.rs"}, refs)
}

func TestTagFile_GoImportBlock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, "main.go", `package main

import (
	"example.com/proj/internal/store"
)

var url = "example.com/proj/internal/other"

func main() {
	switch url {
	case "example.com/proj/internal/third":
	}
}
`)
	writeFile(t, root, "go.mod", "module example.com/proj\n")
	writeFile(t, root, "internal/store/store.go", "package store\n")
	writeFile(t, root, "internal/other/other.go", "package other\n")
	writeFile(t, root, "internal/third/third.go", "package third\n")

	exists := func(p string) bool {
		switch p {
		case "main.go", "internal/store", "internal/other", "internal/third",
			"internal":
			return true
		}
		return false
	}
	_, refs, err := tagFile(root, "main.go", "example.com/proj", exists)
	require.NoError(t, err)

	// String literals outside the import block are not imports.
	require.Equal(t, []string{"internal/store"}, refs)
}

func TestTagFile_GoImportBlockEdgeCases(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// A `) // comment` closer and a single-line `import ("x")` must
	// not stick the gate open — literals after them aren't imports.
	writeFile(t, root, "a.go", `package main

import (
	"example.com/proj/internal/store"
) // grouped imports

var url = "example.com/proj/internal/other"
`)
	writeFile(t, root, "b.go", `package main

import ("example.com/proj/internal/third")

var dep = "example.com/proj/internal/other"
`)
	writeFile(t, root, "go.mod", "module example.com/proj\n")
	writeFile(t, root, "internal/store/store.go", "package store\n")
	writeFile(t, root, "internal/other/other.go", "package other\n")
	writeFile(t, root, "internal/third/third.go", "package third\n")

	exists := func(p string) bool {
		switch p {
		case "a.go", "b.go", "internal/store", "internal/other",
			"internal/third", "internal":
			return true
		}
		return false
	}

	_, refs, err := tagFile(root, "a.go", "example.com/proj", exists)
	require.NoError(t, err)
	require.Equal(t, []string{"internal/store"}, refs)

	// The single-line form still captures its spec, and the gate
	// stays closed for the literal below it.
	_, refs, err = tagFile(root, "b.go", "example.com/proj", exists)
	require.NoError(t, err)
	require.Equal(t, []string{"internal/third"}, refs)
}

func TestTagFile_Java(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, "src/main/java/com/ex/UserService.java", `package com.ex;

import com.ex.util.Helper;

public class UserService {
	public void find() {}
}
`)
	writeFile(t, root, "src/main/java/com/ex/util/Helper.java",
		"package com.ex.util;\npublic class Helper {}\n")

	exists := func(p string) bool {
		switch p {
		case "src/main/java/com/ex/UserService.java",
			"src/main/java/com/ex/util/Helper.java":
			return true
		}
		return false
	}
	tags, refs, err := tagFile(root, "src/main/java/com/ex/UserService.java", "", exists)
	require.NoError(t, err)

	names := map[string]tag{}
	for _, tg := range tags {
		names[tg.name] = tg
	}
	require.Contains(t, names, "UserService")
	require.Equal(t, "type", names["UserService"].kind)
	require.True(t, names["UserService"].exported)
	require.Equal(t, []string{"src/main/java/com/ex/util/Helper.java"}, refs)
}

func TestSubtree_PathHandling(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dataDir := t.TempDir()

	writeFile(t, root, "main.go", "package main\n\nfunc main() {}\n")
	// LIKE metacharacters in the directory name must stay literal.
	writeFile(t, root, "100%_real/f.go", "package real\n\nfunc F() {}\n")
	writeFile(t, root, "100Xreal/g.go", "package other\n\nfunc G() {}\n")

	svc, err := Open(dataDir, root)
	require.NoError(t, err)
	defer svc.Close()
	ctx := context.Background()
	require.NoError(t, svc.EnsureIndexed(ctx))

	// Absolute and escaping paths get guidance, not empty results.
	out, err := svc.Subtree(ctx, "/etc", 500)
	require.NoError(t, err)
	require.Contains(t, out, "absolute")
	out, err = svc.Subtree(ctx, "../outside", 500)
	require.NoError(t, err)
	require.Contains(t, out, "escapes")

	// A literal "%_" name matches itself only — not the 100Xreal dir.
	out, err = svc.Subtree(ctx, "100%_real", 500)
	require.NoError(t, err)
	require.Contains(t, out, "f.go")
	require.NotContains(t, out, "g.go")

	// A file argument renders the file's symbols, not a dir listing.
	out, err = svc.Subtree(ctx, "main.go", 500)
	require.NoError(t, err)
	require.Contains(t, out, "File main.go:")
	require.Contains(t, out, "main")
}

func TestNotifyWritten_DropsDeleted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dataDir := t.TempDir()

	writeFile(t, root, "gone.go", "package main\n\nfunc Gone() {}\n")
	writeFile(t, root, "keep.go", "package main\n\nfunc Keep() {}\n")

	svc, err := Open(dataDir, root)
	require.NoError(t, err)
	defer svc.Close()
	ctx := context.Background()
	require.NoError(t, svc.EnsureIndexed(ctx))

	_, _, found := svc.indexedFile(ctx, "gone.go")
	require.True(t, found)

	require.NoError(t, os.Remove(filepath.Join(root, "gone.go")))
	svc.touchFile(filepath.Join(root, "gone.go"))

	_, _, found = svc.indexedFile(ctx, "gone.go")
	require.False(t, found)
}

func TestSkeleton_DuringBuild(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dataDir := t.TempDir()

	// Enough files that the build is still in flight when the query
	// lands — the point is that reads don't block on the build.
	for i := range 400 {
		writeFile(t, root, fmt.Sprintf("pkg/f%04d.go", i),
			fmt.Sprintf("package pkg\n\nfunc F%d() {}\n", i))
	}

	svc, err := Open(dataDir, root)
	require.NoError(t, err)
	defer svc.Close()
	ctx := context.Background()

	done := make(chan string, 1)
	go func() {
		out, err := svc.Skeleton(ctx, 500)
		if err != nil {
			done <- "ERR:" + err.Error()
			return
		}
		done <- out
	}()

	var out string
	select {
	case out = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("Skeleton blocked on the in-flight build")
	}
	require.Contains(t, out, "Project map —")

	// Repeated queries during the build must all return — partial
	// results are the design, blocking is the bug.
	deadline := time.Now().Add(30 * time.Second)
	for svc.indexing.Load() && time.Now().Before(deadline) {
		sub, err := svc.Subtree(ctx, "pkg", 500)
		require.NoError(t, err)
		require.NotEmpty(t, sub)
	}
	require.NoError(t, svc.EnsureIndexed(ctx))
}

func TestCapOutput_UTF8(t *testing.T) {
	t.Parallel()

	// Multibyte rune straddling the cut — output stays valid UTF-8.
	out := capOutput(strings.Repeat("é", 100), 10) // 40-byte cap
	require.True(t, utf8.ValidString(out))
	require.Contains(t, out, "truncated")

	// An invalid byte early in the string must not eat the output.
	out = capOutput("a\xff"+strings.Repeat("x", 100), 10)
	require.Contains(t, out, "truncated")
	require.Greater(t, len(out), 30)
}

func TestTagFile_RustImplFor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, "src/foo.rs", `pub struct Foo;

impl Foo {
	pub fn new() -> Self { Self {} }
}

impl Display for Foo {
	fn fmt(&self) {}
}

fn publish() {}
`)
	exists := func(string) bool { return true }
	tags, _, err := tagFile(root, "src/foo.rs", "", exists)
	require.NoError(t, err)

	impls := map[string]bool{}
	for _, tg := range tags {
		if tg.kind == "impl" {
			impls[tg.name] = true
		}
	}
	// `impl Display for Foo` must tag Foo, not Display.
	require.True(t, impls["Foo"])
	require.False(t, impls["Display"])

	// Word-boundary export check: `publish` is not `pub`.
	for _, tg := range tags {
		if tg.name == "publish" {
			require.False(t, tg.exported)
		}
	}
}

func TestTagFile_PythonRelative(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, "src/mypkg/a.py", `from .sibling import helper
from ..top import util
from mypkg.other import thing
`)
	writeFile(t, root, "src/mypkg/sibling.py", "def helper(): pass\n")
	writeFile(t, root, "src/top.py", "def util(): pass\n")
	writeFile(t, root, "src/mypkg/other.py", "def thing(): pass\n")

	exists := func(p string) bool {
		switch p {
		case "src/mypkg/a.py", "src/mypkg/sibling.py", "src/top.py",
			"src/mypkg/other.py":
			return true
		}
		return false
	}
	_, refs, err := tagFile(root, "src/mypkg/a.py", "", exists)
	require.NoError(t, err)

	// `.sibling` resolves beside the file; `..top` one level up;
	// absolute `mypkg.other` resolves under the src/ layout.
	require.ElementsMatch(t, []string{
		"src/mypkg/sibling.py", "src/top.py", "src/mypkg/other.py",
	}, refs)
}

func TestTouchFile_NoDBNoIndex(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dataDir := t.TempDir()

	writeFile(t, root, "main.go", "package main\n\nfunc main() {}\n")

	// Shared-style lazy service: the DB is created on first index
	// use, not at construction.
	svc := newService(dataDir, root)
	defer svc.Close()

	// With the DB never created (map disabled / never called), a
	// write notification must not create index.db.
	svc.touchFile(filepath.Join(root, "main.go"))
	_, err := os.Stat(filepath.Join(dataDir, IndexFilename))
	require.True(t, errors.Is(err, fs.ErrNotExist))
}

func TestTouchFile_ZeroByteDrops(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dataDir := t.TempDir()

	writeFile(t, root, "f.go", "package main\n\nfunc F() {}\n")

	svc, err := Open(dataDir, root)
	require.NoError(t, err)
	defer svc.Close()
	ctx := context.Background()
	require.NoError(t, svc.EnsureIndexed(ctx))

	_, _, found := svc.indexedFile(ctx, "f.go")
	require.True(t, found)

	// Truncating to zero bytes drops the row immediately — the walk
	// never indexes empty files.
	require.NoError(t, os.WriteFile(filepath.Join(root, "f.go"), nil, 0o644))
	svc.touchFile(filepath.Join(root, "f.go"))

	_, _, found = svc.indexedFile(ctx, "f.go")
	require.False(t, found)
}

func TestTouchFile_AncestorDirSkipped(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dataDir := t.TempDir()

	writeFile(t, root, "main.go", "package main\n\nfunc main() {}\n")
	// Not gitignored — only the built-in dir-level ignore applies.
	writeFile(t, root, "node_modules/pkg/index.js", "exports.x = 1\n")

	svc, err := Open(dataDir, root)
	require.NoError(t, err)
	defer svc.Close()
	ctx := context.Background()
	require.NoError(t, svc.EnsureIndexed(ctx))

	// The walk never emits files under node_modules; a write
	// notification must not upsert one either.
	svc.touchFile(filepath.Join(root, "node_modules/pkg/index.js"))
	_, _, found := svc.indexedFile(ctx, "node_modules/pkg/index.js")
	require.False(t, found)

	// A write under a normal directory still lands.
	svc.touchFile(filepath.Join(root, "main.go"))
	_, _, found = svc.indexedFile(ctx, "main.go")
	require.True(t, found)
}

func TestWalk_ReconcileKeepsLiveRows(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dataDir := t.TempDir()

	writeFile(t, root, "main.go", "package main\n\nfunc main() {}\n")
	// A path the walk never collects (ignored dir) but that exists
	// on disk — simulates a NotifyWritten row committed mid-walk.
	writeFile(t, root, "node_modules/pkg/x.js", "export const x = 1\n")

	svc, err := Open(dataDir, root)
	require.NoError(t, err)
	defer svc.Close()
	ctx := context.Background()
	require.NoError(t, svc.EnsureIndexed(ctx))

	// Simulate the mid-walk write: a row lands for a path collect
	// already passed. Stat must keep it — a live file's row is
	// stale-healable, an erased row is invisible until next write.
	_, err = svc.db.ExecContext(ctx,
		`INSERT INTO files (path, mtime, size, indexed_at) VALUES (?, 1, 1, 1)`,
		"node_modules/pkg/x.js")
	require.NoError(t, err)

	require.NoError(t, svc.walk(ctx))
	_, _, found := svc.indexedFile(ctx, "node_modules/pkg/x.js")
	require.True(t, found)
}

func TestWalk_RefixResolvesMidBuildTags(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dataDir := t.TempDir()

	writeFile(t, root, "go.mod", "module example.com/p\n")
	writeFile(t, root, "a/a.go", `package a

import "example.com/p/b"

func A() { b.B() }
`)
	writeFile(t, root, "b/b.go", "package b\n\nfunc B() {}\n")

	svc, err := Open(dataDir, root)
	require.NoError(t, err)
	defer svc.Close()
	ctx := context.Background()

	// Lazily tag a.go while a build is "in flight" — the ref target
	// isn't indexed yet, so the ref drops and refix records it.
	svc.indexing.Store(true)
	info, err := os.Stat(filepath.Join(root, "a/a.go"))
	require.NoError(t, err)
	svc.refreshIfStale(ctx, "a/a.go", info.ModTime().UnixNano(), info.Size())
	svc.indexing.Store(false)

	// Without the refix pass this row would stay missing — the
	// file's {mtime,size} pair matches, so walks never re-tag it.
	require.NoError(t, svc.walk(ctx))

	var dst string
	err = svc.db.QueryRowContext(ctx,
		`SELECT dst_path FROM refs WHERE src_path = 'a/a.go'`).Scan(&dst)
	require.NoError(t, err)
	require.Equal(t, "b", dst)
}

func TestChurnRanking(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	dataDir := t.TempDir()

	// a.go and b.go carry no refs — pure in-degree leaves them tied;
	// b.go's commit history must lift it first in the ranking.
	writeFile(t, root, "a.go", "package main\n")
	writeFile(t, root, "b.go", "package main\n")

	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s: %s", args, out)
	}
	git("init", "-q")
	git("add", ".")
	git("commit", "-qm", "init")
	for i := range 3 {
		writeFile(t, root, "b.go", fmt.Sprintf("package main\n\nvar v%d int\n", i))
		git("add", "b.go")
		git("commit", "-qm", "touch")
	}

	svc, err := Open(dataDir, root)
	require.NoError(t, err)
	defer svc.Close()
	ctx := context.Background()
	require.NoError(t, svc.EnsureIndexed(ctx))

	skel, err := svc.Skeleton(ctx, 500)
	require.NoError(t, err)
	require.Contains(t, skel, ", 4 commits") // init + 3 touch commits
	require.Less(t, strings.Index(skel, "b.go"), strings.Index(skel, "a.go"),
		"churned file must outrank the untouched one: %s", skel)
}
