package chat

import (
	"strconv"
	"testing"

	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestPendingShellItemRendersStreamedOutput verifies that a pending shell
// item surfaces output appended during execution, rather than hiding it
// behind the spinner until completion.
func TestPendingShellItemRendersStreamedOutput(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewPendingShellItem(&sty, "ping -c 3 localhost")

	require.NotContains(t, ansi.Strip(item.Render(80)), "bytes from",
		"freshly created pending item should have no output yet")

	item.AppendOutput("64 bytes from localhost: icmp_seq=0\n")
	item.AppendOutput("64 bytes from localhost: icmp_seq=1\n")

	rendered := ansi.Strip(item.Render(80))
	require.Contains(t, rendered, "icmp_seq=0",
		"streamed output must be visible while the command is still running")
	require.Contains(t, rendered, "icmp_seq=1")
}

// TestPendingShellItemShowsTail verifies that while streaming, the most
// recent lines are shown (tail) rather than the first lines.
func TestPendingShellItemShowsTail(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewPendingShellItem(&sty, "seq 30")

	for i := 1; i <= 30; i++ {
		item.AppendOutput(strconv.Itoa(i) + "\n")
	}

	rendered := ansi.Strip(item.Render(80))
	require.Contains(t, rendered, "30", "the newest line must be visible while streaming")
	require.Contains(t, rendered, "earlier lines", "older lines should be summarized as a count")
}

// TestCompletedShellItemRendersOutput verifies completion still shows output.
func TestCompletedShellItemRendersOutput(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewPendingShellItem(&sty, "echo hi")
	item.Complete("hi\n", 0)

	require.Contains(t, ansi.Strip(item.Render(80)), "hi")
}
