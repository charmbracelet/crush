# Self-Critic Feature Guide

> Automated code review that runs after every agent edit. The critic reviews diffs, LSP diagnostics, and tool outputs, then decides whether to approve, request revisions, or halt execution.

---

## Table of Contents

- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Toggle On / Off](#toggle-on--off)
- [CLI Commands](#cli-commands)
- [Provider Setup](#provider-setup)
- [How It Works](#how-it-works)
- [Troubleshooting](#troubleshooting)
- [Known Limitations](#known-limitations)

---

## Quick Start

1. Add critic config to your crush config (must be under `options`):

```json
{
  "options": {
    "critic": {
      "enabled": true,
      "model": "minimax-china/MiniMax-M2.7-highspeed",
      "max_iterations": 2,
      "threshold": 0.85,
      "auto_approve": true
    }
  }
}
```

2. Run crush as usual:

```bash
crush run "Create a hello.go file"
```

3. After the agent finishes, inspect reviews:

```bash
crush critic stats
crush critic list --session <session-id>
crush critic show --message <message-id>
```

---

## Configuration

The critic config lives under `options.critic` in your `crush.json`. All fields are optional and have sensible defaults.

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `false` | Master switch for the critic. |
| `model` | string | (small model) | Which model to use for the critic reviewer. Format: `provider/model`. |
| `max_iterations` | int | `3` | Max revision loops per checkpoint. 0 = review once, no revision. |
| `threshold` | float | `0.85` | Confidence threshold. Below this, revise requires confirmation (unless `auto_approve`). |
| `auto_approve` | bool | `false` | Skip confirmation prompts on revise decisions. |
| `max_diff_size` | int | `32768` | Max diff bytes sent to the critic (32 KB). |
| `max_file_size` | int | `10485760` | Max file size snapshotted for review (10 MB). |
| `timeout` | string | `"10s"` | Timeout for each critic LLM call. |
| `cache_size` | int | `32` | LRU cache size for identical diff reviews. |
| `retention_days` | int | `30` | Days to keep critic reviews in the database. |

### Config file locations (searched in order, later wins)

1. `~/.config/crush/crush.json`
2. `~/.local/share/crush/crush.json`
3. `crush.json` / `.crush.json` found recursively from CWD upward
4. `.crush/crush.json` in the working directory

### Project-local overrides

Create `.crush/skills/critic/config.json` in your project root to override the global critic config for that project only:

```json
{
  "enabled": true,
  "max_iterations": 1,
  "threshold": 0.9
}
```

Durations can be written as strings: `"timeout": "5s"`.

---

## Toggle On / Off

### Global disable (env variable)

```bash
# Force-disable critic for this process, regardless of config
export CRUSH_CRITIC_GLOBAL_DISABLE=1
crush run "Do something"
```

### Per-session disable (code)

When calling `SessionAgentCall` programmatically:

```go
enabled := false
call := agent.SessionAgentCall{
    SessionID:       "sess-123",
    Prompt:          "Write some code",
    CriticEnabled:   &enabled, // nil = use global config; false = disable for this call
}
```

### Via config file

Edit `crush.json` and set `"enabled": true` or `"enabled": false` under `options.critic`.

---

## CLI Commands

### `crush critic list --session <id>`

List all critic reviews for a session in a table:

```
VERDICT  CONFIDENCE  CONCERNS  SUMMARY
approve  0.95        0         This is a minimal, syntactically correct...
revise   0.72        1         Critical nil pointer risk detected
halt     0.31        1         Security: hardcoded credentials
```

### `crush critic show --message <id>`

Show the full review for a specific assistant message:

```
Verdict:    approve
Confidence: 0.95
Summary:    This is a minimal, syntactically correct Go program...
Concerns:
  - [info | style] Minor indentation issue
    Suggestion:     fixed
```

### `crush critic stats`

Aggregate statistics across all stored reviews:

```
Total reviews:  4
Approved:       2 (50.0%)
Revised:        1 (25.0%)
Halted:         1 (25.0%)
```

---

## Provider Setup

### MiniMax China (M2.7 / M2.7-highspeed)

MiniMax uses the Anthropic message format internally, so it works out of the box with the critic.

Example `crush.json`:

```json
{
  "providers": {
    "minimax-china": {
      "api_key": "sk-..."
    }
  },
  "models": {
    "large": {
      "model": "MiniMax-M2.7",
      "provider": "minimax-china"
    },
    "small": {
      "model": "MiniMax-M2.7-highspeed",
      "provider": "minimax-china"
    }
  },
  "options": {
    "critic": {
      "enabled": true,
      "model": "MiniMax-M2.7-highspeed",
      "max_iterations": 2,
      "auto_approve": true
    }
  }
}
```

### Other providers

Any provider supported by Crush can be used for the critic. The critic model should be a **cheap/fast** model since it runs after every edit. Good candidates:

- `minimax-china/MiniMax-M2.7-highspeed`
- `openai/gpt-4o-mini`
- `anthropic/claude-3-haiku`

---

## How It Works

```
User Prompt
    |
    v
+----------------------------+
|  1. Snapshot files the     |
|     agent has read so far  |
+----------------------------+
    |
    v
+----------------------------+
|  2. Run primary agent      |
|     (LLM + tools)          |
+----------------------------+
    |
    v
+----------------------------+
|  3. Detect changed files   |
|     (compare snapshot)     |
+----------------------------+
    |
    v
+----------------------------+
|  4. Compute diff           |
+----------------------------+
    |
    v
+----------------------------+
|  5. Fetch LSP diagnostics  |
+----------------------------+
    |
    v
+----------------------------+
|  6. Send to critic LLM     |
|     (review prompt)        |
+----------------------------+
    |
    v
+----------------------------+
|  7. Gate decision          |
|     Approve -> return      |
|     Revise  -> rollback +  |
|                retry       |
|     Halt    -> rollback +  |
|                error       |
+----------------------------+
```

### Gate decisions

| Verdict | Action |
|---|---|
| **approve** | Accept the edit and return the result. |
| **revise** | Roll back files, inject feedback into the conversation, and retry (up to `max_iterations`). |
| **halt** | Roll back files and return an error with the review summary. |

### Fail-closed behavior

- If the critic LLM call fails (timeout, network error, parse error), the middleware **defaults to revise** (fail-closed).
- If the circuit breaker is open (5 consecutive retryable errors), reviews are skipped for that session.

---

## Troubleshooting

### "Critic enabled" never appears in logs

- Check that `critic` is under `options`, not at the top level of `crush.json`.
- Check that `CRUSH_CRITIC_GLOBAL_DISABLE` is not set.
- Verify your config is being loaded: `crush models` should show your configured providers.

### "No critic reviews found for this session"

- The critic only reviews edits that change files. If the agent only chat-responded without using tools, no review is generated.
- On the **first run of a session**, the critic may miss new files because the file tracker only knows about previously-read files (see [Known Limitations](#known-limitations)). Continue the session and edit again to trigger a review.

### Reviews are too slow

- Use a faster model for the critic (e.g., `MiniMax-M2.7-highspeed` instead of `MiniMax-M2.7`).
- Reduce `max_iterations` to 1 or 0.
- Increase `threshold` so fewer edits trigger revision loops.
- Enable `auto_approve` to skip confirmation prompts.

### High API costs

- Reduce `max_iterations`.
- Lower `max_diff_size` to send smaller diffs to the critic.
- Set `max_file_size` to skip large files.
- The LRU cache deduplicates identical diffs automatically.

### Circuit breaker keeps opening

- Check network connectivity to your critic provider.
- Increase `timeout` if the critic model is slow.
- Verify the provider API key is valid.

---

## Known Limitations

1. **First-run blind spot**: On the first turn of a session, the critic may not detect newly created files because the snapshot is based on files the agent has previously read. This only affects the first edit in a session; subsequent edits in the same session are reviewed correctly.

2. **New files without read**: If the agent writes a file it never read, the critic may not see it in the diff. This is related to #1.

3. **LSP required for diagnostics**: LSP diagnostics enrichment only works if the relevant language server (e.g., `gopls`) is installed and discoverable. Missing LSPs do not block reviews; they just reduce context.

4. **Binary files**: Binary files are detected and skipped in diff computation.

---

## Environment Variables

| Variable | Effect |
|---|---|
| `CRUSH_CRITIC_GLOBAL_DISABLE=1` | Force-disable critic regardless of config. |
| `CRUSH_CRITIC_MODEL` | Override critic model. |
| `CRUSH_CRITIC_THRESHOLD` | Override confidence threshold. |
| `CRUSH_CRITIC_MAX_ITERATIONS` | Override max revision loops. |
| `CRUSH_CRITIC_AUTO_APPROVE` | Override auto-approve setting. |
| `CRUSH_CRITIC_TIMEOUT` | Override review timeout (e.g., `5s`). |
| `CRUSH_CRITIC_MAX_DIFF_SIZE` | Override diff size limit. |
| `CRUSH_CRITIC_MAX_FILE_SIZE` | Override file size limit. |
| `CRUSH_CRITIC_RETENTION_DAYS` | Override data retention period. |
