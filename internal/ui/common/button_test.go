package common

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestButtonHitCompositorLayouts(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	buttons := []ButtonOpts{
		{Text: "Implement", Padding: 1, UnderlineIndex: -1},
		{Text: "Request changes", Padding: 1, UnderlineIndex: -1},
	}

	t.Run("horizontal", func(t *testing.T) {
		t.Parallel()

		const x, y = 4, 7
		compositor := ButtonHitCompositor(&sty, buttons, " ", x, y)
		firstWidth := lipgloss.Width(Button(&sty, buttons[0]))

		require.Equal(t, 0, HitButtonIndex(compositor, x, y))
		require.Equal(t, 1, HitButtonIndex(compositor, x+firstWidth+1, y))
	})

	t.Run("vertical", func(t *testing.T) {
		t.Parallel()

		const x, y = 4, 7
		compositor := ButtonHitCompositor(&sty, buttons, "\n", x, y)

		require.Equal(t, 2, lipgloss.Height(ButtonGroup(&sty, buttons, "\n")))
		require.Equal(t, 0, HitButtonIndex(compositor, x, y))
		require.Equal(t, 1, HitButtonIndex(compositor, x, y+1))
	})
}
