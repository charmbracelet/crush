package dialog

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/commands"
	"github.com/stretchr/testify/require"
)

func newPickerTestCommands(t *testing.T) *Commands {
	t.Helper()
	c, err := NewCommands(newPickerTestCommon(), "", false, false, false, false, nil, nil)
	require.NoError(t, err)
	return c
}

// TestCommandsResizeKeepsFilter verifies resizing the terminal does not
// clear what the user typed into the command filter.
func TestCommandsResizeKeepsFilter(t *testing.T) {
	t.Parallel()
	c := newPickerTestCommands(t)
	drawFits(t, c, 100, 30)
	for _, r := range "sess" {
		c.HandleMsg(keyMsg(r))
	}
	filtered := len(c.list.FilteredItems())
	text := strings.Join(drawFits(t, c, 90, 30), "\n")
	require.Equal(t, "sess", c.input.Value())
	require.Contains(t, text, "> sess")
	require.Len(t, c.list.FilteredItems(), filtered)
}

// TestCommandsShowsScrollbar verifies the command list shows a scrollbar
// when it overflows, like the other pickers.
func TestCommandsShowsScrollbar(t *testing.T) {
	t.Parallel()
	c := newPickerTestCommands(t)
	text := strings.Join(drawFits(t, c, 100, 12), "\n")
	require.Contains(t, text, "┃")
}

// TestCommandsHelpHidesTabWithoutLists verifies tab is only advertised
// when there are other command lists to switch to, after enter and esc.
func TestCommandsHelpHidesTabWithoutLists(t *testing.T) {
	t.Parallel()
	c := newPickerTestCommands(t)
	text := strings.Join(drawFits(t, c, 100, 30), "\n")
	require.NotContains(t, text, "tab")
	require.Contains(t, text, "esc")

	c, err := NewCommands(newPickerTestCommon(), "", false, false, false, false,
		[]commands.CustomCommand{{ID: "user:hi", Name: "hi"}}, nil)
	require.NoError(t, err)
	help := c.ShortHelp()
	require.Equal(t, "tab", help[len(help)-1].Help().Key)
}
