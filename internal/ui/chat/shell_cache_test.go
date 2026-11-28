package chat

import (
	"fmt"
	"strings"
	"testing"
	"unsafe"

	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// A pending shell item reuses its rendered output across spinner frames,
// and still picks up new output, expansion, focus and scrolling at once.
func TestPendingShellItemCachesBodyAcrossFrames(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewPendingShellItem(&sty, "make")
	var b strings.Builder
	for i := range 30 {
		fmt.Fprintf(&b, "building target %d %s\n", i, strings.Repeat("=", i*4))
	}
	item.AppendOutput(b.String())

	first := item.RawRender(80)
	body := item.pendingBody
	require.NotEmpty(t, body)
	require.True(t, strings.HasPrefix(first, body+"\n"))

	// A spinner frame only redraws the spinner line.
	item.Advance()
	require.Same(t, unsafe.StringData(body), unsafe.StringData(item.pendingBody))
	framed := item.RawRender(80)
	require.True(t, strings.HasPrefix(framed, body+"\n"))
	require.Same(t, unsafe.StringData(body), unsafe.StringData(item.pendingBody), "a frame must reuse the body")

	item.AppendOutput("building target 30\n")
	require.Contains(t, ansi.Strip(item.RawRender(80)), "building target 30")

	item.ToggleExpanded()
	expanded := ansi.Strip(item.RawRender(80))
	require.Contains(t, expanded, "building target 0 ")
	require.NotContains(t, expanded, "earlier lines")

	item.ScrollHorizontal(10)
	require.Contains(t, ansi.Strip(item.RawRender(80)), "…")

	item.SetFocused(true)
	focused := item.RawRender(80)
	item.SetFocused(false)
	require.NotEqual(t, focused, item.RawRender(80))

	item.Complete("done\n", 0)
	require.Empty(t, item.pendingBody)
	require.Contains(t, ansi.Strip(item.RawRender(80)), "done")
}
