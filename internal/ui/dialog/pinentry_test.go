package dialog

import (
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/pinentry"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

func TestPinentryKeyDetailsAboveInput(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	m := NewPinentry(&common.Common{Styles: &s}, pinentry.PromptRequest{
		ID:     "req",
		Kind:   pinentry.KindPIN,
		Prompt: "Your security key wants its PIN. It's not getting any younger.",
		Key:    pinentry.Key{Fingerprint: "DC6511D5A8A66E5D70C2A344877E0EA9036E900E"},
	})
	scr := uv.NewScreenBuffer(100, 30)
	cur := m.Draw(scr, scr.Bounds())
	require.NotNil(t, cur)

	lines := strings.Split(scr.String(), "\n")
	keyRow := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, "Key:") })
	require.NotEqual(t, -1, keyRow)
	require.Less(t, keyRow, cur.Y, "key details sit above the input")
	require.Contains(t, lines[cur.Y], "> Type your PIN...", "cursor lands on the input row")
}

// TestPinentrySmallTerminalKeepsInputVisible verifies that a long
// description is trimmed so the input row, help and border stay inside a
// small terminal. Previously the description wrapped freely and at 30x10
// pushed the input row (and the cursor) below the area.
func TestPinentrySmallTerminalKeepsInputVisible(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	for _, sz := range [][2]int{{30, 10}, {30, 7}, {40, 14}} {
		m := NewPinentry(&common.Common{Styles: &s}, pinentry.PromptRequest{
			ID:     "req",
			Kind:   pinentry.KindPIN,
			Prompt: "Your security key wants its PIN. It's not getting any younger.",
			Key:    pinentry.Key{Fingerprint: "DC6511D5A8A66E5D70C2A344877E0EA9036E900E"},
		})
		lines := drawFits(t, m, sz[0], sz[1])
		cur := m.Draw(uv.NewScreenBuffer(sz[0], sz[1]), image.Rect(0, 0, sz[0], sz[1]))
		require.NotNil(t, cur)
		require.Contains(t, lines[cur.Y], "> ", "cursor lands on the input row at %dx%d", sz[0], sz[1])
	}
}
