package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/sahilm/fuzzy"
	"github.com/stretchr/testify/require"
)

// TestCommandItem_FilterAlias is a regression guard for the commands dialog
// swallowing slash-command names. Typing "/clear" (or "clear") after opening
// the dialog must keep "New Session" visible. The alias makes it discoverable;
// the shortcut is display-only and intentionally excluded from the filter.
func TestCommandItem_FilterAlias(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	item := NewCommandItem(&s, "new_session", "New Session", "ctrl+n", ActionNewSession{}).
		WithAliases("clear")

	filter := item.Filter()
	require.Contains(t, filter, "New Session", "title must be in the filter string")
	require.Contains(t, filter, "clear", "alias must be in the filter string")
	require.NotContains(t, filter, "ctrl+n", "shortcut must not be in the filter string")

	// The fuzzy matcher must find the item by its alias.
	matches := fuzzy.FindFrom("clear", list.FilterableItemsSource{item})
	require.Len(t, matches, 1, "typing 'clear' should match the New Session command via its alias")
}
