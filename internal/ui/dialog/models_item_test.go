package dialog

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestModelItemSelectedModelEnablesLMStudioThinking(t *testing.T) {
	sty := styles.CharmtonePantera()
	item := NewModelItem(&sty, catwalk.Provider{
		ID:   "tailnet-lmstudio",
		Type: catwalk.Type("lmstudio"),
	}, catwalk.Model{
		ID:        "qwythos-9b",
		CanReason: true,
	}, ModelTypeLarge, false)

	selected := item.SelectedModel()

	require.True(t, selected.Think)
}
