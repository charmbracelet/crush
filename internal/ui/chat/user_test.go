package chat

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func newTestUserItem(text string, createdAt int64) *UserMessageItem {
	sty := styles.CharmtonePantera()
	msg := &message.Message{
		ID:        "test-id",
		Role:      message.User,
		CreatedAt: createdAt,
		Parts:     []message.ContentPart{message.TextContent{Text: text}},
	}
	r := attachments.NewRenderer(
		sty.Attachments.Normal,
		sty.Attachments.Deleting,
		sty.Attachments.Image,
		sty.Attachments.Text,
		sty.Attachments.Skill,
		sty.Attachments.Remove,
	)
	return NewUserMessageItem(&sty, msg, r).(*UserMessageItem)
}

// TestUserMessageItemCopyIconRenderedFocused verifies that a focused user
// message with content renders the ⎘ icon on the last content row, and
// the recorded click geometry agrees with the rendered output.
func TestUserMessageItemCopyIconRenderedFocused(t *testing.T) {
	t.Parallel()

	item := newTestUserItem("hello **world**", 0)
	item.SetFocused(true)

	const width = 40
	rendered := item.RawRender(width)
	lines := strings.Split(rendered, "\n")

	require.GreaterOrEqual(t, item.copyIconRow, 0, "focused message with content must render the copy icon")
	require.Less(t, item.copyIconRow, len(lines), "recorded icon row must be within the rendered output")
	require.Equal(t, len(lines)-1, item.copyIconRow,
		"icon row must be the last rendered line")

	iconLine := lines[item.copyIconRow]
	require.Contains(t, iconLine, "⎘", "last line must contain the copy glyph")
	plain := ansi.Strip(iconLine)
	require.Equal(t, item.copyIconColStart, len([]rune(strings.Split(plain, "⎘")[0])),
		"icon must sit at the recorded start column")
	require.Equal(t, item.copyIconColStart+1, item.copyIconColEnd,
		"the single-cell glyph must span exactly one column")
}

// TestUserMessageItemCopyIconSuppressed verifies the states where the
// icon must not render: unfocused and empty content.
func TestUserMessageItemCopyIconSuppressed(t *testing.T) {
	t.Parallel()

	// Unfocused: icon hidden.
	unfocused := newTestUserItem("hello world", 0)
	unfocused.RawRender(40)
	require.Equal(t, -1, unfocused.copyIconRow,
		"unfocused message must not render the copy icon")

	// Empty content: nothing to copy.
	empty := newTestUserItem("", 0)
	empty.SetFocused(true)
	empty.RawRender(40)
	require.Equal(t, -1, empty.copyIconRow,
		"empty message must not render the copy icon")
}

// TestUserMessageItemCopyIconStaysWithinWidth guards against the icon
// overflowing the item width: glamour pads the last content line to the
// full capped width, so the icon must overwrite trailing padding rather
// than extend the line. A line wider than the item width is clipped by the
// screen compositor, making the icon invisible and unclickable on any
// terminal narrower than maxTextWidth+2.
func TestUserMessageItemCopyIconStaysWithinWidth(t *testing.T) {
	t.Parallel()

	for _, width := range []int{30, 80, 120} {
		item := newTestUserItem("hello **world** with a longer line of text that wraps", 0)
		item.SetFocused(true)
		rendered := item.RawRender(width)

		capped := cappedMessageWidth(width)
		for i, line := range strings.Split(rendered, "\n") {
			require.LessOrEqual(t, lipgloss.Width(line), capped,
				"line %d must not exceed the capped content width at width %d", i, width)
		}
		require.Contains(t, rendered, "⎘", "icon must still render")
		require.Less(t, item.copyIconColEnd, width,
			"icon must end within the item width so the mouse can reach it")
	}
}

// TestUserMessageItemCopyIconClick verifies the click contract: a click
// on the copy glyph must be handled and return the copy command.
func TestUserMessageItemCopyIconClick(t *testing.T) {
	t.Parallel()

	item := newTestUserItem("copy my **prompt**", 0)
	item.SetFocused(true)
	item.RawRender(40)
	require.GreaterOrEqual(t, item.copyIconRow, 0)

	// Click on the glyph: x is chat-relative, so offset by MessageLeftPaddingTotal.
	handled, cmd := item.HandleMouseClickCmd(ansi.MouseLeft,
		item.copyIconColStart+MessageLeftPaddingTotal, item.copyIconRow)
	require.True(t, handled, "click on the copy glyph must be handled")
	require.NotNil(t, cmd, "click on the copy glyph must produce the copy command")

	// Click to the left of the glyph on the same row is not a copy.
	handled, cmd = item.HandleMouseClickCmd(ansi.MouseLeft, 0, item.copyIconRow)
	require.False(t, handled, "click outside the glyph span must not copy")
	require.Nil(t, cmd)

	// Right button on the glyph is ignored.
	handled, cmd = item.HandleMouseClickCmd(ansi.MouseRight,
		item.copyIconColStart+MessageLeftPaddingTotal, item.copyIconRow)
	require.False(t, handled)
	require.Nil(t, cmd)

	// The plain MouseClickable path must agree.
	require.True(t, item.HandleMouseClick(ansi.MouseLeft,
		item.copyIconColStart+MessageLeftPaddingTotal, item.copyIconRow))
}
