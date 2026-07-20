package agent

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestRunOversizedFirstPromptDispatchesNeitherMainNorTitleRequest(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	provider := &contextWindowTestModel{}
	model := Model{
		Model: provider,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    4_096,
			DefaultMaxTokens: 512,
		},
	}
	agent := testSessionAgent(env, provider, provider, "system prompt").(*sessionAgent)
	agent.SetModels(model, model)

	sess, err := env.sessions.Create(t.Context(), "new session")
	require.NoError(t, err)
	var completions []string
	_, err = agent.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		RunID:     "oversized-first-prompt",
		Prompt:    strings.Repeat("large first prompt ", 2_000),
		OnComplete: func(complete notify.RunComplete) {
			completions = append(completions, complete.Error)
		},
	})

	require.ErrorContains(t, err, "model request exceeds context window")
	require.Zero(t, provider.streamCalls.Load(), "neither title nor main request may reach the provider")
	require.Len(t, completions, 1, "rejected run must still have exactly one terminal event")
	require.Contains(t, completions[0], "model request exceeds context window")
}

func TestRunOversizedTextAttachmentFailsBeforeProviderDispatch(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	provider := &contextWindowTestModel{}
	model := Model{
		Model: provider,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    4_096,
			DefaultMaxTokens: 512,
		},
	}
	agent := testSessionAgent(env, provider, provider, "system prompt").(*sessionAgent)
	agent.SetModels(model, model)

	sess, err := env.sessions.Create(t.Context(), "large attachment")
	require.NoError(t, err)
	_, err = agent.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Attachments: []message.Attachment{
			{
				FileName: "large.txt",
				MimeType: "text/plain",
				Content:  []byte(strings.Repeat("attachment content ", 2_000)),
			},
		},
	})
	require.ErrorContains(t, err, "model request exceeds context window")
	require.Zero(t, provider.streamCalls.Load())
}

func TestRunAlwaysSendsKnownOutputBound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		modelDefault int64
		requested    int64
		expected     int64
	}{
		{name: "model default", modelDefault: 512, expected: 512},
		{name: "context reserve fallback", expected: 819},
		{name: "call-specific limit", modelDefault: 512, requested: 256, expected: 256},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := testEnv(t)
			provider := &compactionTestModel{}
			model := Model{
				Model: provider,
				CatwalkCfg: catwalk.Model{
					ContextWindow:    4_096,
					DefaultMaxTokens: tt.modelDefault,
				},
			}
			agent := testSessionAgent(env, provider, provider, "system prompt").(*sessionAgent)
			agent.SetModels(model, model)

			sess, err := env.sessions.Create(t.Context(), "bounded output")
			require.NoError(t, err)
			_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
				Role:  message.User,
				Parts: []message.ContentPart{message.TextContent{Text: "existing history suppresses title generation"}},
			})
			require.NoError(t, err)

			_, err = agent.Run(t.Context(), SessionAgentCall{
				SessionID:       sess.ID,
				Prompt:          "continue",
				MaxOutputTokens: tt.requested,
			})
			require.NoError(t, err)

			calls := provider.recordedCalls()
			require.Len(t, calls, 1)
			require.NotNil(t, calls[0].MaxOutputTokens)
			require.Equal(t, tt.expected, *calls[0].MaxOutputTokens)
		})
	}
}

func TestRunWithAutoCompactionDisabledFailsClosedWithoutProviderDispatch(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	provider := &compactionTestModel{}
	model := Model{
		Model: provider,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    4_096,
			DefaultMaxTokens: 512,
		},
	}
	agent := testSessionAgent(env, provider, provider, "system prompt").(*sessionAgent)
	agent.SetModels(model, model)
	agent.disableAutoSummarize = true

	sess, err := env.sessions.Create(t.Context(), "disabled compaction")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: strings.Repeat("oversized history ", 1_000)},
		},
	})
	require.NoError(t, err)

	_, err = agent.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "continue",
	})
	require.ErrorContains(t, err, "model request exceeds context window")
	require.Empty(t, provider.recordedCalls(), "disabled compaction must fail before provider dispatch")
	sess, err = env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Empty(t, sess.SummaryMessageID)
}

func TestRunRetryUsesReplacementModelsSmallerContextGuard(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	initial := &contextWindowTestModel{streamErr: &fantasy.ProviderError{
		Message:    "expired credentials",
		StatusCode: http.StatusUnauthorized,
	}}
	replacement := &contextWindowTestModel{}
	initialModel := Model{
		Model: initial,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    4_096,
			DefaultMaxTokens: 512,
		},
	}
	replacementModel := Model{
		Model: replacement,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    2_048,
			DefaultMaxTokens: 512,
		},
	}
	agent := testSessionAgent(env, initial, initial, "system prompt").(*sessionAgent)
	agent.SetModels(initialModel, initialModel)

	sess, err := env.sessions.Create(t.Context(), "retry context")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: strings.Repeat("word ", 700)},
		},
	})
	require.NoError(t, err)

	refreshCalls := 0
	_, err = agent.Run(t.Context(), SessionAgentCall{
		SessionID:       sess.ID,
		Prompt:          "continue",
		MaxOutputTokens: 512,
		OnAuthRefresh: func(context.Context, *fantasy.ProviderError) error {
			refreshCalls++
			agent.SetModels(replacementModel, replacementModel)
			return nil
		},
	})
	require.ErrorContains(t, err, "model request exceeds context window")
	require.Equal(t, 1, refreshCalls)
	require.Equal(t, int64(1), initial.streamCalls.Load())
	require.Zero(t, replacement.streamCalls.Load(), "replacement provider must be guarded by its own smaller context")
}
