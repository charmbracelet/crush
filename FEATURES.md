# Feature Differences from Upstream

This document describes the features added in this fork compared to
[charmbracelet/crush](https://github.com/charmbracelet/crush).

---

## Module Rename

All import paths changed:

| Upstream | Fork |
|----------|------|
| `github.com/charmbracelet/crush` | `github.com/taigrr/crush` |
| `charm.land/fantasy` | `github.com/taigrr/fantasy` |
| `charm.land/catwalk` | `github.com/taigrr/catwalk` |

Charm dependencies that are **not** forked (bubbletea, lipgloss, glamour,
fang, bubbles) remain on their upstream `charm.land/` paths.

---

## Telemetry Removal

The entire `internal/event/` package has been deleted. This removes:

- PostHog analytics client and all event tracking
- Machine ID fingerprinting (`machineid`, MAC address hashing)
- All `event.Init()`, `event.AppInitialized()`, `event.PromptSent()`,
  `event.Error()`, etc. call sites throughout the codebase
- The `posthog-go` dependency

No usage data leaves the machine.

---

## Provider Simplification

`internal/config/provider.go` has been reduced from ~236 lines to ~33 lines.

**Removed:**

- Network-based provider fetching (Catwalk URL sync, ETag caching)
- `UpdateProviders()` function
- `HyperSync()` and the entire `internal/config/hyper.go` module
- `internal/config/catwalk.go` (Catwalk URL resolution logic)
- Local JSON caching of provider data
- Background provider refresh

**Kept:**

- `embedded.GetAll()` — providers are compiled in from the catwalk
  embedded catalog. To update providers, update the `catwalk` dependency.

---

## Extended Context (1M Token Support)

`internal/agent/agent.go` gains dynamic context window management for
models that support 1M context (e.g., Gemini, Claude with beta flags).

### Context Modes

Configurable per-model via `context_mode` in `crush.json`:

| Mode | Behavior |
|------|----------|
| `standard` | Use the model's default context window |
| `extended` | Always use 1M context |
| `dynamic` | Auto-switch to 1M when 80% of standard window is consumed; summarize at 90% of 1M |

### Implementation

- New constants: `extendedContextWindow` (1M), switch ratio (0.8),
  summarize ratio (0.9)
- `extendedContextMode` map tracks per-session extended state
- `useExtendedContext()` injects beta flags for providers that require
  them (e.g., `context-1m-2025-08-07` for Anthropic)
- `IsExtendedContext(sessionID)` exposed on the agent interface

### UI

- New **Context Mode** dialog (`internal/ui/dialog/context_mode.go`) for
  switching between Standard/Extended/Dynamic
- Status bar shows current context mode

---

## Checkpoints & Snapshots

New package: `internal/checkpoint/`

A private git repository (stored in `.crush/git/`, never touches the
user's `.git/`) that captures filesystem state at each user message.

### Capabilities

- **Create snapshots** — content-addressed tree/blob storage via go-git
- **Restore snapshots** — checkout any historical state
- **Diff snapshots** — unified diff between any two points
- **Named refs** — `refs/snapshots/{session_id}/{message_id}`
- **Session-scoped listing** — list all snapshots for a session
- **Configurable exclusions** — `node_modules`, `vendor`, `dist`, etc.
  via `snapshots.exclude` in config

### Components

| File | Purpose |
|------|---------|
| `checkpoint/repo.go` | go-git repository wrapper (622 lines) |
| `checkpoint/repo_test.go` | Full test coverage (560 lines) |
| `checkpoint/service.go` | High-level snapshot service (521 lines) |
| `checkpoint/service_test.go` | Service tests |

### Database

Migration `20260511112917_add_snapshots_table.sql` adds:

```sql
CREATE TABLE snapshots (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    parent_snapshot_id TEXT,
    git_commit_hash TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL
);
```

---

## Worktrees

New package: `internal/worktree/`

Manages isolated working directories (stored in `.crush/worktrees/`) so
multiple conversation branches can have independent filesystem state.

### Capabilities

- **Create worktrees** from any snapshot
- **Switch** between worktrees (moves uncommitted changes)
- **Delete** worktrees
- **Auto-name generation** via small model (conventional commit style)
- **Post-create hooks** — run `npm install`, `go mod download`, etc.
  based on lockfile detection
- **Merge worktrees** — bring changes from one worktree back
- **Startup validation** — detect orphaned/stale worktrees

### Database

Migration `20260511114224_add_worktrees_table.sql` adds:

```sql
CREATE TABLE worktrees (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    current_snapshot_id TEXT,
    is_active BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL
);
```

### UI Dialogs

- `internal/ui/dialog/worktrees.go` — worktree list/switch/merge dialog
- `internal/ui/dialog/merge_worktree.go` — merge confirmation dialog

---

## Conversation Forking

New package: `internal/fork/`

Fork a conversation from any message, creating a new session that shares
history up to the fork point.

### Flow

1. User invokes fork on a message
2. Creates a snapshot at that message (if not exists)
3. Creates a new session with `forked_from_snapshot_id`
4. Optionally creates a new worktree from the snapshot
5. Switches UI to the new session

### UI

- `internal/ui/dialog/fork.go` — fork dialog (session name, worktree
  toggle)
- Fork action available on each user message in the chat view

---

## Archived Sessions

`internal/session/session.go` adds:

- `Archive(ctx, id)` — soft-delete by setting `archived_at`
- `Unarchive(ctx, id)` — restore archived session
- `ListArchived(ctx)` — list only archived sessions

Database migration `20260512141646_add_session_archived_at.sql` adds the
`archived_at` column.

The session list dialog (`internal/ui/dialog/sessions.go`) has been
significantly expanded (~300 lines added) with archive/unarchive actions.

---

## Server Snapshot & Worktree API

`internal/server/snapshots.go` (451 lines) adds HTTP endpoints:

| Method | Path | Description |
|--------|------|-------------|
| POST | `/workspaces/{id}/snapshots` | Create snapshot |
| GET | `/workspaces/{id}/snapshots` | List snapshots |
| GET | `/workspaces/{id}/snapshots/{sid}` | Get snapshot |
| POST | `/workspaces/{id}/snapshots/{sid}/restore` | Restore snapshot |
| GET | `/workspaces/{id}/snapshots/{sid}/diff` | Diff against snapshot |
| POST | `/workspaces/{id}/worktrees` | Create worktree |
| GET | `/workspaces/{id}/worktrees` | List worktrees |
| DELETE | `/workspaces/{id}/worktrees/{wid}` | Delete worktree |
| POST | `/workspaces/{id}/worktrees/{wid}/switch` | Switch worktree |
| POST | `/workspaces/{id}/sessions/{sid}/fork` | Fork session |

Backend support: `internal/backend/snapshot.go`, `backend/worktree.go`,
`backend/fork.go`.

Client SDK: `internal/client/snapshots.go` (341 lines).

---

## Shell Improvements

- `internal/shell/process_unix.go` / `process_other.go` — platform-
  specific process group handling
- `internal/shell/background.go` — expanded background job management

---

## `app_workspace.go` Removal

The standalone `internal/workspace/app_workspace.go` (406 lines) has been
deleted. Its functionality is consolidated into the existing
`client_workspace.go` which grew to accommodate the new workspace
interface methods (worktrees, snapshots, forking, context mode).

---

## Workspace Interface Additions

`internal/workspace/workspace.go` gains these methods:

```go
// Session management
ListArchivedSessions(ctx) ([]session.Session, error)
ArchiveSession(ctx, id) error
UnarchiveSession(ctx, id) error
SetActiveSessionID(sessionID string)
ActiveSessionID() string

// Extended context
AgentIsExtendedContext(sessionID string) bool

// Snapshots
ListSnapshots(ctx, sessionID) ([]*checkpoint.Snapshot, error)
CreateSnapshot(ctx, sessionID, messageID, description) (*checkpoint.Snapshot, error)
RestoreSnapshot(ctx, snapshotID) error
DiffSnapshot(ctx, snapshotID) (string, error)

// Worktrees
ListAllWorktrees(ctx) ([]*worktree.Worktree, error)
CreateWorktree(ctx, sessionID, name, fromSnapshotID) (*worktree.Worktree, error)
SwitchWorktree(ctx, sessionID, worktreeID) error
DeleteWorktree(ctx, worktreeID) error
MergeWorktree(ctx, worktreeID) error

// Forking
ForkSession(ctx, sessionID, messageID, title, createWorktree) (session.Session, error)
```

---

## UI Additions

### New Dialogs

| File | Purpose |
|------|---------|
| `dialog/context_mode.go` | Select context mode (Standard/Extended/Dynamic) |
| `dialog/fork.go` | Fork conversation from a message |
| `dialog/snapshots.go` | Browse and restore snapshots |
| `dialog/worktrees.go` | List, switch, delete, merge worktrees |
| `dialog/merge_worktree.go` | Merge confirmation with options |

### Expanded Dialogs

- `dialog/sessions.go` — archive/unarchive, fork actions (+297 lines)
- `dialog/permissions.go` — refactored permission UI (+127/-67 lines)

### Model & Keybindings

- `model/ui.go` — major expansion (+429 lines) wiring new features
- `model/keys.go` — new keybindings for worktree/snapshot/fork/context
- `model/header.go` — worktree/branch indicators in status bar
- `model/sidebar.go` — expanded sidebar with worktree info

### Styles

- `styles/grad.go` — new gradient utilities (50 lines)
- `styles/quickstyle.go` — additional quick style helpers

---

## Configuration Additions

New fields in `crush.json`:

```json
{
  "snapshots": {
    "enabled": true,
    "exclude": ["node_modules", "vendor", "dist", "build", ...]
  },
  "worktree": {
    "post_create": [
      {"if_exists": "bun.lockb", "run": "bun i"},
      {"if_exists": "go.sum", "run": "go mod download"}
    ]
  }
}
```

Per-model `context_mode` field: `"standard"`, `"extended"`, or
`"dynamic"`.

---

## Spec Document

Full design spec: `docs/specs/WORKTREES_AND_SNAPSHOTS.md` (643 lines)
covering architecture, flows, UI wireframes, API design, and
implementation phases.

---

## Summary by the Numbers

| Metric | Value |
|--------|-------|
| Files changed | 321 |
| Lines added | ~10,986 |
| Lines removed | ~4,455 |
| Net new code | ~6,531 |
| New packages | 3 (`checkpoint`, `worktree`, `fork`) |
| Deleted packages | 1 (`event`) |
| New DB migrations | 3 |
| New UI dialogs | 5 |
| New API endpoints | ~10 |
