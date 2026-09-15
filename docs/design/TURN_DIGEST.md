# Turn Digest — Fidelity Drop at the Turn Boundary

> **Status:** Spec. Splits context into two planes: the conversation
> (user messages, assistant answers, question/answer pairs — full
> continuity) and the execution transcript (tool calls + results —
> collapsed at turn end). Composes with `TOOL_RESULT_PRUNING.md`
> (the `prior_turn` predicate rides stub machinery), `SESSION_KNOWLEDGE.md`
> (the digest is a checkpoint-shaped entry at turn granularity),
> `CONTEXT_NOTEBOOK.md` (generator, selection, recall), and
> `EVAL_HARNESS.md` (three-arm paired experiment).
>
> **Depends on:** nothing new — every mechanism referenced below is
> shipped. Digest generation reuses the post-run pipeline slot that
> `SESSION_KNOWLEDGE.md` already reserves for run-end checkpoints.
>
> **Ship when:** `stub` mode after implementation; `digest` mode and
> any default change only behind the paired-eval gates below.

## Goal

Cap context growth **across** turns. Within-turn stubbing shrinks
stale results; it does nothing about the accumulated transcript of
every turn before the current one — which is the dominant context
mass in a long session. At each turn boundary the execution plane
drops a fidelity level: raw tool events stop rendering and a
checkpoint-shaped **turn digest** stands in their place.

```text
current turn      → full fidelity (calls + results, stubs as today)
completed turns   → collapsed: stub pairs + turn digest in notebook
closed session    → session checkpoint        (SESSION_KNOWLEDGE)
older sessions    → mem0 hydration            (SESSION_KNOWLEDGE)
```

## Problem

Two symptoms, one missing boundary:

1. **The transcript compounds.** In a 20-turn session the model
   re-reads (in the cache sense) every prior turn's tool calls and
   results on every request — even though the information that turn
   produced is either in the notebook, in the assistant's answer, or
   safely stored. Token cost per request grows with session age,
   independent of task difficulty.
2. **Nothing consolidates a finished turn.** The notebook logs
   events; the assistant's final text is written for the user, not
   for the model's next turn — it may be terse ("done") or omit
   which files were touched. A turn that ended in exploration
   leaves *no* usable trace once its events collapse.

## What exists

- **The render seam.** `preparePrompt` (`agent.go:1848`) is where
  stored events become rendered context; `promoteSupersededStubs`
  (`stubs.go:468`) already applies marks there. `prior_turn` is
  another mark applied at the same point.
- **Turn numbers are on the data.** Messages/entries carry
  `TurnNumber` (`db/models.go:41,63`); the predicate
  `event.TurnNumber < currentTurn` is a field read, not a heuristic.
- **The stub contract fits.** Invariant 6 — "stub beats drop" —
  extends cleanly: a collapsed pair keeps call/result structure and
  IDs (provider protocols require pairing) and names its recovery
  path (`recall("result:<tool_call_id>")`, `recall.go`).
- **The generator slot is reserved.** The post-run pipeline
  (`agent.go:1406+`) is where `SESSION_KNOWLEDGE.md` already plans
  run-end checkpoints; the small-model generator (`generator.go`)
  turns classified events into entry prose. A turn digest is the
  same generation over the finished turn's events.
- **Tri-state options are supported.** `optString`
  (`shellconfig/options.go:156`) — precedent: `option turn-context`.
- **Stored-vs-rendered split is established.** Stubs never rewrite
  `ToolResult.Content`; edit-safety scans (`pendingEdits`,
  read-before-write) and notebook coverage operate on stored
  events. Collapse inherits the same discipline.

## Design

### 1. `prior_turn` — the render predicate

At `preparePrompt`, every tool call/result pair whose `TurnNumber`
is below the current turn renders as a minimal stub pair:

```text
call:   [prior turn 3] edit(file_path=internal/agent/agent.go)
result: [prior turn 3 — consolidated in turn digest;
         recall("result:a91f") to recover this result]
```

- **Both sides collapse.** Call args are often the largest payload
  (edit `old_string`/`new_string` carry full content) and are stale
  write material once the turn ends — keeping them verbatim is a
  drift vector, not context.
- **Structure is preserved.** IDs, roles, and pairing survive; only
  content is replaced. One line per side.
- **Ordering:** `prior_turn` is evaluated first at render. Other
  stub kinds are irrelevant inside a collapsed turn — the flagging
  passes may keep marking stored events harmlessly, but render
  checks the turn boundary before consulting them.
- **Stub text is constant.** It does not reference whether the
  digest exists yet (see the async race below) — it always names
  `recall` as the recovery path and mentions the digest only as a
  fixed phrase. Text that changes with generation state would
  re-render and invalidate the prefix.

**What stays verbatim** — the conversation plane:

- User messages.
- Assistant text parts (the end-of-turn answer is itself a summary;
  collapsing it would break conversational continuity).
- `question` tool call/result pairs — they are user input in
  disguise and carry decisions the model must not lose.

### 2. The turn digest — checkpoint at `granularity: turn`

A notebook entry generated at run end, over the finished turn's
events:

```text
## Turn 4 digest — <topic>
Established:
- <claim> — evidence: file:internal/agent/agent.go:1401
Files touched:
- internal/agent/stubs.go (edited), eval/flags.json (read)
Open:
- <question> — blocked on / next probe
```

- **Same entry machinery.** `Entry` schema holds it unchanged; the
  `Established/Open` shape is the `checkpoint` contract from
  `SESSION_KNOWLEDGE.md` with `Files touched` added — a turn digest
  records *work done*, not just knowledge established.
- **Granularity is a tag, not a type.** `granularity:turn` on a
  checkpoint-shaped entry; the axis is `turn | boundary | session`.
- **Not self-pinned.** Boundary checkpoints pin because they are
  rare; a pinned entry per turn floods the working set. Turn
  digests are ordinary entries — selectable, compressible, eligible
  for the summary/tags tiers. If a digest compresses away, the
  stub's `recall` pointer still resolves (stored events are
  untouched), so no dead pointer is created.
- **Trigger:** run end, every run, when mode is `digest`. This
  absorbs `SESSION_KNOWLEDGE.md`'s run-end trigger ("if context was
  gathered and no checkpoint exists") — under digest mode the
  run-end checkpoint *is* the turn digest; under other modes the
  original conditional trigger stands.
- **Placement:** the notebook splice (`PrepareStep`), like every
  entry — selection, compaction, recall, auto-inject all apply
  unchanged. No new render path.

### 3. Flag: `notebook-prior-turns` (tri-state)

```text
option notebook-prior-turns verbatim   # today: full transcript (default)
option notebook-prior-turns stub       # collapse to labeled pairs, no digest
option notebook-prior-turns digest     # collapse + generated turn digest
```

- Three values map exactly onto the three eval arms — one knob, no
  mode pair to keep in sync.
- **Gated by the notebook.** Like `StubSuperseded`
  (`coordinator.go:789`): notebook disabled → coerced to `verbatim`,
  because `recall` is the recovery path and invariant 3 forbids dead
  pointers.
- **crushrc wiring is a known gap.** No `notebook-*` key exists in
  `optionSpecs` today (tracked in #38); `notebook-prior-turns` must
  be added to `optionSpecs` as `optString` — it does not inherit the
  gap, it is blocked by it until the entry lands.
- **Escape hatch:** set back to `verbatim`, restart not required —
  render-time means the next `PrepareStep` shows the full
  transcript again.

### 4. Safety invariants (preserved from TOOL_RESULT_PRUNING)

1. Stored events are never rewritten — collapse is render-time
   only.
2. `pendingEdits`, read-before-write, and filetracker operate on
   stored events; a collapsed turn still counts as "the session
   wrote this file."
3. No dead pointers: `stub`/`digest` modes never ship without
   `recall` available (notebook gate).
4. Size accounting uses post-collapse render size.
5. Segment coverage excludes digest entries — a digest is generated
   *from* coverage and must never feed back in
   (`classifyEvents` skips `granularity:turn` entries).

### 5. Edge cases

- **Async race.** The digest generates post-run; a fast next turn
  can render before it lands. Fine — stubs render regardless; the
  digest appears in the notebook when ready and the constant stub
  text means no re-render.
- **Aborted runs.** Their events belong to a completed turn next
  turn and collapse normally; the digest headline notes
  "interrupted" so a partial turn isn't read as finished work.
- **The dominant-turn case.** Exploration *and* execution inside
  one long turn (the token-drain scenario) is not helped by this
  doc — within-turn machinery (stubs, mid-turn checkpoint) owns
  that. Turn digests cap growth *between* turns; the two compose.
- **Headless/queued runs** collapse identically — the predicate is
  turn numbers, not UI state.

## Measurement

`EVAL_HARNESS` paired experiment, three arms on the multi-turn
corpus slice: `verbatim` / `stub` / `digest`.

| Metric | Question |
| ------ | -------- |
| Cost-normalized tokens-to-done | Does collapse actually pay, after generation cost? |
| Edit-failure rate | Does losing prior-turn write args cause phantom `old_string`s? |
| `result:` recall into prior turns | Is the digest losing needed content? |
| Re-read rate (files the digest claims were touched) | Does the model trust the digest or re-verify? |
| Verdict parity | Same task outcomes across arms? |

Ship gates, mirroring `TOOL_RESULT_PRUNING` acceptance criteria:

- `stub` mode: zero flip-flops, recall rate bounded, structure
  valid on all providers in the matrix.
- `digest` mode over `stub`: meaningful token win **and** flat
  re-read rate — if digests don't reduce re-reading vs. bare stubs,
  the per-run generation call isn't earning its tokens and `stub`
  is the mode to ship.
- Default change (if any) follows the #38 evidence process; the
  default stays `verbatim` until then.

## Non-goals

- **No selection of which turns to keep verbatim.** All-or-nothing
  per turn boundary. Graduated recency windows ("keep last two
  turns full") are a pressure-escalation question —
  `CONTEXT_WINDOW_SAFETY.md` owns it.
- **No mid-turn collapse.** Within-turn compression is stubs +
  boundary checkpoint (`SESSION_KNOWLEDGE.md`); the first-write
  checkpoint remains the in-flight consolidation and becomes the
  `phase-confirm` gate's review payload (`RUN_EDGES.md`).
- **No cross-session scope.** Session close and hydration stay in
  `SESSION_KNOWLEDGE.md`; turn digests are session-scoped inputs to
  the session checkpoint, which summarizes digests rather than raw
  events.
- **No model-chosen collapse.** The predicate is deterministic;
  letting the model exempt its own transcript is how pruning
  becomes optional.

## PR ordering

1. `prior_turn` predicate + `stub` mode + `optionSpecs` entry —
   pure render, no generation; smallest PR, exercises the
   collapse path and its invariants first.
2. Turn-digest generation + `digest` mode + `granularity:turn` tag
   + coverage exclusion.
3. Reconciliation with `SESSION_KNOWLEDGE.md`: run-end checkpoint
   trigger folds into the digest path when mode is `digest`.
4. Three-arm eval + ship gates; default stays `verbatim`.
5. Telemetry: turns collapsed, events collapsed, digest
   present-at-render rate (measures the async race), recall-into-
   prior-turns count in `stubStats`/`crush stats`.

## Risks

- **Edit-failure regression is the gate that matters.** Collapsing
  prior-turn write args removes the content that made a file
  "read"; the model must re-`view` before editing. Storage-side
  safety is intact, but rendered-context staleness is exactly what
  the edit-failure arm measures. If it moves, `stub`/`digest` don't
  ship — same standard as supersession.
- **Summary-of-summaries quality decay.** A session checkpoint over
  turn digests is a third compression pass. The `Established/Open/
  Files touched` shape with evidence handles is the mitigation —
  claims degrade but pointers don't; if eval shows digest detail
  loss, the session checkpoint can reach raw events on demand.
- **Per-run generation cost.** One small-model call per turn is the
  price of `digest` mode; the eval arm exists to prove it pays for
  itself. `stub` mode is the zero-cost fallback that still captures
  most of the savings.
