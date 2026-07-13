Reload re.code configuration from disk and restart configured MCP clients.

Use this after a validated MCP configuration change. Set `name` to the exact
server that changed; a successful named result is runtime evidence that the
server connected. Set `all=true` only for an intentional full reconciliation.
The result reports connected, disabled, removed, and failed clients. A file
write alone is not successful MCP setup. Do not invent shell reload commands.
