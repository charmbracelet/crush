# Cliffy Visual System - Mockups & Examples

This document shows concrete examples of how the Cliffy visual system looks in practice across different scenarios.

## Table of Contents
1. [Basic Task Execution](#basic-task-execution)
2. [Parallel Volley](#parallel-volley)
3. [Error & Retry Scenarios](#error--retry-scenarios)
4. [Advanced Visualizations](#advanced-visualizations)
5. [Debug & Diagnostic Views](#debug--diagnostic-views)

---

## Basic Task Execution

### Single Task - Simple

**Input**:
```bash
./bin/cliffy "analyze the authentication system"
```

**Output (during)**:
```
◍═══╕  1 task volleyed
    ╰──╮ Using claude-3-5-sonnet

1   ◑ analyze the authentication system
```

**Output (complete)**:
```
◍═══╕  1 task volleyed
    ╰──╮ Using claude-3-5-sonnet

1   ● analyze the authentication system (2.3s, 3.2k tokens)

◍ 1/1 tasks succeeded in 2.3s
```

### Single Task - With Tools

**Input**:
```bash
./bin/cliffy --verbose "refactor the user authentication to use bcrypt"
```

**Output (during - phase 1)**:
```
◍═══╕  1 task volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ◔ refactor the user authentication to use bcrypt (worker 1)
```

**Output (during - phase 2)**:
```
◍═══╕  1 task volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ◑ refactor the user authentication to use bcrypt (worker 1)
  ├───▣ glob     **/*auth*.go  0.2s
  ├───▣ read     internal/auth/hash.go  0.1s
  ╰───▥ grep     bcrypt.*  running...
```

**Output (during - phase 3)**:
```
◍═══╕  1 task volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ◕ refactor the user authentication to use bcrypt (worker 1)
  ├───▣ read     internal/auth/hash.go  0.1s
  ├───▣ grep     bcrypt  0.1s
  ├───▣ edit     internal/auth/hash.go  0.3s
  ╰───▤ bash     go test ./internal/auth  starting...
```

**Output (complete)**:
```
◍═══╕  1 task volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ● refactor user auth [glob read grep edit bash]  4.5k tokens $0.0056  3.8s

◍ 1/1 tasks succeeded in 3.8s
```

---

## Parallel Volley

### Three Tasks - Different Progress States

**Input**:
```bash
./bin/cliffy --verbose \
  "analyze auth.go for security issues" \
  "refactor database connection pooling" \
  "add unit tests for user service"
```

**Output (early stage)**:
```
◍═══╕  3 tasks volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ◕ analyze auth.go for security issues (worker 1)
  ├───▣ read     internal/auth/auth.go  0.2s
  ├───▣ grep     sql.*exec  0.1s
  ╰───▣ grep     password  0.1s
2 ╮ ◑ refactor database connection pooling (worker 2)
  ├───▣ glob     **/*db*.go  0.3s
  ╰───▥ read     internal/db/pool.go  reading...
3   ◔ add unit tests for user service (worker 3)
```

**Output (mid-execution)**:
```
◍═══╕  3 tasks volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ● analyze auth.go [read 2×grep]  2.8k tokens $0.0035  2.1s
2 ╮ ◕ refactor database connection pooling (worker 2)
  ├───▣ read     internal/db/pool.go  0.2s
  ├───▣ edit     internal/db/pool.go  0.4s
  ╰───▤ bash     go test ./internal/db  starting...
3 ╮ ◑ add unit tests for user service (worker 1)
  ├───▣ read     internal/user/service.go  0.2s
  ╰───▥ write    internal/user/service_test.go  writing...
```

**Output (complete)**:
```
◍═══╕  3 tasks volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ● analyze auth.go [read 2×grep]  2.8k tokens $0.0035  2.1s
2 ╮ ● refactor db pooling [glob read edit bash]  4.2k tokens $0.0051  3.4s
3 ╮ ● add unit tests [read write bash]  3.9k tokens $0.0048  3.2s

◍ 3/3 tasks succeeded in 3.9s
  10.9k tokens  $0.0134  ⬢⬢⬢ (3 workers used)
```

### Five Tasks - Worker Visualization

**Input**:
```bash
./bin/cliffy --verbose --workers 3 \
  "task 1" "task 2" "task 3" "task 4" "task 5"
```

**Output (worker view mode)**:
```
◍═══╕  5 tasks volleyed ⬢⬢⬢ (3 workers)
    ╰──╮ Using x-ai/grok-4-fast:free

║ Worker 1 ⬢ ║ → 1 ╮ ● task 1 completed  1.2s
║ Worker 2 ⬢ ║ → 2 ╮ ◕ task 2 finalizing...
║ Worker 3 ⬢ ║ → 3   ◑ task 3 processing...
║═══════════║
║   Queue   ║ → 4   ○ task 4 queued
              → 5   ○ task 5 queued
```

---

## Error & Retry Scenarios

### Rate Limit + Retry

**Output (rate limited)**:
```
◍═══╕  3 tasks volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ● analyze codebase [glob read]  2.1k tokens $0.0026  1.8s
2 ╮ ◌ generate documentation (attempt 1) ⟲
    ⤴ Error: rate limited (429)
    ⏸ Retrying in 2.0s...
3   ○ build project
```

**Output (retry in progress)**:
```
◍═══╕  3 tasks volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ● analyze codebase [glob read]  2.1k tokens $0.0026  1.8s
2 ╮ ◔ generate documentation (worker 2, attempt 2)
3   ○ build project
```

**Output (retry succeeded)**:
```
◍═══╕  3 tasks volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ● analyze codebase [glob read]  2.1k tokens $0.0026  1.8s
2 ╮ ● generate docs [read write] ⟲×1  3.4k tokens $0.0042  4.2s
3   ○ build project
```

### Tool Failure

**Output (tool failed)**:
```
◍═══╕  2 tasks volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ● run tests [bash]  1.8k tokens $0.0022  2.1s
2 ╮ ◌ deploy to staging ✗
  ├───▣ bash     git push origin staging  0.8s
  ├───▩ bash     ./deploy.sh staging  ⊗ exit 1
  │         ⤴ Error: deployment script failed
  ╰───▫ bash     rollback.sh  (not executed)

◍ 1/2 succeeded, 1 failed in 3.2s
  ✓ Task: 1
  ✗ Task: 2 (deploy to staging)
```

### Multiple Retries + Failure

**Output (exhausted retries)**:
```
◍═══╕  1 task volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ◌ fetch data from API ✗ (failed after 3 retries)
  ├───⟲ Attempt 1: timeout after 30s
  ├───⟲ Attempt 2: connection refused
  ├───⟲ Attempt 3: timeout after 30s
  ╰───⊗ Gave up
    ⤴ Error: maximum retries exceeded

◍ 0/1 succeeded, 1 failed in 92.4s
```

---

## Advanced Visualizations

### Nested Agent Tool Execution

**Output (agent calling agent)**:
```
◍═══╕  1 task volleyed
    ╰──╮ Using claude-3-5-sonnet (coder agent)

1 ╮ ◑ implement user registration API (worker 1)
  ├───▣ read     internal/api/handlers.go  0.2s
  ├───▦ agent    analyze database schema  1.2s
  │   ├───▣ read     internal/db/schema.go  0.2s
  │   ╰───▣ grep     CREATE TABLE users  0.1s
  ├───▣ edit     internal/api/handlers.go  0.4s
  ╰───▥ bash     go test ./internal/api  running...
```

### Complex Processing Pipeline

**Output (showing data transformations)**:
```
◍═══╕  1 task volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ◑ migrate database schema (worker 1)
  │
  ├───▤ Phase 1: Analysis  initializing...
  │   ├───▣ read     migrations/*.sql  0.3s
  │   ╰───▣ grep     ALTER TABLE  0.1s
  │
  ├───▥ Phase 2: Planning  processing...
  │   ├───▣ glob     internal/db/**/*.go  0.2s
  │   ╰───▦ write    migration-plan.md  generating...
  │
  ╰───▫ Phase 3: Execution  pending...
```

### Cache Hit Visualization

**Output (with cache)**:
```
◍═══╕  3 tasks volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ◉ analyze auth.go [cached]  0.2k tokens  0.1s
2 ╮ ● review database.go [read grep]  2.8k tokens $0.0035  2.1s
3   ◑ check tests.go (worker 2)
```

---

## Debug & Diagnostic Views

### Verbose Mode - Full Tool Details

**Output (verbose mode)**:
```
◍═══╕  1 task volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ◕ refactor authentication (worker 1)
  │
  ├───▣ glob     internal/**/*auth*.go
  │         → matched 5 files  0.3s
  │
  ├───▣ read     internal/auth/handler.go
  │         → 245 lines  0.1s
  │
  ├───▣ grep     -i "password.*hash"
  │         → 8 matches  0.1s
  │
  ├───▣ edit     internal/auth/handler.go
  │         → replaced 3 blocks  0.4s
  │
  ╰───▣ bash     go test -v ./internal/auth
  │         → PASS  0.8s
  │         → coverage: 87.3%
```

### Data Flow Diagram (Debug Mode)

**Output (data flow visualization)**:
```
Task: "build and test the project"

Input → Scheduler → Worker → Agent → Tools → Output
═══     ╬╬╬         ⬢        ╱╲      ▣▣▣     ═══

□ build ⇨ queue ⇨ worker-1 ⇨ agent ⇨ bash ⇨ ● success
                                  ↓     ↓
                               context tools
                                  │     │
                                  ╰─────╯

Pipeline Stages:
  ○ Queued       0.1s  (waiting for worker)
  ◔ Initialize   0.2s  (loading context)
  ◑ Process      2.1s  (executing tools)
  ◕ Finalize     0.3s  (formatting output)
  ● Complete     2.7s  (total duration)

Tool Execution:
  bash "go build"       → exit 0   1.2s
  bash "go test ./..."  → exit 0   0.9s
```

### Performance Metrics View

**Output (metrics mode)**:
```
◍═══╕  10 tasks volleyed ⬢⬢⬢ (3 workers)
    ╰──╮ Using claude-3-5-sonnet

Performance Metrics:
  ⟨ Duration ⟩
    Total:        45.2s
    Per task:     4.5s avg
    Slowest:      8.2s (task 7)
    Fastest:      2.1s (task 3)

  ⟨ Tokens ⟩
    Total:        48.7k
    Per task:     4.9k avg
    Input:        32.1k (65.9%)
    Output:       16.6k (34.1%)

  ⟨ Cost ⟩
    Total:        $0.0598
    Per task:     $0.0060 avg

  ⟨ Workers ⟩
    Utilization:  87.3%
    Task 1:       3.2s (worker 1)
    Task 2:       3.1s (worker 2)
    Task 3:       2.1s (worker 3)
    ...

  ⟨ Tools ⟩
    Total calls:  47
    read:         12 (25.5%)
    bash:         10 (21.3%)
    edit:         8  (17.0%)
    grep:         8  (17.0%)
    glob:         5  (10.6%)
    write:        4  (8.5%)
```

### Error Trace View

**Output (error debugging)**:
```
◍═══╕  1 task volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ◌ deploy application ✗
  │
  ├───▣ bash     git status
  │         → On branch main
  │
  ├───▣ bash     git push origin main
  │         → Everything up-to-date
  │
  ├───▩ bash     ./scripts/deploy.sh production  ⊗ exit 1
  │         ⤴ Error: Deployment failed
  │         │
  │         ├─ stderr: Connection refused (port 22)
  │         ├─ stdout: Connecting to prod-server-1...
  │         ├─ exit:   1
  │         ├─ time:   2.3s
  │         │
  │         ╰─ Stack:
  │              deploy.sh:45   ssh_connect()
  │              deploy.sh:12   main()
  │
  ╰───▫ bash     ./scripts/rollback.sh  (not executed)

Error Chain:
  ○ Task started
  ◔ Context loaded
  ◑ Tools executing
  ⊗ SSH connection failed
  ◌ Task failed
```

### Timeline View (Gantt-style)

**Output (timeline visualization)**:
```
◍═══╕  5 tasks volleyed ⬢⬢⬢ (3 workers)
    ╰──╮ Using claude-3-5-sonnet

Timeline (45.2s total):

Worker 1: ══════════════════════════════════════════════
  Task 1: ●●●●●●●── (3.2s)
  Task 4: ────────●●●●●●●●●●── (4.8s)
  Task 5: ────────────────────●●● (1.5s)

Worker 2: ══════════════════════════════════════════════
  Task 2: ●●●●●── (2.8s)
  Task 6: ──────●●●●●●●●●●●●── (6.2s)

Worker 3: ══════════════════════════════════════════════
  Task 3: ●●── (1.2s)
  Task 7: ───●●●●●●●●●●●●●●●●── (8.2s)

Queue:
  ⏸ Task 8-10: waiting... (0 in queue)

Legend: ● = active  ─ = waiting  ⏸ = queued
```

---

## Compact vs Detailed Modes

### Compact Mode (default)

```
◍═══╕  5 tasks volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ● analyze [read grep]  2.1k $0.0026  1.8s
2 ╮ ● refactor [edit bash]  3.2k $0.0039  2.3s
3 ╮ ● test [write bash]  2.8k $0.0034  2.1s
4 ╮ ● docs [read write]  3.5k $0.0043  2.6s
5 ╮ ● build [bash]  1.9k $0.0023  1.5s

◍ 5/5 tasks succeeded in 3.2s
```

### Detailed Mode (--verbose)

```
◍═══╕  5 tasks volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ● analyze authentication system
  ├───▣ read     internal/auth/handler.go  245 lines  0.2s
  ╰───▣ grep     password.*hash  8 matches  0.1s
    → Duration: 1.8s  Tokens: 2.1k  Cost: $0.0026

2 ╮ ● refactor database pooling
  ├───▣ edit     internal/db/pool.go  replaced 3 blocks  0.4s
  ╰───▣ bash     go test ./internal/db  PASS  0.8s
    → Duration: 2.3s  Tokens: 3.2k  Cost: $0.0039

...
```

---

## Theme Variations

### Minimal Theme (ASCII only)

```
o===+  3 tasks volleyed
    \--+ Using claude-3-5-sonnet

1 + * analyze [read grep]  2.1k $0.0026  1.8s
2 + * refactor [edit]  3.2k $0.0039  2.3s
3   o test (worker 1)

o 2/3 tasks succeeded, 1 running
```

### Maximum Theme (all symbols)

```
◍═══╕  3 tasks volleyed ⬢⬢⬢
    ╰──╮ Using claude-3-5-sonnet

║ Worker ⬢1 ║ → 1 ╮ ● analyze ⟨2.1k⟩ ⟨$0.0026⟩ ⟨1.8s⟩
║ Worker ⬢2 ║ → 2 ╮ ● refactor ⟨3.2k⟩ ⟨$0.0039⟩ ⟨2.3s⟩
║ Worker ⬢3 ║ → 3 ╮ ◑ test processing...
║═══════════║

◍ 2/3 tasks: ●● ◑
  ⟨5.3k tokens⟩ ⟨$0.0065⟩ ⟨2.3s⟩
```

---

## Real-World Example: Full Development Workflow

**Input**:
```bash
./bin/cliffy --verbose \
  "analyze the authentication system for security issues" \
  "refactor to use bcrypt instead of md5" \
  "add unit tests for password hashing" \
  "update documentation" \
  "run full test suite"
```

**Output (complete)**:
```
◍═══╕  5 tasks volleyed ⬢⬢⬢ (3 workers)
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ● analyze the authentication system for security issues
  ├───▣ glob     internal/**/*auth*.go  5 files  0.3s
  ├───▣ read     internal/auth/handler.go  245 lines  0.2s
  ├───▣ grep     password  12 matches  0.1s
  ├───▣ grep     md5  3 matches  0.1s
  ╰───▣ read     internal/auth/hash.go  89 lines  0.1s
    → Found security issues: MD5 hashing (insecure), no salt
    → Duration: 2.8s  Tokens: 4.2k  Cost: $0.0052

2 ╮ ● refactor to use bcrypt instead of md5
  ├───▣ read     internal/auth/hash.go  89 lines  0.1s
  ├───▣ edit     internal/auth/hash.go  replaced 2 blocks  0.4s
  ├───▣ edit     go.mod  added bcrypt dependency  0.1s
  ╰───▣ bash     go mod tidy  PASS  0.6s
    → Replaced MD5 with bcrypt, added salt generation
    → Duration: 3.4s  Tokens: 5.1k  Cost: $0.0063

3 ╮ ● add unit tests for password hashing
  ├───▣ read     internal/auth/hash.go  103 lines  0.1s
  ├───▣ write    internal/auth/hash_test.go  created  0.3s
  ╰───▣ bash     go test -v ./internal/auth  PASS  1.2s
    → Added 5 test cases, coverage: 92.3%
    → Duration: 3.8s  Tokens: 4.8k  Cost: $0.0059

4 ╮ ● update documentation
  ├───▣ read     README.md  0.1s
  ├───▣ read     docs/authentication.md  0.1s
  ╰───▣ edit     docs/authentication.md  updated section  0.2s
    → Updated security section with bcrypt details
    → Duration: 1.9s  Tokens: 2.9k  Cost: $0.0036

5 ╮ ● run full test suite
  ├───▣ bash     go test ./...  PASS  4.2s
  ╰───▣ bash     go build ./cmd/cliffy  SUCCESS  1.8s
    → All tests passed (127 tests), build successful
    → Duration: 6.3s  Tokens: 3.2k  Cost: $0.0039

◍ 5/5 tasks succeeded in 7.8s
  ⟨Summary⟩
    Total tokens:  20.2k
    Total cost:    $0.0249
    Workers used:  ⬢⬢⬢ (3)
    Tools called:  18 total (bash: 5, read: 6, edit: 4, grep: 2, glob: 1, write: 1)

  ⟨Results⟩
    ✓ Security issues identified and fixed
    ✓ MD5 replaced with bcrypt
    ✓ 5 new tests added (92.3% coverage)
    ✓ Documentation updated
    ✓ All tests passing, build successful
```

---

## Summary

The Cliffy visual system provides:

1. **Clear State Communication**: Instantly see what's happening
2. **Progressive Detail**: More info as needed (compact → detailed → debug)
3. **Distinctive Style**: Tennis/volley metaphor throughout
4. **Terminal-Friendly**: Works everywhere Unicode does
5. **Scalable**: From single task to complex parallel workflows

The symbol system creates a cohesive visual language that makes Cliffy's parallel execution model transparent and understandable at a glance.
