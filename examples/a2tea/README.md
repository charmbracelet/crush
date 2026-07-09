# a2tea example

The smallest possible demonstration of rendering an
[a2tea](https://github.com/joestump-agent/a2tea) TUI element inside a Bubble
Tea program. It describes one card as A2UI-inspired JSON, hands it to
`a2tea.Render`, and runs the result.

This is a **standalone nested module**, intentionally separate from the crush
root module — crush's `go build ./...`, `go test ./...`, and `go mod tidy` all
stop at this directory's `go.mod`, so the example never affects crush's build
or CI.

## Running it

The `a2tea` module declares the path `github.com/joestump/a2tea` but is hosted
at `joestump-agent/a2tea`, so it can't be fetched by its declared path yet.
This module resolves it from a **sibling checkout** via a `replace` directive.
Check both repos out next to each other:

```
some-dir/
├── a2tea/     # github.com/joestump-agent/a2tea
└── crush/     # this repo
```

Then:

```sh
cd crush/examples/a2tea
go run .
```

Press `q` (or `Esc` / `Ctrl+C`) to quit.

Because the a2tea renderers are still stubs, this currently draws a
`[a2tea: card]` placeholder — the point is to show the JSON → `tea.Model`
wiring end-to-end. When a2tea's real renderers land, the same program will draw
an actual card with no changes here.
