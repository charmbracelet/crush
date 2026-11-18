---
name: crush-hooks
description: Use when the user wants to add, write, debug, or configure a Crush hook — gating or blocking tool calls, approving or rewriting tool input before execution, injecting context into tool results, or troubleshooting hook behavior in crush.json.
---

# Crush Hooks

Hooks are user-defined commands in `crush.json` that fire at specific points
during execution, giving deterministic control over tool behavior. They run
**before** permission checks and **only on the top-level agent's** tool calls —
subagent calls (task tool, agentic_fetch, etc.) are not intercepted, though
the subagent tool call itself is.

For the full reference, see `docs/hooks/README.md`. This skill covers what you
need to author correct hooks.

## Supported Events

Crush supports 13 hook events: `PreToolUse`, `PostToolUse`,
`PostToolUseFailure`, `PermissionRequest`, `PermissionResult`,
`SessionStart`, `SessionEnd`, `TurnStart`, `TurnEnd`, `StopFailure`,
`Interrupt`, `PreCompact`, `PostCompact`. Event names are case-insensitive
and accept snake_case (`PreToolUse`, `pretooluse`, `pre_tool_use` all work);
an unknown name is a config error. The five events that carry a tool call
(`PreToolUse`, `PostToolUse`, `PostToolUseFailure`, `PermissionRequest`,
`PermissionResult`) accept `matcher`; the rest run every configured hook and
reject `matcher`.

Only `PreToolUse` (allow/deny/halt/rewrite/context), `PostToolUse`
(halt/replace output/context/feedback), `PostToolUseFailure`
(halt/context), and `PreCompact` (block) honor output. The rest are
observe-only. Turn and tool events fire only for the top-level agent, never
for subagents (`agentic_fetch`, the task tool), so a `TurnEnd` hook runs
once per user prompt.

Per-event payload extras: `PostToolUse` has `tool_response` (the output
string), `PostToolUseFailure` and `StopFailure` have `error`, `TurnEnd` has
`last_assistant_message`, `PermissionResult` has `decision`
(`granted`/`denied`/`cancelled`), `SessionStart` has `source`
(`startup`/`resume`), `SessionEnd` has `reason`, and the compaction events
have `trigger` (`auto`/`manual`). Payloads also carry `hook_event_name` and
`tool_use_id`, and hooks see `CLAUDE_PROJECT_DIR`, so scripts written for
Claude Code usually work unchanged.

## Configuration

```jsonc
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "^bash$",              // regex against tool name (optional; omit to match all)
        "command": "./hooks/my-hook.sh",   // required: shell command to run
        "timeout": 10                     // optional: seconds, default 30
      }
    ]
  }
}
```

Project-level hooks take precedence over global. Matching hooks are deduped by
`command`, run in parallel, and aggregated in **config order** (not finish order).

## Language

`command` is a shell command, so hooks can be written in any language by
invoking the interpreter: `node ./hooks/h.js`, `python3 ./hooks/h.py`,
`./hooks/h.sh`, inline `echo '…'`, etc. The rest of this skill shows bash, but
the input/output contract is identical regardless of language.

## Input

**Environment variables:**

| Variable                     | Description                              |
| ---------------------------- | ---------------------------------------- |
| `CRUSH_EVENT`                | Event name (e.g. `PreToolUse`)           |
| `CRUSH_TOOL_NAME`            | Tool being called (e.g. `bash`)          |
| `CRUSH_SESSION_ID`           | Current session ID                       |
| `CRUSH_CWD`                  | Working directory                        |
| `CRUSH_PROJECT_DIR`          | Project root directory                   |
| `CLAUDE_PROJECT_DIR`         | Same value, for Claude Code scripts      |
| `CRUSH_TOOL_INPUT_COMMAND`   | For `bash` calls: the shell command      |
| `CRUSH_TOOL_INPUT_FILE_PATH` | For file tools: the target file path     |

The `CRUSH_TOOL_*` variables are only set on events that carry a tool call.

**JSON on stdin:**

```json
{
  "event": "PreToolUse",
  "hook_event_name": "PreToolUse",
  "session_id": "313909e",
  "cwd": "/home/user/project",
  "tool_name": "bash",
  "tool_input": {"command": "rm -rf /"},
  "tool_use_id": "call_01"
}
```

## Output

Communicate back via exit code (+ stderr) or JSON on stdout.

| Exit Code | Meaning                                                       |
| --------- | ------------------------------------------------------------- |
| 0         | Success. Stdout is parsed as the JSON envelope below.         |
| 2         | Block this tool call. Stderr becomes the deny reason.         |
| 49        | Halt the whole turn. Stderr becomes the halt reason.          |
| Other     | Non-blocking error unless stdout holds a JSON object.         |

Exit 2 blocks one tool call (agent sees the reason and can try again); exit 49
ends the whole turn (user takes over). Default to deny — reach for halt only
when letting the agent retry is itself the problem (e.g. secrets detected,
policy violation).

**JSON envelope (exit 0):**

```json
{
  "version": 1,
  "decision": "allow",
  "halt": false,
  "reason": "...",
  "context": "Extra info for the model",
  "updated_input": {"command": "rewritten"}
}
```

- **`decision`**: `"allow"`, `"deny"`, or omit. `"allow"` is **affirmative
  pre-approval** — it bypasses the permission prompt entirely. Omit it
  (or `null`) when you only want to inject context or rewrite input without
  also auto-approving the call.
- **`halt: true`**: ends the turn (same as exit 49).
- **`reason`**: shown to the model on deny; to model and user on halt.
- **`context`**: string **or array of strings**. Appended to what the model
  sees. Empty entries are dropped.
- **`updated_input`**: **shallow-merge patch** against `tool_input`, not a
  replacement. Keys you include overwrite; keys you don't are preserved.
  Nested objects are replaced wholesale, not deep-merged. Ignored on deny/halt.
- **`updated_output`** (`PostToolUse` only): a string that replaces the tool
  output. The last hook to set it wins.
- On `PostToolUse`, `decision: "deny"` cannot block (the tool already ran);
  its `reason` is appended to the output as feedback for the model.
- Claude Code forms are accepted too: `hookSpecificOutput`, top-level
  `decision: "block"` with `reason`, and `continue: false` with `stopReason`.

## Aggregation (Multiple Hooks)

Composed in **config order**:

- `deny` > `allow` > no opinion. First deny decides; subsequent allows don't override.
- `halt` is sticky: any hook halting ends the turn.
- `reason` and `context` concatenate in config order (newline-joined).
- `updated_input` patches shallow-merge sequentially; later patches win on colliding keys.

## Canonical Examples

### Block destructive commands

```bash
#!/usr/bin/env bash
set -euo pipefail

if echo "$CRUSH_TOOL_INPUT_COMMAND" | grep -qE 'rm\s+-(rf|fr)\s+/'; then
  echo "Refusing to run rm -rf against root" >&2
  exit 2
fi
```

Config: `{"matcher": "^bash$", "command": "./hooks/no-rm-rf.sh"}`

### Auto-approve read-only tools (inline, no script)

```jsonc
{"matcher": "^(view|ls|grep|glob)$", "command": "echo '{\"decision\":\"allow\"}'"}
```

Every `view`/`ls`/`grep`/`glob` call now runs without prompting.

### Inject context without auto-approving

Emit only `context` — omit `decision` so the normal permission flow still runs.

```bash
#!/usr/bin/env bash
set -euo pipefail

if [[ "$CRUSH_TOOL_INPUT_FILE_PATH" == *.go ]]; then
  echo '{"context": "Remember: run gofumpt after editing Go files."}'
else
  echo '{}'
fi
```

Config: `{"matcher": "^(edit|write|multiedit)$", "command": "./hooks/go-context.sh"}`

### Rewrite tool input (shallow merge)

```bash
#!/usr/bin/env bash
set -euo pipefail

rewritten=$(jq -r '.tool_input.command' | some-rewriter)
jq -n --arg cmd "$rewritten" '{context: "Rewrote command", updated_input: {command: $cmd}}'
```

Build JSON with `jq -n --arg` rather than string interpolation so quotes and
newlines in the value cannot break the envelope.

### Redact tool output (PostToolUse)

```jsonc
{"matcher": "^bash$", "command": "jq -c '{updated_output: (.tool_response | gsub(\"(sk-|ghp_)[A-Za-z0-9]{20,}\"; \"[REDACTED]\"))}'"}
```

If the original call was `{"command": "npm test", "timeout": 60000}`, the
tool runs with `{"command": "<rewritten>", "timeout": 60000}` — `timeout` is
preserved.

## Authoring Checklist

1. Add `#!/usr/bin/env bash` and `set -euo pipefail` (for shell scripts).
2. `chmod +x` the script.
3. Add the entry under `hooks.PreToolUse` in `crush.json` with the right matcher.
4. Decide intent: inject context (omit `decision`), auto-approve (`"allow"`),
   block (`exit 2`), or halt (`exit 49`).
5. If rewriting input, remember `updated_input` is a shallow merge — only
   include the keys you want to change.

## Debugging

- Timeouts kill the hook silently and the tool call proceeds. Bump `timeout` if needed.
- Non-zero exit codes other than 2/49 are logged but don't block — check Crush logs.
- Use `echo "debug info" >&2` for logging without corrupting stdout JSON.
- `matcher` is a regex against the tool name. Use `^bash$` (not `bash`) if you
  don't also want to match `mcp_something_bash`.

## Claude Code Compatibility

Crush also accepts Claude Code's `hookSpecificOutput` envelope. One intentional
divergence: Crush treats `updated_input` as shallow-merge, Claude Code replaces.
Existing Claude Code hooks work without modification for the matcher/decision
parts; revisit any that relied on `updatedInput` fully replacing tool input.
