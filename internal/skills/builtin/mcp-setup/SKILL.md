---
name: mcp-setup
description: Use when installing, configuring, validating, repairing, calling, or debugging any MCP server, tool, or resource.
---

# MCP Setup and Repair

Use MCP tools deliberately. MCP is a capability layer, not the default way to inspect local files.

## Tool Selection Order

1. Use native re.code tools for local repository inspection when they fit: `view`, `grep`, `glob`, `ls`, `lsp_diagnostics`, `lsp_references`, and `recode_info`.
2. Use Bash for host/runtime facts, package manager commands, git commands, process/service checks, disk usage, and bounded command output.
3. Use native `web_search`/`web_fetch` for current web facts and explicit URL follow-up.
4. Use specialized MCP servers when the task needs that integration: Context7 for official docs, GitHub Grep for public code search, Playwright for browser state, memory for durable facts.
5. Use filesystem MCP as a precise-path fallback for exact reads/lists/writes or when native tools cannot access the needed file shape.

Do not use filesystem MCP as a broad discovery engine from `/`. For broad host inspection, use a bounded shell command with `timeout`, `head`, `maxdepth`, a specific path, or an explicit command that returns a finite snapshot.

## MCP Workflow

1. Run `recode_info` first and inventory configured, initialized, and failed
   servers separately. Its `[mcp]` and `[mcp_config]` sections are the
   authoritative runtime and saved-configuration view. Do not reopen or parse
   `crush.json` for an MCP task after those sections are available. A connected
   HTTP server is not an npm package that must be installed again.
2. List active MCP servers and tools when the tool shape is not already in context.
3. Read server instructions from initialization output when available.
4. Inspect tool names, required parameters, optional parameters, and approval behavior.
5. Prefer MCP resources for read-only structured data when a resource exists.
6. Authenticate only when the server advertises an auth flow or the user explicitly asks.
7. Prefer verifying an unfamiliar package or URL against official
   documentation or its package registry. Documentation lookup supports the
   decision but is not a prerequisite for proposing an exact configuration to
   the user. Do not infer package names from MCP display names.
8. Do not guess parameter names. If the schema is unavailable, say so and use a safer native or shell path.
9. Call `mcp_add` only when the exact user-requested server is missing or its
   saved configuration is invalid. Set exactly one of `stdio`, `http`, or
   `sse`; their fields cannot be mixed. Pass `source_url` when useful, but do
   not block installation because a documentation site cannot be fetched.
   Treat dependency or credential errors as exact blockers. Server names are
   exact; never substitute a related integration. If correcting a saved
   configuration, set `replace=true`. Use `mcp_refresh` for an already
   configured server or explicit bulk reconciliation. A failed `mcp_add`
   candidate is rolled back and ends the turn; do not continue to another
   server.

Do not delegate MCP configuration discovery to a sub-agent. The delegated
agent has the same runtime state and tool-schema cost; use the structured
`recode_info` -> primary source -> `mcp_add` path directly.

In MCP configuration, the transport key is `type`; valid values are `stdio`,
`sse`, and `http`. Never use `lsp_diagnostics` as MCP validation. Report
"configured" separately from "connected".

- `stdio` uses `command`, `args`, and optional `env`; it must not contain `url`
  or HTTP headers.
- `http` and `sse` use `url` and optional headers; they must not contain
  `command`, `args`, or process `env`.

After the first package-identity failure, verify the official server identity,
transport, and host requirements before one corrected install attempt. If that
attempt fails with the same error class, stop changing package names.

## Safe Command Reference

| Goal | Preferred action |
| --- | --- |
| Current directory | `pwd` |
| Active re.code config and MCP state | `recode_info` |
| Exact local file contents | native `view` with the loaded absolute path |
| Verify a known npm package exists | `npm view <verified-package> version` |
| Find an unknown/current package name | native `web_search`, then official docs or registry |
| Validate JSON portably when Node exists | `node -e "JSON.parse(require('fs').readFileSync(process.argv[1],'utf8')); console.log('valid')" "<path>"` |

Do not use invented commands such as `npx search-*`. Do not use `npx` as a
package search engine. On Windows, do not append GNU filters such as `head`
unless their availability was verified; use bounded native tool output or a
portable runtime instead.

## Native Web And Code Search

- `web_search` and `web_fetch` are native tools, not MCP tools. Use them first for current web facts and explicit URL follow-up.
- Use Sourcegraph or GitHub Grep for public code search. Do not use general web search for symbol lookup when a code-search tool is available.
- Use external MCP search/fetch only when native web tools are unavailable or the MCP provider has a clear advantage for the task.

## Filesystem MCP

- Use for exact paths, small directory listings, and structured file operations.
- Prefer native `view`/`grep`/`glob`/`ls` for repository work when available.
- Prefer bounded Bash for whole-host searches, cache size checks, process inspection, and other system facts.
- Treat filesystem MCP listings as shape evidence only. Use measured shell output for sizes, free space, process state, service state, and command success.
- Read before edit. Use absolute paths when the server requires them.
- Confirm allowed directories before writing outside the current project.
- Do not use filesystem MCP to bypass project trust, hook policy, or user scope.
- If a filesystem MCP call times out or returns EOF, do not retry the same broad request. Narrow the path or switch to a bounded native/shell command.

## Browser MCP

- Navigate, snapshot, inspect console/network, then act.
- For visual verification, use screenshots and check the actual page state.
- Stop after repeated failed interactions and report what was attempted.

## Remote and Provider MCP

- Treat GitHub, Slack, Telegram, cloud, and similar MCPs as external side effects.
- Separate read-only inspection from mutation.
- Do not create, delete, send, merge, deploy, or publish unless the user clearly asked for that action.
