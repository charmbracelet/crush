---
name: crush-config
description: Use when the user needs help configuring Crush — writing crushrc (the Bash config format) or crush.json, setting up providers, models, LSPs, MCP servers, hooks, skills, permissions, or changing Crush behavior.
---

# Crush Configuration

Crush supports two config formats:

- **`crushrc`** — a Bash script that builds config by calling Crush builtins.
  **Preferred.** Because it is real Bash you get includes, secrets,
  conditionals, and variables for free.
- **`crush.json`** — static JSON. Fully supported; see
  [Legacy JSON format](#legacy-json-format).

Both are discovered together and deep-merged. Priority (highest to lowest):

1. `.crushrc` / `crushrc` / `.crush.json` / `crush.json` (project-local,
   closer-to-cwd wins)
2. `$XDG_CONFIG_HOME/crush/` or `$HOME/.config/crush/` (global)

If a directory has both `crushrc` and `crush.json`, they merge (`crushrc` wins
on conflicts) and Crush logs a warning.

## crushrc at a glance

A `crushrc` is a plain Bash script executed at load time with the same embedded
shell the `bash` tool uses. It builds config by calling builtins (`provider`,
`model`, `mcp`, `lsp`, `hook`, `permissions`, `option`). Statements run top to
bottom; later statements win, and `remove`/`reset` operate on anything defined
earlier or pulled in via `source`.

```bash
#!/usr/bin/env bash
# Includes and secrets are just Bash.
source ~/.config/crush/shared.sh

provider add anthropic --api-key "$ANTHROPIC_API_KEY"

model large anthropic/claude-sonnet-4-20250514 --max-tokens 16384
model small anthropic/claude-haiku-4-20250514

option skill-path ./skills
permissions allow view ls grep edit
```

Values are ordinary Bash — quote and expand normally (`"$VAR"`, `$(cmd)`,
`${VAR:?required}`). A failing `$(command)` aborts the load.

`CRUSH_VERSION` is exported into the script so you can feature-detect the
running Crush (it is the literal `devel` for local builds):

```bash
[[ "$CRUSH_VERSION" != devel ]] && lsp add gopls --command gopls
```

## Commands

All entity commands are verb-first. `remove` accepts `rm` as an alias. Booleans
accept `true/false/1/0/yes/no`, case-insensitive.

### providers

```bash
provider add <id> [flags]    # define/update; repeated calls merge
provider remove <id>         # alias: rm — removes the provider and its models
```

Flags: `--name`, `--type` (`openai`, `openai-compat`, `anthropic`, or a local
type like `ollama`, `lmstudio`, `llamacpp`), `--api-key`, `--base-url`,
`--disable BOOL`, `--flat-rate BOOL`, `--system-prompt-prefix TEXT`,
`--extra-header KEY VALUE` (repeatable).

```bash
provider add deepseek \
  --type openai-compat \
  --base-url "https://api.deepseek.com/v1" \
  --api-key "${DEEPSEEK_API_KEY:?set DEEPSEEK_API_KEY}"
```

### models

```bash
model add <provider>/<id> [flags]      # register a custom model (provider must exist)
model remove <provider>/<id>           # alias: rm
model large [<provider>/<id>] [flags]  # set the large slot; no arg prints it
model small [<provider>/<id>] [flags]  # set the small slot; no arg prints it
```

- `<provider>/<id>` is the same form `crush models` prints. A missing slash is
  an error. `model add` requires the provider to already exist.
- `model add` flags: `--name`, `--context-window N`, `--default-max-tokens N`,
  `--can-reason BOOL`, `--supports-images BOOL`, `--cost-per-1m-in F`,
  `--cost-per-1m-out F`, `--reasoning-effort low|medium|high`.
- `model large`/`model small` flags: `--think`, `--reasoning-effort`,
  `--max-tokens N`, `--temperature F`.
- `model large` with no argument prints the current selection as `provider/id`,
  usable in `$(model large)`.

`large` is the primary coding model; `small` is used for summarization.

### mcp

```bash
mcp add <name> --type stdio|sse|http [flags]   # default type is stdio
mcp remove <name>                              # alias: rm
```

Flags: `--command CMD`, `--args ARG` (repeatable), `--env KEY VALUE`
(repeatable), `--url URL`, `--header KEY VALUE` (repeatable), `--timeout N`,
`--disabled BOOL`, `--disabled-tools TOOL` (repeatable), `--enabled-tools TOOL`
(repeatable).

```bash
mcp add github --type http \
  --url "https://api.githubcopilot.com/mcp/" \
  --header Authorization "Bearer $GH_PAT"

mcp add filesystem --command node --args /path/to/mcp-server.js
```

### lsp

```bash
lsp add <name> --command CMD [flags]
lsp remove <name>                     # alias: rm
```

Flags: `--args ARG` (repeatable), `--env KEY VALUE` (repeatable),
`--filetypes TYPE` (repeatable), `--root-markers MARKER` (repeatable),
`--timeout N`, `--disabled BOOL`, `--init-options JSON`, `--options JSON`.

```bash
lsp add go --command gopls --env GOPATH "$HOME/go"
lsp add typescript --command typescript-language-server --args --stdio
```

### hooks

```bash
hook add <event> --command CMD [--name NAME] [--matcher REGEX] [--timeout N]
hook remove <event> [--name NAME]    # alias: rm; without --name clears the event
```

Only named hooks can be removed individually — give a hook `--name` if you
intend to remove it later. See [Hooks runtime](#hooks-runtime) for how hooks
execute (stdin payload, env vars, decisions).

```bash
hook add PreToolUse --matcher "^bash$" --command ".crush/hooks/no-haskell.sh" --name no-haskell
```

### permissions

```bash
permissions allow <tool> [<tool> ...]   # tools that skip permission prompts
```

### options

```bash
option <key> [value]
option reset <list-key>    # clear a list option back to empty
```

- **Boolean keys** (value optional, defaults `true`): `debug`, `debug-lsp`,
  `auto-lsp`, `progress`.
- **Boolean keys phrased positively** (stored as the negated field): `metrics`,
  `notifications`, `auto-summarize`, `provider-auto-update`,
  `default-providers`. Example: `option metrics false` disables metrics.
- **String keys**: `data-directory`, `initialize-as`, `notification-style`.
- **List keys** (singular, one value per call, repeatable): `context-path`,
  `global-context-path`, `skill-path`, `disable-tool`, `disable-skill`. Use
  `option reset <key>` to wipe inherited values (e.g. after `source`).

```bash
option progress false
option skill-path ./skills
option disable-tool bash
option disable-skill crush-config
```

> [!IMPORTANT] These skill paths are loaded by default and do NOT need
> `skill-path`: `.agents/skills`, `.crush/skills`, `.claude/skills`,
> `.cursor/skills`.

### Fields not yet expressible in crushrc

A few advanced fields have no builtin yet. Put them in a `crush.json` alongside
your `crushrc` (they merge):

- Nested `options.tui` (`compact_mode`, `diff_mode`, `transparent`) and
  `options.attribution`.
- Extended model tuning: `top_p`, `top_k`, `frequency_penalty`,
  `presence_penalty`, `provider_options`.
- Provider `extra_body`, `provider_options`, `api_endpoint`, `discover_models`.

## Hooks runtime

Hooks are user-defined shell commands that fire on agent events. Currently only
`PreToolUse` is supported, which runs before a tool executes. This behavior is
the same however the hook is defined (`hook add` or JSON).

### How hooks work

1. When a tool is about to be called, all `PreToolUse` hooks with a matching
   `matcher` (or no matcher) run in parallel.
2. Duplicate commands are deduplicated — each unique command runs at most once.
3. The hook receives JSON on **stdin** and hook-specific **environment
   variables**.

Event names are case-insensitive and accept snake_case: `PreToolUse`,
`pretooluse`, `pre_tool_use`, `PRE_TOOL_USE` all work.

### Hook input (stdin)

```json
{
  "event": "PreToolUse",
  "session_id": "abc-123",
  "cwd": "/path/to/project",
  "tool_name": "bash",
  "tool_input": { "command": "ls -la" }
}
```

### Hook environment variables

| Variable                     | Description                                       |
| ---------------------------- | ------------------------------------------------- |
| `CRUSH_EVENT`                | Event name (e.g. `PreToolUse`)                    |
| `CRUSH_TOOL_NAME`            | Name of the tool being called                     |
| `CRUSH_SESSION_ID`           | Current session ID                                |
| `CRUSH_CWD`                  | Current working directory                         |
| `CRUSH_PROJECT_DIR`          | Project root directory                            |
| `CRUSH_TOOL_INPUT_COMMAND`   | Value of `command` from tool input (if present)   |
| `CRUSH_TOOL_INPUT_FILE_PATH` | Value of `file_path` from tool input (if present) |

### Hook output

**Exit code 0** — hook succeeded. Stdout is parsed as JSON:

```json
{ "decision": "allow", "context": "optional context appended to tool result" }
```

- `decision`: `allow` to explicitly allow, `deny` to block, `none` (or omit).
- `reason`: explanation (used when denying).
- `context`: extra context appended to the tool result.
- `updated_input`: replacement JSON for the tool input; last non-empty wins.

**Exit code 2** — the tool call is blocked; stderr is the deny reason.

**Any other exit code** — non-blocking error; the tool call proceeds.

### Decision aggregation

- **Deny wins over allow** — any deny blocks the call.
- **Allow wins over none** — a lone allow lets it proceed.
- Deny reasons and context strings are concatenated (newline-separated).
- For `updated_input`, the last non-empty value wins.

### Claude Code compatibility

Crush also accepts the Claude Code hook output format, so existing hooks work
unchanged:

```json
{
  "hookSpecificOutput": {
    "permissionDecision": "allow",
    "permissionDecisionReason": "Auto-approved",
    "updatedInput": { "command": "echo rewritten" }
  }
}
```

## User-invocable skills

Skills can be invoked as commands. Add `user-invocable: true` to the skill's
YAML frontmatter:

```yaml
---
name: my-skill
description: A skill that can be invoked as a command.
user-invocable: true
---
```

- Global skills appear as `user:skill-name`; project skills as
  `project:skill-name`.
- Add `disable-model-invocation: true` to keep a skill user-only (hidden from
  the model's available-skills list but still manually invocable).

## Environment variables

- `CRUSH_VERSION` — exported into `crushrc` at load; the running version (or
  `devel` for local builds).
- `CRUSH_GLOBAL_CONFIG` — override global config location.
- `CRUSH_GLOBAL_DATA` — override data directory location.
- `CRUSH_SKILLS_DIR` — override default skills directory.

## Legacy JSON format

`crush.json` is the original static format. It still works and merges with
`crushrc`. Basic structure:

```json
{
  "$schema": "https://charm.land/crush.json",
  "models": {},
  "providers": {},
  "mcp": {},
  "lsp": {},
  "hooks": {},
  "options": {},
  "permissions": {}
}
```

The `$schema` property enables IDE autocomplete but is optional.

### crushrc ↔ crush.json mapping

| crushrc                             | crush.json                                             |
| ------------------------------------ | ------------------------------------------------------ |
| `provider add openai --api-key "$K"` | `providers.openai = {"api_key": "$K"}`                 |
| `model add openai/gpt-x --name X`    | append to `providers.openai.models[]`                  |
| `model large openai/gpt-x`           | `models.large = {"provider":"openai","model":"gpt-x"}` |
| `mcp add gh --type http --url U`     | `mcp.gh = {"type":"http","url":"U"}`                   |
| `lsp add go --command gopls`         | `lsp.go = {"command":"gopls"}`                         |
| `hook add PreToolUse --command C`    | append to `hooks.PreToolUse[]`                         |
| `permissions allow view ls`          | `permissions.allowed_tools = ["view","ls"]`            |
| `option skill-path ./skills`         | `options.skills_paths = ["./skills"]`                  |
| `option metrics false`               | `options.disable_metrics = true`                       |

### Shell expansion in crush.json

In JSON, only selected string fields are run through the embedded shell at load
time (in `crushrc`, everything is native Bash so this table does not apply):

| Surface                                                         | Expansion                          |
| --------------------------------------------------------------- | ---------------------------------- |
| Provider `api_key`, `base_url`, `api_endpoint`, `extra_headers` | yes                                |
| Provider `extra_body`                                           | **no** (JSON passthrough)          |
| MCP `command`, `args`, `env`, `headers`, `url`                  | yes                                |
| LSP `command`, `args`, `env`                                    | yes                                |
| Hook `command`                                                  | runs via `sh -c`, not the resolver |

Supported constructs: `$VAR`, `${VAR}`, `${VAR:-default}`, `${VAR:+alt}`,
`${VAR:?message}`, `$(command)`. An unset variable expands to empty; a failing
`$(command)` is a hard error. A header that resolves to empty is dropped from
the request.

### Security note

Both formats are trusted code. `crushrc` runs entirely, and any `$(...)` in
`crush.json` runs at load time, with the invoking user's shell privileges,
before the UI appears. Don't launch Crush in a directory whose config you
haven't reviewed.
