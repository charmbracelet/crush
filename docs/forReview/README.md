# Dispatch System Architecture Review

**Reviewer Instructions:** Read this document, then browse the `docs/` directory. We're looking for critical feedback, design concerns, and implementation suggestions.

---

## What Is This Project?

Crush is an AI-powered CLI tool for software development (similar to Goose, Aider, Claude Code). This repository contains a **multi-agent dispatch system** that allows Crush to coordinate work across multiple AI CLI tools.

**The problem:** One AI agent can't do everything well. You want Goose to handle refactoring, Aider to write tests, Crush to orchestrate.

**The solution:** A session-based dispatch system where a "Host" agent delegates tasks to "Worker" agents, tracks their progress, and collects results.

---

## Mental Model

**Email for AI agents.**

- Dispatches = Emails
- API Server = Mail server
- Sessions = Mailboxes
- Results = Replies

Workers don't message each other. They read/write documents through a central server.

---

## Architecture Overview

```
SERVER                          MACHINE
  │                                 │
  │                                 ▼
  │                           ┌───────────────┐
  │                           │   HANDLER     │
  │                           │ (one per      │
  │                           │  machine)     │
  │                           │               │
  │◄───────────────────── long-poll ──────────┤
  │  GET /machine/{id}/commands?wait=30       │
  │                           │               │
  │                           │  Manages:     │
  │                           │  Sessions:    │
  │                           │  - goose-fe   │
  │                           │  - goose-be   │
  │                           │  - crush-fe   │
  │                           │               │
  │──────────────────────────────────────►    │
  │  [{action, session, spec}]                │
  │                           │               │
  │                           ▼               │
  │                    ┌──────────────┐       │
  │                    │   WORKER     │       │
  │                    │  (ephemeral) │       │
  │                    └──────────────┘       │
  │                           │               │
  │◄────────────────────── POST claim ────────┤
  │◄────────────────────── POST heartbeat ────┤
  │◄────────────────────── POST start ────────┤
  │◄────────────────────── POST finish ───────┤
  │◄────────────────────── POST activity ─────┤
```

---

## Core Concept: Sessions

**The session is the fundamental unit.**

```
Session = {worker_type} + {machine} + {directory}

Examples:
  goose-frontend:  goose  + laptop-A + /repo/frontend
  goose-backend:   goose  + laptop-B + /repo/backend
  crush-frontend:  crush  + laptop-A + /repo/frontend
```

### Session States

```
Session is either:
  - available: ready to accept work
  - busy: currently executing a dispatch
```

### Directory Constraint

**Only one session per worker type per machine per directory.**

```
laptop-A:/repo/frontend:
  ✅ goose-frontend  (goose)
  ✅ crush-frontend  (crush)
  ❌ Cannot create another goose session here

laptop-B:/repo/frontend:
  ✅ goose-frontend  (goose) -- Different machine, allowed

Why? Agents share agentic documents in their directory.
Two goose sessions on the same machine would conflict on the same files.
```

### Nested Directory Constraint

**Nested directories are not allowed for the same worker type on the same machine.**

```
laptop-A:
  ✅ Session at /repo
  ❌ Session at /repo/backend (BLOCKED - nested)

Why? A worker at /repo may traverse into /repo/backend,
causing conflicts with a dedicated backend session.
```

---

## Key Design Decisions

### 1. Session-Based Dispatch

Dispatches target sessions explicitly.

```
POST /dispatch {session: "goose-frontend", task: "fix auth", timeout: 3600}
```

**No scheduling algorithm needed** - the target is explicit.

### 2. Long-Polling for Command Delivery

Handler initiates all communication (works through firewalls/NAT).

```
Handler: GET /machine/{id}/commands?wait=30
         (server holds request up to 30s, returns when command arrives)
```

### 3. One Handler Per Machine

Single handler process manages all sessions on a machine concurrently.

```
Machine "laptop-A":
  Handler (one process)
    ├─ goose-frontend  (/repo/frontend)
    ├─ goose-backend   (/repo/backend)
    └─ crush-frontend  (/repo/frontend)
```

### 4. Structured Execution Specs (No Shell)

**Security: Never use `sh -c` with raw strings.**

Server sends structured execution specs:

```json
{
  "spec": {
    "binary": "goose",
    "args": ["run", "fix auth"],
    "cwd": "/repo/frontend",
    "timeout": 3600
  }
}
```

Handler executes directly: `exec.Command(binary, args...)`

| Issue | `sh -c` string | Structured spec |
|-------|----------------|-----------------|
| Command injection | Vulnerable | Args are separate, no shell |
| Cross-platform | Shell varies | Direct exec |
| Security audit | Hard | Easy to validate |

### 5. Claim/Start/Finish with Leases + Fencing

Commands have explicit lifecycle with lease-based ownership and fencing.

```
1. POST /commands/{id}/claim   → Reserved, gets lease + generation
2. POST /commands/{id}/heartbeat → Renew lease (must include generation)
3. POST /commands/{id}/start   → Process spawned, PID known (must include generation)
4. POST /commands/{id}/finish  → Process exited, result known (must include generation)
```

**Why leases:**
- Handler crashes → lease expires → command recoverable
- Network partition → lease expires → another handler can take over
- No stale state persists forever

**Why fencing:**
- Each claim returns `lease_generation` integer
- Generation increments on each new claim
- Handler must include generation on all subsequent calls
- Prevents stale handler from overwriting after reclaim

### 6. Session-Scoped Command Ordering

**One actionable command per session at a time.**

```
Server enforces:
  - If session has command in pending/claimed/started/orphaned state,
    don't return another command for that session.
  - Next command blocked until current is terminal.

This prevents:
  - Spawn and cancel both visible for same session
  - Racing commands from redirects
```

### 7. Spec Authority Validation

Handler validates spec matches session registration:

```
- spec.binary must match session's binary_name or binary_path
- spec.cwd must equal session's directory exactly
```

Server cannot send arbitrary binaries or directories.

### 8. Trust Model

**Local Mode:** Server is trusted. Controls binary, args, cwd, timeout. Handler validation is sanity check.

**Cloud Mode (Phase 2):** Server not fully trusted. Spec must be signed. Handler validates signature.

### 9. Process-Tree Termination

Kill full process tree, not just top-level PID:

```
Unix: Setpgid=true, then kill(-pid) to terminate group
Windows: Phase 2 (Job Objects)
```

### 9. Per-Session State Files

Handler maintains separate state file per session:

```
~/.config/crush/handler-sessions/{session_name}.json

{
  "session": "goose-frontend",
  "current_command": {
    "command_id": "cmd-1",
    "lease_generation": 5,
    "pid": 12345,
    "pid_start_time": 1709600300,
    "lease_expires_at": 1709600400
  },
  "pending_results": [
    {"command_id": "cmd-0", "lease_generation": 4, "result": "...", "attempts": 2}
  ]
}
```

Results keyed by `(command_id, lease_generation)`, not just `dispatch_id`.

### 10. Process Fingerprint for Crash Recovery

On startup, for each session:
1. Read state file
2. If PID exists:
   - Check if process exists AND start time matches
   - If yes, kill process tree (orphan from crash)
3. Clear state, start polling

### 11. Result Persistence and Retry

Results survive network failures:

```
On worker completion:
1. Save result to pending_results (keyed by command_id + lease_generation)
2. POST /dispatch/{id}/result (includes lease_generation)
3. On success: remove from pending_results
4. On failure: retry on next poll cycle

Result keying: (command_id, lease_generation) prevents stale writes.
```

### 12. State Machine

```
┌─────────┐    ┌─────────┐    ┌─────────┐    ┌───────────┐
│ pending │───►│ claimed │───►│ started │───►│ completed │
└─────────┘    └─────────┘    └─────────┘    └───────────┘
                                   │
                              (lease expires)
                                   │
                                   ▼
                             ┌──────────┐
                             │ orphaned │
                             └──────────┘
```

**Orphaned state:** Started commands that lose their lease go to `orphaned`,
not back to `pending`. Prevents duplicate execution when process may still be running.

**Session availability:** Orphaned commands do NOT block the session. The session
becomes available immediately. Next command can override or augment the orphaned work.

### 13. Cancel (Out-of-Band)

Cancel is not a queued command. It sets a bit on the active command:

```
1. POST /dispatch/{id}/cancel {directive: "..."}
2. Server sets cancel_requested=true, stores cancel_directive
3. Handler sees cancel_requested=true on next heartbeat
4. Handler kills process tree, calls finish, posts result
```

The `directive` is for the caller/next command. Handler doesn't interpret it - just passes it through.

---

## API Endpoints

| Endpoint | Purpose |
|----------|---------|
| `POST /session` | Register a session (worker + machine + directory) |
| `GET /machine/{id}/commands` | Long-poll for commands (handler) |
| `POST /commands/{id}/claim` | Reserve command, get lease + generation |
| `POST /commands/{id}/heartbeat` | Renew lease (requires generation) |
| `POST /commands/{id}/start` | Report process started (requires generation) |
| `POST /commands/{id}/finish` | Report process finished (requires generation) |
| `POST /dispatch` | Create dispatch (targets session) |
| `GET /dispatch/{id}` | Read dispatch status |
| `POST /dispatch/{id}/result` | Submit result (idempotent) |
| `POST /dispatch/{id}/activity` | Stream activity |
| `POST /dispatch/{id}/cancel` | Cancel running dispatch |

---

## The Flow

```
SETUP:
1. Register sessions:
   POST /session {
     name: "goose-frontend",
     worker_type: "goose",
     machine_id: "laptop-A",
     directory: "/repo/frontend",
     cli_command: "goose run"
   }

2. Server validates:
   - No existing goose session for laptop-A:/repo/frontend ✓
   - No nested directory conflict ✓

3. Handler starts:
   - Recover orphans (per-session state files)
   - Retry pending results
   - Start heartbeat goroutine
   - Poll for commands

DISPATCH:
1. Host creates dispatch:
   POST /dispatch {session: "goose-frontend", task: "fix auth", timeout: 3600}

2. Server:
   - Creates dispatch document
   - Creates spawn command with structured spec
   - Returns dispatch_id

3. Handler's long-poll returns command

4. Handler:
   - Claims command (gets lease)
   - Starts heartbeat goroutine
   - Executes: exec.Command("goose", "run", "fix auth")
   - Reports start with PID fingerprint
   - Watches session, posts activity (buffered)
   - On completion:
     - Saves result to pending_results
     - Finishes command
     - Posts result (retries until acknowledged)
     - Removes from pending_results

CANCEL:
1. Human cancels:
   POST /dispatch/{id}/cancel {reason: "Taking too long"}

2. Server sets cancel_requested=true on active command (out-of-band)

3. Handler:
   - Sees cancel_requested=true on next heartbeat
   - Kills process tree
   - Finishes with termination_reason: "canceled"

LEASE EXPIRY:
1. Handler crashes or loses network
2. Server detects lease expired
3. If claimed but not started → returns to "pending"
4. If started → moves to "orphaned" (not auto-requeued)
5. Orphaned commands require manual reconciliation

REDIRECT (Phase 2):
1. Human redirects:
   POST /dispatch/{id}/redirect {reason: "...", new_directive: "..."}

2. Server:
   - Marks old dispatch: superseded
   - Sets cancel_requested=true on active command
   - Creates new dispatch with parent_id

3. Handler:
   - Sees cancel_requested, kills old process, finishes
   - On finish, server creates spawn for new dispatch
   - Handler starts new process with new directive
```

---

## What's Implemented

See `docs/implemented/dispatch-mvp.md`:

- `dispatch_task` tool in Crush
- Local worker spawning (no API yet)
- SQLite database schema
- CLI commands for worker management

---

## What's Being Built Next

See `docs/active-design/dispatch-api.md`:

- REST API server
- Session registry with directory + nesting constraints
- Long-poll command delivery
- Claim/Start/Finish with leases
- Heartbeat endpoint
- Per-session state files
- Crash recovery with process fingerprint
- Result persistence and retry
- Structured execution specs (no shell)
- Cancel (explicit stop)

**Phase 2:**
- Redirect (cancel + respawn with lineage)
- Session activity streaming
- Cloud mode with signed specs

---

## What We Want From You

### Critical Review Questions

1. **Session model:** Does session = worker + machine + directory make sense? Any edge cases?

2. **Directory constraints:** One session per worker type per machine per directory + no nesting - sufficient?

3. **Structured specs:** Binary + args instead of shell strings - robust enough?

4. **Leases + fencing:** 60s lease with 30s heartbeats + generation token - right balance?

5. **Crash recovery:** Per-session state files + PID fingerprint + result persistence - any gaps?

6. **State machine:** pending → claimed → started → completed/orphaned - correct?

7. **Cancel model:** Out-of-band `cancel_requested` bit checked on heartbeat - correct approach?

8. **Orphaned state:** Started commands go to orphaned (not pending) on lease expiry - correct?

9. **Process-tree termination:** Unix Setpgid + kill group - correct approach?

10. **Idempotency:** Result keyed by (command_id, lease_generation) - correct?

### Format for Feedback

Please create a file in this directory: `{YYYY-MM-DD_HH-MM}_{your-name}-review.md`

Example: `2026-03-05_14-30_codex-review.md`

Include:
- Overall assessment
- Critical concerns (things that will break)
- Suggestions (nice to haves)
- Questions (things you didn't understand)
- Alternative approaches we should consider

---

## Key Files to Read

| File | Purpose |
|------|---------|
| `docs/README.md` | Navigation and status |
| `docs/implemented/dispatch-mvp.md` | What works now |
| `docs/active-design/dispatch-api.md` | What we're building (full spec) |
| `docs/future/multi-agent-orchestration.md` | Full vision |
| `internal/dispatch/service.go` | Current service implementation |
| `internal/agent/tools/dispatch.go` | dispatch_task tool |

---

## Review History

| Round | Date | Reviewers | Key Changes |
|-------|------|-----------|-------------|
| 1 | Mar 5, 2026 | Codex, Gemini CLI | Initial feedback |
| 2 | Mar 5, 2026 | Codex, Gemini CLI | Session model, claim/start/finish |
| 3 | Mar 5, 2026 | Codex, Gemini CLI | Structured specs, leases, per-session state |
| 4 | Mar 5, 2026 | Codex, Gemini CLI | Lease fencing, session ordering, spec authority, process-tree |
| 5 | Mar 5, 2026 | Codex, Gemini CLI | Out-of-band cancel, orphaned state, trust model |
| 6 | Mar 5, 2026 | Codex | Doc consistency: removed queued cancel remnants, fixed result keying pseudo-code |
| 6.1 | Mar 5, 2026 | Internal | Orphaned doesn't block session; cancel requires directive (handler just passes through) |

---

## Quick Start for Reviewers

```bash
# Browse docs
cd docs/
cat README.md

# Check implemented code
cd ../internal/dispatch
cat service.go

# Check the dispatch tool
cd ../internal/agent/tools
cat dispatch.go dispatch.md
```

---

## Contact

After reviewing, drop your `{YYYY-MM-DD_HH-MM}_{name}-review.md` file in this `forReview/` directory.

Thank you for your time and critical eyes.
