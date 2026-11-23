package model

import (
	"testing"

	"charm.land/x/nerdfont"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
	"github.com/stretchr/testify/require"
)

// TestLSPDiagnosticsIcons covers the LSP diagnostic counts shown in the
// sidebar: the Nerd Font severity glyphs render only when the terminal is
// expected to render them, and the E, W, I, and H letters remain the
// fallback otherwise.
func TestLSPDiagnosticsIcons(t *testing.T) {
	// Not parallel: t.Setenv is process-wide, as is the nerdfont probe memo.
	st := styles.CharmtonePantera()
	diagnostics := map[protocol.DiagnosticSeverity]int{
		protocol.SeverityError:       1,
		protocol.SeverityWarning:     2,
		protocol.SeverityInformation: 3,
		protocol.SeverityHint:        4,
	}

	tests := []struct {
		name      string
		nerdFonts string
		want      string
	}{
		{
			name:      "glyphs with nerd font support",
			nerdFonts: "true",
			want: styles.LSPErrorIcon + "1 " +
				styles.LSPWarningIcon + "2 " +
				styles.LSPHintIcon + "4 " +
				styles.LSPInfoIcon + "3",
		},
		{
			name:      "letters without nerd font support",
			nerdFonts: "false",
			want:      "E1 W2 H4 I3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(nerdfont.EnvVar, tt.nerdFonts)
			nerdfont.Reset()
			t.Cleanup(nerdfont.Reset)

			require.Equal(t, tt.want, ansi.Strip(lspDiagnostics(&st, diagnostics)))
		})
	}
}
