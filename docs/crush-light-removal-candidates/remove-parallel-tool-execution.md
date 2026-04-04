# Remove parallel tool execution

- **Suggested branch:** `draft/remove-parallel-tool-execution`
- **Risk level:** `high`

## Short summary
Remove agent-facing parallel tool execution semantics and convert or delete tool definitions that currently rely on `fantasy.NewParallelAgentTool`.

## Why this is a removal candidate
Parallel tool execution is an explicitly approved removal target and affects a recognizable slice of tool registration and tests.

## Why the chosen risk level applies
Parallel execution is a cross-cutting behavior rather than a single package. The work is bounded by tool construction, tests/fixtures, and any concurrency assumptions, but it touches many tools and may intersect fantasy replacement planning.

## User-visible behavior affected
Agents stop advertising or using parallel tool calls; affected tools either become sequential or disappear as part of other removals.

## Code entrypoints
- `internal/agent/agent_tool.go`
- `internal/agent/agentic_fetch_tool.go`
- `internal/agent/tools/{fetch,download,web_fetch,web_search,sourcegraph,list_mcp_resources,read_mcp_resource}.go`
- `internal/agent/agent_test.go`

## Known touch points
- Files/packages: all `fantasy.NewParallelAgentTool` call sites, coordinator tests, agent fixtures containing `parallel_tool_calls`
- Config: none primary, but tool availability docs/help may mention concurrency
- Docs/tests: tool descriptions, regression fixtures, README/product copy if it advertises parallelism
- API/server: none primary
- UI: tool-progress rendering may assume concurrent activity
- Persistence/data model: none

## Dependencies on kept features
Must preserve core tool availability, single-agent execution, and model switching.

## Things that must be preserved while removing it
Keep tools functional sequentially where the feature is retained, and avoid entangling this work with fantasy replacement more than necessary. The retained providers/tools are expected to keep working once execution becomes sequential-only.

## Suggested removal order
After removals that delete whole parallel-only tools (MCP/sub-agent/remote research) so fewer call sites remain; before fantasy replacement if it simplifies the swap.

## Acceptance criteria for the future implementation PR
- No retained tool uses parallel execution semantics.
- Parallel-tool-call fixtures/tests are removed or updated.
- User-facing docs no longer describe concurrent tool calls.
- Retained tools still work sequentially.

## Open questions / uncertainties
- None currently.
