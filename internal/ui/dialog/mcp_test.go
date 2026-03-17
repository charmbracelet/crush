package dialog

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestCommandsDefaultCommandsIncludeMCPServers(t *testing.T) {
	com := testCommon(t, `{"options":{"disable_provider_auto_update":true},"tools":{}}`)
	d, err := NewCommands(com, "", false, false, false, session.CollaborationModeDefault, "", nil, nil)
	require.NoError(t, err)

	commands := d.defaultCommands()
	idx := slices.IndexFunc(commands, func(item *CommandItem) bool {
		return item.ID() == "mcp_servers"
	})
	require.NotEqual(t, -1, idx)

	action, ok := commands[idx].Action().(ActionOpenDialog)
	require.True(t, ok)
	require.Equal(t, MCPID, action.DialogID)
}

func TestMCPDialogItemsPreferAuthenticateForAuthNeededServers(t *testing.T) {
	t.Parallel()

	styles := testStyles()
	cfg := config.MCPs{
		"notion": {
			Type: config.MCPHttp,
			URL:  "https://example.com/mcp",
			OAuth: &config.MCPOAuthConfig{
				Enabled: true,
			},
		},
	}
	states := map[string]mcp.ClientInfo{
		"notion": {
			Name:   "notion",
			State:  mcp.StateNeedsAuth,
			Error:  errors.New("login required"),
			Counts: mcp.Counts{Tools: 2},
		},
	}

	items := mcpDialogItems(&styles, cfg, states)
	require.Len(t, items, 1)

	item, ok := items[0].(*MCPItem)
	require.True(t, ok)
	require.True(t, item.CanAuthenticate())
	require.True(t, item.CanReconnect())

	action, ok := item.DefaultAction().(ActionOpenMCPDetail)
	require.True(t, ok)
	require.Equal(t, "notion", action.Name)

	rendered := item.Render(120)
	require.Contains(t, rendered, "Authentication required")
	require.Contains(t, rendered, "OAuth")
	require.Contains(t, rendered, "Last error: login required")
}

func TestMCPDialogItemsAllowAuthenticateWithoutOAuthBlock(t *testing.T) {
	t.Parallel()

	styles := testStyles()
	cfg := config.MCPs{
		"notion": {
			Type: config.MCPHttp,
			URL:  "https://example.com/mcp",
		},
	}
	states := map[string]mcp.ClientInfo{
		"notion": {
			Name:  "notion",
			State: mcp.StateError,
			Error: errors.New("calling \"initialize\": sending \"initialize\": Unauthorized"),
		},
	}

	items := mcpDialogItems(&styles, cfg, states)
	require.Len(t, items, 1)

	item, ok := items[0].(*MCPItem)
	require.True(t, ok)
	require.True(t, item.CanAuthenticate())
	require.True(t, item.CanReconnect())

	action, ok := item.DefaultAction().(ActionOpenMCPDetail)
	require.True(t, ok)
	require.Equal(t, "notion", action.Name)
}

func testCommon(t *testing.T, configContent string) *common.Common {
	t.Helper()

	baseDir, err := os.MkdirTemp("", "crush-dialog-test-*")
	require.NoError(t, err)
	dataHome := filepath.Join(baseDir, "data-home")
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("LOCALAPPDATA", dataHome)
	t.Setenv("APPDATA", dataHome)
	t.Setenv("USERPROFILE", baseDir)

	workingDir := filepath.Join(baseDir, "workspace")
	require.NoError(t, os.MkdirAll(workingDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "crush.json"), []byte(configContent), 0o644))

	store, err := config.Init(workingDir, filepath.Join(baseDir, "state"), false)
	require.NoError(t, err)

	dbDir := filepath.Join(baseDir, "db")
	require.NoError(t, os.MkdirAll(dbDir, 0o755))
	conn, err := db.Connect(t.Context(), dbDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	application, err := app.New(t.Context(), conn, store)
	require.NoError(t, err)
	t.Cleanup(application.Shutdown)

	return common.DefaultCommon(application)
}

func testStyles() styles.Styles {
	return styles.DefaultStyles()
}
