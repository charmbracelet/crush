package chat

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestAssistantMessageItemExpandable guards the Expandable contract on
// AssistantMessageItem. The earlier implementation returned no value, which
// meant the type silently did not satisfy chat.Expandable and the
// keyboard-driven expand path in model/chat.go skipped thinking blocks.
func TestAssistantMessageItemExpandable(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := &message.Message{ID: "m1", Role: message.Assistant}
	item := NewAssistantMessageItem(&sty, msg)

	exp, ok := item.(Expandable)
	require.True(t, ok, "AssistantMessageItem must satisfy Expandable")

	require.True(t, exp.ToggleExpanded(), "first toggle should report expanded")
	require.False(t, exp.ToggleExpanded(), "second toggle should report collapsed")
}

// TestAssistantMessageItemHandleMouseClick ensures HandleMouseClick does not
// toggle expansion on its own. The generic Expandable path in
// model/chat.go does the toggle; doing it here too would double-toggle and
// net to no change.
func TestAssistantMessageItemHandleMouseClick(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := &message.Message{ID: "m2", Role: message.Assistant}
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	item.thinkingBoxHeight = 5

	// Click inside the thinking box signals handled but must not mutate
	// the expanded state.
	require.True(t, item.HandleMouseClick(ansi.MouseLeft, 0, 2))
	require.False(t, item.thinkingExpanded, "HandleMouseClick must not toggle expansion on its own")

	// Click outside the thinking box is ignored entirely.
	require.False(t, item.HandleMouseClick(ansi.MouseLeft, 0, 10))
	require.False(t, item.thinkingExpanded)

	// Non-left button is ignored.
	require.False(t, item.HandleMouseClick(ansi.MouseRight, 0, 2))
	require.False(t, item.thinkingExpanded)
}
