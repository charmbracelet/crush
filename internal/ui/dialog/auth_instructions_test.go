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
