package agent

import (
	"errors"
	"strings"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestContextOverflowClassification(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"reported provider context", &fantasy.ProviderError{ContextTooLargeErr: true}, true},
		{"frontier wording", &fantasy.ProviderError{StatusCode: 400, Message: "Your input exceeds the context window of this model. Please adjust your input and try again."}, true},
		{"SSE code", &fantasy.Error{Message: "context_length_exceeded"}, true},
		{"unknown validation", &fantasy.ProviderError{StatusCode: 400, Message: "Invalid request"}, false},
		{"transport", errors.New("unexpected EOF"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isContextOverflow(tc.err))
		})
	}
}

func TestSummaryChunks(t *testing.T) {
	t.Parallel()
	messages := []fantasy.Message{
		fantasy.NewUserMessage(strings.Repeat("a", 1800)),
		fantasy.NewUserMessage(strings.Repeat("b", 1800)),
		fantasy.NewUserMessage("latest task"),
	}
	chunks, err := summaryChunks(messages, 2048)
	require.NoError(t, err)
	require.Len(t, chunks, 3)
	require.Equal(t, messages[0], chunks[0][0])
	require.Equal(t, messages[2], chunks[2][0])

	_, err = summaryChunks([]fantasy.Message{fantasy.NewUserMessage(strings.Repeat("x", 5000))}, 2048)
	require.ErrorIs(t, err, errContextBudget)

	pair := []fantasy.Message{
		fantasy.NewUserMessage(strings.Repeat("a", 1800)),
		{Role: fantasy.MessageRoleAssistant, Content: []fantasy.MessagePart{fantasy.ToolCallPart{ToolCallID: "one", ToolName: "read", Input: "{}"}}},
		{Role: fantasy.MessageRoleTool, Content: []fantasy.MessagePart{fantasy.ToolResultPart{ToolCallID: "one", Output: fantasy.ToolResultOutputContentText{Text: "result"}}}},
	}
	chunks, err = summaryChunks(pair, 2048)
	require.NoError(t, err)
	require.Len(t, chunks, 2)
	require.Len(t, chunks[1], 2)
}

func TestContextBudgetExceeded(t *testing.T) {
	t.Parallel()
	model := Model{CatwalkCfg: catwalk.Model{ContextWindow: 128_000, DefaultMaxTokens: 4096}}
	require.False(t, contextBudgetExceeded(model, "system", "", nil, "hi", nil, nil, 0))
	largeOutput := strings.Repeat("word ", 52_000)
	require.True(t, contextBudgetExceeded(model, "system", "", []fantasy.Message{fantasy.NewUserMessage(largeOutput)}, "hi", nil, nil, 0))
	model.CatwalkCfg.ContextWindow = 0
	require.False(t, contextBudgetExceeded(model, "system", "", []fantasy.Message{fantasy.NewUserMessage(largeOutput)}, "hi", nil, nil, 0))
}
