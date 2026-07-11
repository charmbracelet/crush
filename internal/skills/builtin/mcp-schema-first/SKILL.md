---
name: mcp-schema-first
description: Use when configuring, calling, debugging, or reasoning about MCP tools, MCP resources, filesystem MCP, browser MCP, GitHub MCP, docs MCP, or any external tool server.
---

# MCP Schema First

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
   servers separately. A connected HTTP server is not an npm package that must
   be installed again.
2. List active MCP servers and tools when the tool shape is not already in context.
3. Read server instructions from initialization output when available.
4. Inspect tool names, required parameters, optional parameters, and approval behavior.
5. Prefer MCP resources for read-only structured data when a resource exists.
6. Authenticate only when the server advertises an auth flow or the user explicitly asks.
7. Verify the exact package or URL against official documentation or the
   package registry. Do not infer package names from MCP display names. If the
   first exact lookup fails, use native `web_search` immediately; do not guess
   another package name from memory.
8. Do not guess parameter names. If the schema is unavailable, say so and use a safer native or shell path.

In `crush.json`, the transport key is `type`; valid values are `stdio`, `sse`,
and `http`. Parse before and after a structured edit, preserve unrelated
entries, and never use `lsp_diagnostics` as MCP validation. Reload clients and
report "configured" separately from "initialized".

After two failures with the same error class, stop changing package names.
Re-check the premise with native web search, the official server identity,
transport, and host requirements before running another install command.

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
