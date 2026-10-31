package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// refusingModel refuses its first `refusals` streams, then answers.
type refusingModel struct {
	fakeLanguageModel
	refusals int
	calls    int
}

func (m *refusingModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	m.calls++
	reason := fantasy.FinishReasonStop
	if m.calls <= m.refusals {
		reason = fantasy.FinishReasonContentFilter
	}
	return func(yield func(fantasy.StreamPart) bool) {
		_ = yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "t"}) &&
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "t", Delta: "answer"}) &&
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "t"}) &&
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: reason})
	}, nil
}

func TestRefusalRetryModel_TurnsRefusalIntoRetryableError(t *testing.T) {
	t.Parallel()
	stream, err := newRefusalRetryModel(&refusingModel{refusals: 1}).Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)

	var last fantasy.StreamPart
	for part := range stream {
		last = part
	}
	require.Equal(t, fantasy.StreamPartTypeError, last.Type)
	require.ErrorIs(t, last.Error, errRefused)
	var providerErr *fantasy.ProviderError
	require.True(t, errors.As(last.Error, &providerErr))
	require.True(t, providerErr.IsRetryable())
}

func TestRefusalRetryModel_PassesOtherFinishesThrough(t *testing.T) {
	t.Parallel()
	stream, err := newRefusalRetryModel(&refusingModel{}).Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)

	var last fantasy.StreamPart
	for part := range stream {
		last = part
	}
	require.Equal(t, fantasy.StreamPartTypeFinish, last.Type)
	require.Equal(t, fantasy.FinishReasonStop, last.FinishReason)
}

// TestRefusalRetryModel_AgentRetriesRefusal runs the real agent loop, so it
// waits out one retry backoff.
func TestRefusalRetryModel_AgentRetriesRefusal(t *testing.T) {
	t.Parallel()
	model := &refusingModel{refusals: 1}
	retries := 0
	result, err := fantasy.NewAgent(newRefusalRetryModel(model)).Stream(t.Context(), fantasy.AgentStreamCall{
		Prompt:  "hi",
		OnRetry: func(*fantasy.ProviderError, time.Duration) { retries++ },
	})
	require.NoError(t, err)
	require.Equal(t, 1, retries)
	require.Equal(t, 2, model.calls)
	require.Equal(t, "answer", result.Response.Content.Text())
}

func TestTurnEndReason_Refused(t *testing.T) {
	t.Parallel()
	require.Equal(t, refusedToolCallResult, turnEndReason(&message.Message{}, refusedError()))
}
