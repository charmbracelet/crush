# Performance & Scaling Ideas

ᕕ( ᐛ )ᕗ  Making Cliffy faster and handling scale gracefully

## Current Performance

**Benchmark results (vs Crush):**
- List files: 6902ms (1.11x faster)
- Count lines: 9243ms (1.54x faster)

**Target improvements:**
- Cold start: Currently ~200ms, target <100ms
- First token: Currently ~800ms, target <500ms
- Tool execution: Vary by tool, target parallel where possible

## Performance Philosophy

**Cliffy's advantage:** We optimize for one-off tasks, not sessions.

This means:
- No database writes (instant savings)
- No session management (cleaner code path)
- No title generation (skip extra LLM call)
- Direct streaming (no buffering for UI)

**But we can do better:**
- Parallel tool execution
- Lazy loading of heavy dependencies
- Smarter prompt construction
- Connection pooling
- Caching where appropriate

## Quick Wins (Low Effort, High Impact)

### 1. Lazy LSP Initialization

**Current:** LSP clients initialize even if never used

**Opportunity:**
```go
// internal/lsp/client.go

type LazyLSPClient struct {
    config LSPConfig
    client *Client  // nil until first use
    mu     sync.Mutex
}

func (l *LazyLSPClient) Get() (*Client, error) {
    l.mu.Lock()
    defer l.mu.Unlock()

    if l.client == nil {
        // Only initialize when actually needed
        client, err := NewClient(l.config)
        if err != nil {
            return nil, err
        }
        l.client = client
    }

    return l.client, nil
}
```

**Impact:** Skip 50-100ms startup for tasks that don't need LSP

**Effort:** Low - refactor initialization, keep interface same

### 2. Parallel Tool Execution

**Current:** Agent executes tools sequentially

**Opportunity:** When LLM requests multiple independent tools, run in parallel

```go
// internal/llm/agent/executor.go

func (e *Executor) ExecuteToolsParallel(ctx context.Context, calls []ToolCall) []ToolResult {
    results := make([]ToolResult, len(calls))

    // Check for dependencies
    independent := groupIndependentCalls(calls)

    for _, group := range independent {
        var wg sync.WaitGroup
        for i, call := range group {
            wg.Add(1)
            go func(idx int, tc ToolCall) {
                defer wg.Done()
                results[idx] = e.ExecuteTool(ctx, tc)
            }(i, call)
        }
        wg.Wait()
    }

    return results
}

func groupIndependentCalls(calls []ToolCall) [][]ToolCall {
    // Identify dependencies:
    // - Multiple file reads → parallel
    // - Edit after read → sequential
    // - Multiple searches → parallel
    // - Write after edit → sequential
}
```

**Impact:** 2-5x faster when LLM requests multiple reads/searches

**Effort:** Medium - need dependency analysis, careful error handling

**Example scenario:**
```
LLM requests: Read file1, Read file2, Read file3
Current: 300ms total (100ms each, sequential)
Parallel: 100ms total (all at once)
```

### 3. Prompt Optimization

**Current:** Prompt includes instructions for features Cliffy doesn't have

**Opportunity:** Stripped-down prompt for headless execution

```markdown
# Current: ~2500 tokens (includes TUI instructions, session management)
# Target: ~1000 tokens (only direct execution concepts)

You are Cliffy, a fast AI coding assistant for one-off tasks.

EXECUTION MODEL:
- No sessions - each invocation is independent
- No confirmations - execute tools directly
- No waiting - stream output as you work

TOOL USAGE:
[streamlined tool descriptions]

USER REQUEST:
{prompt}
```

**Impact:**
- Faster first token (less to process)
- Lower cost (~$0.0045 per call savings)
- Clearer context for model

**Effort:** Low - write new prompt, test thoroughly

### 4. Connection Pooling

**Current:** New HTTP client for each provider call

**Opportunity:**
```go
// internal/llm/provider/pool.go

var httpClientPool = &sync.Pool{
    New: func() interface{} {
        return &http.Client{
            Timeout: 30 * time.Second,
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 10,
                IdleConnTimeout:     90 * time.Second,
            },
        }
    },
}

func getHTTPClient() *http.Client {
    return httpClientPool.Get().(*http.Client)
}
```

**Impact:** 20-50ms savings on subsequent requests (keep-alive)

**Effort:** Low - replace client creation

### 5. Smart Config Caching

**Current:** Parse config JSON on every startup

**Opportunity:**
```go
// Check if config changed since last run
func LoadConfigCached(path string) (*Config, error) {
    cacheKey := configCacheKey(path)

    // Check memory cache first (for repeated runs)
    if cached, ok := configCache.Get(cacheKey); ok {
        return cached, nil
    }

    // Load and parse
    cfg, err := parseConfig(path)
    if err != nil {
        return nil, err
    }

    configCache.Set(cacheKey, cfg)
    return cfg, nil
}
```

**Impact:** 5-10ms savings on config parsing

**Effort:** Low - add simple cache

## Big Ideas (Medium Effort, High Impact)

### 6. Batch Execution Mode

**Use case:** Run multiple one-off tasks in sequence or parallel

```bash
# Sequential batch
cliffy --batch tasks.txt

# Parallel batch (with rate limiting)
cliffy --batch tasks.txt --parallel 3

# From stdin
cat tasks.txt | cliffy --batch -
```

**tasks.txt:**
```
list all Go files
count lines of code
find TODO comments
generate test coverage report
```

**Implementation:**
```go
// cmd/cliffy/batch.go

func RunBatch(ctx context.Context, tasks []string, parallel int) error {
    if parallel <= 1 {
        return runSequential(ctx, tasks)
    }
    return runParallel(ctx, tasks, parallel)
}

func runParallel(ctx context.Context, tasks []string, workers int) error {
    limiter := rate.NewLimiter(rate.Limit(workers), workers)

    var wg sync.WaitGroup
    results := make(chan BatchResult, len(tasks))

    for i, task := range tasks {
        wg.Add(1)
        go func(idx int, t string) {
            defer wg.Done()

            // Wait for rate limit slot
            if err := limiter.Wait(ctx); err != nil {
                results <- BatchResult{idx, nil, err}
                return
            }

            // Run task
            result, err := runSingleTask(ctx, t)
            results <- BatchResult{idx, result, err}
        }(i, task)
    }

    wg.Wait()
    close(results)

    return formatBatchResults(results)
}
```

**Benefits:**
- Run daily automation tasks in one shot
- Amortize startup cost across multiple tasks
- Rate limiting prevents API throttling
- Great for CI/CD pipelines

**Considerations:**
- Each task still gets own LLM context (no state bleeding)
- Failed tasks don't block others
- Output clearly separated per task
- Support JSON output for parsing

### 7. Streaming Response Optimization

**Current:** Stream events processed one at a time

**Opportunity:** Buffer and batch writes to stdout

```go
// internal/output/stream.go

type BufferedStream struct {
    buf     *bytes.Buffer
    out     io.Writer
    ticker  *time.Ticker
    mu      sync.Mutex
}

func (s *BufferedStream) Write(p []byte) (n int, err error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Buffer writes
    n, err = s.buf.Write(p)

    // Flush on newline or buffer full
    if bytes.Contains(p, []byte{'\n'}) || s.buf.Len() > 4096 {
        return n, s.flush()
    }

    return n, err
}

func (s *BufferedStream) flush() error {
    if s.buf.Len() == 0 {
        return nil
    }
    _, err := s.out.Write(s.buf.Bytes())
    s.buf.Reset()
    return err
}
```

**Impact:** Smoother output, less syscall overhead

**Effort:** Low-medium - wrap existing output

### 8. Provider Response Caching (Optional)

**Use case:** Repeated identical tasks (testing, demos)

```go
// internal/llm/provider/cache.go

type ResponseCache struct {
    store map[string]CachedResponse
    ttl   time.Duration
    mu    sync.RWMutex
}

func (c *ResponseCache) Get(prompt string, model string) (*Response, bool) {
    key := cacheKey(prompt, model)

    c.mu.RLock()
    defer c.mu.RUnlock()

    cached, ok := c.store[key]
    if !ok || time.Since(cached.timestamp) > c.ttl {
        return nil, false
    }

    return &cached.response, true
}
```

**When to use:**
- `--cache` flag (opt-in, never default)
- Short TTL (5-10 minutes max)
- Development/testing only

**When NOT to use:**
- Production automation (stale responses bad)
- Tasks that read/write files (context changes)
- Default behavior (surprising to users)

## Advanced Ideas (High Effort, High Impact)

### 9. Parallel Prompt Array

**Use case:** Run same prompt across multiple contexts

```bash
# Run prompt on each file
cliffy --for-each "*.go" "add godoc comments to all exported functions"

# Run prompt with different parameters
cliffy --array '[{"lang": "go"}, {"lang": "python"}]' \
  "write fibonacci function in {{lang}}"
```

**Implementation:**
```go
func RunForEach(ctx context.Context, pattern string, prompt string) error {
    files, err := filepath.Glob(pattern)
    if err != nil {
        return err
    }

    // Generate prompts with context
    prompts := make([]string, len(files))
    for i, file := range files {
        prompts[i] = fmt.Sprintf("Working on %s:\n%s", file, prompt)
    }

    // Run in parallel with rate limiting
    return RunBatch(ctx, prompts, 3)
}
```

**Benefits:**
- Bulk operations on multiple files
- A/B testing different approaches
- Parallel refactoring tasks

**Rate limiting critical:** Don't hammer API with 100 concurrent requests

### 10. Smart Tool Preloading

**Observation:** Certain prompts predictably use certain tools

```go
// internal/llm/tools/preload.go

func PreloadTools(prompt string) {
    // Heuristics:
    // "list files" → likely needs Glob
    // "refactor" → likely needs View + Edit
    // "analyze" → likely needs View + Grep

    if strings.Contains(prompt, "list") || strings.Contains(prompt, "find") {
        go warmupTool("Glob")
    }

    if strings.Contains(prompt, "refactor") || strings.Contains(prompt, "edit") {
        go warmupTool("View")
        go warmupTool("Edit")
    }
}

func warmupTool(name string) {
    // Pre-compile regex, pre-allocate buffers, etc.
}
```

**Impact:** 10-20ms savings on first tool use

**Effort:** Low for simple heuristics, high for ML-based prediction

### 11. Result Streaming to Multiple Destinations

**Use case:** Send output to multiple places simultaneously

```bash
# Stream to stdout and save to file
cliffy "generate report" | tee report.txt

# Better: built-in tee
cliffy --output report.txt "generate report"

# Or: multiple outputs
cliffy --output json:report.json --output text:report.txt "generate report"
```

**Implementation:**
```go
type MultiWriter struct {
    writers []OutputWriter
}

func (m *MultiWriter) Write(event Event) error {
    var errs []error
    for _, w := range m.writers {
        if err := w.Write(event); err != nil {
            errs = append(errs, err)
        }
    }
    return errors.Join(errs...)
}
```

## Rate Limiting Strategies

### Provider Limits

OpenRouter, Anthropic, etc. have rate limits:
- Requests per minute (RPM)
- Tokens per minute (TPM)
- Concurrent requests

**Strategy:**
```go
// internal/llm/provider/limiter.go

type RateLimiter struct {
    rpm     rate.Limiter  // Requests per minute
    tpm     rate.Limiter  // Tokens per minute
    concurrent semaphore  // Max parallel requests
}

func (r *RateLimiter) Acquire(ctx context.Context, estimatedTokens int) error {
    // Wait for concurrent slot
    if err := r.concurrent.Acquire(ctx, 1); err != nil {
        return err
    }
    defer r.concurrent.Release(1)

    // Wait for token budget
    if err := r.tpm.WaitN(ctx, estimatedTokens); err != nil {
        return err
    }

    // Wait for request slot
    return r.rpm.Wait(ctx)
}
```

### Batch Rate Limiting

When running batch tasks:

```bash
# Respect rate limits
cliffy --batch tasks.txt --rpm 10 --parallel 3

# Exponential backoff on 429 errors
cliffy --batch tasks.txt --retry-on-rate-limit
```

**Default limits (conservative):**
- 10 RPM for free tiers
- 3 concurrent requests
- 100k TPM for paid tiers

**User can override:**
```json
// ~/.config/cliffy/cliffy.json
{
  "rate_limits": {
    "openrouter": {
      "rpm": 20,
      "tpm": 200000,
      "concurrent": 5
    }
  }
}
```

## Measuring Performance

### Built-in Profiling

```bash
# Time breakdown
cliffy --timings "task"

# CPU profile
cliffy --cpuprofile cpu.prof "task"

# Memory profile
cliffy --memprofile mem.prof "task"

# Trace
cliffy --trace trace.out "task"
```

### Benchmarking Suite

```bash
# Run performance tests
cd benchmark
./bench.sh --profile

# Compare before/after
./bench.sh --baseline v0.1.0 --current HEAD
```

### Key Metrics to Track

1. **Cold start time:** Binary launch to first API call
   - Target: <100ms
   - Current: ~200ms

2. **Time to first token:** API call to first streamed response
   - Target: <500ms
   - Current: ~800ms

3. **Tool execution time:** Individual tool performance
   - View: <50ms for small files
   - Grep: <100ms for large codebases
   - Edit: <20ms per edit

4. **Memory usage:** Peak memory during execution
   - Target: <50MB for typical tasks
   - Current: ~80MB

5. **Token efficiency:** Prompt size and response tokens
   - Reduce prompt from 2500 to 1000 tokens
   - Minimize tool result verbosity

## Implementation Priority

### Phase 1: Quick Wins (Week 1)
- [x] Lazy LSP initialization
- [x] Prompt optimization
- [x] Connection pooling
- [x] Config caching

### Phase 2: Parallel Execution (Week 2)
- [ ] Parallel tool execution
- [ ] Streaming optimization
- [ ] Basic rate limiting

### Phase 3: Batch Mode (Week 3)
- [ ] Sequential batch
- [ ] Parallel batch with workers
- [ ] Rate limit integration
- [ ] Output formatting

### Phase 4: Advanced Features (Week 4+)
- [ ] Array/for-each patterns
- [ ] Multi-output streaming
- [ ] Smart tool preloading
- [ ] Response caching (opt-in)

## Success Criteria

**Performance targets met when:**
- Cold start <100ms on repeated runs
- First token <500ms (model dependent)
- Batch mode handles 10+ tasks smoothly
- Rate limiting prevents API throttling
- Memory stays <50MB for typical tasks

**Scale handling works when:**
- Can run 100 tasks in batch without issues
- Rate limits respected automatically
- Failures don't cascade
- Output remains parseable

## Trade-offs to Consider

### Speed vs Reliability
- Parallel execution faster but more complex error handling
- Caching faster but risk of stale data
- Async faster but harder to debug

**Decision:** Reliability first, then speed. Cliffy must be dependable.

### Flexibility vs Simplicity
- Batch mode adds complexity
- Multiple output formats add maintenance
- Rate limit configuration adds surface area

**Decision:** Start simple, add features based on real user needs.

### Optimization vs Maintainability
- Micro-optimizations can make code harder to read
- Profile before optimizing
- Document why optimizations exist

**Decision:** Only optimize hot paths, keep cold paths clean.

## The Bottom Line

Cliffy is already faster than Crush for one-off tasks due to architectural differences (no DB, no sessions). But we can be much faster:

1. **Immediate wins:** Lazy loading, better prompts, connection pooling
2. **Big features:** Parallel tools, batch mode
3. **Future exploration:** Smart preloading, response streaming

Always measure. Always profile. Always keep the ballboy spirit: fast, focused, reliable.

ᕕ( ᐛ )ᕗ  Fast is our middle name
