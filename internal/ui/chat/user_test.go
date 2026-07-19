package chat

import (
	"strings"
	"testing"

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
