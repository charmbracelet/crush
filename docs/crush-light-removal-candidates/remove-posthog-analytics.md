# Remove PostHog analytics

- **Suggested branch:** `draft/remove-posthog-analytics`
- **Risk level:** `low`

## Short summary
Remove PostHog metrics, machine-id telemetry support, event helpers, config gating, and analytics docs/code paths.

## Why this is a removal candidate
PostHog is an explicitly approved removal target and is a well-bounded third-party integration.

## Why the chosen risk level applies
Telemetry lives in a distinct internal/event package and a small number of call sites. The main care point is deleting references cleanly without disturbing startup/session flows.

## User-visible behavior affected
Users lose anonymous metrics/error reporting; no core coding or session behavior should change.

## Code entrypoints
- `internal/event/event.go`
- `internal/event/identifier.go`
- `internal/app/app.go`
- `internal/cmd/root.go`

## Known touch points
- Files/packages: internal/event/** plus callers across app/session/cmd
- Config: options.disable_metrics, schema.json, README/help text
- Docs/tests: event tests, dependency docs, go.mod/go.sum
- API/server: none primary
- UI: any metrics/privacy copy
- Persistence/data model: none

## Dependencies on kept features
Must preserve startup, shutdown, session creation/deletion, and logging without the analytics hooks.

## Things that must be preserved while removing it
Keep crash/error logging, session state, user-facing auth/model features, and any valuable local-only diagnostics or metrics for power users untouched.

## Suggested removal order
First or early; low-risk dependency cleanup before harder removals.

## Acceptance criteria for the future implementation PR
- No PostHog or machineid dependency remains.
- No telemetry init/send/flush hooks remain.
- Disable-metrics config/docs removed or rewritten if obsolete.
- Local-only diagnostics/metrics that are still useful to users remain available.
- No behavior regressions outside metrics removal.

## Open questions / uncertainties
- Distinguish clearly between remote telemetry that must go and local-only diagnostics that still provide user value.
