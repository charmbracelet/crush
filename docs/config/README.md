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
  --discover-models BOOL         Auto-discover and merge provider models
  --system-prompt-prefix TEXT    Text prepended to the system prompt
  --extra-header KEY VALUE       Add an HTTP header (repeatable)
  --extra-body JSON              Merge a JSON object into request bodies
  --provider-options JSON        Merge a provider-specific JSON object
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
  --price-input F               Input price per 1M tokens
  --price-output F              Output price per 1M tokens
  --price-cache-create F        Cache-creation price per 1M tokens
  --price-cache-hit F           Cache-hit price per 1M tokens
  --reasoning-effort LEVEL      low | medium | high

  # model large / model small
  --think                       Enable thinking mode
  --reasoning-effort LEVEL      low | medium | high
  --max-tokens N                Max output tokens
  --temperature F               Sampling temperature
  --top-p F                     Top-p sampling (0–1)
  --top-k N                     Top-k sampling
  --frequency-penalty F         Frequency penalty
  --presence-penalty F          Presence penalty
  --provider-options JSON       Merge a provider-specific JSON object
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
permissions deny <tool> [<tool> …]    Hide these tools from the agent entirely
```

`deny` is the inverse of `allow` — it writes `options.disabled_tools`, so a
denied tool is hidden, not just prompted for.

```bash
permissions allow view ls grep edit
permissions deny bash
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

  Attribution:
    attribution-trailer-style     none | co-authored-by | assisted-by
    attribution-generated-with    true | false (case-insensitive)

  UI:
    option ui compact BOOL
    option ui diff unified|split
    option ui transparent BOOL
    option ui scrollbar default|always|never
    option ui completions-max-depth N
    option ui completions-max-items N

  Lists (one value per call, repeatable; clear with "option reset <key>"):
    context-path, global-context-path, skill-path, disable-skill
```

```bash
option progress false
option skill-path ./skills
option disable-skill crush-config
option attribution-trailer-style assisted-by
option attribution-generated-with true
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
