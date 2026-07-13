Inspect or control existing MCP servers without rebuilding their definitions.

- `status` reports one exact server or all configured servers.
- `refresh` restarts one exact server; set `all=true` only for an explicit full reconciliation.
- `enable` and `disable` atomically change the saved state and refresh that server.
- `remove` atomically removes one exact saved server and clears its runtime state.

Use `mcp_add` only to create or replace a server definition. Do not edit
`crush.json` merely to enable, disable, remove, or refresh an MCP server.
