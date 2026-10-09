package styles

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuiltinThemes_SubagentPaletteFullyPopulated ensures every built-in
// theme carries a complete, non-zero subagent identity palette so subagent
// breadcrumbs always render with a distinct color instead of falling back
// to an unstyled dot.
func TestBuiltinThemes_SubagentPaletteFullyPopulated(t *testing.T) {
	t.Parallel()

	for _, name := range BuiltinThemeNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s, err := LoadTheme(name)
			require.NoError(t, err)
			require.Len(t, s.SubagentPalette, len(SubagentColorNames))
			for i, c := range s.SubagentPalette {
				require.NotNilf(t, c, "theme %q: SubagentPalette[%d] (%s) is nil", name, i, SubagentColorNames[i])
			}
		})
	}
}

// TestLoadPaletteTheme_SubagentPaletteFullyPopulated ensures that loading a
// theme through the palette-override path (e.g. a user theme file with a
// charmtone base and no explicit overrides) still yields a fully populated
// subagent palette, since ToQuickStyleOpts must forward it from the base.
func TestLoadPaletteTheme_SubagentPaletteFullyPopulated(t *testing.T) {
	t.Parallel()

	s, err := LoadPaletteTheme("charmtone-panther", Palette{})
	require.NoError(t, err)
	require.Len(t, s.SubagentPalette, len(SubagentColorNames))
	for i, c := range s.SubagentPalette {
		require.NotNilf(t, c, "LoadPaletteTheme(charmtone-panther, {}): SubagentPalette[%d] (%s) is nil", i, SubagentColorNames[i])
	}
}
