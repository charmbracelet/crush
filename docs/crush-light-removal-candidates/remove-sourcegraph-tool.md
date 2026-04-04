# Remove Sourcegraph tool

- **Suggested branch:** `draft/remove-sourcegraph-tool`
- **Risk level:** `low`

## Short summary
Remove the standalone `sourcegraph` agent tool in its own future PR rather than folding it into the broader remote-research-tools removal.

## Why this is a removal candidate
The `sourcegraph` tool is an optional external code-search integration with a clear boundary and a much smaller scope than MCP, LSP, or the remote web-research stack.

## Why the chosen risk level applies
The implementation is isolated to a single tool plus registration/docs/tests. Removing it should not disturb local repository tooling or the main agent loop.

## User-visible behavior affected
Users lose the ability to query Sourcegraph from the agent, but local `grep`, `glob`, `view`, and other repository tools remain.

## Code entrypoints
- `internal/agent/tools/sourcegraph.go`
- `internal/agent/coordinator.go`
- `internal/config/config.go`
- `internal/agent/agent_test.go`

## Known touch points
- Files/packages: `internal/agent/tools/sourcegraph.go`, tool registration in coordinator, config defaults that mention `sourcegraph`
- Config: disabled-tools examples/default tool lists in `internal/config/config.go`, schema/help text
- Docs/tests: tool markdown, README examples, agent test fixtures covering `sourcegraph`
- API/server: none direct beyond generic agent execution
- UI: generic tool output rendering only
- Persistence/data model: none

## Dependencies on kept features
Must preserve the rest of the local repository inspection toolset and normal single-agent execution.

## Things that must be preserved while removing it
Keep local search/view tools and primary agent behavior intact.

## Suggested removal order
After the remote-research-tools PR or independently from it; before fantasy replacement if trimming optional tools first is helpful.

## Acceptance criteria for the future implementation PR
- No `sourcegraph` tool remains.
- Tool registration, docs, config examples/defaults, and tests are updated accordingly.
- Local repository inspection tools continue to work unchanged.

## Open questions / uncertainties
- None currently.
