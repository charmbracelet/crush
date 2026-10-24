package dialog

import (
	"testing"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/crush/internal/ui/keyhint"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestCommandItemInfoTextUsesPlatformLabel(t *testing.T) {
	item := NewCommandItem(nil, "test", "Test", "ctrl+p", nil)

	require.Equal(t, "ctrl+p", item.Shortcut(),
		"key matching must keep the raw shortcut")
	require.Equal(t, keyhint.Label("ctrl+p"), item.InfoText())
}

func TestCommandItemRenderUsesPlatformLabel(t *testing.T) {
	sty := styles.CharmtonePantera()
	item := NewCommandItem(&sty, "test", "Test", "ctrl+p", nil)

	got := ansi.Strip(item.Render(120))
	require.Contains(t, got, keyhint.Label("ctrl+p"))
}

func TestShortHelpLineUsesPlatformLabel(t *testing.T) {
	h := help.New()
	bindings := []key.Binding{
		key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "commands"),
		),
	}

	got := ansi.Strip(shortHelpLine(&h, bindings, 80))
	require.Contains(t, got, keyhint.Label("ctrl+p"))
}

func TestQuitHintUsesPlatformLabel(t *testing.T) {
	q := newTestQuit()
	scr := drawQuit(t, q)

	require.Contains(t, ansi.Strip(scr.Render()), keyhint.Label("press ctrl+c twice."))
}
