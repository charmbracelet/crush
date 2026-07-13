# Plan: Bash-Powered Config Format for Crush

## Motivation

Crush's config (`crush.json`) is JSON parsed by Go. String values already
support shell expansion (`$VAR`, `$(cmd)`, `${VAR:-default}`) via
`VariableResolver` / `shell.ExpandValue` at load time. But JSON itself is static
— no includes, no conditionals, no variables, no composition.

A Bash-based config format (`crush.sh`) would give users:

- **Includes** — `source ~/.config/crush/shared.sh` (native shell)
- **Secrets** — `provider openai --api-key "$(op read 'op://vault/openai')"`
  (native `$(...)`)
- **Conditionals** — different providers/models per machine, OS, or environment
- **Variables** — define a key once, reuse across providers
- **Environment awareness** — `if [[ -n "$CI" ]]; then ...`

The shell evaluation path already exists and is battle-tested
(`internal/config/resolve.go`, `internal/shell/ExpandValue`). A Bash config
format reuses that infrastructure — it just needs a loader and a set of shell
builtins.

## Design

### Format

A `crush.sh` file is a plain Bash script. Crush discovers it alongside
`crush.json` in the same directory traversal (`internal/config/load.go:866`
`lookupConfigs`). The script uses Crush-provided builtin commands (shell
builtins registered via `internal/shell/run.go`) to build a `Config` struct in
Go memory.

Each builtin is a flag-based command — the same pattern `jq` already uses
(`internal/shell/jq.go`). No block parser, no stateful context, no reserved
sub-commands. One builtin per config section, each parsing its own
`--flag value` pairs from the args slice.

### Example

```bash
#!/usr/bin/env bash
# crush.sh

# Includes
source ~/.config/crush/secrets.sh

# Conditionals
if [[ "$(hostname)" == "work-laptop" ]]; then
  OPENAI_KEY=$(op read "op://work/openai/key")
else
  OPENAI_KEY=$(cat ~/.secrets/openai)
fi

# Providers
provider openai --api-key "$OPENAI_KEY"
provider anthropic --api-key "$ANTHROPIC_API_KEY"
provider my-llm \
  --type openai \
  --api-key "ollama" \
  --base-url "http://localhost:11434/v1"

# Models
model large --provider openai --model gpt-4o --think
model small --provider anthropic --model claude-3-5-haiku

# MCP Servers
mcp github --type stdio --command npx --args "-y" "@modelcontextprotocol/server-github"
mcp local-server --type http --url "http://localhost:3000/mcp" --header "Authorization" "Bearer $TOKEN"

# LSP Servers
lsp gopls --command gopls --filetypes go,mod --root-markers go.mod

# Permissions
permissions --allow bash --allow view

# Hooks
hook PreToolUse --matcher "bash" --command "echo 'Running bash'" --timeout 10

# Options
options --data-directory .crush --disable-metrics
```

### Why flags (not key/value pairs or block style)

- **Universal**: `--flag value` is understood by every CLI user
- **Order-independent**: no implicit "current provider" state
- **No reserved words**: `type`, `env`, `command` are flag names, not
  sub-commands that shadow real shell commands
- **Self-documenting**: `--api-key` is clearer than positional `api_key`
- **Proven pattern**: the `jq` builtin already parses `--flag value` pairs from
  its args slice the same way (`internal/shell/jq.go:66`)
- **Shell handles line continuation**: `\` for long configs, no parser needed

## Architecture

### 1. Config Builder

A Go struct that accumulates config state as builtins execute. Stored on the
shell context so builtins can access it:

```go
// internal/shell/config_builder.go

type ConfigBuilder struct {
    Config *config.Config
}

type configBuilderCtxKey struct{}

func ConfigBuilderFromCtx(ctx context.Context) *ConfigBuilder {
    v, _ := ctx.Value(configBuilderCtxKey{}).(*ConfigBuilder)
    return v
}

func WithConfigBuilder(b *ConfigBuilder) context.Context {
    return context.WithValue(context.Background(), configBuilderCtxKey{}, b)
}
```

### 2. Shell Builtins

Each config section gets one builtin. Registered in `builtinHandler()` in
`internal/shell/run.go:311`, alongside the existing `jq` case.

#### `provider <id> [flags]`

Maps to `config.ProviderConfig` (`internal/config/config.go:90`).

| Flag                     | Field                | Type   | Default               |
| ------------------------ | -------------------- | ------ | --------------------- |
| `--name`                 | `Name`               | string | same as `<id>`        |
| `--type`                 | `Type`               | string | `"openai"`            |
| `--api-key`              | `APIKey`             | string | —                     |
| `--base-url`             | `BaseURL`            | string | —                     |
| `--disable`              | `Disable`            | bool   | `false`               |
| `--flat-rate`            | `FlatRate`           | bool   | `false`               |
| `--extra-header`         | `ExtraHeaders[k]`    | string | — (takes key + value) |
| `--system-prompt-prefix` | `SystemPromptPrefix` | string | —                     |

#### `model <large|small> [flags]`

Maps to `config.SelectedModel` (`internal/config/config.go:64`).

| Flag                 | Field             | Type                  |
| -------------------- | ----------------- | --------------------- |
| `--provider`         | `Provider`        | string                |
| `--model`            | `Model`           | string                |
| `--think`            | `Think`           | bool (flag, no value) |
| `--reasoning-effort` | `ReasoningEffort` | string                |
| `--max-tokens`       | `MaxTokens`       | int64                 |
| `--temperature`      | `Temperature`     | float64               |

#### `mcp <name> [flags]`

Maps to `config.MCPConfig` (`internal/config/config.go:189`). Stored in
`Config.MCP` (a `map[string]MCPConfig`).

| Flag               | Field           | Type     | Default               |
| ------------------ | --------------- | -------- | --------------------- |
| `--type`           | `Type`          | string   | `"stdio"`             |
| `--command`        | `Command`       | string   | —                     |
| `--args`           | `Args`          | string[] | — (repeatable)        |
| `--env`            | `Env[k]`        | string   | — (takes key + value) |
| `--url`            | `URL`           | string   | —                     |
| `--header`         | `Headers[k]`    | string   | — (takes key + value) |
| `--timeout`        | `Timeout`       | int      | `15`                  |
| `--disabled`       | `Disabled`      | bool     | `false`               |
| `--disabled-tools` | `DisabledTools` | string[] | — (repeatable)        |
| `--enabled-tools`  | `EnabledTools`  | string[] | — (repeatable)        |

#### `lsp <name> [flags]`

Maps to `config.LSPConfig` (`internal/config/config.go:209`). Stored in
`Config.LSP` (a `map[string]LSPConfig`).

| Flag             | Field         | Type        | Default                           |
| ---------------- | ------------- | ----------- | --------------------------------- |
| `--command`      | `Command`     | string      | —                                 |
| `--args`         | `Args`        | string[]    | — (repeatable)                    |
| `--env`          | `Env[k]`      | string      | — (takes key + value)             |
| `--filetypes`    | `FileTypes`   | string[]    | — (repeatable or comma-separated) |
| `--root-markers` | `RootMarkers` | string[]    | — (repeatable or comma-separated) |
| `--timeout`      | `Timeout`     | int         | `30`                              |
| `--disabled`     | `Disabled`    | bool        | `false`                           |
| `--init-options` | `InitOptions` | JSON string | —                                 |
| `--options`      | `Options`     | JSON string | —                                 |

#### `permissions [flags]`

Maps to `config.Permissions` (`internal/config/config.go:249`).

| Flag      | Field          | Type                  |
| --------- | -------------- | --------------------- |
| `--allow` | `AllowedTools` | string[] (repeatable) |

#### `hook <event> [flags]`

Maps to `config.HookConfig` (`internal/config/config.go:573`). Stored in
`Config.Hooks` (a `map[string][]HookConfig`).

| Flag        | Field     | Type              | Default       |
| ----------- | --------- | ----------------- | ------------- |
| `--matcher` | `Matcher` | string            | — (match all) |
| `--command` | `Command` | string (required) | —             |
| `--timeout` | `Timeout` | int               | `30`          |
| `--name`    | `Name`    | string            | —             |

#### `options [flags]`

Maps to `config.Options` (`internal/config/config.go:276`).

| Flag                             | Field                       | Type              | Default        |
| -------------------------------- | --------------------------- | ----------------- | -------------- |
| `--data-directory`               | `DataDirectory`             | string            | `.crush`       |
| `--context-path`                 | `ContextPaths`              | string[]          | — (repeatable) |
| `--global-context-path`          | `GlobalContextPaths`        | string[]          | —              |
| `--skills-path`                  | `SkillsPaths`               | string[]          | —              |
| `--debug`                        | `Debug`                     | bool              | `false`        |
| `--debug-lsp`                    | `DebugLSP`                  | bool              | `false`        |
| `--disable-auto-summarize`       | `DisableAutoSummarize`      | bool              | `false`        |
| `--disable-provider-auto-update` | `DisableProviderAutoUpdate` | bool              | `false`        |
| `--disable-default-providers`    | `DisableDefaultProviders`   | bool              | `false`        |
| `--disable-metrics`              | `DisableMetrics`            | bool              | `false`        |
| `--disable-notifications`        | `DisableNotifications`      | bool              | `false`        |
| `--initialize-as`                | `InitializeAs`              | string            | `AGENTS.md`    |
| `--notification-style`           | `NotificationStyle`         | string            | `auto`         |
| `--disabled-tools`               | `DisabledTools`             | string[]          | —              |
| `--disabled-skills`              | `DisabledSkills`            | string[]          | —              |
| `--auto-lsp`                     | `AutoLSP`                   | bool              | `true`         |
| `--progress`                     | `Progress`                  | bool              | `true`         |
| `--no-auto-lsp`                  | `AutoLSP`                   | bool (sets false) | —              |
| `--no-progress`                  | `Progress`                  | bool (sets false) | —              |

### 3. Builtin Implementation Pattern

Each builtin follows the `jq` pattern (`internal/shell/jq.go`):

```go
// internal/shell/config_provider.go

func handleProvider(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
    if len(args) < 2 {
        fmt.Fprintln(stderr, "usage: provider <id> [--name NAME] [--type TYPE] [--api-key KEY] ...")
        return interp.ExitStatus(2)
    }

    b := ConfigBuilderFromCtx(ctx)
    if b == nil {
        return fmt.Errorf("provider: no config builder in context")
    }

    p := &config.ProviderConfig{
        ID:   args[1],
        Name: args[1],
    }

    i := 2
    for i < len(args) {
        switch args[i] {
        case "--name":
            p.Name = args[i+1]
            i += 2
        case "--type":
            p.Type = catwalk.Type(args[i+1])
            i += 2
        case "--api-key":
            p.APIKey = args[i+1]
            i += 2
        case "--base-url":
            p.BaseURL = args[i+1]
            i += 2
        case "--disable":
            p.Disable = args[i+1] == "true"
            i += 2
        case "--flat-rate":
            p.FlatRate = args[i+1] == "true"
            i += 2
        case "--system-prompt-prefix":
            p.SystemPromptPrefix = args[i+1]
            i += 2
        case "--extra-header":
            if i+2 >= len(args) {
                fmt.Fprintln(stderr, "provider: --extra-header requires key and value")
                return interp.ExitStatus(2)
            }
            if p.ExtraHeaders == nil {
                p.ExtraHeaders = make(map[string]string)
            }
            p.ExtraHeaders[args[i+1]] = args[i+2]
            i += 3
        default:
            fmt.Fprintf(stderr, "provider: unknown flag %s\n", args[i])
            return interp.ExitStatus(2)
        }
    }

    b.Config.SetProvider(p.ID, *p)
    return nil
}
```

### 4. Registration

Add cases to the switch in `builtinHandler()` in `internal/shell/run.go:317`:

```go
switch args[0] {
case "jq":
    hc := interp.HandlerCtx(ctx)
    return handleJQ(ctx, args, hc.Stdin, hc.Stdout, hc.Stderr)
case "provider":
    hc := interp.HandlerCtx(ctx)
    return handleProvider(ctx, args, hc.Stdin, hc.Stdout, hc.Stderr)
case "model":
    hc := interp.HandlerCtx(ctx)
    return handleModel(ctx, args, hc.Stdin, hc.Stdout, hc.Stderr)
case "mcp":
    hc := interp.HandlerCtx(ctx)
    return handleMCP(ctx, args, hc.Stdin, hc.Stdout, hc.Stderr)
case "lsp":
    hc := interp.HandlerCtx(ctx)
    return handleLSP(ctx, args, hc.Stdin, hc.Stdout, hc.Stderr)
case "permissions":
    hc := interp.HandlerCtx(ctx)
    return handlePermissions(ctx, args, hc.Stdin, hc.Stdout, hc.Stderr)
case "hook":
    hc := interp.HandlerCtx(ctx)
    return handleHook(ctx, args, hc.Stdin, hc.Stdout, hc.Stderr)
case "options":
    hc := interp.HandlerCtx(ctx)
    return handleOptions(ctx, args, hc.Stdin, hc.Stdout, hc.Stderr)
default:
    return next(ctx, args)
}
```

### 5. Discovery and Loading

Modify `lookupConfigs` (`internal/config/load.go:866`) to also search for
`crush.sh` and `.crush.sh`:

```go
configNames := []string{
    appName + ".json",
    "." + appName + ".json",
    appName + ".sh",
    "." + appName + ".sh",
}
```

Modify `loadFromConfigPaths` (`internal/config/load.go:887`) to handle `.sh`
files differently from `.json` files:

- `.json` files: read bytes, validate JSON, collect for `jsons.Merge`
- `.sh` files: create a `ConfigBuilder`, seed it onto the shell context, run the
  script via `shell.Run`, extract the populated `*Config` from the builder,
  marshal it to JSON, and collect that JSON for `jsons.Merge`

This keeps the existing merge pipeline intact — `.sh` files produce JSON that
merges with `.json` files using the same deep-merge logic. Priority order is
unchanged: closer-to-cwd wins, global configs are lowest priority.

```go
func loadFromConfigPaths(configPaths []string) (*Config, []string, error) {
    var configs [][]byte
    var loaded []string

    for _, path := range configPaths {
        data, err := os.ReadFile(path)
        if err != nil {
            if os.IsNotExist(err) {
                continue
            }
            return nil, nil, fmt.Errorf("failed to open config file %s: %w", path, err)
        }
        if len(data) == 0 {
            continue
        }

        if strings.HasSuffix(path, ".sh") {
            jsonBytes, err := loadShellConfig(path)
            if err != nil {
                return nil, nil, fmt.Errorf("failed to load shell config %s: %w", path, err)
            }
            configs = append(configs, jsonBytes)
        } else {
            if !json.Valid(data) {
                return nil, nil, fmt.Errorf("invalid JSON in config file %s", path)
            }
            configs = append(configs, data)
        }
        loaded = append(loaded, path)
    }

    cfg, err := loadFromBytes(configs)
    if err != nil {
        return nil, nil, err
    }
    return cfg, loaded, nil
}

func loadShellConfig(path string) ([]byte, error) {
    src, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    builder := &ConfigBuilder{Config: &Config{}}
    ctx := WithConfigBuilder(builder)

    err = shell.Run(ctx, shell.RunOptions{
        Command: string(src),
        Cwd:     filepath.Dir(path),
        Env:     os.Environ(),
    })
    if err != nil {
        return nil, err
    }

    return json.Marshal(builder.Config)
}
```

### 6. JSON Output from Shell Configs

Since `loadShellConfig` marshals the builder's `Config` to JSON and feeds it
into the same `jsons.Merge` pipeline, shell configs and JSON configs are fully
interoperable:

- A global `crush.json` can define base providers
- A project-level `crush.sh` can override or add providers, hooks, MCPs
- The merge resolves conflicts with the same priority rules (closer-to-cwd wins)

### 7. Scope of Builtins

The config builtins (`provider`, `model`, `mcp`, `lsp`, `permissions`, `hook`,
`options`) should only be available when Crush is loading a `crush.sh` config
file, not when the `bash` tool runs commands during an agent session. This
prevents agents from modifying config via shell commands.

Two approaches:

- **Option A — Separate handler**: Use a dedicated `standardHandlers` variant
  that includes config builtins only for config loading, not for the bash tool.
- **Option B — Context gating**: Always register the builtins but check for the
  `ConfigBuilder` in context. If absent (normal bash tool execution), fall
  through to `next`. This is simpler and naturally scoped.

Recommended: **Option B**. The builtins are no-ops without a `ConfigBuilder` in
the context, which only exists during config loading.

## Implementation Steps

1. **Create `ConfigBuilder`** (`internal/shell/config_builder.go`)
   - Struct with `*config.Config` pointer
   - Context key and helper functions

2. **Implement `provider` builtin** (`internal/shell/config_provider.go`)
   - Flag parsing loop
   - Maps to `config.ProviderConfig`
   - Unit tests

3. **Implement `model` builtin** (`internal/shell/config_model.go`)
   - Maps to `config.SelectedModel`
   - Unit tests

4. **Implement `mcp` builtin** (`internal/shell/config_mcp.go`)
   - Maps to `config.MCPConfig`
   - Handle repeatable `--args`, `--env` key/value, `--header` key/value
   - Unit tests

5. **Implement `lsp` builtin** (`internal/shell/config_lsp.go`)
   - Maps to `config.LSPConfig`
   - Unit tests

6. **Implement `permissions` builtin** (`internal/shell/config_permissions.go`)
   - Maps to `config.Permissions`
   - Unit tests

7. **Implement `hook` builtin** (`internal/shell/config_hook.go`)
   - Maps to `config.HookConfig`
   - Stored in `Config.Hooks[event]`
   - Unit tests

8. **Implement `options` builtin** (`internal/shell/config_options.go`)
   - Maps to `config.Options`
   - Unit tests

9. **Register builtins** in `builtinHandler()` (`internal/shell/run.go`)
   - Add cases for all config builtins
   - Use context gating (Option B)

10. **Discovery** — modify `lookupConfigs` (`internal/config/load.go`)
    - Add `crush.sh` and `.crush.sh` to `configNames`

11. **Loader** — modify `loadFromConfigPaths` (`internal/config/load.go`)
    - Branch on `.sh` suffix
    - Implement `loadShellConfig` function
    - Marshal builder output to JSON for merge pipeline

12. **Integration tests**
    - End-to-end: write a `crush.sh`, load it, verify `Config` struct
    - Merge: `crush.json` + `crush.sh` in the same directory tree
    - Includes: `source` another `.sh` file with config builtins
    - Error cases: unknown flags, missing required args, script failures

13. **Documentation**
    - Update `AGENTS.md` with the new config format
    - User-facing docs explaining the Bash config format
    - Migration guide: JSON to Bash (optional, both coexist)

## Precedence and Merging

`crush.json` and `crush.sh` coexist and merge through the same `jsons.Merge`
deep-merge pipeline. The priority rules are unchanged from today:

- Global configs (lowest priority) → parent directories → cwd (highest
  priority)
- Within each directory, `crush.sh` is listed after `crush.json` in
  `configNames`, so on conflicting keys `.sh` wins over `.json`

### Same-directory coexistence

If both `crush.json` and `crush.sh` exist in the **same directory**, Crush
emits a warning and still merges them (`.sh` wins on conflicts). This handles
the common case of migrating from JSON to Bash incrementally — you can move
sections one at a time from `crush.json` to `crush.sh` without breaking
anything.

### Cross-directory coexistence

This is the expected primary use case. A global `~/.config/crush/crush.json`
can define base providers, while a project-level `crush.sh` overrides or adds
providers, hooks, and MCPs. The merge is seamless — `.sh` output is JSON
that feeds into the same pipeline.

## Open Questions

- **`--args` for MCP/LSP**: Should `--args` be repeatable
  (`--args foo --args bar`) or comma-separated (`--args foo,bar`), or both?
  Repeatable is more shell-natural but verbose. Comma-separated is compact but
  breaks on args containing commas. Recommend: repeatable only.

- **`--env` and `--header` key/value**: These take two positional args after the
  flag (`--env KEY VALUE`). Should the value be quoted? Yes — the shell handles
  quoting, and the builtin receives the already-parsed value.

- **Nested options for LSP**: `--init-options` and `--options` take JSON
  strings. Alternative would be a dotted key syntax
  (`--init-option foo.bar value`), but JSON strings are simpler and match the
  existing `ExtraBody` pattern for providers.

- **Config introspection builtins**: Should there be a `get` builtin for reading
  config values during script execution? E.g. `get provider openai --api-key`.
  Useful for conditional logic but adds complexity. Defer to a later phase.

- **Validation**: Should the shell config loader validate the `Config` struct
  the same way JSON configs are validated? Yes — after merge, the existing
  validation in `Load()` runs unchanged.

## Non-Goals

- **Replacing JSON config**: JSON and Bash configs coexist and merge. Users
  choose whichever they prefer.
- **Turing-complete config evaluation**: While Bash is Turing-complete, the
  config builtins are a flat set of commands. No loops over providers, no
  generated config sections. Users can do this in Bash if they want, but the
  builtins don't encourage it.
- **Config file generation from Bash**: No `export` or `emit` command. The
  builtins write directly to Go memory; there's no intermediate JSON.
