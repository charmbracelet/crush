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

func newTestChannelInfoItem(text string, createdAt int64) *ChannelInfoItem {
	sty := styles.CharmtonePantera()
	msg := &message.Message{
		ID:        "test-id",
		Role:      message.User,
		CreatedAt: createdAt,
		Parts:     []message.ContentPart{message.TextContent{Text: text}},
	}
	return NewChannelInfoItem(&sty, msg).(*ChannelInfoItem)
}

// --- UserMessageItem body tests (body only, no metadata) ---

// TestRawRender_ChannelMessageBody verifies that the UserMessageItem for a
// channel message renders only the body content, not the metadata line.
func TestRawRender_ChannelMessageBody(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal" sender="+15551234567" sender_name="Alice" time="14:30">Hello from Signal!</channel>`

	item := newTestUserItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	// Body should be rendered as markdown, not raw XML.
	require.Contains(t, out, "Hello from Signal!")

	// Raw XML tags must not be visible.
	require.NotContains(t, out, "<channel")
	require.NotContains(t, out, "</channel>")

	// Metadata must NOT appear in the body — it is rendered by ChannelInfoItem.
	require.NotContains(t, out, "via signal")
	require.NotContains(t, out, "at 14:30")
}

// TestRawRender_ChannelMessageSourceOnlyBody verifies that a message with
// only a source renders just the body in UserMessageItem.
func TestRawRender_ChannelMessageSourceOnlyBody(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal">Just the source, no sender or time.</channel>`

	item := newTestUserItem(text, 1752456000)
	out := ansi.Strip(item.RawRender(80))

	require.Contains(t, out, "Just the source")
	require.NotContains(t, out, "<channel")
	require.NotContains(t, out, "</channel>")
	require.NotContains(t, out, "via signal")
}

// TestRawRender_ChannelMessageMalformed verifies that truncated/invalid XML
// does not panic and falls back to plain markdown rendering.
func TestRawRender_ChannelMessageMalformed(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal" sender="broken`

	item := newTestUserItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	require.NotEmpty(t, out)
	require.Contains(t, out, "<channel")
}

// TestRawRender_NormalMessage verifies that non-channel messages are not
// affected by channel rendering logic.
func TestRawRender_NormalMessage(t *testing.T) {
	t.Parallel()

	text := `This is a normal user message.`

	item := newTestUserItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	require.Contains(t, out, "This is a normal user message.")
	require.NotContains(t, out, "signal")
	require.NotContains(t, out, "via")
	require.NotContains(t, out, "<channel")
}

// TestRawRender_ChannelMessageEmptyBody verifies that a channel message with
// no body renders an empty string in UserMessageItem.
func TestRawRender_ChannelMessageEmptyBody(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal" sender="+15551234567" sender_name="Bob" time="09:15"></channel>`

	item := newTestUserItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	require.NotContains(t, out, "<channel")
	require.NotContains(t, out, "</channel>")
	require.NotContains(t, out, "via signal")
}

// TestRawRender_ChannelMessageSenderFallbackBody verifies that the body
// renders correctly when sender is provided but sender_name is not.
func TestRawRender_ChannelMessageSenderFallbackBody(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal" sender="+15559876543">Fallback sender</channel>`

	item := newTestUserItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	require.Contains(t, out, "Fallback sender")
	require.NotContains(t, out, "<channel")
}

// --- ChannelInfoItem metadata tests ---

// TestChannelInfo_Full verifies that the ChannelInfoItem renders the full
// metadata line: "[icon] [sender] via [channel] at [timestamp]".
func TestChannelInfo_Full(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal" sender="+15551234567" sender_name="Alice" time="14:30">Hello!</channel>`

	item := newTestChannelInfoItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	// Must contain sender, channel source, and timestamp.
	require.Contains(t, out, "Alice")
	require.Contains(t, out, "via signal")
	require.Contains(t, out, "at 14:30")

	// Must contain the chat-bubble glyph icon.
	require.Contains(t, out, styles.ChannelIcon)

	// Must NOT contain the body content — that's in UserMessageItem.
	require.NotContains(t, out, "Hello!")

	// Must NOT contain raw XML.
	require.NotContains(t, out, "<channel")
	require.NotContains(t, out, "</channel>")
}

// TestChannelInfo_SenderFallback verifies that when sender is provided but
// sender_name is not, the raw sender value is shown.
func TestChannelInfo_SenderFallback(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal" sender="+15559876543">Body text</channel>`

	item := newTestChannelInfoItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	require.Contains(t, out, "+15559876543")
	require.Contains(t, out, "via signal")
}

// TestChannelInfo_CreatedAtFallback verifies that when no time attribute is
// present, the message's CreatedAt timestamp is used.
func TestChannelInfo_CreatedAtFallback(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal">Uses CreatedAt</channel>`

	item := newTestChannelInfoItem(text, 1752456000)
	out := ansi.Strip(item.RawRender(80))

	require.Contains(t, out, "via signal")
	require.Contains(t, out, "at ")
}

// TestChannelInfo_NoSource verifies that when no source attribute is present,
// the "via" clause is omitted but metadata still renders without panicking.
func TestChannelInfo_NoSource(t *testing.T) {
	t.Parallel()

	text := `<channel sender="+1234" sender_name="Eve" time="08:00">No source</channel>`

	item := newTestChannelInfoItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	require.Contains(t, out, "Eve")
	require.Contains(t, out, "at 08:00")
	require.NotContains(t, out, "via")
}

// TestChannelInfo_NoMetadata verifies that when only the body is present with
// no metadata attributes, the ChannelInfoItem renders an empty string.
func TestChannelInfo_NoMetadata(t *testing.T) {
	t.Parallel()

	text := `<channel>Just a body, nothing else.</channel>`

	item := newTestChannelInfoItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	// With no metadata, the info item should be empty.
	require.Empty(t, out)
}

// TestChannelInfo_MalformedXML verifies that malformed XML does not panic.
func TestChannelInfo_MalformedXML(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal" sender="broken`

	item := newTestChannelInfoItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	// Should not panic and should be empty (XML parse fails).
	require.Empty(t, out)
}

// TestChannelInfo_RenderHasSectionHeader verifies that the Render method
// applies the SectionHeader padding ( paddingLeft(2) ), confirming the
// metadata appears as a separate item outside the message body.
func TestChannelInfo_RenderHasSectionHeader(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal" sender_name="Alice" time="14:30">Hi</channel>`

	item := newTestChannelInfoItem(text, 0)
	out := ansi.Strip(item.Render(80))

	// The rendered output should start with at least 2 spaces (paddingLeft(2)).
	require.True(t, strings.HasPrefix(out, "  "), "ChannelInfoItem.Render must apply SectionHeader padding")
}

// --- UserMessageItem click-to-copy tests ---

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
// icon must not render: unfocused, empty content, and channel messages.
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
