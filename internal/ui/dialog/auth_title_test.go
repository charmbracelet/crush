package dialog

import (
	"strings"
	"testing"

	mcptools "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

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
