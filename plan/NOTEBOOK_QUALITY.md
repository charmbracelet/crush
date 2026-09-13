# Notebook Quality — Selection Relevance & Compaction Robustness

> **Status:** Implemented (PR #26). Scope: `selectNotebookEntries`,
> `maybeAutoInject`, `Compact`, and the feedback signals that tell us
> whether the notebook is keeping the right things.

## Problem

Selection (`agent/notebook_selection.go:30`) is structural:
last-two-segments → pinned files → prompt/todo refs → newest-first
fill under `maxNotebookInjectionTokens` (12K). Three quality gaps:

1. **Relevance is string-matching only — and thin.** Pass 2 matches
   basenames derived from `extractExplicitFilePaths`, which requires
   at least one slash — a bare "fix auth.go" produces _no ref at
   all_. It also misses the session's actual working set (files
   touched but not named) and has substring false positives
   (`auth.go` matches `oauth.go` via `entryMatchesRefs`).
2. **No staleness awareness at entry level.** Entries tagged to
   files since deleted or renamed compete equally with live files.
   `dropSupersededReads` handles same-file supersession
   (success-aware, line 194) but not file death.
3. **No sufficiency signal.** If entries lose needed detail, the
   symptom is `result:` recall and re-view pressure — the raw
   material exists (a `slog.Debug` at recall.go:57) but Debug is
   off at the default log level (`log.go:33-36`), so nothing is
   observable in production. The layer has no feedback loop.

Plus one robustness gap:

4. **Compaction can silently stall.** A PreCompact hook that denies
   forever leaves the notebook unbounded — prompt-safe (the 12K
   injection cap holds) but silently growing in the DB with no
   user-visible warning. Post-segments it's worse: `Compact` runs
   per segment commit (`segments.go:262`), so denials accrue per
   ~15K-token segment, not per turn. Same failure mode without any
   hook: when every remaining entry at a level is pinned,
   `compactOldestToLevel` makes no progress and `Compact` returns
   nil (retrieve.go:288-291).

## What exists

- `selectNotebookEntries`: four-pass selection + chronological
  return; `dropSupersededReads` is success-aware (line 194).
- `PinnedFileTagsSince` (`notebook/retrieve.go:158`): segment-
  grained pin set — files with edit entries at/after the recency
  floor. Already a crude working-set detector. (The turn-grained
  `PinnedFileTags` variant backs `Compact`'s pin skip.)
- `filetracker.ListReadFiles` (`internal/filetracker/service.go:77`):
  full paths of files read this session — resolves `file:` basenames
  to real paths (tags store basenames only). Edit/write/multiedit
  also `RecordRead`, so the set covers reads _and_ tool writes —
  but not files touched only via bash (`sed`, `rm`, `git
checkout`). **Cumulative**: it grows monotonically for the life
  of the session.
- `prefixFingerprint` (`agent/notebook_segments.go:707`): the
  rendered prefix is cached on `(boundary, bKey, entries, refs)`.
  **Any new selection input must join this hash or the cache
  serves stale renders** — and anything hashed is computed _per
  step_ (before the cache check), not per render.
- `stubStats` (`AgentOptions.StubStats`, agent.go:314): the
  existing precedent for sharing a per-session counter map from
  the coordinator into the agent.
- `recall` granularity lags coverage granularity: `turn:` exists,
  no `segment:` query (`searchNotebook`, recall.go:160-180).

## Proposal

### PR 0: Sufficiency instrumentation (before any tuning)

- Signals, counted per session — _split by recall type_:
  - Entry recall (tag/turn/text queries — hits the notebook):
    this layer's sufficiency signal.
  - `result:` recall — _not a notebook signal._ It searches the
    raw message list for a tool call ID (`recall.go:125-156`) and
    exists because _stubbing_ removed content — TOOL_RESULT_PRUNING's
    domain. High `result:` recall can mean thin entries, aggressive
    stubs, or a model correctly following a stub's pointer
    (designed behavior). Count it, but interpret against the stub
    track's metrics, not this one's.
  - Plumbing: `recallContext` has no path back to the agent —
    thread a shared counter map through `notebooktools.Build` into
    `NewRecallTool`, same pattern as `StubStats` in `AgentOptions`.
  - Re-views of stubbed files (expected — cheap) and re-views of
    files with _injected_ entries (signal). The view tool can't see
    the selection set — compute this agent-side at step time, where
    both the message list and the rendered selection are in scope:
    join new `view` calls in the step's messages against the files
    the last prefix render injected.
  - Selection-diff per pass (which pass contributed each rendered
    entry) — instrument `trySelect` to record its contributing
    pass.
- Aggregate alongside `stubStats`; emit in the step-composition
  log line / telemetry the same way.
- Optional, same PR: add `segment:` recall syntax so recall
  granularity matches coverage granularity.
- Interpretation rules: high _entry_ recall on covered entries ⇒
  entries too thin or selection wrong; high re-view of
  entry-covered files ⇒ entry insufficient; high `result:` recall
  alone ⇒ look at the stub track first; low everything ⇒ layer is
  working.
- Lands first: it is the tuning instrument for everything below.

### PR 1: Working-set relevance

- Plumb `filetracker` into `sessionAgent` (coordinator already
  holds `c.filetracker`).
- New pass between ref-matching and newest-fill: entries tagged to
  the session working set outrank pure recency. The working set is
  `ListReadFiles` alone — the pin set is already a subset (every
  edit tool `RecordRead`s its file: edit.go:182,273,
  multiedit.go:232, write.go:164); a union only covers
  `RecordRead` failure.
- **Bound and order by recency.** The working set is cumulative —
  in a long session it degenerates to "every file ever touched."
  Iterate this pass newest-first and cap the set (files touched in
  the last N segments, or weight by last-touch). Otherwise a
  30-segments-ago working-set entry can starve a 4-segments-ago
  relevant entry that would have won the fill pass — the failure
  mode is mid-recency starvation, not just noise.
- Resolve `file:` basename tags to full paths via `ListReadFiles`.
  Basename tags can't self-disambiguate; on collision (two
  `auth.go`s both tracked), prefer entries whose `EntryTextFull`
  contains the tracked full path (always populated —
  `storeEntry` writes it `Valid: true`, classify.go:328).
- Refs that don't resolve to a tracked path keep the existing
  substring match — working-set resolution kills false positives
  only where it applies.
- **Add the working set to `prefixFingerprint`** — it grows
  mid-turn without touching entries/refs/boundary. Cost: one DB
  query per step, consistent with `GetEntries` already running
  per step.
- Verify: selection-diff test — entry for a touched-but-unnamed
  file beats a newer unrelated entry, _and_ a large-working-set
  test so the pass doesn't starve recency.

### PR 2: Type weighting for old entries

- Pass 3 stays newest-first for entries within a recency band
  (~last N _segments_ — the recency unit here; a turn can hold
  many segments). Recent is likely relevant regardless of type.
- Beyond the band, fill order becomes type rank then recency:
  `file_edit` > `command` > `file_read` > `exploration` >
  `general` (last: default bucket for MCP/unknown tools —
  least predictable content). `decision` is deliberately not
  first: `hasDecision` is a keyword heuristic (classify.go:235-248)
  — "I chose to run the tests first" produces a decision entry —
  so top-ranking it amplifies the noisiest signal over entries
  grounded in real tool events. Rank it mid-list, or tighten the
  heuristic before promoting it.
- A 20-segments-old file edit outranks a 6-segments-old command
  result.
- Risk: type bias evicting a load-bearing recent entry — mitigated
  by the recency band. Measure selection-diff churn before/after.

### PR 3: Dead-reference demotion

- Depends on PR 1 (uses its filetracker plumbing).
- Entries whose `file:` tags resolve (via `ListReadFiles` paths)
  only to files that no longer exist (`os.Stat`) demote below
  live-file entries in the fill pass.
- Unresolvable tags (basename never tracked — e.g. bash-only
  files) default to **live**: absence of tracking data is not
  evidence of death. Demote only when ≥1 tracked path resolves
  and all resolved paths fail `os.Stat`.
- Scope: `selectNotebookEntries` only — **not** `maybeAutoInject`.
  Auto-inject is user-explicit (they named the file in the current
  message); history for a deleted file they asked about is
  arguably what they want, not the phantom-state hazard.
- **Demote, never drop** — "why was X removed" is legitimate
  history. Deletion itself surfaces as a `command` entry (`rm` via
  bash), which is untagged and unaffected either way.
- **Add the stat results to `prefixFingerprint`** — a deletion
  changes no existing input, so without it the cached prefix keeps
  rendering the dead file's entries as live. Cost is per step, not
  per render — scope the stats to tags present in candidate
  entries, not every tracked file.
- Verify: unit test — deleted-file entry ranks below same-age
  live-file entry, still selectable when budget allows.

### PR 4: Compaction robustness — bounded stalls

- Count consecutive denied _rounds_ per session (per-segment
  `Compact` calls — a streak of ~3 can accrue within a single
  turn). New per-session state in the notebook service. Reset the
  counter on any round that makes progress — a session
  alternating stall/progress shouldn't accumulate warnings.
- After ~3 consecutive stalls, surface a persistent UI warning via
  pubsub ("compaction permanently blocked; notebook DB grows
  unbounded — prompt unaffected").
- Same check for pin-stall: `Compact` runs, stays over budget, and
  no entry compresses — same warning, different reason string.
  Implementation note: `compactOldestToLevel` returns nil both on
  under-budget success and on stall (retrieve.go:290) — detecting
  "no progress" needs it to report a compressed count (signature
  change) or a before/after token diff.
- **Never override a deny or a pin** — both are user intent. Warn
  loudly, don't act silently.
- Rejected alternative: hard-cap notebook DB growth — silent data
  loss is worse than a warning.

### PR 5: Feed `ErrorHeadline` into generation input

- The field already exists on `EntryInput` and is populated
  (classify.go:63); the generator prompt just never emits it —
  `generator.go:80-91` writes Type/Title/Description only. The
  change is ~3 lines in that loop; no schema work.
- Value: the failure _signal_ already reaches the prompt —
  `describeToolCall` prefixes errors with `[ERROR]` and includes
  the first 2000 chars (classify.go:167-169). The headline adds
  the distilled first line plus the `Exit code N` tail that
  `errorHeadline` scans from the _full_ result (classify.go:283-294)
  past the truncation cut. A digest guaranteed to survive
  truncation — the marginal gain is precision, not detection.

## Measurement

Selection-diff per pass (PR 0 plumbing), entry-recall hit rate,
covered-file re-view rate, deny/pin-stall warnings. Success:
covered-file re-view rate trends down as PRs 1–3 land; entry
recall rate stays flat or drops.

## Non-goals

- Raw-window mechanics — `TOOL_RESULT_PRUNING.md`.
- Budget policy and overflow — `CONTEXT_WINDOW_SAFETY.md`.
- mem0 cross-session semantics — sync shape stays as-is.
- Worker/execution sub-agents — separate track.

## Risks

- **Working-set degeneration**: the set is cumulative; without the
  recency bound, PR 1 decays into a second chronological pass.
  Mitigated by newest-first iteration + last-touch cap.
- **Weighting overfits to tag noise**: basenames collide across
  directories; path resolution via filetracker plus full-path
  preference in `EntryTextFull` mitigates.
- **Decision rank bets on heuristic precision**: `hasDecision` is
  keyword-matching; promoting `decision` past grounded types
  amplifies noise. Mitigated by mid-list rank or tightening the
  heuristic first.
- **Cache staleness**: every new selection input (working set,
  stat results) must join `prefixFingerprint` — the fingerprint,
  not the render, is where freshness is decided. Corollary:
  fingerprint inputs are computed per step — keep them cheap and
  scoped (stat candidate tags only, not all tracked files).
- **Feedback misread**: entry recall pressure can mean "entries
  thin" OR "selection wrong" — the selection-diff data
  disambiguates which. `result:` recall is a stub-track signal,
  not a notebook one — keep it out of this layer's interpretation.
