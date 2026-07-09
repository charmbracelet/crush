// Command a2tea-card is the smallest possible demonstration of driving a
// terminal UI element from an A2UI-inspired JSON document using
// github.com/joestump/a2tea — the bridge crush will eventually use to let an
// agent describe UI as structured messages instead of raw text.
//
// It builds one "card" element, hands the JSON to a2tea.Render, and runs the
// resulting Bubble Tea model. a2tea.Render returns an *embeddable* child
// component (it never quits on its own), so we wrap it in a2tea.Standalone to
// run it as a self-contained program that exits on q / Esc / Ctrl+C.
//
// The a2tea renderers are still stubs, so this draws a "[a2tea: card]"
// placeholder today; the point is to show the wiring end-to-end.
package main

import (
	"encoding/json"
	"log"

	tea "charm.land/bubbletea/v2"

	"github.com/joestump/a2tea"
)

// card is a single A2UI-inspired component: a titled card with one button.
const card = `{
  "kind": "card",
  "id": "hello-crush",
  "title": "Hello from a2tea",
  "body": "This element was described as JSON and rendered by a2tea.",
  "buttons": [
    { "id": "ok", "label": "OK" }
  ]
}`

func main() {
	model, err := a2tea.Render(json.RawMessage(card))
	if err != nil {
		log.Fatalf("a2tea render: %v", err)
	}

	if _, err := tea.NewProgram(a2tea.Standalone(model)).Run(); err != nil {
		log.Fatalf("run: %v", err)
	}
}
