# Harness Topology — Loop Kernel, Deterministic Edges, Plan Artifact

> **Status:** Spec. Maps the existing machinery (verification gate,
> session todos, subagent dispatch, notebook) onto the loop / graph /
> planning design space, and scopes the three pieces still missing:
> an edge abstraction, a typed plan object, and a join. Composes with
> `BACKGROUND_SUBAGENTS.md` (fan-out), `SWARM_DEDUP.md` (dispatch
> ledger), `EVAL_HARNESS.md` (named transitions to assert on), and
> `VERIFICATION_LOOPS.md` (the first edge, implemented — PRs #28,
> #30).

## Problem

In a flat agent loop the transition relation is complete over the
bound tool set: every tool is reachable from every step, and the
model is the only router. Crush's loop (`agent.Stream` + `StopWhen`,
`agent.go:1228`) is exactly this. The harness cannot see the control
flow because there isn't one — routing, retries, and termination
are all emergent from generation.

Three concrete symptoms, all already visible in this codebase:

1. **Deterministic transitions accrete ad hoc.** The verify gate
   (`verify_gate.go:62`) hand-rolls the entire "run an external check,
   maybe force another turn" pipeline: trigger scan over
   `result.Steps`, check resolution, budget accounting, clone-call
   retry with RunID propagation, sessionMu prepend, cancel-mark
   drop, RunComplete suppression. Each future transition of the same
   shape — stall-replan, join-on-subagents, summarize-and-continue —
   would re-roll that machinery or bolt onto the gate sideways.
   There is one hardcoded edge site; there is no edge _type_.
2. **No plan artifact.** `session.Todo` is
   `{Content, Status, ActiveForm}` (`session.go:34-38`) — flat
   strings, unordered, no dependencies, model-managed. The todos
   gate (`verify_gate.go:533`) can ask "are items open?" and nothing
   more structured. `BACKGROUND_SUBAGENTS.md` needs a dispatch unit;
   today the dispatch spec is a prompt string the model composes
   inline in the `agent` tool call, with no handle the harness can
   track, dedup against, or verify.
3. **No join.** `runSubAgent` (`coordinator.go:1843`) is synchronous —
   `params.Agent.Run` blocks the parent loop; one child at a time.
   Fan-out requires async dispatch plus a barrier transition
   ("this run does not terminate while children are outstanding"),
   and neither exists as a concept.

## The design space, distilled

Three topologies, with the production examples worth stealing from:

- **Loop.** Model-as-router: generate → dispatch tools → repeat until
  text-without-tool-calls. Claude Code (`query()` — one async
  generator runs every interaction; subagents recurse into the same
  function), OpenHands `CodeActAgent` (event stream: Action →
  Observation, a Condenser for history compression, a security
  analyzer as a mid-loop gate), Aider. Flexibility is total; every
  routing decision costs tokens; guarantees are unspecified.
- **Graph.** Explicit nodes/edges over typed state — LangGraph's
  `StateGraph`. The genuinely valuable parts are not the DAG: they
  are (a) named transitions the model cannot route around, and (b)
  a checkpointer giving pause/resume/replay over a state artifact
  richer than message history. Notably LangChain's `create_agent` —
  the "just a loop" API — compiles to a `StateGraph` underneath:
  even loop products sit on a specified runtime once they need
  durability or interrupts.
- **Planning.** Planner produces a structured artifact, executor
  works it, a monitor replans. Devin's planner/executor split;
  OpenHands' optional planning agent; Claude Code's TodoWrite is the
  degenerate case — a model-managed flat list with no executor
  contract — which is precisely what `todos` is today
  (`tools/todos.go:36`).

The honest convergence: every production coding tool kept a loop
kernel and bolted structure on — gates, todos, subagents — rather
than compiling tasks to DAGs. Open-ended coding breaks hand-designed
graphs; the long tail is where the loop earns its keep. The
correctness-relevant subset of graph runtimes is _specified
transitions + durable state_, and both are obtainable without the
DAG.

## What exists — in topology terms

- **Kernel.** `agent.Stream` loop; `StopWhen` (`agent.go:1228`) holds
  auto-summarize and `hasRepeatedToolCalls`
  (`loop_detection.go:19`, window/repeats params) — the only
  deterministic transition today, and it can only stop, never
  reroute. Re-entry is `return a.Run(ctx, firstQueuedMessage)`
  (`agent.go:1563`) under the `sessionMu`/`BeginAccepted` handoff
  (`agent.go:1475-1551`).
- **One working edge — and it is generic inside.**
  `runVerificationGate` (`agent.go:1401` call site;
  `verify_gate.go:62`) already implements the complete edge
  contract: scan the run's steps for a trigger (failed/pending
  verification checks, open todos), run the deterministic part
  (`runGateChecks`, `verify_gate.go:276` — satisfy-from-observed,
  then harness-side shell execution), persist outcomes before the
  notebook goroutine can List (`FlushAll`, `verify_gate.go:106`),
  and enqueue a budgeted retry. The retry is a clone of the caller's
  `SessionAgentCall` with `Prompt` replaced, `VerificationAttempts`
  incremented, `RunID` kept (non-foldable, suppresses the premature
  RunComplete), `Accepted`/`acceptSeq` cleared (cancel-mark
  droppable) — `verify_gate.go:159-175`. **This is a
  "harness-initiated follow-up turn" primitive instantiated exactly
  once.** The multi-trigger aggregation also already exists:
  verification failures and open todos merge into ONE
  `gateRetryPrompt` (`verify_gate.go:559`) — one retry per run
  boundary, merged evidence, single budget increment.
- **Trigger vocabulary.** `result.Steps` (in-memory, whole-run scope,
  folded turns included), `preTurnMsgCount` (`agent.go:841`),
  terminal-step `FinishReason` + `StopTurn` discrimination
  (`verify_gate.go:66-76` — a hook halt or permission denial is not
  a completion claim).
- **State artifacts.** `session.Todos` on the session row;
  `message.VerificationCheck` lists on tool-result `Metadata`
  (union-merged against pre-gate snapshots, `verify_gate.go:448`);
  notebook entries + segment coverage (`internal/notebook/`) — the
  per-event summarized state that already survives compaction;
  filetracker read/write sets.
- **Model slots.** large / small / summary
  (`SUMMARY_MODEL_SWITCHER.md`, implemented) — the precedent for
  per-transition model selection, currently bound to exactly one
  non-loop call site.
- **Fan-out.** `AgentToolName` → `runSubAgent` → `CreateTaskSession`
  (`coordinator.go:1849`): synchronous, child session (context
  isolation is the point — the parent's transcript never sees child
  verbosity; the Claude Code sidechain equivalent), cost propagated
  back to parent. Async dispatch + completion seam + result reader
  are spec'd in `BACKGROUND_SUBAGENTS.md`; the dispatch ledger and
  dedup cache in `SWARM_DEDUP.md`.

### Field traps the edge layer inherits

All already documented in `VERIFICATION_LOOPS.md`; restated because
every new edge must obey them:

- `StopWhen` can only stop the loop — an edge that needs another
  turn must live at the run boundary, never inside `Stream`.
- A follow-up call enqueued without `RunID` while a run is active
  gets _folded_ into a later run's first `drainQueueForStep`
  (`agent.go:528-553`) instead of gating. RunID is the fold
  exemption; `acceptSeq = 0` is the cancel-drop.
- Any counter scoped to a `Run` invocation resets on retry — budgets
  live on `SessionAgentCall` (the `VerificationAttempts` precedent,
  `agent.go:136-141`), not in run locals or session-keyed state.
- Edge writes race the detached post-run goroutine
  (`context.WithoutCancel`, `agent.go:1396`): outcomes must flush
  before the notebook spawn AND union stored `Superseded`/`Metadata`
  (`unionToolMetadata`, `verify_gate.go:448`).
- Sub-agent work lands in a child session — a parent's step scan
  sees only the task-tool result. Edges fire inside each agent's own
  `Run`; parent-side visibility needs the completion seam, not a
  wider scan.

## Design

Three layers, in dependency order. None replaces the loop.

### 1. Edge type — extract, don't add

Promote the gate's generic half into a declarative edge:

```go
// A runEdge is a deterministic transition evaluated at the run
// boundary, after Stream returns and before the queue dequeues.
type runEdge struct {
	name string
	// scan inspects the finished run; nil trigger means no fire.
	scan func(steps []fantasy.StepResult, sess *session.Session) *edgeTrigger
	// resolve runs the deterministic part (shell checks, ledger
	// queries). May be nil for prompt-only edges.
	resolve func(ctx context.Context, t *edgeTrigger) error
	// prompt builds the retry message from resolved evidence.
	prompt func(t *edgeTrigger) string
}
```

Evaluated in sequence at the `agent.go:1401` site. Aggregation rule,
already precedented: all firing edges merge evidence into **one**
retry prompt, one prepend, one budget increment — two edges each
enqueueing a turn would double every repair. Budget becomes a shared
repair counter on `SessionAgentCall` (rename `VerificationAttempts`
→ `RepairAttempts`; per-edge sub-budgets only if a thrash pattern
demands it). `maxVerificationAttempts = 2` (`verify_gate.go:27`)
stays the initial shared bound.

Edges by readiness:

| Edge               | Trigger                       | Status today                          |
| ------------------ | ----------------------------- | ------------------------------------- |
| verification       | failed/pending checks         | implemented (the extraction source)   |
| todos-reconcile    | open plan items at clean stop | implemented (same site)               |
| stall-replan       | loop-detector / no-progress   | new — repurposes the stop-only signal |
| join-subagents     | outstanding dispatch ledger   | blocked on BACKGROUND_SUBAGENTS       |
| summarize-continue | context pressure at run end   | new — reframes auto-summarize         |

The stall-replan row is the tell that this abstraction earns its
keep: `hasRepeatedToolCalls` today _stops_ a thrashing turn — the
information is right and the action is wrong. As an edge it becomes
"evidence of no progress → one replan turn within budget."

Once the project index lands (`CONTEXT_PREFETCH.md`), `resolve`
gains a deterministic evidence source: a repair prompt can carry a
`map` slice over the run's touched dirs (filetracker knows the write
set), so the forced turn starts re-oriented at zero model cost
instead of the model re-gathering in its first calls back — the
re-read loop the index exists to kill, closed at the run boundary
rather than left to the model remembering a tool exists.

Mid-step control is explicitly out of scope for edges — hooks and
permissions own that boundary (`hooked_tool.go`); edges only ever
fire between turns.

### 2. Plan artifact — todos grow types

`session.Todo` → `PlanItem{ID, Content, DependsOn []ID, Status,
Evidence []string}`:

- `DependsOn` is the dispatch unit for fan-out: independent subtrees
  are what a background subagent may be handed, and the join edge
  reads "outstanding" off the same structure.
- `Evidence` binds an item to check names — an item is `completed`
  only when its checks resolved green. This unifies today's two
  gate triggers (failed checks, open todos) into one definition of
  done instead of two scans of the same run. Index paths are a
  second evidence kind: an item bound to `internal/agent/` lets a
  stall-replan edge render the current symbols of the files it
  names (`CONTEXT_PREFETCH.md`) into the replan prompt — the plan
  carries its own map.
- Persistence: todos already ride the session row; the durable win
  is a `plan` notebook entry type so the plan survives compaction as
  ground truth — today the summary has to re-derive what the plan
  was from rendered todo tool calls.
- The model still writes the plan (the `todos` tool graduates, or a
  `plan` tool replaces it with a migration shim reading old
  `session.Todos`). The harness gains structure to _check_; it does
  not gain a planner that replaces the model. That is the side of
  the line the production tools landed on, and it keeps prompt-mode
  cost at zero for tasks too small to plan.

### 3. Join — fan-out needs a barrier

`BACKGROUND_SUBAGENTS.md` supplies async dispatch + the completion
notification seam; `SWARM_DEDUP.md` supplies the ledger. The missing
piece is the barrier edge: at the run boundary, outstanding ledger
entries are a trigger — either block terminal (fold completion
notifications into a wait turn) or, when the parent genuinely has
nothing to contribute, end the turn and let completions arrive as
queued calls. Blocking is the defensible default: a finished
notification claiming done while children still run is the same
premature-done claim the gate exists to catch.

Parent-context protection stays as designed — child session,
summary returns — which is also the token argument: fan-out keeps
the 79-request / ~6.2M-cached-read session profile
(`INTRA_TURN_BOUNDARIES.md`) out of the parent entirely rather than
pruning it after the fact.

## What a graph runtime would still buy — honest accounting

- **Checkpoint/replay, time-travel.** LangGraph's checkpointer is
  the real feature; crush has session persistence but cannot resume
  mid-run or branch from a prior state. Cost: versioning every state
  write. Not worth it until resume-across-restart is a product
  requirement.
- **Interrupt/resume as a primitive.** `interrupt()` +
  `Command(resume=...)`. Crush approximates user-input interrupts
  with the queue + cancel marks — equivalent expressive power for
  users, none for programmatic pauses.
- **Generalized per-node model selection.** The summary-model slot
  is the precedent; an edge that wants a cheap classifier model
  would need its own binding. Defer until an edge actually asks.
- **An auditable transition table.** ~90% recoverable by logging
  edge firings per turn — which `EVAL_HARNESS.md` wants anyway as
  trajectory assertions. Named transitions give the corpus stable
  checkpoints to assert on; asserting on emergent loop behavior does
  not.

## Non-goals

- No DAG compilation of user tasks; no planner model that owns the
  loop.
- No mid-step control flow in the edge layer (hooks' job).
- No graph framework dependency — the pattern is what's being
  imported, not LangGraph (Python-only anyway).
- No mandatory planning — `PlanItem` must never make a one-line fix
  pay a planning turn.

## Token-cost framing

Edges spend tokens only on repair turns and only when a
deterministic trigger fired — bounded by the shared budget. The plan
artifact cuts re-derivation thrash (the model re-reading results to
reconstruct "where was I"). Fan-out keeps child verbosity out of the
parent context instead of stubbing it later. This is the reviewable
answer to "agent loops resend history every step": the loop still
replays, but what it replays is pruned, and where it goes next is
partly decided by code that costs nothing.

## PR ordering

1. Extract `runEdge` from `runVerificationGate` — pure refactor, the
   gate becomes the first two instances (verification, todos).
2. `PlanItem` schema + `todos`-tool migration + gate reads the typed
   list.
3. Stall-replan edge (loop-detector signal → trigger).
4. Join edge (requires `BACKGROUND_SUBAGENTS` completion seam +
   `SWARM_DEDUP` ledger).
5. Edge-firing records per turn for `EVAL_HARNESS`.
