Read a resource by URI from an MCP server; returns text content.

<when_to_use>
Use this tool to fetch a specific resource URI exposed by an MCP server.
</when_to_use>

<usage>
- Provide MCP server name and resource URI
- Returns resource text content
</usage>

<parameters>
- mcp_name: The MCP server name
- uri: The resource URI to read
</parameters>

<notes>
- Returns text content by concatenating resource parts
- Binary resources are returned as UTF-8 text when possible
</notes>
