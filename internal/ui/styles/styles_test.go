package styles

import (
	"testing"

	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/stretchr/testify/require"
)

func TestDefaultStylesForBackground(t *testing.T) {
	t.Parallel()

	dark := DefaultStylesForBackground(true)
	light := DefaultStylesForBackground(false)

	require.Equal(t, charmtone.Pepper, dark.Background)
	require.Equal(t, charmtone.Salt, light.Background)
	require.Equal(t, charmtone.Pepper, light.FgBase)
}
