---
name: execution-routing
description: Route substantial re.code tasks across native tools, web research, and sub-agents without repeating failed strategies.
---

# Execution Routing

Use the smallest capable tool first, preserve the user's intent across steps,
and escalate only when evidence justifies it.

## Decision Path

1. Ground the task before acting:
   - `pwd` for the working directory.
   - `recode_info` for active runtime, config, MCP, skill, and permission state.
   - native `view`, `grep`, `glob`, and `ls` for local files.
2. Read before editing. Use structured parsers for JSON, YAML, databases, and
   other structured data. Verify the result on the same surface that changed.
3. For external names, packages, APIs, versions, or server identities, use
   native `web_search`, then an official source or authoritative registry.
4. Delegate a heavy independent investigation to `agent` only after the root
   agent has verified the target path and can state a precise deliverable.
5. Keep ownership at the root: reconcile sub-agent findings, perform edits,
   run tests, and continue until the user's intent is satisfied.

## Native Tool Reference

| Need | Tool |
| --- | --- |
| Exact file read | `view` |
| Text or symbol search | `grep` |
| File discovery | `glob` or bounded `ls` |
| Host/runtime/package facts | shell with finite output |
| Current external fact | `web_search`, then `web_fetch` for the chosen source |
| Public repository symbol search | Sourcegraph or GitHub Grep when available |
| Multi-file independent research | `agent` after grounding |
| re.code configuration/runtime truth and canonical write target | `recode_info` |
| Configure, start, and verify one MCP server | `mcp_add` |
| Reconcile all existing MCP servers | `mcp_refresh` |

## Sub-Agent Contract

Give a sub-agent the verified absolute path, the concrete question or change,
constraints, and expected evidence. Do not delegate a vague request such as
"fix everything" or ask it to rediscover the entire machine. Parallelize only
independent reads or investigations; serialize edits that can touch the same
files or configuration.

## Failure Escalation

- First failure: return the complete error to the current run and correct the evidenced assumption once.
- External identity lookup failure: research immediately instead of guessing.
- Second failure of the same class: stop that path and use review only when the
  remaining diagnosis is genuinely ambiguous.
- Never repeat a disproven command with cosmetic argument changes.
- A narrated next step is not progress. Invoke the tool in the same turn.
