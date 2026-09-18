package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// newTestUserItem builds a UserMessageItem carrying text.
func newTestUserItem(t *testing.T, text string) *UserMessageItem {
	t.Helper()
	sty := styles.CharmtonePantera()
	msg := &message.Message{
		ID:    "user-1",
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: text}},
	}
	item := NewUserMessageItem(&sty, msg, nil)
	userItem, ok := item.(*UserMessageItem)
	require.True(t, ok, "NewUserMessageItem must return *UserMessageItem")
	return userItem
}

// renderedLines returns the rendered message as trimmed, non-empty-aware
// lines so assertions can talk about visual line structure without
// depending on ANSI styling or trailing pad.
func renderedLines(t *testing.T, text string, width int) []string {
	t.Helper()
	out := newTestUserItem(t, text).RawRender(width)
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(ansi.Strip(l))
	}
	return lines
}

// TestUserMessagePreservesSingleLineBreaks is the regression test for
// charmbracelet/crush#3502: a user submitting
//
//	a
//	b
//
//	c
//
// saw "a" and "b" collapsed onto one line ("ab"/"a b") in the chat
// display, because user input was rendered through the standard
// Markdown renderer where a lone newline is a soft break. The history
// sent to the model was always correct; only the display was wrong.
func TestUserMessagePreservesSingleLineBreaks(t *testing.T) {
	t.Parallel()

	lines := renderedLines(t, "a\nb\n\nc", 80)

	var got []string
	for _, l := range lines {
		if l != "" {
			got = append(got, l)
		}
	}
	require.Equal(t, []string{"a", "b", "c"}, got,
		"each typed line must render on its own visual line")

	// The blank line the user typed between "b" and "c" must survive as a
	// paragraph break, so this is not merely "everything got hard-wrapped".
	joined := strings.Join(lines, "\n")
	require.Contains(t, joined, "b\n\nc",
		"the blank line between paragraphs must be preserved")
}

// TestUserMessageSoftWrapsLongLines guards the other direction: a
// single long line that exceeds the render width must still soft wrap.
// Preserving *authored* newlines must not disable wrapping.
func TestUserMessageSoftWrapsLongLines(t *testing.T) {
	t.Parallel()

	long := "this is a single very long line of user input that comfortably exceeds the render width and therefore has to be soft wrapped by the renderer"
	lines := renderedLines(t, long, 40)

	var nonEmpty int
	for _, l := range lines {
		if l != "" {
			nonEmpty++
		}
		require.LessOrEqual(t, len(l), 40,
			"no rendered line may exceed the render width")
	}
	require.Greater(t, nonEmpty, 1,
		"a line longer than the width must wrap onto multiple lines")
}

// TestUserMessageMarkdownConstructsUnaffected pins the blast radius of
// the #3502 fix: preserving newlines must not disturb block-level
// Markdown that users legitimately paste into the prompt.
func TestUserMessageMarkdownConstructsUnaffected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "bullet list keeps one item per line",
			input: "- one\n- two\n- three",
			want:  []string{"one", "two", "three"},
		},
		{
			name:  "numbered list keeps one item per line",
			input: "1. first\n2. second",
			want:  []string{"first", "second"},
		},
		{
			name:  "fenced code block keeps its lines",
			input: "```go\nfunc main() {\n\tx := 1\n}\n```",
			want:  []string{"func main() {", "x := 1", "}"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			joined := strings.Join(renderedLines(t, tt.input, 80), "\n")
			for _, w := range tt.want {
				require.Contains(t, joined, w)
			}
		})
	}
}
