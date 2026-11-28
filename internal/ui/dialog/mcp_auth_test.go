package dialog

import (
	"errors"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
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

// TestMCPAuthSpinnerTicksOnlyWhileAuthenticating verifies the prompt, which
// draws no spinner, keeps no tick chain running, while authenticating does.
func TestMCPAuthSpinnerTicksOnlyWhileAuthenticating(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	m, cmd := NewMCPAuth(&common.Common{Styles: &s}, []mcptools.PendingAuthServer{
		{Name: "linear", URL: "https://linear.example.com/mcp"},
	}, nil)
	require.Nil(t, cmd)
	require.Equal(t, MCPAuthStatePrompt, m.state)

	tick := m.spinner.Tick().(spinner.TickMsg)
	require.Nil(t, m.HandleMsg(tick))

	m.state = MCPAuthStateAuthenticating
	action, ok := m.HandleMsg(tick).(ActionCmd)
	require.True(t, ok)
	require.NotNil(t, action.Cmd)
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
	require.Equal(t, CloseKey.Help().Desc, escLabel(m.ShortHelp()))

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
		require.Equal(t, cancelKey.Help().Desc, escLabel(m.ShortHelp()), "state %d", st)
		out := strings.Join(drawFits(t, m, 40, 30), "\n")
		require.Contains(t, out, "esc "+cancelKey.Help().Desc, "state %d", st)
	}
}

// TestMCPAuthDrawsWholeURL verifies a long authorization URL is drawn
// whole, broken to the content width rather than truncated.
func TestMCPAuthDrawsWholeURL(t *testing.T) {
	t.Parallel()

	const url = "https://auth.example.com/authorize?client-id=abc-def-ghi&redirect-uri=http%3A%2F%2Flocalhost&state=xyz-123"
	for _, w := range []int{100, 40} {
		m := newTestMCPAuth("linear")
		m.authURLFn = func(string) string { return url }
		m.state = MCPAuthStateAuthenticating
		requireLinkDrawn(t, drawFits(t, m, w, 30), url, w)
	}
}

// TestMCPAuthLeavesEnterToHelp verifies the dialog does not repeat the
// help bar with "press enter" instructions.
func TestMCPAuthLeavesEnterToHelp(t *testing.T) {
	t.Parallel()

	out := strings.Join(drawFits(t, newTestMCPAuth("linear", "notion"), 100, 30), "\n")
	require.NotContains(t, out, "Press enter")
	require.Contains(t, out, "enter open browser")
}

// TestMCPAuthTitleShowsQueueProgress verifies the title names the server
// and, with several queued, how far through the queue it is.
func TestMCPAuthTitleShowsQueueProgress(t *testing.T) {
	t.Parallel()

	out := strings.Join(drawFits(t, newTestMCPAuth("linear", "notion"), 100, 30), "\n")
	require.Contains(t, out, "Authenticate with linear (1/2)")

	out = strings.Join(drawFits(t, newTestMCPAuth("linear"), 100, 30), "\n")
	require.Contains(t, out, "Authenticate with linear")
	require.NotContains(t, out, "(1/1)")
}

// TestMCPAuthEscLabels verifies esc cancels while signing in and exits
// once the server has failed; the last server's success uses the shared
// finish keys, while earlier ones keep esc for leaving the queue.
func TestMCPAuthEscLabels(t *testing.T) {
	t.Parallel()

	m := newTestMCPAuth("linear")
	require.Equal(t, cancelKey.Help().Desc, escLabel(m.ShortHelp()), "prompt")
	m.state = MCPAuthStateError
	require.Equal(t, CloseKey.Help().Desc, escLabel(m.ShortHelp()), "failed")
	m.state = MCPAuthStateSuccess
	require.Equal(t, finishKey("finish").Keys(), m.ShortHelp()[0].Keys(), "finished")

	m = newTestMCPAuth("linear", "notion")
	m.state = MCPAuthStateSuccess
	require.Equal(t, CloseKey.Help().Desc, escLabel(m.ShortHelp()), "more queued")
}

// TestMCPAuthSpinnerSingleSpace verifies the waiting status puts exactly
// one space between the spinner and its text.
func TestMCPAuthSpinnerSingleSpace(t *testing.T) {
	t.Parallel()

	m := newTestMCPAuth("linear")
	m.state = MCPAuthStateAuthenticating
	out := strings.Join(drawFits(t, m, 100, 30), "\n")
	require.Contains(t, out, "Waiting for authorization...")
	require.NotContains(t, out, "  Waiting for authorization...")
}
