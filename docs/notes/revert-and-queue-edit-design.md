# Design: Revert + Edit-Queued-Message

**Status:** Proposal / design
**Scope:** Two independent "basic" features.
- **A. Revert** — restore the workspace files + conversation to a previous point (rewind/checkpoint).
- **B. Edit queued message** — while the agent is busy, view/edit/remove/reorder prompts
  the user has queued, before they run.

All file:line refs are against this checkout (`internal/...`).

---

## 0. Feasibility TL;DR

| Feature | Verdict | Why |
|---|---|---|
| **B. Edit queued message** | **Easy–Medium.** Mostly additive plumbing. | Queue, read API, server/client/workspace methods, and a "clear" mutation already exist; we add per-item mutations + a dialog. The queue is in-memory, single source of truth on the agent. |
| **A. Revert** | **Medium, with one real gap.** | Full-content file snapshots already exist per edit; message delete exists. **But there is no link between a file version and the message that produced it**, and no "delete after message X". Revert is cheap *once* we add that linkage + a small orchestration. |

The load-bearing decision for Revert is **how to correlate file versions to a rewind point**
(§A.2 / D-R2). Everything else is straightforward.

---

# Feature A — Revert (checkpoint / rewind)

## A.1 Verified facts

- **No existing revert/checkpoint/rewind/undo** anywhere (commands, keybinds, endpoints).
  Only whole-session `DeleteSession` exists.
- **File history = full-content snapshots, per version** (`internal/history/file.go:18`,
  DB `files` table: `UNIQUE(path, session_id, version)`, `content TEXT`). Restoring is just
  copying a `content` field — cheap. Storage is full content, not diffs (DB bloat, but easy revert).
- **Versions are written by the edit/write tools**, capturing *before* and *after*:
  - `edit.go:173` create-new-file: `files.Create(...,"")` then `CreateVersion(...,content)`.
  - `edit.go:299` modify: `files.Create(...,oldContent)` then `CreateVersion(...,newContent)`.
  - `write.go:143`: snapshot old content then `CreateVersion(...,newContent)`.
  - `history.Create` = version 0 (`file.go:58`), `CreateVersion` = next version (`file.go:65`).
- **`filetracker` tracks reads only** (`internal/filetracker/service.go`) — orthogonal, not
  useful for revert.
- **Messages are deletable individually** (`message.Delete(id)`, `message.go:137`;
  DB `DeleteMessage`, `DeleteSessionMessages`, `ListMessagesBySession`). Delete publishes
  `pubsub.DeletedEvent` so the UI reacts.

### A.1.1 The gap (load-bearing)
- **No message → file-version linkage.** `history.File` has `SessionID` only — no
  `message_id`/`tool_call_id`. Correlation is possible *only* by `CreatedAt` timestamps, which
  are **second-precision** ⇒ two edits in the same second can't be ordered. (`models.go:21`,
  `history/file.go:18`.)
- **No "delete messages after X"** query — only single-delete or delete-all.
- **No "file version as of message X"** query.
- **Untracked mutations:** agent *deletions* aren't recorded in history; *user* manual edits
  to disk aren't tracked. Both are revert edge cases (§A.5).

## A.2 HLD

### Goals / non-goals
- **G** Revert to a chosen **user-message boundary** (a turn): restore files changed since
  then and drop the messages after it.
- **N (non-goal v1)** Partial/per-file revert; reverting across sessions; redo.

### Approach
Revert is an **orchestration over existing primitives plus one schema addition**:
1. **Tag each file version with the message that produced it** (new nullable `message_id`
   on `files`) so "which versions belong to the reverted turn" is *exact*, not a timestamp guess.
2. `RevertToMessage(sessionID, messageID)`:
   - guard: refuse/cancel if the session is busy (no in-flight run),
   - for every path touched at-or-after the checkpoint: restore to the newest version
     **before** the checkpoint, or **delete the file** if it had no earlier version
     (agent-created in the reverted turn),
   - delete the now-orphaned newer file versions,
   - delete messages at-or-after the checkpoint,
   - publish file + session + message-deleted events so the TUI reloads.

### Key decisions

| # | Decision | Choice | Rationale |
|---|---|---|---|
| D-R1 | Revert granularity | **User-message boundary** (whole turn) | matches mental model; avoids mid-turn inconsistency |
| D-R2 | File↔checkpoint correlation | **Add `message_id` to `files`** (nullable, populated going forward; timestamp fallback for legacy rows) | second-precision timestamps are ambiguous; explicit id makes revert deterministic |
| D-R3 | Restore vs delete a path | restore to newest pre-checkpoint version; **delete** if none existed | correctly undoes agent-created files |
| D-R4 | User-edit safety | before clobbering, compare disk content to latest tracked version; **confirm** on mismatch | avoid silently destroying out-of-band user edits |
| D-R5 | Busy guard | revert only when session idle (else cancel first) | prevents racing a running tool |
| D-R6 | Are revert writes themselves versioned? | **No** new history versions during revert; just truncate forward | keeps history monotonic with the conversation |

## A.3 LLD

### A.3.1 Schema + queries
- **Migration** `internal/db/migrations/<ts>_files_message_id.sql`:
  `ALTER TABLE files ADD COLUMN message_id TEXT;` (nullable; back-compat).
- **New SQL** (`internal/db/sql/files.sql`, `messages.sql`) + sqlc regen:
  - `ListFileVersionsForMessagesAfter(session_id, message_ids)` — versions to drop.
  - `GetLatestFileVersionBeforeMessage(path, session_id, checkpoint_created_at)` — restore source.
  - `DeleteFileVersion(id)` (or `DeleteFileVersionsByMessage(message_id IN ...)`).
  - `DeleteMessagesAfter(session_id, created_at)` (or `DeleteMessagesByIDs`).
  - `ListMessagesBySession` already exists for computing the cut set.

### A.3.2 Thread message id into history writes
`history.Create/CreateVersion` currently take `(sessionID, path, content)`. Add a
`messageID` param and pass it from the edit/write tools (`edit.go:173,299`, `write.go:143`).
**PV-R1:** confirm the producing assistant message id is in scope in the tool execution
context (the tool call belongs to an assistant message — if not directly available, thread it
via the tool context).

### A.3.3 Revert orchestration (new service)
`internal/revert/` (or a method on the session/history service):
```go
func (s *Service) RevertToMessage(ctx context.Context, sessionID, messageID string) (RevertResult, error)
// 1. if coordinator.IsSessionBusy(sessionID) -> error/cancel (D-R5)
// 2. checkpoint := messages.Get(messageID); cut := messages after checkpoint (by order/CreatedAt)
// 3. touched := distinct paths in file versions tagged with any cut message_id (or CreatedAt >= checkpoint)
// 4. for path in touched:
//      prev := GetLatestFileVersionBeforeMessage(path, sessionID, checkpoint.CreatedAt)
//      if prev == nil { os.Remove(path); history.DeleteVersionsForPath(path, sessionID, afterCheckpoint) }
//      else {
//          if disk(path) != latestTracked(path) -> needConfirm (D-R4)
//          os.WriteFile(path, prev.Content); history.DeleteVersionsAfter(path, sessionID, prev.Version)
//      }
// 5. for m in cut: messages.Delete(m.ID)   // publishes DeletedEvent each
// 6. publish file events for touched paths + a session-updated event
```
`RevertResult{MessagesDeleted int, FilesRestored []string, FilesDeleted []string, NeedsConfirm []string}`.

### A.3.4 Wire + UI
- **Endpoint** `POST /v1/workspaces/{id}/agent/sessions/{sid}/revert/{messageID}`
  (`internal/server/proto.go`, model on `handleDeleteWorkspaceSession:544`) → backend →
  coordinator/revert service. Add swagger annotation (regen).
- **Client** `RevertToMessage(ctx, id, sessionID, messageID)` (`internal/client/proto.go`).
- **Workspace** interface method `RevertToMessage(sessionID, messageID) (RevertResult, error)`
  with `AppWorkspace` (direct) + `ClientWorkspace` (HTTP) impls.
- **TUI:** select a **user** message in the chat list, keybind (e.g. `ctrl+r`) → a confirm
  dialog ("Revert to here? Undo N messages, restore M files, delete K files") → on confirm
  call workspace → reload via existing `setSessionMessages` (`ui.go:999`) and let file
  `pubsub.DeletedEvent`/file events refresh the sidebar. If `NeedsConfirm` non-empty, surface
  the out-of-band-edit warning first.

## A.4 Milestones
- **R0** schema migration + message-id linkage (populate forward; harmless no-op until used).
- **R1** `RevertToMessage` backend + endpoint + workspace methods (files + messages). CLI/test-drivable.
- **R2** TUI affordance + confirm dialog + reload.
- **R3** safety polish: user-edit detection (D-R4), optional agent-deletion tracking in history.

## A.5 Risks / edge cases
- **Out-of-band user edits** clobbered → mitigated by D-R4 confirm.
- **Agent-deleted files** aren't in history → a revert can't recreate a file the agent deleted
  in the reverted turn (R3: add a delete marker to history to close this).
- **DB bloat** from full-content versions is pre-existing, not introduced here.
- **Legacy rows** (no `message_id`) fall back to timestamp correlation — less precise; acceptable.

---

# Feature B — Edit queued message

## B.1 Verified facts

- **Queue is in-memory, agent-side, FIFO, per session:**
  `messageQueue *csync.Map[string, []SessionAgentCall]` (`agent.go:165`). Items are
  `SessionAgentCall` (`agent.go:73-124`; carries `Prompt`, `Attachments`, `RunID`, model params).
  **Not persisted** — the agent is the single source of truth.
- **Existing ops:** `enqueueCall` (`agent.go:344`), `drainQueueForStep` (`:380`),
  `QueuedPrompts` count (`:1929`), `QueuedPromptsList []string` (`:1937`, **text only**),
  `clearQueueAndNotify` (`:445`) / `ClearQueue` (`:1887`). Enqueue happens when
  `IsSessionBusy` (`:1924`, i.e. an `activeRequests` entry exists) at submit time
  (`:605, :627`). Dequeue/recurse at run end pops index 0 and re-`Run`s (`:1251, :1272`).
- **Full pipeline already exists for read + clear:**
  - server: `GET .../prompts/queued` (`proto.go:867`), `GET .../prompts/list` (`:929`),
    `POST .../prompts/clear` (`:888`).
  - backend: `QueuedPrompts/QueuedPromptsList/ClearQueue` (`backend/agent.go:191-230`).
  - client SDK: `GetAgentSessionQueuedPrompts/...List/ClearAgentSessionQueuedPrompts`
    (`client/proto.go:349,730,366`).
  - workspace iface: `AgentQueuedPrompts/AgentQueuedPromptsList/AgentClearQueue`
    (`workspace.go:90-92`), both `App`/`Client` impls.
- **UI today:** only a `promptQueue int` count (`ui.go:262`), polled each `Update`
  (`:553-557`), surfaced as "clear queue" help text. **No list, no per-item control.**
- **No per-item mutation, no queue-change events.** Only whole-queue clear; UI polls.

### B.1.1 The one real hazard
Items are addressed **by position**, and `drainQueueForStep` pops index 0 when the current
run ends — which can happen *while the user is editing the queue*. Index identity is racy.
⇒ add a **stable per-item id** (D-Q1).

## B.2 HLD

### Goals / non-goals
- **G1** View queued prompts (full text, not just truncated count).
- **G2** Remove a queued prompt.
- **G3** Edit a queued prompt's text (and optionally attachments).
- **G4** Reorder queued prompts.
- **N (non-goal v1)** Editing model params of a queued item; cross-session queue view.

### Approach
Mirror the existing queue pipeline (it already has read + clear end-to-end) and add three
**per-item mutations** behind a stable id, plus a queue dialog. Edit reuses the **main editor**
("pull the prompt back into the textarea") instead of an in-dialog text editor.

### Key decisions

| # | Decision | Choice | Rationale |
|---|---|---|---|
| D-Q1 | Item identity | **Stable UUID per queued item** (assigned at enqueue) | index shifts on drain mid-edit (B.1.1) |
| D-Q2 | Edit UX | **Pull-into-editor**: select → remove from queue → load text+attachments into main textarea → user re-submits | reuses the real editor; least new code; safest |
| D-Q3 | Concurrency | all mutations under the **same `dispatchMu`** as `drainQueueForStep` | avoids races with dequeue at run end |
| D-Q4 | Data exposure | new `QueuedPromptInfo{ID, Prompt, HasAttachments, RunID}` list method | current list returns `[]string` only |
| D-Q5 | Reactivity | **keep polling** for v1 (UI already polls count); add queue-change notify events as enhancement | minimal; matches existing pattern |
| D-Q6 | RunID items | on remove/edit of a `RunID`-bearing item, publish `RunComplete{Cancelled:true}` (mirror `clearQueueAndNotify`) | keeps `crush run`/non-interactive callers correct |

## B.3 LLD

### B.3.1 Agent core (`internal/agent/agent.go`)
Wrap queued items so they carry an id (cleanest):
```go
type queuedItem struct { ID string; Call SessionAgentCall }
messageQueue *csync.Map[string, []queuedItem]   // was []SessionAgentCall
```
Update `enqueueCall` (assign `uuid` ), `drainQueueForStep`, `QueuedPrompts`,
`QueuedPromptsList`, `clearQueueAndNotify` to the new element type. New methods (all take
`dispatchMu`):
```go
func (a *sessionAgent) QueuedPromptsDetailed(sessionID string) []QueuedPromptInfo
func (a *sessionAgent) RemoveQueuedPrompt(sessionID, itemID string) error   // D-Q6 publish if RunID
func (a *sessionAgent) EditQueuedPrompt(sessionID, itemID, prompt string, att []message.Attachment) error
func (a *sessionAgent) ReorderQueue(sessionID string, order []string) error // permutation of ids
```

### B.3.2 Wire (mirror the existing 3 endpoints)
- **proto** `internal/proto/agent.go`: `QueuedPromptInfo{ID, Prompt, HasAttachments, RunID}` DTO.
- **server** `internal/server/proto.go` (model on `:867/:888/:929`):
  - `GET  .../prompts/detailed` → `[]QueuedPromptInfo`
  - `POST .../prompts/{itemID}/remove`
  - `POST .../prompts/{itemID}/edit`   (body: `{prompt, attachments}`)
  - `POST .../prompts/reorder`         (body: `{order: []string}`)
- **backend** `internal/backend/agent.go` (model on `:191-230`): four wrappers.
- **client** `internal/client/proto.go` (model on `:349/:366/:730`): four methods.
- **workspace** `internal/workspace/workspace.go:90` + both impls: four methods.

### B.3.3 UI (`internal/ui/...`)
- New `internal/ui/dialog/queue.go` (template `dialog/sessions.go:31`): list of
  `QueuedPromptInfo`; keys — `enter`=edit (D-Q2), `ctrl+x`=remove, `J/K` or
  `shift+↑/↓`=reorder, `esc`=close. Actions in `dialog/actions.go`:
  `ActionEditQueuedPrompt{ID,Prompt,Attachments}`, `ActionRemoveQueuedPrompt{ID}`,
  `ActionReorderQueue{Order}`.
- Open via a keybind (only meaningful when `promptQueue > 0`). Handle actions in
  `ui.go Update` (model on `:1397`): remove/reorder call workspace; **edit** =
  `Workspace.AgentRemoveQueuedPrompt(id)` + set textarea value & attachments
  (the user re-submits, which re-queues it at the tail).
- Optional: replace the bare count with a small "queued" indicator that opens the dialog.

## B.4 Milestones
- **Q1** detailed list + **remove** (smallest end-to-end slice; proves the pipeline).
- **Q2** **edit** (pull-into-editor).
- **Q3** **reorder**.
- **Q4** (optional) queue-change events to drop polling.

## B.5 Risks / edge cases
- **Drain-during-edit race** → solved by stable id (D-Q1) + `dispatchMu` (D-Q3): a removed/
  edited id that already drained is a no-op/return-not-found.
- **RunID items** (non-interactive callers) must emit cancellation on removal (D-Q6).
- **Attachment editing** is fiddly; v1 may restrict edit to **prompt text**, keeping existing
  attachments (PV-Q3).

---

## C. Cross-cutting

### C.1 Sequencing recommendation
Ship **B (queue edit) first** — it's lower-risk, fully additive, and the read/clear pipeline
already exists so each slice is small. Then **A (revert)**, starting with **R0** (the
`message_id` linkage migration) since it's a harmless no-op that must land before revert can
be exact.

### C.2 Pre-flight verification (before coding)
- **PV-R1** Is the producing assistant **message id in scope** in `edit.go`/`write.go` when
  they call `history.Create/CreateVersion`? (Decides clean linkage vs timestamp fallback.)
- **PV-R2** Confirm `RevertToMessage` must run only when **idle**; how to detect/cancel a busy
  session cleanly (reuse `IsSessionBusy`, `agent.go:1924`).
- **PV-R3** Confirm file events (`pubsub.DeletedEvent`/file events) cause the sidebar
  "Modified Files" panel to refresh after revert.
- **PV-Q1** Confirm `dispatchMu` is reachable from the new mutation methods without deadlock
  vs `drainQueueForStep` (`agent.go:380`).
- **PV-Q2** Confirm removed/edited `RunID` items need `RunComplete{Cancelled}` (mirror
  `publishCanceledQueueDrops` in `clearQueueAndNotify`, `agent.go:451`).
- **PV-Q3** Decide v1 edit scope: text-only vs text+attachments.

### C.3 Testing
- **B:** unit-test `Remove/Edit/Reorder` under concurrent `drainQueueForStep`; round-trip the
  new endpoints; UI dialog navigation + edit-pulls-into-editor.
- **A:** unit-test `RevertToMessage` (files restored/deleted, messages truncated) on a seeded
  session; edge cases — agent-created file (delete), pre-existing file (restore), out-of-band
  user edit (confirm path); wire round-trip; UI confirm + reload.
