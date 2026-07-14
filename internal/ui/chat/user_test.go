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

// TestRawRender_ChannelMessageFull verifies the happy path: a channel message
// with all attributes renders the body first, then a metadata line below in
// "[sender] via [channel] at [timestamp]" format.
func TestRawRender_ChannelMessageFull(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal" sender="+15551234567" sender_name="Alice" time="14:30">Hello from Signal!</channel>`

	item := newTestUserItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	// Metadata should contain the sender name.
	require.Contains(t, out, "Alice")

	// Metadata should contain "via" and the channel source.
	require.Contains(t, out, "via signal")

	// Metadata should contain "at" and the time from the XML attribute.
	require.Contains(t, out, "at 14:30")

	// Body should be rendered as markdown, not raw XML.
	require.Contains(t, out, "Hello from Signal!")

	// Raw XML tags must not be visible.
	require.NotContains(t, out, "<channel")
	require.NotContains(t, out, "</channel>")

	// Metadata must appear below the body.
	bodyIdx := strings.Index(out, "Hello from Signal!")
	metaIdx := strings.Index(out, "via signal")
	require.Greater(t, bodyIdx, -1, "body must be present")
	require.Greater(t, metaIdx, -1, "metadata must be present")
	require.Less(t, bodyIdx, metaIdx, "body must appear before metadata")
}

// TestRawRender_ChannelMessageSourceOnly verifies that a message with only a
// source (no sender, no time) still renders the body and shows the channel
// name in the metadata.
func TestRawRender_ChannelMessageSourceOnly(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal">Just the source, no sender or time.</channel>`

	item := newTestUserItem(text, 1752456000) // CreatedAt fallback
	out := ansi.Strip(item.RawRender(80))

	// Metadata should contain "via" and the channel source.
	require.Contains(t, out, "via signal")

	// Body should be present.
	require.Contains(t, out, "Just the source")

	// No raw XML tags.
	require.NotContains(t, out, "<channel")
	require.NotContains(t, out, "</channel>")
}

// TestRawRender_ChannelMessageMalformed verifies that truncated/invalid XML
// does not panic and falls back to plain markdown rendering.
func TestRawRender_ChannelMessageMalformed(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal" sender="broken`

	item := newTestUserItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	// Should not panic and should contain the raw text (fallback).
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
// no body content still renders the metadata line.
func TestRawRender_ChannelMessageEmptyBody(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal" sender="+15551234567" sender_name="Bob" time="09:15"></channel>`

	item := newTestUserItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	require.Contains(t, out, "Bob")
	require.Contains(t, out, "via signal")
	require.Contains(t, out, "at 09:15")
	require.NotContains(t, out, "<channel")
	require.NotContains(t, out, "</channel>")
}

// TestRawRender_ChannelMessageSenderFallback verifies that when sender is
// provided but sender_name is not, the raw sender value is shown.
func TestRawRender_ChannelMessageSenderFallback(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal" sender="+15559876543">Fallback sender</channel>`

	item := newTestUserItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	require.Contains(t, out, "+15559876543")
	require.Contains(t, out, "via signal")
	require.Contains(t, out, "Fallback sender")
	require.NotContains(t, out, "<channel")
}

// TestRawRender_ChannelMessageCreatedAtFallback verifies that when no time
// attribute is present, the message's CreatedAt timestamp is used.
func TestRawRender_ChannelMessageCreatedAtFallback(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal">Uses CreatedAt</channel>`

	item := newTestUserItem(text, 1752456000)
	out := ansi.Strip(item.RawRender(80))

	require.Contains(t, out, "via signal")
	require.Contains(t, out, "Uses CreatedAt")
	require.Contains(t, out, "at ") // CreatedAt-formatted time
	require.NotContains(t, out, "<channel")
}

// TestRawRender_ChannelMessageNoSource verifies the unhappy path where the
// channel element has no source attribute — metadata should omit the "via"
// clause but still render without panicking.
func TestRawRender_ChannelMessageNoSource(t *testing.T) {
	t.Parallel()

	text := `<channel sender="+1234" sender_name="Eve" time="08:00">No source attr</channel>`

	item := newTestUserItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	require.Contains(t, out, "Eve")
	require.Contains(t, out, "at 08:00")
	require.Contains(t, out, "No source attr")
	require.NotContains(t, out, "via")
	require.NotContains(t, out, "<channel")
}

// TestRawRender_ChannelMessageNoMetadata verifies the unhappy path where only
// the body is present with no metadata attributes — the body should still
// render and no metadata parts should be shown.
func TestRawRender_ChannelMessageNoMetadata(t *testing.T) {
	t.Parallel()

	text := `<channel>Just a body, nothing else.</channel>`

	item := newTestUserItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	require.Contains(t, out, "Just a body, nothing else.")
	require.NotContains(t, out, "via")
	require.NotContains(t, out, "at ")
	require.NotContains(t, out, "<channel")
	// With no metadata, there must be no lone Section separator line under the body.
	require.NotContains(t, out, styles.SectionSeparator, "no separator line when there is no metadata")
}

// TestRawRender_ChannelMessageMetadataFormat verifies the exact format of the
// metadata line: "[sender] via [channel] at [timestamp]" with the body above.
func TestRawRender_ChannelMessageMetadataFormat(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal" sender="+15551234567" sender_name="Alice" time="14:30">Hello!</channel>`

	item := newTestUserItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	// The metadata section must contain the full formatted string.
	require.Contains(t, out, "Alice via signal at 14:30")

	// The old "·" separator style must not be used.
	require.NotContains(t, out, "·")
}
