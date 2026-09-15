# Background Subagents — non-blocking parallel task agents

> **Status:** Plan draft. Tracks issue #3 (fork
> `andikapradanaarif/crush`). Scope: non-blocking `agent` tool
> invocations, a completion-notification seam in the parent loop, an
> on-demand result reader, and a minimal TUI surface. Motivation ties
> into the measured 79-request / ~6.2M-cached-read session
> (`INTRA_TURN_BOUNDARIES.md`): research-style work should happen
> _outside_ the parent context, not inline where it grows the 97K
> verbatim re-read per step.

## Problem

Crush can already run multiple task agents in parallel **within a
single turn** — the model calls `agent` multiple times, fantasy
dispatches them concurrently (semaphore-bounded, ~5), and the parent
blocks until all complete. What it cannot do:

- Run subagents in the **background** while the parent keeps working.
- Let the user interact with the parent while subagents run.
- Notify the parent when a background subagent completes.
- Let the parent read results **on demand**.

A 30-second research task blocks the entire conversation — and inline,
its tool results enter the parent's history and get re-read on every
later step (the exact cost shape measured in session `23826edc`).

## What exists

| Capability                                                                         | Status | Location                                                           |
| ---------------------------------------------------------------------------------- | ------ | ------------------------------------------------------------------ |
| `agent` tool, `AgentParams{Prompt}`                                                | Exists | `internal/agent/agent_tool.go:18-20`                               |
| Task agent is stateless (own child session)                                        | Exists | `coordinator.go:1786-1789` (`runSubAgent`)                         |
| Task agent marked parallel-safe                                                    | Exists | `agent_tool.go:40` (`NewParallelAgentTool`)                        |
| Fantasy dispatches parallel tool calls concurrently                                | Exists | `charm.land/fantasy` agent loop (semaphore ~5)                     |
| Model can call `agent` multiple times per turn                                     | Exists | `agent_test.go` parallel-dispatch coverage                         |
| Deterministic child session id `messageID$$toolCallID`                             | Exists | `workspace/client_workspace.go:176`                                |
| Child sessions persist messages in SQLite                                          | Exists | `session/session.go:110` (`CreateTaskSession`)                     |
| Pubsub events (`notify.Type*`)                                                     | Exists | `agent/notify/notify.go:11`                                        |
| Cost propagation child → parent                                                    | Exists | `coordinator.go:1865-1884`                                         |
| Read-only task agent (glob/grep/ls/view + LSP, no bash/edit/write), model-pinnable | Exists | `DELEGATION.md`; `config.go` agent defaults                        |
| Parent-loop injection seam (auto-inject)                                           | Exists | `agent.go:1744-1748` (run start), `agent.go:897-925` (PrepareStep) |
| UI fetch of nested child messages                                                  | Exists | `ui/model/ui.go:1585-1591`                                         |

## Design

### Core property: isolation, not just parallelism

A background subagent runs in its own session with its own context;
its tool results **never enter the parent's history**. The parent's
context grows only by (a) the one-line "started" tool response and
(b) whatever the parent explicitly pulls via `read_subagent`. This is
the same decoupling the stub machinery achieves mechanically — but
interpretive: the parent pays for a distilled report, never the
intermediate transcript.

### 1. `background` parameter and the registry

`AgentParams` gains two fields:

```go
type AgentParams struct {
    Prompt     string `json:"prompt" description:"The task for the agent to perform"`
    Background bool   `json:"background,omitempty" description:"Run in the background and return immediately; notify on completion"`
    Name       string `json:"name,omitempty" description:"Optional label for the background agent (defaults to truncated prompt)"`
}
```

Foreground behavior is byte-for-byte unchanged. When
`Background=true`, the tool callback:

1. Derives the child session id up front
   (`CreateAgentToolSessionID(messageID, toolCallID)`).
2. Registers the agent in a new coordinator-level
   `backgroundAgentRegistry` keyed by child session id:
   `{parentSessionID, name, status(running|done|error), summary, err,
done chan struct{}}`, guarded by a mutex. (v2 grows this into the
   persisted dispatch ledger — fingerprint + `files_touched` columns —
   so keep the entry shape extensible; see `SWARM_DEDUP.md` §1.)
3. Spawns a goroutine running the existing sub-agent machinery
   against a **detached context** — `context.WithoutCancel` derived
   from the coordinator's context, so cancelling the parent run (or
   the user interrupting) does **not** kill background agents. The
   coordinator's own shutdown cancels them.
4. Returns immediately: `Started background agent "<name>"
(id=<childSessionID>). You will be notified when it completes; use
read_subagent(id=...) to fetch the full results.`

**Concurrency cap.** Fantasy's semaphore does not cover goroutine
spawns, so the registry enforces its own `maxBackgroundAgents`
(default 5, constant or `options` entry). Exceeding it returns a
clear error (`Already 5 background agents running; wait or cancel one
first.`), leaving the model free to retry in foreground.

**Result storage.** Nothing new — the child run persists its messages
in the child session exactly as foreground subagents do today.
`read_subagent` reads from there.

**Cost propagation — fix the existing race.** `updateParentSessionCost`
(`coordinator.go:1865`) is a read-modify-write (`parent.Cost += child`)
with a last-write-wins `Save`. Foreground parallel agents can already
race; background completions make it likely. Serialize the
accumulation (mutex around the read/update/write, or do it inside the
insert transaction) and propagate **exactly once**, on the
running→done transition.

### 2. Completion notification

A completion publishes a pubsub event (new `notify.Type`
`background_agent_finished`, carrying child session id, parent session
id, name, and a ~200-char summary) — UI hooks here. The **parent
model** learns about it through the existing injection seam:

- **Run start**: next to the notebook auto-inject
  (`agent.go:1744-1748`).
- **Per step**: in `PrepareStep`, after the fold block
  (`agent.go:919-925`) and before the cache-control pass — so
  breakpoints and the `promptPrefix` prepend apply to the rebuilt
  list unchanged.

Injected shape (one system message, one per completion, **consumed
once** — the registry tracks notified):

```
<background_agents_completed>
- "research auth" (id=<childSessionID>) finished: <summary>
Use read_subagent(id=...) for full results.
</background_agents_completed>
```

The session agent gets the registry through
`SessionAgentOptions` (a query func `completedBackgroundAgents(sessionID) []Completion`),
keeping `sessionAgent` decoupled — same pattern as `Sessions`/
`Messages` injection.

**Cache note.** Each injection is a real prefix change — one prompt-cache
invalidation per completion. That is the price of the notification;
it is strictly cheaper than the alternative (inline results re-read
every step). Byte-stable: an unchanged registry produces no
injection and no invalidation.

### 3. `read_subagent` tool

New tool, coordinator-implemented (needs `sessions` + `messages`):

```
read_subagent(id) -> status of background agent and full text output
```

- `running` → `"still running"`.
- `done` → full final assistant text of the child session (same
  `subAgentOutput` the foreground path returns, read back from
  storage).
- `error` → the recorded error.
- `unknown` → `"no background agent with id <id>"`.
- **interrupted** (registry lost, e.g. crush restarted mid-run) →
  derive from the child session: a final assistant message exists →
  return it; otherwise `"interrupted by restart"`.

Added to the **coder** toolset only (never to subagents). Companion
edit to `internal/agent/templates/agent_tool.md` teaching the model
the `background` parameter, the notification protocol, and
`read_subagent`.

### 4. TUI

- **Chat renderer** (`ui/chat/agent.go`): the `agent` tool item gains
  a background state (started → running spinner → completed badge /
  failed). Expanding it already fetches nested child messages
  (`ui.go:1585-1591`) — works unchanged once the run finishes.
- **Subagent panel**: a new component listing background agents
  (name, status, elapsed, summary), fed by pubsub. Per
  `internal/ui/AGENTS.md`: imperative methods on a stateful struct,
  no `Update` participation, screen-based drawing. Click/enter opens
  the results (reuses the nested-message renderer).

### 5. Lifecycle and telemetry

- **Parent cancel / interrupt**: background agents survive (detached
  context); their completion notification lands on the next run
  start.
- **Session delete**: cancel that session's background agents.
- **Coordinator shutdown**: cancel all background agents.
- **Restart**: no resume (non-goal). `read_subagent` falls back to
  the interrupted path above.
- **Telemetry**: per-session spawn/completion counters; surfaced in
  `crush stats` (spawned, completed, failed, bytes of results read
  into parent context).

## PR breakdown

1. **Plumbing** — `background` param, registry + cap, detached
   execution, immediate start response, serialized exactly-once cost
   propagation (with the race fix). Tests: 3 concurrent runs all
   complete; cap rejects the 6th; parent cancel does not kill
   children; cost propagated once under concurrent completions.
2. **Notifications** — injection at run start + PrepareStep, consume-
   once dedup, summary truncation. Tests: exactly one notification
   per completion across start+step; notification arrives on the next
   run after a mid-run cancel; no-op registry produces no message and
   no prefix change (golden diff).
3. **`read_subagent`** — tool + description, coder-only wiring,
   `agent_tool.md` update, all four status paths tested.
4. **UI** — renderer states + panel; verify against
   `internal/ui/AGENTS.md` invariants (screen-based drawing, dialog
   width rules if results open in an overlay).
5. **Lifecycle + telemetry** — shutdown/delete cancellation, restart
   fallback, stats counters.

## Acceptance criteria

- 3 background agents + simultaneous parent work: parent continues
  immediately, receives one notification each, `read_subagent`
  returns full results, cost propagated exactly once.
- Parent cancelled mid-run: background agents finish; notification
  appears on the next run start.
- Background results never appear in the parent's stored history
  until `read_subagent` is called (assert on message parts).
- No duplicate notifications or double cost propagation under
  concurrent completions.
- Registry cap enforced with a clear tool error.
- Restart mid-run: finished agents readable, unfinished report
  "interrupted by restart".
- Foreground `agent` calls behave identically to today (regression
  suite passes).

## Non-goals (v1)

- Edit-capable background agents (task agent stays read-only;
  background inherits its restricted toolset).
- Swarm orchestration / planning across agents; auto-delegation
  heuristics (the parent decides when to spawn). Dispatch dedup,
  result caching, and spawn admission live in `SWARM_DEDUP.md`
  (v2).
- Background agents spawning background agents.
- Resume of in-flight agents across crush restarts.
- Multi-parent fan-out (a background agent reports to the session
  that spawned it).

## Risks

- **Goroutine lifecycle** — leaked goroutines on crash paths; the
  registry's done-channel + coordinator-scoped cancel bounds this.
- **Cost-propagation race** — latent today (parallel foreground
  agents), made likely by background; must ship with the fix.
- **Cache invalidation per completion** — one prefix invalidation per
  notification; acceptable vs. the re-read it replaces, but
  measurable — telemetry should record invalidations.
- **Cold-start overhead per spawn** — each background agent is a
  fresh context (static ~10-13K system prompt/context-files/tool
  schemas, no cache carryover; on DashScope implicit cache this is
  first-call full-price). The crossover favors background agents for
  high tool-call-volume recon; not for trivial tasks.
- **Parent context growth on read** — `read_subagent` results land in
  history like any tool result; the parent can over-read. The
  notification's summary is the guard — keep it informative enough to
  defer reading.
- **UI regression** — new panel must follow the dialog/width rules in
  `internal/ui/AGENTS.md`; reuse existing nested-message rendering
  rather than building a parallel one.
