package agent

import (
	"context"
	"strings"
	"testing"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"github.com/stretchr/testify/require"
)

type memoryPrefetchTwoStepAgent struct {
	t                 *testing.T
	afterFirstPrepare func()
	preparedSteps     [][]fantasy.Message
}

func (memoryPrefetchTwoStepAgent) Generate(context.Context, fantasy.AgentCall) (*fantasy.AgentResult, error) {
	return &fantasy.AgentResult{}, nil
}

func (a *memoryPrefetchTwoStepAgent) Stream(ctx context.Context, call fantasy.AgentStreamCall) (*fantasy.AgentResult, error) {
	preparedCtx, prepared, err := call.PrepareStep(ctx, fantasy.PrepareStepFunctionOptions{Messages: call.Messages})
	require.NoError(a.t, err)
	a.preparedSteps = append(a.preparedSteps, append([]fantasy.Message(nil), prepared.Messages...))

	if a.afterFirstPrepare != nil {
		a.afterFirstPrepare()
		a.afterFirstPrepare = nil
	}

	_, preparedSecond, err := call.PrepareStep(preparedCtx, fantasy.PrepareStepFunctionOptions{Messages: prepared.Messages})
	require.NoError(a.t, err)
	a.preparedSteps = append(a.preparedSteps, append([]fantasy.Message(nil), preparedSecond.Messages...))

	if call.OnTextDelta != nil {
		require.NoError(a.t, call.OnTextDelta("reply", "ok"))
	}
	if call.OnStepFinish != nil {
		require.NoError(a.t, call.OnStepFinish(fantasy.StepResult{Response: fantasy.Response{FinishReason: fantasy.FinishReasonStop}}))
	}

	return &fantasy.AgentResult{}, nil
}

type memoryPrefetchSingleStepAgent struct {
	t             *testing.T
	beforePrepare func()
	preparedByRun [][]fantasy.Message
}

func (memoryPrefetchSingleStepAgent) Generate(context.Context, fantasy.AgentCall) (*fantasy.AgentResult, error) {
	return &fantasy.AgentResult{}, nil
}

func (a *memoryPrefetchSingleStepAgent) Stream(ctx context.Context, call fantasy.AgentStreamCall) (*fantasy.AgentResult, error) {
	if a.beforePrepare != nil {
		a.beforePrepare()
		a.beforePrepare = nil
	}

	_, prepared, err := call.PrepareStep(ctx, fantasy.PrepareStepFunctionOptions{Messages: call.Messages})
	require.NoError(a.t, err)
	a.preparedByRun = append(a.preparedByRun, append([]fantasy.Message(nil), prepared.Messages...))

	if call.OnTextDelta != nil {
		require.NoError(a.t, call.OnTextDelta("reply", "ok"))
	}
	if call.OnStepFinish != nil {
		require.NoError(a.t, call.OnStepFinish(fantasy.StepResult{Response: fantasy.Response{FinishReason: fantasy.FinishReasonStop}}))
	}

	return &fantasy.AgentResult{}, nil
}

type memoryPrefetchAnthropicRetryAgent struct {
	t                        *testing.T
	onFirstAttemptBeforeStep func()
	attempts                 int
	preparedByAttempt        [][]fantasy.Message
}

func (memoryPrefetchAnthropicRetryAgent) Generate(context.Context, fantasy.AgentCall) (*fantasy.AgentResult, error) {
	return &fantasy.AgentResult{}, nil
}

func (a *memoryPrefetchAnthropicRetryAgent) Stream(ctx context.Context, call fantasy.AgentStreamCall) (*fantasy.AgentResult, error) {
	a.attempts++
	if a.attempts == 1 && a.onFirstAttemptBeforeStep != nil {
		a.onFirstAttemptBeforeStep()
		a.onFirstAttemptBeforeStep = nil
	}

	_, prepared, err := call.PrepareStep(ctx, fantasy.PrepareStepFunctionOptions{Messages: call.Messages})
	require.NoError(a.t, err)
	a.preparedByAttempt = append(a.preparedByAttempt, append([]fantasy.Message(nil), prepared.Messages...))

	if a.attempts == 1 {
		return nil, &fantasy.ProviderError{
			StatusCode: 400,
			Message:    "thinking is enabled but reasoning_content is missing in assistant tool call message at index 86",
		}
	}

	if call.OnTextDelta != nil {
		require.NoError(a.t, call.OnTextDelta("reply", "ok"))
	}
	if call.OnStepFinish != nil {
		require.NoError(a.t, call.OnStepFinish(fantasy.StepResult{Response: fantasy.Response{FinishReason: fantasy.FinishReasonStop}}))
	}

	return &fantasy.AgentResult{}, nil
}

func TestRunInjectsSettledMemoryPrefetchOnLaterStep(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	fakeAgent := &memoryPrefetchTwoStepAgent{t: t}
	sessionAgent := newQueuePrepareTestSessionAgent(env, fakeAgent)

	sess, err := env.sessions.Create(t.Context(), "memory prefetch later step")
	require.NoError(t, err)

	prefetch := &MemoryPrefetch{}
	fakeAgent.afterFirstPrepare = func() {
		prefetch.Settle("remember to use strict mode")
	}

	result, err := sessionAgent.Run(t.Context(), SessionAgentCall{
		SessionID:       sess.ID,
		Prompt:          "help me fix lint",
		MaxOutputTokens: 1000,
		NonInteractive:  true,
		MemoryPrefetch:  prefetch,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, fakeAgent.preparedSteps, 2)

	require.False(t, hasAutoRecallBlock(fakeAgent.preparedSteps[0], "strict mode"))
	require.True(t, hasAutoRecallBlock(fakeAgent.preparedSteps[1], "strict mode"))
}

func TestMemoryPrefetchCanBeReusedAcrossRuns(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	fakeAgent := &memoryPrefetchSingleStepAgent{t: t}
	sessionAgent := newQueuePrepareTestSessionAgent(env, fakeAgent)

	sess, err := env.sessions.Create(t.Context(), "memory prefetch retry")
	require.NoError(t, err)

	prefetch := &MemoryPrefetch{}

	_, err = sessionAgent.Run(t.Context(), SessionAgentCall{
		SessionID:       sess.ID,
		Prompt:          "first attempt",
		MaxOutputTokens: 1000,
		NonInteractive:  true,
		MemoryPrefetch:  prefetch,
	})
	require.NoError(t, err)

	fakeAgent.beforePrepare = func() {
		prefetch.Settle("retry should still include this recall")
	}

	_, err = sessionAgent.Run(t.Context(), SessionAgentCall{
		SessionID:       sess.ID,
		Prompt:          "retry attempt",
		MaxOutputTokens: 1000,
		NonInteractive:  true,
		MemoryPrefetch:  prefetch,
	})
	require.NoError(t, err)
	require.Len(t, fakeAgent.preparedByRun, 2)

	require.False(t, hasAutoRecallBlock(fakeAgent.preparedByRun[0], "retry should still include this recall"))
	require.True(t, hasAutoRecallBlock(fakeAgent.preparedByRun[1], "retry should still include this recall"))

	result1, settled1 := prefetch.GetSettled()
	result2, settled2 := prefetch.GetSettled()
	require.True(t, settled1)
	require.True(t, settled2)
	require.Equal(t, result1, result2)
}

func TestRunReinjectsMemoryPrefetchAfterAnthropicRetry(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	prefetch := &MemoryPrefetch{}
	fakeAgent := &memoryPrefetchAnthropicRetryAgent{t: t}
	sessionAgent := newQueuePrepareTestSessionAgent(env, fakeAgent)

	sess, err := env.sessions.Create(t.Context(), "memory prefetch anthropic retry")
	require.NoError(t, err)

	fakeAgent.onFirstAttemptBeforeStep = func() {
		prefetch.Settle("retry recall should still be present")
	}

	result, err := sessionAgent.Run(t.Context(), SessionAgentCall{
		SessionID:       sess.ID,
		Prompt:          "retry with anthropic thinking",
		MaxOutputTokens: 1000,
		NonInteractive:  true,
		MemoryPrefetch:  prefetch,
		ProviderOptions: fantasy.ProviderOptions{
			anthropic.Name: &anthropic.ProviderOptions{
				Thinking: &anthropic.ThinkingProviderOption{BudgetTokens: 2000},
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, fakeAgent.preparedByAttempt, 2)

	require.True(t, hasAutoRecallBlock(fakeAgent.preparedByAttempt[0], "retry recall should still be present"))
	require.True(t, hasAutoRecallBlock(fakeAgent.preparedByAttempt[1], "retry recall should still be present"))
}

func hasAutoRecallBlock(messages []fantasy.Message, snippet string) bool {
	for _, msg := range messages {
		if msg.Role != fantasy.MessageRoleSystem {
			continue
		}
		for _, part := range msg.Content {
			textPart, ok := part.(fantasy.TextPart)
			if !ok {
				continue
			}
			if strings.Contains(textPart.Text, "<auto_recall>") && strings.Contains(textPart.Text, snippet) {
				return true
			}
		}
	}
	return false
}
