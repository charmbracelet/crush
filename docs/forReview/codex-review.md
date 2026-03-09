# Codex Review

## Overall Assessment

The core direction is good: a machine-local handler that pulls work from a
central service is the right boundary for controlling developer-installed CLIs.
Long-polling is a reasonable transport choice, and one handler per machine is a
clean operating model.

The design is still missing the distributed-systems rules that make the model
safe: command claiming, retry/idempotency, execution fencing, and a stricter
trust boundary. If you implement the API exactly as written, the likely failure
modes are duplicate spawns, stale aborts killing the wrong process, stuck
commands after reconnect, and telemetry/results being attached to the wrong
attempt.

There is also a real documentation drift already. The docs say workers read
their task from an API, but the current MVP still embeds an alert string in the
spawn command and launches the process locally from
`internal/dispatch/service.go`. The tool description in
`internal/agent/tools/dispatch.md` is ahead of the implementation.

## Critical Concerns

### 1. Ack semantics are too weak.

`POST /commands/{id}/ack` currently mixes together "I received this command",
"I started it", and in some docs "it completed". Those are different events.
With one ack field you cannot recover cleanly from disconnects or handler
crashes.

You need at least:
- `issued`
- `claimed` or `received`
- `started`
- terminal status: `completed`, `failed`, `canceled`

Otherwise reconnect behavior is ambiguous. A handler may have launched a worker,
crashed before acking, then relaunch the same command on restart.

### 2. The command model is not idempotent.

`spawn` and `abort` must be safe to replay. Today the docs rely on "fetch
un-acked commands again" plus a local state file, but that is not enough.

Needed additions:
- command IDs that are unique and durable
- an execution or lease ID per spawn attempt
- abort commands that target an execution ID, not just a dispatch ID or worker
- server rules for what happens if the same command is claimed twice

Without fencing, a stale abort can kill a newer redirected run.

### 3. The handler is not actually "dumb" once crash recovery and telemetry are added.

The docs describe the handler as a pure executor, but the design gives it real
responsibility for:
- orphan detection
- process ownership
- session watching
- replay after reconnect
- result submission

That is a stateful control-plane component, not a dumb shim. Treating it as
"dumb" risks underdesigning its storage, tests, and security model.

### 4. Command whitelisting alone is not a strong security boundary.

A registered template like `goose run` is not by itself enough protection in
cloud mode. The dangerous part is the untrusted argument payload, working
directory, environment, and the fact that the handler can kill processes.

At minimum I would want:
- machine authentication and server authentication
- revocable machine tokens or mTLS
- per-worker workspace allowlists
- explicit allowed executable plus fixed base args
- argument construction from structured fields, not raw command strings
- process-group isolation so abort kills only the spawned tree
- audit logs for spawn, abort, redirect, and token use

### 5. The current crash recovery story can corrupt live work.

"If PID exists, kill it" is not robust enough. PIDs are recyclable, and after a
restart you may kill an unrelated process if the old child already exited and
the OS reused the PID.

The handler should persist more than PID:
- process start time
- execution ID
- command ID
- dispatch attempt ID
- worker type

On restart, verify the process identity before killing anything.

### 6. Session-file watching is a weak source of truth.

Telemetry via local session files is useful, but these files are tooling-
specific, version-specific, and race-prone during redirects and crashes.

Use session watching only as best-effort activity enrichment. The source of
truth for dispatch state should be explicit handler lifecycle events:
spawned, running, exit code, canceled, upload result failed, and so on.

### 7. The current implementation already shows command-construction fragility.

In `internal/dispatch/service.go`, the MVP uses `strings.Fields` to split the
configured worker command, then appends the alert message as a final argument.
That breaks quoted commands and makes the template format brittle. It also does
not track process state or cancellation beyond a background `CombinedOutput()`
call.

That implementation detail matters because the API design inherits the same
assumption: stringly-typed commands are easy to reason about. They are not.

### 8. Queueing and capacity are underspecified.

"One handler per machine" is fine, but the docs do not define whether each
worker type has one execution slot, many slots, or machine-wide concurrency.
That matters for fairness, backpressure, and redirect behavior.

Before implementation, define:
- max concurrent executions per machine
- max concurrent executions per worker type
- queue order and priority semantics
- whether redirect preempts capacity or re-enters the queue

## Suggestions

### 1. Introduce an explicit execution record.

Separate these concepts:
- `dispatch`: user intent
- `attempt`: one redirect generation of that intent
- `command`: control-plane instruction such as spawn or abort
- `execution`: the concrete running process on a machine

That gives you somewhere to store PID, exit code, start time, machine ID, and
fencing tokens without overloading `dispatch_messages`.

### 2. Split ack into receive/start/finish events.

Recommended flow:
1. Server creates `spawn` command.
2. Handler claims command.
3. Handler starts process and reports execution ID + PID metadata.
4. Handler reports terminal state separately.
5. Server marks command terminal and updates the attempt state.

This is more verbose than a single ack, but it closes most recovery holes.

### 3. Make redirect lineage first-class.

`parent_id + attempt + reason` is close, but I would also add:
- `root_dispatch_id`
- `superseded_by_attempt_id`
- `superseded_at`
- `redirected_from_execution_id`
- `terminal_status_reason`

That makes the audit trail and UI much easier to reason about.

### 4. Build commands from structured registration, not full strings.

Prefer registration like:
```json
{
  "worker_type": "goose",
  "executable": "goose",
  "args": ["run"],
  "allowed_workdirs": ["/repo"],
  "env_allowlist": ["PATH", "HOME"]
}
```

Then the server sends structured task metadata, not a raw shell command.
The handler can construct the argv safely without shell parsing.

### 5. Add handler leases and heartbeats.

For cloud mode especially, each running handler instance should hold a lease.
Only the active lease owner for a machine should be allowed to claim commands,
post activity, or submit results.

### 6. Treat long-polling as fine, but size it operationally.

A 30-second wait is reasonable. I would keep long-polling over simple polling.
Simple polling wastes requests and adds avoidable latency.

For scale, the important part is not the timeout value but:
- heartbeat and lease expiry
- efficient command lookup indexes
- backoff and jitter on reconnect
- limits on outstanding polls per machine

100 machines is trivial. 1000 machines is still fine with long-polling if the
server is event-driven and the database/indexing is competent.

### 7. Narrow the initial MVP.

I would ship phase 1 as:
- register machine
- durable spawn command
- claim/start/finish lifecycle
- result submission
- explicit cancel

Then add:
- redirect
- session-derived telemetry
- richer lineage
- cloud hardening

That sequence reduces the chance of baking recovery bugs into a more complex API.

## Questions

1. Is a machine allowed to run multiple simultaneous executions of the same
worker type, or is each worker type single-flight?
2. When a redirect happens, should the new attempt inherit any context or
partial output from the previous attempt?
3. Who is the intended source of the final result in the steady state: the
worker process, the handler, or either?
4. Is cloud mode intended to tolerate compromised machines, or only honest but
untrusted networks?
5. What is the scheduling key when several machines register the same worker
capability: least-loaded machine, static pinning, affinity, or manual routing?
6. Do you need exactly-once semantics for spawn, or only at-least-once with
idempotent handler behavior?

## Alternative Approaches

### 1. Pull queue with durable claims.

Keep long-polling, but think of it as a wake-up mechanism for a durable queue.
That is the simplest version of this architecture and likely the right one.

### 2. Small adapter layer per CLI.

Instead of reverse-engineering session files, define an adapter contract per
supported tool:
- start task
- stop task
- emit structured progress
- collect final result

That is more work upfront, but it will age better than file-format scraping.

### 3. Reuse a real queue before adopting a full orchestrator.

You are not obviously reinventing Kubernetes. The problem is local-machine
mediation for developer tools, which existing cluster schedulers do not solve
well. But if cloud mode grows, it may be worth using an existing durable queue
or job system for command delivery rather than inventing one inside the API.

### 4. Make redirect a second-wave feature.

Redirect is valuable, but it multiplies the failure surface because it adds
preemption, lineage, and stale-command risks. It should sit on top of a proven
execution/lease model rather than define it.

## Bottom Line

The high-level architecture is sound. The weak point is not long-polling versus
SSE; it is the lack of a precise execution model.

If you add durable command claims, execution IDs, handler leases, structured
command construction, and a stricter crash-recovery contract, this design can
hold up well. If you skip those and go straight to spawn/abort/redirect over a
single ack endpoint, you will spend the next phase debugging duplicate work and
incorrect recovery behavior.
