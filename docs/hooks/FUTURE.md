# Hooks: Future Work

This document tracks planned features and design notes for hooks that are not
yet implemented. Nothing here is part of the current contract. Treat it as a
scratchpad for what's next, not as documentation of current behavior.

> [!NOTE]
> This document was largely LLM-generated.

## `context_files`

**Status:** planned, not implemented.

### Motivation

Today, a hook that wants to inject reference material into the agent's context
has exactly one knob: `context` (string or array of strings). Whatever the hook
puts there is concatenated into what the model sees. That's fine for short notes
("current branch: main", "scrubbed secrets") but it scales badly:

- Dumping a whole `README.md` or `package.json` into `context` burns tokens on
  every tool call where the hook fires.
- The model sees the file contents even if it doesn't need them.
- Large files can push the turn past the context window.

`context_files` is the lazy alternative: the hook returns **paths**, not
contents. Crush tells the agent the files exist and are relevant, and the agent
decides whether to open them with its existing `view` tool.

### Proposed shape

Additive envelope field. Accepts a list of strings:

```jsonc
{
  "decision": "allow",
  "context": "Scrubbed one secret",
  "context_files": ["README.md", "docs/ARCHITECTURE.md"],
}
```

Paths are resolved relative to `CRUSH_CWD`. Non-existent paths are dropped with
a debug log (don't fail the hook over a missing file).

### How the agent sees it

Crush appends a short note to the turn's context along the lines of:

```
## Referenced files
- README.md
- docs/ARCHITECTURE.md
```

No file contents are inlined. The agent opens them with `view` if it decides
they're relevant. This keeps cost proportional to need.

### Aggregation

Matches the existing rules for lists:

- Concatenates across matching hooks in config order.
- Deduplicates paths (same file referenced by two hooks → listed once).
- Dropped entirely if the final decision is `deny` or `halt`.

### Backwards compatibility

Purely additive. Hooks that don't emit `context_files` are unaffected. Existing
envelopes keep working unchanged. No version bump required.

### Open questions

- Should `context_files` paths be constrained to `CRUSH_PROJECT_DIR`? Probably
  yes, to avoid hooks smuggling in arbitrary filesystem reads.
- Do we want a per-file line range (`"README.md:1-40"`) or keep it dead simple
  (whole-file references only)? Start simple; add ranges only if asked for.
- Should we annotate "why this file is relevant" per entry? An object form
  (`{"path": "...", "reason": "..."}`) would allow that but complicates the
  schema. Defer until there's a real user need.

## Sub-agent opt-in

**Status:** not implemented.

### Background

Today hooks fire **only** on the top-level agent's tool calls. Sub-agents
(`agent` task tool, `agentic_fetch`, future delegated loops) run without hook
interception so a single delegated turn doesn't trigger the user's hook N times.

The outer sub-agent tool call itself is hooked, so blanket policy like "never
spawn sub-agents" or "rewrite prompts sent to the task agent" still works from
the coder's side. The sub-agent's inner loop is the part that's exempt.

### Why users might want the escape hatch

- Audit logging of every tool call, including delegated ones.
- Redaction hooks that want to apply uniformly regardless of who called the
  tool.
- Policy that cares about the _tool_ not the _caller_: "never fetch from this
  domain, even in `agentic_fetch`."

Until someone actually asks, don't ship this. YAGNI.

### Proposed shape

Additive, per-hook. Zero-value matches current default (skip sub-agents):

```jsonc
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "^bash$",
        "command": "./hooks/audit.sh",
        "include_sub_agents": true, // default false
      },
    ],
  },
}
```

Implementation changes where `wrapToolsWithHooks` decides to skip. Instead of a
single `isSubAgent` bailout, the runner filters per-hook matches by the hook's
`include_sub_agents` flag. Hooks that opt in get wrapped into sub-agent tool
slices too; everything else stays skipped.

### Backwards Compatibility

Purely additive. Hooks that don't set `include_sub_agents` get the default
(`false` = skip sub-agents). No wire format change, no version bump. The initial
transition from "hooks fire everywhere" to "hooks skip sub-agents by default"
was a one-time behavior change; adding the opt-in is pure addition.

### Side benefit: payload awareness

Extend the stdin payload with `"is_sub_agent": true|false` so hook scripts that
opt in can branch on caller type ("audit top-level and sub-agent calls
differently"). Also purely additive — hooks that don't read the field are
unaffected.

### Open questions

- Per-hook flag (above) vs a global `hooks.include_sub_agents` default? A global
  toggle is simpler but coarse-grained; per-hook is more flexible and
  composable. Start per-hook; a global default can be layered on later with
  explicit precedence ("per-hook overrides global").
- Does an opt-in hook see hooks from _nested_ sub-agents too (a sub-agent that
  itself calls a sub-agent)? Probably yes — once you've opted in you want the
  full tree. But call it out explicitly in docs so users aren't surprised by N²
  explosions on pathological configs.

## Cross-platform shell (Windows support)

**Status:** implemented. See the [Execution model](README.md#execution-model)
section in `README.md` for the current behavior and contract.
