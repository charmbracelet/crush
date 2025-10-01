# Cliffy - Copy this to ~/.claude/CLAUDE.md

Add this block to your `~/.claude/CLAUDE.md` file to ensure Claude Code uses cliffy volley effectively:

---

<IMPORTANT_TOOL_USAGE>
## Cliffy Volley - Parallel AI Task Executor

You have access to `cliffy volley` - a command-line tool for executing multiple AI tasks in parallel with smart rate limiting.

### Critical Usage Rules

**ALWAYS use `cliffy volley` when:**
- User requests multiple similar/independent tasks
- User provides a list to process (files, questions, translations, etc.)
- Tasks can run concurrently without dependencies
- User says "for each", "all of these", "batch process", etc.

**NEVER use sequential cliffy calls when volley is appropriate** - this wastes 3-10x more time!

### Command Syntax

```bash
# Basic parallel execution
cliffy volley "task 1" "task 2" "task 3"

# With shared context for all tasks
cliffy volley --context "You are a Python expert" \
  "explain decorators" "explain generators" "explain metaclasses"

# Control concurrency (default: 3)
cliffy volley --max-concurrent 5 "task1" "task2" "task3" "task4" "task5"

# Stop on first failure
cliffy volley --fail-fast "task1" "task2" "task3"

# Suppress progress output
cliffy volley --quiet "task1" "task2"
```

### Real Examples

**✅ CORRECT - Parallel batch processing:**
```bash
# User: "Summarize all the design docs"
cliffy volley \
  "summarize docs/auth-design.md" \
  "summarize docs/api-design.md" \
  "summarize docs/db-design.md"

# User: "Review these files for security issues"
cliffy volley --context "Security review: find vulnerabilities" \
  "review auth.go" "review server.go" "review db.go"

# User: "Generate tests for these functions"
cliffy volley \
  "write tests for parseJSON()" \
  "write tests for validateUser()" \
  "write tests for hashPassword()"
```

**❌ WRONG - Sequential when parallel is possible:**
```bash
# DON'T DO THIS - wastes time!
cliffy "summarize docs/file1.md"
cliffy "summarize docs/file2.md"
cliffy "summarize docs/file3.md"
```

### Output Format

**Progress (stderr):**
```
[1/3] ▶ task 1 (worker 1)
[1/3] ✓ task 1 (2.3s, 12.5k tokens, $0.0036, claude-sonnet-4)
```

**Summary:**
```
Completed:  3/3 tasks
Duration:   7.5s
Tokens:     37,500 total (avg 12,500/task)
Cost:       $0.0108 total
Model:      claude-sonnet-4
Workers:    3 concurrent (max)
```

### Key Features

- **Parallel execution** with configurable workers (default: 3)
- **Smart rate limiting** with automatic retry on 429 errors
- **Token tracking** with per-task and total usage
- **Model display** shows which AI model was used
- **Progress to stderr, results to stdout** (pipeable)
- **Fail-fast mode** stops on first error

### Rate Limit Guidelines

Current provider limits (see `docs/provider-rate-limits.md`):
- **Anthropic**: 50 RPM → use default `--max-concurrent 3`
- **OpenRouter (free)**: 20 RPM → use `--max-concurrent 2`
- **OpenRouter (paid)**: 500 RPS → can use `--max-concurrent 10+`
- **OpenAI**: 1,000+ RPM → can use `--max-concurrent 10+`

### Decision Tree

```
User request
    │
    ├─ Multiple independent tasks? ──→ YES ──→ Use cliffy volley
    │
    └─ Single task or sequential? ──→ NO ───→ Use regular cliffy
```

### Pattern Recognition

Use cliffy volley when you see these patterns in user requests:

- "summarize **all/these/each** [files/docs/items]"
- "review **multiple/several** [files/components]"
- "analyze **each of** [items in list]"
- "translate **these** [phrases/sentences]"
- "generate **tests for** [multiple functions]"
- "process **all** [items in batch]"

### CRITICAL: Always Prefer Parallel

When in doubt between sequential and parallel execution, **ALWAYS choose cliffy volley for multiple independent tasks**. The performance gain (3-10x faster) far outweighs any potential overhead.

**Before using regular cliffy multiple times in a row, ask yourself:**
> "Could these run in parallel with cliffy volley?"
>
> If YES → Use cliffy volley
> If NO → Explain why tasks must be sequential

</IMPORTANT_TOOL_USAGE>

---

**Installation Complete!**

Now Claude Code will automatically recognize when to use cliffy volley for parallel task execution.
