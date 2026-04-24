# Hooks

> [!NOTE]
> This document was designed for both humans and agents.

Hooks are user-defined shell commands that fire at specific points during
Crush's execution, giving you deterministic control over an agent's wily
behavior.

Hooks in Crush are shell-based, with a focus on simplicity. This allows hooks
to effectively be written in any language. In this document we'll primarily
focus on Bash for simplicity's sake, though we'll include some examples in
other languages at the end, too.

### Hot Hook Facts

- Hooks run before permission checks. If a hook denies a tool call, you'll
  never see a permission prompt for it.
- Hooks are also compatible with hooks in Claude Code, however this document
  covers the Crush-specific API only. One intentional divergence: Crush
  treats `updated_input` as a shallow-merge patch rather than a full
  replacement — see [Output](#output) below.
- Crush currently supports just one hook, `PreToolUse`, with plans to support
  the full gamut. If there's a hook you'd like to see, let us know.

## Configuration

Hooks can be added to your `crush.json` at both the global and project-level,
with project level hooks taking precedence.

```jsonc
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "bash",                   // regex tested against the tool name
        "command": "./hooks/my-hot-hook.sh", // the path to the hook
        "timeout": 10,                       // in seconds; default 30
      },
    ],
  },
}
```

Hooks are keyed by event name. Only `command` is required; omit `matcher` to
match all tools.

## Events

Here are the events you can hook into (spoiler: there's currently just one):

### PreToolUse

This hook fires before every tool call. Use it to block dangerous commands,
enforce policies, rewrite tool input, or inject context the model should see.

**Matched against**: the tool name (e.g. `bash`, `edit`, `write`,
`mcp_github_create_pull_request`).

> [!NOTE]
> Event names are case insensitive and snake-caseable, so `PreToolUse`,
> `pretooluse`, `PRETOOLUSE`, `pre_tool_use`, and `PRE_TOOL_USE` all work.

## Baby's First Hook

Hooks are just shell scripts. Go crazy.

```bash
#!/usr/bin/env bash

# Log all bash tool calls to a file.
printf "%s: %s / %s" \
    "$(date -Iseconds)" \
    "$CRUSH_SESSION_ID" \
    "$CRUSH_TOOL_INPUT_COMMAND" >> ./bash.log
```

That's basically it. For the full guide on how hooks work, however, read on.

## Building Hooks

When a hook fires, Crush:

1. Filters hooks whose `matcher` regex matches the tool name (no matcher =
   match all).
2. Deduplicates by `command` (identical commands run once).
3. Runs all matching hooks **in parallel** as subprocesses.
4. Waits for all to finish (or time out), then aggregates results **in config
   order**: deny wins over allow, allow wins over none; `updated_input`
   patches shallow-merge in order.

Note that you can omit `matcher` and match in your shell script instead,
however you'll incur some additional overhead as Crush will `exec` each
script.

### Input

Each hook receives data two ways: environment variables and stdin (as JSON).
Environment variables are typically easier to work with, with JSON being
available when input is more complex.

#### Environment Variables

The available environment variables are:

| Variable                     | Description                                    |
| ---------------------------- | ---------------------------------------------- |
| `CRUSH_EVENT`                | The hook event name (e.g. `PreToolUse`).       |
| `CRUSH_TOOL_NAME`            | The tool being called (e.g. `bash`).           |
| `CRUSH_SESSION_ID`           | Current session ID.                            |
| `CRUSH_CWD`                  | Working directory.                             |
| `CRUSH_PROJECT_DIR`          | Project root directory.                        |
| `CRUSH_TOOL_INPUT_COMMAND`   | For `bash` calls: the shell command being run. |
| `CRUSH_TOOL_INPUT_FILE_PATH` | For file tools: the target file path.          |

#### JSON

Standard input provides the full context as JSON:

```jsonc
{
  "event": "PreToolUse",                // Hook event name
  "session_id": "313909e",              // Current session ID
  "cwd": "/home/user/project",          // Working directory
  "tool_name": "bash",                  // The tool being called
  "tool_input": {"command": "rm -rf /"} // The tool's input
}
```

Note that `tool_input` field contains the raw JSON the model sent to the tool.

To parse the stdin JSON in your hook script, read from stdin and use a tool
like `jq`:

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
simplest way to do this is to return an error code and print additional
context to stderr. For example:

```bash
# Here, error code 2 blocks the tool, using stderr as the reason:
if some_bad_condition; then
  echo "Blocked: reason here" >&2
  exit 2
fi
```

| Exit Code | Meaning                                                          |
| --------- | ---------------------------------------------------------------- |
| 0         | Success. Stdout is parsed as JSON (see fields below).            |
| 2         | **Block the tool.** Stderr is used as the deny reason (no JSON). |
| 49        | **Halt the turn.** Stderr is used as the halt reason (no JSON).  |
| Other     | Non-blocking error. Logged and ignored — the tool call proceeds. |

The difference between exit 2 and exit 49:

- **Exit 2** blocks the current tool call. The agent sees the error and can
  try something else.
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
  "version": 1,                     // Output envelope version. Optional; defaults to 1.
  "decision": "allow",              // "allow", "deny", or null. Omit for no opinion.
  "halt": false,                    // If true, halts the turn entirely.
  "reason": "LGTM",                 // Shown when denying or halting.
  "context": "Scrubbed secrets",    // String or array of strings. Appended to what the model sees.
  "updated_input": {"command": "…"} // Shallow-merged into the tool's input before execution.
}
```

`version` is an optional integer at the top of the envelope. It defaults to
`1` if omitted. Unknown higher versions are still parsed; the field exists so
the envelope can evolve without a compatibility shim.

`updated_input` is a shallow-merge patch. Keys you include overwrite matching
keys in `tool_input`; keys you don't include are preserved. If the model
called `bash` with `{"command": "npm test", "timeout": 60000}` and your hook
returns `{"updated_input": {"command": "bun test"}}`, the tool runs with
`{"command": "bun test", "timeout": 60000}` — the timeout isn't dropped.
The merge is shallow: nested objects are replaced wholesale, not deep-merged.

`halt: true` stops the turn entirely. The agent doesn't get to respond
further; the user takes over. The exit-code shorthand is `exit 49` with
stderr as the reason.

`context` accepts either a string or an array of strings. Use the string form
for a single observation; use the array form when a hook produces multiple
distinct notes and you'd rather not concatenate them by hand. Empty strings
and empty array entries are dropped.

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

Hooks run in parallel, but their results compose in config order. Whichever
hook finishes first doesn't get to "win" by virtue of timing; composition is
deterministic based on the order hooks appear in `crush.json`.

When multiple hooks match the same tool call:

- If **any** hook denies, the tool call is blocked. `reason` values are
  concatenated in config order (newline-separated).
- If **any** hook halts, the turn ends after the tool call is blocked.
- If no hook denies or halts but at least one allows, the tool call proceeds.
- `context` values are concatenated in config order. Strings and arrays
  compose uniformly — each string becomes one entry, and array entries are
  flattened in.
- `updated_input` patches shallow-merge in config order against the original
  tool input. Later hooks override earlier ones on colliding keys. If denied
  or halted, `updated_input` patches are ignored.

### Timeouts

If a hook exceeds its timeout, the process is killed and treated as a
non-blocking error and the tool call proceeds. The default timeout is 30
seconds.

## Examples

### Block destructive commands

Prevent the agent from running `rm -rf` in bash:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "^bash$",
        "command": "./hooks/no-rm-rf.sh"
      }
    ]
  }
}
```

`hooks/no-rm-rf.sh`:

```bash
#!/usr/bin/env bash
# Block rm -rf commands in the bash tool.

if echo "$CRUSH_TOOL_INPUT_COMMAND" | grep -qE 'rm\s+-(rf|fr)\s+/'; then
  echo "Refusing to run rm -rf against root" >&2
  exit 2
fi

echo '{"decision": "allow"}'
```

### Inject context into file writes

Add a reminder to the model whenever it writes a Go file:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "^(edit|write|multiedit)$",
        "command": "./hooks/go-context.sh"
      }
    ]
  }
}
```

`hooks/go-context.sh`:

```bash
#!/usr/bin/env bash
# Remind the model about Go formatting when editing .go files.

if [[ "$CRUSH_TOOL_INPUT_FILE_PATH" == *.go ]]; then
  echo '{"decision": "allow", "context": "Remember: run gofumpt after editing Go files."}'
else
  echo '{}'
fi
```

### Block all MCP tools

The `command` can be inline. This one-liner matches all MCP tools and blocks
them:

```jsonc
{"matcher": "^mcp_", "command": "echo 'MCP tools are disabled' >&2; exit 2"}
```

### Log every tool call

With no `matcher` this fires for every tool. It exits 0 with no stdout so the
tool call always proceeds.

```jsonc
{"command": "echo \"$(date -Iseconds) $CRUSH_TOOL_NAME\" >> ./tools.log"}
```

### A real-world Example:

For a more practical example, see [`rtk-rewrite.sh`](./examples/rtk-rewrite.sh),
which demonstrates how to rewrite tool input using [RTK](https://github.com/rtk-ai/rtk) to save tokens.

### Using other languages

Hooks aren't limited to shell scripts: any executable works. Here's the same
"block rm -rf" example in some other languages.

#### Lua

`{"matcher": "^bash$", "command": "lua ./hooks/no-rm-rf.lua"}`

```lua
local input = io.read("*a")
local tool_input = input:match('"command":"(.-)"') or ""

if tool_input:match("rm%s+%-[rf][rf]%s+/") then
  io.stderr:write("Refusing to run rm -rf against root\n")
  os.exit(2)
end

print('{"decision": "allow"}')
```

#### JavaScript

`{"matcher": "^bash$", "command": "node ./hooks/no-rm-rf.js"}`

```js
let input = "";
process.stdin.on("data", (chunk) => (input += chunk));
process.stdin.on("end", () => {
  const { tool_input: toolInput } = JSON.parse(input);

  if (/rm\s+-[rf]{2}\s+\//.test(toolInput.command)) {
    process.stderr.write("Refusing to run rm -rf against root\n");
    process.exit(2);
  }

  console.log(JSON.stringify({ decision: "allow" }));
});
```

---

## Reference

A strict-form summary of the narrative above. When prose and this section
disagree, the prose is canonical for intent; this section is canonical for
shape.

Both the stdin payload and the output envelope have **common fields** that
apply to every event and **per-event fields** that only some events
recognize. When an event doesn't understand a field, it's ignored.

### Hook config

Each entry under a `hooks.<EventName>` array:

| Field     | Type     | Required | Default       | Description                                   |
| --------- | -------- | -------- | ------------- | --------------------------------------------- |
| `matcher` | `string` | no       | `""` (all)    | Regex tested against the tool name.           |
| `command` | `string` | **yes**  | —             | Shell command to execute.                     |
| `timeout` | `number` | no       | `30`          | Seconds before the hook is killed.            |

### Stdin payload (common)

Present in every hook event:

| Field        | Type     | Description                                   |
| ------------ | -------- | --------------------------------------------- |
| `event`      | `string` | Hook event name (e.g. `"PreToolUse"`).        |
| `session_id` | `string` | Current session ID.                           |
| `cwd`        | `string` | Working directory when the hook was invoked.  |

### Stdin payload — PreToolUse

Adds to the common payload:

| Field        | Type     | Description                                                   |
| ------------ | -------- | ------------------------------------------------------------- |
| `tool_name`  | `string` | The tool being called (e.g. `"bash"`).                        |
| `tool_input` | `object` | Raw JSON input the model sent to the tool. Shape is per-tool. |

### Output envelope (common)

Fields a hook may print to stdout on exit 0. All are optional and apply
to every event:

| Field        | Type                  | Default  | Description                                                                        |
| ------------ | --------------------- | -------- | ---------------------------------------------------------------------------------- |
| `version`    | `number`              | `1`      | Envelope version. Unknown higher values still parse; exists for forward-compat.    |
| `halt`       | `boolean`             | `false`  | If `true`, ends the turn entirely. User takes over.                                |
| `reason`     | `string`              | `""`     | Shown when denying (to the model) or halting (to the model and user).              |
| `context`    | `string \| string[]`  | `""`     | Appended to what the model sees. Empty strings and empty entries are dropped.      |

### Output envelope — PreToolUse

Adds to the common envelope:

| Field           | Type                          | Default     | Description                                                                                                      |
| --------------- | ----------------------------- | ----------- | ---------------------------------------------------------------------------------------------------------------- |
| `decision`      | `"allow" \| "deny" \| null`   | `null`      | `null`/omitted = no opinion. `"deny"` blocks the tool call; the model sees the error and may try something else. |
| `updated_input` | `object`                      | `{}` no-op  | Shallow-merge patch against `tool_input`. Nested objects are replaced wholesale, not deep-merged.                |

### Exit codes

| Code    | Meaning                                                                                              |
| ------- | ---------------------------------------------------------------------------------------------------- |
| `0`     | Success. Stdout is parsed as the output envelope.                                                    |
| `2`     | Block this tool call. Stderr becomes the deny reason. Stdout is ignored.                             |
| `49`    | Halt the whole turn. Stderr becomes the halt reason. Stdout is ignored.                              |
| other   | Non-blocking error. Logged and ignored; the tool call proceeds.                                      |

Exit `2` only applies to events that can block something. On events where
there's nothing to block, it's treated as a non-blocking error.

### Aggregation

When multiple hooks match the same event, results compose in **config
order**.

Universal rules:

1. `halt` is sticky: if any hook halts, the turn ends.
2. `reason` values concatenate with `\n` in config order. Halt-only hooks
   without a deny still contribute their reason.
3. `context` values concatenate with `\n` in config order. String entries
   and array entries flatten uniformly.

PreToolUse-specific rules:

4. `decision` precedence: `deny` > `allow` > `null`. First deny determines
   the outcome; subsequent allows don't override.
5. `updated_input` patches shallow-merge sequentially against the original
   `tool_input`. Later patches override earlier ones on colliding keys.
   Patches are **ignored** if the final decision is deny or halt.

### Environment variables

See [Environment Variables](#environment-variables) above for the full list.

---

## Whatcha think?

We'd love to hear your thoughts on this project. Need help? We gotchu. You
can find us on:

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
