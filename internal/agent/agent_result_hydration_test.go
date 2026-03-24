package agent

import (
	"context"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestRunHydratesResultTextFromAssistantMessageWhenStreamMissesTextEnd(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	model := stubLanguageModel{
		stream: func(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
			return func(yield func(fantasy.StreamPart) bool) {
				if !yield(fantasy.StreamPart{
					Type: fantasy.StreamPartTypeTextStart,
					ID:   "text-1",
				}) {
					return
				}
				if !yield(fantasy.StreamPart{
					Type:  fantasy.StreamPartTypeTextDelta,
					ID:    "text-1",
					Delta: "hello from delta only",
				}) {
					return
				}
				yield(fantasy.StreamPart{
					Type:         fantasy.StreamPartTypeFinish,
					FinishReason: fantasy.FinishReasonStop,
				})
			}, nil
		},
	}

	agent := NewSessionAgent(SessionAgentOptions{
		LargeModel: Model{
			Model: model,
			CatwalkCfg: catwalk.Model{
				ContextWindow:    200000,
				DefaultMaxTokens: 1000,
			},
			ModelCfg: config.SelectedModel{
				Provider: model.Provider(),
				Model:    model.Model(),
			},
		},
		SmallModel: Model{
			Model: model,
			CatwalkCfg: catwalk.Model{
				ContextWindow:    200000,
				DefaultMaxTokens: 1000,
			},
			ModelCfg: config.SelectedModel{
				Provider: model.Provider(),
				Model:    model.Model(),
			},
		},
		SystemPrompt:         "",
		WorkingDir:           env.workingDir,
		Sessions:             env.sessions,
		Messages:             env.messages,
		DisableAutoSummarize: true,
		IsYolo:               true,
	})

	sess, err := env.sessions.Create(t.Context(), "hydrate result")
	require.NoError(t, err)

	result, err := agent.Run(t.Context(), SessionAgentCall{
		SessionID:       sess.ID,
		Prompt:          "say hello",
		MaxOutputTokens: 100,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "hello from delta only", result.Response.Content.Text())
	require.Len(t, result.Steps, 1)
	require.Equal(t, "hello from delta only", result.Steps[0].Content.Text())
}
