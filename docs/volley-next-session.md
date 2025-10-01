# Volley: Next Session Prompt

## Context

We've completed **Phase 1** of the Volley feature - a parallel AI task executor with smart rate limiting for Cliffy. The core scheduler is **working and tested**, but has a few issues to fix before moving to Phase 2.

**Current Status:** ✅ Parallel execution works, retry logic works, CLI works
**Blocking Issue:** 🐛 Token usage tracking returns 0 (needs 1-2 hour fix)

## What Was Built

We implemented a worker pool scheduler that:
- Executes multiple AI tasks concurrently (default: 3 workers)
- Retries failed tasks with exponential backoff
- Tracks progress in real-time to stderr
- Outputs results to stdout
- Handles cancellation gracefully

**Proof it works:**
```bash
$ ./bin/cliffy volley "what is 2+2?" "what is 3+3?" "what is 4+4?"
# Successfully completed 3 parallel tasks in 7.5s
```

See `docs/volley-phase1-complete.md` for full details.

## Your Task: Fix Token Tracking & Complete Phase 2

### Priority 1: Fix Token Usage Tracking (1-2 hours)

**Problem:** All tasks show "0 tokens, $0.0000" in output

**Location:** `internal/volley/scheduler.go:240-241`
```go
// TODO: Extract token usage from message metadata
// For now, return what we have
return output, usage, nil
```

**What to do:**

1. **Read how the agent tracks tokens:**
   ```bash
   # Check how agent.Run() returns token usage
   Read internal/llm/agent/agent.go around line 645 (trackUsage function)
   ```

2. **Extract usage from agent events:**
   - Look at `agent.AgentEventTypeResponse` event structure
   - Check if `ProviderResponse` includes `Usage` field
   - Extract and populate the `Usage` struct in `executeViaAgent()`

3. **Test the fix:**
   ```bash
   go build -o bin/cliffy ./cmd/cliffy
   ./bin/cliffy volley "what is 2+2?"
   # Should show actual token count, not 0
   ```

4. **Expected output:**
   ```
   [1/1] ✓ what is 2+2? (2.5s, 1.2k tokens, $0.0036)
   ```

### Priority 2: Test Rate Limit Handling (2-3 hours)

**Goal:** Verify the scheduler handles 429 errors and backs off properly

**Steps:**

1. **Create a mock agent that simulates rate limits:**
   - Modify `internal/volley/integration_test.go`
   - Add `mockAgent` that returns 429 every N calls
   - Verify scheduler reduces concurrency on failures

2. **Add integration test:**
   ```go
   func TestSchedulerRateLimitHandling(t *testing.T) {
       // Mock agent returns 429 on first 3 calls
       // Verify: retries happen, concurrency adjusts
   }
   ```

3. **Test with real API (optional):**
   ```bash
   # Intentionally trigger rate limit with many concurrent tasks
   ./bin/cliffy volley --max-concurrent 10 $(printf "task%d " {1..50})
   ```

### Priority 3: Research Provider Rate Limits (1 hour)

**Goal:** Document actual rate limits for adaptive throttling

**What to research:**
1. OpenRouter API rate limits (requests/minute, requests/second)
2. Anthropic Claude API rate limits
3. Free tier vs paid tier differences

**Document findings in:**
```bash
# Create new file
docs/provider-rate-limits.md

# Structure:
## OpenRouter
- Free tier: X req/min, Y req/sec, Z concurrent
- Paid tier: ...

## Anthropic
- Tier 1: ...
```

**Then update config:**
```go
// internal/config/provider.go
type RateLimits struct {
    MaxConcurrent     int
    RequestsPerMinute int
    RequestsPerSecond int
}
```

## How to Start

### 1. **READ THIS FIRST:** Phase 1 summary
```bash
cat docs/volley-phase1-complete.md
```
**⚠️ IMPORTANT:** Read the entire Phase 1 summary before touching any code. It contains critical context about what works, what doesn't, and architectural decisions made.

### 2. Review the design doc
```bash
cat docs/volley-design.md
```

### 3. Check current code
```bash
# Key files to understand:
internal/volley/scheduler.go    # Main scheduler logic
internal/volley/task.go          # Types and options
internal/volley/progress.go      # Output formatting
cmd/cliffy/volley.go            # CLI interface
```

### 4. Run existing tests
```bash
# Quick unit tests
go test ./internal/volley -v

# Full integration tests (43s)
go test ./internal/volley -v -run TestScheduler
```

### 5. Try the real API
```bash
go build -o bin/cliffy ./cmd/cliffy
./bin/cliffy volley "what is 2+2?" "what is 3+3?"
```

## Key Architecture Points

### The Flow
```
CLI → Scheduler → Worker Pool → Agent → API
                      ↓
                  Task Queue
                      ↓
             Progress Tracker → stderr
                      ↓
                  Results → stdout
```

### Important Design Decisions
- **Context is optional** - Tasks can share context or run independently
- **Retries consume worker slots** - Simpler than separate retry queue
- **No state between invocations** - Stays true to Cliffy's philosophy
- **Progress to stderr, results to stdout** - Enables piping

### What NOT to Change
- Don't add persistence/sessions (against Cliffy's design)
- Don't make volley the default (it's opt-in)
- Don't block on cost estimation (nice-to-have, not required)

## Success Criteria for This Session

By the end of this session, we should have:

✅ **Token usage showing real numbers** (not 0)
```bash
./bin/cliffy volley "task"
# Output: (2.5s, 1.2k tokens, $0.0036)  ← Not zeros!
```

✅ **Rate limit test passing**
```bash
go test ./internal/volley -v -run TestSchedulerRateLimit
# Shows: adaptive concurrency reduces on 429s
```

✅ **Provider limits documented**
```bash
cat docs/provider-rate-limits.md
# Shows: OpenRouter: 200 req/min, Anthropic: 50 req/min, etc.
```

**Bonus:** Fix output ordering (stderr appearing after stdout)

## If You Get Stuck

### Token tracking not working?
- Check if `agent.AgentEventTypeResponse` includes usage data
- Look at how `internal/runner/runner.go` handles token tracking
- The agent already tracks tokens for regular cliffy - reuse that logic

### Rate limit tests flaky?
- Use mock agent, don't rely on real API for tests
- Add deterministic failure patterns (every 3rd call fails)
- Use short delays (10ms) in tests to keep them fast

### Output ordering still wrong?
- Try `os.Stderr.Sync()` after progress updates
- Verify stderr vs stdout separation in progress tracker
- May need to buffer results and flush at the end

## Questions to Answer

Before starting Phase 3, we need to know:

1. **What are the actual provider rate limits?** (from research)
2. **Does adaptive throttling work?** (from testing)
3. **What's the overhead of the scheduler?** (benchmark needed)
4. **Should we support prompt caching?** (cost savings potential)

## Phase 3 Preview (Future Work)

After fixing Phase 2 issues, Phase 3 will add:
- Cost estimation with `--estimate` flag
- JSON output format (currently stubbed)
- File/stdin input modes (`-f tasks.json`)
- Provider-specific rate limit config
- Prompt caching optimization (Anthropic)

But don't worry about that now - focus on **fixing token tracking** first!

## Final Notes

The volley feature is **80% done**. The scheduler works, tests pass, real API calls succeed. We just need to:
1. Wire up token tracking (the TODO in the code)
2. Validate rate limit handling
3. Document provider limits

This is totally achievable in a 2-3 hour session. You got this! 🎾

---

**TL;DR:** Fix the `TODO` at `internal/volley/scheduler.go:240` to extract token usage from agent events, add a rate limit test, research provider limits, then we're done with Phase 2!
