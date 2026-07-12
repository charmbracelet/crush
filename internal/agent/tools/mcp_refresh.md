Reload re.code configuration from disk and restart configured MCP clients.

Use this after a validated MCP configuration change. Set `name` to refresh one
server, or omit it to reconcile and restart all configured MCP servers. The
result reports connected, disabled, removed, and failed clients. Do not invent
shell commands to reload MCP clients.
