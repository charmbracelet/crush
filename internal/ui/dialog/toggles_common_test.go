package dialog

import (
	"fmt"
	"image"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestTogglesFitTheWindow checks that both toggles dialogs stay whole in a
// short or narrow window: a short window shows fewer rows that still follow
// the cursor, and a narrow one cuts long names instead of wrapping rows.
func TestTogglesFitTheWindow(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		build func(names ...string) Dialog
		// status is painted at the right edge of every row.
		status string
	}{
		{"mcp", func(names ...string) Dialog {
			items := make([]MCPToggleItem, len(names))
			for i, name := range names {
				items[i] = MCPToggleItem{Name: name, Status: "connected"}
			}
			return newMCPTogglesForTest(items)
		}, "connected"},
		{"mcp with a long error", func(names ...string) Dialog {
			items := make([]MCPToggleItem, len(names))
			for i, name := range names {
				items[i] = MCPToggleItem{Name: name, Status: `error: failed to create mcp client: exec: "npx": executable file not found in $PATH`}
			}
			return newMCPTogglesForTest(items)
		}, "error: "},
		{"skills", func(names ...string) Dialog {
			items := make([]SkillToggleItem, len(names))
			for i, name := range names {
				items[i] = SkillToggleItem{Name: name, Error: "unreadable"}
			}
			return newSkillsTogglesForTest(items)
		}, "error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			paint := func(d Dialog, area image.Rectangle) string {
				scr := uv.NewScreenBuffer(area.Max.X+2, area.Max.Y+2)
				d.Draw(scr, area)
				return ansi.Strip(scr.Render())
			}

			names := make([]string, 20)
			for i := range names {
				names[i] = fmt.Sprintf("entry-%02d", i)
			}
			d := test.build(names...)
			// Twelve rows and the dialog's chrome need 18 lines.
			short := image.Rect(3, 2, 87, 16)
			require.Contains(t, paint(d, short), "╰", "the dialog's bottom border fits the window")
			for range len(names) - 1 {
				d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
			}
			screen := paint(d, short)
			require.Contains(t, screen, "╰")
			require.Contains(t, screen, "entry-19", "the cursor row stays in view")
			require.NotContains(t, screen, "entry-00")

			long := "a-very-long-name-that-does-not-fit-in-a-narrow-window"
			for _, line := range strings.Split(paint(test.build(long, "short"), image.Rect(0, 0, 50, 30)), "\n") {
				if strings.Contains(line, "a-very-long") {
					require.Contains(t, line, "…", "the name is cut")
					require.Contains(t, line, test.status, "the status stays on the name's line")
					return
				}
			}
			t.Fatal("the long name is not painted")
		})
	}
}
