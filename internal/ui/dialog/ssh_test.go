package dialog

import (
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/sshaskpass"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

func TestSSHDescriptionAboveInput(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	req := sshaskpass.ParsePrompt("Enter PIN for ED25519-SK key /nowhere/id_sk: ")
	req.ID = "req"
	m := NewSSH(&common.Common{Styles: &s}, req)
	scr := uv.NewScreenBuffer(100, 30)
	cur := m.Draw(scr, scr.Bounds())
	require.NotNil(t, cur)

	lines := strings.Split(scr.String(), "\n")
	keyRow := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, "Key: ") })
	require.NotEqual(t, -1, keyRow)
	require.Less(t, keyRow, cur.Y)
	require.Contains(t, lines[cur.Y], "> Type your PIN...")
	require.NotContains(t, lines[keyRow], "id_sk:")
}

// TestSSHSmallTerminalKeepsInputVisible verifies that the description is
// trimmed so the input row, help and border stay inside a small terminal.
func TestSSHSmallTerminalKeepsInputVisible(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	for _, sz := range [][2]int{{30, 10}, {30, 7}, {40, 14}} {
		req := sshaskpass.ParsePrompt("Enter PIN for ED25519-SK key /some/very/long/path/to/the/key/id_ed25519_sk: ")
		req.ID = "req"
		m := NewSSH(&common.Common{Styles: &s}, req)
		lines := drawFits(t, m, sz[0], sz[1])
		cur := m.Draw(uv.NewScreenBuffer(sz[0], sz[1]), image.Rect(0, 0, sz[0], sz[1]))
		require.NotNil(t, cur)
		require.Contains(t, lines[cur.Y], "> ", "cursor lands on the input row at %dx%d", sz[0], sz[1])
	}
}
