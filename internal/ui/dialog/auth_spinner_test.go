package dialog

import (
	"strings"
	"testing"

	mcptools "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

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
