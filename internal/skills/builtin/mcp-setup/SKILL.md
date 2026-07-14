---
name: mcp-setup
description: Use when installing, configuring, validating, repairing, calling, or debugging an MCP server, tool, or resource.
---

# MCP Setup and Repair

MCP is a live capability layer. Distinguish saved configuration, connected
servers, and callable tools instead of inferring one from another.

## Existing Capabilities

1. Use `mcp_manage` with `action=status` when current server state is unknown.
2. Every connected deferred tool is advertised by exact name through
   `mcp_tool_search`. When the needed name is clear, load it with
   `select:<exact_name>`. Otherwise search once with capability words. This
   searches the tool catalog, not data such as repositories or files.
3. Call the returned native tool directly. Its complete schema remains
   available for the rest of the current user turn; do not search the catalog
   again to query data or reconstruct its schema from memory.
4. When an external object's exact coordinates are known, prefer a direct
   get/read capability. Use broad data search only when discovery is required.
5. Use MCP resources when a server exposes the needed read-only data.

Do not reinstall or web-search a capability that the live catalog already
reports. Do not invoke MCP tools through the shell.

## Configuration Changes

1. Use `recode_info` only when the saved definition or canonical write target
   is required.
2. Use `mcp_manage` to refresh, enable, disable, remove, or inspect an existing
   server.
3. Use `mcp_add` only to create or replace one exact definition. Set one
   transport: `stdio`, `http`, or `sse`.
4. Verify unfamiliar package names, URLs, authentication requirements, and
   transport syntax with official documentation or an authoritative registry
   before installation. Never infer a package name from a display name.
5. After mutation, verify both saved state and runtime connection, then make one
   representative native tool call when possible.

For `stdio`, use `command`, `args`, and optional `env`. For `http` or `sse`, use
`url` and optional headers. Never mix process and URL fields. Preserve unknown
and unrelated configuration fields.

## Recovery

- On the first failure, use the exact error to correct one assumption.
- Treat empty or partial results as the current server and credentials' visible
  scope, not proof that an external object does not exist.
- If package or server identity is uncertain, research before retrying.
- If the same failure class repeats, stop that path and report the exact
  dependency, credential, platform, or transport blocker.
- On Windows, do not assume GNU or PowerShell commands exist inside the
  portable shell. Prefer native tools, portable runtimes, or invoke an
  available host shell explicitly.
