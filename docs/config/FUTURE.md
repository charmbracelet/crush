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
