---
name: crush-hooks
description: Create, debug, and configure Crush hooks (user-defined shell commands that fire before tool execution). Use when the user wants to add a hook, write a hook script, troubleshoot hook behavior, or configure hooks in crush.json.
---

# Crush Hooks

Hooks are user-defined shell commands in `crush.json` that fire at specific
points during execution, giving deterministic control over tool behavior. Hooks
run before permission checks.

## Supported Events

Only `PreToolUse` is currently supported. It fires before every tool call.

Event names are case-insensitive and accept snake_case:
`PreToolUse`, `pretooluse`, `pre_tool_use`, `PRE_TOOL_USE` all work.

## Configuration

Add hooks to `crush.json` (project-level or global). Project-level hooks take
precedence.

```jsonc
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "^bash$",                // regex against tool name (optional; omit to match all)
        "command": "./hooks/my-hook.sh",     // required: shell command to run
        "timeout": 10                        // optional: seconds, default 30
      }
    ]
  }
}
```

Only `command` is required. Omit `matcher` to match all tools.

## Writing Hook Scripts

### Input

Hooks receive data two ways:

**Environment variables:**

| Variable                     | Description                              |
| ---------------------------- | ---------------------------------------- |
| `CRUSH_EVENT`                | Event name (e.g. `PreToolUse`)           |
| `CRUSH_TOOL_NAME`            | Tool being called (e.g. `bash`)          |
| `CRUSH_SESSION_ID`           | Current session ID                       |
| `CRUSH_CWD`                  | Working directory                        |
| `CRUSH_PROJECT_DIR`          | Project root directory                   |
| `CRUSH_TOOL_INPUT_COMMAND`   | For `bash` calls: the shell command      |
| `CRUSH_TOOL_INPUT_FILE_PATH` | For file tools: the target file path     |

**JSON on stdin:**

```json
{
  "event": "PreToolUse",
  "session_id": "313909e",
  "cwd": "/home/user/project",
  "tool_name": "bash",
  "tool_input": {"command": "rm -rf /"}
}
```

Parse with `jq`:

```bash
read -r input
tool_name=$(echo "$input" | jq -r '.tool_name')
command=$(echo "$input" | jq -r '.tool_input.command // empty')
```

### Output

**Exit codes:**

| Exit Code | Meaning                                                    |
| --------- | ---------------------------------------------------------- |
| 0         | Success. Stdout is parsed as JSON (see below).             |
| 2         | Block the tool. Stderr is used as the deny reason.         |
| Other     | Non-blocking error. Logged and ignored; tool call proceeds. |

**Simplest form** — block with exit code 2 and stderr:

```bash
if some_bad_condition; then
  echo "Blocked: reason here" >&2
  exit 2
fi
```

**JSON form** — exit 0 with a JSON object on stdout for more control:

```json
{
  "decision": "allow",
  "reason": "not allowed",
  "context": "Extra info appended to tool result",
  "updated_input": {"command": "rewritten command"}
}
```

- `decision`: `"allow"`, `"deny"`, or omit for no opinion.
- `reason`: Shown to the model when denying.
- `context`: Appended to the tool response the model sees.
- `updated_input`: Replaces tool input before execution.

### Multiple Hooks

When multiple hooks match the same tool call:

- **Deny wins** over allow; allow wins over no opinion.
- All deny reasons are concatenated (newline-separated).
- All context strings are concatenated.
- Last non-empty `updated_input` wins. Ignored if denied.

### Timeouts

Default: 30 seconds. If exceeded, the hook is killed and treated as
a non-blocking error (tool call proceeds).

## Hook Templates

When creating hooks, always include `#!/usr/bin/env bash`, use `set -euo pipefail`,
and make the file executable (`chmod +x`).

### Block a dangerous command

```bash
#!/usr/bin/env bash
set -euo pipefail

# Block rm -rf against root.
if echo "$CRUSH_TOOL_INPUT_COMMAND" | grep -qE 'rm\s+-(rf|fr)\s+/'; then
  echo "Refusing to run rm -rf against root" >&2
  exit 2
fi

echo '{"decision": "allow"}'
```

Config: `{"matcher": "^bash$", "command": "./hooks/no-rm-rf.sh"}`

### Inject context into file writes

```bash
#!/usr/bin/env bash
set -euo pipefail

# Remind about formatting when editing Go files.
if [[ "$CRUSH_TOOL_INPUT_FILE_PATH" == *.go ]]; then
  echo '{"decision": "allow", "context": "Remember: run gofumpt after editing Go files."}'
else
  echo '{}'
fi
```

Config: `{"matcher": "^(edit|write|multiedit)$", "command": "./hooks/go-context.sh"}`

### Block all MCP tools (inline)

```jsonc
{"matcher": "^mcp_", "command": "echo 'MCP tools are disabled' >&2; exit 2"}
```

### Log every tool call (inline)

```jsonc
{"command": "echo \"$(date -Iseconds) $CRUSH_TOOL_NAME\" >> ./tools.log"}
```

### Rewrite tool input

```bash
#!/usr/bin/env bash
set -euo pipefail

read -r input
original_cmd=$(echo "$input" | jq -r '.tool_input.command')
rewritten=$(some-rewriter "$original_cmd")

cat <<EOF
{
  "decision": "allow",
  "context": "Rewrote command for efficiency",
  "updated_input": {"command": "$rewritten"}
}
EOF
```

### Restrict file writes to a directory

```bash
#!/usr/bin/env bash
set -euo pipefail

FILE_PATH="${CRUSH_TOOL_INPUT_FILE_PATH:-}"

if [ -z "$FILE_PATH" ]; then
  exit 0
fi

ALLOWED_DIR="./src"

case "$FILE_PATH" in
  "$ALLOWED_DIR"/*)
    echo '{"decision": "allow"}'
    ;;
  *)
    echo "Writes outside $ALLOWED_DIR are not allowed" >&2
    exit 2
    ;;
esac
```

Config: `{"matcher": "^(edit|write|multiedit)$", "command": "./hooks/restrict-writes.sh"}`

## Checklist for Creating Hooks

1. Create the hook script (or use an inline command).
2. Add `#!/usr/bin/env bash` and `set -euo pipefail` for shell scripts.
3. Make the script executable: `chmod +x ./hooks/my-hook.sh`.
4. Add the hook entry to `crush.json` under `hooks.PreToolUse`.
5. Set `matcher` to a regex matching the target tool names, or omit for all tools.
6. Test the hook by triggering the relevant tool call.

## Debugging Hooks

- Hooks that exceed their timeout are killed silently; increase `timeout` if needed.
- Non-zero exit codes other than 2 are logged but don't block — check Crush logs.
- Use `echo "debug info" >&2` for stderr logging that won't interfere with JSON output.
- Verify `matcher` regex matches the intended tool name (e.g. `^bash$` not `bash`
  if you only want the bash tool, not `mcp_something_bash`).

## Claude Code Compatibility

Crush also accepts the Claude Code hook output format:

```json
{
  "hookSpecificOutput": {
    "permissionDecision": "allow",
    "permissionDecisionReason": "Auto-approved",
    "updatedInput": {"command": "echo rewritten"}
  }
}
```

Existing Claude Code hooks work without modification.
