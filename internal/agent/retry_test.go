package agent

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/openai"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

type retryTestModel struct {
	fakeLanguageModel
	attempts int
	streamFn func(int) (fantasy.StreamResponse, error)
}

func (model *retryTestModel) Stream(_ context.Context, _ fantasy.Call) (fantasy.StreamResponse, error) {
	model.attempts++
	return model.streamFn(model.attempts)
}

func retryParts(parts ...fantasy.StreamPart) fantasy.StreamResponse {
	return func(yield func(fantasy.StreamPart) bool) {
		for _, part := range parts {
			if !yield(part) {
				return
			}
		}
	}
}

func TestRetryModelSafeStep(t *testing.T) {
	for _, maxRetries := range []int{0, 2, 5} {
		t.Run(strings.Repeat("retry", maxRetries+1), func(t *testing.T) {
			policy, err := config.ResolveRetry(nil, nil)
			require.NoError(t, err)
			policy.MaxRetries = maxRetries
			policy.InitialDelay = 0
			policy.Jitter = "none"
			model := &retryTestModel{streamFn: func(attempt int) (fantasy.StreamResponse, error) {
				if attempt <= maxRetries {
					return retryParts(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: "abandoned"}, fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: &fantasy.ProviderError{Cause: io.ErrUnexpectedEOF}}), nil
				}
				return retryParts(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: "committed"}, fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish}), nil
			}}
			wrapper := newRetryModel(model, policy).(retryModel)
			var received []fantasy.StreamPart
			stream, err := wrapper.Stream(t.Context(), fantasy.Call{})
			require.NoError(t, err)
			stream(func(part fantasy.StreamPart) bool { received = append(received, part); return true })
			require.Equal(t, maxRetries+1, model.attempts)
			require.Len(t, received, 2)
			require.Equal(t, "committed", received[0].Delta)
		})
	}
}

func TestRetryModelSafeStepToolInput(t *testing.T) {
	policy, err := config.ResolveRetry(nil, nil)
	require.NoError(t, err)
	policy.InitialDelay = 0
	policy.Jitter = "none"
	model := &retryTestModel{streamFn: func(attempt int) (fantasy.StreamResponse, error) {
		if attempt == 1 {
			return retryParts(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputStart, ID: "tool"}, fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputDelta, ID: "tool", Delta: "{\"partial\""}, fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: &fantasy.ProviderError{Cause: io.ErrUnexpectedEOF}}), nil
		}
		return retryParts(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: "tool", ToolCallName: "read", ToolCallInput: "{}"}, fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls}), nil
	}}
	stream, err := newRetryModel(model, policy).Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)
	var received []fantasy.StreamPart
	stream(func(part fantasy.StreamPart) bool { received = append(received, part); return true })
	require.Equal(t, 2, model.attempts)
	require.Len(t, received, 2)
	require.Equal(t, fantasy.StreamPartTypeToolCall, received[0].Type)
}

func TestRetryModelBeforeOutputAndCancellation(t *testing.T) {
	policy, err := config.ResolveRetry(nil, nil)
	require.NoError(t, err)
	policy.StreamPolicy = "before_output"
	policy.MaxRetries = 5
	policy.InitialDelay = 0
	model := &retryTestModel{streamFn: func(int) (fantasy.StreamResponse, error) {
		return retryParts(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: "partial"}, fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: &fantasy.ProviderError{Cause: io.ErrUnexpectedEOF}}), nil
	}}
	stream, err := newRetryModel(model, policy).Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)
	var received []fantasy.StreamPart
	stream(func(part fantasy.StreamPart) bool { received = append(received, part); return true })
	require.Equal(t, 1, model.attempts)
	require.Len(t, received, 2)
	require.Equal(t, "partial", received[0].Delta)
	require.ErrorIs(t, received[1].Error, io.ErrUnexpectedEOF)

	cancelCtx, cancel := context.WithCancel(t.Context())
	model = &retryTestModel{streamFn: func(int) (fantasy.StreamResponse, error) {
		return retryParts(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: &fantasy.ProviderError{Cause: io.ErrUnexpectedEOF}}), nil
	}}
	wrapper := newRetryModel(model, policy).(retryModel)
	wrapper.wait = func(context.Context, time.Duration) error { cancel(); return cancelCtx.Err() }
	stream, err = wrapper.Stream(cancelCtx, fantasy.Call{})
	require.NoError(t, err)
	stream(func(part fantasy.StreamPart) bool { received = append(received, part); return true })
	require.Equal(t, 1, model.attempts)
	require.ErrorIs(t, received[len(received)-1].Error, context.Canceled)
}

func TestRetryModelIncompleteStreamAndToolSafety(t *testing.T) {
	policy, err := config.ResolveRetry(nil, nil)
	require.NoError(t, err)
	policy.InitialDelay = 0
	policy.Jitter = "none"
	model := &retryTestModel{streamFn: func(attempt int) (fantasy.StreamResponse, error) {
		if attempt == 1 {
			return retryParts(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: "lost"}), nil
		}
		return retryParts(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish}), nil
	}}
	stream, err := newRetryModel(model, policy).Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)
	var received []fantasy.StreamPart
	stream(func(part fantasy.StreamPart) bool { received = append(received, part); return true })
	require.Equal(t, 2, model.attempts)
	require.Len(t, received, 1)

	model = &retryTestModel{streamFn: func(int) (fantasy.StreamResponse, error) {
		return retryParts(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ProviderExecuted: true}, fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: &fantasy.ProviderError{Cause: io.ErrUnexpectedEOF}}), nil
	}}
	stream, err = newRetryModel(model, policy).Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)
	received = nil
	stream(func(part fantasy.StreamPart) bool { received = append(received, part); return true })
	require.Equal(t, 1, model.attempts)
	require.Len(t, received, 2)
	require.ErrorIs(t, received[1].Error, io.ErrUnexpectedEOF)
}

func TestRetryModelPreservesTurnAndToolResults(t *testing.T) {
	env := testEnv(t)
	session, err := env.sessions.Create(t.Context(), "retry")
	require.NoError(t, err)
	policy, err := config.ResolveRetry(nil, nil)
	require.NoError(t, err)
	policy.InitialDelay = 0
	policy.Jitter = "none"
	model := &retryTestModel{streamFn: func(attempt int) (fantasy.StreamResponse, error) {
		if attempt == 1 {
			return retryParts(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "text"}, fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "text", Delta: "discarded"}, fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: &fantasy.ProviderError{Cause: io.ErrUnexpectedEOF}}), nil
		}
		return retryParts(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "text"}, fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "text", Delta: "complete"}, fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "text"}, fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop}), nil
	}}
	agent := testSessionAgent(env, newRetryModel(model, policy), model, "system")
	_, err = env.messages.Create(t.Context(), session.ID, message.CreateMessageParams{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "earlier"}}})
	require.NoError(t, err)
	_, err = agent.Run(t.Context(), SessionAgentCall{SessionID: session.ID, Prompt: "request"})
	require.NoError(t, err)
	require.Equal(t, 2, model.attempts)
	messages, err := env.messages.List(t.Context(), session.ID)
	require.NoError(t, err)
	var userCount int
	for _, item := range messages {
		if item.Role == message.User && item.Content().String() == "request" {
			userCount++
		}
		if item.Role == message.Assistant {
			require.NotContains(t, item.Content().String(), "discarded")
		}
	}
	require.Equal(t, 1, userCount)
}

func TestRetryProgressEvents(t *testing.T) {
	policy, err := config.ResolveRetry(nil, nil)
	require.NoError(t, err)
	policy.InitialDelay = 0
	policy.Jitter = "none"
	broker := pubsub.NewBroker[notify.Notification]()
	ctx := context.WithValue(t.Context(), tools.SessionIDContextKey, "session")
	ctx = WithRunID(ctx, "run")
	events := broker.Subscribe(ctx)
	model := &retryTestModel{streamFn: func(attempt int) (fantasy.StreamResponse, error) {
		if attempt == 1 {
			return nil, &fantasy.ProviderError{Cause: io.ErrUnexpectedEOF}
		}
		return retryParts(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish}), nil
	}}
	stream, err := newRetryModel(model, policy, broker).Stream(ctx, fantasy.Call{})
	require.NoError(t, err)
	stream(func(fantasy.StreamPart) bool { return true })
	first := <-events
	require.Equal(t, notify.TypeRetry, first.Payload.Type)
	require.Equal(t, "session", first.Payload.SessionID)
	require.Equal(t, "run", first.Payload.RunID)
	require.Equal(t, 2, first.Payload.Attempt)
	require.False(t, first.Payload.Done)
	second := <-events
	require.True(t, second.Payload.Done)
}

func TestRetryHTTPStreamReset(t *testing.T) {
	attempts := 0
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		attempts++
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)
		if attempts == 1 {
			_, _ = io.WriteString(writer, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"discarded\"}}]}\n\n")
			writer.(http.Flusher).Flush()
			panic(http.ErrAbortHandler)
		}
		_, _ = io.WriteString(writer, "data: {\"id\":\"2\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"committed\"}}]}\n\n")
		_, _ = io.WriteString(writer, "data: {\"id\":\"2\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		_, _ = io.WriteString(writer, "data: [DONE]\n\n")
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	provider, err := openai.New(openai.WithAPIKey("test"), openai.WithBaseURL(server.URL), openai.WithHTTPClient(server.Client()))
	require.NoError(t, err)
	model, err := provider.LanguageModel(t.Context(), "gpt-3.5-turbo")
	require.NoError(t, err)
	policy, err := config.ResolveRetry(nil, nil)
	require.NoError(t, err)
	policy.InitialDelay = 0
	policy.Jitter = "none"
	stream, err := newRetryModel(model, policy).Stream(t.Context(), fantasy.Call{Prompt: fantasy.Prompt{fantasy.NewUserMessage("hello")}})
	require.NoError(t, err)
	var received []fantasy.StreamPart
	stream(func(part fantasy.StreamPart) bool { received = append(received, part); return true })
	require.Equal(t, 2, attempts)
	for _, part := range received {
		require.NotEqual(t, fantasy.StreamPartTypeError, part.Type, "%v", part.Error)
		require.NotContains(t, part.Delta, "discarded")
	}
}

func TestRetryTimeBudgetAndCallerCancellation(t *testing.T) {
	policy, err := config.ResolveRetry(nil, nil)
	require.NoError(t, err)
	policy.MaxRetries = 5
	policy.InitialDelay = time.Second
	policy.MaxElapsed = 2 * time.Second
	policy.Jitter = "none"
	model := &retryTestModel{streamFn: func(int) (fantasy.StreamResponse, error) {
		return nil, &fantasy.ProviderError{StatusCode: 429, ResponseHeaders: map[string]string{"Retry-After": "3"}}
	}}
	stream, err := newRetryModel(model, policy).Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)
	var received []fantasy.StreamPart
	stream(func(part fantasy.StreamPart) bool { received = append(received, part); return true })
	require.Equal(t, 1, model.attempts)
	require.Len(t, received, 1)
	require.ErrorContains(t, received[0].Error, "retry time budget exhausted")
	var providerErr *fantasy.ProviderError
	require.ErrorAs(t, received[0].Error, &providerErr)
}

func TestRetryClassificationAndHeaders(t *testing.T) {
	require.False(t, retryableModelError(t.Context(), &fantasy.ProviderError{StatusCode: 400, ContextTooLargeErr: true}))
	require.False(t, retryableModelError(t.Context(), &fantasy.ProviderError{StatusCode: 500, ResponseHeaders: map[string]string{"X-Should-Retry": "false"}}))
	require.False(t, retryableModelError(t.Context(), errors.New("unexpected EOF")))
	require.True(t, retryableModelError(t.Context(), &fantasy.ProviderError{Cause: io.ErrUnexpectedEOF}))
	policy, err := config.ResolveRetry(nil, nil)
	require.NoError(t, err)
	policy.Jitter = "none"
	wrapper := newRetryModel(&retryTestModel{}, policy).(retryModel)
	require.Equal(t, 250*time.Millisecond, serverRetryDelay(&fantasy.ProviderError{ResponseHeaders: map[string]string{"Retry-After-Ms": "250.0"}}, time.Now()))
	require.Equal(t, 3*time.Second, wrapper.delay(0, &fantasy.ProviderError{ResponseHeaders: map[string]string{"Retry-After": "3"}}))
	require.Equal(t, 1500*time.Millisecond, serverRetryDelay(&fantasy.ProviderError{ResponseHeaders: map[string]string{"RETRY-AFTER": "1.5"}}, time.Now()))
	instant := time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("GMT", 0))
	require.Equal(t, 2*time.Second, serverRetryDelay(&fantasy.ProviderError{ResponseHeaders: map[string]string{"Retry-After": instant.Add(2 * time.Second).Format(time.RFC1123)}}, instant))
	wrapper.random = func() float64 { return 0.5 }
	wrapper.policy.Jitter = "full"
	require.Equal(t, time.Second, wrapper.delay(0, nil))
	wrapper.policy.Jitter = "none"
	require.Equal(t, 4*time.Second, wrapper.delay(1, nil))
}
