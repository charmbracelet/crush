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

// TestLibrarySubagentItem_ErrorRendered verifies that a broken definition
// renders its discovery diagnostic in place of the description.
func TestLibrarySubagentItem_ErrorRendered(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	item := NewLibrarySubagentItem(&st, LibrarySubagentItemData{
		Name:        "broken-agent",
		Description: "does stuff",
		Scope:       "user",
		Error:       "unclosed frontmatter",
	})

	rendered := item.Render(80)
	plain := stripANSIDialog(rendered)

	require.Contains(t, plain, "broken-agent")
	require.Contains(t, plain, "unclosed frontmatter")
	require.NotContains(t, plain, "does stuff")
}

// TestLibrarySubagentItem_ErrorIDIsFilePath verifies that a broken definition
// identifies itself by path. Several files can claim one name — only one wins
// discovery — so keying rows by name would collapse a broken entry and the
// valid namesake it shadows into a single list identity.
func TestLibrarySubagentItem_ErrorIDIsFilePath(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()

	broken := NewLibrarySubagentItem(&st, LibrarySubagentItemData{
		Name:     "reviewer",
		FilePath: "/project/.crush/subagents/reviewer.md",
		Error:    "unknown model",
	})
	valid := NewLibrarySubagentItem(&st, LibrarySubagentItemData{
		Name:     "reviewer",
		FilePath: "/home/me/.config/crush/subagents/reviewer.md",
	})

	require.Equal(t, "/project/.crush/subagents/reviewer.md", broken.ID())
	require.Equal(t, "reviewer", valid.ID(), "valid rows keep the name as their identity")
	require.NotEqual(t, valid.ID(), broken.ID())
}
