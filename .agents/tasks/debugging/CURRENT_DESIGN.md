# Current Design: File History System

## Schema

The `files` table (`internal/db/migrations/20250424200609_initial.sql:24-34`):

```sql
CREATE TABLE IF NOT EXISTS files (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    path TEXT NOT NULL,
    content TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE,
    UNIQUE(path, session_id, version)
);
```

The UNIQUE constraint is on `(path, session_id, version)`.

## The Bug

**Error:** `error creating file history: constraint failed: UNIQUE constraint failed: files.path, files.session_id, files.version (2067)`

This occurs in `edit.go` at lines 167, 292, or 423 when `Create()` (which inserts version 0) hits the UNIQUE constraint.

## Root Cause Analysis

The `Create()` method (`history/file.go:58`) always inserts version 0. The call sites in `edit.go` (e.g. line 163-164, 289, 420) call `Create()` when `GetByPathAndSession()` returns an error (file not yet in history for this session).

**However**, the `createNewFile()` path at line 163-164 does NOT check `GetByPathAndSession()` first — it unconditionally calls `Create()` with version 0. If the same file path was already created (version 0) in the same session (e.g., the file was created, deleted, and created again), this will hit the UNIQUE constraint.

More critically for the reported scenario: `CreateVersion()` (`history/file.go:65-81`) calls `ListFilesByPath()` which queries **all sessions** for the path (not scoped to current session). It then uses the **global** max version + 1. But the UNIQUE constraint is on `(path, session_id, version)` — so the version number from another session might already exist in the current session.

### Scenario that triggers the bug

1. Session A edits file `/foo/bar.astro`, creating versions 0, 1, 2 in session A.
2. Session B (new session via `new_session` tool) edits the same file.
3. `GetByPathAndSession()` returns error (file not in session B's history).
4. `Create()` is called with version 0 for session B — this **succeeds** (unique tuple `(path, sessionB, 0)` is new).
5. `CreateVersion()` is called → `ListFilesByPath()` returns files from **both sessions**, latest version is 2 (from session A).
6. Inserts version 3 for session B — **succeeds**.
7. Later, another edit in session B calls `Create()` for same path... but version 0 already exists for `(path, sessionB, 0)` → **UNIQUE constraint failure**.

Wait — actually the guard `GetByPathAndSession()` should prevent re-calling `Create()`. Let me reconsider.

### Alternative scenario (more likely)

The `createNewFile()` path (line 110-185) does NOT check history first — it unconditionally calls `Create(ctx, sessionID, filePath, "")`. If the agent creates a file that was already tracked in history for this session (e.g., creates a file, then the edit tool's `createNewFile` is called again for the same path in the same session), the UNIQUE constraint fires because `(path, sessionID, 0)` already exists.

This could happen if:
- The agent creates a new file (version 0 inserted)
- The agent later edits the same file, and the edit path ends up in `createNewFile()` again (old_string is empty)
- Version 0 already exists → constraint violation

### The retry logic

`createWithVersion()` has a 3-retry loop that increments version on UNIQUE failure. But `Create()` starts at version 0, and if version 0 already exists, it tries 1, then 2. If versions 0, 1, and 2 all exist, it fails after 3 attempts.

## Key Files

| File | Role |
|------|------|
| `internal/history/file.go` | File history service — `Create()`, `CreateVersion()`, `createWithVersion()` |
| `internal/db/sql/files.sql` | SQL queries — notably `ListFilesByPath` is NOT session-scoped |
| `internal/agent/tools/edit.go` | Edit tool — `createNewFile()`, `deleteContent()`, `replaceContent()` |
| `internal/agent/tools/write.go` | Write tool — similar file history pattern |
| `internal/agent/tools/multiedit.go` | Multiedit tool — similar file history pattern |
| `internal/agent/tools/new_session.go` | New session tool — creates fresh session, old files may be re-edited |
| `internal/db/migrations/20250424200609_initial.sql` | Schema with UNIQUE constraint |
