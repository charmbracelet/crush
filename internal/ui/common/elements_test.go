package common

import (
	"testing"

	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestFormatContextUsage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		tokens        int64
		contextWindow int64
		want          string
	}{
		{name: "formats usage without context window", tokens: 1500, contextWindow: 0, want: "1.5k"},
		{name: "clamps usage to context window", tokens: 120, contextWindow: 100, want: "100 100%"},
		{name: "clamps negative usage to zero", tokens: -1, contextWindow: 100, want: "0 0%"},
		{name: "keeps usage below context window", tokens: 55, contextWindow: 100, want: "55 55%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, FormatContextUsage(tt.tokens, tt.contextWindow))
		})
	}
}

func TestFormatTokensAndCost(t *testing.T) {
	t.Parallel()

	theme := styles.DefaultStyles()
	rendered := ansi.Strip(formatTokensAndCost(&theme, 120, 25, 100, 1.23))

	require.Contains(t, rendered, "100 100% in")
	require.Contains(t, rendered, "25 out")
	require.Contains(t, rendered, "$1.23")
}
