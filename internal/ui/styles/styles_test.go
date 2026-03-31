package styles

import (
	"testing"

	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/stretchr/testify/require"
)

func TestThemeForBackground(t *testing.T) {
	t.Parallel()

	dark := ThemeForBackground(true)
	light := ThemeForBackground(false)

	require.Equal(t, charmtone.Pepper, dark.Background)
	require.Equal(t, charmtone.Salt, light.Background)
}
