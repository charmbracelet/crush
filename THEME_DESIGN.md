# Crush Theme System Design

## Status

**Phase 1 (shipped):** Built-in theme switching with live preview.
**Phase 2 (shipped):** Realtime editing of existing theme palettes from within Crush.
**Phase 3 (next):** Polish editor UX, add named custom themes, and support sharing theme files.

---

## Phase 1: Theme Switching (Shipped)

### What Exists

- **Theme registry** (`internal/ui/styles/themes.go`): `LoadTheme(name)`,
  `BuiltinThemeNames()`, map-based lookup with case-insensitive matching.
- **Two built-in themes**: `charmtone` (default), `gruvbox-dark`.
- **Config field**: `TUIOptions.Theme` string in `crush.json`:
  ```json
  { "tui": { "theme": "gruvbox-dark" } }
  ```
- **Command palette dialog** (`internal/ui/dialog/theme.go`): "Switch Theme"
  entry opens a filterable list. Cursor movement previews live; Enter
  persists to config; Esc reverts.
- **Live apply pipeline**: `applyTheme()` → `InvalidateMarkdownRendererCache()`
  → `refreshStyles()` hits all cached components.
- **All CLI paths updated**: `root.go`, `run.go`, `session.go`, `app.go`
  use config-based theme instead of provider-based selection.

### Architecture

Themes are defined at the `quickStyleOpts` level — a ~25-color semantic
palette that `quickStyle()` expands into the full `Styles` struct (~50+
fields). This is the right abstraction because:

- 25 named colors are manageable; 50+ style fields are not
- Semantic names (`primary`, `fgBase`, `destructive`) convey intent
- All built-in themes already use this layer
- Changes propagate automatically through `quickStyle()`

### Cache Invalidation

`refreshStyles()` must hit every component that bakes in style values at
construction time:

- Header logos (via `header.refresh()`)
- Sidebar logo cache
- Textarea styles
- Completions popup styles
- Attachment chip renderer styles (5-arg `SetStyles` including skill)
- Todo spinner style
- Help bar styles
- Chat message render caches (via `InvalidateRenderCaches`)
- Markdown renderer cache (via `InvalidateMarkdownRendererCache`)

Missing any of these causes stale rendering after a theme switch.

### Design Decisions

| Decision | Rationale |
|----------|-----------|
| String config (`"theme": "name"`) | Simplest form for built-in selection; extensible later |
| Replace `ThemeForProvider` | User choice > provider binding; can re-add as fallback |
| JSON round-trip for `Clone()` | Pragmatic deep copy of `ansi.StyleConfig` pointers; panic acceptable at dev time |
| No file watching yet | Command palette covers the use case; fsnotify adds complexity |
| No serializable `Palette` type yet | No custom theme consumer exists; add when needed |

---

## Phase 2: Realtime Theme Editing (Next)

### Goal

Edit an existing theme's colors from within Crush and see changes
instantly, without restarting or editing config files by hand.

### Approach: Serializable Palette Type

Introduce a `Palette` struct that mirrors `quickStyleOpts` but uses hex
strings and is fully JSON-serializable. This enables:

1. Exporting a built-in theme's palette as editable JSON
2. Applying partial overrides on top of a base theme
3. Persisting custom palettes to config or disk
4. Building an in-TUI color editor that manipulates `Palette` values

```go
// Palette is a JSON-serializable theme palette. Each field maps to a
// quickStyleOpts color. Empty strings mean "inherit from base".
type Palette struct {
    Primary   string `json:"primary,omitempty"`
    Secondary string `json:"secondary,omitempty"`
    Accent    string `json:"accent,omitempty"`
    Keyword   string `json:"keyword,omitempty"`

    FgBase       string `json:"fg_base,omitempty"`
    FgSubtle     string `json:"fg_subtle,omitempty"`
    FgMoreSubtle string `json:"fg_more_subtle,omitempty"`
    FgMostSubtle string `json:"fg_most_subtle,omitempty"`

    BgBase         string `json:"bg_base,omitempty"`
    BgMostVisible  string `json:"bg_most_visible,omitempty"`
    BgLessVisible  string `json:"bg_less_visible,omitempty"`
    BgLeastVisible string `json:"bg_least_visible,omitempty"`

    OnPrimary string `json:"on_primary,omitempty"`
    Separator string `json:"separator,omitempty"`

    Destructive       string `json:"destructive,omitempty"`
    Error             string `json:"error,omitempty"`
    Warning           string `json:"warning,omitempty"`
    WarningSubtle     string `json:"warning_subtle,omitempty"`
    Denied            string `json:"denied,omitempty"`
    Busy              string `json:"busy,omitempty"`
    Info              string `json:"info,omitempty"`
    InfoMoreSubtle    string `json:"info_more_subtle,omitempty"`
    InfoMostSubtle    string `json:"info_most_subtle,omitempty"`
    Success           string `json:"success,omitempty"`
    SuccessMoreSubtle string `json:"success_more_subtle,omitempty"`
    SuccessMostSubtle string `json:"success_most_subtle,omitempty"`
}
```

### Conversion Functions

```go
// PaletteFromOpts extracts a Palette from quickStyleOpts, converting
// each color.Color to its "#rrggbb" hex representation.
func PaletteFromOpts(o quickStyleOpts) Palette

// ToQuickStyleOpts converts a Palette back to quickStyleOpts, parsing
// hex strings via lipgloss.Color(). Empty fields fall back to the
// provided base palette.
func (p Palette) ToQuickStyleOpts(base quickStyleOpts) quickStyleOpts

// Validate checks that all non-empty hex strings are valid colors.
func (p Palette) Validate() error
```

### Config Extension

Extend `TUIOptions.Theme` to accept either a string (built-in name) or
an object (custom palette with optional base):

```jsonc
// Built-in (existing)
{ "tui": { "theme": "gruvbox-dark" } }

// Custom override on top of a base
{ "tui": { "theme": { "base": "charmtone", "primary": "#FF6B6B" } } }

// Fully custom (no base)
{ "tui": { "theme": { "primary": "#FF6B6B", "bg_base": "#1A1A2E", ... } } }
```

Implementation: use `json.RawMessage` + custom unmarshal to detect
string vs object. When object, parse `Palette`, resolve base via
`LoadTheme`, merge with `ToQuickStyleOpts(base)`.

### In-TUI Editor Dialog

A new dialog (`internal/ui/dialog/theme_editor.go`) accessible via
command palette "Edit Theme":

1. Load current theme's palette via `PaletteFromOpts`
2. Display a scrollable list of color slots, each showing:
   - Semantic name (e.g., "Primary")
   - Current hex value
   - Color swatch (rendered block character with foreground color)
3. Navigation: up/down to select slot
4. Editing: type hex value directly, or press Enter to open a
   charmtone color picker sub-dialog
5. Live preview: on every keystroke/change, convert palette to
   `quickStyleOpts` via `ToQuickStyleOpts(currentBase)`, run
   `quickStyle()`, call `applyTheme()`
6. Save: persist to config as palette object; Esc reverts to snapshot

### Implementation Order

1. **`Palette` type + conversion functions** — pure data, testable
   independently. Add `PaletteFromOpts`, `ToQuickStyleOpts`, `Validate`.
2. **Config extension** — custom unmarshal for `TUIOptions.Theme` to
   handle string | object. Wire into `LoadThemeStyles`.
3. **Export built-in palettes** — add `ThemePalette(name) (Palette, error)`
   so the editor can load any built-in theme as an editable starting point.
4. **Editor dialog** — TUI component with color slot list, hex input,
   live preview via existing `applyTheme` pipeline.
5. **Persistence** — save edited palette to config; support "Save As"
   to create named custom themes.

### Open Questions

- **Where to store custom themes?** Inline in `crush.json` vs separate
  files in `~/.config/crush/themes/`. Separate files are cleaner for
  sharing but add filesystem management. Start inline, extract later.
- **Color picker UX?** Hex input is universal but slow. A charmtone
  palette browser would be nice for Charm-branded themes. Could support
  both: type hex or browse charmtone.
- **Undo history?** Single-level revert (Esc) matches the theme switcher.
  Multi-step undo for iterative editing would be better but more complex.
  Start single-level.
- **Validation feedback?** Show invalid hex inline (red border) vs
  blocking save. Inline is better for exploration.

---

## Appendix: Field Name Reference

Current `quickStyleOpts` fields (as of main `a4181d6d`):

| Field | Semantic Role |
|-------|---------------|
| `primary` | Brand/accent color |
| `secondary` | Secondary brand |
| `accent` | Tertiary accent |
| `keyword` | Syntax keyword highlight |
| `fgBase` | Default foreground |
| `fgSubtle` | Slightly muted foreground |
| `fgMoreSubtle` | More muted foreground |
| `fgMostSubtle` | Most muted foreground |
| `bgBase` | Default background |
| `bgMostVisible` | Most visible background layer |
| `bgLessVisible` | Less visible background layer |
| `bgLeastVisible` | Least visible background layer |
| `onPrimary` | Foreground on primary backgrounds |
| `separator` | Dividers, borders, rules |
| `destructive` | Destructive action color |
| `error` | Error state |
| `warning` | Warning state |
| `warningSubtle` | Subtle warning |
| `denied` | Permission denied |
| `busy` | Loading/processing |
| `info` | Informational |
| `infoMoreSubtle` | Subtle info |
| `infoMostSubtle` | Most subtle info |
| `success` | Success state |
| `successMoreSubtle` | Subtle success |
| `successMostSubtle` | Most subtle success |
