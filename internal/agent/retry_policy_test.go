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

func TestModelRetryPolicyUsesTenRetries(t *testing.T) {
	for _, test := range []struct {
		name     string
		err      error
		attempts int
	}{
		{name: "rate limit", err: providerRetryError(429), attempts: maxModelRetries + 1},
		{name: "server error", err: providerRetryError(503), attempts: maxModelRetries + 1},
		{name: "client error", err: providerRetryError(400), attempts: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			model := &retryPolicyModel{err: test.err}
			agent := fantasy.NewAgent(model, fantasy.WithMaxRetries(maxModelRetries))

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
	agent := fantasy.NewAgent(model, fantasy.WithMaxRetries(maxModelRetries))
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

func TestModelRetryPolicyStreamUsesRetryPolicy(t *testing.T) {
	for _, test := range []struct {
		name     string
		err      error
		attempts int
	}{
		{name: "rate limit", err: providerRetryError(429), attempts: maxModelRetries + 1},
		{name: "server error", err: providerRetryError(503), attempts: maxModelRetries + 1},
		{name: "client error", err: providerRetryError(400), attempts: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			model := &retryPolicyModel{err: test.err}
			agent := fantasy.NewAgent(model, fantasy.WithMaxRetries(maxModelRetries))

			result, err := agent.Stream(t.Context(), fantasy.AgentStreamCall{Prompt: "test"})

			require.Error(t, err)
			require.Nil(t, result)
			require.Equal(t, test.attempts, model.calls)
		})
	}
}

type retryPolicyModel struct {
	calls int
	err   error
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
