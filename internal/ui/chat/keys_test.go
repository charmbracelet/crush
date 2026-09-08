package chat

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// Rebinding the package vars reaches per-item handlers; the old
// literal stops matching.
func TestItemKeys_RemappedCopyReachesHandler(t *testing.T) {
	t.Parallel()
	old := ItemCopy
	ItemCopy = key.NewBinding(key.WithKeys("x"))
	defer func() { ItemCopy = old }()

	sty := styles.CharmtonePantera()
	item := NewShellItem(&sty, "echo hi", "hi", 0)

	handled, _ := item.(KeyEventHandler).HandleKeyEvent(tea.KeyPressMsg{Code: 'x'})
	require.True(t, handled, "remapped copy key was not handled")

	handled, _ = item.(KeyEventHandler).HandleKeyEvent(tea.KeyPressMsg{Code: 'c'})
	require.False(t, handled, "old copy literal still handled after remap")
}
