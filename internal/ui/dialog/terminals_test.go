package dialog

import (
	"image"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func newTerminalsForTest(entries ...TerminalEntry) *Terminals {
	s := styles.CharmtonePantera()
	return NewTerminals(&common.Common{Styles: &s}, entries)
}

func drawTerminals(d *Terminals) string {
	scr := uv.NewScreenBuffer(80, 24)
	d.Draw(scr, image.Rect(0, 0, 80, 24))
	return ansi.Strip(scr.Render())
}

func TestTerminalsListsEntries(t *testing.T) {
	t.Parallel()

	d := newTerminalsForTest(
		TerminalEntry{Session: &shell.InteractiveSession{}, Shown: true, Chat: "this chat"},
		TerminalEntry{Session: &shell.InteractiveSession{}, Chat: "another chat"},
	)

	view := drawTerminals(d)
	require.Contains(t, view, "shell", "a bare shell is labeled as such")
	require.Contains(t, view, "shown · this chat · you drive")
	require.Contains(t, view, "hidden · another chat · you drive")
}

func TestTerminalsEmpty(t *testing.T) {
	t.Parallel()

	d := newTerminalsForTest()
	require.Contains(t, drawTerminals(d), "No running terminals.")
	require.Nil(t, d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
	require.Nil(t, d.HandleMsg(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl}))
}

func TestTerminalsOpenAndKill(t *testing.T) {
	t.Parallel()

	first, second := &shell.InteractiveSession{}, &shell.InteractiveSession{}
	d := newTerminalsForTest(TerminalEntry{Session: first}, TerminalEntry{Session: second})

	open, ok := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}).(ActionOpenTerminal)
	require.True(t, ok)
	require.Same(t, first, open.Session)

	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	kill, ok := d.HandleMsg(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl}).(ActionKillTerminal)
	require.True(t, ok)
	require.Same(t, second, kill.Session)
	require.Len(t, d.list.Items(), 1, "a killed terminal leaves the list")
	require.Same(t, first, d.list.Items()[0].(*TerminalItem).entry.Session)
}
