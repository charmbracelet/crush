# Dispatch API Server

**Status: Active Design (Not Yet Implemented)**

A REST API server with session-based dispatch and long-poll command delivery. Sessions are the fundamental unit - each session is a unique combination of worker type, machine, and directory.

## Why We Need This

The MVP only works on a single machine. Workers spawn locally and write directly to SQLite. To support:
- Workers on different machines
- Cloud-based coordination
- Authenticated access
- Centralized audit logs
- Live observability of worker activity

We need an API layer and handlers that act as the server's execution proxies on worker machines.

---

## Core Concept: Sessions

**Session = worker_type + machine + directory**

```
Examples:

goose-frontend:
  worker_type: goose
  machine: laptop-A
  directory: /repo/frontend
  status: available

goose-backend:
  worker_type: goose
  machine: laptop-B
  directory: /repo/backend
  status: busy
  current_dispatch: abc123

crush-frontend:
  worker_type: crush
  machine: laptop-A
  directory: /repo/frontend
  status: available
```

### Directory Constraint

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

### Nested Directory Constraint

**Nested directories are not allowed for the same worker type on the same machine.**

```
laptop-A:
  ✅ Session at /repo
  ❌ Session at /repo/backend (BLOCKED - nested under /repo)

Why? A worker at /repo may traverse into /repo/backend, causing conflicts with a
dedicated backend session. Either use one session at /repo, or use separate sessions
at /repo/frontend and /repo/backend (non-nested).
```

### Session States

```
available:       No dispatch running, ready to accept work
busy:            Dispatch in progress
needs_recovery:  Session blocked by an orphaned command until reconciled
```

---

## Architecture Overview

```
SERVER                          MACHINE (laptop-A)
  │                                 │
  │                                 ▼
  │                           ┌───────────────┐
  │                           │   HANDLER     │
  │                           │               │
  │◄───────────────────── long-poll ──────────┤
  │  GET /machine/{id}/commands?wait=30       │
  │                           │               │
  │                           │  Sessions:    │
  │                           │  - goose-fe   │
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
  │◄────────────────────── POST activity ─────┤
  │◄────────────────────── POST claim ────────┤
  │◄────────────────────── POST heartbeat ────┤
  │◄────────────────────── POST finish ───────┤
```

---

## API Endpoints

### POST /sessions

Register a session.

**Request:**
```json
{
  "name": "goose-frontend",
  "worker_type": "goose",
  "machine_id": "laptop-A",
  "directory": "/repo/frontend",
  "cli_command": "goose run"
}
```

**Response:**
```json
{
  "name": "goose-frontend",
  "status": "available",
  "binary_path": "/usr/local/bin/goose"
}
```

**Validation:**
- Worker type + machine + directory combination must be unique
- Machine must exist
- Directory must not nest under or contain an existing session of same worker type on same machine
- Binary is resolved to absolute path via `exec.LookPath` at registration time

**Binary Resolution:**
- Handler calls `exec.LookPath("goose")` to get absolute path
- Stores both `binary_name` ("goose") and `binary_path` ("/usr/local/bin/goose")
- Server's spec can use either form, handler validates against both

### GET /sessions

List all sessions.

**Query params:**
- `machine_id` - Filter by machine
- `worker_type` - Filter by worker type
- `status` - Filter by status (`available`, `busy`, `needs_recovery`)

**Response:**
```json
{
  "sessions": [
    {"name": "goose-frontend", "worker_type": "goose", "machine_id": "laptop-A", "directory": "/repo/frontend", "status": "available"},
    {"name": "goose-backend", "worker_type": "goose", "machine_id": "laptop-B", "directory": "/repo/backend", "status": "needs_recovery"}
  ]
}
```

### POST /dispatch

Create a dispatch targeting a session.

**Request:**
```json
{
  "session": "goose-frontend",
  "task": "Fix the auth bug in login.go",
  "timeout": 3600
}
```

**Response:**
```json
{
  "dispatch_id": "abc123",
  "session": "goose-frontend",
  "status": "queued"
}
```

**Validation:**
- Session must exist
- Session must be available (or dispatch is queued)

### GET /dispatch/{id}

Read dispatch status and result.

### POST /dispatch/{id}/cancel

Cancel a running dispatch. (Phase 1)

Cancel is out-of-band (not a queued command). It sets a bit that the handler
observes during heartbeat.

**Request:**
```json
{
  "directive": "Stop immediately, human intervention required"
}
```

**Response:**
```json
{
  "status": "canceling",
  "cancel_requested_at": 1709600500
}
```

**Flow:**
1. Server sets `cancel_requested=true`, stores `cancel_directive`
2. Handler sees it on next heartbeat response
3. Handler kills process tree
4. Handler calls `POST /commands/{id}/finish` with `termination_reason: "canceled"`
5. Handler posts whatever result it has

The `directive` is for the caller/next command. Handler doesn't interpret it - just passes it through.

### POST /dispatch/{id}/redirect

Cancel and respawn with new directive. (Phase 2)

### POST /dispatch/{id}/result

Submit result. Called by handler when worker completes. Idempotent - safe to retry.

### POST /dispatch/{id}/activity

Stream activity from handler.

---

## Machine & Handler Endpoints

### POST /machine

Register a machine.

**Request:**
```json
{
  "machine_id": "laptop-A"
}
```

### GET /machine/{id}/commands

Long-poll for commands. Handler's main loop.

**Query params:**
- `wait` - Max seconds to wait (default 30)

**Response:**
```json
{
  "commands": [
    {
      "id": "cmd-123",
      "action": "spawn",
      "session": "goose-frontend",
      "dispatch_id": "abc123",
      "spec": {
        "binary": "goose",
        "args": ["run", "Fix the auth bug in login.go"],
        "cwd": "/repo/frontend",
        "timeout": 3600
      }
    }
  ]
}
```

### POST /commands/{id}/claim

Claim a command. Prevents other handlers from executing it. Returns a lease with generation.

**Request:**
```json
{
  "handler_id": "handler-abc"
}
```

**Response:**
```json
{
  "lease_expires_at": 1709600400,
  "lease_generation": 5
}
```

### POST /commands/{id}/heartbeat

Renew the lease on a claimed/started command. Must be called before lease expires.

**Request:**
```json
{
  "handler_id": "handler-abc",
  "lease_generation": 5
}
```

**Response:**
```json
{
  "lease_expires_at": 1709600700,
  "cancel_requested": false
}
```

**If cancel requested:**
```json
{
  "lease_expires_at": 1709600700,
  "cancel_requested": true,
  "cancel_directive": "Stop immediately, human intervention required"
}
```

Handler kills process, calls finish, posts result. Handler doesn't interpret the directive.

**Error (stale generation):**
```json
{
  "error": "stale_lease_generation",
  "current_generation": 6
}
```

### POST /commands/{id}/start

Report execution started.

**Request:**
```json
{
  "lease_generation": 5,
  "pid": 12345,
  "started_at": 1709600300
}
```

### POST /commands/{id}/finish

Report execution finished. Idempotent - safe to retry.

**Request:**
```json
{
  "lease_generation": 5,
  "exit_code": 0,
  "result": "Fixed in login.go:42",
  "stdout": "...",
  "stderr": "...",
  "termination_reason": "exited"
}
```

---

## Command Execution: Structured Spec

**Security: Never use `sh -c` with raw strings.**

Commands are sent as structured execution specs, not shell strings:

```json
{
  "spec": {
    "binary": "goose",
    "args": ["run", "Fix the auth bug"],
    "cwd": "/repo/frontend",
    "env": {},
    "timeout": 3600
  }
}
```

### Why Structured Specs

| Issue | `sh -c` string | Structured spec |
|-------|----------------|-----------------|
| Command injection | Vulnerable to `'rm -rf ~'` | Args are separate, no shell parsing |
| Cross-platform | Shell syntax varies | Direct exec, no shell |
| Security audit | Hard to verify | Easy to validate binary + args |
| Cloud mode signing | Must sign whole string | Sign structured JSON |

### Handler Execution

```go
func (h *Handler) spawnWorker(ctx context.Context, cmd Command) error {
    spec := cmd.Spec

    // Validate spec matches session authority
    session := h.getSession(cmd.Session)
    if !h.specMatchesSession(spec, session) {
        return fmt.Errorf("spec binary/cwd does not match session registration")
    }

    // Direct execution - no shell
    execCmd := exec.Command(spec.Binary, spec.Args...)
    execCmd.Dir = spec.Cwd

    // Process group for clean tree termination (Unix)
    execCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

    // Set timeout
    if spec.Timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, time.Duration(spec.Timeout)*time.Second)
        defer cancel()
    }

    // Capture output (with truncation)
    execCmd.Stdout = newLimitedBuffer(10 * 1024 * 1024) // 10MB max
    execCmd.Stderr = newLimitedBuffer(10 * 1024 * 1024)

    // ... rest of execution
}

// Kill process tree (Unix)
func (h *Handler) killProcessTree(pid int) error {
    // Kill the process group (negative PID = group)
    return syscall.Kill(-pid, syscall.SIGKILL)
}
```

### Spec Authority Validation

Handler validates that spec matches session registration:

```go
func (h *Handler) specMatchesSession(spec Spec, session Session) bool {
    // Binary must match session's registered binary (by name or absolute path)
    if spec.Binary != session.BinaryName && spec.Binary != session.BinaryPath {
        return false
    }

    // Cwd must match session's registered directory exactly
    if spec.Cwd != session.Directory {
        return false
    }

    return true
}
```

This prevents server from sending arbitrary binaries or directories.

### Trust Model

**Local Mode:**
- Server is trusted. It controls binary, args, cwd, timeout.
- Handler validation is a sanity check, not a security boundary.
- If server is compromised, game over anyway (it has DB access).

**Cloud Mode (Phase 2):**
- Server is not fully trusted by handler.
- Spec must be signed by authorized party.
- Handler validates signature before execution.
- Environment variables: allowlist only (or none initially).

---

## Ack Protocol: Claim/Start/Finish with Leases

Every command goes through three phases with lease-based ownership:

```
1. CLAIM - Handler takes ownership (gets lease + generation)
   POST /commands/{id}/claim
   → Server returns: lease_expires_at, lease_generation
   → Handler stores generation, must include on all subsequent calls
   → Prevents duplicate execution

2. START - Process is running
   POST /commands/{id}/start {lease_generation: 5, pid: 12345, ...}
   → Server validates generation matches current
   → Server marks: started_at, pid

3. FINISH - Process exited
   POST /commands/{id}/finish {lease_generation: 5, exit_code: 0, ...}
   → Server validates generation matches current
   → Server marks: completed_at, exit_code, result
   → Terminal state
```

### Lease Semantics

```
Default lease duration: 60 seconds
Heartbeat interval: Every 30 seconds (half of lease duration)

On lease expiry:
  - Server marks command as "abandoned"
  - Command returns to "pending" state
  - lease_generation is incremented
  - Any calls with old generation are rejected (stale handler)
```

### Lease Fencing (Prevents Stale Writes)

```
Problem: Handler A claims, network partitions, lease expires,
         Handler B claims, Handler A reconnects and tries to finish.

Solution: Each claim returns a lease_generation integer.
          - Generation increments on each new claim
          - Handler must include generation on heartbeat/start/finish
          - Server rejects if generation doesn't match

Example:
  1. Handler A claims → generation=5
  2. Network fails, lease expires
  3. Handler B claims → generation=6
  4. Handler A tries finish with generation=5 → REJECTED (stale)
```

### Why Leases

| Scenario | Without leases | With leases |
|----------|----------------|-------------|
| Handler claims, then dies | Server thinks in progress forever | Lease expires, command recoverable |
| Handler loses network mid-execution | Server has no way to detect | Lease expires, can reconcile |
| Server restarts | Stale state persists | Can identify expired leases |

---

## Crash Recovery

Handler maintains per-session state with process fingerprint.

### State File (Per-Session)

```
~/.config/crush/handler-sessions/{session_name}.json

{
  "session": "goose-frontend",
  "machine_id": "laptop-A",
  "current_command": {
    "command_id": "cmd-123",
    "lease_generation": 5,
    "dispatch_id": "abc123",
    "pid": 12345,
    "pid_start_time": 1709600200,
    "claimed_at": 1709600100,
    "lease_expires_at": 1709600400
  },
  "pending_results": [
    {
      "command_id": "cmd-122",
      "lease_generation": 4,
      "dispatch_id": "abc122",
      "exit_code": 0,
      "result": "...",
      "attempts": 3
    }
  ]
}
```

**Note:** Results are keyed by `(command_id, lease_generation)`, not just `dispatch_id`.
This prevents stale result replay after reclaim.

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
1. Save result to state file (pending_results)
2. POST /dispatch/{id}/result
3. On success: remove from pending_results
4. On failure: keep in pending_results, retry on next poll cycle

Idempotency:
- Server must handle duplicate result submissions
- Use dispatch_id as idempotency key
```

### Atomic State Writes

```
Write to: {session}.json.tmp
Then: os.Rename("{session}.json.tmp", "{session}.json")

Prevents corruption if crash happens mid-write.
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
- Session becomes available for new commands
- Next command can override or augment the orphaned work

Why: A process may still be running locally. The next command's spec
determines whether to start fresh or continue the work.
```

### Cancel (Out-of-Band, Not Queued)

Cancel is not a queued command. It sets a bit on the active command:

```
1. POST /dispatch/{id}/cancel {directive: "..."}
2. Server sets cancel_requested=true, stores cancel_directive
3. Handler sees cancel_requested=true during next heartbeat
4. Handler kills process, calls finish, posts result

The directive is for the caller/next command. Handler doesn't interpret it.
This avoids the deadlock: cancel doesn't need to wait for spawn to be terminal.
```

### Idempotency

All endpoints are idempotent:

- **Claim**: Returns current state if already claimed by same handler
- **Start**: No-op if already started (validates lease_generation)
- **Finish**: No-op if already finished (validates lease_generation)
- **Result**: Uses `(command_id, lease_generation)` as idempotency key

Result keying by command_id + lease_generation prevents stale handlers
from overwriting results after reclaim.

### Session-Scoped Command Ordering

**One actionable command per session at a time.**

```
Server enforces:
  - If session has a command in pending/claimed/started state,
    do not return another command for that session.
  - Next command for that session is blocked until current is terminal (completed/failed/orphaned).

This prevents:
  - Two handlers acting on different commands for one session
  - Duplicate execution on the same session
```

### Ordering Guarantees

Commands for the same session are delivered in order:

```
1. Server creates commands with sequence numbers per session
2. Server only returns one command per session at a time
3. Handler processes commands in order received

This prevents:
- Double-finishes from race conditions
```

---

## Database Schema

### machines

```sql
CREATE TABLE machines (
  id TEXT PRIMARY KEY,
  handler_instance_id TEXT,  -- Distinguishes restarts on same machine
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
  binary_path TEXT NOT NULL,  -- e.g., "/usr/local/bin/goose" (resolved at registration)
  status TEXT DEFAULT 'available',
  current_dispatch_id TEXT,
  last_activity_at INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  FOREIGN KEY (machine_id) REFERENCES machines(id),
  FOREIGN KEY (current_dispatch_id) REFERENCES dispatch_messages(id),
  UNIQUE (worker_type, machine_id, directory)  -- One session per worker type per machine per directory
);

-- Prevent nested directories for same worker type on same machine
-- (enforced in application logic, not SQL)
```

### commands

```sql
CREATE TABLE commands (
  id TEXT PRIMARY KEY,
  machine_id TEXT NOT NULL,
  action TEXT NOT NULL,  -- spawn
  session TEXT NOT NULL,
  dispatch_id TEXT,

  -- Structured execution spec (not raw string)
  spec_json TEXT NOT NULL,  -- JSON: {binary, args, cwd, timeout}

  status TEXT DEFAULT 'pending',  -- pending, claimed, started, completed, failed, orphaned
  claimed_at INTEGER,
  claimed_by TEXT,
  lease_generation INTEGER DEFAULT 0,  -- Increments on each claim, used for fencing
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

## Handler Implementation

### Main Loop

```go
func (h *Handler) Run(ctx context.Context) error {
    // Recover from crash
    h.recoverOrphans()

    // Start heartbeat goroutine
    go h.heartbeatLoop(ctx)

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            // Long-poll for commands
            cmds, err := h.pollCommands(ctx, 30*time.Second)
            if err != nil {
                // Add jitter and backoff
                time.Sleep(time.Second + time.Duration(rand.Intn(5))*time.Second)
                continue
            }

            // Execute each command (concurrently per session)
            for _, cmd := range cmds {
                go h.processCommand(ctx, cmd)
            }
        }
    }
}

func (h *Handler) heartbeatLoop(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            h.renewAllLeases(ctx)
        }
    }
}

func (h *Handler) renewAllLeases(ctx context.Context) {
    for _, session := range h.getActiveSessions() {
        resp, err := h.heartbeat(ctx, session.CurrentCommand.CommandID, session.CurrentCommand.LeaseGeneration)
        if err != nil {
            // Retry on transient errors
            continue
        }
        if resp.CancelRequested {
            // Signal cancellation (don't finish here - wait path owns terminalization)
            session.RequestCancel(resp.CancelReason)
            // Kill process tree - wait() will see process exit and finish
            h.killProcessTree(session.CurrentCommand.PID)
        }
    }
}

func (h *Handler) processCommand(ctx context.Context, cmd Command) {
    // Claim command
    lease, err := h.claimCommand(ctx, cmd.ID)
    if err != nil {
        return // Another handler claimed it
    }

    // Only spawn action (cancel is out-of-band, not queued)
    h.spawnWorker(ctx, cmd, lease)
}
```

### Execute Command

```go
func (h *Handler) spawnWorker(ctx context.Context, cmd Command, lease Lease) error {
    session := h.getSession(cmd.Session)
    spec := cmd.Spec

    // Save state (before starting, for crash recovery)
    session.SetCurrentCommand(cmd, lease)

    // Direct execution - no shell
    execCmd := exec.Command(spec.Binary, spec.Args...)
    execCmd.Dir = spec.Cwd

    // Capture output
    var stdout, stderr bytes.Buffer
    execCmd.Stdout = &stdout
    execCmd.Stderr = &stderr

    // Apply timeout
    if spec.Timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, time.Duration(spec.Timeout)*time.Second)
        defer cancel()
    }

    // Start process
    if err := execCmd.Start(); err != nil {
        session.ClearCurrentCommand()
        h.finishCommand(ctx, cmd.ID, FinishParams{
            ExitCode:          -1,
            TerminationReason: "failed_to_start",
            Stderr:            err.Error(),
        })
        return err
    }

    // Report start with process fingerprint
    pidStartTime := h.getProcessStartTime(execCmd.Process.Pid)
    h.startCommand(ctx, cmd.ID, execCmd.Process.Pid, pidStartTime)
    session.UpdatePID(execCmd.Process.Pid, pidStartTime)

    // Wait for completion
    err := execCmd.Wait()
    exitCode := 0
    terminationReason := "exited"

    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            exitCode = exitErr.ExitCode()
        }
        if ctx.Err() == context.DeadlineExceeded {
            terminationReason = "timeout"
            execCmd.Process.Kill()
        }
    }

    // Prepare result
    result := FinishParams{
        ExitCode:          exitCode,
        TerminationReason: terminationReason,
        Stdout:            stdout.String(),
        Stderr:            stderr.String(),
    }

    // Persist result locally first (for network resilience)
    // Key by command_id + lease_generation to prevent stale result replay
    session.AddPendingResult(cmd.ID, lease.Generation, result)

    // Report finish
    h.finishCommand(ctx, cmd.ID, result)

    // Submit result to dispatch (keyed by command_id + lease_generation)
    h.submitResult(ctx, cmd.ID, lease.Generation, result)

    // Clear state
    session.ClearCurrentCommand()

    return nil
}
```

### Crash Recovery

```go
func (h *Handler) recoverOrphans() {
    sessions := h.loadAllSessionStates()

    for _, session := range sessions {
        if session.CurrentCommand.PID > 0 {
            // Check if process exists and is our orphan
            if proc, err := os.FindProcess(session.CurrentCommand.PID); err == nil {
                // Verify process start time matches
                if h.processStartTimeMatches(proc, session.CurrentCommand.PIDStartTime) {
                    proc.Kill()
                }
            }
        }

        // Clear current command
        session.ClearCurrentCommand()

        // Retry pending results (keyed by command_id + lease_generation)
        for _, pending := range session.PendingResults {
            go h.submitResult(context.Background(), pending.CommandID, pending.LeaseGeneration, pending.Result)
        }
    }
}

func (h *Handler) processStartTimeMatches(proc *os.Process, expectedTime int64) bool {
    // Platform-specific: read /proc/{pid}/stat on Linux,
    // or use process creation time on Windows
    // Return true only if it's the same process we started
    actualTime := h.getProcessStartTime(proc.Pid)
    return actualTime == expectedTime
}
```

---

## Directory Canonicalization

To prevent path-based conflicts:

```go
func canonicalizeDirectory(path string) (string, error) {
    // 1. Convert to absolute path
    abs, err := filepath.Abs(path)
    if err != nil {
        return "", err
    }

    // 2. Resolve symlinks
    resolved, err := filepath.EvalSymlinks(abs)
    if err != nil {
        return "", err
    }

    // 3. Clean path (remove . and ..)
    return filepath.Clean(resolved), nil
}

func directoriesOverlap(dir1, dir2 string) bool {
    // Check if dir1 is parent of dir2 or vice versa
    rel, err := filepath.Rel(dir1, dir2)
    if err != nil {
        return false
    }
    // Nested if relative path doesn't start with ..
    return !strings.HasPrefix(rel, "..")
}
```

---

## Implementation Tasks

### Phase 1

- [ ] Create `internal/api/` package
- [ ] Implement session registry with directory constraint and nesting check
- [ ] Implement long-poll endpoint
- [ ] Implement claim/start/finish with leases
- [ ] Implement heartbeat endpoint
- [ ] Create handler binary (`cmd/handler/`)
- [ ] Implement crash recovery with process fingerprint
- [ ] Implement per-session state files
- [ ] Implement result persistence and retry
- [ ] Add jitter/backoff on reconnect
- [ ] Implement out-of-band cancel signaling
- [ ] Add directory canonicalization

### Phase 2

- [ ] Implement redirect with lineage
- [ ] Implement session activity streaming
- [ ] Add cloud mode authentication
- [ ] Add structured spec signing for cloud mode

## Files to Create

```
internal/api/
├── server.go        # HTTP server setup
├── handlers.go      # REST endpoint handlers
├── sessions.go      # Session management
├── commands.go      # Command queue + ack + leases
└── server_test.go   # Tests

cmd/handler/
└── main.go          # Handler binary entrypoint

internal/handler/
├── handler.go       # Main loop, polling
├── state.go         # Per-session state file management
├── recovery.go      # Crash recovery with fingerprint
├── process.go       # Process management
├── lease.go         # Lease management, heartbeats
└── handler_test.go  # Tests
```

