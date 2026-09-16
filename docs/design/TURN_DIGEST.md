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
> **Depends on:** no unshipped prerequisites — but this spec
> introduces new render-path code itself (the `ToolCallPart.Input`
> collapse, the coverage predicate, the selection demotion). Digest
> generation reuses the post-run pipeline slot that
> `SESSION_KNOWLEDGE.md` already reserves for run-end checkpoints.
>
> **Ship when:** `stub` mode after implementation; `digest` mode and
> any default change only behind the paired-eval gates below.

## Goal

Cap context growth **across** turns. Within-turn stubbing shrinks
stale results; it does nothing about prior turns' share of the
rendered window — covered segments inside the raw window render
raw **and** as notebook entries, the double render. At each turn
boundary the execution plane drops a fidelity level: raw tool
events stop rendering and a checkpoint-shaped **turn digest**
stands in their place.

```text
current turn      → full fidelity (calls + results, stubs as today)
completed turns   → collapsed: stub pairs + turn digest in notebook
closed session    → session checkpoint        (SESSION_KNOWLEDGE)
older sessions    → mem0 hydration            (SESSION_KNOWLEDGE)
```

## Problem

Two symptoms, one missing boundary:

1. **Prior turns double-render inside the raw window.** Prior turns
   beyond the raw-window boundary already don't render — the
   boundary caps them. What remains is the _double render_: covered
   prior-turn segments inside the ~25K raw window render raw **and**
   as notebook entries. Collapse dedupes the raw window's
   prior-turn share against the notebook — savings scale with
   raw-window composition, not session age.
2. **Nothing consolidates a finished turn.** The notebook logs
   events; the assistant's final text is written for the user, not
   for the model's next turn — it may be terse ("done") or omit
   which files were touched. A turn that ended in exploration
   leaves _no_ usable trace once its events collapse.

## What exists

- **The render seam.** `preparePrompt` (`agent.go:1848`) is where
  stored events become rendered context; `promoteSupersededStubs`
  (`stubs.go:441`) already applies marks there. `prior_turn` is
  another mark applied at the same point.
- **Turn numbers are derived, not stored — and can move mid-run.**
  `db.Message` has no `TurnNumber` (`db/models.go:41,63` are
  `NotebookEntry`/`ProcessedSegment`); message turns are computed
  positionally by `messageTurns` — cheap, already computed in
  `flagPrunableToolResults`. Consequence: `drainQueueForStep` folds
  a queued user prompt into the run mid-flight, advancing
  `currentTurn` _while the run executes_ — naive
  `turn < currentTurn` would collapse the active run's own earlier
  events. The predicate is `turn < turnThatStartedThisRun`, not
  `turn < currentTurn`.
- **The stub contract fits.** Invariant 6 — "stub beats drop" —
  extends cleanly: a collapsed pair keeps call/result structure and
  IDs (provider protocols require pairing) and names its recovery
  path (`recall("result:<tool_call_id>")`, `recall.go`).
- **The generator slot is reserved.** The post-run pipeline
  (`agent.go:1507`) is where `SESSION_KNOWLEDGE.md` already plans
  run-end checkpoints; the small-model generator (`generator.go`)
  turns classified events into entry prose. A turn digest is the
  same generation over the finished turn's events.
- **Tri-state options are supported.** `optString`
  (`shellconfig/options.go:156`) — precedent: `option turn-context`.
  The `notebook-*` `optionSpecs` keys exist post-#45, so
  `notebook-prior-turns` is a one-line map entry plus a new
  `Options` field (`notebook_prior_turns`, enum).
- **Stored-vs-rendered split is established.** Stubs never rewrite
  `ToolResult.Content`; edit-safety (`filetracker.LastReadTime`,
  populated at tool-execution time) and notebook coverage are
  independent of rendering. Collapse inherits the same discipline.

## Design

### 1. `prior_turn` — the render predicate

At `preparePrompt`, every tool call/result pair in a **fully
covered, completed** turn renders as a minimal stub pair:

```text
call:   {"_collapsed":"prior turn 3"}
result: [prior turn 3 — collapsed; recall("result:a91f") to recover]
```

(`digest`-mode result text may instead read "consolidated in turn
digest" — constant per mode, see below.)

- **The predicate is coverage, not age.** `turn < runStartTurn`
  **and** all of that turn's segments processed — segment generation lags (in-flight, backed-off, or
  burst-limited), and collapsing an uncovered turn produces stubs
  with no entries behind them: "drop with extra steps." Coverage is
  the _whole_ gate — the stub's recovery pointer resolves against
  stored events and needs no digest, so `digest` mode is exactly
  `stub` + generation; gating collapse on the digest would make
  generator latency drive collapse timing (async prefix churn). A
  persistent small-model outage degrades to `verbatim`-like
  behavior — safe direction.
- **Run-start turn, not current turn.** `drainQueueForStep` can fold
  a queued prompt mid-run, advancing `currentTurn` under the active
  run — the predicate uses the turn that _started_ the run so its
  own earlier events can't collapse mid-flight.
- **Both sides collapse — new machinery, wire constraint.** Call
  args are often the largest payload (edit `old_string`/
  `new_string` are stale write material once the turn ends), but
  `Superseded` marks live on `ToolResult` only — collapsing
  `ToolCallPart.Input` at render is a new code path
  (`ToAIMessage` emits stored input verbatim, `content.go:762`).
  And the collapsed input must stay **valid JSON** — providers
  require `tool_use.input` to be an object; a prose label fails
  late on Anthropic. The shape is minimal —
  `{"_collapsed":"prior turn 3"}` with no per-tool preview (the
  tool name already renders from `ToolCallPart.ToolName`, and the
  paired result carries the recall pointer).
- **Structure is preserved.** IDs, roles, and pairing survive; only
  content is replaced.
- **Ordering:** `prior_turn` is evaluated first at render. Other
  stub kinds are irrelevant inside a collapsed turn — flagging
  passes may keep marking stored events harmlessly, but render
  checks the turn boundary before consulting them.
- **Stub text is constant _per mode_.** `stub` mode never says
  "consolidated in turn digest" — no digest exists; mode-specific
  constant text costs nothing (mode flips re-render anyway). Never
  reflects digest availability — async state must not churn the
  prefix.

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
  records _work done_, not just knowledge established.
- **Granularity is a tag, not a type.** `granularity:turn` on a
  checkpoint-shaped entry; the axis is `turn | boundary | session`.
- **Coexists, with selection demotion — the duplicate-
  representation decision.** The turn still gets per-segment
  entries (coverage requires segment processing, and they're the
  substrate for `PinnedFileTags` — which only fires on
  `EventFileEdit` — plus per-segment `recall` granularity). A digest
  on top means the same work twice in the notebook. Rule: when a
  `granularity:turn` entry for turn N is selected, same-turn
  segment entries are skipped — applied to the candidate set, so
  it covers `maybeAutoInject` (`agent.go:2089`) too, which queries
  by `file:` tags independently of selection. (Replacing segment
  entries was the alternative; rejected: it loses file pins and
  per-segment recall.)
- **Not self-pinned.** Boundary checkpoints pin because they are
  rare; a pinned entry per turn floods the working set. Turn
  digests are ordinary entries — selectable, compressible, eligible
  for the summary/tags tiers. If a digest compresses away, the
  stub's `recall` pointer still resolves (stored events are
  untouched), so no dead pointer is created.
- **Trigger:** run end, when mode is `digest` **and** the turn
  produced ≥1 classified event — a ≥1-event floor keeps trivial
  turns ("thanks", one-question turns) from burning a small-model
  call for an empty digest. This absorbs
  `SESSION_KNOWLEDGE.md`'s run-end trigger ("if context was
  gathered and no checkpoint exists") — under digest mode the
  run-end checkpoint _is_ the turn digest; under other modes the
  original conditional trigger stands.
- **Generator input:** the turn's _classified events_, not its
  entries — so no exclusion filter is needed (entries never enter
  classification; if it ever generates over entries instead, filter
  `granularity:turn` there).
- **Placement:** the notebook splice (`PrepareStep`), like every
  entry — selection (with the demotion rule), compaction, recall,
  auto-inject. The only new render-path logic is the demotion skip.

### 3. Flag: `notebook-prior-turns` (tri-state)

```text
option notebook-prior-turns verbatim   # today: full transcript (default)
option notebook-prior-turns stub       # collapse to labeled pairs, no digest
option notebook-prior-turns digest     # collapse + generated turn digest
```

- Three values map exactly onto the three eval arms — one knob, no
  mode pair to keep in sync.
- **Gated by the notebook.** Like `StubSuperseded`
  (`coordinator.go:848`): notebook disabled → coerced to `verbatim`,
  because `recall` is the recovery path and invariant 3 forbids dead
  pointers.
- **crushrc wiring:** `optionSpecs` already carries the
  `notebook-*` keys post-#45; `notebook-prior-turns` is a one-line
  `optString` entry plus the `Options` field.
- **Escape hatch:** set back to `verbatim` — takes effect on the
  next agent build (options are captured at agent construction,
  `coordinator.go:848`), after which the next `PrepareStep` shows
  the full transcript again.

### 4. Safety invariants (preserved from TOOL_RESULT_PRUNING)

1. Stored events are never rewritten — collapse is render-time
   only.
2. Read-before-write holds — `filetracker.LastReadTime`
   (`edit.go:292`) is populated at tool-execution time and never
   scans stored message events, so the check is independent of
   rendering entirely; collapse can't weaken it.
3. No dead pointers: `stub`/`digest` modes never ship without
   `recall` available (notebook gate).
4. Size accounting uses **pre-collapse** sizing:
   `findSegmentBoundaryByTokenBudget` keeps measuring stored
   content, so the window boundary doesn't move — collapse is a
   pure within-window transform and the savings are real, not
   reinvested in pulling more prior turns in as stubs.
5. Digests never feed back: the generator reads classified _events_
   (which can't contain entries — `classifyEvents` runs over
   messages, so no skip is needed there), and coverage gating counts
   only segment processing, never digest presence.

### 5. Edge cases

- **Async race.** The digest generates post-run; a fast next turn
  can render before it lands. Fine — stubs render regardless; the
  digest appears in the notebook when ready and the constant stub
  text means no re-render.
- **Aborted runs.** Their events belong to a completed turn next
  turn and collapse normally; the digest headline notes
  "interrupted" so a partial turn isn't read as finished work.
- **The dominant-turn case.** Exploration _and_ execution inside
  one long turn (the token-drain scenario) is not helped by this
  doc — within-turn machinery (stubs, mid-turn checkpoint) owns
  that. Turn digests cap growth _between_ turns; the two compose.
- **Headless/queued runs** collapse identically — the predicate is
  turn numbers, not UI state. Queued prompts folded mid-run by
  `drainQueueForStep` can't trigger collapse of the active run's
  events: the predicate compares against the run-start turn, not
  the (advancing) current turn.
- **Reasoning parts drop with the turn.** Providers require
  thinking signatures only within the in-flight tool-use turn
  (Anthropic strips prior-turn thinking anyway), so prior-turn
  `Thinking` collapses with everything else — often the largest
  prior-turn payload on extended-thinking sessions. The fragile
  combo to avoid is _keeping_ a signature while mutating the
  `tool_use.input` it signed (Gemini `ThoughtSignature` binds to
  `ToolID`, `content.go:750`) — collapse removes both together.
  Provider matrix must include a signed-thinking model; if one
  rejects it, that provider falls back to keeping prior-turn
  reasoning and inputs verbatim.
- **Turn N−1 concentrates the risk.** The all-or-nothing boundary
  collapses the _most recently completed_ turn — the one a
  follow-up most likely references. The edit-failure eval arm is
  the gate; graduated recency windows stay deferred to
  `CONTEXT_WINDOW_SAFETY.md`.
- **`Summarize` sees the collapsed view.** It calls `preparePrompt`
  (`agent.go:1672`), so post-implementation the summarizer's input
  is stubs + notebook — accepted, since digests/entries carry the
  information; if summary quality regresses, `Summarize` can render
  verbatim.
- **`runStartTurn` plumbing.** `preparePrompt` has three call
  sites (run :969, summarize :1672, shared
  `notebook_segments.go:875`); capture the run-start turn once at
  run start and thread it through — for the summarize call site all
  completed turns are prior turns.

## Measurement

`EVAL_HARNESS` paired experiment, three arms on the multi-turn
corpus slice: `verbatim` / `stub` / `digest`.

| Metric                                              | Question                                                       |
| --------------------------------------------------- | -------------------------------------------------------------- |
| Cost-normalized tokens-to-done                      | Does collapse actually pay, after generation cost?             |
| Edit-failure rate                                   | Does losing prior-turn write args cause phantom `old_string`s? |
| `result:` recall into prior turns                   | Is the digest losing needed content?                           |
| Re-read rate (files the digest claims were touched) | Does the model trust the digest or re-verify?                  |
| Verdict parity                                      | Same task outcomes across arms?                                |

Ship gates, mirroring `TOOL_RESULT_PRUNING` acceptance criteria:

- `stub` mode: zero flip-flops, recall rate bounded, structure
  valid on all providers in the matrix — explicitly including a
  signed-thinking model (thought signatures on collapsed inputs).
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
  boundary checkpoint (`SESSION_KNOWLEDGE.md`); the checkpoint is
  the review payload for **post-checkpoint** gates — the
  first-boundary `phase-confirm` resolves pre-write, before its
  own checkpoint can exist.
- **No cross-session scope.** Session close and hydration stay in
  `SESSION_KNOWLEDGE.md`; turn digests are session-scoped inputs to
  the session checkpoint, which summarizes digests rather than raw
  events — and it's the _checkpoint's_ sync that touches mem0; the
  `working_dir` partition (shipped in #56) keeps consolidated
  digests from bleeding cross-project downstream.
- **No model-chosen collapse.** The predicate is deterministic;
  letting the model exempt its own transcript is how pruning
  becomes optional.

## PR ordering

1. `prior_turn` predicate + `stub` mode + `optionSpecs` entry —
   pure render, no generation, but not tiny: the predicate is
   `turn < runStartTurn` **and** turn fully covered, and call-side
   collapse needs the `ToolCallPart.Input` render path with the
   `{"_collapsed": ...}` JSON constraint. Exercises the collapse
   path and its invariants first.
2. Turn-digest generation + `digest` mode + `granularity:turn` tag
   - the same-turn demotion rule; generator reads the turn's
     classified events.
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
  turn digests is a third compression pass. The
  `Established/Open/Files touched` shape with evidence handles is
  the mitigation —
  claims degrade but pointers don't; if eval shows digest detail
  loss, the session checkpoint can reach raw events on demand.
- **Per-run generation cost.** One small-model call per turn is the
  price of `digest` mode; the eval arm exists to prove it pays for
  itself. `stub` mode is the zero-cost fallback that still captures
  most of the savings.
