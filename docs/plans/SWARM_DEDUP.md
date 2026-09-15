# Swarm Dispatch — Dedup & Retention for Subagent Results

> **Status:** Plan draft (v2 — depends on `BACKGROUND_SUBAGENTS.md`
> v1: the `background` param, the agent registry, the completion
> notification seam, and `read_subagent`). Scope: treating a subagent
> dispatch as a tool call inside the existing retention framework —
> a dispatch ledger, a dedup cache, and pressure-aware admission.
> Multi-agent orchestration itself stays out; the parent model still
> decides what to ask.

## Problem

Once background subagents exist, the parent can fan out research
freely — and can re-ask. Two failure modes follow:

- **Redundant dispatch.** The model asks a near-duplicate of a
  question it already dispatched three turns ago ("what does
  auth.go do" vs. "explain auth.go's structure"). A subagent run is
  the most expensive tool call in the system — fresh context, cold
  prompt cache, full tool-call loop. Re-asking is the single worst
  cost mistake available.
- **Ephemeral answers.** The completion notification is consumed
  once; the summary it carries is not durable. If the conclusion
  needs to survive compaction, nothing currently guarantees it.

Both constraints — dedup and no-raw-dump — point at the same move:
model the dispatch as a tool call with its own reproducibility
class inside the existing retention machinery, not as a separate
subsystem.

## What exists

| Machinery | Status | Location |
|---|---|---|
| Significant/trivial classification; `agent`/`read_subagent` already classify significant by default | Exists | `notebook/classify.go:23-39` |
| `result:<tool_call_id>` recall pointers for stubbed parent-session results | Exists | `tools/notebook/recall.go:107` |
| Entry cap ~1000 tokens, small-model generation, tag metadata | Exists | `notebook/notebook.go:39`, `classify.go:415` |
| Observed-mutation supersession — `FileMtime` stamped on read results, re-stat'd at flag time | Exists | `agent/stubs.go:276-315`, `:429` |
| Child-session read set recorded (view calls `RecordRead` under the child session id) | Exists | `filetracker.ListReadFiles(childSessionID)` |
| Duplicate/rerun supersession by canonical input | Exists | `agent/stubs.go:317-371` |
| SQLite store, per-session tables | Exists | `internal/db/` |
| In-memory run-state registry with `running|done|error` | Planned (v1) | `BACKGROUND_SUBAGENTS.md` §1 |

Note the correction from review: `filetracker` records read
*timestamps*, not mtimes (`TOOL_RESULT_PRUNING.md` — "filetracker
wasn't the vehicle"). The mutation signal is the per-result
`FileMtime` stamp, which child sessions already carry because task
agents share the notebook/stub machinery.

## Design

### Core property: a new tool class, not a new architecture

Swarm dispatch joins the retention table:

| Artifact | Reproducible? | Policy |
|---|---|---|
| `view`/`edit`/`bash`/`grep` results | (existing rows) | (existing policies) |
| swarm subagent result | **Conditionally** — same query + unchanged state ≈ same answer, but not guaranteed | Stub after boundary move; cache-checked before re-dispatch |

"Conditionally reproducible" is the key design fact: an LLM
subagent isn't deterministic like `grep`, but if nothing the query
depends on changed, re-running it is redundant cost for a
near-identical answer. So the rule isn't "cache because
deterministic" — it's **"cache with an invalidation signal,
because it's stable, not deterministic."**

### 1. Dispatch ledger — the registry, persisted

The dedup question "have I asked this" must be a metadata lookup,
not a context scan — an in-context answer is subject to the same
compaction loss as everything else. Maintain a ledger in SQLite:

```
{fingerprint, query_text, role, child_session_id, status,
 dispatched_at, files_touched[], read_mtimes[], succeeded}
```

This is the v1 `backgroundAgentRegistry` grown a fingerprint
column and a table — one structure, not two. `status` includes
`running`, which is what makes **in-flight dedup** possible: a
fan-out of five near-identical queries in a single turn is the
common redundant-dispatch case, and a completed-only cache misses
all five. A dispatch matching a `running` fingerprint attaches to
the pending result (or returns "already running as `<id>`; use
`read_subagent`").

Ledger rows persist across restarts — the registry doesn't — and
stay valid because they point at child *sessions*, which are
durable. `read_subagent`'s interrupted path already covers
post-restart reads.

Scope: **per parent session**. Cross-session dedup interacts with
mem0 and is a separate decision.

### 2. Dedup — two-phase, not a single hash

The lookup key cannot contain state — you don't know which files a
query depends on until a subagent runs and reads them. So:

- **Lookup** key: `(normalized_query, role)` → candidate entries.
  v1 normalization is lowercase/whitespace/punctuation canonicalization
  plus token-Jaccard similarity over prior `query_text`s — zero new
  dependencies. Embedding-based near-duplicate matching is a later
  upgrade measured against this baseline; nothing in `internal/` does
  embeddings today, and a provider embedding call per dispatch
  decision is a real new cost, not "≈ zero".
- **Validate** each candidate: re-stat its recorded
  `files_touched` mtimes (the same check pass-2 supersession runs
  on `FileMtime`). Unchanged → cache hit, return the stored summary,
  no agent spawned. Changed → miss, dispatch fresh.

**Search-derived answers under-cover.** `files_touched` only
records `view` reads; grep/glob/ls answers depend on every file
matching the pattern, including files created *after* dispatch. A
new `auth_test.go` invalidates "which tests cover auth" without
touching any recorded read. Handling: ledger entries also record
the dispatch's search patterns, and search-derived results
invalidate on **any successful write in the parent session since
dispatch** — coarse but sound. (Refinement: a workspace-wide
mtime/file-list fingerprint if the coarse rule proves too eager.)

**Cache-hit semantics.** A hit on `background=true` returns the
cached summary synchronously, foreground-style — no spawn, no
notification — and the response self-labels: which dispatch it was
cached from, that files were verified unchanged, and the
`read_subagent` id for the full trace. Applies to foreground `agent`
calls identically; dedup is a property of dispatch, not of
background mode.

**Success-only caching.** Only complete, non-error subagent
results enter the ledger as satisfiable entries — same invariant
as success-only supersession. A failed or interrupted run must
never satisfy a later identical query.

### 3. Result routing — notification, entry, and pointers

Three channels, three lifetimes — no new pointer types:

- **Notification** (v1) is the *interrupt*: ~200-char summary,
  consumed once. Ephemeral by design.
- **Notebook entry** is the *durable artifact*: every `agent` and
  `read_subagent` call already classifies significant → small-model
  entry, tagged `#swarm:<role>` `#dispatch:<child_session_id>`,
  capped at the existing ~1000-token limit. Nothing new enters the
  main context beyond what the notebook already renders; the entry
  is what survives compaction and notification consumption.
- **Recall** already covers both layers: `result:<tool_call_id>`
  for the parent's stored tool result; `read_subagent(child_id)`
  for the child's full trace. No `result:<swarm_call_hash>` — the
  cache key is not a pointer; the two existing schemes address
  both artifacts.

For the raw-window side, add `read_subagent` (and the `agent`
tool's result) to the stub machinery's coverage — today neither is
in a tool-name set, so their results never stub. Adding
`read_subagent` to `commandToolNames` gets pass-3 dedup free:
identical re-reads group by canonical input and flag duplicates.
Note the division of labor: stubs already prevent the *context*
cost of a repeated read; the cache prevents the *run* cost of a
repeated dispatch. Different savings — don't conflate them.

### 4. Pressure-aware admission — denial, not a router

v1's non-goal stands for the default path: the parent model decides
when to spawn; no small-model router second-guesses each dispatch
(a model call per `agent` invocation to review a decision the main
model just made is bad economics). What pressure adds is
*admission control* on top of the existing `maxBackgroundAgents`
cap — the same ladder shape as the pruning merge point:

| Swarm budget pressure | Behavior |
|---|---|
| Low | Fan out freely |
| Medium | Cache-first (hits preferred); `background=true` spawns require the ledger to show no plausible match |
| High | Deny new background spawns with a clear tool error ("at capacity; answer from cache or inline"); the error message is the batching hint — the model can fold subtasks into fewer, broader dispatches |

This keeps the decision in the tool callback — zero added model
calls — and the fan-out-vs-single-agent judgment in `agent_tool.md`
guidance where v1 already puts it. If measurements later show the
model over-fanning despite guidance, *then* a router earns its
call; that's the amendment trigger for the v1 non-goal, decided by
data.

### 5. Same invariants, extended

- **Stub beats drop** → cached-hit beats redispatch; a cache miss
  still gets a full dispatch. Dedup is the stub of the swarm layer,
  never a silent drop of a genuinely needed subtask.
- **No dead pointers** → a ledger entry must not outlive the state
  it describes; the `files_touched` + search-space invalidation is
  what proves it's still current.
- **Only success supersedes** → only successful, complete subagent
  returns satisfy cache lookups.
- **Everything self-labels** → every swarm-sourced response and
  notebook entry states which dispatch, cached vs. fresh, and where
  to recall the trace.

## What's genuinely new (not reused)

- The dispatch ledger table + fingerprint column on the registry.
- Two-phase dedup: normalized-query/Jaccard lookup, mtime
  validation. (Embedding similarity is the measured-later upgrade.)
- In-flight dedup via `running` ledger status.
- Search-space invalidation for non-view-derived answers.
- Pressure-based spawn admission (denial), beyond the static cap.

## Measurement

Parallel to `stubStats` and the eval-harness run record:

- **Dedup hit rate** — cache hits / dispatch decisions.
- **In-flight joins** — fan-out duplicates caught by `running`
  entries.
- **False-merge events** — a hit later invalidated or followed by a
  fresh dispatch of the same query (the threshold-tuning signal:
  false merges are *worse* than redundant dispatches).
- **Swarm-dispatch cost per task** — tokens spent on subagent runs,
  split fresh vs. cached-served.
- **Admission denials** — how often pressure gating fires, and
  whether the model batches or drops.

The gate question is empirical: does the workload actually repeat
sub-asks? If hit rate ≈ 0 on real sessions, the Jaccard layer is
enough and embeddings stay out.

## Non-goals

- Swarm orchestration, planning across agents, auto-delegation —
  the parent decides what to ask (v1 non-goal, unchanged).
- Cross-session dedup (mem0 interaction is a separate design).
- Edit-capable subagents, nested background spawns — inherited from
  v1 non-goals.
- Embedding-based similarity (deferred pending hit-rate data).
- A fan-out router model (deferred; admission denial covers the
  pressure case).

## Risks

- **False merge** — two genuinely different questions treated as
  the same cached answer is worse than a redundant dispatch.
  Mitigation: conservative Jaccard threshold, role in the key,
  fingerprint validation, self-labeled hits the model can override
  (`read_subagent` or re-dispatch with a clarifier). Metric:
  false-merge events.
- **Stale-answer under-coverage** — search-derived results
  invalidated only by the coarse any-write rule; a query whose true
  dependency set wasn't recorded can serve stale. Mitigation: the
  rule is deliberately conservative; false *misses* (over-
  invalidation) are the acceptable error direction.
- **Ledger/registry split-brain** — if the in-memory registry and
  the persisted ledger disagree (crash between transitions), the
  ledger is authoritative for dedup and `read_subagent`'s
  session-derived status is authoritative for results. One
  structure, one write path — the registry becomes a read cache of
  the ledger, not a parallel truth.
- **Cache-hit semantics surprise** — a synchronous return on a
  `background=true` call must be unambiguous in the response text,
  or the model waits for a notification that never comes.
