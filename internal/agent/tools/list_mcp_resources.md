List available resource URIs and resource templates from an MCP server by name; use before read_mcp_resource.

Resource templates have URI templates (e.g. `cairn://run/{id}/a2ui`) that can be expanded to concrete URIs.
To read a template resource, substitute the `{id}` parameter and pass the resulting URI to read_mcp_resource.