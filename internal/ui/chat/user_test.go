package chat

import (
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

func TestRawRender_ChannelMessageFull(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal" sender="+15551234567" sender_name="Alice" time="14:30">Hello from Signal!</channel>`

	item := newTestUserItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	// Header should contain the channel source.
	require.Contains(t, out, "signal")

	// Header should contain the sender name.
	require.Contains(t, out, "Alice")

	// Header should contain the time from the XML attribute.
	require.Contains(t, out, "14:30")

	// Body should be rendered as markdown, not raw XML.
	require.Contains(t, out, "Hello from Signal!")

	// Raw XML tags must not be visible.
	require.NotContains(t, out, "<channel")
	require.NotContains(t, out, "</channel>")
}

func TestRawRender_ChannelMessageSourceOnly(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal">Just the source, no sender or time.</channel>`

	item := newTestUserItem(text, 1752456000) // CreatedAt fallback
	out := ansi.Strip(item.RawRender(80))

	// Header should contain the channel source.
	require.Contains(t, out, "signal")

	// Body should be present.
	require.Contains(t, out, "Just the source")

	// No raw XML tags.
	require.NotContains(t, out, "<channel")
	require.NotContains(t, out, "</channel>")
}

func TestRawRender_ChannelMessageMalformed(t *testing.T) {
	t.Parallel()

	// Incomplete/truncated XML should not panic and should fall back to
	// plain rendering.
	text := `<channel source="signal" sender="broken`

	item := newTestUserItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	// Should not panic and should contain the raw text (fallback).
	require.NotEmpty(t, out)
	require.Contains(t, out, "<channel")
}

func TestRawRender_NormalMessage(t *testing.T) {
	t.Parallel()

	text := `This is a normal user message.`

	item := newTestUserItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	require.Contains(t, out, "This is a normal user message.")
	require.NotContains(t, out, "signal")
	require.NotContains(t, out, "<channel")
}

func TestRawRender_ChannelMessageEmptyBody(t *testing.T) {
	t.Parallel()

	text := `<channel source="signal" sender="+15551234567" sender_name="Bob" time="09:15"></channel>`

	item := newTestUserItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	require.Contains(t, out, "signal")
	require.Contains(t, out, "Bob")
	require.Contains(t, out, "09:15")
	require.NotContains(t, out, "<channel")
	require.NotContains(t, out, "</channel>")
}

func TestRawRender_ChannelMessageSenderFallback(t *testing.T) {
	t.Parallel()

	// sender provided but no sender_name — should show sender as the name.
	text := `<channel source="signal" sender="+15559876543">Fallback sender</channel>`

	item := newTestUserItem(text, 0)
	out := ansi.Strip(item.RawRender(80))

	require.Contains(t, out, "signal")
	require.Contains(t, out, "+15559876543")
	require.Contains(t, out, "Fallback sender")
	require.NotContains(t, out, "<channel")
}

func TestRawRender_ChannelMessageCreatedAtFallback(t *testing.T) {
	t.Parallel()

	// No time attribute, but CreatedAt is set on the message.
	text := `<channel source="signal">Uses CreatedAt</channel>`

	item := newTestUserItem(text, 1752456000)
	out := ansi.Strip(item.RawRender(80))

	require.Contains(t, out, "signal")
	require.Contains(t, out, "Uses CreatedAt")
	require.NotContains(t, out, "<channel")
}
