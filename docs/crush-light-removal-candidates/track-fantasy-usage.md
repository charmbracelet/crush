# Track charm.land/fantasy usage for later replacement

- **Suggested branch:** `draft/track-fantasy-usage`
- **Risk level:** `high`

## Short summary
Create a dedicated tracker PR that inventories every `charm.land/fantasy` usage/dependency point without attempting the replacement yet.

## Why this is a removal candidate
The problem statement requires exactly one dedicated draft PR for fantasy tracking and explicitly forbids doing the replacement in this task.

## Why the chosen risk level applies
Fantasy touches agent construction, provider wiring, tool interfaces, message conversion, and many tests. The tracking PR must stay inventory-only to avoid accidental refactors.

## User-visible behavior affected
No user-visible behavior change; this is a planning/tracking-only work item.

## Code entrypoints
- `go.mod`
- `Taskfile.yaml`
- `internal/agent/**/*.go`
- `internal/message/content.go`
- `internal/app/app.go`

## Known touch points
- Files/packages: all direct imports/usages plus provider-specific fantasy packages
- Config: provider/model wiring that depends on fantasy abstractions
- Docs/tests: AGENTS.md, dependency-update tasks, tests importing fantasy
- API/server: indirect via agent runtime only
- UI: indirect via model/tool/result rendering only
- Persistence/data model: message conversion paths that adapt fantasy message/tool structures

## Dependencies on kept features
Must preserve all current functionality; this future PR is inventory-only.

## Things that must be preserved while removing it
Keep provider auth, model switching, sessions, tools, server/API, and all existing runtime behavior.

## Suggested removal order
Tracking PR should exist immediately and remain open while other simplification PRs reduce future replacement scope.

## Acceptance criteria for the future implementation PR
- Draft PR exists only as a tracker.
- Fantasy usage references are collected in the PR description and audit docs.
- No replacement/refactor code lands in the tracking PR.

## Open questions / uncertainties
- How to post follow-up usage comments if tool support for PR comments remains unavailable in this environment.
