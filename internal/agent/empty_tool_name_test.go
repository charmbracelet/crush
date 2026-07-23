package agent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

type emptyToolNameStreamModel struct {
	calls atomic.Int64
}

func (m *emptyToolNameStreamModel) Provider() string { return "fake" }
func (m *emptyToolNameStreamModel) Model() string    { return "fake-model" }

func (m *emptyToolNameStreamModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not implemented")
}

func (m *emptyToolNameStreamModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	if m.calls.Add(1) > 1 {
		return func(yield func(fantasy.StreamPart) bool) {
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
		}, nil
	}

	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{
			Type:         fantasy.StreamPartTypeToolInputStart,
			ID:           "call-empty-name",
			ToolCallName: "",
		}) {
			return
		}
		if !yield(fantasy.StreamPart{
			Type:          fantasy.StreamPartTypeToolCall,
			ID:            "call-empty-name",
			ToolCallName:  "",
			ToolCallInput: "{}",
		}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
	}, nil
}

func (m *emptyToolNameStreamModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *emptyToolNameStreamModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func TestRun_EmptyToolNameDoesNotCorruptSession(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	large := &emptyToolNameStreamModel{}
	small := &finishStreamModel{text: "title"}
	agent := testSessionAgent(env, large, small, "system").(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	_, err = agent.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "Use a tool",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), large.calls.Load(), "malformed call must not be replayed to the provider")

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	for _, msg := range msgs {
		for _, toolCall := range msg.ToolCalls() {
			require.NotEmpty(t, toolCall.Name, "empty tool name must not be persisted")
		}
		for _, toolResult := range msg.ToolResults() {
			require.NotEqual(t, "call-empty-name", toolResult.ToolCallID, "result for malformed call must not be persisted")
		}
	}

	require.Equal(t, message.FinishReasonError, msgs[len(msgs)-1].FinishReason())
	require.Contains(t, msgs[len(msgs)-1].FinishPart().Details, "without a name")

	_, err = agent.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "Continue after the malformed call",
	})
	require.NoError(t, err, "the next turn must remain usable")
	require.Equal(t, int64(2), large.calls.Load())
}
