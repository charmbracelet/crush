# Worktrees and Snapshots — Technical Specification

## Overview

This spec covers four interconnected features:

1. **Worktrees** — Crush-managed git worktrees in `.crush/worktrees/`
2. **Per-message snapshots** — Automatic filesystem checkpoints using embedded go-git
3. **Conversation forking** — Branch conversations from any point, with filesystem restore
4. **Client-server as default** — Single daemon owns `.crush/`, multiple clients connect

---

## 1. Storage Layout

```
.crush/
├── crush.db                    # SQLite: sessions, messages, snapshots, worktrees
├── crush.json                  # User configuration
├── git/                        # Crush's private git repo (go-git)
│   ├── config
│   ├── objects/                # Content-addressed blob/tree storage
│   ├── refs/
│   │   └── snapshots/          # refs/snapshots/{session_id}/{message_id}
│   └── HEAD
├── worktrees/                  # Managed worktrees
│   ├── feat-add-jwt-auth/
│   │   ├── .git                # Worktree link file
│   │   ├── src/
│   │   └── ...
│   └── fix-null-pointer/
└── logs/
```

Key points:
- User's `.git/` is **never touched**
- All Crush git operations use `.crush/git/` as `GIT_DIR`
- Worktrees share the object store (deduplication)
- `node_modules` and similar are excluded from snapshots

---

## 2. Database Schema

### New Tables

```sql
-- Snapshots: filesystem state at each user message
CREATE TABLE snapshots (
    id TEXT PRIMARY KEY,                    -- UUID
    session_id TEXT NOT NULL,               -- FK → sessions.id
    message_id TEXT NOT NULL,               -- FK → messages.id (the user message)
    parent_snapshot_id TEXT,                -- For branching: which snapshot this forked from
    git_commit_hash TEXT NOT NULL,          -- go-git commit hash
    description TEXT,                       -- Auto-generated from message content
    created_at INTEGER NOT NULL,            -- Unix timestamp ms
    
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_snapshot_id) REFERENCES snapshots(id) ON DELETE SET NULL
);

CREATE INDEX idx_snapshots_session_id ON snapshots(session_id);
CREATE INDEX idx_snapshots_message_id ON snapshots(message_id);

-- Worktrees: managed worktree state
CREATE TABLE worktrees (
    id TEXT PRIMARY KEY,                    -- UUID
    session_id TEXT NOT NULL,               -- FK → sessions.id
    name TEXT NOT NULL,                     -- e.g., "feat-add-jwt-auth"
    path TEXT NOT NULL,                     -- Absolute path to worktree
    current_snapshot_id TEXT,               -- Current position in snapshot history
    is_active BOOLEAN NOT NULL DEFAULT 0,   -- Is this the active worktree for session?
    created_at INTEGER NOT NULL,
    
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (current_snapshot_id) REFERENCES snapshots(id) ON DELETE SET NULL,
    UNIQUE(session_id, name)
);

CREATE INDEX idx_worktrees_session_id ON worktrees(session_id);
```

### Session Table Additions

```sql
-- Add to sessions table
ALTER TABLE sessions ADD COLUMN worktree_id TEXT REFERENCES worktrees(id) ON DELETE SET NULL;
ALTER TABLE sessions ADD COLUMN forked_from_snapshot_id TEXT REFERENCES snapshots(id) ON DELETE SET NULL;
```

---

## 3. New Packages

### `internal/checkpoint/`

Core snapshot and git operations using go-git.

```go
// internal/checkpoint/repo.go

package checkpoint

import (
    "github.com/go-git/go-git/v5"
    "github.com/go-git/go-git/v5/plumbing"
    "github.com/go-git/go-git/v5/plumbing/object"
)

// Repo manages Crush's private git repository for snapshots.
type Repo struct {
    repo       *git.Repository
    gitDir     string      // .crush/git
    projectDir string      // User's project root
    config     *Config
}

// Config holds snapshot configuration.
type Config struct {
    Exclude []string // Paths to exclude (node_modules, etc.)
}

// Init initializes or opens the Crush git repo.
func Init(projectDir string, cfg *Config) (*Repo, error)

// CreateSnapshot creates a snapshot of the current filesystem state.
// Returns the git commit hash.
func (r *Repo) CreateSnapshot(description string) (string, error)

// RestoreSnapshot restores the filesystem to a snapshot.
// Also runs post-restore hooks (npm install, etc.).
func (r *Repo) RestoreSnapshot(commitHash string, targetDir string) error

// Diff returns the diff between two snapshots.
func (r *Repo) Diff(fromHash, toHash string) (string, error)

// CreateWorktree creates a new git worktree from a snapshot.
func (r *Repo) CreateWorktree(name string, commitHash string) (string, error)

// DeleteWorktree removes a worktree.
func (r *Repo) DeleteWorktree(name string) error
```

```go
// internal/checkpoint/service.go

// Service coordinates snapshots with the database.
type Service interface {
    // CreateSnapshot creates a snapshot for a user message.
    CreateSnapshot(ctx context.Context, sessionID, messageID string) (*Snapshot, error)
    
    // RestoreSnapshot restores to a snapshot, updating the session's current position.
    RestoreSnapshot(ctx context.Context, snapshotID string) error
    
    // ListSnapshots returns all snapshots for a session.
    ListSnapshots(ctx context.Context, sessionID string) ([]*Snapshot, error)
    
    // GetSnapshotTree returns snapshots as a tree (for branched conversations).
    GetSnapshotTree(ctx context.Context, sessionID string) (*SnapshotTree, error)
    
    // DiffSnapshots returns the diff between two snapshots.
    DiffSnapshots(ctx context.Context, fromID, toID string) (string, error)
}

// Snapshot represents a filesystem snapshot.
type Snapshot struct {
    ID               string
    SessionID        string
    MessageID        string
    ParentSnapshotID string
    GitCommitHash    string
    Description      string
    CreatedAt        time.Time
}

// SnapshotTree represents the tree structure of snapshots.
type SnapshotTree struct {
    Root     *SnapshotNode
    Current  string // Current snapshot ID
}

type SnapshotNode struct {
    Snapshot *Snapshot
    Children []*SnapshotNode
}
```

### `internal/worktree/`

Worktree lifecycle management.

```go
// internal/worktree/service.go

package worktree

// Service manages Crush worktrees.
type Service interface {
    // Create creates a new worktree, optionally from a snapshot.
    // If name is empty, generates one using the small model.
    Create(ctx context.Context, sessionID string, name string, fromSnapshotID string) (*Worktree, error)
    
    // Switch switches to a worktree, moving uncommitted changes.
    Switch(ctx context.Context, sessionID string, worktreeID string) error
    
    // Delete deletes a worktree.
    Delete(ctx context.Context, worktreeID string) error
    
    // List lists all worktrees for a session.
    List(ctx context.Context, sessionID string) ([]*Worktree, error)
    
    // GetActive returns the active worktree for a session.
    GetActive(ctx context.Context, sessionID string) (*Worktree, error)
    
    // GenerateName generates a worktree name from a description.
    GenerateName(ctx context.Context, description string) (string, error)
    
    // RunPostCreateHooks runs configured post-create commands.
    RunPostCreateHooks(ctx context.Context, worktreePath string) error
    
    // ValidateState checks for external modifications on startup.
    ValidateState(ctx context.Context) error
}

// Worktree represents a managed worktree.
type Worktree struct {
    ID                string
    SessionID         string
    Name              string
    Path              string
    CurrentSnapshotID string
    IsActive          bool
    CreatedAt         time.Time
}
```

---

## 4. Configuration

### `.crush/crush.json` additions

```json
{
  "snapshots": {
    "enabled": true,
    "exclude": [
      "node_modules",
      "vendor",
      ".venv",
      "target",
      "dist",
      "build",
      ".next",
      "__pycache__",
      "*.pyc"
    ]
  },
  "worktree": {
    "post_create": [
      {
        "if_exists": "bun.lockb",
        "run": "bun i"
      },
      {
        "if_exists": "pnpm-lock.yaml",
        "run": "pnpm i"
      },
      {
        "if_exists": "yarn.lock",
        "run": "yarn"
      },
      {
        "if_exists": "package-lock.json",
        "run": "npm ci"
      },
      {
        "if_exists": "go.sum",
        "run": "go mod download"
      },
      {
        "if_exists": "Cargo.lock",
        "run": "cargo fetch"
      },
      {
        "if_exists": "requirements.txt",
        "run": "pip install -r requirements.txt"
      }
    ]
  }
}
```

### Config struct additions

```go
// internal/config/config.go

type SnapshotsConfig struct {
    Enabled bool     `json:"enabled,omitempty"`
    Exclude []string `json:"exclude,omitempty"`
}

type WorktreeConfig struct {
    PostCreate []PostCreateHook `json:"post_create,omitempty"`
}

type PostCreateHook struct {
    IfExists string `json:"if_exists"`
    Run      string `json:"run"`
}

// Add to Config struct:
type Config struct {
    // ... existing fields ...
    Snapshots *SnapshotsConfig `json:"snapshots,omitempty"`
    Worktree  *WorktreeConfig  `json:"worktree,omitempty"`
}
```

---

## 5. Commands

### `/worktree`

| Command | Description |
|---------|-------------|
| `/worktree` | Show current worktree status |
| `/worktree list` | List all worktrees for session |
| `/worktree create [name]` | Create new worktree (name auto-generated if omitted) |
| `/worktree switch <name>` | Switch to worktree, moving uncommitted changes |
| `/worktree delete <name>` | Delete worktree |

### `/snapshot`

| Command | Description |
|---------|-------------|
| `/snapshot` | Show current snapshot info |
| `/snapshot list` | List snapshot history as tree |
| `/snapshot restore <id>` | Restore to snapshot |
| `/snapshot diff [id]` | Diff current state vs snapshot (default: previous) |

### `/rename`

| Command | Description |
|---------|-------------|
| `/rename <name>` | Rename current session |

### `/gc`

| Command | Description |
|---------|-------------|
| `/gc` | Interactive cleanup wizard |

---

## 6. UI Changes

### Status Bar

Add to existing status bar:
- Git branch (from user's `.git/`)
- Worktree indicator (if in a Crush worktree)
- Fork indicator (if conversation was forked)

```
┌────────────────────────────────────────────────────────────────────┐
│ 🔀 main • 📁 feat-add-jwt • ⑂ forked                              │
└────────────────────────────────────────────────────────────────────┘
```

### Conversation View

Each user message gets additional actions:
- **Fork** (⑂) — Fork conversation from this point
- **Restore** (↩) — Restore filesystem to this snapshot

```
┌─────────────────────────────────────────────────────────────────┐
│ You                                              ⑂ ↩  2:34 PM   │
│ Add JWT authentication to the auth module                       │
├─────────────────────────────────────────────────────────────────┤
│ Crush                                                 2:35 PM   │
│ I've added JWT support...                                       │
└─────────────────────────────────────────────────────────────────┘
```

### Session/Snapshot Tree View

New panel showing session structure:

```
┌─ Sessions ─────────────────────────────────────────┐
│                                                    │
│ ▼ auth-refactor                                    │
│   ├─ ● "Add JWT support"                           │
│   ├─ ● "Add refresh tokens"  ← current             │
│   └─ ○ "Try OAuth instead" (fork)                  │
│       ├─ ● "Add Google provider"                   │
│       └─ ● "Add GitHub provider"                   │
│                                                    │
│ ▶ bugfix-user-profile (3 messages)                 │
│ ▶ feature-dark-mode (7 messages)                   │
│                                                    │
└────────────────────────────────────────────────────┘
```

---

## 7. Conversation Forking Flow

When user forks from message N:

1. **Create snapshot** (if not exists) at message N
2. **Create new worktree** from that snapshot
3. **Create new session** with `forked_from_snapshot_id` set
4. **Switch UI** to new session in new worktree
5. **Run post-create hooks** (npm install, etc.)

The original session and worktree remain untouched.

---

## 8. Snapshot Creation Flow

On each user message (before agent processes):

1. **Check if snapshots enabled** (config)
2. **Walk project directory**, excluding configured paths
3. **Create git tree** using go-git (content-addressed, deduplicated)
4. **Create git commit** pointing to tree
5. **Create snapshot record** in database
6. **Update ref** `refs/snapshots/{session_id}/{message_id}`

---

## 9. Snapshot Restoration Flow

When user restores to snapshot N:

1. **Get snapshot** from database
2. **Checkout commit** to current worktree (or project dir)
3. **Run post-create hooks** to restore excluded directories
4. **Update session** current snapshot pointer
5. **Truncate messages** after snapshot's message (optional, confirm with user)

---

## 10. Worktree Name Generation

When creating a worktree without explicit name:

1. **Get recent messages** or session title
2. **Call small model** with prompt:
   ```
   Generate a git branch name for this task. Use conventional commit style 
   (feat/, fix/, refactor/, etc). Keep it short (max 40 chars). 
   For monorepos, prefix with project name.
   
   Task: {description}
   
   Examples: feat/add-jwt-auth, fix/null-pointer-users, api/rate-limiting
   ```
3. **Sanitize result** (lowercase, replace spaces with hyphens, etc.)
4. **Check uniqueness**, append number if needed

---

## 11. Startup Validation

On daemon startup:

1. **Scan `.crush/worktrees/`** for actual directories
2. **Compare with database** worktrees table
3. **For each mismatch**:
   - Worktree exists but not in DB → orphan, offer to delete or import
   - DB entry but no worktree → stale record, clean up
4. **Check for uncommitted changes** in worktrees
5. **Log warnings** for any issues found

---

## 12. GC (Garbage Collection)

`/gc` command workflow:

1. **List all sessions** with:
   - Last activity date
   - Message count
   - Snapshot count
   - Worktree count
   - Disk usage estimate

2. **User selects** sessions to delete

3. **For each selected session**:
   - Delete worktrees (filesystem)
   - Delete snapshots (database + git refs)
   - Delete session (cascades to messages)
   - Run `git gc` on `.crush/git/`

4. **Show summary** of space reclaimed

---

## 13. Client-Server Integration

### Backend additions

```go
// internal/backend/backend.go

type Workspace struct {
    // ... existing fields ...
    Checkpoints checkpoint.Service
    Worktrees   worktree.Service
}
```

### New API endpoints

```
POST   /v1/workspaces/{id}/snapshots                    # Create snapshot
GET    /v1/workspaces/{id}/snapshots                    # List snapshots
GET    /v1/workspaces/{id}/snapshots/{sid}              # Get snapshot
POST   /v1/workspaces/{id}/snapshots/{sid}/restore      # Restore snapshot
GET    /v1/workspaces/{id}/snapshots/{sid}/diff         # Diff snapshot

POST   /v1/workspaces/{id}/worktrees                    # Create worktree
GET    /v1/workspaces/{id}/worktrees                    # List worktrees
DELETE /v1/workspaces/{id}/worktrees/{wid}              # Delete worktree
POST   /v1/workspaces/{id}/worktrees/{wid}/switch       # Switch worktree

POST   /v1/workspaces/{id}/sessions/{sid}/fork          # Fork session
POST   /v1/workspaces/{id}/sessions/{sid}/rename        # Rename session
POST   /v1/workspaces/{id}/gc                           # Run GC
```

### Proto additions

```go
// internal/proto/snapshot.go

type Snapshot struct {
    ID               string    `json:"id"`
    SessionID        string    `json:"session_id"`
    MessageID        string    `json:"message_id"`
    ParentSnapshotID string    `json:"parent_snapshot_id,omitempty"`
    GitCommitHash    string    `json:"git_commit_hash"`
    Description      string    `json:"description"`
    CreatedAt        time.Time `json:"created_at"`
}

type SnapshotTree struct {
    Root    *SnapshotNode `json:"root"`
    Current string        `json:"current"`
}

type SnapshotNode struct {
    Snapshot *Snapshot       `json:"snapshot"`
    Children []*SnapshotNode `json:"children,omitempty"`
}

// internal/proto/worktree.go

type Worktree struct {
    ID                string    `json:"id"`
    SessionID         string    `json:"session_id"`
    Name              string    `json:"name"`
    Path              string    `json:"path"`
    CurrentSnapshotID string    `json:"current_snapshot_id,omitempty"`
    IsActive          bool      `json:"is_active"`
    CreatedAt         time.Time `json:"created_at"`
}
```

---

## 14. Implementation Phases

### Phase 1: Core Snapshot Infrastructure
- [ ] `internal/checkpoint/repo.go` — go-git wrapper
- [ ] `internal/checkpoint/service.go` — snapshot service
- [ ] Database migration for `snapshots` table
- [ ] Config additions for `snapshots.exclude`
- [ ] Integration with message creation flow

### Phase 2: Worktree Management
- [ ] `internal/worktree/service.go` — worktree service
- [ ] Database migration for `worktrees` table
- [ ] Worktree name generation (small model)
- [ ] Post-create hooks execution
- [ ] `/worktree` command

### Phase 3: Conversation Forking
- [ ] Fork flow implementation
- [ ] Session table additions
- [ ] UI: fork/restore buttons on messages
- [ ] `/snapshot` command

### Phase 4: UI Polish
- [ ] Status bar: branch/worktree/fork indicators
- [ ] Session tree view panel
- [ ] `/rename` command

### Phase 5: Maintenance
- [ ] `/gc` command
- [ ] Startup validation
- [ ] Client-server API endpoints

---

## 15. Testing Strategy

### Unit Tests
- `checkpoint/repo_test.go` — go-git operations
- `checkpoint/service_test.go` — snapshot CRUD
- `worktree/service_test.go` — worktree lifecycle

### Integration Tests
- Snapshot creation during message flow
- Fork and restore workflows
- Post-create hooks execution
- GC cleanup

### Golden Tests
- Snapshot tree rendering
- Status bar with worktree info

---

## 16. Open Questions (Resolved)

| Question | Resolution |
|----------|------------|
| Auto-cleanup policy? | No auto-cleanup. Manual `/gc` only. |
| Cross-session forking? | Yes. Sessions persist until `/gc`. |
| Worktree naming? | Auto-generated via small model, conventional-commit style. |
| External modifications? | Validate on startup, warn about mismatches. |
| Disk space limits? | No caps. `/gc` for manual cleanup. |
| Large directories? | Exclude from snapshots, run install commands on restore. |
