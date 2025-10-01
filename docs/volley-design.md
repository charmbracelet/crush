# Volley: Parallel AI Task Execution with Smart Scheduling

## Overview

Volley is Cliffy's parallel task execution feature that intelligently manages API rate limits and concurrency to run multiple AI tasks efficiently. The key innovation is the **request scheduler** - not just running tasks in parallel, but doing so optimally without hitting rate limits or wasting time.

## Problem Statement

Current pain points:
```bash
# Naive parallel execution
cliffy "update tests in auth.go" &
cliffy "update tests in db.go" &
cliffy "update tests in api.go" &
wait
# Problem: All hit API at once → 429 rate limit errors
```

With Volley:
```bash
cliffy volley "$(cat test-update-context.md)" \
  "update tests in auth.go" \
  "update tests in db.go" \
  "update tests in api.go"
# Cliffy manages rate limits, runs as fast as possible
```

## Core Value Proposition

**The scheduler is the feature.** Smart rate limiting, optimal concurrency, and graceful degradation differentiate Cliffy from:
- Naive shell parallelism (`&` + `wait`)
- Manual rate limit management
- Sequential execution (too slow)
- External orchestration tools (too complex)

## Architecture

```
                                    ┌─────────────────────────┐
                                    │   OpenRouter API        │
                                    │   (rate limited)        │
                                    └──────────┬──────────────┘
                                               │
                                               │ Smart throttling
                                               │ (max N concurrent)
                                               │
                    ┌──────────────────────────┴──────────────────────────┐
                    │          Cliffy Internal Scheduler                  │
                    │                                                      │
                    │  ┌─────────┐  ┌─────────┐  ┌─────────┐            │
                    │  │ Worker 1│  │ Worker 2│  │ Worker 3│  ...       │
                    │  └────┬────┘  └────┬────┘  └────┬────┘            │
                    │       │            │            │                   │
                    └───────┼────────────┼────────────┼───────────────────┘
                            │            │            │
                            │            │            │
                    ┌───────▼────┬───────▼────┬───────▼────┐
                    │  Task 1    │  Task 2    │  Task 3    │
                    │  (auth.go) │  (db.go)   │  (api.go)  │
                    └────────────┴────────────┴────────────┘
```

## Key Design Decisions

### 1. Context Handling

**Shared context is optional** - enables two use cases:

#### Use Case A: Pure Parallel (No Context)
```bash
# Independent analysis tasks
cliffy volley \
  "analyze auth.go for security issues" \
  "analyze db.go for performance issues" \
  "analyze api.go for error handling"
```

Each task receives:
```
[System Prompt] + [Task Prompt]
```

#### Use Case B: Shared Context
```bash
# Related refactoring tasks
cliffy volley --context "$(cat refactoring-plan.md)" \
  "refactor auth.go" \
  "refactor db.go" \
  "refactor api.go"
```

Each task receives:
```
[System Prompt] + [Shared Context] + [Task Prompt]
```

**Important:** Tasks are **independent** - no accumulating conversation history between tasks.

### 2. Concurrency Strategy

**Dynamic/Adaptive Scheduler:**

```go
type AdaptiveScheduler struct {
    maxConcurrent     int           // Provider-specific ceiling
    currentConcurrent int           // Current active workers
    successCount      int           // Consecutive successes
    failureCount      int           // Consecutive rate limit errors
}

// Start conservative, scale up on success
func (s *AdaptiveScheduler) AdjustConcurrency() {
    if s.successCount > 5 && s.currentConcurrent < s.maxConcurrent {
        s.currentConcurrent++
        s.successCount = 0
    }

    if s.failureCount > 2 {
        s.currentConcurrent = max(1, s.currentConcurrent - 1)
        s.failureCount = 0
    }
}
```

**Worker Pool Pattern:**
- Fixed pool of N workers
- Workers pull from shared task queue
- Graceful shutdown on context cancellation

### 3. Retry Logic

**Retries consume worker slots** (same as original requests):
- Simpler mental model
- Natural rate limiting
- No special retry queue

**Exponential Backoff with Jitter:**
```go
func retryDelay(attempt int) time.Duration {
    base := 1 * time.Second
    maxDelay := 60 * time.Second

    // Exponential: 1s, 2s, 4s, 8s, 16s, 32s, 60s (capped)
    delay := min(base * (1 << attempt), maxDelay)

    // Add jitter (0-1000ms) to avoid thundering herd
    jitter := time.Duration(rand.Intn(1000)) * time.Millisecond

    return delay + jitter
}
```

**Max Retries:**
- Default: 3 attempts
- Configurable via `--max-retries N`
- After max retries, task marked as failed

### 4. Provider-Specific Tuning

**Auto-detect from config:**

```go
// internal/config/provider.go
type RateLimits struct {
    MaxConcurrent     int `json:"max_concurrent"`
    RequestsPerMinute int `json:"requests_per_minute"`
    RequestsPerSecond int `json:"requests_per_second"`
}

func (p *ProviderConfig) GetRateLimits() RateLimits {
    if p.RateLimits != nil {
        return *p.RateLimits
    }

    // Conservative defaults
    return RateLimits{
        MaxConcurrent: 2,
        RequestsPerMinute: 50,
        RequestsPerSecond: 2,
    }
}
```

**Provider-specific defaults (to be researched):**
```json
{
  "providers": {
    "openrouter": {
      "rate_limits": {
        "max_concurrent": 5,
        "requests_per_minute": 200,
        "requests_per_second": 10
      }
    },
    "anthropic": {
      "rate_limits": {
        "max_concurrent": 3,
        "requests_per_minute": 50,
        "requests_per_second": 5
      }
    }
  }
}
```

### 5. Cost Estimation

**Skip by default** - 50-100ms overhead matters for small volleys

**Auto-warn if expensive:**
```go
func shouldWarnAboutCost(volley Volley) bool {
    totalChars := len(volley.Context)
    for _, task := range volley.Tasks {
        totalChars += len(task.Prompt)
    }

    // Rough: 1 token ≈ 4 chars
    estimatedTokens := totalChars / 4

    // Warn if > 50k tokens OR > 10 tasks
    return estimatedTokens > 50000 || len(volley.Tasks) > 10
}
```

**Warning format:**
```bash
⚠ Large volley detected
  Tasks: 10
  Estimated tokens: ~100k
  Estimated cost: ~$0.30
  Estimated time: 2-3 minutes (3 concurrent)

  Continue? [Y/n]
```

**Flags:**
- `--estimate` - Always show estimation
- `-y, --yes` - Skip confirmation
- `--no-estimate` - Never estimate (fastest)

## CLI Interface

### Basic Usage

```bash
# No shared context (pure parallel)
cliffy volley "task 1" "task 2" "task 3"

# With shared context
cliffy volley --context "$(cat context.md)" "task 1" "task 2" "task 3"

# Context from file
cliffy volley --context-file context.md "task 1" "task 2" "task 3"

# Multiple arguments per task (quotes optional for simple strings)
cliffy volley \
  "analyze auth.go for bugs" \
  "analyze db.go for bugs" \
  "analyze api.go for bugs"
```

### Advanced Options

```bash
# Control concurrency
cliffy volley --max-concurrent 5 task1 task2 task3

# Retry configuration
cliffy volley --max-retries 5 --retry-delay 2s task1 task2 task3

# Output formatting
cliffy volley --output-format json task1 task2 task3
cliffy volley --quiet task1 task2 task3  # Suppress progress

# Cost management
cliffy volley --estimate task1 task2 task3
cliffy volley -y task1 task2 task3  # Skip confirmation

# Model selection
cliffy volley --fast task1 task2 task3
cliffy volley --model grok-4 task1 task2 task3
```

### File and Stdin Input

```bash
# Tasks from file (one per line)
cat tasks.txt
analyze auth.go
analyze db.go
analyze api.go

cliffy volley --context "$(cat context.md)" -f tasks.txt

# Or JSON format
cat tasks.json
{
  "context": "Refactoring plan...",
  "max_concurrent": 5,
  "tasks": [
    {"prompt": "refactor auth.go", "timeout": "5m"},
    {"prompt": "refactor db.go", "timeout": "10m"}
  ]
}

cliffy volley -f tasks.json

# From stdin
ls src/**/*_test.go | \
  xargs -I {} echo "update tests in {}" | \
  cliffy volley --context "$(cat context.md)" --stdin
```

## Output Design

### Live Progress (stderr)

Shows real-time status as tasks execute:

```
Volley: 5 tasks queued, max 3 concurrent

[1/5] ▶ auth.go (worker 1)
[2/5] ▶ db.go (worker 2)
[3/5] ▶ api.go (worker 3)
[1/5] ✓ auth.go (15.2s, 3.2k tokens, $0.01)
[4/5] ▶ handlers.go (worker 1)
[2/5] ✓ db.go (18.7s, 4.1k tokens, $0.01)
[5/5] ▶ middleware.go (worker 2)
[3/5] ⚠ api.go rate limited, retrying in 2s... (worker 3)
[4/5] ✓ handlers.go (12.3s, 2.8k tokens, $0.01)
[3/5] ▶ api.go (retry 1, worker 1)
[5/5] ✓ middleware.go (14.1s, 3.5k tokens, $0.01)
[3/5] ✓ api.go (16.2s, 3.9k tokens, $0.01)

Volley complete: 5/5 tasks succeeded in 45.3s
```

### Task Output (stdout)

Results shown in order with clear separation:

```
═══════════════════════════════════════════════════════════
Task 1/5: analyze auth.go for bugs
═══════════════════════════════════════════════════════════

I found 3 potential issues in auth.go:

1. Line 45: SQL injection risk in Login()
2. Line 78: Password comparison not constant-time
3. Line 112: Missing error handling on token validation

[detailed analysis...]

═══════════════════════════════════════════════════════════
Task 2/5: analyze db.go for bugs
═══════════════════════════════════════════════════════════

[output continues...]

═══════════════════════════════════════════════════════════
Volley Summary
═══════════════════════════════════════════════════════════

Completed:  5/5 tasks
Failed:     0/5 tasks
Duration:   45.3s
Tokens:     17.5k total (avg 3.5k/task)
Cost:       $0.05 total
Workers:    3 concurrent (max)
Retries:    1 total (task 3)
```

### JSON Output

For automation and parsing:

```json
{
  "volley_id": "volley-abc123",
  "status": "completed",
  "summary": {
    "total_tasks": 5,
    "succeeded": 5,
    "failed": 0,
    "duration_sec": 45.3,
    "total_tokens": 17500,
    "total_cost": 0.05,
    "avg_tokens_per_task": 3500,
    "max_concurrent_used": 3,
    "total_retries": 1
  },
  "tasks": [
    {
      "index": 1,
      "prompt": "analyze auth.go for bugs",
      "status": "success",
      "duration_sec": 15.2,
      "tokens_input": 2100,
      "tokens_output": 1100,
      "tokens_total": 3200,
      "cost": 0.01,
      "retries": 0,
      "output": "I found 3 potential issues..."
    }
  ]
}
```

## Implementation Plan

### Phase 1: Core Scheduler (Week 1)

**Goals:**
- Working worker pool with configurable concurrency
- Basic task queuing and execution
- Simple retry logic with exponential backoff
- Live progress output to stderr
- Task output to stdout

**Components:**
```
internal/volley/
  ├── scheduler.go      # AdaptiveScheduler, worker pool
  ├── worker.go         # Individual worker logic
  ├── task.go          # Task definition and result
  ├── progress.go      # Progress tracking and output
  └── scheduler_test.go

cmd/cliffy/volley.go   # CLI command
```

**Deliverables:**
- `cliffy volley task1 task2 task3` works
- Shows live progress
- Handles errors and retries
- Basic tests

### Phase 2: Provider Integration (Week 2)

**Goals:**
- Auto-detect rate limits from config
- Provider-specific tuning
- 429 handling and adaptive throttling
- Token/cost tracking per task

**Components:**
```
internal/config/
  └── ratelimits.go    # Rate limit config and defaults

internal/volley/
  ├── ratelimiter.go   # Rate limit tracking
  └── backoff.go       # Retry strategies
```

**Deliverables:**
- Rate limits respected per provider
- Adaptive concurrency scaling
- Accurate token tracking
- Cost calculation

### Phase 3: Advanced Features (Week 3)

**Goals:**
- Cost estimation with smart warnings
- JSON output format
- File/stdin input modes
- Shared context support

**Components:**
```
internal/volley/
  ├── estimator.go     # Token estimation
  ├── input.go         # File/stdin parsing
  └── output.go        # Output formatting

cmd/cliffy/volley.go   # Extended CLI flags
```

**Deliverables:**
- `--context` flag works
- `--estimate` shows cost before running
- `-f tasks.json` loads from file
- `--output-format json` works

### Phase 4: Polish (Week 4)

**Goals:**
- Performance optimization
- Comprehensive documentation
- Integration tests
- Production hardening

**Deliverables:**
- Full documentation with examples
- Integration test suite
- Benchmarks
- Error recovery edge cases

## Error Handling

### Error Categories

1. **Task Errors** - LLM returns error in response
   - Mark task as failed
   - Continue with other tasks
   - Include in summary

2. **Rate Limit Errors (429)**
   - Retry with exponential backoff
   - Reduce concurrency
   - Track in progress output

3. **Network Errors**
   - Retry with backoff
   - Max retries then fail
   - Don't reduce concurrency (transient)

4. **Auth Errors (401, 403)**
   - Fail immediately
   - Don't retry
   - Stop volley

5. **Context Cancellation**
   - Graceful worker shutdown
   - Return partial results
   - Clear summary of what completed

### Failure Modes

```bash
# Partial failure
Volley complete: 4/5 tasks succeeded, 1 failed

Failed tasks:
  Task 3 (api.go): Max retries exceeded (3/3)

# Complete failure
Volley failed: Authentication error
  Check CLIFFY_OPENROUTER_API_KEY

# User cancellation (Ctrl-C)
Volley cancelled by user

Completed: 2/5 tasks
In progress: 3/5 tasks (stopped)
```

## Open Questions for Research

1. **Provider Rate Limits**
   - OpenRouter: exact limits per tier?
   - Anthropic: concurrent request limits?
   - Other providers: documented limits?

2. **Optimal Defaults**
   - What's the sweet spot for max_concurrent?
   - Should it vary by model size?
   - Free tier vs paid tier differences?

3. **Streaming Strategy**
   - Can we stream results as they complete?
   - Or must we buffer entire response?
   - Impact on progress display?

4. **Worker Lifecycle**
   - Persistent workers or per-task?
   - Connection pooling benefits?
   - Memory management?

5. **Prompt Caching**
   - Can we leverage Anthropic's prompt caching?
   - Does shared context benefit from caching?
   - Cost savings potential?

## Success Metrics

### Performance
- 3-5x faster than sequential execution
- < 5% overhead from scheduling
- Graceful degradation under rate limits

### Reliability
- Zero 429 errors in normal operation
- 99% task success rate (excluding LLM errors)
- Predictable cost estimation (± 10%)

### Usability
- Clear progress indication
- Understandable error messages
- Intuitive CLI interface

## Future Enhancements

**Phase 5+ (Future):**
- Resume failed volleys: `cliffy volley --resume <id>`
- Task dependencies: "run task 2 after task 1"
- Interactive mode: pause/resume/inspect
- Volley templates: save common workflows
- Multi-provider: distribute tasks across providers
- Cost limits: stop if approaching budget

## References

- [Simon Willison's LLM](https://llm.datasette.io/) - Session management patterns
- [Crush Sessions](https://github.com/charmbracelet/crush) - What we removed
- OpenRouter API docs (TBD)
- Anthropic rate limits (TBD)
