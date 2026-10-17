package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/lsp"
	uistyles "github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestResourceListsCountHiddenItems verifies that the "...and N more" line
// counts every item it hides. Five items with maxItems 4 show three rows and
// the hint, so two items are hidden.
func TestResourceListsCountHiddenItems(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	names := []string{"one", "two", "three", "four", "five"}

	var lsps []LSPInfo
	var mcps []mcp.ClientInfo
	for _, name := range names {
		lsps = append(lsps, LSPInfo{LSPClientInfo: workspace.LSPClientInfo{Name: name, State: lsp.StateDisabled}})
		mcps = append(mcps, mcp.ClientInfo{Name: name, State: mcp.StateDisabled})
	}

	for kind, out := range map[string]string{
		"lsp": lspList(&st, lsps, 80, 4),
		"mcp": mcpList(&st, mcps, 80, 4),
	} {
		out = ansi.Strip(out)
		shown := 0
		for _, name := range names {
			if strings.Contains(out, name) {
				shown++
			}
		}
		require.Equal(t, 3, shown, "%s: visible rows", kind)
		require.Contains(t, out, fmt.Sprintf("and %d more", len(names)-shown), "%s: hint", kind)
	}
}
