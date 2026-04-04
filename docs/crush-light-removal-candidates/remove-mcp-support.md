# Remove MCP support

- **Suggested branch:** `draft/remove-mcp-support`
- **Risk level:** `medium`

## Short summary
Remove Model Context Protocol support, including static MCP tools, dynamic MCP tool loading, MCP endpoints, MCP config keys, Docker MCP helpers, and MCP docs.

## Why this is a removal candidate
MCP is an explicitly approved removal target and is a large optional extensibility surface that adds tool loading, server lifecycle, config, UI, API, and permission complexity.

## Why the chosen risk level applies
The boundary is broad but recognizable: config, tool adapters, backend refresh/read endpoints, UI prompts, and documentation all pivot on MCP. Core sessions, OAuth, and the HTTP server can remain intact if only MCP-specific surfaces are removed.

## User-visible behavior affected
Users lose MCP server configuration, MCP-backed tools, MCP resource/prompt discovery, Docker MCP shortcuts, and MCP-related UI/server routes.

## Code entrypoints
- `internal/agent/tools/mcp-tools.go`
- `internal/agent/tools/mcp/`
- `internal/backend/mcp.go`
- `internal/server/server.go`
- `internal/config/config.go`
- `internal/ui/chat/docker_mcp.go`

## Known touch points
- Files/packages: internal/agent/tools/mcp-tools.go, internal/agent/tools/mcp/**, internal/backend/mcp.go, internal/config/docker_mcp.go, internal/ui/chat/docker_mcp.go, internal/workspace/**
- Config: Config.MCP / MCPConfig / schema.json / crush.json / README MCP docs
- Docs/tests: README.md, AGENTS.md, tool descriptions, MCP tests/fixtures, workflow/docs references
- API/server: /v1/workspaces/{id}/mcp/* endpoints in internal/server/server.go and related proto types
- UI: MCP enable/disable affordances and Docker MCP chat prompts
- Persistence/data model: none primary, but workspace/config state references must be cleaned up

## Dependencies on kept features
Must preserve the internal HTTP server, Swagger generation, working sessions, and generic tool execution for non-MCP tools.

## Things that must be preserved while removing it
Keep server startup, non-MCP tools, OAuth/BYOK provider auth, model switching, and session persistence untouched.

## Suggested removal order
After analytics removal is safe; before fantasy replacement; before or alongside sub-agent/parallel cleanup if MCP tool wrappers still rely on fantasy interfaces.

## Acceptance criteria for the future implementation PR
- No MCP config keys, schema entries, or sample config remain.
- No MCP endpoints remain in backend/server/swagger for removed behavior.
- No MCP tool descriptions, dynamic MCP tool loading, or Docker MCP helpers remain.
- Build/tests/docs updated so non-MCP workflows continue to work.

## Open questions / uncertainties
- Whether any existing users rely on Docker MCP onboarding copy that should be replaced with a lighter extension story.
- Whether MCP refresh/read APIs share proto messages with kept features that need surgical retention.
