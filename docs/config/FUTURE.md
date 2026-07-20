# Config: Future Work

This document tracks planned features and design notes for configuration that
are not yet implemented. Nothing here is part of the current contract. Treat
it as a scratchpad for what's next, not as documentation of current behavior.

> [!NOTE]
> This document was largely LLM-generated.

## Real-time config from the `bash` tool

**Status:** planned, not implemented.

### Motivation

Right now `crushrc` runs once, at startup. If you want to change something
mid-session — swap the large model, allow a tool, add an MCP server — you edit
the file and restart.

The idea: let the agent's `bash` tool run the same config commands
(`model large …`, `option …`, `mcp add …`) to reconfigure the **running
session**. The mental model is exactly a shell and its `.bashrc`:

- Running a config command changes the **current session only** — like typing
  `export` or `alias` at a live prompt.
- To make it stick, you edit your `crushrc` — like editing `.bashrc`.

So you could say "Crush, switch to the small model for a bit" and it just
runs `model small …`, live, no restart.

> [!IMPORTANT]
> **Persistence is a non-goal.** The bash tool never writes config files.
> This is deliberate: a script can't be round-tripped. You don't regenerate
> your `.bashrc` from the live shell's state, and we won't regenerate
> `crushrc` from live config state. Want it permanent? Edit `crushrc`.

### Why it's mostly wiring

The commands already exist and the bash tool already reaches them — they're
just switched off there on purpose. Config builtins check for a config sink on
the context and no-op when there isn't one, which is why typing `provider add`
in a normal bash tool call today does nothing. Real-time config is largely a
matter of attaching a **live, ephemeral** sink instead of that no-op.

The runtime mutation layer also already exists and is used today by the
model-switch dialog and API-key entry:

- An in-memory, copy-on-write config mutator.
- Ephemeral "runtime overrides" that never touch disk — the sink we'd target.
- Pub/sub plus on-demand MCP/LSP startup for reacting to changes.

### Proposed shape

No new commands — the same builtins, now live:

```bash
# Inside a session, via the bash tool:
model small anthropic/claude-haiku-4-20250514   # switch models for this turn
option progress false                            # quiet the UI
permissions allow grep                           # stop asking about grep
```

Each applies immediately to the session and is forgotten on exit.

### Steps

1. **Attach a live applier to the bash tool's context** — analogous to the
   load-time builder, but pointed at the ephemeral runtime overrides. When
   present, config builtins apply to the session instead of no-oping.
2. **Route builtin output to ephemeral mutation** — never to the on-disk
   writers. Live, not persisted.
3. **Reconcile subsystems**, roughly in order of difficulty:
   - Easy: `option` flags, `permissions allow`, disabled tools/skills — read
     on demand.
   - Medium: `model large|small` — already reconciled live by the switch
     dialog.
   - Harder: `provider add` — rebuild the model client with the new
     key/base-url.
   - Hardest: `mcp` / `lsp` add/remove — process lifecycle
     (start/stop/reconnect); the on-demand-startup machinery helps.
4. **Guardrails** — see below.

### Guardrails

This is a privilege-escalation surface: an agent that can rewrite its own
config could grant itself tools, swap providers, or add an MCP server that
exfiltrates data. So:

- Gate it behind a permission prompt, like other sensitive actions.
- Make it opt-in.
- Consider a denylist for the scariest fields (API keys, `permissions`) even
  when the feature is on.

### Suggested first cut

Ephemeral, session-only apply for the **easy tier** (`option`, `permissions`,
`model`) behind a permission prompt. High value, small blast radius, and it
sidesteps persistence entirely. Live `provider` / `mcp` / `lsp` reload is a
larger, later increment.

### Open questions

- Should a live change be echoed back to the user somehow ("switched large
  model to …"), so it's not silent?
- Do we want a way to *see* the effective live config from the bash tool
  (a read-only `option get` / `model large` print)? `model large` already
  prints its selection; a broader introspection surface could follow.
- Should sub-agents be allowed to reconfigure the session, or only the
  top-level agent? Probably top-level only, mirroring how hooks scope.

## Separate machine state from user configuration

**Status:** planned for a later phase; not implemented.

### Motivation

Crush currently uses JSON files in data directories as both persisted machine
state and high-priority configuration:

```text
~/.local/share/crush/crush.json
.crush/crush.json
```

These files hold mutable choices such as preferred/recent models, UI settings,
workspace overrides, and some credentials. Treating them as ordinary config
means state enters the same generic JSON merge/reload path as user-authored
`crushrc` and legacy `crush.json` files.

The goal is to make the roles explicit:

| Role | Format |
|---|---|
| User-authored executable configuration | `crushrc` / `.crushrc` |
| Legacy user-authored static configuration | `crush.json` / `.crush.json` |
| Crush-owned persistent preferences and history | versioned `state.json` |
| Session-only changes | memory |
| Credentials/OAuth tokens | dedicated secure storage |

JSON remains a good state format: it is standard-library supported, readable,
easy to debug, easy to attach to bug reports, and straightforward to migrate.
The problem is not JSON serialization itself; it is state pretending to be
config and being deep-merged through the config pipeline.

### Proposed files

```text
~/.config/crush/crushrc             global user config
~/.local/share/crush/state.json     global machine state
./crushrc / ./.crushrc              project user config
.crush/state.json                   workspace machine state
```

State files should be machine-owned, written with `0600`, protected by the
existing process/file locks, and updated with atomic temp-file renames. They
should include a format version and an explicit generated-file notice.

```json
{
  "version": 1,
  "recent_models": {
    "large": [
      {"provider": "openai", "model": "gpt-5"}
    ]
  },
  "preferred_models": {
    "large": {"provider": "openai", "model": "gpt-5"}
  },
  "ui": {
    "compact_mode": true
  }
}
```

Use typed state structs with pointer fields where `false` must be
distinguishable from "not remembered". Avoid arbitrary dotted JSON paths and
avoid decoding state into the full `config.Config` shape.

### Load precedence

Explicit user configuration should beat remembered state:

```text
built-in defaults
→ global state defaults
→ workspace state defaults
→ global legacy crush.json
→ global crushrc
→ project legacy crush.json
→ project crushrc
→ project .crush.json
→ project .crushrc
→ runtime-only overrides
```

Recent models are metadata, not defaults, so they can be attached after config
building without participating in precedence.

### Legacy JSON compatibility

User-authored JSON remains a supported config input during this work:

```text
~/.config/crush/crush.json
./crush.json
./.crush.json
```

It must be decoded as configuration, never migrated as state. Only the
machine-owned files in data/workspace directories are migrated. The role is
determined by path, not guessed from content.

If user JSON is retired later:

1. Keep reading it for a compatibility period.
2. Warn only when a user-authored JSON config is loaded.
3. Provide an explicit conversion command (for example,
   `crush config convert crush.json > crushrc`).
4. Never rewrite user config automatically.

Using JSON internally for state is independent of deprecating JSON as a user
configuration language.

### State store API

Introduce a narrow typed store rather than generic config-field mutation:

```go
type StateStore interface {
    Load(context.Context) (State, error)
    Update(context.Context, func(*State) error) error
}
```

An update should lock, read/migrate, mutate, validate, marshal with indentation,
write atomically, and quarantine corrupt files. It should not trigger a full
config reload.

Typed config mutators then update live config and the corresponding state
field:

```go
func (s *ConfigStore) SetCompactMode(scope Scope, enabled bool) error {
    s.mutateInMemory(func(c *Config) {
        c.ensureTUI().CompactMode = enabled
    })

    return s.stateStore(scope).Update(func(st *State) error {
        st.UI.CompactMode = ptr.To(enabled)
        return nil
    })
}
```

Real-time commands run through the Bash tool remain session-only and do not
write state, preserving the shell/`.bashrc` mental model.

### Typed crushrc builder

This is related but separate. Today the Bash path is:

```text
crushrc → map[string]any → JSON → Config
```

A later typed-builder phase should become:

```text
crushrc → typed ConfigBuilder → Config
state.json → typed StateStore ───────┘
```

Legacy `crush.json` would decode into a typed config patch and apply to the same
builder. This may require moving pure config data types into a dependency-neutral
package to avoid import cycles.

### Migration plan

1. Add a versioned `State` type and atomic JSON state store.
2. Migrate recent/preferred models.
3. Migrate remembered UI settings.
4. Stop merging global/workspace data JSON as config.
5. Move provider credentials and OAuth tokens to dedicated secure storage.
6. Remove generic state callers of `SetConfigField` / dotted JSON paths.
7. Replace the `crushrc` map/JSON bridge with a typed config builder.

Migration must preserve unknown legacy fields or warn and leave the original
file untouched. Successfully migrated files can be renamed to
`crush.json.migrated`; corrupt files should be quarantined as timestamped
`state.json.corrupt-*` files and replaced with defaults.

## Permission-level hard deny

**Status:** not implemented; probably unnecessary until a real use case
appears.

Crush currently has three useful tool states across both config formats:

| State | `crushrc` | `crush.json` | Behavior |
|---|---|---|---|
| Auto-approved | `permissions allow bash` | `permissions.allowed_tools` | Visible; runs without prompting |
| Prompted | neither list | neither list | Visible; asks the user before running |
| Disabled | `permissions deny bash` | `options.disabled_tools` | Hidden from the agent; cannot be called |

`permissions deny` writes `options.disabled_tools`. That is a practical hard
block: because the tool is absent from the agent's tool list, the model cannot
attempt to use it.

The one state Crush does **not** have is "visible but always rejected": the
model can see and choose the tool, but the permission engine denies every
request without prompting. Supporting that would require a separate
permission-level deny list in both the config schema and permission engine.

### Why defer it

A visible-but-unusable tool wastes model attention and tool-call attempts. If
a tool must never run, hiding it is both stronger and clearer. Add a true deny
list only if someone has a concrete need for the model to know a tool exists
while being categorically forbidden from calling it.
