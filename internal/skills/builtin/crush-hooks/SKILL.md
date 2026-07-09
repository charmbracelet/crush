---
name: crush-hooks
description: Use when the user wants to add, write, debug, or configure Crush hooks for prompts, tool calls, tool results, or turn completion.
---

# Crush Hooks

Hooks are commands in `crush.json` that fire at deterministic points in the agent loop. Use them for policy, logging, prompt gating, tool-call rewriting, context injection, and final validation. Hooks run for the top-level agent only; delegated sub-agents do not fire hooks internally.

## Supported Events

- `UserPromptSubmit`: after the user submits a prompt and before the prompt is sent to the model.
- `PreToolUse`: before a tool runs and before permission checks.
- `PostToolUse`: after a tool returns.
- `PostToolUseFailure`: after a tool returns an error result; when configured, it runs instead of `PostToolUse` for that failed result.
- `Stop`: when a turn is complete and before the run-complete event is published.

Event names are case-insensitive and accept snake case, so `pre_tool_use` normalizes to `PreToolUse`.

## Configuration

```jsonc
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "matcher": "prompt",
        "command": "./hooks/prompt-policy.sh",
        "timeout": 10
      }
    ],
    "PreToolUse": [
      {
        "matcher": "^bash$",
        "command": "./hooks/pre-bash.sh",
        "timeout": 10
      }
    ],
    "PostToolUse": [
      {
        "matcher": "^bash$",
        "command": "./hooks/post-bash.sh",
        "timeout": 10
      }
    ],
    "PostToolUseFailure": [
      {
        "matcher": "^bash$",
        "command": "./hooks/recover-failed-bash.sh",
        "timeout": 10
      }
    ],
    "Stop": [
      {
        "matcher": "stop",
        "command": "./hooks/final-check.sh",
        "timeout": 10
      }
    ]
  }
}
```

Project-level hooks take precedence over global hooks. Matching hooks are deduplicated by command, run in parallel, and aggregated in config order.

For global config, use absolute hook paths because relative paths resolve against the current working directory, not the config file.

## Input

All hooks receive environment variables:

| Variable | Description |
| --- | --- |
| `CRUSH_EVENT` | Event name |
| `CRUSH_TOOL_NAME` | Tool name or lifecycle target such as `prompt` or `stop` |
| `CRUSH_SESSION_ID` | Session ID |
| `CRUSH_CWD` | Working directory |
| `CRUSH_PROJECT_DIR` | Project root directory |
| `CRUSH_TOOL_INPUT_COMMAND` | Bash command when present |
| `CRUSH_TOOL_INPUT_FILE_PATH` | File path when present |

Hooks also receive JSON on stdin.

`UserPromptSubmit`:

```json
{
  "event": "UserPromptSubmit",
  "session_id": "abc",
  "cwd": "/repo",
  "prompt": "fix login",
  "attachments": ["screenshot.png"]
}
```

`PreToolUse`:

```json
{
  "event": "PreToolUse",
  "session_id": "abc",
  "cwd": "/repo",
  "tool_name": "bash",
  "tool_input": {"command": "npm test"}
}
```

`PostToolUse`:

```json
{
  "event": "PostToolUse",
  "session_id": "abc",
  "cwd": "/repo",
  "tool_name": "bash",
  "tool_input": {"command": "npm test"},
  "tool_result": {"content": "...", "is_error": false, "metadata": "{}"}
}
```

`PostToolUseFailure` uses the same payload shape as `PostToolUse`, but `event` is `PostToolUseFailure` and `tool_result.is_error` is true.

```json
{
  "event": "PostToolUseFailure",
  "session_id": "abc",
  "cwd": "/repo",
  "tool_name": "bash",
  "tool_input": {"command": "npm test"},
  "tool_result": {"content": "Exit code 1", "is_error": true, "metadata": "{}"}
}
```

`Stop`:

```json
{
  "event": "Stop",
  "session_id": "abc",
  "cwd": "/repo",
  "response": "final assistant text",
  "error": "",
  "cancelled": false
}
```

## Output

Exit codes:

| Exit code | Meaning |
| --- | --- |
| 0 | Parse stdout as JSON |
| 2 | Deny/block this event; stderr is the reason |
| 49 | Halt the whole turn; stderr is the reason |
| Other | Non-blocking hook failure |

JSON envelope:

```json
{
  "version": 1,
  "decision": "allow",
  "halt": false,
  "reason": "optional reason",
  "context": "extra model-visible context",
  "updated_input": {"command": "rewritten command"},
  "updated_prompt": "replacement prompt"
}
```

- `decision`: `allow`, `deny`, or omit. `allow` pre-approves permission prompts only for `PreToolUse`.
- `halt`: ends the turn.
- `reason`: shown when denying or halting.
- `context`: string or array of strings. Concatenated in config order.
- `updated_input`: shallow-merge patch against `tool_input`; only meaningful for `PreToolUse`.
- `updated_prompt`: full replacement for the admitted prompt sent to the model and saved in history; only meaningful for `UserPromptSubmit`.

Claude Code-style `hookSpecificOutput` is also accepted for permission decisions, `updatedInput`, and `additionalContext`.

## Examples

### Intent gate

```bash
#!/usr/bin/env bash
set -euo pipefail
payload=$(cat)
prompt=$(printf '%s' "$payload" | jq -r '.prompt // ""')

case "$prompt" in
  *"research first"*|*"do not edit"*)
    echo '{"context":"This is research-only. Do not edit files unless the user later gives a directive."}'
    ;;
  *)
    echo '{}'
    ;;
esac
```

### Block destructive bash

```bash
#!/usr/bin/env bash
set -euo pipefail

if printf '%s' "$CRUSH_TOOL_INPUT_COMMAND" | grep -qE 'rm\s+-(rf|fr)\s+/'; then
  echo "Refusing to run rm -rf against root" >&2
  exit 2
fi
```

### Final quality gate

```bash
#!/usr/bin/env bash
set -euo pipefail
payload=$(cat)
err=$(printf '%s' "$payload" | jq -r '.error // ""')

if [ -n "$err" ]; then
  echo "{\"context\":\"Final hook observed run error: $err\"}"
else
  echo '{}'
fi
```
