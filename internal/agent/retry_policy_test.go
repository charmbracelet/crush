package agent

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestModelRetryPolicyGenerate(t *testing.T) {
	for _, test := range retryPolicyCases() {
		t.Run(test.name, func(t *testing.T) {
			model := &retryPolicyModel{err: test.err}
			agent := fantasy.NewAgent(model, (&sessionAgent{maxRetries: test.maxRetries}).retryOption())

			result, err := agent.Generate(t.Context(), fantasy.AgentCall{Prompt: "test"})

			require.Error(t, err)
			require.Nil(t, result)
			require.Equal(t, test.attempts, model.calls)
		})
	}
}

func TestModelRetryPolicyRetriesNetworkErrorsAndHonorsCancellation(t *testing.T) {
	t.Parallel()

	model := &retryPolicyModel{err: &net.DNSError{Err: "temporary", Name: "provider"}}
	agent := fantasy.NewAgent(model, (&sessionAgent{}).retryOption())
	ctx, cancel := context.WithCancel(t.Context())
	retried := false

	result, err := agent.Generate(ctx, fantasy.AgentCall{
		Prompt: "test",
		OnRetry: func(*fantasy.ProviderError, time.Duration) {
			retried = true
			cancel()
		},
	})

	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, result)
	require.True(t, retried)
	require.Equal(t, 1, model.calls)
}

func TestModelRetryPolicyRetriesNetworkErrorsToLimit(t *testing.T) {
	t.Parallel()

	calls := 0
	options := fantasy.DefaultRetryOptions()
	options.InitialDelayIn = time.Millisecond
	options.BackoffFactor = 1
	retry := fantasy.RetryWithExponentialBackoffRespectingRetryHeaders[struct{}](options)

	_, err := retry(t.Context(), func() (struct{}, error) {
		calls++
		return struct{}{}, &net.DNSError{Err: "temporary", Name: "provider"}
	})

	require.Error(t, err)
	require.Equal(t, options.MaxRetries+1, calls)
}

func TestModelRetryPolicyStream(t *testing.T) {
	for _, test := range retryPolicyCases() {
		t.Run(test.name, func(t *testing.T) {
			model := &retryPolicyModel{err: test.err}
			agent := fantasy.NewAgent(model, (&sessionAgent{maxRetries: test.maxRetries}).retryOption())

			result, err := agent.Stream(t.Context(), fantasy.AgentStreamCall{Prompt: "test"})

			require.Error(t, err)
			require.Nil(t, result)
			require.Equal(t, test.attempts, model.calls)
		})
	}
}

func TestModelRetryPolicyReplaysPartialStreams(t *testing.T) {
	t.Parallel()

	model := &retryPolicyModel{stream: func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "text"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "text", Delta: "partial"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: providerRetryError(503)})
	}}
	agent := fantasy.NewAgent(model, fantasy.WithMaxRetries(1))
	var deltas []string

	result, err := agent.Stream(t.Context(), fantasy.AgentStreamCall{
		Prompt: "test",
		OnTextDelta: func(_, text string) error {
			deltas = append(deltas, text)
			return nil
		},
	})

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, 2, model.calls)
	require.Equal(t, []string{"partial", "partial"}, deltas)
}

type retryPolicyCase struct {
	name       string
	err        error
	maxRetries *int
	attempts   int
}

func retryPolicyCases() []retryPolicyCase {
	defaultAttempts := fantasy.DefaultRetryOptions().MaxRetries + 1
	zero := 0
	return []retryPolicyCase{
		{name: "rate limit", err: providerRetryError(429), attempts: defaultAttempts},
		{name: "server error", err: providerRetryError(503), attempts: defaultAttempts},
		{name: "client error", err: providerRetryError(400), attempts: 1},
		{name: "retries disabled", err: providerRetryError(503), maxRetries: &zero, attempts: 1},
	}
}

type retryPolicyModel struct {
	calls  int
	err    error
	stream fantasy.StreamResponse
}

func (m *retryPolicyModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	m.calls++
	return nil, m.err
}

func providerRetryError(status int) error {
	return &fantasy.ProviderError{
		StatusCode:      status,
		Message:         "provider error",
		ResponseHeaders: map[string]string{"retry-after-ms": "1"},
	}
}

func (m *retryPolicyModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	m.calls++
	if m.stream != nil {
		return m.stream, nil
	}
	return nil, m.err
}

func (*retryPolicyModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (*retryPolicyModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func (*retryPolicyModel) Provider() string { return "test" }
func (*retryPolicyModel) Model() string    { return "test" }
