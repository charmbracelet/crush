# Plan: Debug File History Issues

## Problem 1: UNIQUE Constraint Failure

The edit tool returns: `error creating file history: constraint failed: UNIQUE constraint failed: files.path, files.session_id, files.version (2067)`

## Identified Likely Root Cause

There are **two interacting issues**:

### Issue 1: `createNewFile()` unconditionally inserts version 0

In `edit.go:163-164`, the `createNewFile()` path calls `files.Create()` (version 0) **without first checking** if the file already exists in history for this session. The `deleteContent()` and `replaceContent()` paths DO check via `GetByPathAndSession()`, but `createNewFile()` does not.

If the same file path gets "created" twice in the same session (e.g., created → deleted → created again, or old_string="" used on an already-tracked file), version 0 already exists and the UNIQUE constraint fires.

### Issue 2: `CreateVersion()` uses cross-session version numbers

`CreateVersion()` calls `ListFilesByPath()` which returns files from **all sessions** for the given path. It takes the max version across all sessions and adds 1. But the UNIQUE constraint is `(path, session_id, version)` — scoped to a single session.

This means: if session A has versions 0-5 for a file, and session B starts tracking the same file, `CreateVersion()` will try to insert version 6 for session B. This works, but the version numbers become sparse and inconsistent. And if the retry logic bumps into a version that happens to already exist in the current session, it fails.

### Issue 3: The retry loop may be insufficient

`createWithVersion()` retries 3 times, incrementing version each time. If there are already 3+ versions for `(path, sessionID)` starting from the attempted version, all retries are exhausted.

## Plan

### Step 1: Confirm the bug with a unit test

Write a test in `internal/history/file_test.go` (or a new test file) that reproduces the exact scenario:
- Create a session
- Call `Create()` for a file path (version 0)
- Call `Create()` again for the same path+session → should trigger the UNIQUE constraint

Also test the cross-session version scenario:
- Session A: Create + CreateVersion for a file (versions 0, 1)
- Session B: Create for same file → should this succeed? What version does `CreateVersion()` pick?

### Step 2: Fix `createNewFile()` to check history first

Add a `GetByPathAndSession()` check in `createNewFile()` before calling `Create()`, similar to how `deleteContent()` and `replaceContent()` work.

### Step 3: Scope `CreateVersion()` to the current session

Change `CreateVersion()` to call a session-scoped query (e.g., `ListFilesByPathAndSession`) instead of the global `ListFilesByPath()`. This ensures version numbers are consistent per-session.

Alternatively, add a new SQL query `ListFilesByPathAndSession` that filters by both path AND session_id.

### Step 4: Make the retry loop more robust

Consider either:
- Increasing max retries
- Using a SELECT MAX(version) + 1 within the transaction itself (atomic)
- Using INSERT OR REPLACE / ON CONFLICT semantics

### Step 5: Audit write.go and multiedit.go

These tools have the same file history pattern. Ensure the same fix applies.

### Step 6: Run full test suite

Verify no regressions.

---

## Problem 2: Data Race in `goose` Global State (CI `-race` Failure)

The `go test -race -failfast ./...` CI step fails with a data race in `internal/history` tests. The race is between `goose.SetBaseFS()` (a write to the package-level `baseFS` variable) in one goroutine and `goose.CollectMigrations()` (a read of `baseFS`) in another.

### Root Cause

`goose.SetBaseFS(FS)` and `goose.SetDialect("sqlite3")` in `db.Connect()` write to **unsynchronized package-level global variables** in the `goose` package. When multiple parallel tests (`t.Parallel()`) each call `db.Connect()` via `setupTest()`, these global writes race with each other and with reads inside `goose.Up()`.

Specifically:
- `goose.SetBaseFS(FS)` writes to `var baseFS fs.FS` (goose.go:23)
- `goose.SetDialect("sqlite3")` writes to `var store legacystore.Store` (dialect.go:37)
- `goose.CollectMigrations()` reads `baseFS` (migrate.go:176)

All four tests in `internal/history/file_test.go` use `t.Parallel()` and each calls `setupTest()` → `db.Connect()`, causing concurrent writes to these globals.

### Fix

Use `sync.Once` in `db.Connect()` to ensure `goose.SetBaseFS()` and `goose.SetDialect()` are called exactly once, since the values never change (always `FS` and `"sqlite3"`).

### Step 7: Fix the goose global state data race and verify

Wrap the `goose.SetBaseFS(FS)` and `goose.SetDialect("sqlite3")` calls in a `sync.Once` inside `db.Connect()` so they execute exactly once across all goroutines. Then run `go test -race ./internal/history/...` and `go test -race ./...` to confirm the data race is resolved.
