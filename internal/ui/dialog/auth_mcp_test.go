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

// TestMCPAuthHelpKeepsEscWhenNarrow verifies esc stays in the help bar
// when the optional hints are truncated.
func TestMCPAuthHelpKeepsEscWhenNarrow(t *testing.T) {
	t.Parallel()

	for _, st := range []MCPAuthState{MCPAuthStatePrompt, MCPAuthStateAuthenticating} {
		m := newTestMCPAuth("linear", "notion")
		m.state = st
		out := strings.Join(drawFits(t, m, 40, 30), "\n")
		require.Contains(t, out, "esc cancel", "state %d", st)
	}
}

// TestMCPAuthLinkWrapsAsOneToken verifies a long authorization URL is
// broken at fixed widths rather than at its hyphens, so every line but
// the last fills the row and the whole URL reads back in order.
func TestMCPAuthLinkWrapsAsOneToken(t *testing.T) {
	t.Parallel()

	const url = "https://auth.example.com/authorize?client-id=abc-def-ghi&redirect-uri=http%3A%2F%2Flocalhost&state=xyz-123"
	for _, w := range []int{100, 40} {
		m := newTestMCPAuth("linear")
		m.authURLFn = func(string) string { return url }
		m.state = MCPAuthStateAuthenticating
		var rows []string
		for _, l := range drawFits(t, m, w, 30) {
			l = strings.TrimSpace(strings.Trim(strings.TrimSpace(l), "│"))
			if len(rows) > 0 || strings.HasPrefix(l, "https://") {
				if l == "" {
					break
				}
				rows = append(rows, l)
			}
		}
		require.Equal(t, url, strings.Join(rows, ""), "width %d", w)
		for i := 1; i < len(rows)-1; i++ {
			require.Len(t, rows[i], len(rows[0]), "width %d row %d", w, i)
		}
	}
}
