package agent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// contextOverflowModel succeeds on its first Stream call with a small
// usage figure, then fails every subsequent call with a *fantasy.ProviderError
// carrying context-too-large fields, simulating a provider (e.g. OpenRouter)
// rejecting a request because the running conversation grew past the
// model's context window.
type contextOverflowModel struct {
	calls      atomic.Int64
	usedTokens int
	maxTokens  int
}

func (m *contextOverflowModel) Provider() string { return "fake" }
func (m *contextOverflowModel) Model() string    { return "fake-model" }

func (m *contextOverflowModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not implemented")
}

func (m *contextOverflowModel) Stream(_ context.Context, _ fantasy.Call) (fantasy.StreamResponse, error) {
	if m.calls.Add(1) == 1 {
		return func(yield func(fantasy.StreamPart) bool) {
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: "ok"}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"}) {
				return
			}
			yield(fantasy.StreamPart{
				Type:         fantasy.StreamPartTypeFinish,
				FinishReason: fantasy.FinishReasonStop,
				Usage:        fantasy.Usage{InputTokens: 500, OutputTokens: 20},
			})
		}, nil
	}
	return nil, &fantasy.ProviderError{
		Title:              "context_length_exceeded",
		Message:            "This model's maximum context length is exceeded",
		ContextTooLargeErr: true,
		ContextUsedTokens:  m.usedTokens,
		ContextMaxTokens:   m.maxTokens,
	}
}

func (m *contextOverflowModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *contextOverflowModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

// TestRun_ContextTooLargeErrorUpdatesSessionUsage reproduces #3384: after a
// request is rejected for exceeding the model's context window, the
// session's displayed usage must reflect the provider's reported (larger)
// figure for the failed request, not the smaller usage left over from the
// last *successful* step.
func TestRun_ContextTooLargeErrorUpdatesSessionUsage(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	large := &contextOverflowModel{usedTokens: 270000, maxTokens: 262144}
	small := &finishStreamModel{text: "title"}
	sa := testSessionAgent(env, large, small, "system").(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	// First turn succeeds; session usage reflects the small reported usage.
	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "first"})
	require.NoError(t, err)

	sess, err = env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, int64(500), sess.PromptTokens, "baseline usage from the successful first turn")

	// Second turn is rejected for exceeding the context window.
	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "second"})
	require.Error(t, err)

	sess, err = env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, int64(270000), sess.PromptTokens,
		"session usage must reflect the provider's reported context size for the rejected request, not the stale successful-step figure")
	require.True(t, sess.EstimatedUsage, "a failed, unbilled request must never be recorded as authoritative billed usage")
}
