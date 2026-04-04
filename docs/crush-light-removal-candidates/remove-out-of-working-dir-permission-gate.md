# Remove out-of-working-dir permission gate

- **Suggested branch:** `draft/remove-out-of-working-dir-permission-gate`
- **Risk level:** `medium`

## Short summary
Remove the special permission behavior that gates tool actions based on derived working-directory paths outside the active workspace.

## Why this is a removal candidate
The out-of-working-dir permission gate is an explicitly approved removal target and should be isolated from broader tool-permission behavior.

## Why the chosen risk level applies
The logic is concentrated in permission/path handling, but many tools feed paths into it. The future PR should remove the working-directory boundary enforcement entirely and can simplify other human-facing permission prompts at the same time, while still preserving stale-write/session protections.

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
Keep protections that prevent the agent from acting on stale session/file state, while allowing the future PR to remove non-essential human-facing permission prompts together with the out-of-working-dir gate.

## Suggested removal order
After LSP removal if the only remaining strict path gating is in permission/file tools; before or independent of fantasy replacement.

## Acceptance criteria for the future implementation PR
- No code path special-cases derived out-of-working-dir gating behavior.
- The working-directory boundary enforcement is fully removed.
- Non-essential human-facing permission prompts may be removed or simplified with it.
- Docs/tests updated to reflect simplified path behavior.
- No regression to stale-write/session protections.

## Open questions / uncertainties
- LSP path checks may disappear with the broader LSP-removal PR; whichever PR lands first should remove the overlapping boundary enforcement cleanly.
