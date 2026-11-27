package dialog

import (
	"strings"
	"testing"

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
