---
name: crush-config
description: Use when the user needs help configuring Crush — working with crush.json, setting up providers, configuring LSPs, adding MCP servers, managing skills or permissions, or changing Crush behavior.
---

# Crush Configuration

Crush uses JSON configuration files with the following priority (highest to lowest):

1. `.crush.json` (project-local, hidden)
2. `crush.json` (project-local)
3. `$XDG_CONFIG_HOME/crush/crush.json` or `$HOME/.config/crush/crush.json` (global)

## Basic Structure

```json
{
  "$schema": "https://charm.land/crush.json",
  "models": {},
  "providers": {},
  "mcp": {},
  "lsp": {},
  "hooks": {},
  "options": {},
  "permissions": {},
  "tools": {}
}
```

The `$schema` property enables IDE autocomplete but is optional.

## Shell Expansion

Crush runs selected string fields through an embedded bash-compatible
shell at load time, so values can pull from env vars, files, or helper
commands.

Supported constructs (match the `bash` tool):

- `$VAR` and `${VAR}`
- `${VAR:-default}`, `${VAR:+alt}`, `${VAR:?message}`
- `$(command)` with full quoting and nesting
- Single- and double-quoted strings, escapes

Default semantics match bash: an unset variable expands to an empty
string, no error. A failing `$(command)` is always a hard error. For
required credentials, use `${VAR:?message}` so a missing variable
fails loudly at load time with your message.

```json
{ "api_key": "${CODEBERG_TOKEN:?set CODEBERG_TOKEN}" }
```

### Which fields expand

| Surface                                             | Expansion |
| --------------------------------------------------- | --------- |
| Provider `api_key`, `base_url`, `api_endpoint`      | yes       |
| Provider `extra_headers`                            | yes       |
| Provider `extra_body`                               | **no**    |
| MCP `command`, `args`, `env`, `headers`, `url`      | yes       |
| LSP `command`, `args`, `env`                        | yes       |
| Hook `command`                                      | runs via `sh -c`, not the resolver |

`extra_body` is a JSON passthrough. If you need env-driven values in
a request body, put them in `extra_headers`, `api_key`, or
`base_url` instead.

### Empty-resolved headers are dropped

When a header value resolves to the empty string (unset variable,
`$(echo)`, or literal `""`), the header is omitted from the
outgoing request. This keeps optional env-gated headers like
`"OpenAI-Organization": "$OPENAI_ORG_ID"` working cleanly when the
var isn't set. Applies to MCP `headers` and provider `extra_headers`.

### Security note

`crush.json` is trusted code. Any `$(...)` in it runs at load time
with the invoking user's shell privileges, before the UI appears.
Don't launch Crush in a directory whose `crush.json` you haven't
reviewed.

## Common Tasks

- Add a custom provider: add an entry under `providers` with `type`, `base_url`, `api_key`, and `models`.
- Disable a builtin or local skill: add the skill name to `options.disabled_skills`.
- Add an MCP server: add an entry under `mcp` with `type` and either `command` (stdio) or `url` (http/sse).

## Model Selection

```json
{
  "models": {
    "large": {
      "model": "claude-sonnet-4-20250514",
      "provider": "anthropic",
      "max_tokens": 16384
    },
    "small": {
      "model": "claude-haiku-4-20250514",
      "provider": "anthropic"
    }
  }
}
```

- `large` is the primary coding model; `small` is for summarization.
- Only `model` and `provider` are required.
- Optional tuning: `reasoning_effort`, `think`, `max_tokens`, `temperature`, `top_p`, `top_k`, `frequency_penalty`, `presence_penalty`, `provider_options`.

## Custom Providers

```json
{
  "providers": {
    "deepseek": {
      "type": "openai-compat",
      "base_url": "https://api.deepseek.com/v1",
      "api_key": "$DEEPSEEK_API_KEY",
      "models": [
        {
          "id": "deepseek-chat",
          "name": "Deepseek V3",
          "context_window": 64000
        }
      ]
    }
  }
}
```

- `type` (required): `openai`, `openai-compat`, or `anthropic`
- `api_key`, `base_url`, `api_endpoint`, and `extra_headers` are shell-expanded (see [Shell Expansion](#shell-expansion)).
- `extra_body` is a JSON passthrough and is **not** expanded.
- Additional fields: `disable`, `system_prompt_prefix`, `extra_headers`, `extra_body`, `provider_options`.

## LSP Configuration

```json
{
  "lsp": {
    "go": {
      "command": "gopls",
      "env": { "GOPATH": "$HOME/go" }
    },
    "typescript": {
      "command": "typescript-language-server",
      "args": ["--stdio"]
    }
  }
}
```

- `command` (required), `args`, `env` cover most setups.
- `command`, `args`, and `env` values are shell-expanded (see [Shell Expansion](#shell-expansion)).
- Additional fields: `disabled`, `filetypes`, `root_markers`, `init_options`, `options`, `timeout`.

## MCP Servers

```json
{
  "mcp": {
    "filesystem": {
      "type": "stdio",
      "command": "node",
      "args": ["/path/to/mcp-server.js"]
    },
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer $GH_PAT"
      }
    }
  }
}
```

- `type` (required): `stdio`, `sse`, or `http`
- `command`, `args`, `env`, `headers`, and `url` are shell-expanded (see [Shell Expansion](#shell-expansion)).
- Additional fields: `env`, `disabled`, `disabled_tools`, `timeout`.

## Options

```json
{
  "options": {
    "skills_paths": ["./skills"],
    "disabled_tools": ["bash", "sourcegraph"],
    "disabled_skills": ["crush-config"],
    "tui": {
      "compact_mode": false,
      "diff_mode": "unified",
      "transparent": false
    },
    "auto_lsp": true,
    "debug": false,
    "debug_lsp": false,
    "attribution": {
      "trailer_style": "assisted-by",
      "generated_with": true
    }
  }
}
```

> [!IMPORTANT]
> The following skill paths are loaded by default and DO NOT NEED to be added to `skills_paths`:
> `.agents/skills`, `.crush/skills`, `.claude/skills`, `.cursor/skills`

Other options: `context_paths`, `progress`, `disable_notifications`, `disable_auto_summarize`, `disable_metrics`, `disable_provider_auto_update`, `disable_default_providers`, `data_directory`, `initialize_as`.

## Hooks

Hooks are user-defined shell commands that fire on agent events. Currently only `PreToolUse` is supported, which runs before a tool is executed.

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "^(edit|write|multiedit)$",
        "command": ".crush/hooks/protect-files.sh"
      },
      {
        "matcher": "^bash$",
        "command": ".crush/hooks/no-haskell.sh"
      }
    ]
  }
}
```

### Hook Properties

- `command` (required): Shell command to execute. Runs via `sh -c`.
- `matcher` (optional): Regex pattern tested against the tool name. Empty or absent means match all tools.
- `timeout` (optional): Timeout in seconds. Defaults to 30.

### Event Name Normalization

Event names are case-insensitive and accept snake_case variants: `PreToolUse`, `pretooluse`, `pre_tool_use`, and `PRE_TOOL_USE` all work.

### How Hooks Work

1. When a tool is about to be called, all `PreToolUse` hooks with a matching `matcher` (or no matcher) run in parallel.
2. Duplicate commands are deduplicated — each unique command runs at most once.
3. The hook receives JSON on **stdin** and hook-specific **environment variables**.

### Hook Input (stdin)

A JSON payload is piped to the hook command:

```json
{
  "event": "PreToolUse",
  "session_id": "abc-123",
  "cwd": "/path/to/project",
  "tool_name": "bash",
  "tool_input": {"command": "ls -la"}
}
```

### Hook Environment Variables

| Variable | Description |
|---|---|
| `CRUSH_EVENT` | Event name (e.g. `PreToolUse`) |
| `CRUSH_TOOL_NAME` | Name of the tool being called |
| `CRUSH_SESSION_ID` | Current session ID |
| `CRUSH_CWD` | Current working directory |
| `CRUSH_PROJECT_DIR` | Project root directory |
| `CRUSH_TOOL_INPUT_COMMAND` | Value of `command` from tool input (if present) |
| `CRUSH_TOOL_INPUT_FILE_PATH` | Value of `file_path` from tool input (if present) |

### Hook Output

**Exit code 0** — the hook succeeded. Stdout is parsed as JSON:

```json
{"decision": "allow", "context": "optional context appended to tool result"}
```

- `decision`: `allow` to explicitly allow, `deny` to block, `none` (or omit) for no opinion.
- `reason`: Explanation text (used when denying).
- `context`: Extra context appended to the tool result.
- `updated_input`: Replacement JSON for the tool input. Last non-empty value wins.

**Exit code 2** — the tool call is blocked. Stderr is used as the deny reason.

```bash
echo "No Haskell allowed" >&2
exit 2
```

**Any other exit code** — non-blocking error. The tool call proceeds as normal.

### Claude Code Compatibility

Crush also supports the Claude Code hook output format:

```json
{
  "hookSpecificOutput": {
    "permissionDecision": "allow",
    "permissionDecisionReason": "Auto-approved",
    "updatedInput": {"command": "echo rewritten"}
  }
}
```

Existing Claude Code hooks should work without modification.

### Decision Aggregation

When multiple hooks match, their decisions are aggregated:

- **Deny wins over allow** — if any hook denies, the tool call is blocked.
- **Allow wins over none** — if no hook denies but at least one allows, the call proceeds.
- All deny reasons are concatenated (newline-separated).
- All context strings are concatenated (newline-separated).
- For `updated_input`, the last non-empty value wins.

## Tool Permissions

```json
{
  "permissions": {
    "allowed_tools": ["view", "ls", "grep", "edit"]
  }
}
```

## Environment Variables

- `CRUSH_GLOBAL_CONFIG` - Override global config location
- `CRUSH_GLOBAL_DATA` - Override data directory location
- `CRUSH_SKILLS_DIR` - Override default skills directory
