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

func TestSSHHostKeyConfirm(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	req := sshaskpass.ParsePrompt("The authenticity of host 'beef (100.64.0.3)' can't be established.\n" +
		"ED25519 key fingerprint is SHA256:Kx3nZ0h1nqk8m3QYV9Z5Qk2b0dF7Yw3sXo4n0eJt6aE.\n" +
		"Are you sure you want to continue connecting (yes/no/[fingerprint])? ")
	req.ID = "req"
	m := NewSSH(&common.Common{Styles: &s}, req)
	scr := uv.NewScreenBuffer(100, 30)
	require.Nil(t, m.Draw(scr, scr.Bounds()))

	out := scr.String()
	require.Contains(t, out, "New Host ")
	require.NotContains(t, out, "New Host.")
	require.Contains(t, out, "beef (100.64.0.3)")
	require.Contains(t, out, "SHA256:Kx3nZ0h1nqk8m3QYV9Z5Qk2b0dF7Yw3sXo4n0eJt6aE")
	require.Contains(t, out, "esc cancel")
	require.NotContains(t, out, "Press enter")
}

// TestSSHHostKeyKeepsFingerprintWhenShort verifies a short terminal cuts
// the message before the fingerprint: trusting a host key without seeing
// it defeats the prompt.
func TestSSHHostKeyKeepsFingerprintWhenShort(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	req := sshaskpass.ParsePrompt("The authenticity of host 'beef (100.64.0.3)' can't be established.\n" +
		"ED25519 key fingerprint is: SHA256:Hjs/OG4xvJbcRBLKRPAq7WduRSISxINIG/0s4ZlaHI4\n" +
		"Are you sure you want to continue connecting (yes/no/[fingerprint])? ")
	for h := 7; h <= 16; h++ {
		m := NewSSH(&common.Common{Styles: &s}, req)
		out := strings.Join(drawFits(t, m, 60, h), "\n")
		require.Contains(t, out, "SHA256:Hjs/OG4xvJbcRBLKRPAq7WduRSISxINIG/0s4ZlaHI4", "fingerprint cut at height %d", h)
	}
}

// TestSSHHostKeyKeepsGapBeforeFingerprint verifies the blank row between
// the message and the fingerprint is the last spacer to go when short.
func TestSSHHostKeyKeepsGapBeforeFingerprint(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	req := sshaskpass.ParsePrompt("The authenticity of host 'beef (100.64.0.3)' can't be established.\n" +
		"ED25519 key fingerprint is: SHA256:Hjs/OG4xvJbcRBLKRPAq7WduRSISxINIG/0s4ZlaHI4\n" +
		"Are you sure you want to continue connecting (yes/no/[fingerprint])? ")
	lines := drawFits(t, NewSSH(&common.Common{Styles: &s}, req), 60, 9)
	row := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, "Fingerprint:") })
	require.Positive(t, row)
	require.Equal(t, "││", strings.ReplaceAll(strings.TrimSpace(lines[row-1]), " ", ""), "a blank row sits above the fingerprint")
}
