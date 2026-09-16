# Session Knowledge — Consolidated Checkpoints & Cold-Start Hydration

> **Status:** Partially shipped. PR 1 landed in #56 (`working_dir`
> partition, capability-detected server-side filters, fail-closed
> client verification); §1's within-session `checkpoint` entry type
> lands via #48. §2 (cold-start hydration) remains spec — tracked in
> #53. Covers two halves of one gap — the notebook is an
> event log with no consolidated position (within-session), and its
> mem0 sync is write-only in practice because retrieval is pull-only
> (cross-session). Proposes a `checkpoint` entry type plus session
> hydration by notebook seeding. Composes with `PLAN_ARTIFACT.md`
> (the plan artifact's `Evidence` handles), `CONTEXT_NOTEBOOK.md`
> (entry machinery), `NOTEBOOK_QUALITY.md` (selection/sufficiency),
> `TURN_DIGEST.md` (the same checkpoint-shaped entry at turn
> granularity — turn digests are the session checkpoint's input),
> and `EVAL_HARNESS.md` (cold-start is a measurable arm).

## Goal

Give the session a consolidated position — "what is established,
with evidence" vs. "what is still open" — so the model can check
instead of re-reading, and make cross-session knowledge reachable
without the model knowing to ask: the notebook gets seeded on
cold start rather than waiting on a pull that never comes.

## Problem

Long context-gathering tasks expose two faces of the same missing
artifact:

1. **No checkpoint = no confidence to start.** During a long
   investigation the notebook accumulates `file_read`,
   `exploration`, and `decision` entries — an event log, not a
   position. Nothing ever consolidates "what is established, with
   evidence" from "what is still open." The model's "do I need to
   re-read this?" decision is therefore unassisted: it cannot
   distinguish _already known_ from _never checked_, and either
   re-reads (token cost, compounding per step) or proceeds on vibes.
   Sufficiency telemetry (`nbStats`, `agent.go:235`) counts recalls
   after the fact; it does not give the model the artifact that
   would make the recall unnecessary.
2. **New session = cold start from zero.** Entries are
   session-scoped — every `GetEntries`/`SearchByTag`/
   `SearchByText`/`GetByTurn` takes `sessionID` (`retrieve.go:15-70`).
   mem0 sync pushes every entry out (`internal/notebook/mem0.go:84`),
   but the only way back in is `recall cross:` (`recall.go:83-89`) — a
   model-initiated pull. A fresh session has an empty notebook and a
   model that does not know what it does not know, so the query is
   never issued. The knowledge was persisted; the hydration path was
   never built.

## What exists

- **Push-side retrieval already exists — session-scoped.**
  `maybeAutoInject` (`agent.go:2089`) matches file references in the
  user message against notebook entries and splices full entries
  into context without a recall call. The cross-session version is
  the same mechanism pointed at a wider store — the pattern is
  proven; the scope is the gap.
- **Entry schema is already the right shape for a checkpoint.**
  `Entry` (`notebook.go:43-77`): `Title`, `EntryText` +
  `EntryTextFull`, `Tags`, `Succeeded`, `ErrorHeadline`, `Verified`,
  `CompressionLevel`, `(Turn, Segment, Event)` addressing. Event
  types are an open string set — `file_read`, `file_edit`,
  `command`, `decision`, `exploration`, `general`
  (`notebook.go:19-27`) — so `checkpoint` is a constant, not a
  migration. Compression tiers exist (full ≤1K tokens → summary ~100
  → tags ~20, `notebook.go:29-34`) and pins exempt entries from
  compression (`pinnedEntryIDs`, `retrieve.go:310`) — the machinery
  a checkpoint's self-pin will hang off, though today's pin is
  file-tag-driven so a type-driven exemption is new code (§1).
- **The investigation→execution boundary is observable for free.**
  `tools.WriteToolNames` (`internal/agent/tools/toolclass.go:9`) is a
  deterministic classifier;
  the first successful write-class call after a run of
  exploration-class events is a detectable transition — no model
  judgment needed to know _when_ to consolidate.
- **Generation plumbing is reusable.** The small-model generator
  (`generator.go`) already turns classified events into entry prose;
  a checkpoint is a generation over the run's entries (a summary of
  summaries), not over raw events.
- **Injection seam for hydration — with a carve-out.** Notebook
  content is rebuilt/spliced per step at `PrepareStep`
  (`agent.go:1004`) via `rebuildStepMessages` (`agent.go:1051`,
  `notebook_segments.go:864`). Entries seeded into a new session's
  notebook ride that machinery for selection, compaction, recall,
  and auto-inject — **but not for rendering**: `notebookPrefix`
  returns nil when `boundary <= 0` (always true on turn 1 of a fresh
  session — `notebook_segments.go:827`), and `renderNotebookPrefix`
  filters to coverage-positioned keys (`notebook_segments.go:1017`).
  Seeds need a defined render carve-out — see Hydration below; the
  "zero new prompt plumbing" claim is qualified to "one new render
  carve-out, everything else rides."
- **Cache-class machinery for the alternative.** `prompt/sections.go`
  defines `CacheClassSession` (stable within a session, differs
  across) — the right class if hydration lands in the system prompt
  instead of the notebook. See the seam decision below.
- **mem0 metadata is extensible — and now partitioned.**
  `SyncEntries` sends `{session_id, turn_number, event_number,
event_type, tags, compression, working_dir}`
  (`internal/notebook/mem0.go:112`) — `working_dir` normalized by
  `canonicalizeWorkingDir` (:226). Adding keys is additive; old
  memories simply lack them (and are excluded fail-closed).

### Field traps

- **`agent_id: "crush"` was a global namespace — fixed (#56).**
  `SearchMem0`/`SyncEntries` used to key on the bare agent ID with
  no project partition; a `cross:` search in repo A could surface
  repo B's memories. The `working_dir` partition now lands
  _before_ hydration — which is what mattered, since hydration
  would have made the bleed _automatic_ instead of
  model-initiated.
- **Evidence handles die — or worse, resolve wrong — at the session
  boundary.** An entry's `result:<tool_call_id>` recall resolves
  against the current session's stored messages and dies cleanly.
  But `turn:5`/`segment:5.2` are _live_ recall queries that silently
  resolve against the new session's unrelated turn 5 — worse than a
  dead handle. Hydrated entries must strip or annotate **all**
  recallable prefixes (`result:`, `turn:`, `segment:`) — keep them
  as provenance text, never as resolvable handles.
- **Hydrated entries must not re-sync.** Seed entries written to a
  new session's notebook would flow back through `SyncEntries` and
  duplicate the memories they came from — one copy per session,
  compounding forever. Provenance tag → `SyncEntries` skips them.
- **A hydration digest is stale at injection time.** Knowledge
  injected on turn 1 describes the _last_ session's world; files may
  have changed since. `RenderEntries` does not emit `CreatedAt`, so
  the origin date must live **in the entry text** — a hard
  requirement, not a nicety: a checkpoint that can't date itself
  will be trusted past its validity.
- **Seed keying poisons coverage if it uses real turn numbers.**
  `backfillSegmentRegistry` (`notebook_segments.go:589`) marks
  segments of turns that already have entries as processed — a seed
  keyed to a real turn makes that turn's segments look covered and
  their events **never generate entries**. Seeds get sentinel keys
  below any real turn (e.g. `TurnNumber = -1`): no real segments
  exist there for backfill to mark, the keys can't supersede live
  reads (older than everything), and seeds rank oldest in selection
  — natural decay instead of a hard quota. The `hydrated` tag check
  in `SyncEntries` stays as belt-and-suspenders regardless of
  keying.
- **Origin turn numbers are harmful too.** Keys carried over from
  the source session (e.g. seed on turn 37 in a session at turn 0)
  rank as maximally recent — they win selection bands and
  `PinnedFileTags` ahead of live entries, and `dropSupersededReads`
  would drop a _live_ turn-0 read as superseded by a stale seed —
  inverted staleness. Sentinel keys avoid both failure modes.
- **Checkpoint generation must not eat its own output — but the
  filter is by granularity, not type.** The feedback risk is on the
  generator _input_ side — checkpoints are entries, never messages,
  so `classifyEvents` can't consume them; but a checkpoint
  generated over "the run's entries" would read prior checkpoints.
  The filter excludes checkpoint entries of the checkpoint's **own
  granularity or coarser** — `granularity:turn` digests stay
  legitimate input, since the session checkpoint is defined as
  summarizing digests (`TURN_DIGEST.md`); a blanket
  `event_type=checkpoint` filter would remove exactly that input.

## Design

### 1. `checkpoint` event type — the consolidated position

A checkpoint entry consolidates the run's investigation into two
lists:

```text
## Checkpoint — <topic>
Established:
- <claim> — evidence: file:internal/agent/agent.go:1401, e4.2
- <claim> — evidence: result:a91f (this-session handle)
Open:
- <question> — blocked on / next probe
```

- **Generation:** small-model call over **all committed session
  entries** — finer-granularity turn digests included — plus
  classified events for the uncovered tail (summary-of-summaries;
  the raw events are already compressed once, the second pass is
  cheap). Input is _cumulative_, not run-scoped: same-granularity
  checkpoints are excluded, so run-scoped input would leave
  checkpoint N blind to runs 1..N−1 — silently voiding the
  hydration contract ("latest boundary checkpoint = session
  position"). Bound newest-first if size becomes a concern.
- **Triggers, deterministic:**
  - First successful write-class call in a run containing ≥N
    exploration events with no checkpoint **this run** — the
    investigation→execution boundary. **The boundary already has a
    detector: `scopeGate`** (`scope_gate.go` — `isMutatingCall`
    after `scopeGateMinExploration = 8` non-mutating calls). Reuse
    its vocabulary — `isMutatingCall` is a superset of
    `WriteToolNames` (catches `git commit`, `sed -i`, download);
    one boundary, one definition. But **don't hang the checkpoint
    off the gate's wrap path** — it's option-gated
    (`AmbiguityClarificationEnabled && !isSubAgent`,
    `coordinator.go:1089`), so checkpoints would go dark for users
    without the flag. Detect at the per-step rebuild instead: scan
    the run's messages for the first non-error write-class result.
  - **N's units force the vocabulary extraction.** N counts
    classified non-trivial non-mutating _events_ — same units as
    the generator input — and `classifyEvents` is unexported in
    `package notebook`, so the N-check lives inside
    `GenerateCheckpoint`. `EntryInput.EventType` can't substitute
    for the mutating vocabulary (`EventCommand` covers both
    `git commit` and read-only bash; `download` is `EventGeneral`),
    so extract `tools.IsMutatingCall(name, input string)` into
    `internal/agent/tools` — a leaf `notebook` already partially
    imports (`tools/mcp`), the `(name, input)` signature needs no
    fantasy dependency and dissolves the `fantasy.ToolCall` vs
    `message.ToolCall` adapter question — and re-point
    `scope_gate.go` at it. Trivial events don't count toward N;
    this consciously diverges from the gate's raw call count
    (`explore` increments per call; `classifyEvents` skips
    unfinished calls and buckets trivial ones).
  - **Cheap agent-side pre-scan first.** The per-step detector
    checks "first non-error write result present + no checkpoint
    this run" _before_ invoking `GenerateCheckpoint` — otherwise
    every step pays `GetEntries` + `classifyEvents` forever. The
    call→result join can reuse the `resultPos` indexing pattern in
    `segmentBoundaries` (`notebook_segments.go:136`). Note
    `RunStamp` exists only under `genCtx` → `PrepareStep`'s
    `callContext` — detection silently no-ops on the `preparePrompt`
    run-start (:969) and summarize (:1672) paths. That's correct
    behavior; stating it so nobody "fixes" it.
  - Run end, if context was gathered and no checkpoint exists
    (slot: post-run pipeline, `generateRunEndSegments` at
    `agent.go:1507`, same machinery as segment entries).
  - Model-initiated pin via a parameter on the existing `todos`-class
    tool surface — optional, only if the deterministic triggers miss
    in practice.
- **Trigger timing vs. async inputs.** The write-boundary trigger
  fires mid-run but the checkpoint's input lags: the exploration
  segment may still be open and closed-segment generation runs on
  detached goroutines (`tracker.inflight`). **One input rule for
  both triggers:** committed entries plus classified events for the
  uncovered tail — there is no join primitive on `inflight`, so the
  run-end trigger can't wait on the tail segment's commits either.
  Cleanest shape: a `GenerateCheckpoint(ctx, sessionID, msgs)`
  service method doing `GetEntries` + `classifyEvents` internally
  (it's unexported — this keeps the granularity-aware input filter
  next to the data). The mid-run value is real — a checkpoint can
  render mid-run once the boundary passes its key — so the post-run
  fallback forfeits exactly the dominant case (the long single
  turn); it's a fallback, not the feature.
- **Trigger dedup — the full protocol.** Mid-run detection and the
  post-run pass can both observe "no checkpoint" and
  double-generate — a DB read isn't atomic. Per-run inflight claim
  (`markInflight` precedent), **clear on failure** (so the run-end
  trigger can retry — the desired fallback), persist on success,
  plus a post-run DB existence check. Scope by **run**, not turn —
  `RunStamp` is available via `genCtx`/`callContext`
  (`agent.go:148,818`); turn-scoping double-fires on fold-heavy
  runs (a folded prompt starts a new turn inside one run). For the
  existence check, run-start turn isn't plumbed to
  `rebuildStepMessages` — simplest self-contained form: tag the
  entry `run:<stamp>` and `SearchByTag` it (alternative: the
  tracker captures the last turn on stamp rollover — cheaper per
  lookup, more plumbing). Per-session scoping would mean only the
  first investigation ever checkpoints and latest-only-pin is dead
  code.
- **The phase-confirm payload claim, scoped.** The gate intercepts
  the mutating call _before_ it executes (`scopeGateTool.Run`,
  `scope_gate.go:272`); the trigger detects the first non-error
  write _result_ — strictly after the gate resolves. So the
  first-boundary confirm can never carry its own checkpoint. The
  checkpoint is the review payload for **post-checkpoint** gates —
  `stall-replan`, subsequent confirms — not the boundary that
  generates it. Optional composition: the gate's detection path can
  kick `GenerateCheckpoint` asynchronously at intercept, so it
  lands during the user's decision wait — never synchronously in
  the user-waiting path. Wiring note: `scopeGate` holds only
  `question.Service`, so the kick needs a callback plumbed through
  `newScopeGate` from the coordinator; it's purely a latency
  optimization — `Ask()` is already built by then, so it can't be
  the current question's payload anyway.
- **Mid-run render is boundary-gated.** "Renders mid-run" means
  _once the boundary passes its key_ — if the whole investigation
  fits the ~25K raw window (entirely possible past N exploration
  calls), `boundary <= 0` and nothing renders mid-run at all.
  Inherent to the notebook design, not a flaw — but the acceptance
  criterion is "renders once the boundary passes its key." Related
  edge: keying to a last-closed segment whose coverage is still
  in-flight can leave the checkpoint behind a pulled-back boundary
  — transient; worth a test.
- **Compression policy:** checkpoints pin **by type at
  boundary/session granularity, fully exempt from `compressEntry`**
  — a `CompressionLevel` floor of Summary is not enough, since
  compaction to first-sentence + tags destroys the
  `Established:`/`Open:` lists. Today's pin (`pinnedEntryIDs`) is
  file-tag-driven; a type-driven exemption is new code. Only the
  **latest** checkpoint per granularity pins — superseded
  checkpoints decay, or they pile up forever. **`granularity:turn`
  digests are excluded from both the pin and the exemption** — a
  pinned entry per turn floods the working set; they stay ordinary
  compressible entries (`TURN_DIGEST.md`).
- **Entry keying:** `(lastClosedSeg.turn, lastClosedSeg.number,
next event number under the segment write lock)` — the
  _session's_ last closed segment, possibly a prior turn's. An
  _open_-segment key never renders mid-run (the boundary can't
  pass it), and a short investigation inside one open segment has
  no current-turn closed segment at all — keyed to the session's
  last closed segment it renders as soon as the boundary passes.
  Deliberate cost: `TurnNumber` then belongs to the earlier turn,
  so `turn:` recall and `## Turn X.Y` headers label it there.
  The write-lock allocation also prevents `(turn, segment, event)`
  collisions with concurrent `GenerateSegmentEntries` commits —
  no unique constraint, plain indexes only.
- **Supersession exclusion.** Checkpoints carry `file:` tags (refs,
  auto-inject, working-set promotion all key on them) — but a
  checkpoint is not an observation of file state, so it must not
  supersede the reads it cites. Exclude `checkpoint` from the
  _superseder_ side of `newestForFile`/`dropSupersededReads`
  (`notebook_selection.go:463`) — otherwise the checkpoint drops
  its own evidence.
- **Coverage exclusion.** `backfillSegmentRegistry`
  (`notebook_segments.go:589`) marks every segment of a turn that
  has entries as processed — and it _retries on failure_. A
  mid-run checkpoint committed to a real turn while the registry
  is still empty (a short investigation inside one open segment)
  plus a retried backfill → the checkpoint's own turn reads as
  covered → its real events never generate entries.
  `TurnsWithEntries`/backfill must ignore checkpoint-type entries
  — same reasoning as hydration's sentinel keys. The exclusion
  wants the filter in SQL (`GetNotebookTurnsWithEntries ... WHERE
event_type != 'checkpoint'`) — an sqlc query change, not just Go.
- **Selection rank:** `checkpoint` must not take the unknown-type
  default — `entryTypeRank` (`notebook_selection.go:33`) returns 0
  for unknown types, the "least predictable" bucket that fills last.
  The signature takes only `eventType` — granularity lives in tags,
  so rank must widen to see the entry (or its tags); ripples into
  `typeThenRecency`/`selectNotebookEntries`. Give **boundary/session
  granularity** explicit top rank — the anchor, not filler — and
  apply **latest-only to the rank too**, symmetric with the pin:
  otherwise every stale boundary checkpoint ever written outranks
  all newer turns' `file_edit`/`decision` entries beyond the
  recency band, and they accumulate. `granularity:turn` digests get
  an explicit _mid_ rank instead (above raw reads, below boundary
  checkpoints) — a per-turn artifact outranking every other turn's
  `file_edit` would be too large a promotion; the demotion rule
  still applies whenever a digest is selected.
- **Recall dispatch:** add `checkpoint` to `recall`'s event-type
  dispatch (`recall.go:208`) — today it would fall through to text
  search (one case covers `notebook_search` too, which shares
  `searchNotebook`). Also update `RecallParams.Query`'s
  description and `recall.md` — a model that never sees
  "checkpoint" listed won't issue the query.
- **Prompt surface:** one line in `coder.md.tpl` — before re-reading
  a file to re-derive a fact, consult the latest checkpoint. The
  artifact does the work; the prompt just points at it.
- **Coverage:** excluded via the generator input filter above —
  checkpoints are entries, not messages, so segment coverage never
  sees them anyway.
- **Sync is explicit.** The only `SyncEntries` call site is inside
  `generateSegment` (`notebook_segments.go:633`) — a checkpoint
  written outside that path needs its own sync call.
- **Granularity:session has no producer yet.** The session's
  position is its _latest boundary checkpoint_ — that's what
  hydration fetches. `granularity:session` stays reserved; if a
  session-close generator lands later it fills the slot.
- **Sizing headroom.** `MaxEntryTokens` (1000) truncation applies
  via `truncateEntry` — the Established/Open shape over a long run
  may want more; measure before raising.
- **Auto-inject has no rescue for the uncompressed.** `maybeAutoInject`
  only injects `CompressionLevel > 0` entries (`agent.go:2140`) —
  on the assumption uncompressed ⇒ already rendered. A fully-exempt
  checkpoint evicted by the render cap has no rescue path; top rank
  makes eviction unlikely, but the invariant is now load-bearing
  for the anchor entry — if it ever bites, rescue by pin-set, not
  compression level.
- **Metric distortion.** Checkpoint `file:` tags join the injected
  `files` set — a first-ever view of a cited file counts as
  `CoveredReViews`, inflating exactly the metric the eval arm
  reads. Filter checkpoint tags out of that set, or accept the
  bias consciously.
- **`SearchByEventType("checkpoint")` spans granularities** —
  returns turn digests too. Probably fine (recall by granularity
  isn't a stated need), but decide consciously.
- **Test seam:** ride `a.syncSegmentGen` for deterministic
  generation in tests.

### 2. Session hydration — seed, don't inject

On the first turn of a _new_ session (empty notebook, resumptions
keep theirs), hydrate by **seeding the notebook** rather than
injecting a prompt block:

1. Query mem0 for this working dir's prior knowledge. The
   `working_dir` partition is enforced by the shipped machinery
   (#56); what remains is **ordering** — `search_memories` ranks by
   semantic score, but hydration wants metadata order: latest
   checkpoints first, then pinned, then recent
   `decision`/`file_edit`. Fetch a larger `top_k` through the
   partitioned path and sort client-side on metadata
   (`turn_number`, `event_type`, `compression` are all in the
   payload) — semantic top-10 alone would yield false _negatives_
   (project memories exist but didn't rank). Bound to a fixed token
   budget (~2-4K, the `mem0SearchMaxTokens` precedent).
2. Write each selected memory as a notebook entry on the _new_
   session: `Tags += ["hydrated", "origin:<source_session_id>"]`,
   **all recallable prefixes stripped** (`result:`, `turn:`,
   `segment:` — see field traps), origin date **in the entry text**,
   `CreatedAt` preserved.
3. **Sentinel keys:** seeds get `(turn, segment)` below any real
   value (e.g. `TurnNumber = -1`) — see the field trap for why both
   origin and real keys are harmful. Selection, compaction, recall,
   and auto-inject treat them as ordinary entries from there.
4. **Render carve-out — the one new code path this requires.**
   `hydrated`-tagged entries bypass the `boundary <= 0`
   early-return (`notebook_segments.go:827`) so seeds render on
   turn 1. The coverage-position filter
   (`notebook_segments.go:1017`) needs no special case — sentinel
   keys (`turn = -1`) are below any real boundary key, so seeds
   pass it naturally once the boundary moves. Without the
   carve-out, seeds sit in the DB invisible on turn 1 — the model
   pays the re-exploration cost hydration exists to avoid.
5. **Hook point:** before the first `preparePrompt` of the session —
   fires on headless `crush run` too. Note MCP-connection readiness
   and turn-1 latency: the mem0 fetch is on the critical path of the
   user's first request, so it needs a timeout and a proceed-empty
   fallback.

Rejected alternative — system-prompt section (`CacheClassSession`):
keeps the digest out of message history, but bypasses every existing
control (selection ranking, compression under pressure, recall
drill-down) and duplicates the render seam. Seeding gets the same
prefix position for free via the notebook splice and degrades
gracefully under compaction instead of being a fixed tax every
request.

Degraded mode: when the notebook itself is disabled, hydration falls
back to a one-shot injected digest message on turn 1 — strictly
better than today's nothing, but a separate code path; only build it
if non-notebook users matter.

### 3. mem0 partition key — `working_dir` — **shipped (#56)**

- `SyncEntries` sends `working_dir`, normalized by
  `canonicalizeWorkingDir` (`mem0.go:226` — `filepath.Abs` +
  `EvalSymlinks` + case-fold on darwin/windows; symlinked or
  case-variant spellings share one partition).
- `SearchMem0` filters by it, **capability-detected**: the server's
  `search_memories` `InputSchema` is introspected for a `filters`
  arg (`mem0SearchCapsFor`, :271) — server-side filter when
  declared, client-side `filterMem0Results` (:332) verification
  regardless, fail-closed (missing/malformed `working_dir` is
  excluded; an empty result beats a wrong-project one). On a
  filters-grammar rejection it retries unfiltered — the client-side
  check still enforces the partition. Limit arg is schema-detected
  (`mem0LimitArg`, :291) and results are budget-fit.
- Old memories lack the key → they don't hydrate, and `cross:` recall
  no longer returns them either — excluded fail-closed, so
  pre-partition memories are effectively retired rather than demoted.
- The `hydrated` provenance tag is checked in `SyncEntries` — seeds
  never re-sync.
- **For hydration:** the partition is now enforced by machinery
  hydration reuses; what remains is _ordering_ — `SearchMem0` ranks
  by semantic score, while hydration wants metadata order (latest
  checkpoints first, then pinned, then recent `decision`/
  `file_edit`). Sort the filtered results on metadata client-side.

### 4. The handoff composition

Once `PlanItem` exists (`PLAN_ARTIFACT.md`), hydration gains
its highest-value payload: **the prior session's open plan items**.
Checkpoint answers "what was learned"; open items answer "what was
left." Together they are a real session handoff — the new session
starts with position and agenda, not just recall access. Order
dependency: hydration is useful without plans (checkpoints + pins
alone), so this doc does not block on the topology work, but the
open-items payload is the reason to do it.

Second consumer: hydrated `file:`/`decision` entries are also the
session-hot signal the project index's ranking wants
(`CONTEXT_PREFETCH.md`, Ranking) — files a prior session actually
worked in outrank raw import centrality. One-way read; the index
never writes back.

## Measurement

`EVAL_HARNESS.md` arm, two axes:

- **Cold-start:** same task in a fresh session, hydration on vs off.
  Metric: redundant re-reads — tool calls on files already covered
  by hydrated checkpoints (countable via the `file:` tag overlap;
  `nbStats.CoveredReViews` already counts re-views of injected files
  and is a partial reuse).
- **Checkpoint utility:** within a long session, recall calls and
  re-reads before vs after the checkpoint trigger fires. A
  checkpoint that doesn't reduce re-reading is a token cost with no
  return — the arm exists to catch that. Track a
  "checkpoint-present-at-render" rate from day one (parallel to
  TURN_DIGEST's digest-present metric) — it makes the async-race
  and mid-run-render claims measurable.

## Non-goals

- No planner/decider judging "is context sufficient" — the
  checkpoint is an artifact, not a gate; confidence remains the
  model's, made checkable.
- No mid-run interruption to force consolidation — triggers are
  observed boundaries, not interrupts.
- No cross-machine/team memory sharing — `agent_id: "crush"` +
  `working_dir` is per-user per-project; sharing semantics are a
  separate design.
- No semantic/embedding search over the local notebook — mem0
  already owns the fuzzy side; local stays tag/text.

## PR ordering

1. ~~`working_dir` partition key~~ — **shipped (#56).**
   Capability-detected server-side `filters` + fail-closed
   client-side verification, `canonicalizeWorkingDir`
   normalization. Resolved the cross-project bleed and the
   server-filter hard-dependency question.
2. `checkpoint` event type: generator prompt with granularity-
   aware input filter, deterministic triggers (write-boundary +
   run-end) with the async-input rule, **type-driven pin at
   boundary/session granularity with latest-only retention**,
   explicit `entryTypeRank`, `recall` dispatch, `coder.md.tpl`
   pointer line.
3. Hydration seeding: mem0 fetch (metadata-ordered) → write seed
   entries with sentinel keys, provenance tags, stripped handles,
   date-in-text; `SyncEntries` skip for `hydrated`; **the render
   carve-out** (`hydrated` entries bypass the `boundary <= 0`
   gate; sentinel keys already satisfy the position filter).
4. Selection quota/penalty for seeded entries — only if sentinel
   decay proves insufficient in telemetry.
5. Eval arms (cold-start, checkpoint utility) in `EVAL_HARNESS`.
