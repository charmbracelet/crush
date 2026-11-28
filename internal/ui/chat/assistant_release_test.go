package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// A finished message keeps only its section caches: the streaming-markdown
// caches, which hold full renders of the whole thinking trace and content,
// are dropped once the sections are stored. Later renders still come from
// the section caches, and expanding or resizing renders the same bytes as a
// fresh item.
func TestAssistantFinishedReleasesStreamingCaches(t *testing.T) {
	t.Parallel()
	sty := styles.CharmtonePantera()
	const width = 83

	var thinking, content strings.Builder
	for i := range 60 {
		thinking.WriteString("Reasoning step number ")
		thinking.WriteString(strings.Repeat("x", i%7+1))
		thinking.WriteString(".\n\n")
		content.WriteString("Answer paragraph with some words.\n\n")
	}
	build := func(thinking, content string, finished bool) *message.Message {
		msg := thinkingMessage("rel", thinking, content)
		if finished {
			msg.Parts = append(msg.Parts, message.Finish{Reason: message.FinishReasonEndTurn, Time: testFinishTime})
		}
		return msg
	}

	// Stream the message in so the stable-prefix caches get seeded.
	th, ct := thinking.String(), content.String()
	item := NewAssistantMessageItem(&sty, build(th[:10], "", false)).(*AssistantMessageItem)
	for n := 200; n < len(th); n += 400 {
		item.SetMessage(build(th[:n], "", false))
		item.RawRender(width)
	}
	for n := 200; n < len(ct); n += 400 {
		item.SetMessage(build(th, ct[:n], false))
		item.RawRender(width)
	}
	item.SetMessage(build(th, ct, false))
	item.RawRender(width)
	require.NotEmpty(t, item.streamingContent.stablePrefixRender)

	item.SetMessage(build(th, ct, true))
	out := item.RawRender(width)
	require.Empty(t, item.streamingThinking.lastRender)
	require.Empty(t, item.streamingThinking.stablePrefixRender)
	require.Empty(t, item.streamingContent.lastRender)
	require.Empty(t, item.streamingContent.stablePrefixRender)

	before := snapshot(item)
	require.Equal(t, out, item.RawRender(width))
	require.Equal(t, before, snapshot(item))
	require.Empty(t, item.streamingThinking.lastRender, "a hit must not re-render")

	fresh := NewAssistantMessageItem(&sty, build(th, ct, true)).(*AssistantMessageItem)
	require.Equal(t, fresh.RawRender(width), out)

	item.ToggleExpanded()
	fresh.ToggleExpanded()
	require.Equal(t, fresh.RawRender(width), item.RawRender(width))
	require.Equal(t, fresh.RawRender(width-10), item.RawRender(width-10))
}

// While the message streams, the caches stay so each flush stays
// incremental.
func TestAssistantStreamingKeepsStreamingCaches(t *testing.T) {
	t.Parallel()
	sty := styles.CharmtonePantera()
	text := strings.Repeat("Answer paragraph with some words.\n\n", 40)
	msg := thinkingMessage("keep", "", text[:len(text)/2])
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	item.RawRender(80)
	item.SetMessage(thinkingMessage("keep", "", text))
	item.RawRender(80)
	require.NotEmpty(t, item.streamingContent.lastRender)
}
