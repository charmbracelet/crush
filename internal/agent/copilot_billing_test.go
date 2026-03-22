package agent

import (
	"context"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/stretchr/testify/require"
)

func TestRunMarksSubagentFirstStepAsAgentInitiated(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "subagent billing")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), testSession.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "seed history"}},
	})
	require.NoError(t, err)

	capturingAgent := &capturingInitiatorAgent{t: t}
	sessionAgent := NewSessionAgent(SessionAgentOptions{
		LargeModel: Model{
			CatwalkCfg: catwalk.Model{
				ContextWindow:    200000,
				DefaultMaxTokens: 1000,
			},
			ModelCfg: config.SelectedModel{
				Model:    "gpt-5",
				Provider: "copilot",
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
		IsSubAgent:   true,
		IsYolo:       true,
		Sessions:     env.sessions,
		Messages:     env.messages,
		AgentFactory: func(model fantasy.LanguageModel, opts ...fantasy.AgentOption) fantasy.Agent {
			return capturingAgent
		},
	})

	result, err := sessionAgent.Run(t.Context(), SessionAgentCall{
		Prompt:          "Search the web for release notes",
		SessionID:       testSession.ID,
		MaxOutputTokens: 1000,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, []string{copilot.InitiatorAgent}, capturingAgent.initiators)
}

func TestRunBillsEachUserTurnInMainSessionAsUserInitiated(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "main agent billing")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), testSession.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "seed history"}},
	})
	require.NoError(t, err)

	capturingAgent := &capturingInitiatorAgent{t: t}
	sessionAgent := NewSessionAgent(SessionAgentOptions{
		LargeModel: Model{
			CatwalkCfg: catwalk.Model{
				ContextWindow:    200000,
				DefaultMaxTokens: 1000,
			},
			ModelCfg: config.SelectedModel{
				Model:    "gpt-5",
				Provider: "copilot",
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
			return capturingAgent
		},
	})

	firstResult, err := sessionAgent.Run(t.Context(), SessionAgentCall{
		Prompt:          "First user request",
		SessionID:       testSession.ID,
		MaxOutputTokens: 1000,
	})
	require.NoError(t, err)
	require.NotNil(t, firstResult)

	secondResult, err := sessionAgent.Run(t.Context(), SessionAgentCall{
		Prompt:          "Second user request",
		SessionID:       testSession.ID,
		MaxOutputTokens: 1000,
	})
	require.NoError(t, err)
	require.NotNil(t, secondResult)

	require.Equal(t, []string{copilot.InitiatorUser, copilot.InitiatorUser}, capturingAgent.initiators)
}

type capturingInitiatorAgent struct {
	t          *testing.T
	initiators []string
}

func (a *capturingInitiatorAgent) Generate(context.Context, fantasy.AgentCall) (*fantasy.AgentResult, error) {
	return nil, nil
}

func (a *capturingInitiatorAgent) Stream(ctx context.Context, call fantasy.AgentStreamCall) (*fantasy.AgentResult, error) {
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

	if call.OnTextDelta != nil {
		require.NoError(a.t, call.OnTextDelta("text-1", "ok"))
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
