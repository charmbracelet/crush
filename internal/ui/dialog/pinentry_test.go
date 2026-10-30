package dialog

import (
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
