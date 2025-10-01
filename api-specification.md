# Crush-Headless API Specification

## Command Line Interface

### Basic Usage

```bash
crush-headless [flags] "prompt"
```

### Flags

#### Core Flags

##### `--show-thinking` / `-t`
**Type:** Boolean
**Default:** `false`
**Description:** Stream LLM thinking/reasoning to stderr

**Example:**
```bash
crush-headless --show-thinking "refactor the auth module"
```

**Output to stderr:**
```
[THINKING]
Let me analyze the auth module structure first...
I'll need to check for security vulnerabilities...
[/THINKING]
```

##### `--thinking-format`
**Type:** String
**Default:** `text`
**Options:** `json`, `text`, `none`
**Description:** Format for thinking output

**Example (JSON):**
```bash
crush-headless --show-thinking --thinking-format=json "explain the code" 2>thinking.json
```

**JSON Output:**
```json
{"type":"extended_thinking","signature":"<signature>","content":"Let me analyze..."}
{"type":"extended_thinking","signature":"<signature>","content":"Now I'll check..."}
```

**Example (Text):**
```bash
crush-headless --show-thinking --thinking-format=text "explain the code"
```

**Text Output:**
```
[THINKING: <signature>]
Let me analyze this step by step:
1. First I need to understand the structure...
2. Then I'll examine the dependencies...
[/THINKING]
```

##### `--output-format` / `-o`
**Type:** String
**Default:** `text`
**Options:** `text`, `json`, `diff`
**Description:** Format for final output

**Example (Text - default):**
```bash
crush-headless "list all Go files"
```
Output: Plain text streamed to stdout

**Example (JSON):**
```bash
crush-headless --output-format=json "analyze main.go" | jq
```

**JSON Output:**
```json
{
  "content": "The main.go file contains...",
  "thinking": [
    {
      "signature": "<sig1>",
      "content": "First, I'll examine..."
    }
  ],
  "tool_calls": [
    {
      "name": "view",
      "input": "{\"path\":\"main.go\"}"
    }
  ],
  "files_modified": [],
  "tokens_used": {
    "input_tokens": 1234,
    "output_tokens": 567,
    "cache_creation_tokens": 0,
    "cache_read_tokens": 89
  },
  "cost": 0.0034
}
```

**Example (Diff):**
```bash
crush-headless --output-format=diff "fix type errors" > changes.diff
```

**Diff Output:**
```diff
--- a/src/auth.go
+++ b/src/auth.go
@@ -42,7 +42,7 @@
-func validate(token string) error {
+func validate(token string) bool {
```

##### `--quiet` / `-q`
**Type:** Boolean
**Default:** `false`
**Description:** Suppress progress messages to stderr

**Example:**
```bash
crush-headless --quiet "fix bugs" > output.txt
```

No stderr output except errors.

##### `--timeout`
**Type:** Duration
**Default:** `10m`
**Description:** Maximum execution time

**Example:**
```bash
crush-headless --timeout=5m "complex refactor"
```

Exits with error if not complete within 5 minutes.

#### Advanced Flags

##### `--provider` / `-p`
**Type:** String
**Default:** From config
**Description:** Override provider

**Example:**
```bash
crush-headless --provider=openai "task"
```

##### `--model` / `-m`
**Type:** String
**Default:** From config (`large`)
**Description:** Override model (by ID or type)

**Examples:**
```bash
# By model ID
crush-headless --model=claude-3-5-haiku-20241022 "quick task"

# By config type
crush-headless --model=small "simple question"

# With provider
crush-headless --provider=openai --model=gpt-4o "task"
```

##### `--reasoning` / `-r`
**Type:** String
**Default:** Model default
**Options:** `none`, `low`, `medium`, `high`, `auto`
**Description:** Control extended thinking/reasoning level

**Examples:**
```bash
# Disable thinking for speed
crush-headless --reasoning=none "simple task"

# High reasoning for complex problems
crush-headless --reasoning=high "debug race condition"
```

**See:** [Model Selection Guide](./model-selection.md) for details

##### `--fast`
**Type:** Boolean
**Default:** `false`
**Description:** Use fast, cheap model (alias for `--model=small --reasoning=none`)

**Example:**
```bash
crush-headless --fast "what's in main.go?"
```

##### `--smart`
**Type:** Boolean
**Default:** `false`
**Description:** Use most capable model (alias for `--model=large --reasoning=high`)

**Example:**
```bash
crush-headless --smart "complex debugging"
```

##### `--config` / `-c`
**Type:** String
**Default:** `.crush.json` or `crush.json`
**Description:** Path to config file

**Example:**
```bash
crush-headless --config=/path/to/config.json "task"
```

##### `--working-dir` / `-d`
**Type:** String
**Default:** Current directory
**Description:** Working directory for execution

**Example:**
```bash
crush-headless --working-dir=/path/to/project "task"
```

##### `--no-lsp`
**Type:** Boolean
**Default:** `false`
**Description:** Disable LSP integration entirely

**Example:**
```bash
crush-headless --no-lsp "simple task"
```

Faster startup when LSP not needed.

##### `--show-cost`
**Type:** Boolean
**Default:** `false`
**Description:** Show cost estimate before execution and actual cost after

**Example:**
```bash
crush-headless --show-cost "task"
```

**Output:**
```
Estimated cost: $0.01 - $0.05
[normal output]
---
Actual cost: $0.0234 (1,234 input + 567 output + 2,000 thinking tokens)
```

##### `--max-cost`
**Type:** Float
**Default:** Unlimited
**Description:** Abort if estimated cost exceeds limit

**Example:**
```bash
crush-headless --max-cost=0.10 "task"
```

Exits with error code 7 if cost would exceed $0.10.

##### `--progress`
**Type:** Boolean
**Default:** `false`
**Description:** Stream progress events to stderr

**Example:**
```bash
crush-headless --progress "multi-step task"
```

**Stderr output:**
```
[TOOL] view src/main.go
[TOOL] grep TODO
[THINK] Analyzing results...
[TOOL] edit src/main.go:42
```

##### `--max-tokens`
**Type:** Integer
**Default:** Model default
**Description:** Override max output tokens

**Example:**
```bash
crush-headless --max-tokens=8000 "long task"
```

## Input Methods

### Argument
```bash
crush-headless "prompt text"
```

### Stdin
```bash
echo "prompt text" | crush-headless
cat prompt.txt | crush-headless
```

### Combined
```bash
cat context.txt | crush-headless "using the above context, task"
```

## Output Specification

### Stdout

**Purpose:** Primary output (content, results)

**When `--output-format=text` (default):**
- Streamed as generated
- No buffering
- Plain text

**When `--output-format=json`:**
- Buffered until complete
- Single JSON object at end
- Structured data

**When `--output-format=diff`:**
- Buffered until complete
- Unified diff format
- Only changed files

### Stderr

**Purpose:** Metadata, progress, thinking

**Always written:**
- Errors
- Warnings

**Written when `--show-thinking`:**
- Thinking blocks (formatted per `--thinking-format`)

**Written when `--progress`:**
- Tool execution notices
- Progress updates

**Written when not `--quiet`:**
- Tool names as executed

**Written when `--show-cost`:**
- Final cost summary

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Configuration error |
| 3 | Provider/API error |
| 4 | Timeout exceeded |
| 5 | User interruption (Ctrl+C) |
| 6 | Tool execution error |
| 7 | Cost limit exceeded |

## JSON Output Schema

### Complete Schema

```json
{
  "content": "string",
  "model": {
    "id": "string",
    "provider": "string",
    "reasoning_level": "string"
  },
  "thinking": [
    {
      "signature": "string",
      "content": "string"
    }
  ],
  "tool_calls": [
    {
      "name": "string",
      "input": "string"
    }
  ],
  "files_modified": ["string"],
  "tokens_used": {
    "input_tokens": 0,
    "output_tokens": 0,
    "cache_creation_tokens": 0,
    "cache_read_tokens": 0
  },
  "cost": 0.0
}
```

### Field Descriptions

#### `content` (string, required)
The main output text from the LLM.

#### `model` (object, optional)
Information about the model used.

**Fields:**
- `id` (string): Model identifier (e.g., `claude-sonnet-4-20250514`)
- `provider` (string): Provider name (e.g., `anthropic`)
- `reasoning_level` (string): Reasoning level used (e.g., `medium`)

#### `thinking` (array, optional)
Array of thinking blocks, only present if model used extended thinking.

**Fields:**
- `signature` (string): Thinking signature from provider
- `content` (string): The thinking text

#### `tool_calls` (array, optional)
Summary of all tools executed.

**Fields:**
- `name` (string): Tool name
- `input` (string): JSON string of tool input

#### `files_modified` (array, optional)
List of file paths that were modified during execution.

#### `tokens_used` (object, optional)
Token usage statistics.

**Fields:**
- `input_tokens` (int): Total input tokens
- `output_tokens` (int): Total output tokens
- `cache_creation_tokens` (int): Tokens written to cache
- `cache_read_tokens` (int): Tokens read from cache

#### `cost` (float, optional)
Estimated cost in USD.

## Environment Variables

### Standard Crush Variables

All standard Crush environment variables work:

```bash
ANTHROPIC_API_KEY=...
OPENAI_API_KEY=...
GEMINI_API_KEY=...
# etc.
```

### Headless-Specific Variables

#### `CRUSH_HEADLESS_TIMEOUT`
**Default:** `10m`
**Description:** Default timeout for execution

```bash
export CRUSH_HEADLESS_TIMEOUT=5m
crush-headless "task"
```

#### `CRUSH_HEADLESS_SHOW_THINKING`
**Default:** `false`
**Description:** Default for `--show-thinking`

```bash
export CRUSH_HEADLESS_SHOW_THINKING=true
crush-headless "task"
```

#### `CRUSH_HEADLESS_OUTPUT_FORMAT`
**Default:** `text`
**Description:** Default for `--output-format`

```bash
export CRUSH_HEADLESS_OUTPUT_FORMAT=json
crush-headless "task"
```

#### `CRUSH_HEADLESS_QUIET`
**Default:** `false`
**Description:** Default for `--quiet`

```bash
export CRUSH_HEADLESS_QUIET=true
crush-headless "task"
```

#### `CRUSH_HEADLESS_MODEL`
**Default:** From config (`large`)
**Description:** Default model to use

```bash
export CRUSH_HEADLESS_MODEL=claude-3-5-haiku-20241022
crush-headless "task"
```

#### `CRUSH_HEADLESS_PROVIDER`
**Default:** From config
**Description:** Default provider to use

```bash
export CRUSH_HEADLESS_PROVIDER=openai
crush-headless "task"
```

#### `CRUSH_HEADLESS_REASONING`
**Default:** `auto`
**Description:** Default reasoning level

```bash
export CRUSH_HEADLESS_REASONING=low
crush-headless "task"
```

## Configuration File

### Location Priority

1. `--config` flag path
2. `./.crush-headless.json`
3. `./.crush.json`
4. `./crush.json`
5. `~/.config/crush/crush.json`

### Minimal Schema

Headless only reads subset of Crush config:

```json
{
  "providers": {
    "anthropic": {
      "id": "anthropic",
      "type": "anthropic",
      "api_key": "$ANTHROPIC_API_KEY",
      "models": [...]
    }
  },
  "models": {
    "large": {
      "provider": "anthropic",
      "model": "claude-sonnet-4-20250514"
    }
  },
  "lsp": {
    "go": {
      "command": "gopls",
      "enabled": true
    }
  },
  "options": {
    "context_paths": ["CRUSH.md"],
    "debug": false
  }
}
```

**Ignored fields:**
- `permissions.*`
- `options.tui.*`
- `options.attribution.*`
- `options.disable_metrics`
- Session-related settings

## Examples

### Basic Code Review

```bash
# Quick review with fast model
crush-headless --fast "review the changes in src/auth.go"

# Deep review with smart model
crush-headless --smart "review the changes in src/auth.go and suggest improvements"
```

### With Thinking for Debugging

```bash
crush-headless --show-thinking --thinking-format=json \
  "why is the auth test failing?" 2>debug.json
```

Then analyze thinking:
```bash
jq '.content' debug.json
```

### JSON Output for CI/CD

```bash
result=$(crush-headless --output-format=json \
  "analyze this PR for security issues")

issues=$(echo "$result" | jq -r '.content')
cost=$(echo "$result" | jq -r '.cost')

echo "Analysis cost: \$$cost"
echo "$issues" >> pr-review.md
```

### Diff-Only for Code Changes

```bash
crush-headless --output-format=diff \
  "refactor the database layer to use prepared statements" \
  > refactor.diff

git apply refactor.diff
```

### Batch Processing

```bash
for file in src/**/*.go; do
  echo "Processing $file..."
  crush-headless --quiet "add missing error checks to $file" \
    --output-format=diff >> fixes.diff
done
```

### Timeout for CI

```bash
# Fail fast in CI
crush-headless --timeout=2m --quiet \
  "fix linting errors" || exit 1
```

### Cost Tracking

```bash
# Show cost for awareness
crush-headless --show-cost "task"

# Prevent expensive runs
crush-headless --max-cost=0.05 "task"

# Log costs
crush-headless --show-cost --output-format=json \
  "task" | jq '{model, cost, tokens_used}' >> cost-log.jsonl
```

### Multiple Thinking Formats

```bash
# Human readable to terminal
crush-headless --show-thinking --thinking-format=text "task"

# Machine readable to file
crush-headless --show-thinking --thinking-format=json "task" 2>thinking.jsonl

# Parse thinking after
jq -r '.content' thinking.jsonl | less
```

## Streaming Behavior

### Text Mode (default)

**Content:** Streamed immediately to stdout as received
**Thinking:** Streamed immediately to stderr (if enabled)
**Tools:** Names printed to stderr when executed

**Buffering:** Line-buffered

### JSON Mode

**Content:** Buffered until complete
**Thinking:** Buffered until complete
**Tools:** Buffered until complete

**Output:** Single JSON object when done

### Diff Mode

**Content:** Buffered until complete
**Diffs:** Extracted from tool results
**Format:** Unified diff

**Output:** All diffs concatenated

## Error Handling

### Provider Errors

**Example:** Rate limit

**Stderr:**
```
Error: rate limit exceeded, retrying in 5s...
Error: rate limit exceeded, retrying in 10s...
Error: max retries exceeded
```

**Exit code:** 3

### Tool Errors

**Example:** File not found

**Stderr (when not quiet):**
```
[TOOL] view missing.go
Warning: tool execution failed: file not found
```

**Stdout (LLM receives error):**
```
I encountered an error: the file missing.go doesn't exist.
```

**Exit code:** 0 (LLM handled error)

### Timeout

**Example:** Exceeded `--timeout`

**Stderr:**
```
Error: execution timeout exceeded (10m)
```

**Exit code:** 4

### Configuration Error

**Example:** Invalid provider

**Stderr:**
```
Error: provider "invalid" not found in config
```

**Exit code:** 2

## Best Practices

### 1. Use JSON for Automation
```bash
crush-headless --output-format=json "task" | jq '.content'
```

### 2. Enable Thinking for Debugging
```bash
crush-headless --show-thinking "complex task" 2>debug.log
```

### 3. Set Timeouts in CI
```bash
crush-headless --timeout=5m --quiet "ci task"
```

### 4. Track Costs
```bash
crush-headless --show-cost --output-format=json "task" |
  jq '{cost, tokens_used}' >> costs.jsonl
```

### 5. Parallel Processing
```bash
# Process files in parallel
find . -name "*.go" | xargs -P 4 -I {} \
  crush-headless --quiet "lint {}" --output-format=diff > fixes.diff
```

### 6. Progressive Enhancement
```bash
# Try quick fix with fast model first
crush-headless --fast --timeout=1m "quick fix" || \
  # Fall back to deeper analysis with smart model
  crush-headless --smart --show-thinking "deep analysis of the issue"
```

### 7. Model Selection per Task Type
```bash
# Quick questions → fast model
crush-headless --fast "what's in main.go?"

# Standard tasks → default (balanced)
crush-headless "refactor the auth module"

# Complex debugging → smart model
crush-headless --smart "debug this race condition"

# Reasoning-heavy → specific model
crush-headless --model=o1 --reasoning=high "design the architecture"
```

## Future API Extensions

### Planned for v2

#### `--streaming-json`
Stream JSON events instead of buffering:
```bash
crush-headless --streaming-json "task" |
  while read event; do
    echo "$event" | jq '.type'
  done
```

#### `--session-id`
Continue from previous headless session:
```bash
crush-headless --session-id=abc123 "continue previous task"
```

#### `--tools`
Specify which tools to enable:
```bash
crush-headless --tools=view,grep "read-only task"
```

#### `--context-file`
Add file to context:
```bash
crush-headless --context-file=docs/api.md "implement feature X"
```
