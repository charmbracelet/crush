# Run Edges — Deterministic Transitions at the Run Boundary

> **Status:** Partially shipped. The `runEdge` seam, verification,
> todos-reconcile, `escalate-human`, and `phase-confirm` landed via
> #43 — this doc's remaining work is the unimplemented catalog rows
> (stall-replan, burn-watch, summarize-continue, join-subagents) and
> edge-firing records. Split from `HARNESS_TOPOLOGY.md` — that doc
> is the analysis of why this shape; this doc is the work.
> **Ship when:** per edge — the catalog names each trigger.
> **Measured by:** edge-firing records per turn (PR 3 below);
> `EVAL_HARNESS` trajectory assertions on named transitions.

## Goal

Make every deterministic transition in the agent loop a declared
edge — named, budgeted, aggregated — instead of hand-rolled
pipeline code at one hardcoded site. New transitions become
declarations on the seam; the loop kernel stays the router for
everything the edges don't claim.

## The edge type

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

Evaluated in sequence at the `agent.go:1464` site (`runEdges` —
the seam itself shipped in #43; this doc's remaining work is the
unimplemented catalog rows). Aggregation rule,
already precedented: all firing edges merge evidence into **one**
retry prompt, one prepend, one budget increment — two edges each
enqueueing a turn would double every repair. Budget becomes a shared
repair counter on `SessionAgentCall` (rename `VerificationAttempts`
→ `RepairAttempts`; per-edge sub-budgets only if a thrash pattern
demands it). `maxVerificationAttempts = 2` (`verify_gate.go:27`)
stays the initial shared bound.

**Field traps are inherited, not optional.** Every edge obeys the
constraints listed in `HARNESS_TOPOLOGY.md` — run-boundary only,
RunID fold exemption, budgets on `SessionAgentCall`,
flush-before-notebook, union stored metadata. They are not restated
here because the list must have exactly one home.

## Catalog

| Edge               | Trigger                                            | Status today                          |
| ------------------ | -------------------------------------------------- | ------------------------------------- |
| verification       | failed/pending checks                              | implemented (the extraction source)   |
| todos-reconcile    | open plan items at clean stop                      | implemented (same site)               |
| stall-replan       | loop-detector / no-progress                        | new — repurposes the stop-only signal |
| escalate-human     | loop-detector stop, or repair budget spent         | implemented (#43) — question turn     |
| phase-confirm      | first write-class call after ≥N exploration events | implemented (#43) — plan gate         |
| join-subagents     | outstanding dispatch ledger                        | lives in `BACKGROUND_SUBAGENTS.md`    |
| summarize-continue | context pressure at run end                        | new — reframes auto-summarize         |
| burn-watch         | run spent >T tokens with zero write-class calls    | new — the unnoticed-spend tripwire    |

The stall-replan row is the tell that this abstraction earns its
keep: `hasRepeatedToolCalls` today _stops_ a thrashing turn — the
information is right and the action is wrong. As an edge it becomes
"evidence of no progress → one replan turn within budget."

**Edges can target the user, not just the model.** Two rows —
`escalate-human`, `phase-confirm` — resolve into a `question`-tool
turn instead of a retry prompt: the trigger and evidence collection
stay deterministic, but the boundary slot is spent asking the human
rather than prepending for the model. This is the human-in-the-loop
half of the calibrated-autonomy work (#39): the harness decides
_when_ to ask (loop-stop with no progress, first-write boundary on
a large-scope plan, repair budget spent), the prompt decides how.
Escalation frequency is budgeted like repairs — one per distinct
blocker — because an unbudgeted escalation edge is nagging with a
type signature. Headless (`crush run`) degrades both rows to
"state the blocker + chosen option, proceed": the edge still fires,
the question becomes a logged assumption.

`escalate-human` ships in the same series as #39's clause work, with
the `runEdge` extraction folded in as its enabling step — the seam
lands first within the series, the edge second. The ordering is not
scheduling convenience — it is the anti-accretion argument applied
to itself. There is exactly one hardcoded edge site today
(`runVerificationGate`); adding a second hand-rolled transition
beside it — its own trigger scan, budget accounting, clone-call
enqueue, RunID handling — would be the accretion pattern the edge
type exists to prevent. Once the seam exists, the edge is a
declaration, not a bolt-on. `phase-confirm` needs no such
predecessor: its trigger is not a run-boundary transition at all but
a mid-run tool-call gate at the first `writeToolNames` call — a
different seam (permissions/hook territory), unblocked today.

With the project index (`CONTEXT_PREFETCH.md`, #34), `resolve`
gains a deterministic evidence source: a repair prompt can carry a
`map` slice over the run's touched dirs (filetracker knows the write
set), so the forced turn starts re-oriented at zero model cost
instead of the model re-gathering in its first calls back — the
re-read loop the index exists to kill, closed at the run boundary
rather than left to the model remembering a tool exists.

Mid-step control is explicitly out of scope for edges — hooks and
permissions own that boundary (`hooked_tool.go`); edges only ever
fire between turns.

## Edge specs — the unimplemented rows

### stall-replan

Repurposes the stop-only loop-detector signal into a budgeted
replan turn.

- **scan:** the run ended via `hasRepeatedToolCalls` (or carries
  repeated-call evidence in `result.Steps`) and the repair budget
  is not spent.
- **resolve:** collect the repeated call signature, the filetracker
  write set, and — with `project_index` on — a `map` slice over
  the touched dirs.
- **prompt:** "you stopped making progress: <evidence>. State which
  assumption failed and revise the approach." One turn within
  `RepairAttempts`.
- **Precedence vs. `escalate-human`:** both can trigger on a loop
  stop. Resolution order — `stall-replan` is the in-budget
  response; `escalate-human` is what a spent budget falls through
  to. The aggregation rule merges them if both fire: one boundary
  slot, escalation wins the target (a question to the user
  subsumes a retry prompt).

### burn-watch

The spend tripwire the loop detector can't provide: it catches
repeated calls; burn-watch catches _monotone progress that never
produces a write_ — 40 steps, 2M tokens, zero edits, run ends
"cleanly" and nobody noticed. This is the "60M tokens and I didn't
notice" failure as a declared transition.

- **scan:** the finished run consumed >T tokens (or >S steps) and
  produced zero `writeToolNames` calls. T/S are generous tripwire
  thresholds, not a governor — the edge exists to surface, not to
  throttle. Configurable; default on the order of ~200K tokens or
  ~30 steps.
- **resolve:** collect the evidence — steps, tokens, exploration
  event count, files-read-without-write (filetracker).
- **prompt:** escalation-family — a `question` turn ("spent N
  tokens over M steps with no writes — continue / replan /
  stop?"), not a retry prompt. Headless degrades to a logged
  assumption per the shared rule.
- **Fires once per crossing, resets on write.** A marker on
  `SessionAgentCall` suppresses re-firing until a write-class call
  lands — otherwise it nags at every run end forever.
- **Precedence:** shares the escalation path — if `stall-replan`
  or `escalate-human` also fired, the aggregation rule merges them
  and the user-targeted prompt wins the slot.
- **Limit, stated plainly:** edges fire at run boundaries only —
  a single giant turn mid-flight is not caught. Mid-run spend
  pressure is `CONTEXT_WINDOW_SAFETY.md` territory (or a future
  hook), not this edge's job.

### summarize-continue

Reframes auto-summarize as an edge: context pressure at the run
boundary triggers a continue-after-summary turn instead of a
mid-loop condense. Deferred — the interaction with `StopWhen`'s
existing auto-summarize path needs designing first (who owns the
threshold; does the edge replace or wrap it). Not before the other
rows prove the type.

### edge-firing records

Log every edge firing per turn — name, trigger, outcome — into the
run record. This is what `EVAL_HARNESS` consumes as trajectory
assertions: named transitions give the corpus stable checkpoints,
and asserting on emergent loop behavior does not. Also surfaced in
`crush stats` — an edge that fires constantly is a tuning signal,
an edge that never fires is dead code.

## PR ordering

1. Extract `runEdge` from `runVerificationGate` — pure refactor;
   verification + todos become the first two instances. Ships
   inside #39's series — `escalate-human` needs this seam.
2. `stall-replan` edge — same trigger family as `escalate-human`;
   implement the precedence rule above.
3. Edge-firing records per turn for `EVAL_HARNESS` + `crush stats`.
4. `burn-watch` — threshold + once-per-crossing marker; lands after
   records exist so its firing rate is measurable from day one.
5. `summarize-continue` — only after the `StopWhen` interaction is
   designed.

(The join edge is not an item here — it lives in
`BACKGROUND_SUBAGENTS.md` with its dependencies.)
