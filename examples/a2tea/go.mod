// This is a standalone nested module, deliberately separate from the crush
// root module so it never affects crush's build, tests, or CI (go build ./...
// and go mod tidy on the root module stop at this module boundary).
//
// It depends on github.com/joestump/a2tea, whose repository (joestump-agent/
// a2tea) declares the module path github.com/joestump/a2tea. Because those two
// differ, a2tea cannot be `go get`-ed by its declared path yet, so this module
// resolves it from a sibling checkout via the replace directive below. See
// README.md for how to run it.
module github.com/charmbracelet/crush/examples/a2tea

go 1.26.3

require (
	charm.land/bubbletea/v2 v2.0.7
	github.com/joestump/a2tea v0.0.0
)

require (
	charm.land/bubbles/v2 v2.1.0 // indirect
	charm.land/glamour/v2 v2.0.1 // indirect
	charm.land/lipgloss/v2 v2.0.4 // indirect
	github.com/alecthomas/chroma/v2 v2.14.0 // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/harmonica v0.2.0 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20260525132238-948f4557a654 // indirect
	github.com/charmbracelet/x/ansi v0.11.7 // indirect
	github.com/charmbracelet/x/exp/slice v0.0.0-20250327172914-2fdc97757edf // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/dlclark/regexp2 v1.11.0 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.0 // indirect
	github.com/mattn/go-runewidth v0.0.23 // indirect
	github.com/microcosm-cc/bluemonday v1.0.27 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/sahilm/fuzzy v0.1.1 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/yuin/goldmark v1.7.8 // indirect
	github.com/yuin/goldmark-emoji v1.0.5 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.24.0 // indirect
)

// Resolve a2tea from a sibling checkout (…/a2tea next to …/crush).
replace github.com/joestump/a2tea => ../../../a2tea
