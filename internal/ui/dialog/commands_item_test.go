package dialog

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestCommandDescriptionFitsWidth(t *testing.T) {
	t.Parallel()
	sty := styles.CharmtonePantera()
	for _, description := range []string{strings.Repeat("word ", 20), "   "} {
		item := NewCommandItem(&sty, "id", "Title", "", nil).WithDescription(description)
		for _, focused := range []bool{false, true} {
			item.SetFocused(focused)
			for _, line := range strings.Split(item.Render(30), "\n") {
				require.Equal(t, 30, lipgloss.Width(line), "description %q, focused %v: %q", description, focused, line)
			}
		}
	}
}
