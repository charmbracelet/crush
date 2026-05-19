# Self-Critic Skill — Implementation Document

## Overview

The self-critic skill is a middleware decorator that wraps the primary
`agent.SessionAgent`. After the primary agent produces a diff, a secondary
"critic" agent reviews it across six dimensions (correctness, safety,
idiomatics, efficiency, testing, minimalism) and returns a structured verdict:
**approve**, **revise**, or **halt**.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Coordinator                             │
│  ┌─────────────────┐         ┌──────────────────────────┐  │
│  │  Primary Agent  │◄────────│  Critic Middleware       │  │
│  │  (edit/bash/…)  │         │  - snapshot files        │  │
│  └─────────────────┘         │  - compute diff          │  │
│           ▲                  │  - fetch LSP diags       │  │
│           │                  │  - submit checkpoint     │  │
│           │                  │  - gate decision         │  │
│      rollback               │  - inject feedback       │  │
│           │                  └──────────────────────────┘  │
│           │                          │                     │
│           │                    ┌─────┴─────┐               │
│           │                    ▼           ▼               │
│           │              ┌─────────┐  ┌──────────┐        │
│           │              │  Cache  │  │  Store   │        │
│           │              └─────────┘  └──────────┘        │
│           │                                               │
│      re-run with     ┌──────────────────────────────┐    │
│      feedback        │  CriticService → CheckpointEmitter │
│                      │  (LLM call)                    │    │
│                      └──────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

## File Inventory

| File | Lines | Purpose |
|------|-------|---------|
| `checkpoint.go` | 78 | Types (`Checkpoint`, `CriticFeedback`, `GateDecision`) and gating logic |
| `config.go` | 74 | Runtime config with defaults and guardrails |
| `middleware.go` | 354 | Core decorator wrapping `SessionAgent.Run()` |
| `service.go` | 148 | Review orchestration: cache, retry, pub/sub |
| `parser.go` | 66 | JSON extraction with 4 fallback strategies |
| `prompt.go` | 124 | Template rendering with injection defense |
| `diff.go` | 154 | Git vs library diff with binary detection |
| `snapshot.go` | 136 | File snapshot / rollback with size limits |
| `cache.go` | 60 | SHA-256 keyed LRU cache |
| `diagnostics.go` | 97 | LSP diagnostic fetching |
| `store.go` | 136 | SQLite persistence via sqlc |
| `app/critic.go` | 252 | Provider wiring (10 providers) |

## Configuration

### Global Config (crush.json)

```json
{
  "critic": {
    "enabled": true,
    "model": "anthropic/claude-sonnet-4",
    "max_iterations": 3,
    "auto_approve": false,
    "threshold": 0.85,
    "cache_size": 32,
    "max_diff_size": 32768,
    "max_file_size": 10485760,
    "timeout": "10s",
    "retention_days": 30
  }
}
```

| Field | Default | Purpose |
|-------|---------|---------|
| `enabled` | `false` | Master switch |
| `model` | small model | Critic LLM (provider/model format) |
| `max_iterations` | 3 | Revision loop bound |
| `auto_approve` | `false` | Bypass confidence threshold |
| `threshold` | 0.85 | Min confidence for auto-approval |
| `cache_size` | 32 | LRU cache entries |
| `max_diff_size` | 32 KB | Diff truncation limit |
| `max_file_size` | 10 MB | Skip files larger than this |
| `timeout` | 10s | Per-LLM-call timeout |
| `retention_days` | 30 | DB retention for critic reviews |

### Skill Override Config

Place `.crush/skills/critic/config.json` to override any field without
modifying global config:

```json
{"threshold": 0.9, "auto_approve": true}
```

Merge order: global config → skill override. Fields in the skill file take
precedence.

## Middleware Run Loop

```
for iteration := 0; iteration <= MaxIterations; iteration++ {
    1. Capture file snapshots (tracked files only)
    2. Run primary agent
    3. Detect changed files
    4. Compute diff (git preferred, library fallback)
    5. Fetch LSP diagnostics
    6. Build checkpoint → Review via LLM
    7. Persist review to DB
    8. Gate decision:
       - approve → return result
       - halt    → rollback, return error
       - revise  → rollback, inject feedback, loop
}
```

## Prompt Injection Defense

User-controlled content (diff, plan) is wrapped in delimiters:

```
<<<DIFF_BEGIN>>>
{{.PrimaryDiff}}
<<<DIFF_END>>>
```

`escapeDelimiters()` replaces any user content that looks like a delimiter
with a visually-similar Unicode alternative (`««»»`), preventing the LLM
from interpreting injected text as block terminators.

## Fail-Open Strategy

When the critic fails (emitter error, parse error, timeout), the middleware
returns the primary result with a logged warning rather than blocking the
user. This is a deliberate product decision: a broken critic should not
prevent users from getting work done.

## Retry Logic

`CriticService.Review()` retries up to 3 attempts with exponential backoff:
- Attempt 1: immediate
- Attempt 2: 500 ms delay
- Attempt 3: 1 s delay

Each attempt uses a fresh context with `cfg.Timeout`.

## Telemetry

| Event | Trigger |
|-------|---------|
| `critic_review_completed` | Every review (log + PostHog) |
| `critic_rollback` | Halt or revise rollback |
| `critic_loop_completed` | Loop exits (any verdict) |
| `critic_verdict_rendered` | Pub/sub for TUI badge updates |

## Database Schema

```sql
CREATE TABLE critic_reviews (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    message_id TEXT,
    verdict TEXT NOT NULL,
    confidence REAL NOT NULL,
    concerns TEXT,          -- JSON
    summary TEXT,
    diff_snapshot TEXT,
    lsp_diagnostics TEXT,   -- JSON
    created_at INTEGER      -- unix timestamp
);
```

## Project Overrides

Users can customize:

- `.crush/skills/critic/prompt.md.tpl` — custom review prompt
- `.crush/skills/critic/config.json` — threshold overrides

## Self-Critic Fixes Applied (Phase 6)

| # | Issue | Fix |
|---|-------|-----|
| 1 | `service.go` hardcoded `10*time.Second` | Now uses `cs.cfg.Timeout` (default 10s, configurable) |
| 2 | `cache.go` used `json.Marshal(cp.Iteration)` | Replaced with 4-byte binary encoding (no alloc) |
| 3 | `service.go` retry `time.Sleep` blocked goroutine | Changed to `select { case <-time.After; case <-ctx.Done() }` |
| 4 | `templates/critic.md.tpl` used markdown fences without injection defense | Aligned with default template using `<<<DIFF_BEGIN>>>` delimiters |
| 5 | `CriticSkillConfig` missing `Timeout` field | Added `Timeout time.Duration` with `DefaultTimeout` constant |
| 6 | `config.CriticConfig` missing `Timeout` field | Added for JSON schema parity |
| 7 | `config.go` nil-return path omitted new defaults | Added `MaxDiffSize`, `MaxFileSize`, `Timeout`, `RetentionDays` to early return |
| 8 | `config_test.go` didn't cover new fields | Expanded to verify all 9 fields |

## PR Blocker Fixes (Cleanup Pass)

| # | Issue | Fix |
|---|-------|-----|
| 9 | ~20 planning docs in repo root | Deleted all root `PHASE*.md`, `CRITIQUE*.md`, `IMPL*.md` files |
| 10 | `TESTING.md` stale Gate info | Fixed nil feedback → `GateRevise`; removed "fail-open" mislabel |
| 11 | `SKILL.md` advertised config.json loader that didn't exist | Implemented `LoadSkillConfig()` with `.crush/skills/critic/config.json` support |
| 12 | Database growth unbounded | Added `PruneCriticReviews` SQL + `Store.Prune()` + startup pruning with configurable `retention_days` |

## Phase 8: Control Surface & Resilience

| # | Feature | Implementation |
|---|---------|---------------|
| 13 | Per-session disable | `SessionAgentCall.CriticEnabled *bool` — nil uses global, false bypasses |
| 14 | Project context injection | Loads `AGENTS.md`, `CRUSH.md`, `CLAUDE.md`, `.cursorrules` (up to 4 KB) into prompt |
| 15 | Circuit breaker | Per-session, 3-state; only retryable errors count; 5 failures → 30s open; lazy cleanup |
| 16 | Remove unused checkpoint types | Deleted `CheckpointPlan`, `CheckpointTool`, `CheckpointMessage` |
| 17 | Diff size at compute time | `ComputeDiff` takes `maxSize`; early stop in `libraryDiff`; skip git for >10 files |
| 18 | Cache metrics | Atomic hit/miss counters; logged at loop completion |

## Phase 9: CLI Exposure & Observability

| # | Feature | Implementation |
|---|---------|---------------|
| 19 | `crush critic` CLI | `list --session`, `show --message`, `stats` subcommands using tabwriter output |
| 20 | Latency profiling | Structured logging of `snapshot_ms`, `diff_ms`, `diagnostics_ms`, `review_ms`, `total_middleware_ms` |
| 21 | Global kill switch | `CRUSH_CRITIC_GLOBAL_DISABLE=1` forces `Enabled=false` in `NewCriticSkillConfig` |
| 22 | Revision outcome tracking | `revision_outcome` column on `critic_reviews`; updated by middleware on loop completion |

## Testing

56 unit tests covering parser fallbacks, gating, middleware delegation,
snapshot rollback, diff computation, cache eviction, store CRUD, config
defaults, skill config loading, circuit breaker, app integration, CLI commands,
and global disable.

Run: `go test ./internal/skills/critic/... -v`
