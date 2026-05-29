# Crush Theme System Design

## Overview

This document describes the design for realtime theme editing in Crush,
building on PR #2731 and the existing `quickStyle` infrastructure.

## Architecture

### Palette Abstraction

Themes are defined at the `quickStyleOpts` level — a ~25-color semantic
palette that `quickStyle()` expands into the full `Styles` struct (~50+
fields). This is the right abstraction for editing because:

- 25 named colors are manageable; 50+ style fields are not
- Semantic names (`primary`, `fgBase`, `destructive`) convey intent
- All built-in themes already use this layer
- Changes propagate automatically through `quickStyle()`

### Theme Registry

A simple map-based registry in `internal/ui/styles/themes.go`:

```go
var builtinThemes = map[string]func() quickStyleOpts{
    "charmtone":    charmtoneOpts,
    "gruvbox-dark": gruvboxDarkOpts,
}
```

`LoadTheme(name)` does case-insensitive lookup. Empty string returns the
default (Charmtone Pantera). Unknown names return an error with available
options listed.

### Config Integration

Theme selection lives in `TUIOptions.Theme` as a plain string:

```json
{ "tui": { "theme": "gruvbox-dark" } }
```

No nested object, no partial overrides (yet). Just a name. This keeps
the initial implementation simple and matches what PR #2731 proposes.

Future extension points:
- Partial palette overrides: `{ "theme": { "base": "charmtone", "primary": "#FF0000" } }`
- Custom theme files loaded from disk
- Per-provider theme binding

### Live Preview & Revert

The theme picker dialog previews themes on cursor movement:

1. On first preview, snapshot current styles via `Styles.Clone()`
2. Apply new theme via `applyTheme()` which updates the shared pointer
   and refreshes all cached components
3. On Esc, restore the snapshot
4. On Enter, persist to `crush.json` via `SetConfigField` and clear
   the snapshot

`Clone()` deep-copies `ansi.StyleConfig` (which contains pointer fields)
via JSON round-trip. This is pragmatic but fragile — if glamour adds
non-serializable fields it will break. We accept this tradeoff for now
because:
- It works today
- A reflect-based deep copy is more complex and has its own fragility
- The panic is caught during development, not in production usage

### Cache Invalidation

`refreshStyledComponents()` must hit every component that bakes in style
values at construction time:

- Header logos (rebuilt via `invalidateLogos()`)
- Sidebar logo cache
- Textarea styles
- Completions popup styles
- Attachment chip renderer styles
- Todo spinner style
- Help bar styles
- Chat message render caches (via `ClearItemCaches`)
- Markdown renderer cache (via `InvalidateMarkdownRendererCache`)

Missing any of these causes stale rendering after a theme switch.

## Design Decisions

### Why not a serializable Palette type?

We considered a JSON-serializable `Palette` struct with hex strings for
custom theme editing. Deferred because:
- No custom theme support exists yet
- Built-in themes use Go color constants directly
- Adding serialization before there's a consumer adds complexity
- When we add custom themes, we can introduce `Palette` then

### Why string config instead of structured?

`"theme": "gruvbox-dark"` vs `"theme": { "name": "gruvbox-dark" }`.
The string form is simpler, matches the common case (pick a built-in),
and can be extended later with a union type if needed.

### Why replace ThemeForProvider?

The old `ThemeForProvider(providerID)` tied themes to LLM providers,
which doesn't make sense for user-chosen themes. The new system uses
explicit user choice via config/command palette. Provider-based theming
can be re-added as an optional default if desired.

### Why not file watching for live reload?

File watching adds complexity (fsnotify dependency, debounce, error
handling) for a use case that the command palette already covers well.
External editor users can restart Crush. File watching can come later
if there's demand.

## Implementation Notes

### Field Name Mapping (PR #2731 → current main)

The PR was authored against an older `quickStyleOpts` shape. Current
main renamed several fields. Mapping:

| PR field          | Current main field    |
|-------------------|-----------------------|
| `tertiary`        | `accent`              |
| `fgMuted`         | `fgMoreSubtle`        |
| `fgHalfMuted`     | `fgMostSubtle`        |
| `onAccent`        | *(removed)*           |
| `bgBaseLighter`   | `bgLessVisible`       |
| `bgSubtle`        | `bgLeastVisible`      |
| `bgOverlay`       | `bgMostVisible`       |
| `border`          | `separator`           |
| `borderFocus`     | *(no direct equiv)*   |
| `danger`          | `destructive`         |
| `warningStrong`   | `warningSubtle`       |
| `infoSubtle`      | `infoMoreSubtle`      |
| `infoMuted`       | `infoMostSubtle`      |
| `successSubtle`   | `successMoreSubtle`   |
| `successMuted`    | `successMostSubtle`   |

New fields on main not in PR: `keyword`, `denied`.

### Reconciliation with Existing applyTheme

Current main already has `applyTheme()` and `refreshStyles()` at
`ui.go:3200-3226`. The PR adds duplicate methods. Resolution:
- Keep the existing method names (`applyTheme`, `refreshStyles`)
- Merge the PR's additional invalidation targets into `refreshStyles`
- Ensure `InvalidateMarkdownRendererCache()` is called (present in
  current main, missing from PR)

## Future Work

1. **Custom themes** — Serializable `Palette` type, load from
   `~/.config/crush/themes/` or inline in config
2. **Partial overrides** — Merge user palette on top of a base theme
3. **Light mode** — Auto-detect terminal background, select appropriate
   theme variant
4. **File watching** — Live reload when config changes externally
5. **Visual palette editor** — In-TUI color swatch editor
6. **Per-provider defaults** — Optional fallback to provider-based
   theme when no explicit choice is set
