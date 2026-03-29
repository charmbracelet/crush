package agent

import (
	"testing"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"github.com/stretchr/testify/require"
)

func TestDisableAnthropicThinking(t *testing.T) {
	t.Parallel()

	budget := int64(2000)
	opts := fantasy.ProviderOptions{
		anthropic.Name: &anthropic.ProviderOptions{
			Thinking: &anthropic.ThinkingProviderOption{BudgetTokens: budget},
		},
	}

	sanitized, changed := disableAnthropicThinking(opts)
	require.True(t, changed)

	originalAnthropic, ok := opts[anthropic.Name].(*anthropic.ProviderOptions)
	require.True(t, ok)
	require.NotNil(t, originalAnthropic.Thinking)
	require.Equal(t, budget, originalAnthropic.Thinking.BudgetTokens)

	sanitizedAnthropic, ok := sanitized[anthropic.Name].(*anthropic.ProviderOptions)
	require.True(t, ok)
	require.Nil(t, sanitizedAnthropic.Thinking)
}

func TestDisableAnthropicThinking_NoAnthropicThinkingConfigured(t *testing.T) {
	t.Parallel()

	opts := fantasy.ProviderOptions{}

	sanitized, changed := disableAnthropicThinking(opts)
	require.False(t, changed)
	require.Equal(t, opts, sanitized)
}

func TestShouldRetryWithoutAnthropicThinking(t *testing.T) {
	t.Parallel()

	budget := int64(2000)
	opts := fantasy.ProviderOptions{
		anthropic.Name: &anthropic.ProviderOptions{
			Thinking: &anthropic.ThinkingProviderOption{BudgetTokens: budget},
		},
	}

	err := &fantasy.ProviderError{
		StatusCode: 400,
		Message:    "thinking is enabled but reasoning_content is missing in assistant tool call message at index 86",
	}

	require.True(t, shouldRetryWithoutAnthropicThinking(err, opts))
}

func TestShouldRetryWithoutAnthropicThinking_RejectsOtherErrors(t *testing.T) {
	t.Parallel()

	budget := int64(2000)
	opts := fantasy.ProviderOptions{
		anthropic.Name: &anthropic.ProviderOptions{
			Thinking: &anthropic.ThinkingProviderOption{BudgetTokens: budget},
		},
	}

	require.False(t, shouldRetryWithoutAnthropicThinking(&fantasy.ProviderError{
		StatusCode: 400,
		Message:    "different validation error",
	}, opts))
	require.False(t, shouldRetryWithoutAnthropicThinking(&fantasy.ProviderError{
		StatusCode: 500,
		Message:    "thinking is enabled but reasoning_content is missing",
	}, opts))
	require.False(t, shouldRetryWithoutAnthropicThinking(assertiveError("thinking is enabled but reasoning_content is missing"), opts))
	require.False(t, shouldRetryWithoutAnthropicThinking(&fantasy.ProviderError{
		StatusCode: 400,
		Message:    "thinking is enabled but reasoning_content is missing",
	}, fantasy.ProviderOptions{}))
}

func TestShouldRetryWithoutAnthropicThinking_FuzzyProviderMessage(t *testing.T) {
	t.Parallel()

	opts := fantasy.ProviderOptions{
		anthropic.Name: &anthropic.ProviderOptions{
			Thinking: &anthropic.ThinkingProviderOption{BudgetTokens: 2000},
		},
	}

	tests := []struct {
		name    string
		msg     string
		want    bool
		status  int
		options fantasy.ProviderOptions
	}{
		{
			name:   "matches fuzzy tool_use wording",
			msg:    "THINKING failed because reasoning content is required for assistant tool_use payload",
			want:   true,
			status: 400,
		},
		{
			name:   "matches fuzzy tool call wording",
			msg:    "thinking block rejected: reasoning_content missing in assistant tool call",
			want:   true,
			status: 400,
		},
		{
			name:   "rejects missing tool context",
			msg:    "thinking enabled but reasoning_content is required",
			want:   false,
			status: 400,
		},
		{
			name:   "rejects wrong status",
			msg:    "thinking block rejected: reasoning_content missing in assistant tool call",
			want:   false,
			status: 422,
		},
		{
			name:    "rejects when anthropic thinking disabled",
			msg:     "thinking block rejected: reasoning_content missing in assistant tool call",
			want:    false,
			status:  400,
			options: fantasy.ProviderOptions{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			currentOpts := tt.options
			if currentOpts == nil {
				currentOpts = opts
			}
			err := &fantasy.ProviderError{StatusCode: tt.status, Message: tt.msg}
			require.Equal(t, tt.want, shouldRetryWithoutAnthropicThinking(err, currentOpts))
		})
	}
}

type assertiveError string

func (e assertiveError) Error() string { return string(e) }
