# Config

> [!NOTE]
> This document was designed for both humans and agents.

> [!TIP]
>
> Crush can configure itself via a builtin config skill. That is to say,
> can generally just tell Crush want you want to configure using natural
> language.
>
> If you're migrating from the old JSON format, you can also ask Crush to
> convert the config for you.

Crush is configured with Bash via a set of Crush-specific builtin commands. By
default, global config lives at `~/.config/crush/crushrc` on Unix-like systems
and `%USERPROFILE%\.config\crush\crushrc` on Windows. It works like a `.bashrc`:
it runs when Crush starts and configures the agent.

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
if [[ $CRUSH_VERSION == "0.85.*" ]]; then
    option debug true
fi
```

## Security

Just like `crush.json`, `crushrc` is a trusted file. Guard it carefully and
don't download random configs without reading them first.

## Where config lives

Crush looks for config in the following places, with lower numbers taking
precedence:

| Priority | Unix-like                        | Windows                           |
| -------- | -------------------------------- | --------------------------------- |
| 1        | `./.crushrc`                     | `.\.crushrc`                      |
| 2        | `./crushrc`                      | `.\crushrc`                       |
| 3        | `$XDG_CONFIG_HOME/crush/crushrc` | `%XDG_CONFIG_HOME%\crush\crushrc` |

Legacy JSON uses `.crush.json` / `crush.json` in the same directories as the
above. Everything found is merged, with project settings overriding global ones
and `crushrc` overriding JSON in the same directory. If a folder has both, they
merge and Crush logs a warning.

Data directories (`~/.local/share/crush` on Unix-like systems and
`%LOCALAPPDATA%\crush` on Windows) contain machine-owned JSON state. Crush does
not discover or execute a `crushrc` from those locations.

> [!NOTE]
> Crush also stores state data in `$XDG_DATA_HOME/crush`
> (`%LOCALAPPDATA%\crush` on Windows). This is application state, and should
> not be edited by hand.

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
      --oauth-kind string           oauth or api_key (default oauth)
      --oauth-flow string           auto, browser, or device (default auto)
      --oauth-issuer URL            authorization server to discover endpoints from
      --oauth-client-id string      OAuth client ID
      --oauth-client-secret string  OAuth client secret (confidential clients)
      --oauth-scope string          request a scope (repeatable)
      --oauth-auth-url URL          authorization endpoint
      --oauth-token-url URL         token endpoint
      --oauth-device-url URL        device authorization endpoint (RFC 8628)
      --oauth-redirect-uri URI      loopback redirect URI the server registered
      --oauth-callback-port int     loopback port for the browser flow
      --oauth-param key value       extra authorization parameter (repeatable)
      --oauth-secret-basic bool     send the client secret with HTTP Basic auth
      --oauth-token-header key value  header sent to the token endpoint (repeatable)
      --oauth-token-encoding string  form or json (default form)
      --usage-url URL               endpoint reporting the quota left
      --usage-method string         HTTP method for the quota call (default GET)
      --usage-body JSON             request body for the quota call (default {})
      --usage-groups PATH           gjson path to the groups holding meters
      --usage-group-label string    field naming each group (default displayName)
      --usage-meters PATH           gjson path to the meters (default buckets)
      --usage-label string          field naming each meter (default displayName)
      --usage-remaining string      field with the fraction or amount left
      --usage-spent-percent string  field with the percent already spent (0-100)
      --usage-reset string          field with the RFC 3339 reset time
      --usage-title string          label for a meter that carries none
      --usage-window string         field with a short meter tag (default window)
      --usage-model-group PREFIX GROUP  model id prefix to group (repeatable)
      --gateway-request JQ          jq program rewriting each request
      --gateway-response JQ         jq program rewriting each response
      --gateway-http1 bool          speak HTTP/1.1 rather than negotiate HTTP/2
      --catalog-url URL             endpoint listing the provider's models
      --catalog-method string       HTTP method for the catalog call (default GET)
      --catalog-body JSON           request body for the catalog call (default {})
      --catalog-program JQ          jq program reading that listing into models
      --catalog-header key value    header sent only with the catalog call (repeatable)
```

```bash
provider add deepseek \
  --type openai-compat \
  --base-url "https://api.deepseek.com/v1" \
  --api-key "${DEEPSEEK_API_KEY:?set DEEPSEEK_API_KEY}"
```

Headers whose value resolves to the empty string (an unset `$VAR`, a
`$(...)` that prints nothing, or a literal `""`) are dropped from the
outgoing request. This makes env-gated headers safe:

```bash
provider add openai \
  --extra-header OpenAI-Organization "$OPENAI_ORG_ID"
```

If `OPENAI_ORG_ID` is unset, the header is simply not sent.

#### `provider remove`

Remove a provider and all custom models registered on it.

```text
Usage:
  provider remove <id>
  provider rm <id>
```

#### OAuth providers

A provider authenticates with `--api-key` unless the `--oauth-*` flags
declare a flow. Any `--oauth-*` flag marks the provider as one that signs
in, which is what makes `crush login <id>` work for a provider Crush does
not ship:

```bash
provider add example \
  --type openai-compat \
  --base-url "https://api.example.com/v1" \
  --oauth-issuer "https://auth.example.com" \
  --oauth-client-id crush \
  --oauth-scope openid \
  --oauth-scope offline_access
```

Endpoints left blank are discovered from the issuer's well-known documents
(RFC 8414). The browser flow (authorization code with PKCE and a loopback
redirect) is the default, falling back to the device flow (RFC 8628) when no
callback can be served — or pin it with `--oauth-flow device`. Tokens are
persisted and refreshed like the built-in subscription providers, and the
models the credential unlocks are discovered once the sign-in lands.

Servers that need a fixed redirect, nonstandard endpoints, or an extra
parameter such as `audience` are covered by `--oauth-redirect-uri`,
`--oauth-auth-url`/`--oauth-token-url`/`--oauth-device-url`, and
`--oauth-param`. Confidential clients whose server expects HTTP Basic auth
set `--oauth-secret-basic true`.

Some token endpoints only answer a request identifying the client they know,
down to its User-Agent, and reject anything else with an opaque "invalid
request format". `--oauth-token-header KEY VALUE` (repeatable) declares the
headers those calls carry, so the plugin supplies the identity rather than
Crush. A server that takes JSON bodies instead of the RFC 6749 form encoding
sets `--oauth-token-encoding json`; with JSON, the authorization-code exchange
also carries the flow's state, which those servers expect:

```bash
provider add example-sub \
  --type openai-compat \
  --base-url "https://api.example.com/v1" \
  --oauth-token-header User-Agent "example-cli/2.1.0" \
  --oauth-token-encoding json
```

The equivalent JSON lives under the provider's `auth` object:

```json
{
  "providers": {
    "example": {
      "type": "openai-compat",
      "base_url": "https://api.example.com/v1",
      "auth": {
        "issuer": "https://auth.example.com",
        "client_id": "crush",
        "scopes": ["openid", "offline_access"]
      }
    }
  }
}
```

#### Remaining quota

A plan that reports how much allowance is left can be read without any Crush
code: declare where the report lives, and `crush usage` prints it.

```bash
provider add example \
  --usage-url "https://api.example.com/v1/remaining-quota" \
  --usage-method POST \
  --usage-groups groups
```

```text
$ crush usage
Google AI Subscription
  Gemini Models · Weekly Limit Remaining: 99% left · resets in 6d 22h
  Gemini Models · Five Hour Limit Remaining: 99% left · resets in 3h 57m
```

The response is walked with [gjson](https://github.com/tidwall/gjson) paths.
`--usage-groups` names the groups holding the meters (each contributing a
label), `--usage-meters` names the meters inside them, and `--usage-label` /
`--usage-remaining` / `--usage-reset` name the fields within each meter.
Those three default to `displayName`, `remainingFraction`, and `resetTime`,
and `--usage-meters` defaults to `buckets`, so the declaration above needs
nothing else. For a response that is a single flat value, set
`--usage-meters .` and name the field with `--usage-remaining`;
`--usage-title` labels it. `--usage-window` names a field carrying a short
tag for the meter (`5h`, `weekly`), which is what the header shows.

A remaining value of 1 or less is read as a share of the allowance, and
anything larger as an amount, such as a credit balance. A plan that instead
reports the other side of the same allowance — what has been spent — names
that field with `--usage-spent-percent`, as a whole percentage from 0 to 100,
and the meter shows its complement. The unit is in the flag name because the
number alone cannot carry it: a plan 1% spent and a plan fully spent are both
written as `1`. The provider's credential (its login token or API key) and
`--extra-header` values are sent with the request, which matters for gateways
that only answer a request identifying a supported client.

The same figures appear in the UI while the model is in use: the header
shows a compact `◆ 5h 99% · weekly 99%` for the current model, and the
sidebar spells the limits out with their refill times. When a plan reports
different limits per model family, `--usage-model-group PREFIX GROUP`
(repeatable) maps a model id to the group its allowance comes from, so the
display shows the two limits that apply rather than every limit on the plan:

```bash
provider add example \
  --usage-url "https://api.example.com/quota" \
  --usage-groups groups \
  --usage-model-group gemini "Gemini" \
  --usage-model-group cld "Claude"
```

A provider with one plan-wide allowance needs no mapping at all: every meter
applies. With a mapping declared, a model id it does not cover shows nothing,
because one family's limits beside another's model reads as a plan being spent
when it is not.

#### Gateway adapters

Some plans are served by a gateway whose wire format differs from the API the
SDK speaks: a different envelope, different model naming, a differently framed
stream. `--gateway-request` and `--gateway-response` declare that translation
as jq programs, so the provider stays configuration and Crush stays
provider-blind.

The request program receives

```json
{"method": "POST", "url": "…", "headers": {}, "body": {}, "request_id": "…", "token": "…"}
```

and returns any of `url` (the request is re-addressed, and the Host header with
it), `headers` to set, `drop_headers` to remove, and `body` to replace:

```bash
provider add example   --type google   --base-url "https://api.example.com/"   --gateway-request '
      .url as $u | .body as $b | .token as $t | .request_id as $id
      | {url: ("https://gateway.example.com/v1internal" + ($u | capture(":/(?<v>[a-zA-Z]+)$").v)),
         headers: {Authorization: ("Bearer " + $t), "Content-Type": "application/json"},
         drop_headers: ["x-goog-api-key"],
         body: {model: $b.model, requestId: $id, request: $b}}'   --gateway-response '(.body.response // .body)'
```

The response program runs once per reply: for a plain body it receives
`{status, headers, body, request_id, token}` and returns the replacement body;
for a server-sent event stream it receives `{event: true, status, headers,
data, final, request_id, token}` and returns an **array** of replacement
events, which may be empty to drop one or longer to append synthesized ones.
`final` marks the last event, which means a rewritten stream trails the
upstream by one event; events whose data is not JSON pass through untouched.
An event's own name, when the stream carries one, is framing rather than data
and is carried over verbatim, because a reader dispatches on it.
Both programs are compiled at load time, so a jq typo fails startup with the
provider named rather than breaking a request later. Adapters are attached to
`google`- and `anthropic`-type providers today; declaring one for another type
warns at load instead of silently doing nothing.

Beyond jq itself the programs have two functions: `sha256hex(s)` returns the
lowercase hex SHA-256 of a string, and `uuid` returns a fresh random RFC 4122
version 4 id. They exist because a program that signs a request, or stamps it
with a per-request id, cannot do either in plain jq. Catalog programs speak the
same dialect.

`--gateway-http1 true` stops the transport negotiating HTTP/2, which Go does
by default against servers that offer it. A provider being addressed as some
other client's own is usually expected to sound like it, and the JS runtimes
most of these clients ship on speak HTTP/1.1.

#### Model catalog

`--discover-models` reads the OpenAI-shaped `/models` listing, which reports
only ids. A provider that publishes more than that — display names, context
windows, which reasoning levels a model takes — can be read directly with
`--catalog-url` and `--catalog-program`: the endpoint, and the jq program that
turns its response into model objects. The field names belong to the provider,
so they stay in the plugin instead of becoming a Go enricher per provider type.

The program receives the parsed listing as its input and returns an array of
objects in the same shape a config file uses for a model — `id`, `name`,
`context_window`, `default_max_tokens`, `can_reason`, `reasoning_levels`,
`default_reasoning_effort`, `supports_attachments`, and the price fields. An
entry with no id is skipped.

```bash
provider add example --type anthropic --base-url "https://api.anthropic.com" \
  --catalog-url "/v1/models" \
  --catalog-header anthropic-version "2023-06-01" \
  --catalog-program '
    .data | map({
      id,
      name: (.display_name // .id),
      context_window: (.max_input_tokens // 0),
      default_max_tokens: (.max_tokens // 0),
      can_reason: ((.capabilities.thinking // {})["supported"] == true),
      reasoning_levels: ([.capabilities.effort // {} | to_entries[]
                          | select(.value.supported == true) | .key])})'
```

A URL beginning with a slash is joined to the provider's `--base-url`; a full
URL is fetched as given. `--catalog-method POST` sends `--catalog-body`, which
defaults to `{}` — the envelope a gateway that serves its quota over POST
typically serves its catalog over too. `--catalog-header` (repeatable) is sent
with the catalog call only, which keeps a header one listing demands off the
provider's other traffic. The provider's credential — its login token, or its
API key when it has no login — is sent as a bearer automatically, and
`--extra-header` values ride along as well.

Models the config declares are not replaced. A declared entry keeps every
field it sets and adopts the rest from the listing, so pinning one model's
default effort costs you nothing else about it:

```bash
provider add example --catalog-url "/v1/models" --catalog-program '…'
model add example/claude-opus-5-5 --reasoning-effort medium
```

Booleans are the exception: a declaration can add a capability, not veto one,
because `false` and "not stated" are the same value in config. Delete a model
from the listing by not selecting it in the program.

The catalog is fetched at load, under the same three-second budget as model
discovery, and its program is compiled then too — a jq typo fails startup
naming the provider rather than leaving it without models. If the fetch fails,
the provider keeps whatever models it declared.

#### Pricing

Crush prices models from the catalog it ships (`catwalk`) and from the
`--price-input`, `--price-output`, `--price-cache-create`, and
`--price-cache-hit` flags on `model add`. There is no pricing endpoint to
fetch from: providers that serve a subscription often publish none at all,
and their model ids are specific to that plan.

For a subscription, `--flat-rate true` is the accurate setting: Crush then
skips per-token cost accumulation rather than reporting dollars that were
never spent. If you still want nominal figures in the UI, declare them
yourself — a plugin is Bash, so it can fetch a price list and pass the flags:

```bash
provider add example --flat-rate true --usage-url "https://api.example.com/quota"

# Prices are just configuration: fetch them at load time if you have a source.
price=$(curl -fsSL "https://example.com/prices/gemini-3.8-flash.json" | jq -r .input)
model add example/gemini-3.8-flash --name "Gemini 3.8 Flash" --price-input "$price"
```

### plugins

The builtins above are the plugin format. A plugin is a Bash script that
runs at config load through the same embedded interpreter, so a provider
can be added without a Crush release. Two directories are scanned:

- `$XDG_CONFIG_HOME/crush/plugins/` — user-wide
- `.crush/plugins/` — the working directory's project folder

Every non-hidden `*.sh` file runs in name order, along with the files one
directory down, which is where an installed repository lives. A project plugin
overrides a global one, and your own crushrc overrides both, so a plugin never
traps a project. A failing plugin fails the load, exactly like a failing
crushrc — and like a crushrc, a plugin is trusted code with your shell
privileges: review it before you install it.

`crush plugin` installs them from GitHub instead of by hand:

```bash
crush plugin install <author>/<repo>[@ref]   # every *.sh at the repository root
crush plugin update [<author>/<repo>[@ref]]  # one repository, or all of them
crush plugin list [--json]                   # the ref and commit each one pins
crush plugin trust <author>/<repo>           # accept edits made after installing
crush plugin uninstall <author>/<repo>
```

An install writes the scripts into `<plugins>/<author>__<repo>/` and records
the exact commit they came from in a hidden `.plugin.json` beside them, which
is what makes an update able to say which commit it moved from and to, and to
delete a plugin the repository dropped. `--project` installs into
`.crush/plugins` so a team can commit one pinned version together. Nothing is
prompted for and nothing is verified: the recorded commit URL is the thing to
read before the next load runs it. `GITHUB_TOKEN` or `GH_TOKEN` is sent when
set, which is how a private repository installs.

A provider that declares an OAuth flow is a sign-in candidate until
`crush login <id>` stores its token: it appears in the model picker but is
not treated as a configured provider, so Crush asks you to authenticate
rather than selecting it with no credential.

One declared this way can carry any OAuth flow and quota report: for
instance a Google AI subscription (Antigravity) provider, with Google's
OAuth client, the gateway's models, and the client secret supplied through
an environment variable the plugin expands.

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
      --reasoning-level string      a selectable reasoning tier (repeatable)
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
      --disabled-tools string       deny a server tool (repeatable)
      --enabled-tools string        allow only these server tools (repeatable)
      --oauth bool                  enable OAuth 2.1 flow (HTTP only)
      --oauth-client-id string      pre-registered OAuth client ID
      --oauth-client-secret string  pre-registered OAuth client secret
      --oauth-callback-port int     fixed localhost port for the OAuth callback
```

```bash
mcp add github --type http \
  --url "https://api.githubcopilot.com/mcp/" \
  --header Authorization "Bearer $GH_PAT"
```

As with providers, a header whose value resolves to the empty string is
dropped from the outgoing request.

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

Configure general Crush behavior, paths, attribution, and the terminal UI.
Boolean values are optional and default to `true`.

```text
Usage:
  option <key> [value]
  option [command]

Available Commands:
  reset     Clear every value from a list option
  ui        Configure terminal UI behavior

Boolean Keys:
  debug                          enable debug logging
  debug-lsp                      enable LSP debug logging
  auto-lsp                       automatically configure language servers
  progress                       show progress indicators
  metrics                        send anonymous usage metrics
  auto-summarize                 automatically summarize long conversations
  provider-auto-update           update the provider catalog automatically
  default-providers              include built-in providers
  attribution-generated-with     add the Generated with Crush line

String Keys:
  data-directory string            directory for project data and state
  initialize-as string             context filename created by crush init
  notifications string             notification style: auto, native, osc, bell,
                                   or disabled
  attribution-trailer-style string attribution trailer: none, co-authored-by,
                                   or assisted-by

Integer Keys:
  request-timeout int              seconds before an LLM request is aborted;
                                   streaming responses are only aborted after
                                   this much inactivity; 0 waits forever
                                   (default 60)

List Keys:
  context-path string             append a project context path
  global-context-path string      append a global context path
  skill-path string               append a skill directory
  disable-skill string            hide a skill from the agent
```

```bash
option progress false
option skill-path ./skills
option attribution-trailer-style assisted-by
```

#### `option reset`

Clear every value previously added to a list option. Values added after the
reset are kept.

```text
Usage:
  option reset <key>

Available Keys:
  context-path          clear project context paths
  global-context-path   clear global context paths
  skill-path            clear additional skill directories
  disable-skill         clear disabled skill names
```

#### `option ui`

Configure terminal UI presentation and completion-list limits.

```text
Usage:
  option ui <key> <value>

Available Keys:
  compact bool                  use the compact chat layout
  diff unified|split            choose unified or side-by-side diffs
  transparent bool              use the terminal background
  mouse bool                    enable terminal mouse capture for clicks,
                                selection, and scrolling in the TUI (default
                                true); disable to let the terminal emulator
                                or tmux handle text selection and copy/paste
  scrollbar string              control chat scrollbar visibility: default,
                                always, or never
  exit-banner default|compact|none
                                control the post-session banner: default shows
                                the Crush logo, compact shows only the resume
                                hint, none hides it entirely
  completions-max-depth int     maximum directory depth shown by completions
  completions-max-items int     maximum items returned to completions
```

```bash
option ui compact true
option ui diff unified
option ui transparent true
option ui mouse false
option ui scrollbar always
option ui exit-banner compact
option ui completions-max-depth 4
option ui completions-max-items 200
```

> [!IMPORTANT]
> These skill paths load by default — you do NOT need `skill-path`
> for them: `.agents/skills`, `.crush/skills`, `.claude/skills`,
> `.cursor/skills`.

> [!NOTE]
> The command palette's "Disable Background Color" and "Disable Mouse" 
> toggles always write to the global config. If a project config
> also sets `transparent` or `mouse`, project settings win on the next
> launch (see [Where config lives](#where-config-lives)), so the toggle can
> look like it silently reverted.

## Composing configs

Because it's Bash, a shared base config is just a `source`:

```bash
# Unix-like: ~/.config/crush/crushrc
# Windows:   %USERPROFILE%\.config\crush\crushrc
source ~/team/crush-base.sh    # sets up providers, a few skills

# …but on this machine, drop a skill path the base added and add my own.
option reset skill-path
option skill-path ~/my/skills
```

`remove`, `rm`, and `option reset` all act on whatever was set earlier in the
script or pulled in via `source`. Later lines win, just like a shell.

## Legacy JSON

`crush.json` is the original format and is now deprecated. We plan to support
it for the forseeable future, but new configuration options will only be added
to Bash-based config.

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

For a full reference, See the [JSON schema](../../schema.json).

In JSON, only selected string fields (API keys, URLs, MCP/LSP commands and args,
headers) are shell-expanded at load time. In `crushrc` there's no such list —
it's all just Bash.

Both formats are trusted code: they run with your shell privileges before the UI
appears. Don't launch Crush in a directory whose config you haven't read.

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
