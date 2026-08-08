package dialog

import (
	"testing"

	uistyles "github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// TestLibrarySubagentItem_RenderContainsName verifies that the rendered output
// of a LibrarySubagentItem contains the agent name.
func TestLibrarySubagentItem_RenderContainsName(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	item := NewLibrarySubagentItem(&st, LibrarySubagentItemData{
		Name:        "my-agent",
		Description: "does stuff",
		Scope:       "user",
	})

	rendered := item.Render(60)
	plain := stripANSIDialog(rendered)

	require.Contains(t, plain, "my-agent")
}

// TestLibrarySubagentItem_RenderContainsScopeBadge verifies that the rendered
// output contains the scope badge text for the item's scope.
func TestLibrarySubagentItem_RenderContainsScopeBadge(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	item := NewLibrarySubagentItem(&st, LibrarySubagentItemData{
		Name:        "my-agent",
		Description: "does stuff",
		Scope:       "user",
	})

	rendered := item.Render(60)
	plain := stripANSIDialog(rendered)

	require.Contains(t, plain, "user")
}

// TestLibrarySubagentItem_DisabledItemRendered verifies that rendering a
// disabled item does not panic and still contains the agent name.
func TestLibrarySubagentItem_DisabledItemRendered(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	item := NewLibrarySubagentItem(&st, LibrarySubagentItemData{
		Name:        "my-agent",
		Description: "does stuff",
		Scope:       "project",
		Disabled:    true,
	})

	var rendered string
	require.NotPanics(t, func() {
		rendered = item.Render(60)
	})

	plain := stripANSIDialog(rendered)
	require.Contains(t, plain, "my-agent")
}
