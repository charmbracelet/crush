package model

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// newSelectionTestChat renders a single assistant message and returns the
// chat along with the viewport position of the first character of text.
func newSelectionTestChat(t *testing.T, text string) (*Chat, int, int) {
	t.Helper()

	u := newTestUI()
	msg := &message.Message{
		ID:    "m-select",
		Role:  message.Assistant,
		Parts: []message.ContentPart{message.TextContent{Text: text}},
	}
	u.chat.SetSize(80, 20)
	u.chat.SetMessages(chat.NewAssistantMessageItem(u.com.Styles, msg))

	for y, line := range strings.Split(u.chat.list.Render(), "\n") {
		if x := strings.Index(ansi.Strip(line), text); x >= 0 {
			return u.chat, x, y
		}
	}
	t.Fatalf("text %q not found in rendered chat", text)
	return nil, 0, 0
}

// selectedText renders a frame, as the UI does before copying, so the
// highlight range is applied to the items, then returns the selection.
func selectedText(m *Chat) string {
	m.list.Render()
	return m.HighlightContent()
}

func TestChatDragSelectionIncludesCellUnderCursor(t *testing.T) {
	t.Parallel()

	m, x, y := newSelectionTestChat(t, "Hello World")

	m.HandleMouseDown(x, y)
	m.HandleMouseDrag(x+4, y)
	m.HandleMouseUp(x+4, y)

	require.Equal(t, "Hello", selectedText(m))
}

func TestChatBackwardDragSelectionIncludesAnchorCell(t *testing.T) {
	t.Parallel()

	m, x, y := newSelectionTestChat(t, "Hello World")

	m.HandleMouseDown(x+4, y)
	m.HandleMouseDrag(x, y)
	m.HandleMouseUp(x, y)

	require.Equal(t, "Hello", selectedText(m))
}

func TestChatClickWithoutDragHasNoHighlight(t *testing.T) {
	t.Parallel()

	m, x, y := newSelectionTestChat(t, "Hello World")

	m.HandleMouseDown(x, y)
	m.HandleMouseUp(x, y)

	require.Empty(t, selectedText(m))
	require.False(t, m.HasHighlight())
}

func TestChatDoubleClickSelectsWord(t *testing.T) {
	t.Parallel()

	m, x, y := newSelectionTestChat(t, "Hello World")

	m.HandleMouseDown(x+7, y)
	m.HandleMouseUp(x+7, y)
	m.HandleMouseDown(x+7, y)
	m.HandleMouseUp(x+7, y)

	require.Equal(t, "World", selectedText(m))
}
