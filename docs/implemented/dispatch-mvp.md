# Dispatch System MVP

**Status: Implemented**

The dispatch system enables Crush to delegate tasks to other AI CLI tools (workers).

## What's Implemented

### 1. Dispatch Tool (`dispatch_task`)

Location: `internal/agent/tools/dispatch.go`

Crush can dispatch tasks to registered workers:

```
dispatch_task(
  worker: "goose",
  task: "Fix the auth bug in login.go",
  variables: { file: "auth/login.go" }
)
```

**What it does:**
1. Validates worker exists in registry
2. Creates dispatch document in database
3. Spawns worker via bash with alert message
4. Returns dispatch ID

### 2. Dispatch Service

Location: `internal/dispatch/service.go`

Database operations for:
- Creating dispatch documents
- Storing results
- Worker registry (create, list, enable/disable)

### 3. CLI Commands

Location: `internal/cmd/dispatch.go`

```bash
# Worker management
crush agents list
crush agents show <name>
crush agents create <name> --command "goose run"
crush agents enable <name>
crush agents disable <name>

# Dispatch inspection
crush dispatch list
crush dispatch show <id>
```

### 4. Database Schema

Location: `internal/db/migrations/20260305000000_add_dispatch_system.sql`

Tables:
- `dispatch_messages` - Dispatch documents and results
- `agents` - Worker registry

## Current Limitations

| Limitation | Why |
|------------|-----|
| No API server | Workers read/write directly to SQLite |
| No auth | Assumes same-machine trust |
| Local only | Can't distribute across machines |
| No spawn tokens | Anyone with dispatch ID can access |

## How Workers Currently Work

Workers are spawned with the task embedded in the alert message. They don't read from an API yet - the task comes directly in the spawn command.

This works for local, same-machine workflows but won't scale to distributed workers.
