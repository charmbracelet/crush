# Remove out-of-working-dir permission gate

- **Suggested branch:** `draft/remove-out-of-working-dir-permission-gate`
- **Risk level:** `medium`

## Short summary
Remove the special permission behavior that gates tool actions based on derived working-directory paths outside the active workspace.

## Why this is a removal candidate
The out-of-working-dir permission gate is an explicitly approved removal target and should be isolated from broader tool-permission behavior.

## Why the chosen risk level applies
The logic is concentrated in permission/path handling, but many tools feed paths into it. The future PR must avoid weakening unrelated permission prompts or stale-write/session protections.

## User-visible behavior affected
Users no longer see special permission behavior keyed to out-of-working-dir path derivation; permission prompts should simplify accordingly.

## Code entrypoints
- `internal/permission/permission.go`
- `internal/filepathext/**`
- `internal/agent/tools/safe.go`
- `internal/lsp/manager.go`

## Known touch points
- Files/packages: permission service, safe path helpers, tools that compute request paths, LSP start path guard, file tracker/path helpers
- Config: permissions.allowed_tools docs/help if they mention working-dir boundaries
- Docs/tests: permission tests, README/help text, agent instructions about working-dir restrictions
- API/server: permission grant endpoints if payloads/documentation mention this gate
- UI: permission prompt copy
- Persistence/data model: none

## Dependencies on kept features
Must preserve normal permission prompts, session-scoped approvals, and stale-write/file-history safeguards.

## Things that must be preserved while removing it
Keep general permission approval flow and file modification-time protections while removing only the out-of-working-dir gate behavior.

## Suggested removal order
After LSP removal if the only remaining strict path gating is in permission/file tools; before or independent of fantasy replacement.

## Acceptance criteria for the future implementation PR
- No code path special-cases derived out-of-working-dir gating behavior.
- Permission prompts still work for retained tools.
- Docs/tests updated to reflect simplified path behavior.
- No regression to stale-write/session protections.

## Open questions / uncertainties
- Whether LSP start path checks should be removed here or in the LSP-removal PR to avoid overlap.
