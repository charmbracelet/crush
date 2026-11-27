package common

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestFormatTokensAndCostPrefixesEstimatedUsage(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	rendered := formatTokensAndCost(&sty, 120, 1000, 0, true)
	actual := ansi.Strip(rendered)

	require.Contains(t, actual, "~12%")
	require.Contains(t, actual, "(120)")
	require.Contains(t, actual, "$0.00")
	require.True(t, strings.Contains(rendered, sty.ModelInfo.TokenPercentage.Render("~12%")))
}

func TestFormatTokensAndCostOmitsEstimatedPrefix(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	actual := ansi.Strip(formatTokensAndCost(&sty, 120, 1000, 0, false))

	require.Contains(t, actual, "12%")
	require.NotContains(t, actual, "~12%")
}

func TestStatusFitsNarrowWidth(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	for _, tc := range []struct {
		name string
		opts StatusOpts
		want string
	}{
		{"long title", StatusOpts{Icon: "●", Title: "thermo-nuclear-code-review"}, "● thermo-nucl…"},
		{"title squeezes description", StatusOpts{Icon: "●", Title: "cloudflare-mcp", Description: "error"}, "● cloudflare-…"},
		{"fits", StatusOpts{Icon: "●", Title: "jj", Description: "2 tools"}, "● jj 2 tools"},
		{"extra content keeps its room", StatusOpts{Icon: "●", Title: "typescript-lsp", ExtraContent: "E3"}, "● typescri… E3"},
		{"extra content dropped when it crowds the title", StatusOpts{Icon: "●", Title: "typescript-lsp", ExtraContent: "E3 W12 I4 H9"}, "● typescript-…"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Status(&sty, tc.opts, 14)
			require.Equal(t, tc.want, ansi.Strip(got))
			require.LessOrEqual(t, ansi.StringWidth(got), 14)
		})
	}
}
