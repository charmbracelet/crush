# Build Plan + HLD/LLD: Revert (Checkpoint / Rewind) — Crush fork

**Status:** Build-ready design. Supersedes the *revert* section of
`revert-and-queue-edit-design.md` for execution purposes (that doc remains the rationale
reference; this one is the ordered build plan).
**Target:** Match Claude Code's rewind semantics — restore **code** and/or **conversation**
to a checkpoint; **bash side effects are intentionally not reverted** (industry-standard,
same caveat Claude Code documents).
**Fork:** local clone at `~/learn/crush`, `upstream → charmbracelet/crush`, work on
`feat/revert`. Builds clean on Go 1.26.4.

---

## Part 0 — Fork & dev environment

### Layout / git
```
~/learn/crush
  remotes:  upstream → github.com/charmbracelet/crush   (origin slot reserved for your fork)
  main      → tracks upstream/main (keep clean; only fast-forward from upstream)
  feat/revert → working branch (current)
```
Branch model: keep `main` pristine; one feature branch per feature
(`feat/revert`, `feat/queue-edit`, `feat/agents-panel`); rebase each on `upstream/main`
before merging into your own integration branch. Keep commits modular so upstream rebases
stay cheap.

**To publish later** (outward-facing — do only when you choose): create a GitHub fork, then
`git remote add origin <your-fork-url>` and `git push -u origin feat/revert`. Charm requires
a **CLA** to upstream a PR.

### Build / run / test (verified)
```bash
go build -o /tmp/crush .                 # ✅ builds (CGO_ENABLED=0, see AGENTS.md)
go run . --cwd /path/to/scratch/project  # run the TUI against a throwaway repo
go test ./internal/history/... ./internal/message/... ./internal/agent/...
```
Codegen deps (needed for M0):
- **sqlc** — DB access is generated from `internal/db/sql/*.sql` per `sqlc.yaml`; after editing
  SQL run the project's generate step (check `Taskfile.yaml`; install `sqlc` if absent).
- **migrations** — goose-style `.sql` files in `internal/db/migrations/` are `go:embed`ed and
  applied on startup. Adding a migration = add a file; no manual run.

### Ground rules (from repo)
- `internal/ui/AGENTS.md` — **read before any TUI change** (monolithic Bubble Tea model).
- Keep changes additive; don't break the in-process *and* client/server modes (both must work).

---

## Part 1 — HLD

### 1.1 Scope (v1 vs parity follow-up)
| | v1 (this plan) | Parity follow-up (M4) |
|---|---|---|
| restore files the agent **edited** | ✅ | ✅ |
| restore files the agent **created** (delete them) | ✅ | ✅ |
| restore files the agent **deleted** (recreate) | ❌ (history has no delete record) | ✅ (add delete-tracking) |
| truncate conversation after checkpoint | ✅ | ✅ |
| undo bash side effects (installs, `rm`, push, migrations) | ❌ by design | ❌ by design |
| out-of-band / manual user edits | not tracked, left as-is (match Claude Code) | same |

### 1.2 Model
- **Checkpoint granularity = a user-message boundary** (a turn). "Rewind to here" targets a
  user message.
- **Three restore choices** (Claude Code parity): *code + conversation*, *code only*,
  *conversation only*.
- **Mechanism = Crush's existing snapshot store**, not git. The `history` table already keeps
  **full file content per edit** (`internal/history/file.go:18`; `files` table in
  `migrations/20250424200609_initial.sql:24`). This is the same architecture Claude Code uses
  (own snapshot store, files-only). Restore = write a stored `content` back to disk.

### 1.3 What exists vs the gaps (verified)
**Exists:** full-content versions written by edit/write tools (`edit.go:173,299`,
`write.go:143`); `message.Delete(id)` publishing `pubsub.DeletedEvent` (`message.go:137`);
`ListMessagesBySession`; file events that refresh the sidebar; `IsSessionBusy`
(`agent.go:1924`).
**Gaps to build:** (g1) no link from a file version to the message that produced it; (g2) no
"delete messages after X"; (g3) no "file version as of checkpoint" query; (g4) no orchestration;
(g5) no UI; (g6 parity) no deletion record in history.

### 1.4 Data flows
- **Restore (code):** `RevertToMessage` → for each touched path, `GetVersionBeforeCheckpoint` →
  `os.WriteFile` (or delete if none) → drop newer versions → publish file events → sidebar +
  open buffers refresh.
- **Restore (conversation):** delete messages at/after checkpoint (each publishes
  `DeletedEvent`) → TUI reloads via `setSessionMessages` (`ui.go:999`).
- **Trigger:** select a user message in chat → keybind → confirm dialog → workspace call.

### 1.5 Key decisions
| # | Decision | Choice | Rationale |
|---|---|---|---|
| D1 | correlate versions to checkpoint | add `message_id` to `files` (nullable; timestamp fallback for legacy) | second-precision timestamps are ambiguous (closes g1) |
| D2 | safety on manual edits | **don't track / don't confirm** — restore tracked files as-is | matches Claude Code; simpler than the confirm-step in the older doc |
| D3 | busy guard | revert only when `!IsSessionBusy` (else require cancel first) | no racing a live tool |
| D4 | revert writes re-versioned? | **no** — truncate forward, don't append new versions | history stays monotonic with the conversation |
| D5 | deleted-file restore | **defer to M4** behind delete-tracking | keeps v1 small; 90% of value without it |
| D6 | transport | reuse the existing per-session endpoint shape | one new endpoint, mirrors cancel/delete handlers |

---

## Part 2 — LLD (ordered milestones)

> Each milestone is independently compilable and testable. Sizes are relative.

### M0 — Message↔version linkage (small)
**Goal:** every new file version records the assistant message that created it.
- **Migration** `internal/db/migrations/<ts>_add_message_id_to_files.sql`:
  `ALTER TABLE files ADD COLUMN message_id TEXT;` (nullable).
- **SQL** (`internal/db/sql/files.sql`) + regen: add `message_id` to `CreateFile`; new query
  `GetFileVersionBeforeCheckpoint(path, session_id, checkpoint_msg_created_at)` and
  `ListFileVersionsAfter(session_id, checkpoint_msg_created_at)` and `DeleteFileVersion(id)`.
- **history service** (`internal/history/file.go:58,65,84`): thread `messageID` through
  `Create`/`CreateVersion`/`createWithVersion`.
- **callers**: `internal/agent/tools/edit.go:173,299`, `write.go:143` pass the current
  assistant message id. **PV-R1:** confirm the producing message id is in the tool context;
  if not, thread it via the tool ctx (the tool call already belongs to an assistant message).
- **Acceptance:** unit test — after an edit, the new `files` row has the expected `message_id`.
- **Note:** purely additive; legacy rows keep `NULL` and fall back to timestamp ordering.

### M1 — Revert orchestration + queries (medium)
**Goal:** `RevertToMessage` works headlessly (drive from a test).
- **New** `internal/revert/service.go`:
  ```go
  type Result struct {
      MessagesDeleted int
      FilesRestored   []string
      FilesDeleted    []string   // agent-created files removed
  }
  func (s *Service) RevertToMessage(ctx context.Context, sessionID, messageID string, opts Options) (Result, error)
  // Options{ RestoreCode bool; RestoreConversation bool }
  ```
  Algorithm:
  1. if `coordinator.IsSessionBusy(sessionID)` → return `ErrSessionBusy` (D3).
  2. `cp := messages.Get(messageID)`; `cut := messages with CreatedAt >= cp.CreatedAt` (ordered).
  3. if `RestoreCode`: `touched := distinct paths` from versions tagged with a cut `message_id`
     (fallback: `CreatedAt >= cp.CreatedAt`). For each path:
       - `prev := GetFileVersionBeforeCheckpoint(path, sessionID, cp.CreatedAt)`
       - `prev == nil` → `os.Remove(path)`; record FilesDeleted
       - else → `os.WriteFile(path, prev.Content, 0o644)`; record FilesRestored
       - delete the now-orphaned newer versions (`ListFileVersionsAfter` → `DeleteFileVersion`)
  4. if `RestoreConversation`: for `m in cut`: `messages.Delete(m.ID)` (publishes `DeletedEvent`).
  5. publish a file event per touched path + a session-updated event so UIs refresh.
- **deps:** inject `history.Service`, `message.Service`, and the coordinator's `IsSessionBusy`.
- **Acceptance:** table tests on a seeded session — (a) agent-edited file restored to prior
  content; (b) agent-created file deleted; (c) `RestoreConversation=false` leaves messages
  intact; (d) busy session → `ErrSessionBusy`.

### M2 — Wire across both process modes (small, mechanical)
Mirror the existing cancel/delete handlers (`internal/server/proto.go:544,846`).
- **endpoint** `POST /v1/workspaces/{id}/agent/sessions/{sid}/revert/{messageID}` with body
  `{restore_code, restore_conversation}` → `backend.RevertToMessage(...)`. Add swagger
  annotation + regen.
- **backend** `internal/backend/agent.go`: `RevertToMessage(workspaceID, sessionID, messageID, opts)`.
- **client** `internal/client/proto.go`: `RevertToMessage(ctx, id, sessionID, messageID, opts)`.
- **workspace** `internal/workspace/workspace.go` + `app_workspace.go` (direct) +
  `client_workspace.go` (HTTP): `RevertToMessage(sessionID, messageID, opts) (revert.Result, error)`.
- **Acceptance:** client/server round-trip test reverts a seeded session identically to the
  in-process path.

### M3 — TUI (medium; read `internal/ui/AGENTS.md` first)
- **Trigger:** in the chat list, select a **user** message; keybind (propose `ctrl+r`) opens a
  **confirm dialog** modeled on `dialog/sessions.go`, showing the three choices (code+convo /
  code only / convo only) and a summary ("undo N messages, restore M files, delete K files").
- **Action** `dialog/actions.go`: `ActionRevert{MessageID string; RestoreCode, RestoreConversation bool}`.
- **Handle** in `ui.go Update` (model on `ActionSelectSession` at `:1397`): call
  `m.com.Workspace.RevertToMessage(...)`; on success reload conversation via
  `setSessionMessages` (`ui.go:999`); file `DeletedEvent`/file events refresh the sidebar
  "Modified Files" panel automatically (**PV-R3**).
- **Acceptance:** manual run — edit a file across two turns, `ctrl+r` to the first user
  message, choose "code + conversation", verify file on disk + chat both rewind.

### M4 — Deleted-file restore (parity, optional follow-up)
- Record deletions in history: when the agent deletes a file (the delete/bash-rm path that
  goes through a tracked tool), write a tombstone version (e.g. a `deleted BOOL` column or a
  sentinel). Then M1 step 3 recreates files whose checkpoint state was "present".
- Scope note: bash `rm` is **not** a tracked tool, so this only covers deletions via Crush's
  file tools — consistent with the side-effect caveat.

---

## Part 3 — Verification (run before/with coding)
- **PV-R1** (M0 blocker): is the producing assistant **message id in scope** in
  `edit.go`/`write.go` at the `history.Create/CreateVersion` call? Decides clean linkage vs
  timestamp fallback.
- **PV-R2** (M1): confirm `IsSessionBusy` (`agent.go:1924`) is reachable from the revert
  service / backend without import cycles.
- **PV-R3** (M3): confirm `pubsub.DeletedEvent` (messages) and file events actually re-render
  the chat + sidebar (they do for normal edits; verify for the revert path).
- **PV-R4:** confirm migrations auto-apply on startup (embedded goose) so the new column lands
  without a manual step.

## Part 4 — Sequencing
1. **(recommended first, separate branch)** `feat/queue-edit` — lower risk, fully additive,
   pipeline already exists. See `revert-and-queue-edit-design.md` §B.
2. **`feat/revert`** here: **M0 → M1 → M2 → M3**, ship as v1; **M4** as a fast-follow.
3. `feat/agents-panel` — see `running-agents-panel-design.md`.

## Definition of done (v1 revert)
- `ctrl+r` on a user message restores agent-edited/created files and (optionally) truncates the
  conversation, in both process modes; bash side effects explicitly untouched (documented in
  the confirm dialog); covered by unit + round-trip + one manual E2E test.
