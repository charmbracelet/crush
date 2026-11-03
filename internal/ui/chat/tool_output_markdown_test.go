package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestToolOutputMarkdownContent_HasNoBlankTail pins that markdown output ends
// with its last line of text. Glamour ends with a blank, still-painted line,
// which would leave an empty row at the bottom of the tool's result box.
func TestToolOutputMarkdownContent_HasNoBlankTail(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	for _, content := range []string{
		"Done.\n\nAll good.",
		"Contents:\n\n```\na\nb\n```",
	} {
		out := toolOutputMarkdownContent(&sty, content, 80, true)
		lines := strings.Split(ansi.Strip(out), "\n")
		require.NotEmpty(t, strings.TrimSpace(lines[len(lines)-1]), "%q rendered %q", content, lines)
	}
}
