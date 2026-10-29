# Hooks

## General Info

> [!NOTE]
> This document was designed for both humans and agents.

Hooks are user-defined shell scripts that run when various events happen during
the agent lifecycle, allowing you to both build on top of Crush, customize
its behavior, and exert deterministic control over an agent's wily behavior.

Hooks are just shell commands, and were designed to be both simple and future
forward.

### Hot Hook Facts

- Hooks are just shell commands
- Hooks can be written in any language because they're just executables: Bash, Python, Node, Rust, Haskell, whatever
- Hooks are largely Claude Code-compatible: same payload field names, same
  output forms, same exit codes
- Crush ships with a builtin `crush-hook` skill to write, edit, and configure
  hooks; just tell Crush how to configure Crush
- Crush supports 15 hook events covering the full agent lifecycle: tool use,
  session management, turn control, permissions, compaction, and error handling
- Hooks run in parallel for speed, but their results compose in config order
  for determinism

### Some things you can do with hooks

- Block "dangerous" commands: no more `git push -f` or `cabal init`
- Rewrite tool input: turn `node` calls into `deno`, scrub secrets from
  commands, rewrite all mentions of "Haskell" into "Haskell, The Best
  Language", and so on
- Inject context: add notes to the model's context whenever certain tools are
  called. For example: "remember to run gofumpt after editing Go files"
- Auto-approve tools: skip the permission prompt for bash commands that
  you know are safe
- Auto-approve on judgment: run a classifier (rules, an LLM, whatever) only
  for calls that would otherwise prompt, via `PrePermission`
- React to denials: notify, log, or disable things when a request is
  denied, via `PermissionDenied`
- Log certain tool calls

…And lots more. Show us what you're building!

## Baby's First Hook

Let's just dive into it and make a simple hook. This particular hook will
disallow the use of Haskell (but we love you, Simon Peyton Jones).

### Config

The first thing we need to do is set up our hook. Let's add the following to
our **project-level** `crushrc`. Relative paths like `./no-haskell.sh` work
here because the project root is your working directory. If you're configuring
a global hook (`~/.config/crush/crushrc`), use an absolute path instead.

```bash
# PreToolUse is a hook that fires before a tool is used
# Matcher sets the tool we want to hook into
# Command holds the path to the actual hook script
# Name provides a friendly name to show in the TUI
# Timeout ensures a reasonable cancellation window in case something goes wrong
hook add PreToolUse \
  --matcher '^bash$' \
  --command "./no-haskell.sh" \
  --name "no-haskell" \
  --timeout 30
```

Now, let's make our `no-haskell.sh` hook script.

```bash
#!/usr/bin/env bash

# Disallow ghc, cabal, and stack. Pipe the bash command output
# ($CRUSH_TOOL_INPUT_COMMAND) to grep and match on a regexp.
if echo "$CRUSH_TOOL_INPUT_COMMAND" | grep -qE '(^| )((ghc|cabal|stack)(\.exe)?)( |$)'; then

  # Someone is trying to use Haskell. Let's send a message back to the model
  # and user explaining why we're blocking this. Note that we send all feedback
  # like this to stderr.
  echo "No Haskell allowed, kiddo." >&2

  # Now, block the tool call by exiting with code 2.
  exit 2
fi
```

That's basically it. For the full guide on how hooks work, however, read on.

---

## Execution model

Hooks run through Crush's embedded POSIX shell (`mvdan.cc/sh`) — the same
interpreter the `bash` tool uses. Inline commands and shebang-less scripts
execute in-process; scripts with a `#!` shebang dispatch to the named
interpreter via `os/exec`. This contract is identical on macOS, Linux, and
Windows.

What this means in practice:

- **Windows without Unix tooling**: inline shell (`echo`, pipelines, `jq`,
  `grep`), shebang-less `.sh` scripts, inline PowerShell
  (`powershell -Command …`), and `.exe` invocations all work out of the box
  with no WSL, Git Bash, Cygwin, or MSYS required.
- **PowerShell scripts** (`.ps1`) are not auto-dispatched by extension.
  Invoke them explicitly: `powershell -File ./audit.ps1` (or
  `pwsh -File ./audit.ps1`).
- **Shebang'd scripts** require the named interpreter on `PATH`. Git for
  Windows ships `bash.exe`, which makes `#!/bin/bash` and
  `#!/usr/bin/env bash` scripts work on Windows the same way they do on
  Unix. CRLF line endings in the shebang line are tolerated.
- **Permissive shebang fallback**: if the absolute path in a shebang
  doesn't exist (e.g. `#!/bin/bash` on Windows), Crush falls back to a
  `PATH` lookup of the base name (`bash`) before giving up. A debug-level
  log records the fallback. If the interpreter isn't on `PATH` either, the
  hook fails cleanly as a non-blocking warning and the agent proceeds as
  "no opinion".
- **Environment**: every hook sees `CRUSH=1`, `AGENT=crush`, and
  `AI_AGENT=crush` on top of the `CRUSH_*` hook-specific variables. These
  three markers are guaranteed and match what the `bash` tool sets, so
  scripts that detect "am I being run by an AI agent?" behave the same in
  both contexts.
- **Timeout behavior**: when a hook exceeds its timeout, Crush cancels the
  context and waits a short grace period (~1s) for the interpreter to
  yield. If the hook still hasn't returned, Crush abandons it, logs a
  warning, and treats the result as "no opinion" so the agent can proceed.
  Long-running work should honor context cancellation or run in a
  subprocess via a shebang.

## Configuration

Hooks can be added to your `crushrc` (or `.crushrc`) at both the global and
project level, with project level hooks taking precedence. Each hook is one
`hook add` line:

```bash
# name: friendly name shown in the TUI; matcher: regex tested against the
# tool name; timeout: seconds, default 30.
hook add PreToolUse \
  --name "no-rm-rf" \
  --matcher '^bash$' \
  --command "./hooks/my-hot-hook.sh" \
  --timeout 10
hook add TurnEnd \
  --name "ding" \
  --command 'afplay /System/Library/Sounds/Glass.aiff'
```

> [!IMPORTANT]
> The `command` is resolved relative to your **current working directory** —
> not relative to the config file. Relative paths like `./hooks/whatever.sh`
> work fine in project-level `.crushrc` because the project root is also
> your working directory. For **global** config (`~/.config/crush/`),
> however, you must use either an absolute path or an inline command:
>
> ```bash
> # Global ~/.config/crush/crushrc with absolute path
> hook add PreToolUse \
>   --command "/home/you/.config/crush/hooks/no-haskell.sh"
>
> # Or use an inline command:
> # hook add PreToolUse \
> #   --command "echo '{\"decision\":\"allow\"}'"
> ```

Remember, hooks will run in parallel but resolve in config order. Last hook
wins when rewriting input, but first deny wins when blocking.

## Events

Crush supports 15 hook events. Event names are case-insensitive and accept
snake_case (`PreToolUse`, `pre_tool_use`, `PRETOOLUSE` all work). An unknown
event name is a config error.

| Event                | Fires When                                     | Control                                  | Matcher   | Claude Code                 |
| -------------------- | ---------------------------------------------- | ---------------------------------------- | --------- | --------------------------- |
| `PreToolUse`         | Before a tool runs                             | allow/deny, halt, rewrite input, context | tool name | same                        |
| `PostToolUse`        | After a tool succeeds                          | halt, replace output, context, feedback  | tool name | same                        |
| `PostToolUseFailure` | After a tool fails                             | halt, context                            | tool name | same                        |
| `PermissionRequest`  | Permission prompt shown                        | observe                                  | tool name | same (observe only here)    |
| `PrePermission`      | Prompt is about to be shown                    | allow/deny to skip the prompt            | tool name | none                        |
| `PermissionResult`   | Prompt granted, denied, or cancelled           | observe                                  | tool name | none                        |
| `PermissionDenied`   | Prompt was denied                              | observe                                  | tool name | none                        |
| `SessionStart`       | Session created, or resumed with `--continue`  | observe                                  | —         | same                        |
| `SessionEnd`         | App exits (2s budget)                          | observe                                  | —         | same                        |
| `TurnStart`          | Agent begins processing a prompt               | observe                                  | —         | `UserPromptSubmit`          |
| `TurnEnd`            | Agent finishes a turn                          | observe                                  | —         | `Stop`                      |
| `StopFailure`        | Turn ends with a provider error                | observe                                  | —         | same                        |
| `Interrupt`          | User cancels a turn (ctrl-c, escape)           | observe                                  | —         | none                        |
| `PreCompact`         | Before context compaction (auto or `/compact`) | block                                    | —         | same                        |
| `PostCompact`        | After context compaction                       | observe                                  | —         | same                        |

**Matcher** is a regex tested against the tool name (e.g. `^bash$`,
`^(edit|write)$`, `^mcp_`). Omit it to match every tool. It is only accepted on
the events that carry a tool name; configuring one elsewhere is a config error.

**Scope**: tool and turn events only fire for the **top-level agent**.
Sub-agents (the task tool, `agentic_fetch`, and friends) fire none of them,
so a notification hook on `TurnEnd` speaks once per prompt rather than once
per delegated turn. The outer sub-agent tool call itself _is_ hooked.

**Permission events** fire only when a prompt is actually shown. Calls that a
`PreToolUse` hook allowed, that are on the allow-list, or that run in YOLO
mode never prompt and so fire neither. `PermissionResult` always follows its
`PermissionRequest`, even when the turn is cancelled while the prompt is open;
its `decision` field says which way it went.

**SessionEnd** fires once, for the session you were last working in, after the
running turn has been cancelled (so you will see `Interrupt` first). It has a
two second budget shared by every `SessionEnd` hook, so keep it to enqueueing
or notifying; do the heavy work elsewhere.

### What each control field does

- **`decision: "allow"`** — pre-approves the tool call, skips the permission
  prompt. `"deny"` blocks it; the model sees the error and may retry.
- **`halt: true`** — ends the entire turn. For `PreCompact`, blocks compaction
  instead.
- **`updated_input`** — for `PreToolUse`: shallow-merge patch against tool
  input.
- **`updated_output`** — for `PostToolUse`: replaces what the model sees as the
  tool's output.
- **`context`** — string or array appended to what the model sees. Works on
  the tool events.
- **`reason`** with `decision: "deny"` on `PostToolUse` — the tool already
  ran, so instead of blocking anything the reason is handed to the model as
  feedback ("run the tests first"). This is Claude Code's `decision: "block"`
  and it works the same way here.

Events marked "observe" ignore all control fields. They exist for logging,
monitoring, and external integrations (e.g. herdr, dashboards). To auto-approve
tools, use `PreToolUse` with `decision: "allow"` rather than
`PermissionRequest`.

Blocking compaction (`PreCompact`) leaves the conversation as it was. When the
compaction was automatic, the turn ends there, since the next step would only
hit the context limit again; send another message to carry on. A blocked
`/compact` reports the hook's reason.

### Control field examples

**PreToolUse — rewrite input:**

```bash
#!/usr/bin/env bash
# Replace npm with bun in every bash call.
jq -c 'select(.tool_input.command | startswith("npm "))
  | {updated_input: {command: (.tool_input.command | sub("^npm "; "bun "))}}'
```

**PostToolUse — redact secrets from output:**

```bash
#!/usr/bin/env bash
# Replace anything that looks like an API key in tool output.
jq -c '{updated_output: (.tool_response | gsub("(sk-|ghp_|AKIA)[A-Za-z0-9]{20,}"; "[REDACTED]"))}'
```

**PostToolUse — halt on unexpected output:**

```bash
#!/usr/bin/env bash
# Stop the turn if bash output contains a credential leak warning.
if jq -r '.tool_response' | grep -qi 'credential.*leak'; then
  echo '{"halt":true,"reason":"Possible credential leak detected in tool output"}'
fi
```

**PostToolUse — send the model feedback:**

```bash
#!/usr/bin/env bash
# The commit went through, but tell the model what it skipped.
if jq -r '.tool_input.command' | grep -q '^git commit' && [ ! -f .tests-passed ]; then
  echo '{"decision":"deny","reason":"You committed without running the tests. Run them now and amend if needed."}'
fi
```

**PreCompact — skip compaction during critical work:**

```bash
#!/usr/bin/env bash
# Don't compact if we're in the middle of a migration.
if [ -f .migration-in-progress ]; then
  echo '{"halt":true,"reason":"migration in progress"}'
fi
```

**PreToolUse — inject context:**

```bash
#!/usr/bin/env bash
# Remind the model about formatting rules when editing Go files.
if jq -r '.tool_input.file_path // empty' | grep -q '\.go$'; then
  echo '{"context":"Remember: run gofumpt -w after editing Go files."}'
fi
```

**TurnEnd — read the reply aloud:**

```bash
#!/usr/bin/env bash
# Speak the assistant's final message. Background it so the next prompt
# is not held up behind the speech.
jq -r '.last_assistant_message' | say &
```

### Configuration

Hooks are keyed by event name. Only `command` is required:

```jsonc
{
  "hooks": {
    "PreToolUse": [{ "matcher": "^bash$", "command": "./hooks/check.sh" }],
    "PostToolUse": [{ "matcher": "^bash$", "command": "./hooks/redact.sh" }],
    "TurnEnd": [{ "command": "./hooks/notify-idle.sh" }],
    "PreCompact": [{ "command": "./hooks/maybe-skip-compact.sh" }],
  },
}
```

### PrePermission

This hook fires **only when a permission request is about to prompt the user**
— after allowlists, hook approvals (`PreToolUse` allows), session auto-approvals,
and previously granted permissions have all been exhausted. It lets external
policy (rules engines, classifiers, LLMs) decide whether to grant or deny
without ever showing the prompt.

**Matched against**: the tool name (same as `PreToolUse`).

**Scope**: unlike `PreToolUse`, this hook runs at the permission-service level,
so it fires for permission requests from **any agent** (top-level and
sub-agents). The payload includes `session_id` so hooks can scope decisions.

**Ordering**: `PrePermission` runs while holding the permission service's
request mutex, so a slow hook delays other concurrent permission requests.
Keep timeouts short (5–10 s recommended) and prefer deterministic rules over
LLM calls when latency matters.

Decisions:

- `"allow"` — grant immediately; no prompt.
- `"deny"` with `reason` — block immediately; the reason surfaces in the UI
  notification and in the agent's error response.
- Omitted / `null` — fall through to the normal prompt.

`updated_input` is **ignored** for this event: the tool call has already been
accepted by the agent and cannot be rewritten at this stage.

Example — auto-approve safe bash commands via an external classifier:

```jsonc
{
  "hooks": {
    "PrePermission": [
      {
        "matcher": "^bash$",
        "command": "./hooks/auto-classify.sh",
        "timeout": 8,
        "include_transcript": true
      }
    ]
  }
}
```

### PermissionDenied

This hook fires **after** a permission request has been denied (whether by the
user pressing deny in the TUI, by a `PrePermission` hook, or by any other
denial path). It is fire-and-forget: the hook does not block the denial and its
output is ignored. Use it for observability — logging, notifications, metrics,
or disabling features after repeated denials.

**Matched against**: the tool name.

**Scope**: like `PrePermission`, this runs at the permission-service level and
fires for denials from any agent.

The dispatch is asynchronous with a background context so a slow hook never
delays the denial itself.

Example — log every denial:

```jsonc
{
  "hooks": {
    "PermissionDenied": [
      {
        "command": "echo \"$(date -Iseconds) DENIED $CRUSH_TOOL_NAME\" >> ./denials.log"
      }
    ]
  }
}
```

## Building Hooks

When a hook fires, Crush:

1. Filters hooks whose `matcher` regex matches the tool name (no matcher = match
   all).
2. Deduplicates by `command` (identical commands run once).
3. Runs all matching hooks **in parallel** through Crush's embedded POSIX
   shell (see [Execution model](#execution-model)).
4. Waits for all to finish (or time out), then aggregates results **in config
   order**: deny wins over allow, allow wins over none; `updated_input` patches
   shallow-merge in order.
5. Applies the result **before** permission checks. If the aggregated decision
   is `deny`, the tool call is blocked and you never see a permission prompt
   for it. If it's `allow`, Crush treats that as affirmative pre-approval and
   also skips the prompt. Silence (no decision) falls through to the normal
   permission flow.

Note that you can omit `matcher` and match in your shell script instead,
however you'll incur some additional overhead as Crush will still parse and
run each hook.

### Input

Each hook receives data two ways: environment variables and stdin (as JSON).
Environment variables are typically easier to work with, with JSON being
available when input is more complex.

#### Environment Variables

The available environment variables are:

| Variable                     | Description                                    |
| ---------------------------- | ---------------------------------------------- |
| `CRUSH`                      | Always `1` when running under Crush.           |
| `AGENT`                      | Always `crush`.                                |
| `AI_AGENT`                   | Always `crush`.                                |
| `CRUSH_EVENT`                | The hook event name (e.g. `PreToolUse`).       |
| `CRUSH_TOOL_NAME`            | The tool being called (e.g. `bash`).           |
| `CRUSH_SESSION_ID`           | Current session ID.                            |
| `CRUSH_CWD`                  | Working directory.                             |
| `CRUSH_PROJECT_DIR`          | Project root directory.                        |
| `CLAUDE_PROJECT_DIR`         | Same as `CRUSH_PROJECT_DIR`, for scripts written for Claude Code. |
| `CRUSH_TOOL_INPUT_COMMAND`   | For `bash` calls: the shell command being run. |
| `CRUSH_TOOL_INPUT_FILE_PATH` | For file tools: the target file path.          |
| `CRUSH_TRANSCRIPT`             | Reasoning-blind conversation excerpt (opt-in). |

The `CRUSH_TOOL_*` variables are only set on events that carry a tool call.

The `CRUSH`, `AGENT`, and `AI_AGENT` markers are also set by the `bash`
tool, so a script can detect "am I running under Crush?" the same way in
either context.

#### JSON

Standard input provides the full context as JSON:

```jsonc
{
  "event": "PreToolUse", // Hook event name
  "hook_event_name": "PreToolUse", // Same thing, under Claude Code's name
  "session_id": "313909e", // Current session ID
  "cwd": "/home/user/project", // Working directory
  "tool_name": "bash", // The tool being called
  "tool_input": { "command": "rm -rf /" }, // The tool's input
  "tool_use_id": "call_01", // The tool call ID
  // Present only when the hook opts in via include_transcript.
  "transcript": "User: please fix the failing tests\nAction: bash {\"command\":\"npm test\"}"
}
```

Note that `tool_input` field contains the raw JSON the model sent to the tool.
Events without a tool call omit the `tool_*` fields, and some events add one
of their own (`tool_response`, `last_assistant_message`, `error`, `decision`,
`source`, `reason`, `trigger`); see the [reference](#reference) for the full
list.

When present, `transcript` is a reasoning-blind excerpt of the recent session
conversation: user messages and tool calls only. Assistant prose, tool outputs,
system prompts, and reasoning content are stripped so hook classifiers judge
the conversation rather than the model's own argumentation or possibly-injected
tool results. Each entry is middle-truncated to keep the excerpt bounded.

To parse the stdin JSON in your hook script, read from stdin and use a tool like
`jq`:

```bash
#!/usr/bin/env bash
read -r input
tool_name=$(echo "$input" | jq -r '.tool_name')
command=$(echo "$input" | jq -r '.tool_input.command // empty')
```

You can also use tools like Python:

```python
#!/usr/bin/env python3
import json, sys

data = json.load(sys.stdin)
tool_name = data.get("tool_name", "")
command = data.get("tool_input", {}).get("command", "")
```

### Output

Hooks communicate back to Crush via **exit code** and `stdout`/`stderr`. The
simplest way to do this is to return an error code and print additional context
to stderr. For example:

```bash
# Here, error code 2 blocks the tool, using stderr as the reason:
if some_bad_condition; then
  echo "Blocked: reason here" >&2
  exit 2
fi
```

| Exit Code | Meaning                                                                                                         |
| --------- | --------------------------------------------------------------------------------------------------------------- |
| 0         | Success. Stdout is parsed as JSON (see fields below).                                                           |
| 2         | **Block the tool.** Stderr is the deny reason (falling back to the JSON `reason`, if you printed one).           |
| 49        | **Halt the turn.** Stderr is the halt reason.                                                                   |
| Other     | Non-blocking error, logged and ignored. As in Claude Code, a JSON object on stdout still counts and is applied. |

The difference between exit 2 and exit 49:

- **Exit 2** blocks the current tool call. The agent sees the error and can try
  something else.
- **Exit 49** halts the whole turn. The agent doesn't get to respond further;
  the user takes over. Use this when something is wrong enough that the agent
  shouldn't keep trying. 49 sits in an empty slice of the exit-code space —
  between the generic-error range (1-30), the BSD `sysexits.h` range (64-78),
  and the killed-by-signal range (128+) — so it can't be hit by accident.

That said, if you need more control, or if you need to rewrite input, you can
use JSON on stdout. Exit 0 and print a JSON object to provide context, update
the input, or still deny/halt with a reason:

```jsonc
{
  "version": 1, // Output envelope version. Optional; defaults to 1.
  "decision": "allow", // "allow", "deny", or null. Omit for no opinion.
  "halt": false, // If true, halts the turn entirely.
  "reason": "LGTM", // Shown when denying or halting.
  "context": "Scrubbed secrets", // String or array of strings. Appended to what the model sees.
  "updated_input": { "command": "…" }, // Shallow-merged into the tool's input before execution.
}
```

`version` is an optional integer at the top of the envelope. It defaults to `1`
if omitted. Unknown higher versions are still parsed; the field exists so the
envelope can evolve without a compatibility shim.

`decision: "allow"` is **affirmative**: it pre-approves the tool call and
bypasses the permission prompt entirely. Silence (no `decision`, or
`decision: null`) means "no opinion" — the tool still goes through the
normal permission flow. Use `"allow"` when you want to auto-approve; omit it
when you only want to inject context or rewrite input without also vouching
for the call.

`updated_input` is a shallow-merge patch. Keys you include overwrite matching
keys in `tool_input`; keys you don't include are preserved. If the model called
`bash` with `{"command": "npm test", "timeout": 60000}` and your hook returns
`{"updated_input": {"command": "bun test"}}`, the tool runs with
`{"command": "bun test", "timeout": 60000}` — the timeout isn't dropped. The
merge is shallow: nested objects are replaced wholesale, not deep-merged.

`halt: true` stops the turn entirely. The agent doesn't get to respond further;
the user takes over. The exit-code shorthand is `exit 49` with stderr as the
reason.

`context` accepts either a string or an array of strings. Use the string form
for a single observation; use the array form when a hook produces multiple
distinct notes and you'd rather not concatenate them by hand. Empty strings and
empty array entries are dropped.

Here's a full shell script that produces this JSON:

```bash
#!/usr/bin/env bash
# Example: rewrite a bash command using RTK

read -r input
original_cmd=$(echo "$input" | jq -r '.tool_input.command')
rewritten=$(secret-scrubber rewrite "$original_cmd")

cat <<EOF
{
  "decision": "allow",
  "context": "Scrubbed secrets",
  "updated_input": {"command": "$rewritten"}
}
EOF
```

### Multiple Hooks

Hooks run in parallel, but their results compose in config order. Whichever hook
finishes first doesn't get to "win" by virtue of timing; composition is
deterministic based on the order hooks appear in `crushrc`.

When multiple hooks match the same tool call:

- If **any** hook denies, the tool call is blocked. `reason` values are
  concatenated in config order (newline-separated).
- If **any** hook halts, the turn ends after the tool call is blocked.
- If no hook denies or halts but at least one allows, the tool call proceeds
  **and the permission prompt is skipped**.
- `context` values are concatenated in config order. Strings and arrays compose
  uniformly — each string becomes one entry, and array entries are flattened in.
- `updated_input` patches shallow-merge in config order against the original
  tool input. Later hooks override earlier ones on colliding keys. If denied or
  halted, `updated_input` patches are ignored.

### Timeouts

If a hook exceeds its timeout, Crush cancels its context and treats the
result as a non-blocking error so the tool call proceeds. The default
timeout is 30 seconds. Shebang-dispatched subprocesses are killed through
`exec.CommandContext`; in-process hooks get a short grace period to yield
and are then abandoned (the agent moves on regardless). Long-running work
should honor context cancellation or run out-of-process via a shebang.

## Examples

### Block destructive commands

Prevent the agent from running `rm -rf` in bash:

```bash
hook add PreToolUse \
  --matcher '^bash$' \
  --command "./hooks/no-rm-rf.sh"
```

`hooks/no-rm-rf.sh`:

```bash
#!/usr/bin/env bash
# Block rm -rf commands in the bash tool. Otherwise stay silent so the
# normal permission flow runs.

if echo "$CRUSH_TOOL_INPUT_COMMAND" | grep -qE 'rm\s+-(rf|fr)\s+/'; then
  echo "Refusing to run rm -rf against root" >&2
  exit 2
fi

exit 0
```

### Auto-approve read-only tools

Skip the permission prompt for tools that can't change anything. The hook
returns `decision: "allow"`, which tells Crush to pre-approve the call:

```bash
hook add PreToolUse \
  --matcher '^(view|ls|grep|glob)$' \
  --command "echo '{\"decision\":\"allow\"}'"
```

No script file needed — the command is inline. Every `view`/`ls`/`grep`/`glob`
call now runs without prompting. Add the `bash` tool to this list at your own
risk; consider a more targeted allowlist instead:

```bash
#!/usr/bin/env bash
# hooks/safe-bash.sh — auto-approve read-only bash commands.

case "$CRUSH_TOOL_INPUT_COMMAND" in
  ls*|cat*|grep*|rg*|echo*|pwd*)
    echo '{"decision":"allow"}'
    ;;
  *)
    # Silent — fall through to the normal permission prompt.
    exit 0
    ;;
esac
```

### Inject context into file writes

Add a reminder to the model whenever it writes a Go file:

```bash
hook add PreToolUse \
  --matcher '^(edit|write|multiedit)$' \
  --command "./hooks/go-context.sh"
```

`hooks/go-context.sh`:

```bash
#!/usr/bin/env bash
# Remind the model about Go formatting when editing .go files.
# Emit context only; stay silent on `decision` so the normal permission
# prompt still runs for edits/writes.

if [[ "$CRUSH_TOOL_INPUT_FILE_PATH" == *.go ]]; then
  echo '{"context": "Remember: run gofumpt after editing Go files."}'
else
  echo '{}'
fi
```

### Block all MCP tools

The `command` can be inline. This one-liner matches all MCP tools and blocks
them:

```bash
hook add PreToolUse \
  --matcher '^mcp_' \
  --command "echo 'MCP tools are disabled' >&2; exit 2"
```

### Log every tool call

With no `matcher` this fires for every tool. It exits 0 with no stdout so the
tool call always proceeds.

```bash
hook add PreToolUse \
  --command "echo \"$(date -Iseconds) $CRUSH_TOOL_NAME\" >> ./tools.log"
```

### A real-world Example

For a more practical example, see [`rtk-rewrite.sh`](./examples/rtk-rewrite.sh),
which demonstrates how to rewrite tool input using
[RTK](https://github.com/rtk-ai/rtk) to save tokens.

### Using other languages

Hooks aren't limited to shell scripts: any executable works. Here's the same
"block rm -rf" example in some other languages.

#### Lua

```bash
hook add PreToolUse \
  --matcher '^bash$' \
  --command "lua ./hooks/no-rm-rf.lua"
```

```lua
local input = io.read("*a")
local tool_input = input:match('"command":"(.-)"') or ""

if tool_input:match("rm%s+%-[rf][rf]%s+/") then
  io.stderr:write("Refusing to run rm -rf against root\n")
  os.exit(2)
end
```

#### JavaScript

```bash
hook add PreToolUse \
  --matcher '^bash$' \
  --command "node ./hooks/no-rm-rf.js"
```

```js
let input = "";
process.stdin.on("data", (chunk) => (input += chunk));
process.stdin.on("end", () => {
  const { tool_input: toolInput } = JSON.parse(input);

  if (/rm\s+-[rf]{2}\s+\//.test(toolInput.command)) {
    process.stderr.write("Refusing to run rm -rf against root\n");
    process.exit(2);
  }
});
```

---

## Claude Code compatibility

Crush hooks are largely compatible with [Claude Code
hooks](https://code.claude.com/docs/en/hooks). Most hook scripts written for
Claude Code run under Crush unchanged:

- **Payload fields** use Claude Code's names where one exists:
  `hook_event_name`, `session_id`, `cwd`, `tool_name`, `tool_input`,
  `tool_use_id`, `tool_response`, `error`, `last_assistant_message`, `source`,
  `reason`, `trigger`. `CLAUDE_PROJECT_DIR` is exported alongside
  `CRUSH_PROJECT_DIR`.
- **Output** accepts Claude Code's forms as well as Crush's:
  `hookSpecificOutput` (`permissionDecision`, `permissionDecisionReason`,
  `updatedInput`, `updatedToolOutput`, `additionalContext`), the top-level
  `decision: "block"` with `reason`, and `continue: false` with `stopReason`.
- **Exit codes** behave the same: 0 applies the JSON, 2 blocks with stderr as
  the reason, and any other code is a non-blocking error unless stdout held a
  JSON object.

Where Crush differs:

- **Config shape** is flat: a list of `{matcher, command}` per event, not
  Claude Code's nested `matcher` groups with `hooks` arrays. Only `command`
  hooks exist; there are no `http`, `prompt`, or `agent` hooks.
- **Event names**: `TurnStart`, `TurnEnd`, `Interrupt`, and `PermissionResult`
  are Crush's names for the per-turn events; the table in [Events](#events)
  maps each one to its Claude Code counterpart. `PermissionRequest` is
  observe-only here; use `PreToolUse` to allow or deny.
- **`updated_input` on `PreToolUse`** is a shallow-merge patch against the
  original `tool_input` rather than a full replacement, so keys you omit are
  preserved. Claude Code's `updatedInput` is treated the same way.
- **`tool_response`** is the tool's text output as one string, not a
  per-tool object. Crush's own field for replacing it is `updated_output`;
  Claude Code's `updatedToolOutput` maps onto it.
- **`halt` and exit 49** have no Claude Code equivalent; they end the turn.

---

## Reference

This is the official reference of the narrative above. If prose and this section
disagree, the prose should be presumed canonical for intent, while this section
is canonical for shape.

Both the stdin payload and the output envelope have **common fields** that apply
to every event and **per-event fields** that only some events recognize. When an
event doesn't understand a field, it's ignored.

### Hook config

Each entry under a `hooks.<EventName>` array:

```jsonc
{
  // string. Optional. Friendly display name shown in the TUI. Falls back to
  // command when omitted.
  "name": "no-rm-rf",

  // string. Optional. Regex tested against the tool name. Omit to match all.
  "matcher": "^bash$",

  // string. Required. Shell command to run.
  "command": "./hooks/my-hook.sh",

  // number. Optional. Seconds before the hook is killed. Defaults to 30.
  "timeout": 10,

  // boolean. Optional. When true, the hook receives a reasoning-blind
  // excerpt of the recent session conversation in the `transcript` stdin
  // field and as CRUSH_TRANSCRIPT. Assistant prose and tool outputs are
  // stripped. Opt-in only; hooks that omit it are unaffected.
  "include_transcript": false,
}
```

### Stdin payload (common)

Present in every hook event:

```jsonc
{
  // string. Hook event name.
  "event": "PreToolUse",

  // string. The same name again, under the field Claude Code uses.
  "hook_event_name": "PreToolUse",

  // string. Current session ID.
  "session_id": "313909e",

  // string. Working directory when invoked.
  "cwd": "/home/user/project",
}
```

### Stdin payload — tool events

`PreToolUse`, `PostToolUse`, `PostToolUseFailure`, `PermissionRequest`, and
`PermissionResult` extend the common payload with the tool call:

```jsonc
{
  // ...common fields...

  // string. The tool being called.
  "tool_name": "bash",

  // object. Raw JSON input the model sent to the tool. Shape is per-tool.
  // For the permission events this is the request the prompt showed.
  "tool_input": {
    "command": "npm test",
  },

  // string. The tool call ID.
  "tool_use_id": "call_01",

  // string. Present only when the hook opts in via include_transcript.
  // Reasoning-blind excerpt: user messages and tool calls only.
  "transcript": "User: please fix the failing tests\nAction: bash {\"command\":\"npm test\"}",
}
```

`PostToolUse` adds the tool's output, which is what `updated_output` replaces:

```jsonc
{
  // string. Always present, even when the tool printed nothing.
  "tool_response": "PASS\nok  \texample.com/testproject\t0.004s",
}
```

`PostToolUseFailure` adds the failure instead:

```jsonc
{
  // string. The tool's error text.
  "error": "exit status 1\nFAIL\t...",
}
```

`PermissionResult` adds how the prompt was resolved:

```jsonc
{
  // "granted" | "denied" | "cancelled". Cancelled means the turn was
  // cancelled while the prompt was open.
  "decision": "granted",
}
```

### Stdin payload — turn events

`TurnStart` and `Interrupt` use only the common fields. `TurnEnd` adds the
assistant's reply and `StopFailure` adds the provider error:

```jsonc
{
  // string (TurnEnd). The assistant's text for the turn.
  "last_assistant_message": "Here's what I found...",

  // string (StopFailure). The provider error.
  "error": "Rate limit reached",
}
```

### Stdin payload — session and compaction events

```jsonc
{
  // "startup" | "resume" (SessionStart). Resume means --continue or --last.
  "source": "startup",

  // string (SessionEnd). Currently always "exit".
  "reason": "exit",

  // "auto" | "manual" (PreCompact, PostCompact). Manual is /compact.
  "trigger": "auto",
}
```

### Stdin payload — PrePermission

Same shape as `PreToolUse`. The `tool_input` contains the permission request's
`Params` marshaled as JSON (e.g. `{"command":"..."}` for bash, or
`{"file_path":"...","old_content":"...","new_content":"..."}` for edits).

### Stdin payload — PermissionDenied

Same shape as `PrePermission`. Fired after the denial; output is ignored.

### Output envelope (common)

Fields a hook may print to stdout on exit 0. All are optional and apply to every
event:

```jsonc
{
  // number. Defaults to 1. Unknown higher values still parse; exists for
  // forward-compat.
  "version": 1,

  // boolean. If true, ends the turn entirely. User takes over.
  "halt": false,

  // string. Shown when denying (to the model) or halting (to the model and
  // user).
  "reason": "not allowed",

  // string | string[]. Appended to what the model sees. Empty entries are
  // dropped.
  "context": "Rewrote with RTK",
}
```

### Output envelope — PreToolUse

Extends the common envelope:

```jsonc
{
  // ...common fields...

  // "allow" | "deny" | null. null/omitted = no opinion, the tool still goes
  // through the normal permission prompt. "allow" is affirmative: pre-approves
  // the tool call and bypasses the prompt. "deny" blocks the call; the model
  // sees the error and may try something else.
  "decision": "allow",

  // object. Shallow-merge patch against tool_input. Nested objects are
  // replaced wholesale, not deep-merged.
  "updated_input": {
    "command": "bun test",
  },
}
```

### Output envelope — PostToolUse

Same as the common envelope plus `updated_output`, a string that **replaces**
what the model sees as the tool's response. The tool has already
run, so `decision: "deny"` cannot block it; its `reason` is appended to the
output as feedback for the model instead.

`PostToolUseFailure` accepts `halt` and `context` only.

### Output envelope — PrePermission

Only `decision` and `reason` apply. `updated_input` is **ignored** (the tool
call has already been accepted by the agent and cannot be rewritten at this
stage). `halt` is also ignored; use `decision: "deny"` instead.

```jsonc
{
  // "allow" | "deny" | null. "allow" grants immediately without prompting.
  // "deny" blocks immediately; reason surfaces in the UI notification and
  // in the agent's error response. null/omitted falls through to the prompt.
  "decision": "deny",

  // string. Shown when denying. Recommended to include a category tag so
  // the agent can retry safely, e.g. "[data-exfiltration] sending creds to
  // external endpoint".
  "reason": "[destructive] rm -rf outside workspace",
}
```

### Output envelope — PermissionDenied

Ignored entirely. The event is fire-and-forget observability; exit codes and
stdout have no effect on the denial.

### Output envelope — PreCompact

Same as the common envelope. `halt: true`, `decision: "deny"`, or exit 2
blocks compaction and leaves the conversation as it was. Claude Code's
`decision: "block"` works too.

### Exit codes

| Code  | Meaning                                                                                               |
| ----- | ----------------------------------------------------------------------------------------------------- |
| `0`   | Success. Stdout is parsed as the output envelope.                                                     |
| `2`   | Block. Stderr becomes the reason, falling back to the envelope's `reason`.                            |
| `49`  | Halt the whole turn. Stderr becomes the halt reason.                                                  |
| other | Non-blocking error, logged and ignored. If stdout holds a JSON object it is applied all the same.     |

Exit `2` only blocks on events that can block something (`PreToolUse`,
`PreCompact`). On `PostToolUse` it becomes feedback for the model; on
observe-only events it is logged.

### Aggregation

When multiple hooks match the same event, results compose in **config order**.

Universal rules:

1. `halt` is sticky: if any hook halts, the turn ends.
2. `reason` values concatenate with `\n` in config order. Halt-only hooks
   without a deny still contribute their reason.
3. `context` values concatenate with `\n` in config order. String entries and
   array entries flatten uniformly.

PreToolUse-specific rules:

1. `decision` precedence: `deny` > `allow` > `null`. First deny determines the
   outcome; subsequent allows don't override. If the final aggregated decision
   is `allow`, Crush pre-approves the tool call and skips the permission
   prompt. If it's `null` (no hook allowed), the tool goes through the normal
   permission flow.
2. `updated_input` patches shallow-merge sequentially against the original
   `tool_input`. Later patches override earlier ones on colliding keys. Patches
   are **ignored** if the final decision is deny or halt.

PostToolUse-specific rules:

6. `updated_output` **replaces** the tool output. When several hooks set it,
   the last one in config order wins.

PreCompact-specific rules:

7. `halt` or `deny` blocks compaction and leaves the conversation as it was.

### Environment variables

See [Environment Variables](#environment-variables) above for the full list.

---

## Whatcha think?

We'd love to hear your thoughts on this project. Need help? We gotchu. You can
find us on:

- [Twitter](https://twitter.com/charmcli)
- [Slack](https://charm.land/slack)
- [Discord](https://charm.land/discord)
- [The Fediverse](https://mastodon.social/@charmcli)
- [Bluesky](https://bsky.app/profile/charm.land)

---

Part of [Charm](https://charm.land).

<a href="https://charm.land/"><img alt="The Charm logo" width="400" src="https://stuff.charm.sh/charm-banner-softy.jpg" /></a>

<!--prettier-ignore-->
Charm热爱开源 • Charm loves open source
