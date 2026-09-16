package agent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// flakyStreamModel fails its first stream with a retryable in-band
// provider error, then succeeds. It is the honest stand-in for a
// provider that 503s once mid-turn.
type flakyStreamModel struct {
	calls atomic.Int32
}

func (m *flakyStreamModel) Provider() string { return "fake" }
func (m *flakyStreamModel) Model() string    { return "fake-model" }

func (m *flakyStreamModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not implemented")
}

func (m *flakyStreamModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	if m.calls.Add(1) == 1 {
		return func(yield func(fantasy.StreamPart) bool) {
			yield(fantasy.StreamPart{
				Type: fantasy.StreamPartTypeError,
				Error: &fantasy.ProviderError{
					StatusCode:      503,
					Message:         "overloaded",
					ResponseHeaders: map[string]string{"retry-after-ms": "1"},
				},
			})
		}, nil
	}
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: "recovered"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

func (m *flakyStreamModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *flakyStreamModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

// alwaysFailModel never recovers: every stream ends in a retryable
// in-band provider error.
type alwaysFailModel struct {
	calls atomic.Int32
}

func (m *alwaysFailModel) Provider() string { return "fake" }
func (m *alwaysFailModel) Model() string    { return "fake-model" }

func (m *alwaysFailModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not implemented")
}

func (m *alwaysFailModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	m.calls.Add(1)
	return func(yield func(fantasy.StreamPart) bool) {
		yield(fantasy.StreamPart{
			Type: fantasy.StreamPartTypeError,
			Error: &fantasy.ProviderError{
				StatusCode:      503,
				Message:         "overloaded",
				ResponseHeaders: map[string]string{"retry-after-ms": "1"},
			},
		})
	}, nil
}

func (m *alwaysFailModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *alwaysFailModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

// retryNotifyTitleModel answers title generation without touching
// the retry accounting under test.
type retryNotifyTitleModel struct{}

func (retryNotifyTitleModel) Provider() string { return "fake" }
func (retryNotifyTitleModel) Model() string    { return "fake-model" }

func (retryNotifyTitleModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{
		Content:      fantasy.ResponseContent{fantasy.TextContent{Text: "title"}},
		FinishReason: fantasy.FinishReasonStop,
	}, nil
}

func (retryNotifyTitleModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: "title"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

func (retryNotifyTitleModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (retryNotifyTitleModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func retryNotifyAgent(env fakeEnv, model fantasy.LanguageModel, broker *pubsub.Broker[notify.Notification], maxRetries int) SessionAgent {
	large := Model{
		Model:      model,
		CatwalkCfg: catwalk.Model{ID: "mock-model", ContextWindow: 8192, DefaultMaxTokens: 128},
		ModelCfg:   config.SelectedModel{Provider: "mock", Model: "mock-model"},
	}
	small := Model{
		Model:      retryNotifyTitleModel{},
		CatwalkCfg: catwalk.Model{ID: "mock-model", ContextWindow: 8192, DefaultMaxTokens: 128},
		ModelCfg:   config.SelectedModel{Provider: "mock", Model: "mock-model"},
	}
	return NewSessionAgent(SessionAgentOptions{
		LargeModel:           large,
		SmallModel:           small,
		SystemPrompt:         "fake system prompt",
		IsSubAgent:           true,
		DisableAutoSummarize: true,
		IsYolo:               true,
		Sessions:             env.sessions,
		Messages:             env.messages,
		MaxRetries:           &maxRetries,
		Notify:               broker,
	})
}

// TestSessionAgentRun_PublishesRetryNotification proves the retry
// visibility contract: a turn whose first attempt fails retryably
// must emit exactly one TypeAgentRetrying notification carrying the
// session and the failure reason, then complete normally without
// duplicating the user message. Before this, retries were only a
// slog line: the TUI sat silent through the backoff and looked hung,
// typically on the last tool-call spinner.
func TestSessionAgentRun_PublishesRetryNotification(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	broker := pubsub.NewBroker[notify.Notification]()
	t.Cleanup(broker.Shutdown)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	sub := broker.Subscribe(ctx)

	sa := retryNotifyAgent(env, &flakyStreamModel{}, broker, 1)

	sess, err := env.sessions.Create(t.Context(), "retry-notify")
	require.NoError(t, err)

	result, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "go"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "recovered", result.Response.Content.Text())

	select {
	case ev := <-sub:
		require.Equal(t, notify.TypeAgentRetrying, ev.Payload.Type)
		require.Equal(t, sess.ID, ev.Payload.SessionID)
		require.Contains(t, ev.Payload.Message, "overloaded")
		require.Contains(t, ev.Payload.Message, "attempt 1")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the retry notification")
	}

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	userMsgs := 0
	for _, msg := range msgs {
		if msg.Role == message.User {
			userMsgs++
		}
	}
	require.Equal(t, 1, userMsgs, "the retried turn must not duplicate the user message")
}

// TestSessionAgentRun_ExhaustedRetriesToastOnce proves the terminal
// notification contract: when retries run out, the turn emits exactly
// one TypeAgentError carrying the retry count. Per-attempt notices
// stay status-bar only, so the desktop gets a single failure toast
// instead of one per attempt.
func TestSessionAgentRun_ExhaustedRetriesToastOnce(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	broker := pubsub.NewBroker[notify.Notification]()
	t.Cleanup(broker.Shutdown)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	sub := broker.Subscribe(ctx)

	sa := retryNotifyAgent(env, &alwaysFailModel{}, broker, 1)

	sess, err := env.sessions.Create(t.Context(), "retry-exhausted")
	require.NoError(t, err)

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "go"})
	require.Error(t, err, "an unrecoverable provider must still fail the turn")

	var severities []notify.Type
	var errNotice notify.Notification
	deadline := time.After(5 * time.Second)
	for len(severities) < 2 {
		select {
		case ev := <-sub:
			severities = append(severities, ev.Payload.Type)
			if ev.Payload.Type == notify.TypeAgentError {
				errNotice = ev.Payload
			}
		case <-deadline:
			t.Fatalf("timed out waiting for retry+error notices, got %v", severities)
		}
	}
	require.Equal(t, []notify.Type{notify.TypeAgentRetrying, notify.TypeAgentError}, severities,
		"exactly one per-attempt notice followed by one terminal error toast signal")
	require.Equal(t, sess.ID, errNotice.SessionID)
	require.Contains(t, errNotice.Message, "failed after 1 retry")
}
