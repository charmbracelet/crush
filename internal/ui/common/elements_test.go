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

func TestFormatTokensAndCostWarnsWhenContextNearlyFull(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	full := formatTokensAndCost(&sty, 850, 1000, 0, false)
	roomy := formatTokensAndCost(&sty, 120, 1000, 0, false)

	// The warning is colour only, so the text itself gains nothing.
	require.Equal(t, "85% (850) $0.00", ansi.Strip(full))
	require.Contains(t, full, sty.ModelInfo.TokenPercentageWarn.Render("85%"))
	require.Contains(t, roomy, sty.ModelInfo.TokenPercentage.Render("12%"))
}
