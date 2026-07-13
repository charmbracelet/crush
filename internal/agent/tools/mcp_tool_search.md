Search every tool exposed by connected MCP servers.

Use a capability query to find matching native tool names without loading every
schema into context. Results are ranked pages over the complete live catalog;
use `next_offset` to continue. Then call this tool again with
`select:<exact_native_tool_name>` to load that native tool for the next model
step. Multiple exact names may be comma-separated. Call the selected native
tool directly after selection. Use an empty query to inspect the connected
server inventory.
