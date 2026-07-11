---
name: crush-config
user-invocable: true
description: Use when the user needs help configuring Crush — working with crush.json, setting up providers, configuring LSPs, adding MCP servers, managing skills or permissions, or changing Crush behavior.
---

# Crush Configuration

Crush uses JSON configuration files with the following priority (highest to lowest):

1. `.crush.json` (project-local, hidden)
2. `crush.json` (project-local)
3. `$XDG_CONFIG_HOME/crush/crush.json` or `$HOME/.config/crush/crush.json` (global)

On Windows, settings written by the TUI normally live at
`%LOCALAPPDATA%/crush/crush.json`. An additional global layer may exist at
`%USERPROFILE%/.config/crush/crush.json`. These are not the same as a
project-local `.crush/` directory. Use the `crush_info` tool to report active
and candidate paths before searching manually.

## Safe Editing Workflow

Treat `crush.json` as structured data, even when it is minified onto one line.

1. Use `crush_info` to identify the active file and inspect that exact path.
2. Parse the complete file as JSON before editing. An empty result from a
   line-based offset means the requested line does not exist; it does not mean
   a one-line file is empty.
3. Preserve unknown fields and all unrelated configuration entries.
4. Make a backup, apply a structured object mutation, serialize once, and parse
   the result again before replacing the active file.
5. Re-read through `crush_info` and distinguish configured entries from clients
   that actually initialized successfully.

Never concatenate JSON fragments, rewrite the whole file from a partial
terminal view, use an LSP diagnostic as MCP validation, or accept uncertainty
that the file is empty over direct parsed evidence.

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

- `type` (required): `openai`, `openai-compat`, `anthropic`, or a local provider type (`llamacpp`, `omlx`, `lmstudio`, `litellm`, `ollama`)
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
- Additional fields: `env`, `disabled`, `disabled_tools`, `enabled_tools`, `timeout`, `tool_timeout`.
- `timeout` bounds MCP startup/ping behavior. `tool_timeout` bounds individual MCP tool calls so one stalled server or broad operation cannot pin the agent loop indefinitely.

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

## User-Invocable Skills

Skills can be made invocable as commands from the commands palette. Add `user-invocable: true` to the skill's YAML frontmatter:

```yaml
---
name: my-skill
description: A skill that can be invoked as a command.
user-invocable: true
---
```

User-invocable skills appear in the commands palette with a prefix:
- Skills from global directories: `user:skill-name`
- Skills from project directories: `project:skill-name`

When invoked, the skill's instructions are loaded into the conversation context.

To prevent the model from auto-triggering a skill (while still allowing user invocation), add `disable-model-invocation: true`:

```yaml
---
name: my-skill
description: Only invocable by users, not the model.
user-invocable: true
disable-model-invocation: true
---
```

Skills with `disable-model-invocation` won't appear in the model's available skills list but can still be invoked manually by users.

## Hooks

Hooks are user-defined shell commands that fire on agent events. Use the dedicated `crush-hooks` skill for full payload details and examples.

Supported events:

- `UserPromptSubmit`: prompt admission, rewrite, denial, and transient context before the prompt reaches the model.
- `PreToolUse`: tool policy, permission pre-approval, and tool-input rewriting before a tool runs.
- `PostToolUse`: successful tool-result inspection or context injection.
- `PostToolUseFailure`: failed tool-result inspection or recovery guidance; runs instead of `PostToolUse` for failed results.
- `Stop`: final turn validation before run completion.

```json
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "matcher": "prompt",
        "command": ".crush/hooks/prompt-policy.sh",
        "timeout": 10
      }
    ],
    "PreToolUse": [
      {
        "matcher": "^(bash|tmux)$",
        "command": ".crush/hooks/shell-policy.sh",
        "timeout": 10
      }
    ],
    "PostToolUseFailure": [
      {
        "matcher": "^bash$",
        "command": ".crush/hooks/recover-failed-bash.sh",
        "timeout": 10
      }
    ],
    "Stop": [
      {
        "matcher": "stop",
        "command": ".crush/hooks/final-check.sh",
        "timeout": 10
      }
    ]
  }
}
```

### Hook Properties

- `command` (required): Shell command to execute. Runs via `sh -c`.
- `matcher` (optional): Regex pattern tested against the tool name or lifecycle target. Empty or absent means match all hooks for that event.
- `timeout` (optional): Timeout in seconds. Defaults to 30.

Event names are case-insensitive and accept snake_case variants: `PreToolUse`, `pretooluse`, `pre_tool_use`, and `PRE_TOOL_USE` all normalize to the same event.

### Hook Output

Exit code `0` parses stdout as JSON. Exit code `2` denies or blocks the current event. Exit code `49` halts the whole turn. Other exit codes are non-blocking hook failures.

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
- `reason`: explanation shown when denying or halting.
- `context`: extra model-visible context. Prompt-hook context is transient and is not persisted as user history.
- `updated_input`: shallow merge patch against `tool_input`; only meaningful for `PreToolUse`.
- `updated_prompt`: full replacement for the admitted prompt sent to the model and saved in history; only meaningful for `UserPromptSubmit`.

Claude Code-style `hookSpecificOutput` is also accepted for permission decisions, `updatedInput`, and `additionalContext`.

## Tool Permissions

`allowed_tools` remains the legacy auto-allow fallback. Prefer `rules` when resource-specific policy matters.

```json
{
  "permissions": {
    "allowed_tools": ["view", "ls", "grep"],
    "rules": [
      {"tool": "web_fetch", "action": "fetch", "resource": "https://docs.example.com/*", "effect": "allow"},
      {"tool": "bash", "action": "execute", "resource": "rm *", "effect": "deny"},
      {"tool": "tmux", "action": "start", "resource": "*", "effect": "ask"}
    ]
  }
}
```

Rule fields:

- `tool`: tool name such as `bash`, `tmux`, `web_search`, `web_fetch`, `edit`, or `write`.
- `action`: tool action such as `execute`, `start`, `send`, `fetch`, `search`, `read`, or `write`.
- `resource`: command, query, URL, or path. `*` wildcards are supported.
- `effect`: `allow`, `ask`, or `deny`.

Evaluation order:

1. Yolo/skip mode grants everything.
2. Explicit deny rules block before prompts, saved grants, hooks, or `allowed_tools`.
3. Explicit allow rules grant.
4. Explicit ask rules force a prompt.
5. `allowed_tools` grants backward-compatible auto-allow.
6. Hook allow can grant only when no explicit deny matched.

## Environment Variables

- `CRUSH_GLOBAL_CONFIG` - Override global config location
- `CRUSH_GLOBAL_DATA` - Override data directory location
- `CRUSH_SKILLS_DIR` - Override default skills directory
