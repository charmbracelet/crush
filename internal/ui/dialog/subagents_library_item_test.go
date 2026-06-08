package dialog

import (
	"testing"

	uistyles "github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
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

// TestLibrarySubagentItem_SelectedStyleReappliedAfterIcon verifies that the
// selected-row highlight survives past the status icon and dot. Both are
// pre-styled segments ending in an SGR reset, so text concatenated raw after
// them and wrapped in one outer style loses the highlight.
func TestLibrarySubagentItem_SelectedStyleReappliedAfterIcon(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	item := NewLibrarySubagentItem(&st, LibrarySubagentItemData{
		Name:        "my-agent",
		Description: "does stuff",
		Scope:       "user",
	})
	item.SetFocused(true)

	bg := st.Dialog.SelectedItem.GetBackground()
	icon := ansi.Strip(st.Tool.IconSuccess.String())
	scr := drawItem(item.Render(60), 60, 2)
	for _, text := range []string{icon, "●", "my-agent", "user", "does stuff"} {
		requirePlanHandoffColorEqual(t, bg, screenCell(t, scr, text).Style.Bg)
	}

	// A truncated row keeps the highlight through the ellipsis.
	scr = drawItem(item.Render(12), 12, 2)
	requirePlanHandoffColorEqual(t, bg, screenCell(t, scr, "…").Style.Bg)
}
