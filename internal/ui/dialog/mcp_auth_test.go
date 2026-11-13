package dialog

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	tea "charm.land/bubbletea/v2"
	mcptools "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// Enter hands the browser launch to a command, so a slow launcher can't
// stall the UI.
func TestMCPAuthOpensBrowserInCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake browser launcher is a shell script")
	}
	bin := t.TempDir()
	opened := filepath.Join(bin, "opened")
	launcher := "#!/bin/sh\nprintf '%s' \"$1\" > \"${0%/*}/opened\"\n"
	for _, name := range []string{"xdg-open", "open"} {
		require.NoError(t, os.WriteFile(filepath.Join(bin, name), []byte(launcher), 0o755))
	}
	t.Setenv("PATH", bin)
	sty := styles.CharmtonePantera()
	m, _ := NewMCPAuth(&common.Common{Styles: &sty}, []mcptools.PendingAuthServer{{Name: "docs"}},
		func(string) string { return "https://example.com/authorize" })
	m.state = MCPAuthStateAuthenticating
	action, ok := m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}).(ActionCmd)
	require.True(t, ok)
	require.NoFileExists(t, opened, "the update does not launch the browser")
	action.Cmd()
	url, err := os.ReadFile(opened)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/authorize", string(url))
}
