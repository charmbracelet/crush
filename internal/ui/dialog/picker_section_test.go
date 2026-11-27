package dialog

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestSectionHeadersKeepRightPadding verifies picker section headers end
// with the same one-column pad as item rows instead of touching the border.
func TestSectionHeadersKeepRightPadding(t *testing.T) {
	t.Parallel()
	com := newPickerTestCommon()
	theme := (&ThemeSectionHeader{Versioned: &list.Versioned{}, title: "System", t: com.Styles}).Render(40)
	group := (&ModelGroup{Title: "OpenAI", t: com.Styles}).Render(40)
	for _, s := range []string{theme, group} {
		plain := ansi.Strip(s)
		require.Equal(t, 39, ansi.StringWidth(plain), plain)
		require.False(t, strings.HasSuffix(plain, " "), plain)
	}
}
