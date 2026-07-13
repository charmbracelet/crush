---
name: mcp-setup
description: Use when installing, configuring, validating, repairing, calling, or debugging an MCP server, tool, or resource.
---

# MCP Setup and Repair

MCP is a live capability layer. Distinguish saved configuration, connected
servers, and callable tools instead of inferring one from another.

## Existing Capabilities

1. Use `mcp_manage` with `action=status` when current server state is unknown.
2. Use `mcp_tool_search` with an empty query for connected server counts or a
   capability query to search every connected native tool. Follow
   `next_offset` when a broad result has more pages.
3. Select an exact returned name with
   `mcp_tool_search(query="select:<exact_name>")`, then call the activated
   native tool directly. Do not reconstruct its schema from memory.
4. Use MCP resources when a server exposes the needed read-only data.

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
- If package or server identity is uncertain, research before retrying.
- If the same failure class repeats, stop that path and report the exact
  dependency, credential, platform, or transport blocker.
- On Windows, do not assume GNU or PowerShell commands exist inside the
  portable shell. Prefer native tools, portable runtimes, or invoke an
  available host shell explicitly.
