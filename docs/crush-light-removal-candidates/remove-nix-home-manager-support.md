# Remove NixOS and Home Manager module support

- **Suggested branch:** `draft/remove-nix-home-manager-support`
- **Risk level:** `low`

## Short summary
Remove documentation and configuration surface that advertises or supports NixOS/Home Manager module usage and any LSP examples/config tied only to that support.

## Why this is a removal candidate
NixOS/Home Manager module support is an explicitly approved config-surface removal target for the light fork.

## Why the chosen risk level applies
The surface appears primarily in documentation/examples and does not anchor a core runtime subsystem in this repository.

## User-visible behavior affected
Users no longer see NixOS/Home Manager module guidance or examples in the repo/docs/schema for crush-light.

## Code entrypoints
- `README.md`
- `schema.json`
- `crush.json`

## Known touch points
- Files/packages: README.md install/config sections, sample config files, release/install docs
- Config: any sample schema/example values that highlight per-language LSP setup or Nix module usage
- Docs/tests: docs/help/manpages if generated elsewhere, workflows that mention schema/docs refresh
- API/server: none
- UI: none primary
- Persistence/data model: none

## Dependencies on kept features
Must preserve general installation guidance, BYOK/OAuth provider setup, and model/session instructions.

## Things that must be preserved while removing it
Keep standard install/config docs for supported light-variant workflows.

## Suggested removal order
Any time; docs-only cleanup that can land early.

## Acceptance criteria for the future implementation PR
- No NixOS/Home Manager module guidance remains.
- No per-language LSP config examples remain in sample config/docs once LSP support is removed.
- Remaining docs still explain supported installation/auth/session flows.

## Open questions / uncertainties
- Whether any release/install automation outside README still references Nix-specific surfaces.
