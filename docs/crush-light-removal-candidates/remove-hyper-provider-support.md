# Remove Hyper provider support

- **Suggested branch:** `draft/remove-hyper-provider-support`
- **Risk level:** `medium`

## Short summary
Remove Hyper-specific provider support, including the custom provider implementation, Hyper login/auth flows, provider update/sync logic, and related docs/tests.

## Why this is a removal candidate
Hyper appears to be a Crush-specific inference API integration that the fork will not use. It is separate from the protected generic OAuth/BYOK support that must remain for other providers.

## Why the chosen risk level applies
Hyper touches provider wiring, login commands, config sync/update paths, OAuth helpers, tests, and some agent error messaging, but it is still a distinct provider-specific subsystem.

## User-visible behavior affected
Users lose Hyper as a provider/login option. Other providers and generic OAuth/BYOK flows remain.

## Code entrypoints
- `internal/agent/hyper/provider.go`
- `internal/oauth/hyper/device.go`
- `internal/config/{hyper.go,provider.go,store.go,load.go}`
- `internal/cmd/login.go`
- `internal/cmd/update_providers.go`

## Known touch points
- Files/packages: `internal/agent/hyper/**`, `internal/oauth/hyper/**`, Hyper-specific branches in config/provider/store/load code, Hyper-specific login/update-provider command paths
- Config: provider type validation, cached Hyper provider data, update-provider source handling
- Docs/tests: Hyper-related tests, embedded `provider.json`, Taskfile/provider update docs, login/help text
- API/server: none primary beyond provider config APIs
- UI: login/help text and any Hyper-specific error or hyperlink copy
- Persistence/data model: provider config values and cached Hyper provider metadata

## Dependencies on kept features
Must preserve generic provider auth, BYOK support, model selection, and non-Hyper providers.

## Things that must be preserved while removing it
Keep the protected OAuth/BYOK infrastructure for the remaining providers intact.

## Suggested removal order
Can happen independently after the main audit-driven removals are underway; before fantasy replacement if reducing provider-specific surface is useful.

## Acceptance criteria for the future implementation PR
- Hyper is no longer a supported provider or login target.
- Hyper-specific provider code, OAuth flow, update-provider plumbing, docs, and tests are removed.
- Remaining providers and generic OAuth/BYOK flows continue to work.

## Open questions / uncertainties
- None currently.
