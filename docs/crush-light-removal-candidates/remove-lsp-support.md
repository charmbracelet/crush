# Remove LSP support

- **Suggested branch:** `draft/remove-lsp-support`
- **Risk level:** `high`

## Short summary
Remove LSP manager/runtime support, LSP agent tools, LSP server endpoints, auto-LSP behavior, and configurable per-language LSP server settings.

## Why this is a removal candidate
LSP support is an explicitly approved removal target and owns both agent tool surface and a sizable config/runtime subsystem.

## Why the chosen risk level applies
LSP threads through tools, UI affordances, config schema, auto-start logic, view/edit helper behavior, and server endpoints. It is still a bounded subsystem, but many user-visible affordances reference it.

## User-visible behavior affected
Users lose diagnostics/references/restart tools, automatic or configured language-server startup, and LSP-backed code intelligence in the UI/API.

## Code entrypoints
- `internal/lsp/manager.go`
- `internal/agent/tools/diagnostics.go`
- `internal/agent/tools/references.go`
- `internal/agent/tools/lsp_restart.go`
- `internal/server/server.go`

## Known touch points
- Files/packages: internal/lsp/**, internal/agent/tools/{diagnostics,references,lsp_restart}.go, internal/app/lsp_events.go, internal/backend/**, internal/ui/** references to LSP state
- Config: Config.LSP / LSPConfig / Options.AutoLSP / schema.json / crush.json / README LSP docs
- Docs/tests: README.md, AGENTS.md, LSP tests, golden fixtures, workflow/schema references
- API/server: /v1/workspaces/{id}/lsps* endpoints and Swagger/proto surface
- UI: status displays, restart actions, completions, or view/edit affordances that assume LSP state
- Persistence/data model: no dedicated table, but session flows and server payloads expose LSP state

## Dependencies on kept features
Must preserve sessions, file history, stale-write protection, model switching, server/API shell, and generic file tools.

## Things that must be preserved while removing it
Keep view/edit/file operations functional without LSP, and keep server/Swagger healthy after removing LSP-only routes and types.

## Suggested removal order
After MCP or independent from it; before fantasy replacement; should happen before trimming permission behavior that only exists for LSP paths.

## Acceptance criteria for the future implementation PR
- No LSP config or auto-LSP options remain.
- No LSP tools or endpoints remain.
- UI and docs no longer advertise LSP-backed context.
- Core editing and viewing still work without LSP.

## Open questions / uncertainties
- Whether any view/edit code paths need fallback adjustments when hover/diagnostic enrichment disappears.
- Whether removing LSP changes test expectations around file intelligence or prompts.
