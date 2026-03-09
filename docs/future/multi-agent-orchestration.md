# Multi-Agent Orchestration

**Status: Future Vision**

This document describes the full architecture for a session-based dispatch system that enables distributed AI agents to collaborate. It is the reference design for where we're heading.

For what's implemented now, see [implemented/dispatch-mvp.md](../implemented/dispatch-mvp.md).

For what we're building next, see [active-design/dispatch-api.md](../active-design/dispatch-api.md).

---

## Overview

A platform-agnostic orchestration system where Crush (Host) coordinates work across multiple AI CLI tools. The fundamental unit is the **session** - a unique combination of worker type, machine, and directory.

**Mental model:** Email for AI agents. Dispatches are emails. Sessions are mailboxes. The API is the mail server.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      DISPATCH SERVICE                            │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  API Server (HTTP/HTTPS)                                    ││
│  │  - POST /machine              (register machine)            ││
│  │  - POST /sessions             (register session)            ││
│  │  - GET  /sessions             (list sessions)               ││
│  │  - GET  /machine/{id}/commands (long-poll for commands)    ││
│  │  - POST /commands/{id}/claim  (claim command, get lease)   ││
│  │  - POST /commands/{id}/heartbeat (renew lease)             ││
│  │  - POST /commands/{id}/start  (report started)              ││
│  │  - POST /commands/{id}/finish (report finished)             ││
│  │  - POST /dispatch             (create dispatch)             ││
│  │  - GET  /dispatch/{id}        (read dispatch)               ││
│  │  - POST /dispatch/{id}/result (submit result)               ││
│  │  - POST /dispatch/{id}/activity (stream activity)           ││
│  │  - POST /dispatch/{id}/cancel (cancel dispatch)             ││
│  │  - POST /dispatch/{id}/redirect (redirect with lineage)     ││
│  └─────────────────────────────────────────────────────────────┘│
│                              │                                   │
│  ┌───────────────────────────┴───────────────────────────────┐  │
│  │  Database (SQLite or Postgres)                             │  │
│  │  - Sessions (worker_type + machine + directory)            │  │
│  │  - Dispatch documents (audit log)                          │  │
│  │  - Result documents                                        │  │
│  │  - Activity logs                                           │  │
│  │  - Commands (durable queue with leases)                    │  │
│  │  - Machine registry                                        │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
         ▲              ▲              ▲              ▲
         │              │              │              │
    ┌────┴────┐    ┌────┴────┐    ┌────┴────┐    ┌────┴────┐
    │ Machine │    │ Machine │    │ Machine │    │ Machine │
    │    A    │    │    B    │    │    C    │    │    D    │
    │ Handler │    │ Handler │    │ Handler │    │ Handler │
    │ +sessions│   │ +sessions│   │ +sessions│   │ +sessions│
    └─────────┘    └─────────┘    └─────────┘    └─────────┘
```

---

## Terminology

| Term | Definition |
|------|------------|
| **Host** | The CLI that coordinates workers (Crush) |
| **Machine** | A physical or virtual machine running a handler |
| **Handler** | One process per machine that manages workers, polls for commands |
| **Session** | The fundamental unit: worker_type + machine + directory |
| **Worker** | An ephemeral CLI tool that executes tasks |
| **Dispatch** | A task definition stored in the database |
| **Command** | A durable spawn instruction stored as structured spec |
| **Lease** | Time-limited ownership of a command, must be renewed |
| **Activity** | Telemetry from worker's session, streamed to server |

---

## Key Principles

### 1. Sessions Are the Fundamental Unit

Each session is a unique combination of worker type, machine, and directory.

```
Session = {worker_type} + {machine} + {directory}

Examples:
  goose-frontend:  goose  + laptop-A + /repo/frontend
  goose-backend:   goose  + laptop-B + /repo/backend
  crush-frontend:  crush  + laptop-A + /repo/frontend
```

### 2. Directory Constraint

**One session per worker type per machine per directory.**

```
laptop-A:/repo/frontend:
  ✅ goose-frontend (goose)
  ✅ crush-frontend (crush)
  ❌ Another goose session (BLOCKED - would share agentic documents)

laptop-B:/repo/frontend:
  ✅ goose-frontend (goose)  -- Different machine, allowed

Why? Agents working in the same directory share state files, context, and agentic documents.
Multiple agents of the same type in one directory would corrupt each other's work.
```

### 3. Nested Directory Constraint

**Nested directories are not allowed for the same worker type on the same machine.**

```
laptop-A:
  ✅ Session at /repo
  ❌ Session at /repo/backend (BLOCKED - nested under /repo)

Why? A worker at /repo may traverse into /repo/backend, causing conflicts.
```

### 4. One Handler Per Machine

Each machine runs one handler process. The handler manages multiple sessions concurrently.

```
Machine "dev-laptop-1":
  Handler (single process)
    ├─ goose-frontend session (spawned on demand)
    ├─ goose-backend session (spawned on demand)
    └─ crush-frontend session (spawned on demand)
```

### 5. Handler Polls, Server Stores

Server cannot reach handlers (firewalls, NAT). Handlers initiate all communication.

```
Handler: GET /machine/{id}/commands?wait=30
Server: Holds connection up to 30s, returns when command arrives
```

### 6. Handler Is a Dumb Executor

Server sends structured execution specs. Handler executes them directly.

```
Server sends: { "spec": { "binary": "goose", "args": ["run", "fix auth"], "cwd": "/repo/frontend" } }
Handler: exec.Command("goose", "run", "fix auth") -- no shell
```

No shell, no string parsing, no command building.

### 7. Commands Are Durable with Leases

Every command goes through three phases with lease-based ownership:

```
1. CLAIM - Handler takes ownership (gets lease + generation)
   → Server returns: lease_expires_at, lease_generation
   → Handler must include generation on all subsequent calls
   → Prevents stale writes after reclaim

2. START - Process is running (with PID fingerprint)
   → Server validates generation matches
   → Server marks: started_at, pid
   → Lease continues

3. FINISH - Process exited
   → Server validates generation matches
   → Server marks: completed_at, exit_code, result
   → Terminal state
```

**Lease Fencing:**
- Each claim returns a `lease_generation` integer
- Generation increments on each new claim
- Handler must include generation on heartbeat/start/finish
- Server rejects if generation doesn't match (stale handler)

### 8. Session-Scoped Command Ordering

**One actionable command per session at a time.**

```
Server enforces:
  - If session has a command in pending/claimed/started state,
    do not return another command for that session.
  - Next command blocked until current is terminal (completed/failed/orphaned).
  - Orphaned commands do NOT block the session - it becomes available immediately.
```

### 9. Spec Authority Validation

Handler validates that spec matches session registration:

```
- spec.binary must match session's binary_name or binary_path
- spec.cwd must equal session's directory exactly

This prevents server from sending arbitrary commands.
```

### 10. Ephemeral Workers

Workers are spawned, work, exit. Handler manages their lifecycle.

### 11. Redirect Creates Lineage

When redirecting, we track the chain.

```
Dispatch A: superseded, redirect_reason: "wrong approach"
Dispatch B: parent_id: A, attempt: 2
```

### 12. Session History is Local

Each CLI keeps its own session records. Dispatch system stores:
- Sessions (worker_type + machine + directory)
- Dispatches (tasks)
- Results (outputs)
- Activity (telemetry)
- Commands (audit trail)

### 13. Two Deployment Modes

| Mode | Auth | Use Case |
|------|------|----------|
| Local | None | Single machine, same-machine trust |
| Cloud | Required | Distributed, untrusted networks |

---

## The Flow

```
SETUP:
1. Register machine:
   POST /machine {machine_id: "laptop-1"}

2. Register sessions:
   POST /sessions {name: "goose-frontend", worker_type: "goose", machine_id: "laptop-1", directory: "/repo/frontend", cli_command: "goose run"}

3. Handler starts:
   - Read per-session state files, kill any orphans (using process fingerprint)
   - Retry any pending results from previous runs
   - Begin long-poll loop
   - Start heartbeat goroutine

DISPATCH:
1. Host creates dispatch targeting session:
   POST /dispatch {session: "goose-frontend", task: "fix auth", timeout: 3600}

2. Server:
   - Creates dispatch document
   - Creates spawn command with structured spec (status: pending)
   - Returns dispatch_id

3. Handler's long-poll returns:
   {id: "cmd-1", action: "spawn", session: "goose-frontend",
    spec: {binary: "goose", args: ["run", "fix auth"], cwd: "/repo/frontend", timeout: 3600}}

4. Handler:
   a. CLAIM: POST /commands/cmd-1/claim → gets lease (expires in 60s)
   b. Start heartbeat goroutine for this command
   c. Save state to per-session file
   d. Spawn worker: exec.Command("goose", "run", "fix auth")
   e. START: POST /commands/cmd-1/start {pid: 12345, pid_start_time: ...}
   f. Watch session file, POST activity (buffered)
   g. On completion:
      - Save result to pending_results in state file
      - FINISH: POST /commands/cmd-1/finish {exit_code, result, stdout, stderr}
      - POST /dispatch/{id}/result (retry until acknowledged)
      - Remove from pending_results

5. Server updates dispatch with result

CANCEL:
1. Human cancels:
   POST /dispatch/{id}/cancel {reason: "taking too long"}

2. Server:
   - Sets cancel_requested=true on the active command (out-of-band, not queued)

3. Handler:
   - Sees cancel_requested=true on next heartbeat response
   - Kills process tree (verifying fingerprint first)
   - Wait path owns terminalization and posts FINISH with
     termination_reason: "canceled"

LEASE EXPIRY:
1. Handler crashes or loses network
2. Server detects lease_expires_at has passed
3. If command was claimed but not started → returns to "pending"
4. If command was started → moves to "orphaned" (not auto-requeued)
5. Orphaned commands require manual reconciliation or handler restart

REDIRECT (Phase 2):
1. Human redirects:
   POST /dispatch/{id}/redirect {reason: "...", new_directive: "..."}

2. Server:
   - Marks old dispatch: status=superseded, redirect_reason
   - Sets cancel_requested=true on active command
   - Creates new dispatch: parent_id=old, attempt++

3. Handler:
   - Sees cancel_requested, kills worker, finishes command
   - On finish, server creates spawn command for new dispatch
   - Handler receives spawn, starts new worker

4. New worker runs with corrected directive
```

---

## The Handler

### Responsibilities

| Responsibility | How |
|----------------|-----|
| Poll for commands | Long-poll GET /machine/{id}/commands |
| Execute commands | Run structured spec directly (no shell) |
| Claim commands | POST /commands/{id}/claim, get lease |
| Renew leases | POST /commands/{id}/heartbeat every 30s |
| Report start | POST /commands/{id}/start with PID fingerprint |
| Report finish | POST /commands/{id}/finish with exit code, output |
| Watch session | Parse session file, POST activity (buffered) |
| Crash recovery | Per-session state files, kill orphans, retry pending results |

### Per-Session State File

```
~/.config/crush/handler-sessions/{session_name}.json

{
  "session": "goose-frontend",
  "machine_id": "laptop-A",
  "current_command": {
    "command_id": "cmd-1",
    "lease_generation": 5,
    "dispatch_id": "abc123",
    "pid": 12345,
    "pid_start_time": 1709600200,
    "lease_expires_at": 1709600400
  },
  "pending_results": [
    {
      "command_id": "cmd-0",
      "lease_generation": 4,
      "dispatch_id": "abc122",
      "exit_code": 0,
      "result": "...",
      "attempts": 2
    }
  ]
}
```

**Note:** Results keyed by `(command_id, lease_generation)`, not just `dispatch_id`.

### Process Fingerprint

PID alone is not enough - PIDs recycle. Store:

```
pid: 12345
pid_start_time: 1709600200  (when the process started)
```

On recovery:

```
1. For each session state file:
   a. If current_command.pid exists:
      - Find process by PID
      - Check if process start time matches pid_start_time
      - If match: kill it (our orphan)
      - If no match: different process, leave it alone
   b. Clear current_command
   c. Retry any pending_results

2. Start polling
```

### Result Persistence

Results must survive network failures:

```
On worker completion:
1. Save result to pending_results in state file (keyed by command_id + lease_generation)
2. POST /dispatch/{id}/result
3. On success: remove from pending_results
4. On failure: keep in pending_results, retry on next poll cycle

Server must handle duplicate result submissions (idempotency).
```

### Atomic State Writes

```
Write to: {session}.json.tmp
Then: os.Rename("{session}.json.tmp", "{session}.json")

Prevents corruption if crash happens mid-write.
```

### Main Loop

```
1. Recover: For each session state file, kill orphan if exists (verify fingerprint)
2. Retry: Submit any pending_results from previous runs
3. Poll: GET /machine/{id}/commands?wait=30
4. For each command (concurrently per session):
   a. Claim: POST /commands/{id}/claim → get lease
   b. Start heartbeat goroutine
   c. Execute: Run structured spec
   d. Start: POST /commands/{id}/start {pid, pid_start_time}
   e. Watch session, post activity (buffered)
   f. Finish: POST /commands/{id}/finish {exit_code, ...}
   g. Submit result: POST /dispatch/{id}/result (with retry)
5. Repeat
```

---

## State Machine

### Command States

```
                    ┌──────────────────────────────────────┐
                    │                                      │
                    ▼                                      │
┌─────────┐    ┌─────────┐    ┌─────────┐    ┌────────────┴┐
│ pending │───►│ claimed │───►│ started │───►│ completed   │
└─────────┘    └─────────┘    └─────────┘    └─────────────┘
                                   │
                              (lease expires)
                                   │
                                   ▼
                             ┌──────────┐
                             │ orphaned │
                             └──────────┘
```

### State Transitions

| From | To | Trigger |
|------|----|---------|
| pending | claimed | POST /claim |
| claimed | started | POST /start |
| started | completed | POST /finish |
| claimed | pending | Lease expires (not started yet) |
| started | orphaned | Lease expires (process may still be running) |

### Orphaned State

When a started command's lease expires:

```
- Command moves to "orphaned" state (not pending)
- Server does NOT auto-requeue for another handler
- Requires manual reconciliation or handler restart

Why: A process may still be running locally. Auto-requeue could cause
duplicate execution. Handler must recover explicitly.
```

### Cancel (Out-of-Band, Not Queued)

Cancel is not a queued command. It sets a bit on the active command:

```
1. POST /dispatch/{id}/cancel {reason: "..."}
2. Server sets cancel_requested=true on the active command
3. Handler sees cancel_requested=true during next heartbeat
4. Handler kills process
5. Wait/monitor path owns terminalization and calls finish with
   termination_reason: "canceled"

This avoids the deadlock: cancel doesn't need to wait for spawn to be terminal.
```

### Idempotency

All endpoints are idempotent:

- **Claim**: Returns current state if already claimed by same handler
- **Start**: No-op if already started (validates lease_generation)
- **Finish**: No-op if already finished (validates lease_generation)
- **Result**: Uses `(command_id, lease_generation)` as idempotency key

---

## Redirect Lineage

When a worker is redirected, we maintain an audit trail.

```
Attempt 1:
  dispatch_id: abc123
  task: "Fix auth bug"
  status: superseded
  redirect_reason: "Using wrong library"
  completed_at: ...

Attempt 2:
  dispatch_id: def456
  parent_id: abc123
  task: "Fix auth bug using validation library"
  attempt: 2
  status: completed
  result: "Fixed in login.go:42"
```

This enables:
- Cost tracking per attempt
- Debugging why attempts failed
- Understanding the correction history

---

## Security

### Structured Execution Spec

**Never use `sh -c` with raw strings.** Commands are sent as structured specs:

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

| Issue | `sh -c` string | Structured spec |
|-------|----------------|-----------------|
| Command injection | Vulnerable | Args are separate, no shell parsing |
| Cross-platform | Shell syntax varies | Direct exec, no shell |
| Security audit | Hard to verify | Easy to validate binary + args |
| Cloud mode signing | Must sign whole string | Sign structured JSON |

### Local Mode

- No authentication
- Trust same-machine users
- Still uses structured specs (no shell)

### Cloud Mode

- Worker token required
- Structured spec signed by server
- Handler verifies signature before execution

---

## Database Schema

### machines

```sql
CREATE TABLE machines (
  id TEXT PRIMARY KEY,
  handler_instance_id TEXT,  -- Distinguishes restarts
  last_poll_at INTEGER,
  last_heartbeat_at INTEGER,
  created_at INTEGER NOT NULL
);
```

### sessions

```sql
CREATE TABLE sessions (
  name TEXT PRIMARY KEY,
  worker_type TEXT NOT NULL,
  machine_id TEXT NOT NULL,
  directory TEXT NOT NULL,  -- Canonical absolute path
  cli_command TEXT NOT NULL,
  binary_name TEXT NOT NULL,  -- e.g., "goose"
  binary_path TEXT NOT NULL,  -- e.g., "/usr/local/bin/goose"
  status TEXT DEFAULT 'available',
  current_dispatch_id TEXT,
  last_activity_at INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  FOREIGN KEY (machine_id) REFERENCES machines(id),
  FOREIGN KEY (current_dispatch_id) REFERENCES dispatch_messages(id),
  UNIQUE (worker_type, machine_id, directory)  -- Correct constraint
);

-- Nested directory check enforced in application logic
```

### commands

```sql
CREATE TABLE commands (
  id TEXT PRIMARY KEY,
  machine_id TEXT NOT NULL,
  action TEXT NOT NULL,  -- spawn
  session TEXT NOT NULL,

  -- Structured execution spec (not raw string)
  spec_json TEXT NOT NULL,  -- JSON: {binary, args, cwd, timeout}

  dispatch_id TEXT,
  status TEXT DEFAULT 'pending',  -- pending, claimed, started, completed, failed, orphaned
  claimed_at INTEGER,
  claimed_by TEXT,
  lease_generation INTEGER DEFAULT 0,  -- Increments on each claim
  lease_expires_at INTEGER,
  started_at INTEGER,
  pid INTEGER,
  pid_start_time INTEGER,
  cancel_requested INTEGER DEFAULT 0,  -- Out-of-band cancel flag
  cancel_reason TEXT,
  cancel_requested_at INTEGER,
  completed_at INTEGER,
  exit_code INTEGER,
  result TEXT,
  termination_reason TEXT,  -- exited, killed, timeout, canceled
  created_at INTEGER NOT NULL,
  FOREIGN KEY (machine_id) REFERENCES machines(id),
  FOREIGN KEY (session) REFERENCES sessions(name),
  FOREIGN KEY (dispatch_id) REFERENCES dispatch_messages(id)
);
```

### dispatch_messages (extensions)

```sql
ALTER TABLE dispatch_messages ADD COLUMN session TEXT;
ALTER TABLE dispatch_messages ADD COLUMN parent_id TEXT;
ALTER TABLE dispatch_messages ADD COLUMN redirect_reason TEXT;
ALTER TABLE dispatch_messages ADD COLUMN attempt INTEGER DEFAULT 1;
ALTER TABLE dispatch_messages ADD COLUMN timeout INTEGER;
```

---

## CLI Commands

```bash
# Server
crush dispatch serve --local
crush dispatch serve --cloud --port 443

# Register machine (on worker machine)
crush register --machine "laptop-1"

# Register session
crush session create --name "goose-frontend" --worker-type goose --machine "laptop-1" --directory /repo/frontend --command "goose run"
crush session list
crush session show <name>

# Dispatches
crush dispatch list
crush dispatch show <id>
crush dispatch create --session "goose-frontend" --task "fix auth" --timeout 3600
crush dispatch cancel <id> --reason "..."
crush dispatch redirect <id> --reason "..." --directive "..."
```

---

## Benefits

- **Simple transport:** Long-polling works everywhere
- **Robust:** Crash recovery with process fingerprint, leases, result persistence
- **Observable:** Activity stream, lineage tracking
- **Steerable:** Redirect with audit trail
- **Secure:** Structured specs, no shell, no command injection
- **Scalable:** One handler per machine, concurrent sessions
- **Conflict-free:** Directory + nesting constraint prevents corruption
- **Reliable:** Leases enable distributed recovery, idempotent endpoints

