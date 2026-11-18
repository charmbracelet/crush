package agent

import (
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func toolMessage(outputs ...fantasy.ToolResultOutputContent) fantasy.Message {
	msg := fantasy.Message{Role: fantasy.MessageRoleTool}
	for i, out := range outputs {
		msg.Content = append(msg.Content, fantasy.ToolResultPart{
			ToolCallID: string(rune('a' + i)),
			Output:     out,
		})
	}
	return msg
}

func outputOf(t *testing.T, part fantasy.MessagePart) fantasy.ToolResultOutputContent {
	t.Helper()
	result, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part)
	require.True(t, ok)
	return result.Output
}

func TestDropEmptyToolMediaReplacesOnlyEmptyMedia(t *testing.T) {
	t.Parallel()

	good := fantasy.ToolResultOutputContentMedia{Data: "iVBORw==", MediaType: "image/png"}
	original := toolMessage(
		fantasy.ToolResultOutputContentMedia{MediaType: "image/png", Text: "scrolled"},
		good,
		fantasy.ToolResultOutputContentText{Text: "plain"},
	)
	user := fantasy.NewUserMessage("hi")

	got := dropEmptyToolMedia([]fantasy.Message{user, original})

	require.Equal(t, user, got[0])
	require.Equal(t,
		fantasy.ToolResultOutputContentText{Text: "scrolled\n" + emptyMediaNote},
		outputOf(t, got[1].Content[0]),
	)
	require.Equal(t, good, outputOf(t, got[1].Content[1]))
	require.Equal(t, fantasy.ToolResultOutputContentText{Text: "plain"}, outputOf(t, got[1].Content[2]))
}

// The step history fantasy keeps must not be rewritten underneath it.
func TestDropEmptyToolMediaDoesNotMutateSharedParts(t *testing.T) {
	t.Parallel()

	parts := toolMessage(fantasy.ToolResultOutputContentMedia{MediaType: "image/png"}).Content
	dropEmptyToolMedia([]fantasy.Message{{Role: fantasy.MessageRoleTool, Content: parts}})

	_, stillMedia := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](outputOf(t, parts[0]))
	require.True(t, stillMedia)
}

func TestConvertToToolResult_EmptyMedia(t *testing.T) {
	t.Parallel()

	tr := (&sessionAgent{}).convertToToolResult(fantasy.ToolResultContent{
		ToolCallID: "call_789",
		ToolName:   "prickly_computer",
		Result:     fantasy.ToolResultOutputContentMedia{MediaType: "image/png", Text: "scrolled"},
	})
	require.False(t, tr.IsError, "the call succeeded, so a retry would only repeat it")
	require.Empty(t, tr.Data)
	require.Equal(t, "scrolled\n"+emptyMediaNote, tr.Content)
}
