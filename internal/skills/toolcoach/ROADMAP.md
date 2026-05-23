# Toolcoach Roadmap — Phased Implementation Plan

## Executive Summary

This document prioritizes all brainstormed coach enhancements into four phases,
from urgent perf/precision fixes to deferred research features. Each phase
includes success criteria, risk flags, and estimated effort.

---

## Priority Ranking (Urgent → Deferred)

| Rank | Idea | Phase | Effort | Impact | Risk |
|------|------|-------|--------|--------|------|
| 1 | **Measurement infrastructure** | 0 | Small | Critical | None |
| 2 | **Zero-allocation JSON peek** | 1a | Small | High | Low |
| 3 | **Pre-compiled regex pool** | 1a | Tiny | High | None |
| 4 | **Semantic edit validation** | 1b | Small | High | Low |
| 5 | **Time-since-last-view threshold** | 1b | Tiny | Medium | None |
| 6 | **Pattern ordering by hit frequency** | 1b | Small | Medium | Low |
| 7 | **Coach feeds critic context** | 2 | Medium | High | Medium |
| 8 | **Pattern effectiveness SQLite DB** | 2 | Medium | High | Low |
| 9 | **Success/failure tracking per pattern** | 3 | Medium | Medium | Low |
| 10 | **Progressive coaching** | 3 | Small | Medium | Low |
| 11 | **Tip → guided retry** | 3 | Large | High | High |
| 12 | **Project-language awareness** | 4 | Medium | Low | Low |
| 13 | **Command-type-aware bash coaching** | 4 | Small | Low | None |
| 14 | **Session archetype detection** | 4 | Large | Medium | High |
| 15 | **WASM pattern plugins** | 4 | Large | Medium | High |
| 16 | **ML-based pattern ranking** | 4 | Very Large | Unknown | Very High |

---

## Phase 0: Telemetry & Measurement (Prerequisite)

**Goal**: You cannot optimize what you do not measure. Before touching any
heuristics, build a lightweight telemetry pipeline that proves the coach is
working and shows where it fails.

### Tasks

1. **Per-session metrics struct** (`coach_metrics.go`)
   - `toolCallsCoached` — how many calls were analyzed
   - `patternsFired` — per-pattern fire counts
   - `patternsIgnored` — tips the agent did not act on
   - `avgDelayMicros` — p50/p95/p99 of coach delay
   - `totalCoachTimeMs` — cumulative overhead

2. **In-memory metrics exporter**
   - Export metrics at session end via `event` package
   - Tags: `session_id`, `pattern_id`, `was_actually_helpful` (heuristic guess)

3. **Performance regression test**
   - Add `BenchmarkRunCoach` to CI with a threshold gate
   - Fail CI if `runCoach()` p95 > 10µs over 1000 iterations

### Success Criteria
- [ ] Every session emits a `toolcoach.session_summary` event
- [ ] CI blocks PRs that regress benchmark > 10%
- [ ] We can answer: "Which pattern has the highest false-positive rate?"

### Effort: 1 dev-day

---

## Phase 1a: Core Performance (Sprint 1)

**Goal**: Drive `runCoach()` from ~2.7µs to **< 1µs** for the common (no-match)
path. These are pure code-quality changes with zero behavior change.

### Tasks

1. **Zero-allocation JSON peek** (`jsonpeek.go`)
   - Implement `peekFilePath(input []byte) string` that scans raw bytes for
     `"file_path":"..."` without allocating a struct
   - Use `bytes.Index` + manual slice extraction
   - Fallback to `json.Unmarshal` only if the fast path fails

2. **Pre-compiled regex pool** (`patterns.go` refactor)
   - Move all `regexp.MustCompile` calls to `init()` or package vars
   - Replace inline `regexp.MatchString` with pre-compiled pattern vars

3. **Fast-path short-circuit**
   - Before any JSON parsing, check if `toolName` is in a tiny map of
     "tools that have no patterns". Tools like `crush_info`, `todos`, `ls`
     skip all logic instantly.

### Success Criteria
- [ ] `BenchmarkRunCoach` shows p50 < 1µs for no-match path
- [ ] `go test -bench=BenchmarkRunCoach -benchmem` shows **0 allocs/op**
- [ ] All existing tests pass without modification

### Effort: 1–2 dev-days

---

## Phase 1b: Precision Improvements (Sprint 2)

**Goal**: Cut false positives by 50% without adding latency. These are
smarter heuristics, not more heuristics.

### Tasks

1. **Semantic edit validation** (`patterns.go`)
   - For `edit_without_view`, cache recently read file contents in a small
     LRU (max 50 files, max 4KB each)
   - If `old_string` is found in the cache, the agent already knows the file;
     skip the hint
   - Cache is populated by `view` tool results, not disk reads

2. **Time-since-last-view threshold** (`patterns.go`)
   - `repeated_view` only fires if `time.Since(lastView) > 30s`
   - Immediate re-views after edits are legitimate

3. **Pattern ordering by hit frequency** (`coach.go`)
   - Add a package-level `atomic.Uint64` counter per pattern ID
   - On every match, increment the counter
   - `defaultPatterns` is reordered every N calls (e.g., 1000) so hot
     patterns are checked first
   - Use `sort.Slice` with a copy of the slice (avoid mutating the original
     while iterating)

4. **Input fingerprint cache** (`coach.go`)
   - LRU cache of `hash(toolName+input) → *coachResult`
   - TTL = 5s (tool inputs rarely repeat beyond a single turn)
   - Size = 128 entries (tiny, fixed array to avoid map growth)

### Success Criteria
- [ ] `repeated_view` false-positive rate < 5% (measured via Phase 0 metrics)
- [ ] `edit_without_view` false-positive rate < 10%
- [ ] No benchmark regression > 5%

### Effort: 2–3 dev-days

---

## Phase 2: Critic Integration & Learning (Sprint 3)

**Goal**: Close the loop between coach and critic so they share context and
improve each other.

### Tasks

1. **Coach feeds critic context** (`middleware.go` + `internal/skills/critic/`)
   - Before critic review, serialize the session's `coachMetrics` into a
     compact string: `"Coach observed: 3 edit_without_view tips, 1 ignored,
     1 broad_grep."`
   - Inject this as a `System` message before the critic prompt
   - Critic prompt template updated to reference coach observations

2. **Pattern effectiveness SQLite DB** (`store.go`)
   - New table: `toolcoach_effectiveness`
     - `pattern_id TEXT`
     - `session_id TEXT`
     - `fired_at TIMESTAMP`
     - `agent_acted BOOLEAN` (did the next tool call follow the hint?)
     - `critic_verdict TEXT` (if critic reviewed this turn)
   - Queried at session start to set initial `MaxPatternsPerTurn`:
     - High effectiveness → raise limit to 5
     - Low effectiveness → lower limit to 1

3. **Adaptive severity** (`coach.go`)
   - If a pattern's 7-day effectiveness score < 30%, downgrade its severity
     (e.g., `warning` → `hint`)
   - If score > 80%, upgrade severity (e.g., `hint` → `warning`)

### Success Criteria
- [ ] Critic prompt includes coach context when toolcoach is enabled
- [ ] SQLite schema migration is reversible (goose)
- [ ] `MaxPatternsPerTurn` auto-adjusts based on historical effectiveness

### Effort: 3–4 dev-days

---

## Phase 3: Steering & Active Correction (Sprint 4)

**Goal**: The coach evolves from "advisor" to "active co-pilot" that can
intervene and guide retries.

### Tasks

1. **Success/failure tracking per pattern** (`coach.go`)
   - Define "success" heuristically:
     - `edit_without_view` → success if next tool is `view` on same file
     - `broad_grep` → success if next `grep` has a longer pattern
     - `missing_multiedit` → success if next tool is `multiedit`
   - Track in session state; emit at session end

2. **Progressive coaching** (`config.go`)
   - Add `CoachingIntensity` enum: `tutor`, `balanced`, `minimal`
   - `tutor`: all hints, verbose explanations
   - `balanced`: warnings + criticals only after session 3
   - `minimal`: only criticals
   - Default = `tutor` for first 3 sessions, then auto-switch to `balanced`

3. **Tip → guided retry** (`coached_tool.go`)
   - For `broad_grep`, the coach can suggest a better pattern and
     **automatically retry** the tool with the improved input
   - Only if `cfg.AutoRetry == true` (default false)
   - Max 1 auto-retry per turn to avoid loops
   - Requires `coachedTool` to have a reference to the tool's inner handler

### Success Criteria
- [ ] Auto-retry reduces `broad_grep` false negatives by 30%
- [ ] Progressive coaching suppresses > 50% of hints for experienced users
- [ ] No infinite retry loops (hard cap + test)

### Effort: 4–5 dev-days

---

## Phase 4: Advanced Research (Deferred)

**Goal**: Explore high-risk, high-reward features. Only start after Phases 0–3
are stable and metrics prove value.

| Feature | Why Deferred | Precondition |
|---------|--------------|--------------|
| **Project-language awareness** | Requires file-system scanning at startup; adds complexity for marginal gain | Phase 1b precision still insufficient |
| **Command-type-aware bash coaching** | Bash parsing is hard; `shellcheck`-level analysis is a rabbit hole | Phase 3 auto-retry proves viable |
| **Session archetype detection** | Needs clustering/ML on tool sequences; high false-positive risk | 10K+ session metrics in DB |
| **WASM pattern plugins** | Sandboxing overhead likely > 100µs; defeats perf goals | Coach latency stabilized < 1µs |
| **ML-based pattern ranking** | Requires training data pipeline; massive infra cost | Pattern effectiveness DB has 6+ months of data |

---

## Self-Critic of This Plan

### Original Flaws

1. **No measurement first** — The initial brainstorm jumped straight to
   optimizations without defining how to measure success. Phase 0 was added
   explicitly to fix this.

2. **Phase 2 was too ambitious** — The original plan put "Coach as pre-critic
   gate" (halt destructive commands before critic runs) in Phase 2. Self-critic
   revealed this conflates two responsibilities: coaching (advisory) and gating
   (authoritative). A gate that halts turns needs rigorous safety review and
   audit logging. Moved to Phase 3 as "Tip → guided retry" which is softer.

3. **SQLite schema risk** — Phase 2 adds a DB table. If the schema is wrong,
   migrations are painful. Self-critic added: migration must be reversible, and
   the table should be `WITHOUT ROWID` with a composite PK for fast lookups.

4. **Zero-allocation fragility** — Hand-rolled JSON parsing is fast but breaks
   if tool schemas change. Self-critic added: fallback to `json.Unmarshal` with
   a debug log so failures are visible, not silent.

5. **Pattern reordering race** — Reordering `defaultPatterns` while
   `coachedTool.Run` iterates is a data race. Self-critic added: reorder a
   copy, then swap the pointer atomically.

### Dependencies & Ordering Constraints

```
Phase 0 ──→ Phase 1a ──→ Phase 1b ──→ Phase 2 ──→ Phase 3 ──→ Phase 4
   │           │            │            │            │            │
   │           │            │            │            │            └─ Needs 6mo data
   │           │            │            │            └─ Needs Phase 2 DB
   │           │            │            └─ Needs Phase 1b stable
   │           │            └─ Needs Phase 1a perf baseline
   │           └─ Needs Phase 0 benchmark CI
   └─ No prerequisites
```

### Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Hand-rolled JSON parser breaks | Always fallback to stdlib; log fallback rate |
| Pattern reordering causes skew | A/B test reordering on 10% of sessions |
| SQLite write slows coach | Write async to a buffered channel; never block tool path |
| Critic context bloats prompt | Coach context capped at 200 chars; truncated aggressively |
| Auto-retry creates loops | Hard cap of 1 retry; exponential backoff; never retry a retry |

---

## Improved Plan Summary

```
Week 1: Phase 0  → Telemetry + benchmark CI
Week 2: Phase 1a → Zero-allocation + pre-compiled regex + fast path
Week 3: Phase 1b → Smarter heuristics (semantic edit, time threshold, ordering)
Week 4: Phase 2  → Critic context + effectiveness DB + adaptive severity
Week 5–6: Phase 3 → Progressive coaching + success tracking + guided retry
Ongoing: Phase 4  → Research spikes as metrics justify them
```

**Recommended immediate action**: Implement Phase 0 (measurement) + Phase 1a
(perf) together. They are small, independent, and prove the foundation is solid
before adding complexity.
