//go:build darwin

package dialog

import (
	"testing"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestMacOSHintLabels(t *testing.T) {
	sty := styles.CharmtonePantera()
	item := NewCommandItem(&sty, "test", "Test", "ctrl+p", nil)
	require.Equal(t, "⌃p", item.InfoText())
	require.Equal(t, "ctrl+p", item.Shortcut())
	require.Contains(t, ansi.Strip(item.Render(120)), "⌃p")

	h := help.New()
	bindings := []key.Binding{
		key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "commands"),
		),
	}
	got := ansi.Strip(shortHelpLine(&h, bindings, 80))
	require.Contains(t, got, "⌃p")
}
