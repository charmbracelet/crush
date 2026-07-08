---
name: mcp-schema-first
description: Use when configuring, calling, debugging, or reasoning about MCP tools, MCP resources, filesystem MCP, browser MCP, GitHub MCP, docs MCP, or any external tool server.
---

# MCP Schema First

Use MCP tools only after understanding the server's advertised contract.

## Workflow

1. List active MCP servers and tools when the tool shape is not already in context.
2. Read server instructions from initialization output when available.
3. Inspect tool names, required parameters, optional parameters, and approval behavior.
4. Prefer MCP resources for read-only structured data.
5. Authenticate only when the server advertises an auth flow or the user explicitly asks.
6. Do not guess parameter names. If the schema is unavailable, say so and use a safer path.

## Filesystem MCP

- Read before edit.
- Use absolute paths when the server requires them.
- Confirm allowed directories before writing outside the current project.
- Do not use filesystem MCP to bypass project trust, hook policy, or user scope.

## Browser MCP

- Navigate, snapshot, inspect console/network, then act.
- For visual verification, use screenshots and check the actual page state.
- Stop after repeated failed interactions and report what was attempted.

## Remote and Provider MCP

- Treat GitHub, Slack, Telegram, cloud, and similar MCPs as external side effects.
- Separate read-only inspection from mutation.
- Do not create, delete, send, merge, deploy, or publish unless the user clearly asked for that action.
