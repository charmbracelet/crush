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
	require.Contains(t, prompt, "Conversation History")
	require.Contains(t, prompt, "user: hi")
	require.Contains(t, prompt, "assistant: hello there")
	require.Contains(t, prompt, `"action":"stop"`)
	require.Contains(t, prompt, `"action":"continue"`)
}

func TestBuildReplacerPrompt_Empty(t *testing.T) {
	t.Parallel()

	prompt, err := BuildReplacerPrompt(nil)
	require.NoError(t, err)
	require.Contains(t, prompt, "Conversation History")
}
