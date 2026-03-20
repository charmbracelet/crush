package agent

import (
	"context"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/stretchr/testify/require"
)

func TestRunRetriesWithoutAnthropicThinkingOnUnsignedReasoningError(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "retry test")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), testSession.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "seed history"}},
	})
	require.NoError(t, err)

	retryingAgent := &scriptedRetryAgent{t: t}
	budget := int64(2000)
	sessionAgent := NewSessionAgent(SessionAgentOptions{
		LargeModel: Model{
			CatwalkCfg: catwalk.Model{
				ContextWindow:    200000,
				DefaultMaxTokens: 1000,
			},
			ModelCfg: config.SelectedModel{
				Model:    "kimi-k2.5",
				Provider: "anthropic-proxy",
			},
		},
		SmallModel: Model{
			CatwalkCfg: catwalk.Model{
				ContextWindow:    200000,
				DefaultMaxTokens: 1000,
			},
		},
		SystemPrompt: "",
		WorkingDir:   env.workingDir,
		IsYolo:       true,
		Sessions:     env.sessions,
		Messages:     env.messages,
		AgentFactory: func(model fantasy.LanguageModel, opts ...fantasy.AgentOption) fantasy.Agent {
			return retryingAgent
		},
	})

	result, err := sessionAgent.Run(t.Context(), SessionAgentCall{
		Prompt:          "hello",
		SessionID:       testSession.ID,
		MaxOutputTokens: 1000,
		ProviderOptions: fantasy.ProviderOptions{
			anthropic.Name: &anthropic.ProviderOptions{
				Thinking: &anthropic.ThinkingProviderOption{BudgetTokens: budget},
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Len(t, retryingAgent.calls, 2)
	require.Equal(t, []string{copilot.InitiatorUser, copilot.InitiatorAgent}, retryingAgent.initiators)

	firstAnthropic, ok := retryingAgent.calls[0][anthropic.Name].(*anthropic.ProviderOptions)
	require.True(t, ok)
	require.NotNil(t, firstAnthropic.Thinking)
	require.Equal(t, budget, firstAnthropic.Thinking.BudgetTokens)

	secondAnthropic, ok := retryingAgent.calls[1][anthropic.Name].(*anthropic.ProviderOptions)
	require.True(t, ok)
	require.Nil(t, secondAnthropic.Thinking)

	msgs, err := env.messages.List(t.Context(), testSession.ID)
	require.NoError(t, err)

	var assistantCount int
	for _, msg := range msgs {
		if msg.Role != message.Assistant {
			continue
		}
		assistantCount++
		require.Equal(t, "fallback ok", msg.Content().Text)
	}
	require.Equal(t, 1, assistantCount, "failed first attempt assistant should be deleted before retry")
}

func TestRunDoesNotRetryWithoutAnthropicThinkingAfterCompletedStep(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "retry gate test")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), testSession.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "seed history"}},
	})
	require.NoError(t, err)

	agentWithCompletedStepFailure := &scriptedRetryAgent{
		t:                      t,
		failAfterCompletedStep: true,
	}
	budget := int64(2000)
	sessionAgent := NewSessionAgent(SessionAgentOptions{
		LargeModel: Model{
			CatwalkCfg: catwalk.Model{
				ContextWindow:    200000,
				DefaultMaxTokens: 1000,
			},
			ModelCfg: config.SelectedModel{
				Model:    "kimi-k2.5",
				Provider: "anthropic-proxy",
			},
		},
		SmallModel: Model{
			CatwalkCfg: catwalk.Model{
				ContextWindow:    200000,
				DefaultMaxTokens: 1000,
			},
		},
		SystemPrompt: "",
		WorkingDir:   env.workingDir,
		IsYolo:       true,
		Sessions:     env.sessions,
		Messages:     env.messages,
		AgentFactory: func(model fantasy.LanguageModel, opts ...fantasy.AgentOption) fantasy.Agent {
			return agentWithCompletedStepFailure
		},
	})

	_, err = sessionAgent.Run(t.Context(), SessionAgentCall{
		Prompt:          "hello",
		SessionID:       testSession.ID,
		MaxOutputTokens: 1000,
		ProviderOptions: fantasy.ProviderOptions{
			anthropic.Name: &anthropic.ProviderOptions{
				Thinking: &anthropic.ThinkingProviderOption{BudgetTokens: budget},
			},
		},
	})
	require.Error(t, err)
	require.Len(t, agentWithCompletedStepFailure.calls, 1, "fallback must not retry after a step already completed")
}

type scriptedRetryAgent struct {
	t                      *testing.T
	calls                  []fantasy.ProviderOptions
	initiators             []string
	failAfterCompletedStep bool
}

func (a *scriptedRetryAgent) Generate(context.Context, fantasy.AgentCall) (*fantasy.AgentResult, error) {
	return nil, nil
}

func (a *scriptedRetryAgent) Stream(ctx context.Context, call fantasy.AgentStreamCall) (*fantasy.AgentResult, error) {
	a.calls = append(a.calls, call.ProviderOptions)

	if call.PrepareStep != nil {
		callCtx, _, err := call.PrepareStep(ctx, fantasy.PrepareStepFunctionOptions{
			Messages: call.Messages,
		})
		require.NoError(a.t, err)
		if initiator, ok := callCtx.Value(copilot.InitiatorTypeKey).(string); ok {
			a.initiators = append(a.initiators, initiator)
		} else {
			a.initiators = append(a.initiators, "")
		}
	}

	if a.failAfterCompletedStep {
		if call.OnTextDelta != nil {
			require.NoError(a.t, call.OnTextDelta("text-1", "step completed"))
		}
		if call.OnStepFinish != nil {
			require.NoError(a.t, call.OnStepFinish(fantasy.StepResult{
				Response: fantasy.Response{
					FinishReason: fantasy.FinishReasonStop,
				},
			}))
		}
		return nil, &fantasy.ProviderError{
			StatusCode: 400,
			Message:    "thinking is enabled but reasoning_content is missing in assistant tool call message at index 86",
		}
	}

	if len(a.calls) == 1 {
		return nil, &fantasy.ProviderError{
			StatusCode: 400,
			Message:    "thinking is enabled but reasoning_content is missing in assistant tool call message at index 86",
		}
	}

	if call.OnTextDelta != nil {
		require.NoError(a.t, call.OnTextDelta("text-1", "fallback ok"))
	}
	if call.OnStepFinish != nil {
		require.NoError(a.t, call.OnStepFinish(fantasy.StepResult{
			Response: fantasy.Response{
				FinishReason: fantasy.FinishReasonStop,
			},
		}))
	}

	return &fantasy.AgentResult{}, nil
}
