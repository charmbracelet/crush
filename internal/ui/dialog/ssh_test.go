package dialog

import (
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
