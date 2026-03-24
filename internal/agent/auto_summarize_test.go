package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/plugin"
	"github.com/stretchr/testify/require"
)

type autoSummarizeTestAgent struct {
	t            *testing.T
	runCalls     int
	summaryCalls int
	stepUsage    fantasy.Usage
	stepUsages   []fantasy.Usage
	afterStep    func()
	runErr       error
	runErrs      []error
	errAfterStep bool
}

func (a *autoSummarizeTestAgent) Generate(context.Context, fantasy.AgentCall) (*fantasy.AgentResult, error) {
	return nil, nil
}

func (a *autoSummarizeTestAgent) Stream(ctx context.Context, call fantasy.AgentStreamCall) (*fantasy.AgentResult, error) {
	if call.PrepareStep != nil {
		preparedCtx, _, err := call.PrepareStep(ctx, fantasy.PrepareStepFunctionOptions{Messages: call.Messages})
		require.NoError(a.t, err)
		ctx = preparedCtx
	}

	isSummary := call.OnStepFinish == nil
	if isSummary {
		a.summaryCalls++
		if call.OnTextDelta != nil {
			require.NoError(a.t, call.OnTextDelta("summary", "summary"))
		}
		return &fantasy.AgentResult{}, nil
	}

	a.runCalls++
	runErr := a.runErr
	if len(a.runErrs) > 0 {
		runErr = a.runErrs[0]
		a.runErrs = a.runErrs[1:]
	}
	if runErr != nil && !a.errAfterStep {
		return nil, runErr
	}
	if call.OnTextDelta != nil {
		require.NoError(a.t, call.OnTextDelta("text", "ok"))
	}

	stepUsage := a.stepUsage
	if len(a.stepUsages) > 0 {
		stepUsage = a.stepUsages[0]
		a.stepUsages = a.stepUsages[1:]
	}

	stepResult := fantasy.StepResult{
		Response: fantasy.Response{
			FinishReason: fantasy.FinishReasonStop,
			Usage:        stepUsage,
		},
	}
	if call.OnStepFinish != nil {
		require.NoError(a.t, call.OnStepFinish(stepResult))
	}
	if a.afterStep != nil {
		a.afterStep()
	}
	if runErr != nil && a.errAfterStep {
		return nil, runErr
	}
	for _, cond := range call.StopWhen {
		if cond([]fantasy.StepResult{stepResult}) {
			break
		}
	}
	return &fantasy.AgentResult{}, nil
}

type failOnceMessageService struct {
	message.Service
	mu           sync.Mutex
	failNextList bool
}

func (s *failOnceMessageService) FailNextList() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failNextList = true
}

func (s *failOnceMessageService) List(ctx context.Context, sessionID string) ([]message.Message, error) {
	s.mu.Lock()
	shouldFail := s.failNextList
	if shouldFail {
		s.failNextList = false
	}
	s.mu.Unlock()
	if shouldFail {
		return nil, errors.New("forced list failure")
	}
	return s.Service.List(ctx, sessionID)
}

func newAutoSummarizeTestSessionAgent(_ *testing.T, env fakeEnv, fakeAgent fantasy.Agent, messages message.Service, contextWindow int64) SessionAgent {
	model := Model{
		CatwalkCfg: catwalk.Model{
			ContextWindow:    contextWindow,
			DefaultMaxTokens: 1000,
		},
	}

	return NewSessionAgent(SessionAgentOptions{
		LargeModel:   model,
		SmallModel:   model,
		SystemPrompt: "",
		WorkingDir:   env.workingDir,
		IsYolo:       true,
		Sessions:     env.sessions,
		Messages:     messages,
		AgentFactory: func(model fantasy.LanguageModel, opts ...fantasy.AgentOption) fantasy.Agent {
			return fakeAgent
		},
	})
}

func TestRunPreflightAutoSummarizesBeforeRequest(t *testing.T) {
	plugin.Reset()
	t.Cleanup(plugin.Reset)

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "preflight summarize")
	require.NoError(t, err)

	_, err = env.messages.Create(t.Context(), testSession.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: strings.Repeat("x", 30000)}},
	})
	require.NoError(t, err)

	fakeAgent := &autoSummarizeTestAgent{t: t}
	sessionAgent := newAutoSummarizeTestSessionAgent(t, env, fakeAgent, env.messages, 10000)

	result, err := sessionAgent.Run(t.Context(), SessionAgentCall{
		Prompt:          "hello",
		SessionID:       testSession.ID,
		MaxOutputTokens: 1000,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, fakeAgent.summaryCalls)
	require.Equal(t, 1, fakeAgent.runCalls)

	savedSession, err := env.sessions.Get(t.Context(), testSession.ID)
	require.NoError(t, err)
	require.NotEmpty(t, savedSession.SummaryMessageID)
}

func TestRunPreflightAutoSummarizesWhenLastInputTokensAlreadyNearThreshold(t *testing.T) {
	t.Parallel()
	plugin.Reset()
	t.Cleanup(plugin.Reset)

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "preflight summarize by last input")
	require.NoError(t, err)

	_, err = env.messages.Create(t.Context(), testSession.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "small history"}},
	})
	require.NoError(t, err)

	testSession.LastPromptTokens = 168_000
	_, err = env.sessions.Save(t.Context(), testSession)
	require.NoError(t, err)

	fakeAgent := &autoSummarizeTestAgent{t: t}
	sessionAgent := newAutoSummarizeTestSessionAgent(t, env, fakeAgent, env.messages, 200_000)

	result, err := sessionAgent.Run(t.Context(), SessionAgentCall{
		Prompt:          "hello",
		SessionID:       testSession.ID,
		MaxOutputTokens: 50_000,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, fakeAgent.summaryCalls)
	require.Equal(t, 1, fakeAgent.runCalls)

	savedSession, err := env.sessions.Get(t.Context(), testSession.ID)
	require.NoError(t, err)
	require.NotEmpty(t, savedSession.SummaryMessageID)
}

func TestRunStepAutoSummarizesWhenEstimateFallbackExceedsThreshold(t *testing.T) {
	plugin.Reset()
	t.Cleanup(plugin.Reset)

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "step summarize fallback")
	require.NoError(t, err)
	createSeedHistoryMessage(t, env, testSession.ID)

	messages := &failOnceMessageService{Service: env.messages}
	fakeAgent := &autoSummarizeTestAgent{
		t: t,
		stepUsage: fantasy.Usage{
			InputTokens:  6000,
			OutputTokens: 10,
		},
		afterStep: messages.FailNextList,
	}
	sessionAgent := newAutoSummarizeTestSessionAgent(t, env, fakeAgent, messages, 10000)

	result, err := sessionAgent.Run(t.Context(), SessionAgentCall{
		Prompt:          "hello",
		SessionID:       testSession.ID,
		MaxOutputTokens: 1000,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, fakeAgent.runCalls)
	require.Equal(t, 1, fakeAgent.summaryCalls)

	savedSession, err := env.sessions.Get(t.Context(), testSession.ID)
	require.NoError(t, err)
	require.NotEmpty(t, savedSession.SummaryMessageID)
}

func TestRunStepAutoSummarizeFallbackIgnoresOutputTokens(t *testing.T) {
	plugin.Reset()
	t.Cleanup(plugin.Reset)

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "step summarize ignores output")
	require.NoError(t, err)
	createSeedHistoryMessage(t, env, testSession.ID)

	messages := &failOnceMessageService{Service: env.messages}
	fakeAgent := &autoSummarizeTestAgent{
		t: t,
		stepUsage: fantasy.Usage{
			InputTokens:  4000,
			OutputTokens: 9000,
		},
		afterStep: messages.FailNextList,
	}
	sessionAgent := newAutoSummarizeTestSessionAgent(t, env, fakeAgent, messages, 10000)

	result, err := sessionAgent.Run(t.Context(), SessionAgentCall{
		Prompt:          "hello",
		SessionID:       testSession.ID,
		MaxOutputTokens: 1000,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, fakeAgent.runCalls)
	require.Equal(t, 0, fakeAgent.summaryCalls)

	savedSession, err := env.sessions.Get(t.Context(), testSession.ID)
	require.NoError(t, err)
	require.Empty(t, savedSession.SummaryMessageID)
	require.Equal(t, int64(4000), savedSession.LastInputTokens())
	require.Equal(t, int64(9000), savedSession.LastOutputTokens())
}

func TestRunTransientRetryNearContextLimitSummarizesInsteadOfRetrying(t *testing.T) {
	t.Parallel()
	plugin.Reset()
	t.Cleanup(plugin.Reset)

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "retry summarize")
	require.NoError(t, err)
	createSeedHistoryMessage(t, env, testSession.ID)

	fakeAgent := &autoSummarizeTestAgent{
		t: t,
		stepUsages: []fantasy.Usage{
			{
				InputTokens:  168_000,
				OutputTokens: 10,
			},
			{
				InputTokens:  100,
				OutputTokens: 10,
			},
		},
		runErrs: []error{
			&fantasy.ProviderError{
				StatusCode: 503,
				Message:    "service temporarily unavailable",
			},
		},
		errAfterStep: true,
	}
	sessionAgent := newAutoSummarizeTestSessionAgent(t, env, fakeAgent, env.messages, 200_000)

	result, err := sessionAgent.Run(t.Context(), SessionAgentCall{
		Prompt:          "hello",
		SessionID:       testSession.ID,
		MaxOutputTokens: 50_000,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 2, fakeAgent.runCalls)
	require.Equal(t, 1, fakeAgent.summaryCalls)

	savedSession, err := env.sessions.Get(t.Context(), testSession.ID)
	require.NoError(t, err)
	require.NotEmpty(t, savedSession.SummaryMessageID)
	require.Equal(t, int64(100), savedSession.LastInputTokens())
}
