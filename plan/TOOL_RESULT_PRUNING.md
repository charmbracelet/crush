# Tool Result Pruning — Deterministic Context Reduction

> **Status:** Implemented behind `notebook_stub_superseded` (default
> off). Shipped: success-aware supersession, boundary-gated stubs,
> `result:` recall (bounded via `tools.TruncateHeadTail`), error
> digests, file pins, PreCompact, per-session stub telemetry, path
> normalization + case-folding, flip-flop revert-on-failure,
> universal ~50KB capture cap, observed-mutation supersession
> (mtime), duplicate/rerun/stale command stubs, truncation
> provenance labels, and `crush stats` visibility. Remaining:
> pressure-driven escalation (see "merge point") and the default-on
> evidence gate under "Acceptance criteria".

## Layer model

Three layers, three questions:

- **Pruning** (this doc) — deterministic rules, no model call.
  Preserves message structure: call/result pairing, roles, IDs.
- **Compaction** (`CONTEXT_NOTEBOOK.md`) — model-generated per-event
  entries. Lossy, compresses the narrative.
- **Budget policy** (`CONTEXT_WINDOW_SAFETY.md`) — decides when to
  escalate and what to drop last.

Pruning itself splits in two:

| Side                     | Where                                                               | Cache cost                                            |
| ------------------------ | ------------------------------------------------------------------- | ----------------------------------------------------- |
| Ingest (write-time caps) | `convertToToolResult` — content reduced before it lands in history  | Zero — append-only                                    |
| Render (stubs)           | `preparePrompt` — stored original kept, rendered as labeled pointer | One prefix invalidation — paid only at boundary moves |

The pointer contract is what makes render-side pruning non-destructive:
every stub names its recovery path — the live source for reproducible
results ("re-view `foo.go`"), `recall("result:<tool_call_id>")` for
ephemeral ones. SQLite is the offload store; `tool_call_id` is the
pointer.

## Invariants

These held through implementation and load-bearing review; future work
must preserve them.

1. **Flag is metadata.** `ToolResult.Content` is never rewritten.
   Notebook generation and `result:` recall always see the original.
2. **Only success supersedes.** A failed or missing-result write leaves
   the file — and every prior read of it — accurate.
3. **No dead pointers.** A stub may only offer a recovery path that
   exists in the current mode (no `result:` without `recall`).
   Enforced at build: `StubSuperseded` is only set when the notebook
   is enabled (coordinator.go:789) — with the notebook off, previously
   applied marks render verbatim (safe direction; marks stay dormant
   and resume if the notebook returns).
4. **Everything self-labels.** Every truncation and stub states what
   was cut and how to recover. Silent truncation is the known failure
   mode: the model treats a preview as complete.
5. **Measure what the model sees.** Size accounting (boundary
   estimator, telemetry) uses post-stub render size, not stored size.
6. **Stub beats drop.** A stub preserves the call/result skeleton and
   a recallable pointer; dropping is the last resort.

## What shipped — and the intentional divergences

- **Superseded-view stubs, boundary-gated.** `flagSupersededViewResults`
  flags eagerly (metadata, cache-free); `promoteSupersededStubs`
  applies `Applied=true` only when the raw/notebook boundary moves.
  _Divergence:_ the original spec said eager, same-step. Reversed
  because self-healing edit errors (the lifecycle plan's region
  context on `old_string` miss) are the backstop for phantom reads —
  the residual
  cost is one wasted, self-correcting edit call, which doesn't buy a
  second render path in `PrepareStep`. Eager overlay stays rejected
  _unless measured_: trigger is observed phantom-edit failures.
- **Recall folded into `recall`.** `result:<tool_call_id>` dispatched
  in the tool's Run (recall.go:86), not a new tool — one retrieval
  interface. Output is bounded by `tools.TruncateOutput` so the escape
  hatch can't re-import a stubbed 200KB wholesale.
- **Stub shape: name + turn + pointer, no head-prefix.** Right for
  views (a prefix of stale content is itself stale). Keep the
  head-prefix in spec for future _command-output_ stubs, where the
  prefix is the error headline.
- **Error digests** — first line + `Exit code N`, survives all
  compression levels.
- **File pins + PreCompact** — entries for files edited in the last
  two turns survive compaction and selection capping; a deny/halt
  skips the round.
- **Telemetry** — per-session `stubStats`: invalidations, results
  stubbed, bytes saved. Wired into `logStepComposition` and
  surfaced in `crush stats` (results stubbed, bytes saved, per-kind
  breakdown, session count).

## What shipped — second round

1. **Universal cap at `convertToToolResult`** — `toolResultMaxContentBytes`
   (50KB) bounds every tool result's text at write time via
   `tools.TruncateHeadTail` labeled `truncated at capture`. Per-tool
   caps stay tighter; media data is untouched. Destructive at the
   message level by design: recovery is the live source, not recall.
2. **Observed-mutation supersession** — successful `view`/`read`
   results get `FileMtime` stamped at `OnToolResult`; the flag pass
   re-stats the path and flags `modified`/`deleted` kinds when the
   file changed or vanished. Catches bash redirection, `sed -i`,
   MCP writes, and external edits without trusting tool names.
   Results without a recorded mtime (older rows, non-filesystem
   reads) are skipped — a failed stat can't distinguish deletion
   from a read that never touched the filesystem. _Divergence:_
   filetracker wasn't the vehicle — it records read timestamps, not
   mtimes; the per-result stamp is the observation point.
3. **Deferred token-only stubs** — `bash`/`grep`/`glob`/`ls` results
   ≥512B flag `stale`; promotion applies them only at boundary moves
   past the 2-turn recency guard. The stub keeps a ≤320B head-prefix
   as the digest plus the elided byte count and recall pointer.
4. **Duplicate-output supersession** — same tool + canonical input
   (sorted keys, volatile keys like `description` dropped, zero
   values normalized) + byte-identical output → earlier copies flag
   `duplicate`. Byte equality instead of hashing: no collision risk
   and simpler. The latest run always stays verbatim — the age pass
   exempts it so a repeated command keeps one full copy.
5. **Rerun digests** — a differing later output flags the earlier
   result `rerun`, and the stub carries the first line as the
   digest: difference is evidence of delta (fail→pass means the fix
   worked), not redundancy. Error results never flag and never join
   re-run groups.
6. **Labeling audit** — `tools.TruncateHeadTail(content, max,
provenance)` is the single helper behind `TruncateOutput`
   (capture), the universal cap (capture), and `result:` recall
   (recall). Markers state lines + bytes cut, where the cut
   happened, and the recovery path. `SupersededMark.Kind` renders
   per-kind stub text via `StubText`, shared with `crush stats` for
   exact saved-bytes accounting.
7. **User-facing visibility** — `GetPruningStats` aggregates applied
   marks straight from stored message parts; `crush stats` renders a
   "Tool Result Pruning" section (stubbed results, bytes saved,
   sessions, per-kind table) when any exist.

## The merge point with window-safety

Structural triggers (superseded, boundary moved, guard elapsed) are
what's built. The stronger trigger is **pressure**: inside
`PrepareStep`, as the estimated total approaches the input budget,
escalate — stub large results regardless of class → tighten the
recency guard → drop optional sections only after that. A stub
preserves skeleton + pointer; a drop doesn't. This is where the two
plans meet: pruning becomes the graduated response, dropping the last
resort. Spec belongs in `CONTEXT_WINDOW_SAFETY.md` — this doc names
the dependency.

## Acceptance criteria

`notebook_stub_superseded` flips default-on when, across real sessions:

- Edit-failure rate is flat or better vs. flag off (phantom reads
  aren't causing wrong edits).
- `result:` recall rate is low — stubs aren't losing needed content.
- Saved bytes per session ≫ invalidation cost
  (`stubStats.Invalidations` × prefix size vs. `SavedBytes` ×
  remaining turns).
- Zero render flip-flops (stub ↔ verbatim oscillation) in logs.

## Non-goals

- Semantic compaction — `CONTEXT_NOTEBOOK.md`.
- Whole-request budgeting and mandatory-overflow errors —
  `CONTEXT_WINDOW_SAFETY.md` (the universal cap is the shared seam).
- Per-tool cap tuning — existing caps stay.
- Sub-agent sessions — task agents get `Notebook` and
  `StubSuperseded` too (coordinator.go:782+), sharing the
  coordinator's stub maps, so the mechanism covers them incidentally;
  child-session isolation is the bound on their context, not an
  exclusion from stubbing.
- Reasoning-block pruning — provider signature/encryption schemes
  (Anthropic signed thinking, OpenAI encrypted reasoning, Gemini
  thought signatures) make naive stripping fragile; revisit only if
  stored reasoning exceeds ~15-20% of tokens.
- Anthropic's server-side context editing — an alternative executor
  for this same mechanism, provider-locked, no pointer semantics.

## Risks

- **Entry-quality failure looks like recall pressure** — high recall/re-view rates may mean entries lost needed detail, not that stubs are wrong. Read the metrics together.
- **Truncation provenance is ambiguous** — a recalled result that was capped at write-time shows the same generic marker as recall-time truncation; the labeling audit (item 6) covers it.
- **Flip-flop residual** — revert-on-failure plus merge-on-write narrowed oscillation to a single `Get`→`Update` window where a concurrent flag write can still lose a mark; the loss fails verbatim (safe direction) and re-flags next turn, but the acceptance gate stays: zero flip-flops in logs.
