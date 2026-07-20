# Config

> [!NOTE]
> This document was designed for both humans and agents.

Crush is configured with Bash via set a set of crush-specific builtin
commands. By default it lives in `~/.config/crush/crushrc` and works like
a `.bashrc`. It runs when Crush starts and configures the agent.

```bash
# Add Ollama.
provider add ollama --type ollama --base-url "http://localhost:11434/v1"

# Register a model on Ollama.
model add ollama/llama3.3 --name "Llama 3.3" --context-window 128000

# Auto-approve some tools.
permissions allow view edit

# Add an MCP server
mcp add github \
  --type http \
  --url "https://api.githubcopilot.com/mcp/" \
  --header Authorization "Bearer $GITHUB_TOKEN"
```

Since it’s Bash, so you can use logic, `source` other files, and so on. It’s
really handy.

```bash
# Change config based on the machine you're on.
if [[ $HOSTNAME == "babysquid" ]]; then
    option skill-path "$HOME/squid-skills"
fi

# Load some extra config
source "$XDG_CONFIG_HOME/squid-config.sh"

# Get API keys from your password manager.
provider add my-secret-provider \
  --type openai-compat \
  --base-url "https://api.example.com/v1" \
  --api-key "$(op read my-secret-key)"
```

> [!TIP]
>
> Crush can also just configure itself. Just tell it want you want to do.

## Why Bash?

Two reasons:

1. Crush ships with a first-class Bash interpreter, so we get the logic for
   free.
2. Ultimately, Crush needs to be able to configure itself, and command-based
   config allows both users and the agent to use the same tools.

## What about JSON?

JSON is still supported but is deprecated and, while it's supported, it won't
be receiving new features. For more see [Legacy JSON](#legacy-json).

## Config versioning

Not breaking the config API is really important to us! That said, you can
target specific Crush versions with `$CRUSH_VERSION`:

```bash
if [[ $CURSH_VERSION == "0.85.*" ]]; then
    option debug true
fi
```

## Security

Just like `crush.json`, `crushrc` is a trusted file. Guard it carefully and
don't download random configs without reading them first.

## Where config lives

Crush looks for config in the following places, with the lower numbers taking
precedence:

1. `./.crushrc` (project-level)
2. `./crushrc` (project-level)
3. `./.crush.json` / `./crush.json` (project-level JSON; legacy)
4. `$XDG_CONFIG_HOME/crush/crushrc` or `~/.config/crush/crushrc` (global)

Everything found is merged, with project settings overriding global ones, and
`crushrc` overriding `crush.json` in the same directory. If a folder has both,
they merge and Crush logs a warning.

## Command Reference

The sections below read like CLI help. Entity commands use `add` to create or
update something and `remove` (or `rm`) to delete it. Booleans accept
`true/false/1/0/yes/no`, in any case.

```text
Available Commands:
  provider      Manage model providers
  model         Manage models and model selection
  mcp           Manage MCP servers
  lsp           Manage language servers
  hook          Manage hooks
  permissions   Configure tool permissions
  option        Configure general Crush behavior
```

### provider

Manage model providers.

```text
Usage:
  provider [command]

Available Commands:
  add       Add or update a provider
  remove    Remove a provider and its custom models
  rm        Alias for remove
```

#### `provider add`

Add a provider, or update an existing provider with the same ID.

```text
Usage:
  provider add <id> [flags]

Flags:
      --name string                 display name
      --type string                 provider type (openai, openai-compat, anthropic, ollama, …)
      --api-key string              API key
      --base-url string             API base URL
      --disable bool                disable without removing
      --flat-rate bool              use flat-rate billing
      --discover-models bool        auto-discover and merge provider models
      --system-prompt-prefix string text prepended to the system prompt
      --extra-header key value      add an HTTP header (repeatable)
      --extra-body JSON             merge a JSON object into request bodies
      --provider-options JSON       merge a provider-specific JSON object
```

```bash
provider add deepseek \
  --type openai-compat \
  --base-url "https://api.deepseek.com/v1" \
  --api-key "${DEEPSEEK_API_KEY:?set DEEPSEEK_API_KEY}"
```

#### `provider remove`

Remove a provider and all custom models registered on it.

```text
Usage:
  provider remove <id>
  provider rm <id>
```

### model

Manage custom models and the large/small model slots. Model references use the
same `<provider>/<id>` form printed by `crush models`.

```text
Usage:
  model [command]

Available Commands:
  add       Register a custom model on an existing provider
  remove    Remove a custom model
  rm        Alias for remove
  large     Set or print the large model
  small     Set or print the small model
```

#### `model add`

Register a custom model on an existing provider.

```text
Usage:
  model add <provider>/<id> [flags]

Flags:
      --name string                 display name
      --context-window int          context window in tokens
      --default-max-tokens int      default maximum output tokens
      --can-reason bool             model supports reasoning
      --supports-images bool        model accepts image input
      --price-input float           input price per 1M tokens
      --price-output float          output price per 1M tokens
      --price-cache-create float    cache-creation price per 1M tokens
      --price-cache-hit float       cache-hit price per 1M tokens
      --reasoning-effort string     low, medium, or high
```

#### `model remove`

Remove a custom model from its provider.

```text
Usage:
  model remove <provider>/<id>
  model rm <provider>/<id>
```

#### `model large`, `model small`

Set the large or small model slot. With no model argument, print the current
selection.

```text
Usage:
  model large [<provider>/<id>] [flags]
  model small [<provider>/<id>] [flags]

Flags:
      --think                       enable thinking mode
      --reasoning-effort string     low, medium, or high
      --max-tokens int              maximum output tokens
      --temperature float           sampling temperature
      --top-p float                 top-p sampling (0–1)
      --top-k int                   top-k sampling
      --frequency-penalty float     frequency penalty
      --presence-penalty float      presence penalty
      --provider-options JSON       merge a provider-specific JSON object
```

```bash
model large openai/gpt-4o --think
echo "coding with: $(model large)"   # prints: openai/gpt-4o
```

### mcp

Manage Model Context Protocol servers.

```text
Usage:
  mcp [command]

Available Commands:
  add       Add or update an MCP server
  remove    Remove an MCP server
  rm        Alias for remove
```

#### `mcp add`

Add an MCP server, or update an existing server with the same name.

```text
Usage:
  mcp add <name> [flags]

Flags:
      --type string              stdio, sse, or http (default "stdio")
      --command string           executable for stdio servers
      --args string              command argument (repeatable)
      --env key value            environment variable (repeatable)
      --url string               URL for HTTP/SSE servers
      --header key value         HTTP header (repeatable)
      --timeout int              startup timeout in seconds
      --disabled bool            disable without removing
      --disabled-tools string    deny a server tool (repeatable)
      --enabled-tools string     allow only these server tools (repeatable)
```

```bash
mcp add github --type http \
  --url "https://api.githubcopilot.com/mcp/" \
  --header Authorization "Bearer $GH_PAT"
```

#### `mcp remove`

Remove an MCP server.

```text
Usage:
  mcp remove <name>
  mcp rm <name>
```

### lsp

Manage language servers.

```text
Usage:
  lsp [command]

Available Commands:
  add       Add or update a language server
  remove    Remove a language server
  rm        Alias for remove
```

#### `lsp add`

Add a language server, or update an existing server with the same name.

```text
Usage:
  lsp add <name> --command <command> [flags]

Flags:
      --args string              command argument (repeatable)
      --env key value            environment variable (repeatable)
      --filetypes string         file type to attach to (repeatable)
      --root-markers string      root marker file (repeatable)
      --timeout int              startup timeout in seconds
      --disabled bool            disable without removing
      --init-options JSON        initialization options
      --options JSON             server settings
```

```bash
lsp add go --command gopls --env GOPATH "$HOME/go"
```

#### `lsp remove`

Remove a language server.

```text
Usage:
  lsp remove <name>
  lsp rm <name>
```

### hook

Manage hooks. See the [hooks docs](../hooks/) for what they can do and how
they run.

```text
Usage:
  hook [command]

Available Commands:
  add       Add a hook to an event
  remove    Remove a named hook, or clear an event
  rm        Alias for remove
```

#### `hook add`

Add a shell command that runs when the given hook event fires.

```text
Usage:
  hook add <event> --command <command> [flags]

Flags:
      --command string           shell command to run (required)
      --name string              name used for later removal
      --matcher string           regex tested against the tool name
      --timeout int              timeout in seconds (default 30)
```

```bash
hook add PreToolUse --matcher "^bash$" \
  --command "./hooks/no-haskell.sh" --name no-haskell
```

#### `hook remove`

Remove hooks from an event. Without `--name`, remove every hook for the event.

```text
Usage:
  hook remove <event> [--name <name>]
  hook rm <event> [--name <name>]

Flags:
      --name string              remove hooks with this name
```

### permissions

Configure tool permissions. `allow` skips approval prompts; `deny` hides tools
from the agent entirely.

```text
Usage:
  permissions [command]

Available Commands:
  allow     Allow tools without prompting
  deny      Hide tools from the agent
```

#### `permissions allow`

Allow one or more tools to run without prompting.

```text
Usage:
  permissions allow <tool> [<tool> ...]
```

#### `permissions deny`

Hide one or more tools from the agent so they cannot be called.

```text
Usage:
  permissions deny <tool> [<tool> ...]
```

```bash
permissions allow view ls grep edit
permissions deny bash
```

### option

Configure general Crush behavior.

```text
Usage:
  option <key> [value]
  option [command]

Available Commands:
  reset     Clear a list option
  ui        Configure terminal UI behavior

Boolean Keys (value optional; default true):
  debug                       enable debug logging
  debug-lsp                   enable LSP debug logging
  auto-lsp                    automatically configure LSPs
  progress                    show progress indicators
  metrics                     send anonymous metrics
  notifications               enable desktop notifications
  auto-summarize              enable automatic summarization
  provider-auto-update        update the provider catalog automatically
  default-providers           include built-in providers

String Keys:
  data-directory string
  initialize-as string
  notification-style auto|native|osc|bell|disabled
  attribution-trailer-style none|co-authored-by|assisted-by

Other Keys:
  attribution-generated-with bool
  context-path string            append a context path
  global-context-path string     append a global context path
  skill-path string              append a skill path
  disable-skill string           hide a skill
```

#### `option reset`

Clear every value previously added to a list option.

```text
Usage:
  option reset <context-path|global-context-path|skill-path|disable-skill>
```

#### `option ui`

Configure terminal UI behavior.

```text
Usage:
  option ui <key> <value>

Keys:
  compact bool
  diff unified|split
  transparent bool
  scrollbar default|always|never
  completions-max-depth int
  completions-max-items int
```

```bash
option progress false
option skill-path ./skills
option attribution-trailer-style assisted-by
option ui compact true
option ui diff unified
```

> [!IMPORTANT] These skill paths load by default — you do NOT need `skill-path`
> for them: `.agents/skills`, `.crush/skills`, `.claude/skills`,
> `.cursor/skills`.

## Composing configs

Because it's Bash, a shared base config is just a `source`:

```bash
# ~/.config/crush/crushrc
source ~/team/crush-base.sh    # sets up providers, a few skills

# …but on this machine, drop a skill path the base added and add my own.
option reset skill-path
option skill-path ~/my/skills
```

`remove`, `rm`, and `option reset` all act on whatever was set earlier in the
script or pulled in via `source`. Later lines win, just like a shell.

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
headers) are shell-expanded at load time. In `crushrc` there's no such list —
it's all just Bash.

Both formats are trusted code: they run with your shell privileges before the UI
appears. Don't launch Crush in a directory whose config you haven't read.
