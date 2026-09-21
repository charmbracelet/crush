Search and load MCP tools that are hidden from your tool list.

MCP servers are connected but their tool schemas are kept out of context to
save tokens, so they are not callable until you load them. Each call returns
the best matches and makes them available on your very next step: continue
the task immediately instead of asking the user to proceed.

Query forms:

- A plain description of what you need ("create a pull request", "list s3
  buckets"). Name matches outrank description matches.
- `server:<name>` loads every tool from one server.
- `select:<tool>,<tool>` loads exact tools, with or without the
  `mcp_<server>_` prefix.

An empty query lists what exists. Loaded tools stay available for the rest of
the session, up to {{ .MaxLoaded }} at a time; past that the least recently
used ones are dropped again, so load what you need rather than everything.
