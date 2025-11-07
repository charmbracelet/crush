# Volley-First Architecture

**Vision:** Cliffy becomes the parallel AI task execution engine, powered by Fantasy

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                         CLIFFY CLI                           │
│  (Volley-First: Optimized for Parallel Execution)          │
└─────────────────────────────────────────────────────────────┘
                            │
        ┌───────────────────┴──────────────────┐
        │                                       │
        ▼                                       ▼
┌───────────────────┐              ┌────────────────────────┐
│   Single Task     │              │   Volley Mode          │
│   (Runner)        │              │   (Scheduler)          │
│                   │              │                        │
│  Thin wrapper     │              │  ← OUR CORE VALUE ←   │
│  around Fantasy   │              │                        │
└─────────┬─────────┘              └──────────┬─────────────┘
          │                                   │
          │                                   │
          │        ┌──────────────────────────┘
          │        │
          │        │  ┌──────────────────────────────┐
          │        │  │    Volley Scheduler          │
          │        │  │                              │
          │        │  │  - Worker Pool (N workers)  │
          │        └──│  - Task Queue               │
          │           │  - Smart Retry Logic        │
          │           │  - Progress Tracking        │
          │           │  - Health Metrics           │
          │           │  - Cost Tracking            │
          │           │  - Result Aggregation       │
          │           └──────────┬───────────────────┘
          │                      │
          │                      │  (N parallel workers)
          └──────────────────────┘
                     │
                     ▼
          ┌──────────────────────┐
          │   FANTASY LIBRARY    │
          │   (Agent + Providers) │
          └──────────────────────┘
                     │
        ┌────────────┴────────────┐
        │                         │
        ▼                         ▼
┌──────────────┐        ┌──────────────────┐
│  Providers   │        │   Agent Tools     │
│              │        │                   │
│ - Anthropic  │        │  - File Ops       │
│ - OpenAI     │        │  - Grep/Glob      │
│ - OpenRouter │        │  - Bash           │
│ - Bedrock    │        │  - LSP            │
│ - Vertex     │        │  - MCP            │
│ - Azure      │        │  (Adapted to      │
│              │        │   Fantasy API)    │
└──────────────┘        └──────────────────┘
```

---

## Core Components

### 1. Cliffy CLI (Entry Point)
**Responsibility:** Parse arguments, route to single/volley mode
**Size:** ~500 lines
**Code:** `cmd/cliffy/main.go`

```go
// Key decision point
if len(tasks) == 1 {
    // Single task: Use runner (Fantasy wrapper)
    return runner.ExecuteTask(ctx, task, cfg)
} else {
    // Multiple tasks: Use volley scheduler
    return volley.Execute(ctx, tasks, cfg)
}
```

### 2. Runner (Single Task Wrapper)
**Responsibility:** Thin adapter Fantasy → Cliffy output format
**Size:** ~200 lines
**Code:** `internal/runner/runner.go`

```go
package runner

import "charm.land/fantasy"

func ExecuteTask(ctx context.Context, task string, cfg *config.Config) error {
    // 1. Create Fantasy provider
    provider := fantasy.NewProvider(cfg.Provider)

    // 2. Create Fantasy model
    model := provider.LanguageModel(ctx, cfg.Model)

    // 3. Create Fantasy agent with Cliffy tools
    agent := fantasy.NewAgent(model,
        fantasy.WithTools(adaptCliffyTools()...),
        fantasy.WithSystemPrompt(cfg.SystemPrompt),
    )

    // 4. Execute and stream output
    stream, _ := agent.Stream(ctx, fantasy.AgentCall{Prompt: task})
    for chunk := range stream {
        fmt.Print(chunk.Content.Text())
    }

    return nil
}
```

### 3. Volley Scheduler (OUR CORE)
**Responsibility:** Parallel execution, retry, progress, metrics
**Size:** ~800 lines (current) → ~1200 lines (enhanced)
**Code:** `internal/volley/scheduler.go`

```go
type Scheduler struct {
    cfg      *config.Config
    provider fantasy.Provider  // ← Fantasy provider
    options  VolleyOptions

    // Worker pool
    workers      int
    taskQueue    chan Task
    resultsChan  chan Result

    // Smart retry
    retryPolicy  RetryPolicy
    backoff      ExponentialBackoff

    // Progress & metrics
    progress     *ProgressTracker
    metrics      *MetricsCollector
    costTracker  *CostTracker

    // Advanced features (NEW)
    scheduler    *TaskScheduler      // Dependency resolution
    rateLimiter  *RateLimiter        // Per-provider limits
    prioritizer  *TaskPrioritizer    // Urgent tasks first
}

func (s *Scheduler) Execute(ctx context.Context, tasks []Task) ([]Result, error) {
    // 1. Resolve dependencies (if any)
    ordered := s.scheduler.ResolveDependencies(tasks)

    // 2. Prioritize tasks
    prioritized := s.prioritizer.Sort(ordered)

    // 3. Start worker pool
    for i := 0; i < s.workers; i++ {
        go s.worker(ctx, i)
    }

    // 4. Feed tasks to queue (respecting rate limits)
    go s.feedTasks(prioritized)

    // 5. Collect results with progress tracking
    return s.collectResults(ctx, len(tasks))
}

func (s *Scheduler) worker(ctx context.Context, id int) {
    for task := range s.taskQueue {
        // Rate limiting
        s.rateLimiter.Wait(ctx, task.Provider)

        // Execute via Fantasy
        result := s.executeWithRetry(ctx, task)

        // Track metrics
        s.metrics.Record(result)
        s.costTracker.Add(result.TokenUsage, result.Model)

        // Update progress
        s.progress.TaskComplete(task.ID, result.Status)

        // Send result
        s.resultsChan <- result
    }
}
```

### 4. Fantasy Integration Layer
**Responsibility:** Adapt Cliffy's tools to Fantasy's tool API
**Size:** ~300 lines (new)
**Code:** `internal/fantasy/adapter.go`

```go
package fantasy

import (
    "charm.land/fantasy"
    cliffytools "github.com/bwl/cliffy/internal/llm/tools"
)

// AdaptTools converts Cliffy tools to Fantasy tools
func AdaptTools(cliffyTools []cliffytools.BaseTool) []fantasy.Tool {
    fantasyTools := make([]fantasy.Tool, len(cliffyTools))

    for i, ct := range cliffyTools {
        info := ct.Info()

        fantasyTools[i] = fantasy.NewAgentTool(
            info.Name,
            info.Description,
            func(ctx context.Context, params map[string]any) (string, error) {
                // Marshal params to JSON string (Cliffy format)
                input, _ := json.Marshal(params)

                // Call Cliffy tool
                response, err := ct.Run(ctx, cliffytools.ToolCall{
                    Input: string(input),
                })

                // Return response text
                return response.Text(), err
            },
        )
    }

    return fantasyTools
}
```

---

## File Structure (After Refactor)

```
cliffy/
├── cmd/
│   └── cliffy/
│       └── main.go                    # CLI entry (routing logic)
│
├── internal/
│   ├── volley/                        # ← OUR CORE VALUE
│   │   ├── scheduler.go               # Worker pool, retry, progress
│   │   ├── task.go                    # Task definition
│   │   ├── progress.go                # Live progress tracking
│   │   ├── metrics.go                 # Metrics collection (NEW)
│   │   ├── cost.go                    # Cost tracking (NEW)
│   │   ├── dependency.go              # Task dependencies (NEW)
│   │   ├── priority.go                # Task prioritization (NEW)
│   │   └── ratelimit.go               # Per-provider rate limits (NEW)
│   │
│   ├── runner/                        # Single task execution
│   │   └── runner.go                  # Thin Fantasy wrapper
│   │
│   ├── fantasy/                       # Fantasy integration (NEW)
│   │   ├── adapter.go                 # Tool adapter
│   │   └── config.go                  # Config mapping
│   │
│   ├── tools/                         # Cliffy-specific tools
│   │   ├── bash.go                    # Persistent shell
│   │   ├── grep.go                    # Code search
│   │   ├── glob.go                    # File finding
│   │   ├── edit.go                    # File editing
│   │   ├── lsp.go                     # LSP integration
│   │   └── mcp.go                     # MCP integration
│   │
│   ├── config/                        # Configuration
│   │   └── config.go                  # Volley-specific config
│   │
│   └── output/                        # Output formatting
│       ├── json.go
│       ├── markdown.go
│       ├── junit.go                   # JUnit XML (NEW)
│       └── csv.go                     # CSV export (NEW)
│
└── docs/
    ├── STRATEGIC-REFOCUS.md           # This document
    ├── VOLLEY-ARCHITECTURE.md         # Architecture
    ├── FANTASY-MIGRATION.md           # Migration guide
    └── volley/
        ├── scheduling.md              # Advanced scheduling
        ├── batch-processing.md        # Batch mode
        ├── ci-cd-integration.md       # CI/CD examples
        └── cost-optimization.md       # Cost tracking
```

**Deleted (Moved to Fantasy):**
- ~~`internal/llm/agent/`~~ → `fantasy.Agent`
- ~~`internal/llm/provider/`~~ → `fantasy.Provider`
- ~~`internal/message/`~~ → Fantasy handles message state
- ~~`internal/version/`~~ → Use Fantasy's versioning

**Net Result:**
- **Delete:** ~5000 lines of duplicated code
- **Add:** ~1000 lines of volley-specific features
- **Keep:** ~1500 lines of unique Cliffy value

---

## Configuration Changes

### Old Config (Pre-Fantasy)
```json
{
  "models": {
    "large": {
      "provider": "openrouter",
      "model": "deepseek/deepseek-r1:free"
    }
  },
  "providers": {
    "openrouter": {
      "base_url": "https://openrouter.ai/api/v1",
      "api_key": "${CLIFFY_OPENROUTER_API_KEY}"
    }
  },
  "agent": {
    "prompt": "coder"
  }
}
```

### New Config (Post-Fantasy)
```json
{
  // Fantasy provider config (Fantasy handles this)
  "fantasy": {
    "provider": "openrouter",
    "model": "deepseek/deepseek-r1:free",
    "api_key": "${CLIFFY_OPENROUTER_API_KEY}"
  },

  // Volley-specific config (OUR DOMAIN)
  "volley": {
    "max_concurrent": 3,
    "retry": {
      "max_attempts": 3,
      "backoff": "exponential",
      "initial_delay": "1s",
      "max_delay": "60s"
    },
    "rate_limits": {
      "openrouter": "10/min",
      "anthropic": "50/min"
    },
    "scheduling": {
      "mode": "fifo",  // or "priority", "dependency"
      "timeout": "5m"
    },
    "cost_tracking": {
      "enabled": true,
      "budget_limit": 10.00,  // USD
      "alert_threshold": 0.8
    }
  },

  "output": {
    "format": "text",  // or "json", "markdown", "junit", "csv"
    "progress": true,
    "summary": true
  }
}
```

---

## Migration Path

### Phase 1: Proof of Concept (Week 1)
**Goal:** Validate Fantasy integration works

```bash
# Create experiment branch
git checkout -b experiment/fantasy-volley

# Add Fantasy dependency
go get charm.land/fantasy

# Create adapter layer
mkdir -p internal/fantasy
# ... implement adapter.go

# Migrate one provider (OpenRouter)
# Delete internal/llm/provider/openrouter.go
# Use fantasy/providers/openrouter instead

# Test single task execution
go run cmd/cliffy/main.go "what is 2+2"

# Test volley mode
go run cmd/cliffy/main.go "task 1" "task 2" "task 3"

# Benchmark vs current
./benchmark/bench.sh
```

**Success Criteria:**
- ✅ Single task works via Fantasy
- ✅ Volley mode works with Fantasy
- ✅ Performance within 10% of current
- ✅ All tests pass

### Phase 2: Full Migration (Week 2)
**Goal:** Replace all providers with Fantasy

```bash
# Migrate all providers
rm -rf internal/llm/provider/
rm -rf internal/llm/agent/agent.go

# Update imports everywhere
sed -i 's|github.com/bwl/cliffy/internal/llm/provider|charm.land/fantasy|g' **/*.go

# Delete message store (Fantasy handles state)
rm -rf internal/message/

# Update configuration
# ... update config.go to use Fantasy config

# Comprehensive testing
go test ./...
./benchmark/bench.sh --compare main
```

**Success Criteria:**
- ✅ All providers working via Fantasy
- ✅ Tests pass
- ✅ Performance acceptable
- ✅ ~5000 lines of code deleted

### Phase 3: Volley Enhancements (Week 3-4)
**Goal:** Add volley-only features

```bash
# Add new volley features
- Task dependencies
- Priority scheduling
- Advanced rate limiting
- Cost tracking
- Batch processing from file/stdin

# Update documentation
- docs/volley/*.md

# Add CI/CD examples
- examples/github-actions/
- examples/gitlab-ci/
```

---

## Testing Strategy

### Unit Tests
```go
// internal/volley/scheduler_test.go
func TestSchedulerWithFantasy(t *testing.T) {
    // Mock Fantasy provider
    provider := &MockFantasyProvider{}

    // Create scheduler
    scheduler := volley.NewScheduler(cfg, provider, opts)

    // Execute tasks
    results, _ := scheduler.Execute(ctx, tasks)

    // Verify parallel execution
    assert.Equal(t, 3, scheduler.MaxConcurrentUsed())
}
```

### Integration Tests
```bash
# Test with real Fantasy providers
go test -tags=integration ./internal/volley/

# Benchmark volley performance
go test -bench=. ./internal/volley/
```

### E2E Tests
```bash
# Single task
./bin/cliffy "what is 2+2"

# Volley mode
./bin/cliffy "task 1" "task 2" "task 3"

# Batch from file
./bin/cliffy --tasks tasks.json

# With rate limiting
./bin/cliffy --rate-limit anthropic:10/min task1 task2 ... task100
```

---

## Performance Targets

### Before (Current)
- **Cold start:** ~200ms (already good!)
- **Single task:** ~1-3s depending on model
- **Volley (10 tasks, 3 workers):** ~10-15s total
- **Memory:** ~50MB baseline

### After (Fantasy)
- **Cold start:** <250ms (acceptable +50ms for Fantasy)
- **Single task:** ~1-3s (same, via Fantasy)
- **Volley (10 tasks, 3 workers):** ~10-15s (same scheduler)
- **Memory:** ~60MB (acceptable +10MB for Fantasy)

### New Capabilities
- **Volley (100 tasks, 10 workers):** ~60-90s (NEW)
- **Batch from file (1000 tasks):** ~10-15min (NEW)
- **Dependency-based scheduling:** Available (NEW)
- **Cost tracking:** Real-time (NEW)

---

## Risk Mitigation

### Risk 1: Fantasy API Changes
**Probability:** High (it's in preview)
**Impact:** Medium (need to adapt)
**Mitigation:**
- Pin to specific Fantasy version
- Contribute to Fantasy (influence direction)
- Maintain adapter layer (isolates changes)

### Risk 2: Performance Regression
**Probability:** Low
**Impact:** High (kills our value prop)
**Mitigation:**
- Comprehensive benchmarks before migration
- Performance testing in CI
- Rollback plan if >20% slower

### Risk 3: Tool Compatibility
**Probability:** Medium
**Impact:** High (tools are critical)
**Mitigation:**
- Adapter layer for tool API
- Test all tools thoroughly
- Keep Cliffy tool implementations

### Risk 4: Community Confusion
**Probability:** Medium
**Impact:** Low
**Mitigation:**
- Clear migration guide
- Update all docs
- Announce strategic refocus

---

## Success Metrics

### Week 2 (Post-Migration)
- [ ] Fantasy integration complete
- [ ] All tests passing
- [ ] Performance within 10% of baseline
- [ ] 5000+ lines of code deleted
- [ ] Documentation updated

### Month 1
- [ ] 3+ new volley-specific features shipped
- [ ] CI/CD integration examples published
- [ ] 10+ stars on updated README
- [ ] 0 "why not just use Crush?" questions

### Month 3
- [ ] 100+ tasks volley mode working reliably
- [ ] Cost tracking operational
- [ ] First CI/CD production user
- [ ] Benchmarks showing 10x throughput vs Crush

---

## The New Cliffy

**Before:** "Crush, but headless and stateless"
**After:** "Parallel AI task execution engine"

**Before:** Chasing Crush features
**After:** Building volley capabilities Crush can't match

**Before:** 3 versions behind
**After:** Different product, different market

**Go fast. Go parallel. Go Cliffy.** 🚀
