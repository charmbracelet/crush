package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// lazyPaletteWorkspace answers the palette's own read of the configuration.
// The rows about MCP only need a config with a server in it.
type lazyPaletteWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *lazyPaletteWorkspace) Config() *config.Config { return w.cfg }

func newLazyPalette(t *testing.T) *Commands {
	t.Helper()

	sty := styles.CharmtonePantera()
	ws := &lazyPaletteWorkspace{cfg: &config.Config{
		MCP: config.MCPs{"docker": {Type: config.MCPStdio, Command: "docker"}},
		// The palette reads the TUI options for its own display toggles, so a
		// bare Options is not enough to build it.
		Options: &config.Options{TUI: &config.TUIOptions{}},
	}}
	com := &common.Common{Styles: &sty, Workspace: ws}
	dia, err := NewCommands(com, "session-1", true, false, false, nil, nil)
	require.NoError(t, err)
	return dia
}

func paletteIDs(dia *Commands) []string {
	ids := make([]string, 0, len(dia.list.FilteredItems()))
	for _, item := range dia.list.FilteredItems() {
		if cmd, ok := item.(*CommandItem); ok {
			ids = append(ids, cmd.id)
		}
	}
	return ids
}

// TestPaletteOffersNoLazyMCPToggle: laziness is set per server in the MCP
// dialog, which the palette reaches through "Toggle MCPs". A global switch
// alongside it would be a second, conflicting door to the same decision, and
// the one that cannot say which server it is about.
func TestPaletteOffersNoLazyMCPToggle(t *testing.T) {
	t.Parallel()

	ids := paletteIDs(newLazyPalette(t))

	require.Contains(t, ids, "toggle_mcps",
		"the MCP dialog stays the way in to per-server settings")
	require.NotContains(t, ids, "toggle_lazy_mcp",
		"laziness is configured per server, not from the palette")
}
