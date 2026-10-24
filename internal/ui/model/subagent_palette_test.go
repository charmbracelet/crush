package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/subagents"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// TestSubagentPaletteNamesMatch guards the two color-name lists: a name
// subagents accepts but styles lacks would pass validation yet render as an
// unstyled dot.
func TestSubagentPaletteNamesMatch(t *testing.T) {
	t.Parallel()

	palette := subagents.Palette()
	require.ElementsMatch(t, palette[:], styles.SubagentColorNames[:])
}
