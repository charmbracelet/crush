package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestQuitWarnsAboutRunningSubagents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		running int
		want    string
	}{
		{"one", 1, "1 subagent still running; quitting stops them."},
		{"two", 2, "2 subagents still running; quitting stops them."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			q := newTestQuit()
			q.SetRunningSubagents(tt.running)

			lines := strings.Split(ansi.Strip(drawQuit(t, q).Render()), "\n")
			warning, buttons := -1, -1
			for i, line := range lines {
				switch {
				case strings.Contains(line, tt.want):
					warning = i
				case strings.Contains(line, "Yep!"):
					buttons = i
				}
			}
			require.NotEqual(t, -1, warning, "warning line missing")
			require.NotEqual(t, -1, buttons, "buttons missing")
			require.Less(t, warning, buttons, "warning must sit above the buttons")
		})
	}
}

func TestQuitHasNoSubagentWarningWhenNoneRunning(t *testing.T) {
	t.Parallel()

	q := newTestQuit()
	q.SetRunningSubagents(0)

	require.NotContains(t, ansi.Strip(drawQuit(t, q).Render()), "still running")
}

func TestQuitSubagentWarningKeepsButtonsOnTheirHitRegions(t *testing.T) {
	t.Parallel()

	q := newTestQuit()
	q.SetRunningSubagents(2)
	scr := drawQuit(t, q)

	for i, label := range []string{"Yep!", "Nope"} {
		layer := q.compositor.GetLayer(fmt.Sprintf("btn_%d", i))
		require.NotNil(t, layer)
		var got string
		for x := layer.GetX(); x < layer.GetX()+layer.Width(); x++ {
			got += scr.CellAt(x, layer.GetY()).Content
		}
		require.Contains(t, got, label)
	}
}
