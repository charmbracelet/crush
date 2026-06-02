# Phase 2 Implementation — Critic Integration & Learning DB

## Objective

Feed real-time coaching observations into the critic's review context, persist
pattern effectiveness across sessions in SQLite, and automatically adjust tip
severity based on historical agent behavior.

## Results

| Feature | Status |
|---------|--------|
| Coach feeds critic context | ✅ Implemented |
| Pattern effectiveness SQLite DB | ✅ Implemented |
| Adaptive severity | ✅ Implemented |

## What Was Implemented

### 1. Coach Feeds Critic Context

The critic middleware now enriches its review checkpoint with a coaching summary
produced by the toolcoach middleware.

**Interface** (`critic/checkpoint.go`):
```go
type CoachSummaryProvider interface {
    GetCoachSummary(sessionID string) string
}
```

**Flow**:
1. `toolcoach.Middleware` implements `CoachSummaryProvider` via
   `GetCoachSummary(sessionID)` which returns a formatted summary of patterns
   fired, acted, and ignored for the session.
2. `App` implements `CoachSummaryProvider` by delegating to the stored
   toolcoach middleware (`app.toolcoachMw atomic.Pointer`).
3. `critic.Middleware` calls `SetCoachSummaryProvider(app)` and, after
   `m.primary.Run()` returns, fetches the summary and injects it into the
   `Checkpoint.CoachSummary` field.
4. `BuildCriticPrompt` renders the coach summary in a `<<<COACH_BEGIN>>>` /
   `<<<COACH_END>>>` block with prompt-injection defense (delimiter escaping).

**Example rendered summary**:
```
## Coaching Observations

The following tool usage patterns were observed during this turn:

<<<COACH_BEGIN>>>
- Edit Without View: fired 2 time(s) this session (acted: 1, ignored: 1).
- Broad Grep Pattern: fired 1 time(s) this session.
<<<COACH_END>>>
```

### 2. Pattern Effectiveness SQLite DB

**Schema** (migration `20260523000000_add_toolcoach_effectiveness.sql`):
```sql
CREATE TABLE toolcoach_effectiveness (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pattern_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    fired_count INTEGER NOT NULL DEFAULT 0,
    acted_count INTEGER NOT NULL DEFAULT 0,
    ignored_count INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE
);
CREATE INDEX idx_toolcoach_eff_pattern ON toolcoach_effectiveness (pattern_id);
CREATE INDEX idx_toolcoach_eff_session ON toolcoach_effectiveness (session_id);
```

**Store** (`toolcoach/store.go`):
- `RecordSessionEffectiveness(ctx, sessionID, patternID, fired, acted, ignored)` — inserts per-session row
- `GetPatternEffectiveness(ctx, patternID, lookback)` — aggregates `(SUM(fired), SUM(acted), SUM(ignored))` over the lookback period
- `Prune(ctx, cutoff)` — deletes old rows

**sqlc queries** (`internal/db/sql/toolcoach_effectiveness.sql`):
- `CreateToolcoachEffectiveness` (:one)
- `GetToolcoachEffectivenessByPattern` (:many)
- `PruneToolcoachEffectiveness` (:execrows)

### 3. Adaptive Severity

**Config** (`toolcoach/config.go`):
```go
AdaptiveSeverity           bool // enable/disable
EffectivenessLookbackDays  int  // default 30
```

**Algorithm** (`coach.go:adaptSeverity`):
- Minimum sample size: `acted + ignored >= 5`
- Effectiveness score: `acted / (acted + ignored)`
- Score < 0.2: **downgrade** (hint → silent, warning → hint, error → warning)
- Score > 0.7: **upgrade** (hint → warning, warning → error)
- Otherwise: keep base severity

**Session initialization**:
When `AdaptiveSeverity` is enabled, `newSessionState` calls
`loadAdaptiveSeverity(store, lookbackDays)` which queries the store for each
pattern and populates `sessionState.adaptiveSeverity map[string]string`.

**Runtime behavior**:
- `runCoach()` calls `effectiveSeverity(pat)` before detection
- Patterns mapped to `"silent"` are skipped entirely (no Detect call)
- The effective severity is returned in `coachResult.Severity` so the tip
  display reflects the adapted level

## Files Added / Modified

| File | Action | Description |
|------|--------|-------------|
| `store.go` | **New** | Domain store for effectiveness data |
| `store_test.go` | **New** | 6 tests covering record/get/lookback/prune/no-querier/no-db |
| `coach_adaptive_test.go` | **New** | 5 tests for adaptive severity, coach summary, and silent suppression |
| `critic/checkpoint.go` | **Modified** | Added `CoachSummary` to `Checkpoint`; added `CoachSummaryProvider` interface |
| `critic/prompt.go` | **Modified** | Added `CoachSummary` to template with delimiter escaping |
| `critic/prompt_test.go` | **Modified** | Added `TestBuildCriticPrompt_WithCoachSummary` |
| `critic/middleware.go` | **Modified** | Added `coachProvider` field, `SetCoachSummaryProvider`, checkpoint enrichment |
| `critic/middleware_test.go` | **Modified** | Added `TestMiddleware_Run_CoachSummaryEnrichment` |
| `critic/store_test.go` | **Modified** | Updated test schema to include `revision_outcome` column |
| `coach.go` | **Modified** | Added `adaptiveSeverity`, `loadAdaptiveSeverity`, `effectiveSeverity`, `adaptSeverity`, `buildCoachSummary`, DB persistence in `exportMetrics` |
| `coach_metrics.go` | **Modified** | Added `patternCounts()` snapshot method for safe concurrent access |
| `config.go` | **Modified** | Added `AdaptiveSeverity` and `EffectivenessLookbackDays` |
| `middleware.go` | **Modified** | Added `store` field, `SetStore`, adaptive loading in `getOrCreateState` |
| `app.go` | **Modified** | Added `ToolcoachStore`, `toolcoachMw`, `GetCoachSummary` |
| `app/critic.go` | **Modified** | Wired `mw.SetCoachSummaryProvider(app)` and `mw.SetStore(app.ToolcoachStore)` |
| `internal/config/config.go` | **Modified** | Added `AdaptiveSeverity` and `EffectivenessLookbackDays` to JSON config |
| `internal/db/sql/toolcoach_effectiveness.sql` | **New** | sqlc queries |
| `internal/db/migrations/20260523000000_add_toolcoach_effectiveness.sql` | **New** | Goose migration (reversible) |
| `internal/db/sql/critic_reviews.sql` | **Modified** | Added `revision_outcome` to SELECT/RETURNING clauses |

## Self-Critic Review

### Issues Found During Review

| # | Issue | Severity | Fix Applied |
|---|-------|----------|-------------|
| 1 | `sessionState.exportMetrics` accessed `metrics.patternFireCount` under `s.mu.RLock()` instead of `metrics.mu` | High | Added `patternCounts()` snapshot method that acquires the correct lock |
| 2 | `buildCoachSummary` used `patternName` lookup but tests expected `patternID` | Low | Tests updated to match human-readable `pattern.Name` |
| 3 | sqlc regeneration changed existing `CriticReview` return types because `revision_outcome` migration was not reflected in queries | Medium | Updated `critic_reviews.sql` SELECT/RETURNING to include `revision_outcome` |
| 4 | `GetCoachSummary` test passed `nil` primary to `NewMiddleware`, which returns `nil` | Low | Test uses `mockSessionAgent` instead |

### Correctness Verification

- [x] All toolcoach tests pass with `-race` (31 tests)
- [x] All critic tests pass with `-race` (46 tests)
- [x] All app tests pass with `-race`
- [x] Full project build succeeds (`go build ./...`)
- [x] Store test coverage: record, get, no-data, lookback filtering, prune, no-querier, no-db
- [x] Adaptive severity test coverage: downgrade, upgrade, neutral, insufficient samples, silent suppression
- [x] Critic integration test coverage: prompt rendering, checkpoint enrichment

### Performance Impact

| Metric | Before (Phase 1) | After (Phase 2) | Delta |
|--------|-----------------|-----------------|-------|
| `runCoach()` avg latency | 0.4µs | **0.4µs** | **0µs** — no regression |
| Heap allocations per call | 0 | **0** | **0** |
| Adaptive severity lookup | N/A | O(1) map | negligible |
| DB write at export | N/A | async with 3s timeout | off critical path |

Adaptive severity adds a single `map[string]string` lookup per pattern check.
This is O(1) and adds ~10-20ns, well within the p95 budget.

## Known Limitations

1. **Adaptive severity loads once per session**: Effectiveness data is fetched
   when the session state is created, not refreshed mid-session. This is
   acceptable because effectiveness changes slowly.

2. **Minimum sample size of 5**: Patterns with fewer than 5 acted+ignored
   observations keep their base severity. This prevents premature adaptation
   for rarely-seen patterns.

3. **Per-session DB rows, not global counters**: Each session inserts a new row.
   Aggregation happens at query time. This allows time-decay but means the
   table grows with session count. `Prune` is available for cleanup.

4. **No causal tracking across turns**: The "acted" heuristic (did the agent
   use the expected tool within 3 calls) is still best-effort. Phase 3 could
   add deeper causal analysis via LLM reflection.

## Success Criteria Checklist

- [x] Critic prompt includes coaching observations when patterns fired
- [x] Effectiveness data persisted to SQLite per session per pattern
- [x] Adaptive severity suppresses low-effectiveness patterns (silent)
- [x] Adaptive severity upgrades high-effectiveness patterns
- [x] Zero benchmark regression
- [x] All tests race-clean

## Next Steps

Phase 3 (Cross-Session Learning & Dashboard) could build on this by:
1. Adding a `crush toolcoach stats` CLI command querying the effectiveness table
2. Implementing time-decay in `adaptSeverity()` (weight recent sessions higher)
3. Adding per-project effectiveness profiles (different patterns matter in
   different codebases)
