package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// An edit can fail before it ever produces a diff — refusing to touch a file
// that was never read, for one — and the failure then has nothing to show
// underneath it. The error used to be followed by a blank line and an empty
// diff, which read as a gap in the transcript with nothing in it.
func TestEditErrorWithoutDiffHasNoTrailingBlank(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	// Metadata that parses but carries no old/new content, which is what a
	// pre-edit refusal leaves behind.
	const noDiffMeta = `{"hook":{"hooks":[{"name":"wakatime","event":"PreToolUse","duration_ms":12}]}}`

	tests := []struct {
		name string
		item func() ToolMessageItem
	}{
		{
			name: "edit",
			item: func() ToolMessageItem {
				return NewEditToolMessageItem(&sty,
					message.ToolCall{ID: "1", Name: "edit", Input: `{"file_path":"/tmp/x.h"}`, Finished: true},
					&message.ToolResult{ToolCallID: "1", Content: "you must read the file before editing it", IsError: true, Metadata: noDiffMeta},
					false)
			},
		},
		{
			name: "multiedit",
			item: func() ToolMessageItem {
				return NewMultiEditToolMessageItem(&sty,
					message.ToolCall{ID: "1", Name: "multiedit", Input: `{"file_path":"/tmp/x.h","edits":[]}`, Finished: true},
					&message.ToolResult{ToolCallID: "1", Content: "you must read the file before editing it", IsError: true, Metadata: noDiffMeta},
					false)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lines := strings.Split(tt.item().Render(100), "\n")
			require.NotEmpty(t, lines)

			last := lines[len(lines)-1]
			require.NotEmpty(t, strings.TrimSpace(stripANSI(last)),
				"the item ends on a blank line, which shows up as a gap below the "+
					"error with nothing in it; full render: %q", strings.Join(lines, "\n"))
		})
	}
}
