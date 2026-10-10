package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestCommandsTallestTabHeightIsMemoized(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	short := NewCommandItem(&sty, "short", "Short", "", ActionNewSession{})
	long := NewCommandItem(&sty, "long", "Long", "", ActionNewSession{}).
		WithDescription("A description that takes a row of its own.")

	c := &Commands{userItems: []*CommandItem{short}}
	height := c.tallestTabHeight(40)
	require.Equal(t, 1, height)

	// Replacing the items without a rebuild leaves the cached height in
	// place; only an invalidation (as setCommandItems performs) drops it.
	c.userItems = []*CommandItem{long}
	require.Equal(t, height, c.tallestTabHeight(40))

	c.tabHeightsOK = false
	require.Equal(t, 2, c.tallestTabHeight(40))
}

func TestCommandsTallestTabHeightIsMemoizedPerWidth(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	c := &Commands{
		userItems: []*CommandItem{
			NewCommandItem(&sty, "one", "One", "", ActionNewSession{}),
		},
	}

	wide := c.tallestTabHeight(80)
	narrow := c.tallestTabHeight(40)
	require.Equal(t, wide, c.tallestTabHeight(80))
	require.Equal(t, narrow, c.tallestTabHeight(40))
}
