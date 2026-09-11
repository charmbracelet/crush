# Tool Result Lifecycle — Proposal Analysis - Ongoing

## Question

When the agent fixes code, does it need the execution output again?

Answer after analysis: **the conclusions yes, the evidence mostly no** — and
the split is not staleness but _reproducibility_.

## Problem

Every tool result is persisted as a `message.Tool` row
(`agent.go:1050` OnToolResult) and replayed **verbatim** on every
subsequent request via `preparePrompt` (`agent.go:1668`). Per-call
truncation exists (bash 30K chars, grep 100 results) but nothing
considers whether a result is still _relevant_ N turns later.

`CONTEXT_NOTEBOOK.md` measured ~70% of request tokens as old tool
results. The notebook compresses pre-boundary turns into entries, but
the raw window (last ~25K tokens, `DefaultRawTokenBudget`) replays
everything — including content that is stale, superseded, or trivial.

## Retention model

Two axes determine whether a tool result is needed again:

| Axis            | Question                      | Consequence                                                         |
| --------------- | ----------------------------- | ------------------------------------------------------------------- |
| Reproducibility | Can the model re-derive this? | If yes, safe to stub — re-view/re-run recovers it                   |
| Supersession    | Does newer info replace it?   | If yes, the old copy is worse than dead weight — it is _misleading_ |

Applied to the artifacts a fix produces:

| Artifact                  | Reproducible?                          | Superseded by?               | Keep?                                                       |
| ------------------------- | -------------------------------------- | ---------------------------- | ----------------------------------------------------------- |
| `view` result             | Yes (re-view)                          | Successful edit to same file | **Stub** after supersession                                 |
| `edit`/`write` result     | n/a (one-line confirmation)            | —                            | Keep (already tiny)                                         |
| `bash` success output     | Mostly (re-run)                        | —                            | Stub after immediate turn                                   |
| `bash` **failure** output | **No** — pre-fix state is gone forever | —                            | Digest survives in entry; full text recallable              |
| `grep`/`glob`/`ls`        | Yes                                    | —                            | Stub earliest (already classified trivial)                  |
| Any `IsError` result      | No                                     | —                            | Keep digest — needed for "same failure or new?" comparisons |

The critical case: **a `view` result followed by a successful edit to
the same file is actively harmful.** It shows pre-edit state. The model
may diff against phantom content and produce wrong follow-up edits.
Dropping it is not just token savings — it removes a correctness hazard.

The equally important counter-case: **a failed edit does NOT supersede
the read.** The file is unchanged; the earlier read is still current
state. Supersession must be success-aware.

## What already exists

- Per-event classification: `notebook/classify.go` `classifyEvents` —
  significant events get entries, trivial ones (grep/glob/ls) get one
  exploration mini-entry.
- Selection: `agent/notebook_selection.go` `selectNotebookEntries` —
  recency → ref-matching → budget fill, capped at 12K tokens.
- Supersession at entry level: `dropSupersededReads` drops older
  `file_read` entries when a newer same-file entry exists.
- Compression + recall: `EntryTextFull` retains uncompressed text;
  `tools/notebook/recall.go` re-expands compressed entries.
- Cross-session: `notebook/mem0.go` syncs entries with tag metadata.
- Deterministic self-healing: `edit_whitespace.go` auto-corrects
  whitespace on match; `notFoundError` (`tools/edit.go:233`) appends a
  `diagnoseMismatch` hint.
- Loop detection: `agent/loop_detection.go` — signature hashing over a
  10-step window, >5 repeats = loop.
- Malformed-input defense: `sanitizeToolInput` converts bad JSON tool
  calls into clean error results.

## Gaps

1. **Raw window has no supersession.** `dropSupersededReads` operates
   on notebook entries only. A `view` made stale by a same-window edit
   is replayed verbatim.

2. **Bug: failed edits supersede reads.** `isSignificant`
   (`classify.go:30-31`) marks `edit`/`write`/`multiedit` significant
   unconditionally — including failures. `dropSupersededReads` drops a
   `file_read` when _any_ newer same-file entry exists, so a failed
   edit evicts a still-accurate read. `Entry` carries no success bit.

3. **Failed edits cost a full round-trip.** On "old_string not found"
   the model must issue a separate `view` to see current content. The
   harness knows the file; the error could carry it.

4. **No pin for files under active edit.** Claude Code's compaction
   explicitly preserves "files being edited." Nothing protects the
   entry describing a file mid-edit-loop from aging out of selection.

5. **No pre-compression hook.** `Compact()` downgrades entries
   Full→Summary→TagsOnly silently. No way to export state first
   (analog of Claude Code's PreCompact hook).

6. **Stubbed results would not be recallable.** `recall` works on
   notebook entries, not on raw-window tool results. A stub is only
   safe if the original is one tool call away.

## The cache constraint

PR #16 stabilized the prompt prefix for cache reuse; cache breakpoints
sit on the last tool, last system message, and last 2 messages
(`agent.go:727-734`, `928-933`). Everything before is the cached prefix.

**Appending messages is free; mutating them is not.** Any change to a
historical message invalidates the cache from that point forward — one
full-prefix reprocess per transition. A view→edit supersession inside
the raw window means transitions happen constantly.

This forces a design choice:

- **(a) Render-time stubbing, monotonic.** Mark results stale in DB;
  `preparePrompt` renders stubs for flagged results. Once flagged,
  never unflag → deterministic, stable after transition. Cost: one
  prefix invalidation per supersession event. **The flag is metadata —
  it must never overwrite `ToolResult.Content`.** Notebook generation
  (`agent.go:1298-1316`) and PR 4's recall both re-read stored
  messages; overwriting would degrade entries and break recall.
- **(b) Piggyback on boundary moves.** Defer pending stubs until the
  notebook boundary advances — the raw-window prefix shifts then
  anyway, so the invalidation is already paid. Superseded content stays
  verbatim slightly longer; stub application becomes ~free.
- **(c) Append-only only.** Never rewrite history; put all staleness
  handling in the notebook entries and in _new_ tool results. Zero
  cache cost, but raw-window staleness remains.

Recommended: **(b)**, with (a)'s monotonic flag as the mechanism. If
measurement shows prefix reprocessing dominates, fall back to (c) and
accept that raw-window staleness is a per-session tax bounded by
`DefaultRawTokenBudget`.

Note: the failure mode is asymmetric — a stub costs ~20 tokens; a
dropped conclusion costs a broken reasoning chain. Bias toward keeping
digests of ephemeral state.

## Proposal

### PR 1: Success-aware supersession (bug fix)

- Add `Succeeded bool` to `notebook.Entry`; populate from
  `!result.IsError` in `classifyEvents` (result is already in
  `EntryInput`). Persist via migration or reuse an existing column.
- `dropSupersededReads`: only a **successful** newer entry supersedes a
  read. A failed edit leaves the read intact.
- Verification: unit test — read → failed edit → read retained; read →
  successful edit → read dropped. Existing
  `notebook_selection_test.go` cases must still pass.

### PR 2: Self-healing tool results (append-only, cache-free)

Push known recoveries into the tool layer instead of burning a
model round-trip:

- `edit` on "old_string not found": extend `notFoundError` to attach
  the current content of the target region (bounded, e.g. ±40 lines
  around the best fuzzy match). The error result carries fresh ground
  truth; the model retries immediately.
- `bash` on non-zero exit for known build/test commands: append LSP
  `diagnostics` output when available.
- Constraint: attached context is **new** content in a **new** message —
  append-only, no prefix invalidation. Also makes the superseded read
  _genuinely_ stale (fresher state now exists in the error).
- Verification: unit test `notFoundError` output shape; eval — measure
  edit-retry rate and turns-to-success on edit-failure scenarios.

### PR 3: Raw-window supersession stubbing

- On successful `edit`/`write`/`multiedit`, flag prior `view` results
  for the same file as stale (monotonic DB flag on the message part;
  original content retained — see cache-constraint note).
- `preparePrompt` renders flagged results as stubs:
  `"[content of <path> superseded by edit at turn N; re-view for
current state]"`. The tool-result message must remain — providers
  require a result per tool call; only the content shrinks.
- Apply pending stubs at notebook boundary advances (option b above);
  never mid-window.
- Never stub: `IsError` results, results under ~200 bytes (not worth
  the invalidation even when piggybacked), or any result from the
  **last 2 completed turns** — matching the recency rule in
  `selectNotebookEntries`. Turn-based, not count-based: a bash-spam
  turn must not push a still-active view past the guard, and a
  same-turn view→edit pair (the model is actively reasoning over that
  read _right now_) is never eligible.
- **Scope note**: because stubs only apply at boundary advances, a
  view→edit pair near the end of a session is never stubbed — there is
  no future boundary move to piggyback on. Benefit is concentrated in
  long sessions with multiple boundary shifts; short sessions will show
  zero stubbing. Residual exposure is bounded by
  `DefaultRawTokenBudget`.
- **Ordering invariant**: `GenerateEntries` re-fetches stored messages
  via `messages.List` (`agent.go:1298`). Entries must always be
  generated from **original** content, never rendered stubs. In the
  common case this holds by timing (entries generate at turn end;
  flags are set by later turns), but the stale-turn fallback and any
  re-generation path depend on the flag-is-metadata invariant holding
  unconditionally.
- Verification: golden test on `preparePrompt` output; explicit unit
  test — "boundary advance stubs the raw render while the notebook
  entry for the same turn captures full original content"; measure
  cache-invalidation count per session vs tokens saved (PR 1
  telemetry).

### PR 4: Digests for ephemeral results + recall safety valve

- For `command` events with `IsError`, the generator prompt already
  receives `[ERROR]` + truncated content — add a required "error
  headline" field (first error line + exit code) to `GeneratedEntry`
  so the digest survives compression deterministically, not at LLM
  whim.
- Extend `recall` (or add `recall_result`) to fetch an original tool
  result by `tool_call_id` from `messages` — the DB still has the full
  content. Converts every stub from lossy to recoverable.
- Verification: unit test digest field presence on error entries;
  integration test recall-by-tool-call-id.

### PR 5: Pins and hooks

- Pin entries tagged `file:<path>` while that file appears in the last
  2 turns' edit calls — "files being edited" never age out.
- Optional `PreCompact` hook event fired before `Compact()` downgrades
  entries, reusing the hooks engine (`internal/hooks/`). **Caveat**:
  the engine currently defines only `EventPreToolUse`
  (`hooks.go:14-15`) — this needs a new event type and a trigger call
  site in `Compact()`, plus an input-payload shape. The engine is
  decoupled from fantasy/agent so the addition is feasible, but it is
  new plumbing, not a config change.
- Verification: unit test pinned entries survive `Compact` and
  selection-capping.

## Measurement

Reuse `PromptSection` telemetry (PR #12) — add `raw_history` split into
`verbatim` vs `stubbed` bytes. Track per session:

- Stubbed bytes per turn (savings)
- Cache prefix invalidations per session (cost of PR 3)
- `recall`/`recall_result` invocations (are stubs being re-needed?)
- Re-`view` rate of already-read files (direct signal for "did it need
  the info again")
- Edit-failure rate (regression guard for PR 1/2)

The SQLite corpus used for `CONTEXT_NOTEBOOK.md` (107 sessions) can
answer the motivating question empirically before building: how often
does a session re-read a file it already read, and how often does a
successful edit follow a read of the same file? That ratio sizes the
win.

## Non-goals

- **Plan mode.** Claude Code's plan mode is an approval workflow +
  constrained exploration; its context benefit is incidental (fewer
  wasted reads). A real equivalent means a new agent + permission
  gate + UI mode — separate plan. The `task` sub-agent already
  provides delegation.
- **Whole-session summarization.** Covered by `CONTEXT_NOTEBOOK.md` /
  `NOTEBOOK_VS_SUMMARY.md`.
- **Cross-session failure scoring.** mem0 sync covers cross-session
  recall; loop detection covers the acute intra-run case. A flaky-tool
  scoreboard is a telemetry feature, not a context feature.
- **Generative/fuzzed inputs.** Test-suite concern, not runtime.

## Risks

- **Reasoning references to stubbed content.** Assistant text may say
  "the bug is on line 42 where X" — the conclusion survives in the
  text; the evidence does not need to. Mitigation: stubs name the file;
  recall recovers the original.
- **Cache invalidation underestimated.** Mitigation: PR 3 behind config
  flag; measure invalidations before enabling by default.
- **Model-dependent sensitivity.** Some models may degrade more on
  missing context. Mitigation: regression eval per supported model
  before default-on.
