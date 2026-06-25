package agent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const duplicateToolCallID = "dup_id_0001"

type duplicateToolCallModel struct {
	mu                  sync.Mutex
	calls               int
	followupToolCalls   int
	followupToolResults int
}

func (m *duplicateToolCallModel) Provider() string { return "fake" }
func (m *duplicateToolCallModel) Model() string    { return "fake-model" }

func (m *duplicateToolCallModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not implemented")
}

func (m *duplicateToolCallModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	m.calls++
	callNumber := m.calls
	if callNumber == 2 {
		for _, msg := range call.Prompt {
			for _, part := range msg.Content {
				if tc, ok := fantasy.AsMessagePart[fantasy.ToolCallPart](part); ok && tc.ToolCallID == duplicateToolCallID {
					m.followupToolCalls++
				}
				if tr, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part); ok && tr.ToolCallID == duplicateToolCallID {
					m.followupToolResults++
				}
			}
		}
	}
	m.mu.Unlock()

	return func(yield func(fantasy.StreamPart) bool) {
		if callNumber == 1 {
			for range 2 {
				if !yield(fantasy.StreamPart{
					Type:         fantasy.StreamPartTypeToolInputStart,
					ID:           duplicateToolCallID,
					ToolCallName: "counted",
				}) {
					return
				}
				if !yield(fantasy.StreamPart{
					Type:  fantasy.StreamPartTypeToolInputDelta,
					ID:    duplicateToolCallID,
					Delta: `{"value":"same"}`,
				}) {
					return
				}
				if !yield(fantasy.StreamPart{
					Type: fantasy.StreamPartTypeToolInputEnd,
					ID:   duplicateToolCallID,
				}) {
					return
				}
				if !yield(fantasy.StreamPart{
					Type:          fantasy.StreamPartTypeToolCall,
					ID:            duplicateToolCallID,
					ToolCallName:  "counted",
					ToolCallInput: `{"value":"same"}`,
				}) {
					return
				}
			}
			yield(fantasy.StreamPart{
				Type:         fantasy.StreamPartTypeFinish,
				FinishReason: fantasy.FinishReasonToolCalls,
			})
			return
		}

		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "text-1"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "text-1", Delta: "done"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "text-1"}) {
			return
		}
		yield(fantasy.StreamPart{
			Type:         fantasy.StreamPartTypeFinish,
			FinishReason: fantasy.FinishReasonStop,
		})
	}, nil
}

func (m *duplicateToolCallModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *duplicateToolCallModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *duplicateToolCallModel) followupCounts() (int, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.followupToolCalls, m.followupToolResults
}

func TestSessionAgent_DeduplicatesToolCallIDs(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	model := &duplicateToolCallModel{}
	var toolRuns atomic.Int32
	tool := fantasy.NewParallelAgentTool(
		"counted",
		"Count executions.",
		func(context.Context, struct {
			Value string `json:"value"`
		}, fantasy.ToolCall,
		) (fantasy.ToolResponse, error) {
			toolRuns.Add(1)
			time.Sleep(20 * time.Millisecond)
			return fantasy.NewTextResponse("ok"), nil
		},
	)
	agent := testSessionAgent(env, model, &finishStreamModel{text: "title"}, "system", tool)

	sess, err := env.sessions.Create(t.Context(), "Existing session")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "Existing prompt"}},
	})
	require.NoError(t, err)

	_, err = agent.Run(t.Context(), SessionAgentCall{
		Prompt:    "Run the counted tool",
		SessionID: sess.ID,
	})
	require.NoError(t, err)

	followupToolCalls, followupToolResults := model.followupCounts()
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)

	var storedToolCalls int
	var storedToolResults int
	for _, msg := range msgs {
		for _, tc := range msg.ToolCalls() {
			if tc.ID == duplicateToolCallID {
				storedToolCalls++
			}
		}
		for _, tr := range msg.ToolResults() {
			if tr.ToolCallID == duplicateToolCallID {
				storedToolResults++
			}
		}
	}

	assert.Equal(t, int32(1), toolRuns.Load())
	assert.Equal(t, 1, followupToolCalls)
	assert.Equal(t, 1, followupToolResults)
	assert.Equal(t, 1, storedToolCalls)
	assert.Equal(t, 1, storedToolResults)
}
