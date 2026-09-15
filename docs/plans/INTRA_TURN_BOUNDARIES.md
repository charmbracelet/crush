# Intra-turn boundaries — notebook coverage below the user-message grain

Status: implemented (PRs #24, #25).
Root cause verified against live session `23826edc` (79 requests,
6.45M provider-reported tokens, ~6.2M cached reads). Sharpening
context: auto-summarize is disabled when the notebook is on
(`agent.go:1164`), so today a long autonomous run has **no cap at
all**.

## Problem

`preparePrompt` runs **once per turn** — at `Run` start
(`agent.go:872`) and inside `Summarize` (`agent.go:1524`). There is
no per-step recompute: `PrepareStep` (`agent.go:897`) works on
`[]fantasy.Message` and never touches the boundary.

For a single-prompt autonomous task (1 user message → N agent
steps — the dominant real-world shape):

1. Turn-start `preparePrompt` computes the boundary **before any
   of the turn's messages exist**. Fresh session → `boundary = 0`.
2. `rawMsgs = msgs[0:]` — everything, forever. No code path
   recomputes the boundary while the run is in flight.
3. The stale-turn fallback doesn't even fire: it is guarded by
   `boundary > 0` (`agent.go:1723`) and only acts across turns.
4. Superseded machinery counts user-message turns: promotion's
   recency guard never opens mid-run, and
   `flagPrunableToolResults` only runs in the post-run goroutine
   (`agent.go:1333`) — flagging has no mid-run trigger at all.

Measured result: ~97K of history rendered verbatim on every one of
79 requests; final request 113K.

## Design

### 1. Per-step boundary application (the missing machinery)

Inside `PrepareStep`, per step:

- **Placement: before the step's assistant `Create`**
  (`agent.go:972`). At that point the DB holds only completed
  steps — the trailing-empty-assistant edge case doesn't exist,
  and an in-flight placeholder can't trip the step cap early.
- **Message source**: `a.messages.List(ctx, sessionID)` — internal
  `message.Message` is required (`estimateRawMessageTokens` reads
  `ToolResult.Superseded`). Call `a.messages.FlushAll(ctx)` first
  — one line, idempotent; makes correctness independent of the
  service's flush internals (Update is already 33ms-deferred; the
  current safety relies on structural/terminal updates flushing
  synchronously before PrepareStep, which FlushAll makes
  explicit).
- **The splice runs every step** — fantasy rebuilds
  `stepInputMessages = initialPrompt + responseMessages` fresh per
  step; a rewrite of `prepared.Messages` never persists. Skipping
  the splice on an unmoved boundary would flip-flop
  pruned/unpruned — worse than never pruning.
- What can be gated is the **re-render**: cache the rendered
  prefix keyed by the boundary value (`stubBoundary`-style). A
  nice property falls out: `buildNotebookMessage` filters to
  covered segments only, so entries landing mid-segment don't
  change the prefix — churn is exactly one invalidation per
  advance, never one per entries-arrival, and the
  byte-identical-no-op criterion becomes trivially satisfiable.
- **Recompute through the existing `preparePrompt` path** on the
  re-fetched messages, then replace `prepared.Messages` — chosen
  over mapping a stored boundary to a fantasy prefix-drop: the
  preparePrompt path gives flagging, stub promotion, notebook
  injection, and upstream's call-adjacent tool-result ordering
  (#3743) in one representation.
- **Splice position, pinned**: immediately after the fold block
  (~926), before `workaroundProviderMediaLimitations` — the
  cache-control pass (~946-958) and `promptPrefix` prepend then
  apply to the rebuilt list unchanged; anywhere later drops
  breakpoints or the prefix. The rebuild preserves the leading
  system message (`stepInputMessages[0]` comes from
  `initialPrompt`, outside the `preparePrompt` path) and drops
  the fold-time `prepared.Messages` append (`agent.go:924`) —
  folded prompts are persisted, so the rebuild already contains
  them.

### 2. Segments as the coverage/recency unit

A **segment** = a group of completed steps inside a user turn.

**Safe point** — `isTurnEnd` cannot be reused: `ToolCalls()`
returns calls regardless of completion (`content.go:350`), so an
assistant message with zero calls only exists at a turn's last
step. The intra-turn predicate is a **step boundary**: index `i`
is safe iff every tool call in `msgs[:i]` has its result in
`msgs[:i]` — i.e., cut immediately before the next assistant (or
user) message, after a completed result run. New predicate:
`allCallsResolved`-style, **with an orphan escape** — a call that
has no result _anywhere_ in `msgs` (cancelled/interrupted calls
never produce one; render synthesizes instead) must not block.
Otherwise one cancelled call freezes the predicate permanently
and the entire post-cancel tail stays raw for the session.

**Close rule** (first match): ~15-20K estimated tokens since the
segment opened, or ~10 steps, or a user message (see Folded
prompts, Risks). `segmentBoundaries(msgs) []int` — pure,
deterministic, forward-only.

**Snap rule**: coverage is per-segment, so the raw window is
always whole segments — a budget-tripped index landing
mid-segment snaps forward to the next segment start (the stale
pull-back catches uncovered ones). One exception closes the
hole: **the open segment is always raw**, even if it alone trips
the budget — otherwise a trip inside the open segment snaps to
`len(msgs)` and the window goes empty. (Same edge the old
turn-level code handled via `findMostRecentTurnStart`,
`notebook_boundary.go:28-36`.)

**Segment identity is recorded ranges, not recomputed positions**
— `estimateRawMessageTokens` counts stub text once
`Superseded.Applied` flips (`notebook_boundary.go:143`), and
promotion fires on boundary moves inside closed-but-uncovered
segments. A promotion inside `[s, e)` shrinks stored content →
`segmentBoundaries` recomputes segment k's close point to
`e′ > e` → the marker for (T, k) appears to cover `[e, e′)` with
entries that only cover `[s, e)` — invariant 2 silently broken.
Two fixes — the registry carries correctness; the estimator
split is hygiene:

- **Estimator split**: segmentation's accumulator ignores
  `Applied` — always counts full `Content`. With registry
  authority this is hygiene, not correctness — its remaining
  value is preventing close-delay drift in the _open_ tail (a
  promotion mid-segment pushes the close out). Keep it, but it's
  the second line of defense, not the fix.
- **Durable segment registry**: `processed_segments` records each
  closed segment's `start`/`end` (message indices or IDs) at
  close. Identity-by-range is immune to any content mutation —
  promotion, sanitization, future estimators. Catch-up then
  enumerates the registry instead of recompute-and-diff, and
  coverage = "recomputed range fully contained in processed
  ranges" — a drifted, partially-covered segment fails closed
  (stays raw) rather than silently dropping its tail.

**Authority**: the registry is authoritative for _closed_
segments; `segmentBoundaries` recompute governs only the open
tail. `findSegmentBoundaryByTokenBudget` reads registry extents
for closed segments + computed tail — never a full recompute it
then hopes matches the registry.

**Coverage is three states, not "entries exist"** —
`GenerateEntries` stores nothing on an empty event set
(`classify.go:390-393`), so a pure-text segment produces zero
entries and `segmentsWithEntries` never contains it: pinned raw
forever plus catch-up re-firing an LLM call every step —
livelock. But the states must not be conflated — a durable
"started" marker can't work: on generation failure it either
reads as covered (content drops, violating invariant 2) or stays
set and never re-fires (pinned raw). The split:

- **`unprocessed`** — durable `processed_segments` row INSERTed
  **at close** with `state=unprocessed`: the extent is recorded
  immediately, so the registry is authoritative from the moment a
  segment closes (restart reconstructs geometry from the table,
  not recompute) and catch-up enumeration is literally a query.
- **`in-flight`** — in-memory per-session set on the agent;
  written at the decision to generate, cleared **after** the
  entries+marker commit lands (clearing earlier lets a detection
  pass in the gap re-fire). Intra-process dedup only.
- **`processed`** — the row's state UPDATEd **on completion in
  one transaction with the entries**.
  Success-with-zero-entries counts (no pin, no re-fire);
  failure leaves it `unprocessed` (clears in-flight, retry with
  backoff, never covered). The stale fallback and coverage
  queries consult `processed` only — an `unprocessed` or
  `in-flight` segment stays raw until coverage lands, so
  invariant 2 holds through the async window and across
  restarts.

A separate table, not a sentinel entry row: a sentinel would flow
through `GetEntries` → selection → render, `recall`,
`notebook_search`, mem0 sync, token counts, compaction ordering,
and `PinnedFileTags` unless filtered at every consumer.

**The turn-grain version of this bug exists today**: a no-event
user turn never gets entries and pins `findStaleTurnBoundary`
forever — the marker fixes both grains.

**Migration/backfill** — pre-upgrade sessions have an empty
registry, so the first post-upgrade `PrepareStep` sees all prior
history as unrecorded tail: catch-up would regenerate entries for
turns that already have turn-grain entries (notebook doubles) and
until regeneration lands those turns pin as "unprocessed." Fix:
on backfill, **"turn has entries" ⇒ `processed`** — INSERT the
row directly, no generation. And cap/stagger the first-pass
burst — it's every uncovered segment at once otherwise.

**Coverage**: on segment close, fire async `GenerateEntries` over
just that segment's messages **and** `flagPrunableToolResults`
**over the full history** — `flagPrunableToolResults(allMsgs)` at
close time. Scope matters: the mid-run stub win is "a read
superseded 2 segments ago gets stubbed," which only the later
segment's pass over all messages can flag. Flagging just the
segment slice silently guts cross-segment supersession. A closed
segment lacking `processed` stays raw: the stale fallback
pulls back to the **segment** start, exposing ~1 segment, not the
turn.

**Run-end generation scopes to the uncovered tail**:
`extractCurrentTurnMessages` returns the whole turn — with
mid-run segment entries, run-end must generate only for messages
after the last covered segment, or every segment's entries get
duplicated.

**Catch-up generation** — nothing today generates entries for a
past turn: the run-end goroutine is scoped to its own turn via
`extractCurrentTurnMessages` + `notebookTurnNumber`, so a
cancelled turn's uncovered tail stays raw **forever** (stale
fallback pins it in every later render). Segments bound the
damage; they don't remove it. Fix: during segment detection, fire
generation for _any_ closed segment lacking a processed marker —
the registry makes them detectable. **Fire once**: detection
re-runs every step while generation is async — the in-flight
mark must be atomic with the decision to generate (in-memory;
the durable `processed` write stays on completion). And the
generation path must re-derive turn numbers from `messageTurns`,
not reuse the run-start `turnNumber` — folded prompts shift
numbering mid-run.

**Event numbering**: `GenerateEntries` numbers entries per
invocation (`classify.go:395-427`), so two segment calls in one
turn would emit duplicate `(turn, event)` pairs — breaking
`ORDER BY turn_number, event_number`, `dropSupersededReads`' key,
and header identity. Fix: **continue `event_number` across
segments** — offset by the turn's existing max. Keeps
`Turn N.M` display unchanged (M stays `event_number`); no
composite-key surgery on `ORDER BY`/`dropSupersededReads`.
`segment_number` remains a column for coverage keys, never
displayed.

**Serialization — the offset read is a TOCTOU race**: segment
generation is async and unordered. Segment N's goroutine can be
in-flight (small-model call, seconds) when N+1 closes; both read
"existing max" before either stores → overlapping numbers —
recreating the exact duplicate-key bug this fix kills. Serialize
per (session, turn): a generation queue, or compute the offset
inside the insert transaction (SQLite's write lock makes
`SELECT MAX(event_number) WHERE turn=?` + inserts atomic).
Acceptance must cover the _concurrent_ case, not just
sequential.

**Steady state**: raw = open segment + however many closed
segments still lack `processed`. Nominal band ≈ 15-40K (1↔2
segments); under a slow small model, unprocessed segments stack —
the fallback handles it correctly, but the acceptance criterion
must tolerate the wider band. The segment threshold is the
effective granularity — `DefaultRawTokenBudget` becomes a soft
ceiling, not a hard cut.

## Invariants

1. Boundaries land only on resolved step boundaries — never mid
   tool-call sequence (now true inside a turn).
2. Nothing drops without coverage — stale pull-back preserved at
   segment grain.
3. Flagging metadata-only; stubs promote only on boundary moves.
4. Unchanged recompute → byte-identical prefix (no cache churn).
5. Turns, titles, RunComplete, `Turn N.M` display — untouched.
6. Recorded segment extents are immutable — a close-time
   `start`/`end` never changes; that immutability is what makes
   the registry authoritative.

## Work items

1. **Step-boundary predicate** — `allCallsResolved(msgs, i)` +
   `segmentBoundaries(msgs)`.
2. **`segment_number` schema** — migration; `(turn, segment,
event)` ordering continuity via event-offset numbering,
   **serialized** (per-session generation mutex or offset inside
   the insert transaction — the async closes race).
   Plus the **coverage registry**: a `processed_segments` table
   (not a sentinel entry row — a sentinel would leak through
   selection, `recall`, mem0, compaction, and `PinnedFileTags`)
   recording (session, turn, segment, start, end, state) —
   segment identity-by-range survives content mutation (stub
   promotion shrinks stored content and would otherwise drift
   recomputed boundaries). Lifecycle: `INSERT OR IGNORE`
   `unprocessed` at close under `UNIQUE(session_id, turn_number,
   segment_number)` — detection re-runs every step, so the
   close-INSERT must be idempotent — then UPDATE `processed` on
   completion in one transaction with the entries; intra-process
   dedup lives in an in-memory in-flight set. Optional
   `retry_count`/`last_attempt_at` columns make backoff durable.
   Fixes today's turn-grain livelock too: a no-event turn
   currently pins `findStaleTurnBoundary` forever.
3. **Per-step boundary application** — in `PrepareStep`, before
   the assistant `Create`: `FlushAll` → DB re-fetch →
   `findSegmentBoundaryByTokenBudget` (segment-aware replacement
   for `findTurnBoundaryByTokenBudget`: accumulates per-segment
   tokens walking back, snaps to segment starts over registry
   extents for closed segments + computed tail) → rebuild
   `prepared.Messages` via the
   `preparePrompt` path when moved. **Re-render on move; splice
   always** — the rendered prefix is cached on the boundary value.
   Seams: `preparePrompt` returns `(history, files)` — pick one
   attachment channel so stored `BinaryContent` isn't re-emitted
   alongside `Files`; and the `ProviderOptions = nil` strip
   (agent.go:899-901) runs before the splice — the rebuild must
   emit no provider options, or the strip moves post-splice
   (stale breakpoints could exceed Anthropic's 4-cap).
   Byte-stable rebuild. The turn-start `preparePrompt` boundary
   work becomes redundant — `PrepareStep(0)` recomputes and
   splices before the first request — so the Run path can
   degenerate to raw; the real risk is maintaining two boundary
   implementations, so converge on one.
4. **Segment-close trigger** — record closes (the `unprocessed`
   INSERT) _before_ the boundary walk so the same render's
   pull-back sees them — async `GenerateEntries(segment)`
   - `flagPrunableToolResults(allMsgs)`; recency guard counts
     segments. Fire-once via the in-memory in-flight set —
     detection re-runs every step, and async latency would
     otherwise re-fire the same segment. `processed` (durable) is
     what coverage consults; failure clears in-flight and retries
     with backoff — never covered.
5. **Run-end scoping + catch-up** — run-end generates only the
   uncovered tail and writes **entries + `processed` marker for
   the tail in one transaction** (it becomes a closed segment
   when the next user message lands; without the marker,
   catch-up re-fires it into duplicate entries). Segment
   detection also fires generation for _any_ closed segment
   lacking `processed` — including crashed runs (a crash leaves
   no durable state; in-memory in-flight resets are harmless).
   **Backfill:** for pre-upgrade sessions, "turn has entries" ⇒
   INSERT `processed` directly, no regeneration; cap/stagger the
   first-pass burst.
   Generation re-derives turn numbers from `messageTurns`, not
   the run-start `turnNumber` — folded prompts shift numbering
   mid-run.
6. **Coverage keys** — `turnsWithEntries` →
   `segmentsWithEntries`; the actual coverage filter is
   `buildNotebookMessage`'s `e.TurnNumber < boundaryTurn`
   (`agent.go:1934`) → lexicographic `(turn, segment)` compare
   against the boundary segment. Selection, pins, auto-inject
   follow the same key. `PinnedFileTags` is `TurnNumber`-keyed
   ("last two turns",
   `retrieve.go:117`) — during a one-turn run it pins _every_
   edited file; decide segment-grain pinning or keep.
   Compaction stays turn-keyed (`retrieve.go:126` — old turns
   only; in-flight segments aren't compaction targets).
7. **Mem0 sync** keyed by (turn, segment) — needs a
   `GetByTurnSegment` query or in-memory filter; `GetByTurn`
   alone doesn't express it.
8. **Telemetry + cleanup** — boundary/segment-advance count per
   session; `crush stats` derives stub stats from stored marks —
   persist advances or derive from `segment_number` coverage.
   New per-session state (in-flight set, rendered-prefix cache)
   joins `stubStats`/`stubBoundary` in the session-delete watcher
   (`stubs.go:505`); `processed_segments` rows need the same
   `DeleteEntries`-style cascade.
9. **Shadow-mode milestone first** — the splice can't be trusted
   until tail fidelity is proven: the raw tail is rebuilt from
   stored messages each step and must byte-match what fantasy
   accumulated in `responseMessages` for the same range, or every
   step takes a full invalidation (worse than today).
   Reasoning signatures, provider-executed calls, and per-part
   provider metadata are the candidate mismatches. First
   milestone splices nothing — rebuild, diff against
   `options.Messages`, log divergences; only then enable the
   splice.

## Acceptance criteria

- Synthetic 1-user-message / 50-step session (~200K chars):
  mid-run renders keep raw within the 1↔2 segment band (~30-40K),
  boundary advances ≥3 times.
- Concurrent segment generation (two closes racing): no duplicate
  `(turn, event)` pairs — the sequential case alone doesn't
  exercise the race.
- A segment producing zero entries (pure text/reasoning):
  marked processed, neither stalls the boundary nor retriggers
  generation — with a deliberately slow mock generator.
- Orphaned tool call at step K (no stored result): boundaries
  still advance past it; the post-cancel tail doesn't freeze
  raw.
- Drift regression: a stub promoted inside a closed-uncovered
  segment must not shift its recorded coverage — the drifted
  tail stays raw until its own generation lands.
- Tail fidelity (recorded session): rebuilt messages for the
  already-sent range are byte-identical to what was sent —
  golden diff, not just correctness.
- File written at step 30 after view at step 5: applied stub by
  ~step 35.
- Run-end generation produces **no duplicate entries** for
  covered segments.
- Boundary unmoved → no re-render (the splice still runs; only
  the prefix render is cached); a re-render emits a
  byte-identical prefix — golden diff.
- Migration: legacy session with existing turn-grain entries —
  first post-upgrade step marks covered turns `processed`
  without firing generation; no duplicated notebook content.
- Cancel mid-run: open segment stays raw; resume pins one
  segment, not the whole turn (today the unprocessed turn pins
  every later turn start — `agent.go:1305` returns before
  generation).
- Folded-prompt step: mid-run `createUserMessage`
  (`drainQueueForStep`) acts as a hard segment boundary and new
  turn.
- Post-run structure unchanged: ordered entries, identical
  `Turn N.M` headers.

## Risks

- **Generation cost** — same coverage, more calls; bounded by the
  token threshold; skip-if-empty exists.
- **Prefix churn** — each boundary advance is a real
  invalidation (~1 per 15-20K); a stale pull-back is a retreat +
  re-advance pair, so an uncovered-segment episode costs two.
  Still a large win vs ~70K re-read per step. Byte-stability +
  unchanged-skip bound it.
- **Folded user messages** — `drainQueueForStep` →
  `createUserMessage` mid-run creates a real user message, bumps
  `countUserMessages`, and truncates `extractCurrentTurnMessages`
  today; segments treat it as a hard boundary — new turn begins.
- **Cancelled segments** — open segment stays raw; covered on
  resume via catch-up.
- **Entry inflation at segment grain** — `hasDecision`
  (`classify.go:235-248`) fires once per `GenerateEntries` call:
  up to one Decision entry per ~15K-token segment instead of per
  turn. Modest; watch via sufficiency metrics.
- **Late results after orphan escape** — `OnToolResult` uses the
  parent ctx, so a cancelled tool can still write its result
  after a boundary moved past the "orphaned" call: it lands raw
  with no visible call and is dropped by the render with a log
  (agent.go:1820). Graceful, but named.

## Non-goals

- Pressure-driven pruning, worker sub-agent.
- Turn numbering, `Turn N.M` display, compaction scope.
