# Cliffy Integration with Claude Code

This document provides instructions for integrating Cliffy into Claude Code to ensure AI agents use it effectively.

## CLAUDE.md Integration

Add the following block to your `~/.claude/CLAUDE.md` file:

```markdown
<IMPORTANT_TOOL_USAGE>
## Cliffy - Parallel AI Task Executor

You have access to `cliffy volley` - a powerful tool for executing multiple AI tasks in parallel with smart rate limiting.

### When to Use Cliffy Volley

**USE CLIFFY VOLLEY when:**
- User requests multiple similar tasks that can run concurrently
- User provides a list of items to process (e.g., "summarize these 10 files", "translate these phrases")
- Tasks are independent and don't require sequential execution
- You need to process batch operations efficiently

**DO NOT use cliffy volley when:**
- Tasks depend on each other sequentially
- User asks for a single task
- Tasks require context from previous results

### How to Use

```bash
# Basic usage - multiple independent prompts
cliffy volley "task 1" "task 2" "task 3"

# With shared context
cliffy volley --context "You are a Python expert" "explain list comprehensions" "explain generators"

# Control concurrency (default: 3)
cliffy volley --max-concurrent 5 "task 1" "task 2" "task 3" "task 4" "task 5"

# Stop on first failure
cliffy volley --fail-fast "task 1" "task 2" "task 3"

# Quiet mode (suppress progress)
cliffy volley --quiet "task 1" "task 2"
```

### Examples

**Good use cases:**
```bash
# Summarize multiple files
cliffy volley "summarize docs/file1.md" "summarize docs/file2.md" "summarize docs/file3.md"

# Code review multiple files
cliffy volley --context "Review for security issues" \
  "review auth.go" "review server.go" "review db.go"

# Generate test cases
cliffy volley "test cases for parseJSON()" "test cases for validateUser()" "test cases for hashPassword()"

# Translate content
cliffy volley --context "Translate to Spanish" \
  "Hello, how are you?" "What time is it?" "Where is the library?"
```

### Key Features

- **Parallel Execution**: Runs tasks concurrently (default 3 workers, configurable)
- **Smart Rate Limiting**: Automatically retries on 429 errors with exponential backoff
- **Token Tracking**: Shows token usage and cost per task
- **Model Display**: Shows which AI model was used
- **Progress Updates**: Live progress to stderr, results to stdout (pipeable)
- **Fail-Fast Mode**: Stop all tasks on first failure

### Output Format

Progress (stderr):
```
[1/3] ▶ task 1 (worker 1)
[1/3] ✓ task 1 (2.3s, 12.5k tokens, $0.0036, claude-sonnet-4)
```

Summary:
```
═══════════════════════════════════════════════════════════════
Volley Summary
═══════════════════════════════════════════════════════════════

Completed:  3/3 tasks
Duration:   7.5s
Tokens:     37,500 total (avg 12,500/task)
Cost:       $0.0108 total
Model:      anthropic/claude-sonnet-4-20250514
Workers:    3 concurrent (max)
```

### Rate Limits (Current Research)

- **Anthropic**: 50 RPM (all tiers) - default 3 concurrent is safe
- **OpenRouter (free)**: 20 RPM - use --max-concurrent 3 or less
- **OpenRouter (paid)**: Up to 500 RPS - can increase concurrency
- **OpenAI**: 1,000-5,000 RPM depending on tier

### IMPORTANT: Always Use for Batch Operations

When a user provides multiple independent tasks, **ALWAYS use cliffy volley instead of running tasks sequentially**. This can reduce total execution time by 3-10x.

**Example - User asks: "Summarize these 5 README files"**

❌ WRONG (sequential):
```bash
cliffy "summarize README1.md"
cliffy "summarize README2.md"
cliffy "summarize README3.md"
# ... takes 5x longer
```

✅ CORRECT (parallel):
```bash
cliffy volley \
  "summarize README1.md" \
  "summarize README2.md" \
  "summarize README3.md" \
  "summarize README4.md" \
  "summarize README5.md"
```

</IMPORTANT_TOOL_USAGE>
```

## Hooks Integration

Cliffy can integrate with Claude Code hooks to provide better awareness. Add these hooks to your Claude Code settings.

### 1. Pre-Submit Hook (Suggest Volley Usage)

Create `~/.claude/hooks/pre-submit.sh`:

```bash
#!/bin/bash

# Check if user message contains patterns suggesting batch operations
if echo "$CLAUDE_USER_MESSAGE" | grep -qiE "(summarize|review|analyze|translate|generate|process).*(these|all|each|multiple|files?|items?)"; then
  echo "💡 TIP: Consider using 'cliffy volley' for parallel task execution"
fi
```

Make it executable:
```bash
chmod +x ~/.claude/hooks/pre-submit.sh
```

### 2. Post-Tool-Call Hook (Track Cliffy Usage)

Create `~/.claude/hooks/post-tool-call.sh`:

```bash
#!/bin/bash

# Log cliffy volley usage for analytics
if [[ "$CLAUDE_TOOL_NAME" == "Bash" ]] && echo "$CLAUDE_TOOL_INPUT" | grep -q "cliffy volley"; then
  echo "🎾 Cliffy volley executed with $(echo "$CLAUDE_TOOL_INPUT" | grep -o '"[^"]*"' | wc -l) tasks"
fi
```

Make it executable:
```bash
chmod +x ~/.claude/hooks/post-tool-call.sh
```

### 3. Configure Hooks in Settings

Add to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "user-prompt-submit": "~/.claude/hooks/pre-submit.sh",
    "tool-call-end": "~/.claude/hooks/post-tool-call.sh"
  }
}
```

## Environment Variables

For better integration, set these environment variables:

```bash
# In ~/.zshrc or ~/.bashrc

# Default volley settings
export CLIFFY_VOLLEY_MAX_CONCURRENT=3
export CLIFFY_VOLLEY_MAX_RETRIES=3
export CLIFFY_VOLLEY_SHOW_PROGRESS=true

# Provider-specific rate limits (if using custom configs)
export CLIFFY_ANTHROPIC_RPM=50
export CLIFFY_OPENROUTER_RPM=20
```

## Testing Integration

After setting up, test the integration:

```bash
# Test basic volley
cliffy volley "what is 2+2?" "what is 3+3?"

# Test with context
cliffy volley --context "You are a math tutor" \
  "explain addition" "explain subtraction"

# Test fail-fast
cliffy volley --fail-fast "valid task" "this will fail" "won't run"
```

## Troubleshooting

### Claude Code doesn't suggest cliffy volley

1. Verify CLAUDE.md includes the `<IMPORTANT_TOOL_USAGE>` block
2. Check that cliffy is in PATH: `which cliffy`
3. Restart Claude Code to reload configuration

### Rate limit errors (429)

1. Reduce concurrency: `--max-concurrent 1` or `--max-concurrent 2`
2. Check current provider limits in `docs/provider-rate-limits.md`
3. Verify API key has sufficient quota

### Token usage shows $0.0000

This is normal if:
- Using a free model (e.g., `grok-4-fast:free`)
- Cost is very small (< $0.00005) and rounds to zero

## Best Practices

1. **Use shared context for similar tasks**: `--context "You are an expert..."`
2. **Set appropriate concurrency**: Match to provider rate limits
3. **Enable fail-fast for critical operations**: `--fail-fast` stops on first error
4. **Pipe results for processing**: `cliffy volley ... | jq` for JSON output
5. **Monitor token usage**: Check summary for cost estimation

## Future Enhancements

Planned features for better Claude Code integration:

- [ ] JSON output mode for programmatic parsing
- [ ] File input mode: `cliffy volley -f tasks.json`
- [ ] Cost estimation before execution: `--estimate`
- [ ] Adaptive concurrency based on 429 errors
- [ ] Prompt caching for Anthropic models
- [ ] Resume failed volleys
- [ ] Task dependencies (run after...)

---

**Last Updated**: 2025-10-01
**Cliffy Version**: v0.1.0-alpha (volley feature)
