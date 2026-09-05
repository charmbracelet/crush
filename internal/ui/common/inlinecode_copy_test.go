package common

import (
	"testing"

	"charm.land/glamour/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestRenderedInlineCodeHasNoVisibleBackticks guards the display side of the
// inline-code copy fix: codespans render blank padding (a no-break-space
// sentinel), never the backticks themselves. The sentinel is what selection
// copies turn back into backticks (see list.HighlightContent).
func TestRenderedInlineCodeHasNoVisibleBackticks(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	render := func(t *testing.T, r *glamour.TermRenderer, src string) string {
		t.Helper()
		mu := LockMarkdownRenderer(r)
		mu.Lock()
		defer mu.Unlock()
		rendered, err := r.Render(src)
		require.NoError(t, err)
		return ansi.Strip(rendered)
	}

	for name, r := range map[string]*glamour.TermRenderer{
		"markdown": MarkdownRenderer(&sty, 80),
		"quiet":    QuietMarkdownRenderer(&sty, 80),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			actual := render(t, r, `message "this is `+"`code`"+`" ok`)

			require.NotContains(t, actual, "`", "codespan backticks must not render on screen")
			require.Contains(
				t,
				actual,
				`this is `+styles.CodespanPadding+`code`+styles.CodespanPadding+`" ok`,
				"codespan must render with the sentinel padding",
			)
		})
	}
}
