package replacer

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestBuildReplacerPrompt(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		{ID: "1", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hi"}}},
		{ID: "2", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "hello there"}}},
	}

	prompt, err := BuildReplacerPrompt(msgs)
	require.NoError(t, err)
	require.Contains(t, prompt, "Most Recent Exchange")
	require.Contains(t, prompt, "user: hi")
	require.Contains(t, prompt, "assistant: hello there")
	require.Contains(t, prompt, `"action":"stop"`)
	require.Contains(t, prompt, `"action":"continue"`)
}

func TestBuildReplacerPrompt_Empty(t *testing.T) {
	t.Parallel()

	prompt, err := BuildReplacerPrompt(nil)
	require.NoError(t, err)
	require.Contains(t, prompt, "Most Recent Exchange")
}

func TestBuildReplacerPrompt_TruncatesOldMessages(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		{ID: "1", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "old-first"}}},
		{ID: "2", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "old-second"}}},
		{ID: "3", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "old-third"}}},
		{ID: "4", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "old-fourth"}}},
		{ID: "5", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "recent-user"}}},
		{ID: "6", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "recent-assistant"}}},
	}

	prompt, err := BuildReplacerPrompt(msgs)
	require.NoError(t, err)
	require.Contains(t, prompt, "recent-user")
	require.Contains(t, prompt, "recent-assistant")
	require.NotContains(t, prompt, "old-first")
	require.NotContains(t, prompt, "old-second")
}
