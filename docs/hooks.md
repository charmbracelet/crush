# Hooks

> [!NOTE]
> This document was designed for both humans and agents.

Hooks are user-defined shell commands that fire at specific points during
Crush's execution, giving you deterministic control over an agent’s wily
behavior.

Hooks in Crush are shell-based, with a focus on simplicity. This allows hooks to
effectively be written in any language. In this document we'll primarily focus
on Bash for simplicity's sake, though we'll include some examples in other
languages at the end, too.

### Hook Facts

- Hooks run before permission checks. If a hook denies a tool call, you'll
  never see a permission prompt for it.
- Hooks are also compatible with hooks in Claude Code, however this document
  covers the Crush-specific API only.
- Crush currently supports just one hook, `PreToolUse`, with plans to support
  the full gamut. If there's a hook you’d like to see, let us know.

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
        "timeout": 10,
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

## Baby’s First Hook

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

1. Filters hooks whose `matcher` regex matches the tool name (no matcher = match
   all).
2. Deduplicates by `command` (identical commands run once).
3. Runs all matching hooks **in parallel** as subprocesses.
4. Aggregates results: **deny wins** over allow, allow wins over none.

Note that you can omit `matcher` and match in your shell script instead, however
you'll incur some additional overhead as Crush will `exec` each script.

### Input

Each hook receives data two ways: environment variables and stdin (as
JSON). Environment variables are typically easier to work with, with JSON being
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
simplest way to do this is to simply return an error code and print additional
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
| 0         | Success. Stdout is parsed as JSON (see fields above).            |
| 2         | **Block the tool.** Stderr is used as the deny reason (no JSON). |
| Other     | Non-blocking error. Logged and ignored — the tool call proceeds. |

That said, if you need more control, or if you need to rewrite input, you can
use JSON on stdout. Exit 0 and print a JSON object to provide context, update
the input, or still deny with a reason:

```jsonc
{
  "decision": "allow",              // "allow", "deny", or null. Omit for no opinion.
  "reason": "not allowed",          // Shown to the model when denying.
  "context": "Rewrote with RTK",    // Appended to the tool response the model sees.
  "updated_input": {"command": "…"} // Replaces the tool's input before execution.
}
```

Here's a full shell script that produces this JSON:

```bash
#!/usr/bin/env bash
# Example: rewrite a bash command using RTK

read -r input
original_cmd=$(echo "$input" | jq -r '.tool_input.command')
rewritten=$(rtk rewrite "$original_cmd")

cat <<EOF
{
  "decision": "allow",
  "context": "Rewrote with RTK for token savings",
  "updated_input": {"command": "$rewritten"}
}
EOF
```

### Multiple Hooks

When multiple hooks match the same tool call:

- If **any** hook denies, the tool call is blocked. All deny reasons are
  concatenated (newline-separated).
- If no hook denies but at least one allows, the tool call proceeds.
- Context strings from all hooks are concatenated.
- The last non-empty `updated_input` wins. If denied, `updated_input` is
  ignored.

### Timeouts

If a hook exceeds its timeout, the process is killed and treated as
a non-blocking error and the tool call proceeds. The default timeout is 30
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

### Rewrite bash commands with RTK

Use [RTK](https://github.com/rtk-ai/rtk) to rewrite commands for token
savings. See `/examples/hooks/rtk-rewrite.sh` for the full script. It uses
`updated_input` to swap the command before the bash tool executes it.

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

## Whatcha think?

We’d love to hear your thoughts on this project. Need help? We gotchu. You can
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
