# Critic Skill

## Purpose

Audit primary agent output across six dimensions: correctness, safety,
idiomatics, efficiency, testing, minimalism.

## Configuration

### Global Config (crush.json)

```json
{
  "critic": {
    "enabled": true,
    "model": "anthropic/claude-sonnet-4",
    "max_iterations": 3,
    "auto_approve": false,
    "threshold": 0.85,
    "cache_size": 32,
    "max_diff_size": 32768,
    "max_file_size": 10485760,
    "timeout": "10s",
    "retention_days": 30
  }
}
```

### Per-Session Override

Disable the critic for a single session by setting `CriticEnabled` to `false`
in the session call options. This is useful for long refactorings or
exploratory coding where the critic would be noisy.

### Skill Override Config

Place `.crush/skills/critic/config.json` to adjust thresholds without modifying
global config:

```json
{"threshold": 0.9, "auto_approve": true, "timeout": "5s"}
```

Merge order: global config → skill override. Fields in the skill file take
precedence.

### Prompt Override

Place `.crush/skills/critic/prompt.md.tpl` to replace the default review
prompt.

## How It Works

1. The middleware captures file snapshots before the primary agent runs.
2. After the agent produces a diff, the critic LLM reviews it.
3. The critic returns structured feedback: `approve`, `revise`, or `halt`.
4. On `revise`, the middleware rolls back changes and re-drives the agent with
   feedback injected into the conversation.
5. On `halt`, the middleware rolls back and returns an error to the user.

## Operational Features

- **Circuit breaker**: If the critic LLM fails repeatedly (timeouts, 5xx),
  the circuit opens for 30s to avoid blocking every user action.
- **Project context**: The critic prompt automatically includes `AGENTS.md`,
  `CRUSH.md`, `CLAUDE.md`, and `.cursorrules` from the working directory
  (up to 4 KB) so feedback respects project conventions.
- **Database pruning**: Old critic reviews are pruned on startup according to
  `retention_days`.
- **Global kill switch**: Set `CRUSH_CRITIC_GLOBAL_DISABLE=1` to force-disable
  critic across all sessions without editing config.

## CLI Commands

```bash
# List reviews for a session
crush critic list --session <session-id>

# Show full review details for a message
crush critic show --message <message-id>

# Show aggregate statistics
crush critic stats
```

## Latency Profiling

The middleware logs structured timing on every review:
- `snapshot_ms` — file capture time
- `diff_ms` — diff computation time
- `diagnostics_ms` — LSP diagnostic fetch time
- `review_ms` — LLM review time
- `total_middleware_ms` — total overhead added to the edit loop
