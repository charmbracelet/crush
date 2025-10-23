# Project Assessment

## Build & Test Status
- `go test ./...` currently fails. Explorer and layout packages do not compile, and `internal/fsext/lookup_test.go` contains a path assumption that fails on Windows because `C:\invalid\path\that\does\not\exist` is present.
- `go.mod` targets Go `1.25.0`, which is not yet released, so the project cannot build on any current toolchain.
- Large executable artifacts (`crush-enhanced`, `crush-test`, `crush-enhanced.exe`) are committed; they bloat the repo and should be removed before release.

## Critical Code Issues

### `internal/tui/layout/manager.go`
- `handleKeyMsg` returns a `tea.Cmd` only; signature must be `(tea.Model, tea.Cmd)`.
- `resizePane` returns `tea.Cmd` but is called as though it returns `(tea.Model, tea.Cmd)`.
- Typo `panses` (line ~247) prevents compilation.
- Uses `paneModel.View()` directly on `tea.Model`; Bubble Tea v2 models do not expose a `View()` method, so this code cannot compile.

### `internal/explorer/watcher.go`
- Imports `github.com/charmbracelet/log` instead of `github.com/charmbracelet/log/v2`.
- Missing closing brace in `Unsubscribe`; `watchDirectory` becomes nested and never usable.
- Overall package fails to build; dependent TUI components cannot work.

### `internal/tui/components/explorer/explorer.go`
- Relies on a nonexistent `tea.KeyBinding` type (should use `key.Binding`).
- Never subscribes to the watcher; `eventChan` is unused.
- Mouse handling and filesystem refreshes are left as TODOs, so functionality is incomplete.

### `internal/startup/manager.go`
- Status messages and logs contain mojibake (garbled characters), likely due to encoding conversion. These are user-facing prints.
- `ensureNarratorConfig` rewrites YAML blocks incorrectly; the narrator block search stops at the first indented line.
- Within the model discovery loop, `defer cancel()` runs each iteration, leaking context cancellations until loop exit.

### Encoding & Documentation
- `README.md` and several logging strings contain mojibake, indicating a charset corruption that needs correction before publishing.

## Technical Debt & TODO Hotspots
- `internal/themes/manager.go`: custom theme import/export explicitly marked TODO.
- `internal/config` package still relies on a global config instance (`TODO` comments note this debt).
- Chat editor component (`internal/tui/components/chat/editor`) has multiple TODO/XXX notes about cursor management and app dependencies.
- Many TODOs in `internal/tui/tui.go`, including reliance on “magic numbers” for layout adjustments.

## Deployment Readiness
The repository is not ready for deployment:
- Build/test pipeline is red.
- Several subsystems (layout manager, explorer, startup) are incomplete or broken.
- Encoding issues and committed binaries must be resolved.
- Requires dependency cleanup (log import path, Go version) and completion of missing features before release.

## Immediate Next Steps
1. Fix explorer and layout packages so the project compiles and tests run cleanly.
2. Correct Go toolchain version and remove committed binaries; ensure CI targets a released Go version.
3. Resolve mojibake in documentation and user-facing output.
4. Address critical TODOs (theme loading/export, config refactor, chat editor cleanup) or convert them into tracked issues with timelines.
