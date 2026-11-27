package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

// TestThemeNewFitsArea verifies the new theme prompt keeps its whole border
// on narrow screens.
func TestThemeNewFitsArea(t *testing.T) {
	t.Parallel()
	for _, sz := range [][2]int{{100, 30}, {40, 30}, {100, 12}} {
		drawFits(t, NewThemeNew(newPickerTestCommon(), ""), sz[0], sz[1])
	}
}

// TestThemeNewNoReservedErrorSpace verifies the new theme prompt leaves a
// single blank line between the input and help, and fits an error when
// one appears.
func TestThemeNewNoReservedErrorSpace(t *testing.T) {
	t.Parallel()
	d := NewThemeNew(newPickerTestCommon(), "")
	rows := func() []string {
		var out []string
		for _, l := range drawFits(t, d, 100, 30) {
			if l = strings.TrimSpace(l); l != "" {
				out = append(out, strings.Trim(l, "│ "))
			}
		}
		return out
	}
	// Border, title, blank, input, blank, help, border.
	got := rows()
	require.Len(t, got, 7, strings.Join(got, "\n"))
	require.Contains(t, got[5], "enter create")

	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotEmpty(t, d.err)
	got = rows()
	require.Greater(t, len(got), 7)
	require.Contains(t, got[len(got)-2], "enter create")
	require.Empty(t, got[len(got)-3])
}
