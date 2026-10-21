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

func newTestUserItem(t *testing.T, msg *message.Message) *UserMessageItem {
	t.Helper()

	sty := styles.CharmtonePantera()
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

func userMessageWithTextAttachment(t *testing.T, path, content string) *message.Message {
	t.Helper()

	return &message.Message{
		ID:   "u1",
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "See the paste below."},
			message.BinaryContent{
				Path:     path,
				MIMEType: "text/plain; charset=utf-8",
				Data:     []byte(content),
			},
		},
	}
}

// TestUserMessageItem_ToggleExpandedWithTextAttachment ensures pasted
// text attachments expand and collapse, clearing the render cache so
// the transcript updates.
func TestUserMessageItem_ToggleExpandedWithTextAttachment(t *testing.T) {
	t.Parallel()

	item := newTestUserItem(t, userMessageWithTextAttachment(t, "paste_1.txt", "hello world"))
	require.False(t, item.expandedContent, "should start collapsed")

	// Populate the cache so the clearCache assertion below is meaningful.
	item.RawRender(80)
	require.NotEqual(t, "", item.rendered, "precondition: cache should be populated")

	require.True(t, item.ToggleExpanded(), "first toggle should expand")
	require.True(t, item.expandedContent)
	require.Equal(t, "", item.rendered, "expansion must clear the render cache")
	require.NotEqual(t, "", item.RawRender(80), "expanded render should repopulate the cache")

	require.False(t, item.ToggleExpanded(), "second toggle should collapse")
	require.False(t, item.expandedContent)
	require.Equal(t, "", item.rendered, "collapse must clear the render cache")
}

// TestUserMessageItem_ToggleExpandedNoTextAttachment ensures messages
// without text attachments are not expandable.
func TestUserMessageItem_ToggleExpandedNoTextAttachment(t *testing.T) {
	t.Parallel()

	msg := &message.Message{
		ID:   "u1",
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "plain"},
			message.BinaryContent{Path: "image.png", MIMEType: "image/png", Data: []byte{1, 2, 3}},
		},
	}
	item := newTestUserItem(t, msg)

	require.False(t, item.ToggleExpanded(), "image-only messages must not expand")
	require.False(t, item.expandedContent)
}

// TestUserMessageItem_HandleMouseClick ensures only left clicks on
// messages with text attachments signal a handled click.
func TestUserMessageItem_HandleMouseClick(t *testing.T) {
	t.Parallel()

	textItem := newTestUserItem(t, userMessageWithTextAttachment(t, "paste_1.txt", "hello"))
	require.True(t, textItem.HandleMouseClick(ansi.MouseLeft, 0, 0))
	require.False(t, textItem.HandleMouseClick(ansi.MouseRight, 0, 0))

	imageItem := newTestUserItem(t, &message.Message{
		ID:   "u2",
		Role: message.User,
		Parts: []message.ContentPart{
			message.BinaryContent{Path: "image.png", MIMEType: "image/png", Data: []byte{1}},
		},
	})
	require.False(t, imageItem.HandleMouseClick(ansi.MouseLeft, 0, 0))
}

// TestUserMessageItem_RenderTextAttachmentContent ensures collapsed
// renders a truncated preview with an expand hint and expanded renders
// the full pasted content.
func TestUserMessageItem_RenderTextAttachmentContent(t *testing.T) {
	t.Parallel()

	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "line " + string(rune('a'+i%26))
	}
	content := strings.Join(lines, "\n")

	item := newTestUserItem(t, userMessageWithTextAttachment(t, "paste_1.txt", content))

	collapsed := item.renderTextAttachmentContent(80)
	require.Contains(t, collapsed, "paste_1.txt", "collapsed render should keep the filename")
	require.Contains(t, collapsed, "lines hidden", "collapsed render should advertise hidden lines")

	item.expandedContent = true
	expanded := item.renderTextAttachmentContent(80)
	require.Contains(t, expanded, "line a", "expanded render should include the start of the paste")
	require.Contains(t, expanded, "line y", "expanded render should include the end of the paste")
	require.NotContains(t, expanded, "lines hidden", "expanded render should not advertise hidden lines")
}

// TestUserMessageItem_RenderMultipleTextAttachments ensures multiple
// pasted attachments render as separate blocks, each with its filename.
func TestUserMessageItem_RenderMultipleTextAttachments(t *testing.T) {
	t.Parallel()

	msg := &message.Message{
		ID:   "u1",
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "two pastes"},
			message.BinaryContent{Path: "paste_1.txt", MIMEType: "text/plain", Data: []byte("first")},
			message.BinaryContent{Path: "paste_2.txt", MIMEType: "text/plain", Data: []byte("second")},
			message.BinaryContent{Path: "image.png", MIMEType: "image/png", Data: []byte{1}},
		},
	}
	item := newTestUserItem(t, msg)
	item.expandedContent = true

	out := item.renderTextAttachmentContent(80)
	require.Contains(t, out, "paste_1.txt")
	require.Contains(t, out, "first")
	require.Contains(t, out, "paste_2.txt")
	require.Contains(t, out, "second")
	require.NotContains(t, out, "image.png", "image attachments should stay pills only")
}

// TestUserMessageItem_RenderPastedContentNoText checks that the full
// RawRender includes the pasted content when expanded and omits it (only
// the pills) when collapsed for short content under the truncation cap.
func TestUserMessageItem_RenderPastedContentNoText(t *testing.T) {
	t.Parallel()

	msg := &message.Message{
		ID:   "u1",
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "See below."},
			message.BinaryContent{Path: "paste_1.txt", MIMEType: "text/plain", Data: []byte("tiny paste")},
		},
	}
	item := newTestUserItem(t, msg)

	collapsed := item.RawRender(80)
	require.Contains(t, collapsed, "paste_1.txt")

	item.expandedContent = true
	expanded := item.RawRender(80)
	require.Contains(t, expanded, "tiny paste", "expanded render should surface the pasted text")
	_ = lipgloss.Width(expanded)
}
