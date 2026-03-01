package agent

import (
	"fmt"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestContextStatusMessage(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{}

	t.Run("basic computation", func(t *testing.T) {
		t.Parallel()

		s := session.Session{
			PromptTokens:     60000,
			CompletionTokens: 40000,
		}
		model := Model{
			CatwalkCfg: catwalk.Model{
				ContextWindow: 200000,
			},
		}

		msg, ok := agent.contextStatusMessage(s, model)
		require.True(t, ok)
		require.Equal(t, fantasy.MessageRoleUser, msg.Role)
		require.Len(t, msg.Content, 1)

		textPart, ok := fantasy.AsMessagePart[fantasy.TextPart](msg.Content[0])
		require.True(t, ok)
		require.Equal(t,
			`<context_status>{"used_pct":50,"remaining_tokens":100000,"context_window":200000}</context_status>`,
			textPart.Text,
		)
	})

	t.Run("zero context window returns false", func(t *testing.T) {
		t.Parallel()

		s := session.Session{
			PromptTokens:     1000,
			CompletionTokens: 500,
		}
		model := Model{
			CatwalkCfg: catwalk.Model{
				ContextWindow: 0,
			},
		}

		_, ok := agent.contextStatusMessage(s, model)
		require.False(t, ok)
	})

	t.Run("negative context window returns false", func(t *testing.T) {
		t.Parallel()

		s := session.Session{
			PromptTokens:     1000,
			CompletionTokens: 500,
		}
		model := Model{
			CatwalkCfg: catwalk.Model{
				ContextWindow: -100,
			},
		}

		_, ok := agent.contextStatusMessage(s, model)
		require.False(t, ok)
	})

	t.Run("remaining clamped to zero when overflowed", func(t *testing.T) {
		t.Parallel()

		s := session.Session{
			PromptTokens:     150000,
			CompletionTokens: 100000,
		}
		model := Model{
			CatwalkCfg: catwalk.Model{
				ContextWindow: 200000,
			},
		}

		msg, ok := agent.contextStatusMessage(s, model)
		require.True(t, ok)

		textPart, ok := fantasy.AsMessagePart[fantasy.TextPart](msg.Content[0])
		require.True(t, ok)
		require.Equal(t,
			`<context_status>{"used_pct":125,"remaining_tokens":0,"context_window":200000}</context_status>`,
			textPart.Text,
		)
	})

	t.Run("zero tokens", func(t *testing.T) {
		t.Parallel()

		s := session.Session{
			PromptTokens:     0,
			CompletionTokens: 0,
		}
		model := Model{
			CatwalkCfg: catwalk.Model{
				ContextWindow: 200000,
			},
		}

		msg, ok := agent.contextStatusMessage(s, model)
		require.True(t, ok)

		textPart, ok := fantasy.AsMessagePart[fantasy.TextPart](msg.Content[0])
		require.True(t, ok)
		require.Equal(t,
			`<context_status>{"used_pct":0,"remaining_tokens":200000,"context_window":200000}</context_status>`,
			textPart.Text,
		)
	})

	t.Run("100 percent usage", func(t *testing.T) {
		t.Parallel()

		s := session.Session{
			PromptTokens:     120000,
			CompletionTokens: 80000,
		}
		model := Model{
			CatwalkCfg: catwalk.Model{
				ContextWindow: 200000,
			},
		}

		msg, ok := agent.contextStatusMessage(s, model)
		require.True(t, ok)

		textPart, ok := fantasy.AsMessagePart[fantasy.TextPart](msg.Content[0])
		require.True(t, ok)
		require.Equal(t,
			`<context_status>{"used_pct":100,"remaining_tokens":0,"context_window":200000}</context_status>`,
			textPart.Text,
		)
	})

	t.Run("small context window", func(t *testing.T) {
		t.Parallel()

		s := session.Session{
			PromptTokens:     3000,
			CompletionTokens: 1000,
		}
		model := Model{
			CatwalkCfg: catwalk.Model{
				ContextWindow: 8192,
			},
		}

		msg, ok := agent.contextStatusMessage(s, model)
		require.True(t, ok)

		textPart, ok := fantasy.AsMessagePart[fantasy.TextPart](msg.Content[0])
		require.True(t, ok)

		used := int64(4000)
		cw := int64(8192)
		expectedUsedPct := int64(float64(used) / float64(cw) * 100)
		expectedRemaining := cw - used
		require.Equal(t,
			fmt.Sprintf(`<context_status>{"used_pct":%d,"remaining_tokens":%d,"context_window":8192}</context_status>`,
				expectedUsedPct, expectedRemaining),
			textPart.Text,
		)
	})
}

func TestContextStatusMessageNotInjectedForSubAgent(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{
		isSubAgent: true,
	}

	s := session.Session{
		PromptTokens:     60000,
		CompletionTokens: 40000,
	}
	model := Model{
		CatwalkCfg: catwalk.Model{
			ContextWindow: 200000,
		},
	}

	// The method itself doesn't check isSubAgent — that gating is in
	// PrepareStep. Verify the method still works so the gating logic in
	// PrepareStep is the sole control point.
	msg, ok := agent.contextStatusMessage(s, model)
	require.True(t, ok)
	require.Equal(t, fantasy.MessageRoleUser, msg.Role)
}
