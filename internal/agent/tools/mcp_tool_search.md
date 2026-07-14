Fetch complete descriptions and input schemas for deferred MCP tools.

Every available deferred tool appears by exact name in the catalog below. A
name alone cannot be called because its input schema is not loaded yet. Search
once, then call a returned native tool normally on the next model step. Loaded
tools remain callable for the rest of the current user turn and are deferred
again for the next user turn.

Query forms:

- `select:mcp_github_get_me` loads one exact name.
- `select:mcp_memory_search_nodes,mcp_memory_open_nodes` loads several names.
- `github branch` searches names and descriptions and loads up to five matches.
- `github branch` with `max_results: 10` deliberately widens that search.

Exact `select:` queries are not capped. Prefer them when the visible catalog
already identifies the required tool.

This searches tools, not repositories, files, web pages, memories, or other
data. After loading a tool, call it to work with that data. Use an empty query
to print the complete name catalog. If no suitable tool is found after one
meaningfully different retry, load the `mcp-setup` skill instead of looping.
