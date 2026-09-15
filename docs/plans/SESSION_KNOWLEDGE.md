# Session Knowledge — Consolidated Checkpoints & Cold-Start Hydration

> **Status:** Spec. Covers two halves of one gap — the notebook is an
> event log with no consolidated position (within-session), and its
> mem0 sync is write-only in practice because retrieval is pull-only
> (cross-session). Proposes a `checkpoint` entry type plus session
> hydration by notebook seeding. Composes with `HARNESS_TOPOLOGY.md`
> (the plan artifact's `Evidence` handles), `CONTEXT_NOTEBOOK.md`
> (entry machinery), `NOTEBOOK_QUALITY.md` (selection/sufficiency),
> and `EVAL_HARNESS.md` (cold-start is a measurable arm).

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
   Sufficiency telemetry (`nbStats`, `agent.go:225`) counts recalls
   after the fact; it does not give the model the artifact that
   would make the recall unnecessary.
2. **New session = cold start from zero.** Entries are
   session-scoped — every `GetEntries`/`SearchByTag`/
   `SearchByText`/`GetByTurn` takes `sessionID` (`retrieve.go:15-70`).
   mem0 sync pushes every entry out (`mem0.go:35-67`), but the only
   way back in is `recall cross:` (`recall.go:83-89`) — a
   model-initiated pull. A fresh session has an empty notebook and a
   model that does not know what it does not know, so the query is
   never issued. The knowledge was persisted; the hydration path was
   never built.

## What exists

- **Push-side retrieval already exists — session-scoped.**
  `maybeAutoInject` (`agent.go:2015`) matches file references in the
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
  compression (`pinnedEntryIDs`, `retrieve.go:310`) — a checkpoint
  pins itself.
- **The investigation→execution boundary is observable for free.**
  `writeToolNames` (`stubs.go:28-98`) is a deterministic classifier;
  the first successful write-class call after a run of
  exploration-class events is a detectable transition — no model
  judgment needed to know _when_ to consolidate.
- **Generation plumbing is reusable.** The small-model generator
  (`generator.go`) already turns classified events into entry prose;
  a checkpoint is a generation over the run's entries (a summary of
  summaries), not over raw events.
- **Injection seam for hydration.** Notebook content is
  rebuilt/spliced per step at `PrepareStep` (`agent.go:954-976`);
  entries seeded into the new session's notebook ride that machinery
  with zero new prompt plumbing — selection, compaction, recall, and
  auto-inject all apply to them unchanged.
- **Cache-class machinery for the alternative.** `prompt/sections.go`
  defines `CacheClassSession` (stable within a session, differs
  across) — the right class if hydration lands in the system prompt
  instead of the notebook. See the seam decision below.
- **mem0 metadata is extensible.** `SyncEntries` already sends
  `{session_id, turn_number, event_number, event_type, tags,
compression}` (`mem0.go:44-51`) — adding a key is an additive
  change; old memories simply lack it.

### Field traps

- **`agent_id: "crush"` is a global namespace.** `SearchMem0` and
  `SyncEntries` both key on the bare agent_id (`mem0.go:54,84`) with
  no project partition — today a `cross:` search in repo A can
  surface repo B's memories. Hydration would make this bleed
  _automatic_ instead of model-initiated; the partition key must
  land first, not after.
- **Evidence handles die at the session boundary.** An entry's
  `result:<tool_call_id>` recall resolves against the current
  session's stored messages. A hydrated entry carrying a dead handle
  advertises a recall that can never resolve — hydrated entries must
  strip `result:` references (keep `file:`/`turn:` refs as text, not
  as resolvable handles) or carry them under a clearly dead
  provenance marker.
- **Hydrated entries must not re-sync.** Seed entries written to a
  new session's notebook would flow back through `SyncEntries` and
  duplicate the memories they came from — one copy per session,
  compounding forever. Provenance tag → `SyncEntries` skips them.
- **A hydration digest is stale at injection time.** Knowledge
  injected on turn 1 describes the _last_ session's world; files may
  have changed since. Entries must carry their origin timestamp
  (`CreatedAt` already exists) and the digest should say when it was
  written — a checkpoint that can't date itself will be trusted
  past its validity.
- **Selection competition.** Seeded entries join the working-set
  selection pool (`notebook_selection.go`); a flood of stale seeds
  can outrank live turn-1 entries. Seeds need a recency penalty or a
  capped quota in selection, not free admission.
- **Post-run generation ordering.** Run-end checkpoints generate in
  the post-run goroutine (`agent.go:1406`+); segment coverage is
  transactional and never regenerates — a checkpoint is itself an
  event needing coverage, or an explicitly excluded entry type.
  Pick one; silent exclusion is the default to avoid a generation
  feedback loop (checkpoints summarizing checkpoints).

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

- **Generation:** small-model call over the segment's
  exploration/decision entries (summary-of-summaries — the raw
  events are already compressed once; the second pass is cheap).
- **Triggers, deterministic:**
  - First successful `writeToolNames` call in a run containing ≥N
    exploration events with no checkpoint yet — the
    investigation→execution boundary.
  - Run end, if context was gathered and no checkpoint exists
    (slot: post-run pipeline, same machinery as segment entries).
  - Model-initiated pin via a parameter on the existing `todos`-class
    tool surface — optional, only if the deterministic triggers miss
    in practice.
- **Compression policy:** checkpoints self-pin (`CompressionLevel`
  floor = Summary; never TagsOnly). They are the anchor that
  survives compaction — the same artifact answers "where was I"
  after a mid-session compact.
- **Prompt surface:** one line in `coder.md.tpl` — before re-reading
  a file to re-derive a fact, consult the latest checkpoint. The
  artifact does the work; the prompt just points at it.
- **Coverage:** checkpoints are excluded from segment coverage
  (they're generated FROM coverage). Mark via the event type so
  `classifyEvents` never feeds one back in.

### 2. Session hydration — seed, don't inject

On the first turn of a _new_ session (empty notebook, resumptions
keep theirs), hydrate by **seeding the notebook** rather than
injecting a prompt block:

1. Query mem0 for this working dir's prior knowledge, ranked:
   latest session's checkpoint entries first, then pinned entries,
   then recent `decision`/`file_edit` entries — bounded to a fixed
   token budget (~2-4K, the `mem0SearchMaxTokens` precedent).
2. Write each selected memory as a notebook entry on the _new_
   session: `Tags += ["hydrated", "origin:<source_session_id>"]`,
   `result:` handles stripped, `CreatedAt` preserved so staleness is
   visible.
3. Done. Selection, compaction, `recall`, and auto-inject treat
   them as ordinary entries — the whole point of seeding is that no
   new render path exists.

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

### 3. mem0 partition key — `working_dir`

- `SyncEntries` metadata gains `working_dir` (raw path — `file:`
  tags already leak paths, so this adds nothing new sensitivity-wise)
  and the session's origin provenance.
- `SearchMem0` filters by it: server-side metadata filter if the
  MCP `search_memories` tool accepts one (check the server's schema —
  `mcp.RunTool` passes arbitrary args); otherwise post-filter on
  returned metadata. Unfiltered fallback must _not_ silently return
  cross-project memories — an empty result beats a wrong-project
  one.
- Old memories lack the key → they don't hydrate (acceptable; they
  remain reachable via explicit `cross:` recall).
- The `hydrated` provenance tag is checked in `SyncEntries` — seeds
  never re-sync.

### 4. The handoff composition

Once `PlanItem` exists (`HARNESS_TOPOLOGY.md` PR 2), hydration gains
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
  by hydrated checkpoints (countable via the `file:` tag overlap).
- **Checkpoint utility:** within a long session, recall calls and
  re-reads before vs after the checkpoint trigger fires. A
  checkpoint that doesn't reduce re-reading is a token cost with no
  return — the arm exists to catch that.

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

1. `working_dir` partition key in `SyncEntries` + filtered
   `SearchMem0` (unblocks safe hydration; fixes the cross-project
   bleed for `cross:` recall too).
2. `checkpoint` event type: generator prompt, deterministic
   triggers (write-boundary + run-end), self-pin, coverage
   exclusion, `coder.md.tpl` pointer line.
3. Hydration seeding: mem0 fetch → rank → write seed entries with
   provenance tags + stripped handles; `SyncEntries` skip for
   `hydrated`.
4. Selection quota/penalty for seeded entries (if telemetry shows
   seeds crowding out live entries).
5. Eval arms (cold-start, checkpoint utility) in `EVAL_HARNESS`.
