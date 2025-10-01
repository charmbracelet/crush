# Crush-Headless: Overview

## Purpose

Crush-headless is a proposed optimized fork of Crush designed specifically for non-interactive, one-off execution (`crush run -q -y` use cases). It eliminates UI, persistence, and session management overhead while exposing LLM reasoning/thinking for maximum transparency.

## Why Separate from Crush?

The current `crush run -q -y` mode carries significant overhead:
- Full SQLite database initialization and persistence
- Pub/sub event system for UI updates (unused in headless mode)
- Session management and title generation (1 extra LLM call)
- LSP client initialization for all configured languages
- Permission service infrastructure (bypassed with `-y`)
- TUI components (skipped but still initialized)

**Result:** ~800ms cold start, ~50MB memory, complex execution path for what should be a simple streaming operation.

## Key Improvements

### 1. Zero Persistence
- No database, no sessions, no message history
- In-memory delta tracking only
- **4x faster cold start** (800ms → 200ms)

### 2. Thinking/Reasoning Exposure
- Current limitation: Extended thinking is captured but **never shown to users**
- New flags: `--show-thinking`, `--thinking-format=json|text`
- Stream reasoning to stderr while content goes to stdout
- **Critical for debugging and understanding model behavior**

### 3. Direct Streaming
- No pub/sub, no message service layer
- Provider events → stdout/stderr directly
- **2x faster to first token** (1200ms → 600ms)

### 4. Optimized for Automation
- Structured JSON output (`--output-format=json`)
- Progress events for CI/CD monitoring
- Diff-only mode for code reviews
- Cost tracking

## Performance Targets

| Metric | Current | Target | Improvement |
|--------|---------|--------|-------------|
| Cold start | 800ms | 200ms | **4x** |
| First token | 1200ms | 600ms | **2x** |
| Memory | 50MB | 15MB | **3.3x** |
| Code complexity | 100% | 35% | **65% reduction** |
| Thinking visibility | 0% | 100% | **∞** |

## Usage Examples

```bash
# Basic one-off task
crush-headless "fix the type errors in src/main.go"

# With thinking visible
crush-headless --show-thinking "refactor the auth module" 2>thinking.log

# JSON output for parsing
crush-headless --output-format=json "analyze this bug" | jq '.content'

# CI/CD integration
crush-headless --no-interactive --diff "review $(git diff main)" > review.md
```

## Architecture Principles

1. **Zero Persistence:** No DB, no sessions, no history
2. **Minimal Abstraction:** Direct execution paths
3. **Streaming First:** Stdout/stderr as primary interface
4. **Thinking Transparent:** Full reasoning visibility
5. **Fast Boot:** Lazy initialization of everything

## Next Steps

See the detailed documentation:
- [Architecture](./architecture.md) - System design and components
- [Implementation Guide](./implementation-guide.md) - How to build it
- [Performance Analysis](./performance-analysis.md) - Optimization details
- [API Specification](./api-specification.md) - Flags and output formats
