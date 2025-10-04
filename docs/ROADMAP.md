# Cliffy Roadmap

> **Purpose:** This document tracks features mentioned in design docs or referenced in code that are not yet fully implemented.
> **Last Updated:** 2025-10-04

## Currently Implemented ✅

- ✅ Volley scheduler with parallel task execution
- ✅ Worker pool with configurable concurrency
- ✅ Smart retry logic with exponential backoff and jitter
- ✅ Per-error retry policies (rate limits, timeouts, network errors)
- ✅ Progress tracking and real-time tool execution display
- ✅ JSON output format for automation
- ✅ Token usage and cost tracking
- ✅ Tool execution metadata collection
- ✅ NDJSON tool trace emission (`--emit-tool-trace`)
- ✅ Context files (`--context-file`)
- ✅ Preset system (`--preset`)
- ✅ Model selection (`--fast`, `--smart`, `--model`)
- ✅ Shell completions (bash, zsh, fish, powershell)
- ✅ Fail-fast cancellation
- ✅ `doctor` and `init` commands
- ✅ Configurable max concurrent workers (`--max-concurrent`)

## High Priority - Core Features

### 1. Wire CLI Flags into Single-Task Execution ⚠️
**Status:** Partially Implemented
**Location:** `internal/runner/runner.go`, `cmd/cliffy/main.go`

**Current State:**
- Flags are parsed but Runner implementation is minimal
- Thinking display not implemented in single-task path
- Stats display works but limited

**Needed:**
- Full streaming implementation in Runner
- Wire `--show-thinking` to display reasoning in real-time
- Wire `--thinking-format` (json/text)
- Consistent behavior between single-task and volley paths

**Improvement Report Reference:** #1

---

### 2. Complete Diff Output Mode ⚠️
**Status:** Declared but not implemented
**Location:** `internal/output/formatter.go:288-292`

**Current State:**
```go
func FormatDiffOutput(results interface{}) string {
    // TODO: Implement in improvement #2
    return "Diff output not yet implemented\n"
}
```

**Needed:**
- Extract diffs from tool metadata
- Format unified diff output
- Show only files modified with before/after
- Support `--output-format diff` flag

**Improvement Report Reference:** #2

---

### 3. Enhanced Stats Display ⚠️
**Status:** Partially Implemented
**Location:** `cmd/cliffy/main.go`, `internal/volley/scheduler.go`

**Current State:**
- Stats tracked but `--stats` flag behavior incomplete
- Quiet mode suppresses all summary output
- Verbose mode shows detailed stats

**Needed:**
- `--stats` should work independently of verbosity
- Show token totals, cost, timing in dedicated stats section
- Don't hide failures in quiet mode
- Rich context for multi-task failures

**Improvement Report Reference:** #3

---

## Medium Priority - Developer Experience

### 4. STDIN and Task File Support 📋
**Status:** Not Implemented
**Mentioned In:** IMPROVEMENT_REPORT.md #5

**Needed:**
- Accept `-` to read tasks from STDIN
- Add `--tasks-file` flag for newline-separated prompts
- Support JSON task batches for complex workflows
- Enable shell pipeline integration

**Use Cases:**
```bash
# From STDIN
echo "analyze auth.go" | cliffy -

# From file
cliffy --tasks-file tasks.txt

# JSON batch
cat tasks.json | cliffy --tasks-file -
```

---

### 5. Per-Task Metadata Enrichment 📋
**Status:** Partially Implemented
**Location:** `internal/volley/task.go`, `cmd/cliffy/main.go`

**Current State:**
- Basic metadata tracked (tokens, duration, retries)
- Tool metadata collected
- Exit codes not exposed in output

**Needed:**
- Exit codes in JSON output
- Retry counts per task
- Tool invocation summaries
- Per-task timing breakdown

**Improvement Report Reference:** #6

---

### 6. Consolidate Single-Task and Volley Execution ⚠️
**Status:** In Progress
**Location:** `internal/runner/runner.go`, `cmd/cliffy/main.go:118-123`

**Current State:**
- Single task → Runner (minimal implementation)
- Multiple tasks → Volley (full featured)
- Inconsistent UX between paths

**Options:**
1. Enhance Runner with full streaming features
2. Route single tasks through Volley with concurrency=1
3. Fold streaming logic from planned Runner into Volley

**Improvement Report Reference:** #7

---

### 7. Interactive Setup Commands ✅ Partially Done
**Status:** Implemented but can be enhanced
**Location:** `cmd/cliffy/doctor.go`, `cmd/cliffy/init.go`

**Current State:**
- `cliffy doctor` - exists and checks config
- `cliffy init` - exists and creates sample config

**Enhancement Ideas:**
- Verify API keys interactively
- Test provider connectivity
- Guide model selection
- Create `.cliffy/init` marker to skip repeats

**Improvement Report Reference:** #8

---

### 8. Built-In Task Templates & Presets ✅ Partially Done
**Status:** Preset system implemented
**Location:** `internal/preset/preset.go`, `cmd/cliffy/preset.go`

**Current State:**
- Preset system exists and works
- Can apply presets to agents and models
- `cliffy preset list` command exists

**Enhancement Ideas:**
- Ship more curated presets (currently minimal)
- YAML/JSON preset files alongside binary
- Per-task preset overrides
- Preset examples: `fast-qa`, `deep-review`, `sec-review`, `perf-analysis`

**Improvement Report Reference:** #9

---

## Low Priority - Polish & Optimization

### 9. Adaptive Concurrency 📋
**Status:** Infrastructure exists, not utilized
**Location:** `internal/volley/scheduler.go:27-42,359-375`

**Current State:**
- Fields tracked: `currentConcurrent`, `successCount`, `failureCount`
- Health metrics available via `getHealthMetrics()` and `shouldBackoff()`
- Not currently used for adaptive behavior

**Potential Use:**
- Reduce concurrency when failure rate is high
- Increase concurrency when success rate is high
- Back off when hitting rate limits
- Thunder herd prevention beyond jitter

**Improvement Report Reference:** #11

---

### 10. Telemetry & Observability 📋
**Status:** Not Implemented
**Mentioned In:** Design docs, IMPROVEMENT_REPORT.md

**Current State:**
- Tool traces can be emitted as NDJSON
- Progress tracking exists
- No centralized telemetry

**Potential Features:**
- Optional telemetry endpoint
- Metrics aggregation
- Performance profiling hooks
- Distributed tracing for debugging

**Notes:**
- Should be opt-in only
- Privacy-focused (no prompt content by default)
- Useful for CI/CD monitoring

---

### 11. Expanded Test Coverage 📋
**Status:** Partial
**Location:** Various `_test.go` files

**Current Coverage:**
- Volley scheduler: ✅ Good coverage
- CLI flag plumbing: ❌ No tests
- Output formatting: ❌ No tests
- Progress rendering: ❌ No tests
- Runner: ❌ Minimal tests

**Needed:**
- Cobra command tests for flag combinations
- Unit tests for `FormatToolTrace`
- Snapshot tests for progress tracker
- Integration tests for full execution paths

**Improvement Report Reference:** #13

---

## Feature Requests from Design Docs

### Features Mentioned but Never Implemented

1. **Parallel Read-Only Tool Execution**
   - **Mentioned:** implementation-guide.md:660-715
   - **Status:** Not implemented
   - **Complexity:** Medium
   - **Value:** Moderate speedup for multi-file reads

2. **ThinkingFormatter Component**
   - **Mentioned:** architecture.md (original), implementation-guide.md
   - **Status:** Thinking is tracked, but no dedicated formatter
   - **Current:** Displayed inline via progress tracker
   - **Enhancement:** Could add structured thinking output

3. **Lazy LSP Initialization**
   - **Mentioned:** implementation-guide.md:580-620
   - **Status:** LSP clients created but not lazily loaded
   - **Enhancement:** Defer LSP startup until first diagnostic request
   - **Value:** Faster cold start for non-diagnostic tasks

4. **Cost Savings Features**
   - **Mentioned:** docs/README.md:190-206
   - **Current:** Cost tracked and displayed
   - **Enhancement Ideas:**
     - Budget limits per run
     - Cost warnings before expensive operations
     - Cost breakdown by tool type

---

## Recently Completed

- ✅ **Jitter in Retry Logic** - Added in scheduler.go with ±25% jitter
- ✅ **Per-Error Retry Policies** - Implemented ErrorClass system with adaptive backoff
- ✅ **Queue Draining on Cancellation** - Clean shutdown when fail-fast triggers
- ✅ **Max Concurrent Override** - `--max-concurrent` flag added
- ✅ **Preset System** - Full implementation with apply/list commands

---

## Won't Implement (vs. Design Docs)

### HeadlessRunner Component
**Reason:** Implemented differently using Volley + Runner dual-path approach

### StreamingProcessor Component
**Reason:** Event processing handled directly in Scheduler and Agent

### DirectToolExecutor Component
**Reason:** Tool execution integrated into Agent system

### Title Generation
**Reason:** Removed intentionally (1.5s overhead, not needed for one-off tasks)

### Persistence Layer
**Reason:** Zero-persistence is core design principle

---

## How to Contribute

1. Pick an item marked with 📋 (planned) or ⚠️ (partially done)
2. Check the referenced code locations
3. See IMPROVEMENT_REPORT.md for detailed analysis
4. Submit PR with tests
5. Update this roadmap

## Priority Guidelines

- **High Priority:** Core functionality gaps that affect user experience
- **Medium Priority:** DX improvements that make Cliffy easier to use
- **Low Priority:** Nice-to-have optimizations and polish

---

**Questions?** See [IMPROVEMENT_REPORT.md](../IMPROVEMENT_REPORT.md) for detailed analysis of each improvement.
