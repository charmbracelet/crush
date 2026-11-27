package dialog

import (
	"strings"
	"testing"

	mcptools "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// TestAuthNoInstructionsDuplicatingHelp verifies the auth dialogs leave
// "press enter" instructions to the help bar, and that MCP shows its
// queue progress in the title.
func TestAuthNoInstructionsDuplicatingHelp(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s}

	browser := newTestOAuth(false)
	browser.HandleMsg(ActionInitiateOAuth{VerificationURL: testOAuthURL})
	device := newTestOAuth(false)
	device.HandleMsg(ActionInitiateOAuth{UserCode: "ABCD-EFGH", VerificationURL: "https://example.com/device"})
	sso, _ := NewAWSSSO(com, "aws sso login")
	sso.SetURL("https://device.sso.us-east-1.amazonaws.com/")
	mcp, _ := NewMCPAuth(com, []mcptools.PendingAuthServer{
		{Name: "linear", URL: "https://mcp.linear.app/sse"},
		{Name: "notion", URL: "https://mcp.notion.com"},
	}, nil)

	for name, d := range map[string]Dialog{"oauth browser": browser, "oauth device": device, "aws sso": sso, "mcp": mcp} {
		out := strings.Join(drawFits(t, d, 100, 30), "\n")
		require.NotContains(t, out, "Press enter", name)
	}
	out := strings.Join(drawFits(t, mcp, 100, 30), "\n")
	require.Contains(t, out, "Authenticate with linear (1/2)")
}

// TestAuthEscLabelsFollowOneRule verifies every sign-in dialog labels esc
// the same way: "cancel" while a sign-in is in progress, "exit" once it
// has failed and there is nothing left to cancel.
func TestAuthEscLabelsFollowOneRule(t *testing.T) {
	t.Parallel()

	m := newTestOAuth(false)
	require.Equal(t, "cancel", escLabel(m.ShortHelp()), "starting")
	m.HandleMsg(ActionInitiateOAuth{VerificationURL: "https://example.com/device"})
	require.Equal(t, "cancel", escLabel(m.ShortHelp()), "waiting")
	m.State = OAuthStateError
	require.Equal(t, "exit", escLabel(m.ShortHelp()), "failed")

	mcp := newTestMCPAuth("linear")
	require.Equal(t, "cancel", escLabel(mcp.ShortHelp()), "mcp prompt")
	mcp.state = MCPAuthStateError
	require.Equal(t, "exit", escLabel(mcp.ShortHelp()), "mcp failed")
}

// TestAuthSpinnerSingleSpace verifies status lines put exactly one space
// between the spinner and its text; the spinner already ends in one.
func TestAuthSpinnerSingleSpace(t *testing.T) {
	t.Parallel()

	o := newTestOAuth(false)
	o.State = OAuthStateSaving
	out := strings.Join(drawFits(t, o, 100, 30), "\n")
	require.Contains(t, out, "Fetching models...")
	require.NotContains(t, out, "  Fetching models...")

	s := styles.CharmtonePantera()
	m, _ := NewMCPAuth(&common.Common{Styles: &s},
		[]mcptools.PendingAuthServer{{Name: "linear", URL: "https://mcp.linear.app/sse"}}, nil)
	m.state = MCPAuthStateAuthenticating
	out = strings.Join(drawFits(t, m, 100, 30), "\n")
	require.Contains(t, out, "Waiting for authorization...")
	require.NotContains(t, out, "  Waiting for authorization...")
}

// TestAuthDialogTitlesConsistent verifies the auth dialogs share one
// title phrasing.
func TestAuthDialogTitlesConsistent(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	mcp, _ := NewMCPAuth(&common.Common{Styles: &s},
		[]mcptools.PendingAuthServer{{Name: "linear", URL: "https://mcp.linear.app/sse"}}, nil)
	for name, d := range map[string]interface {
		Dialog
	}{
		"auth method": newTestAuthMethod(),
		"oauth":       newTestOAuth(false),
		"mcp":         mcp,
	} {
		out := strings.Join(drawFits(t, d, 100, 30), "\n")
		require.Contains(t, out, "Authenticate with ", name)
		require.NotContains(t, out, "Let’s", name)
	}
}
