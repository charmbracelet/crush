package dialog

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	mcptools "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func newTestMCPAuth(servers ...string) *MCPAuth {
	s := styles.CharmtonePantera()
	var pending []mcptools.PendingAuthServer
	for _, n := range servers {
		pending = append(pending, mcptools.PendingAuthServer{Name: n, URL: "https://" + n + ".example.com/mcp"})
	}
	m, _ := NewMCPAuth(&common.Common{Styles: &s}, pending, nil)
	return m
}

// TestMCPAuthErrorCanSkipToNextServer verifies a failed server can be
// skipped so the rest of the queue still gets a chance, and that the
// error carries a clear header above the raw reason.
func TestMCPAuthErrorCanSkipToNextServer(t *testing.T) {
	t.Parallel()

	m := newTestMCPAuth("linear", "notion")
	m.HandleMsg(ActionMCPAuthErrored{Error: errors.New("oauth: invalid_grant")})
	out := strings.Join(drawFits(t, m, 100, 30), "\n")
	require.Contains(t, out, "Authentication failed.")
	require.Contains(t, out, "oauth: invalid_grant")
	require.Contains(t, out, "s skip")
	require.Contains(t, out, "esc exit")

	require.Nil(t, m.HandleMsg(tea.KeyPressMsg{Code: 's', Text: "s"}))
	require.Equal(t, MCPAuthStatePrompt, m.state)
	require.Equal(t, "notion", m.currentServer().Name)

	// The last server has nothing to skip to.
	m.HandleMsg(ActionMCPAuthErrored{Error: errors.New("boom")})
	out = strings.Join(drawFits(t, m, 100, 30), "\n")
	require.NotContains(t, out, "s skip")
	require.Nil(t, m.HandleMsg(tea.KeyPressMsg{Code: 's', Text: "s"}))
	require.Equal(t, MCPAuthStateError, m.state)
}
