---
name: mcp-schema-first
description: Use when configuring, calling, debugging, or reasoning about MCP tools, MCP resources, filesystem MCP, browser MCP, GitHub MCP, docs MCP, or any external tool server.
---

# MCP Schema First

Use MCP tools deliberately. MCP is a capability layer, not the default way to inspect local files.

## Tool Selection Order

1. Use native Crush tools for local repository inspection when they fit: `view`, `grep`, `glob`, `ls`, `lsp_diagnostics`, `lsp_references`, and `crush_info`.
2. Use Bash for host/runtime facts, package manager commands, git commands, process/service checks, disk usage, and bounded command output.
3. Use native `web_search`/`web_fetch` for current web facts and explicit URL follow-up.
4. Use specialized MCP servers when the task needs that integration: Context7 for official docs, GitHub Grep for public code search, Playwright for browser state, memory for durable facts.
5. Use filesystem MCP as a precise-path fallback for exact reads/lists/writes or when native tools cannot access the needed file shape.

Do not use filesystem MCP as a broad discovery engine from `/`. For broad host inspection, use a bounded shell command with `timeout`, `head`, `maxdepth`, a specific path, or an explicit command that returns a finite snapshot.

## MCP Workflow

1. List active MCP servers and tools when the tool shape is not already in context.
2. Read server instructions from initialization output when available.
3. Inspect tool names, required parameters, optional parameters, and approval behavior.
4. Prefer MCP resources for read-only structured data when a resource exists.
5. Authenticate only when the server advertises an auth flow or the user explicitly asks.
6. Do not guess parameter names. If the schema is unavailable, say so and use a safer native or shell path.

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
