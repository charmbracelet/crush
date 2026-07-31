package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/sahilm/fuzzy"
	"github.com/stretchr/testify/require"
)

// TestCommandItem_FilterIncludesShortcut is a regression guard for the
// commands dialog swallowing slash-command names. Typing "/clear" (or
// "clear") after opening the dialog must keep "New Session" visible; the
// filter previously matched only title+aliases+description, so a query for
// the shortcut text left the list empty.
func TestCommandItem_FilterIncludesShortcut(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	item := NewCommandItem(&s, "new_session", "New Session", "ctrl+n / /clear", ActionNewSession{})

	filter := item.Filter()
	require.Contains(t, filter, "/clear", "shortcut must be part of the filter string")
	require.Contains(t, filter, "New Session", "title must remain in the filter string")

	// The fuzzy matcher must find the item by its slash-command name.
	matches := fuzzy.FindFrom("clear", list.FilterableItemsSource{item})
	require.Len(t, matches, 1, "typing 'clear' should match the New Session command")
}
