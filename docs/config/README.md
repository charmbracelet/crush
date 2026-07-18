# Config

> [!NOTE] This document was designed for both humans and agents.

Crush is configured with a little Bash script called `crush.sh`. Think of it as
a `.bashrc` for your agent: it runs when Crush starts, and you set things up by
calling a handful of small commands.

Because it's real Bash, you get all the good stuff for free — `source` other
files, pull secrets from `$(op read …)`, branch on `$HOSTNAME`, define variables
once and reuse them. No new syntax to learn.

Prefer JSON? `crush.json` still works and happily coexists with `crush.sh`. See
[Legacy JSON](#legacy-json) at the bottom.

### Hot Config Facts

- Config is just a Bash script — it runs top to bottom when Crush starts.
- Every setting is a small verb-first command: `provider add`, `model large`,
  `mcp add`, `option`, and friends.
- Entities you can add you can also remove (`remove`, alias `rm`).
- It's real Bash, so `source`, `$VARS`, `$(commands)`, and `if` all work.
- `crush.json` still works and merges with `crush.sh`.
- `$CRUSH_VERSION` is available inside the script for feature detection.

## Baby's First Config

Let's set a provider and pick our models. Drop this in
`~/.config/crush/crush.sh` (global) or `./crush.sh` (project):

```bash
#!/usr/bin/env bash

# Add a provider. The API key is pulled from your environment at load time.
provider add anthropic --api-key "$ANTHROPIC_API_KEY"

# Pick the big model for coding and a small one for summaries.
model large anthropic/claude-sonnet-4-20250514 --max-tokens 16384
model small anthropic/claude-haiku-4-20250514

# Auto-approve a few safe tools so Crush stops asking.
permissions allow view ls grep
```

That's it. Start Crush and you're configured. Read on for the full command list.

## Where config lives

Crush looks for these, closest-to-you wins:

1. `./.crush.sh` / `./crush.sh` / `./.crush.json` / `./crush.json` (project)
2. `$XDG_CONFIG_HOME/crush/` or `~/.config/crush/` (global)

Everything found is merged, with project settings overriding global ones. If a
folder has both a `.sh` and a `.json`, they merge too (`.sh` wins on conflicts)
and Crush logs a friendly warning.

## Commands

All the entity commands read the same way: `<thing> add …` to create,
`<thing> remove …` (or `rm`) to delete. Booleans accept `true/false/1/0/yes/no`,
any case.

### providers

```text
provider add <id> [flags]        Define or update a provider (repeat to merge)
provider remove <id>             Remove it (and its custom models); alias: rm

  --name NAME                    Display name
  --type TYPE                    openai | openai-compat | anthropic | ollama | …
  --api-key KEY                  API key (quote it; expand env with "$VAR")
  --base-url URL                 API base URL
  --disable BOOL                 Turn the provider off without deleting it
  --flat-rate BOOL               Flat-rate billing
  --system-prompt-prefix TEXT    Text prepended to the system prompt
  --extra-header KEY VALUE       Add an HTTP header (repeatable)
```

```bash
provider add deepseek \
  --type openai-compat \
  --base-url "https://api.deepseek.com/v1" \
  --api-key "${DEEPSEEK_API_KEY:?set DEEPSEEK_API_KEY}"
```

### models

```text
model add <provider>/<id> [flags]    Register a custom model on a provider
model remove <provider>/<id>         Remove it; alias: rm
model large [<provider>/<id>]        Set the large slot — or print it if no arg
model small [<provider>/<id>]        Set the small slot — or print it if no arg

  # model add
  --name NAME                   Display name
  --context-window N            Context window in tokens
  --default-max-tokens N        Default max output tokens
  --can-reason BOOL             Model supports reasoning
  --supports-images BOOL        Model accepts image input
  --cost-per-1m-in F            Input cost per 1M tokens
  --cost-per-1m-out F           Output cost per 1M tokens
  --reasoning-effort LEVEL      low | medium | high

  # model large / model small
  --think                       Enable thinking mode
  --reasoning-effort LEVEL      low | medium | high
  --max-tokens N                Max output tokens
  --temperature F               Sampling temperature
```

The `<provider>/<id>` form is exactly what `crush models` prints. `large` is
your primary coding model; `small` is used for summaries. `model add` needs the
provider to exist first.

```bash
model large openai/gpt-4o --think
echo "coding with: $(model large)"   # prints: openai/gpt-4o
```

### mcp

```text
mcp add <name> --type stdio|sse|http [flags]   Add an MCP server (default: stdio)
mcp remove <name>                              Remove it; alias: rm

  --command CMD                 Executable for stdio servers
  --args ARG                    Command argument (repeatable)
  --env KEY VALUE               Environment variable (repeatable)
  --url URL                     URL for sse/http servers
  --header KEY VALUE            HTTP header (repeatable)
  --timeout N                   Startup timeout in seconds
  --disabled BOOL               Turn it off without deleting
  --disabled-tools TOOL         Deny a tool from this server (repeatable)
  --enabled-tools TOOL          Allow only these tools (repeatable)
```

```bash
mcp add github --type http \
  --url "https://api.githubcopilot.com/mcp/" \
  --header Authorization "Bearer $GH_PAT"
```

### lsp

```text
lsp add <name> --command CMD [flags]    Add a language server
lsp remove <name>                       Remove it; alias: rm

  --args ARG                    Command argument (repeatable)
  --env KEY VALUE               Environment variable (repeatable)
  --filetypes TYPE              File type to attach to (repeatable)
  --root-markers MARKER         Root marker file (repeatable)
  --timeout N                   Startup timeout in seconds
  --disabled BOOL               Turn it off without deleting
  --init-options JSON           Initialization options (JSON string)
  --options JSON                Server options (JSON string)
```

```bash
lsp add go --command gopls --env GOPATH "$HOME/go"
```

### hooks

```text
hook add <event> --command CMD [flags]   Add a hook to an event
hook remove <event> [--name NAME]        Remove a named hook, or clear the event

  --command CMD                 Shell command to run (required)
  --name NAME                   Name it (needed if you want to remove it later)
  --matcher REGEX               Regex tested against the tool name
  --timeout N                   Timeout in seconds (default 30)
```

Hooks are a whole topic of their own — see the [hooks docs](../hooks/) for what
they can do and how they run.

```bash
hook add PreToolUse --matcher "^bash$" \
  --command "./hooks/no-haskell.sh" --name no-haskell
```

### permissions

```text
permissions allow <tool> [<tool> …]   Let these tools skip the approval prompt
```

```bash
permissions allow view ls grep edit
```

### options

```text
option <key> [value]         Set a single option
option reset <list-key>      Clear a list option back to empty

  Booleans (value optional, defaults true):
    debug, debug-lsp, auto-lsp, progress

  Booleans phrased positively (e.g. "option metrics false" turns metrics off):
    metrics, notifications, auto-summarize, provider-auto-update,
    default-providers

  Strings:
    data-directory, initialize-as, notification-style

  Lists (one value per call, repeatable; clear with "option reset <key>"):
    context-path, global-context-path, skill-path, disable-tool, disable-skill
```

```bash
option progress false
option skill-path ./skills
option disable-tool bash
```

> [!IMPORTANT] These skill paths load by default — you do NOT need `skill-path`
> for them: `.agents/skills`, `.crush/skills`, `.claude/skills`,
> `.cursor/skills`.

## Composing configs

Because it's Bash, a shared base config is just a `source`:

```bash
# ~/.config/crush/crush.sh
source ~/team/crush-base.sh    # sets up providers, a few skills

# …but on this machine, drop a skill path the base added and add my own.
option reset skill-path
option skill-path ~/my/skills
```

`remove`, `rm`, and `option reset` all act on whatever was set earlier in the
script or pulled in via `source`. Later lines win, just like a shell.

## A few things still live in JSON

Most config has a command, but a handful of advanced knobs don't yet. Put them
in a `crush.json` next to your `crush.sh` (they merge):

- Nested `options.tui` (compact mode, diff mode, transparency) and
  `options.attribution`.
- Extra model tuning: `top_p`, `top_k`, `frequency_penalty`, `presence_penalty`,
  `provider_options`.
- Provider `extra_body`, `provider_options`, `api_endpoint`, `discover_models`.

## Legacy JSON

`crush.json` is the original format and isn't going anywhere yet. Same concepts,
just static:

```jsonc
{
  "$schema": "https://charm.land/crush.json",
  "providers": {
    "anthropic": { "api_key": "$ANTHROPIC_API_KEY" },
  },
  "models": {
    "large": { "provider": "anthropic", "model": "claude-sonnet-4-20250514" },
  },
  "permissions": { "allowed_tools": ["view", "ls", "grep"] },
}
```

In JSON, only selected string fields (API keys, URLs, MCP/LSP commands and args,
headers) are shell-expanded at load time. In `crush.sh` there's no such list —
it's all just Bash.

Both formats are trusted code: they run with your shell privileges before the UI
appears. Don't launch Crush in a directory whose config you haven't read.
