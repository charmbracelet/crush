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

type compactingPurposePlugin struct {
	purposes        []plugin.ChatTransformPurpose
	messagePurposes []plugin.ChatTransformPurpose
}

func (p *compactingPurposePlugin) Name() string { return "compacting-purpose-plugin" }

func (p *compactingPurposePlugin) Init(context.Context, plugin.PluginInput) (plugin.Hooks, error) {
	return plugin.Hooks{
		ChatMessagesTransform: func(_ context.Context, input plugin.ChatMessagesTransformInput, _ *plugin.ChatMessagesTransformOutput) error {
			p.messagePurposes = append(p.messagePurposes, input.Purpose)
			return nil
		},
		SessionCompacting: func(_ context.Context, input plugin.SessionCompactingInput, _ *plugin.SessionCompactingOutput) error {
			p.purposes = append(p.purposes, input.Purpose)
			return nil
		},
	}, nil
}

func (p *compactingPurposePlugin) Close(context.Context) error { return nil }

func initCompactingPurposePlugin(t *testing.T, env fakeEnv) *compactingPurposePlugin {
	tracker := &compactingPurposePlugin{}
	plugin.Register(tracker)
	err := plugin.Init(context.Background(), plugin.PluginInput{
		Sessions:   env.sessions,
		Messages:   env.messages,
		WorkingDir: env.workingDir,
	})
	require.NoError(t, err)
	return tracker
}

type autoSummarizeTestAgent struct {
	t                       *testing.T
	runCalls                int
	summaryCalls            int
	stepUsage               fantasy.Usage
	stepUsages              []fantasy.Usage
	afterStep               func()
	runErr                  error
	runErrs                 []error
	errAfterStep            bool
	startToolBeforeRunError bool
	toolCallID              string
	toolName                string
}

func (a *autoSummarizeTestAgent) Generate(context.Context, fantasy.AgentCall) (*fantasy.AgentResult, error) {
	return nil, nil
}

func (a *autoSummarizeTestAgent) Stream(ctx context.Context, call fantasy.AgentStreamCall) (*fantasy.AgentResult, error) {
	if call.PrepareStep != nil {
		_, _, err := call.PrepareStep(ctx, fantasy.PrepareStepFunctionOptions{Messages: call.Messages})
		require.NoError(a.t, err)
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
		if a.startToolBeforeRunError && call.OnToolInputStart != nil {
			toolCallID := a.toolCallID
			if toolCallID == "" {
				toolCallID = "tool-call-before-error"
			}
			toolName := a.toolName
			if toolName == "" {
				toolName = "view"
			}
			require.NoError(a.t, call.OnToolInputStart(toolCallID, toolName))
		}
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
	purposeTracker := initCompactingPurposePlugin(t, env)
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
	require.Contains(t, purposeTracker.messagePurposes, plugin.ChatTransformPurposeMicroCompact)
	require.Contains(t, purposeTracker.messagePurposes, plugin.ChatTransformPurposeCollapse)
	require.Contains(t, purposeTracker.messagePurposes, plugin.ChatTransformPurposeReactiveCompact)
	require.Equal(t, []plugin.ChatTransformPurpose{plugin.ChatTransformPurposeProactiveCompact}, purposeTracker.purposes)

	savedSession, err := env.sessions.Get(t.Context(), testSession.ID)
	require.NoError(t, err)
	require.NotEmpty(t, savedSession.SummaryMessageID)
}

func TestRunPreflightAutoSummarizesWhenLastInputTokensAlreadyNearThreshold(t *testing.T) {
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
	plugin.Reset()
	t.Cleanup(plugin.Reset)

	env := testEnv(t)
	purposeTracker := initCompactingPurposePlugin(t, env)
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
	require.Contains(t, purposeTracker.messagePurposes, plugin.ChatTransformPurposeMicroCompact)
	require.Contains(t, purposeTracker.messagePurposes, plugin.ChatTransformPurposeCollapse)
	require.Contains(t, purposeTracker.messagePurposes, plugin.ChatTransformPurposeReactiveCompact)
	require.Equal(t, []plugin.ChatTransformPurpose{plugin.ChatTransformPurposeRecover}, purposeTracker.purposes)

	savedSession, err := env.sessions.Get(t.Context(), testSession.ID)
	require.NoError(t, err)
	require.NotEmpty(t, savedSession.SummaryMessageID)
	require.Equal(t, int64(100), savedSession.LastInputTokens())
}

func TestRunStreamingContextWindowErrorStringForcesSummarizeRecovery(t *testing.T) {
	plugin.Reset()
	t.Cleanup(plugin.Reset)

	env := testEnv(t)
	purposeTracker := initCompactingPurposePlugin(t, env)
	testSession, err := env.sessions.Create(t.Context(), "streaming context-window recover")
	require.NoError(t, err)

	const toolCallID = "tool-call-before-overflow"
	fakeAgent := &autoSummarizeTestAgent{
		t:                       t,
		startToolBeforeRunError: true,
		toolCallID:              toolCallID,
		runErrs: []error{
			errors.New("received error while streaming: {\"message\":\"{\\\"error\\\":{\\\"message\\\":\\\"Your input exceeds the context window of this model. Please adjust your input and try again.\\\",\\\"code\\\":\\\"invalid_request_body\\\"}}\"}"),
			nil,
		},
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
	require.Contains(t, purposeTracker.messagePurposes, plugin.ChatTransformPurposeMicroCompact)
	require.Contains(t, purposeTracker.messagePurposes, plugin.ChatTransformPurposeCollapse)
	require.Contains(t, purposeTracker.messagePurposes, plugin.ChatTransformPurposeReactiveCompact)
	require.Equal(t, []plugin.ChatTransformPurpose{plugin.ChatTransformPurposeRecover}, purposeTracker.purposes)

	savedSession, err := env.sessions.Get(t.Context(), testSession.ID)
	require.NoError(t, err)
	require.NotEmpty(t, savedSession.SummaryMessageID)

	msgs, err := env.messages.List(t.Context(), testSession.ID)
	require.NoError(t, err)
	var foundToolCall bool
	for _, msg := range msgs {
		for _, tc := range msg.ToolCalls() {
			if tc.ID == toolCallID {
				foundToolCall = true
				require.True(t, tc.Finished)
			}
		}
	}
	require.True(t, foundToolCall)

	var foundSyntheticToolResult bool
	for _, msg := range msgs {
		for _, tr := range msg.ToolResults() {
			if tr.ToolCallID == toolCallID {
				foundSyntheticToolResult = true
				require.True(t, tr.IsError)
				require.Contains(t, tr.Content, "error while executing the tool")
			}
		}
	}
	require.True(t, foundSyntheticToolResult)
}

func TestRunStreamingContextWindowRecoveryOnlyOnce(t *testing.T) {
	plugin.Reset()
	t.Cleanup(plugin.Reset)

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "streaming context-window no loop")
	require.NoError(t, err)

	streamErr := errors.New("received error while streaming: {\"message\":\"{\\\"error\\\":{\\\"message\\\":\\\"Your input exceeds the context window of this model. Please adjust your input and try again.\\\",\\\"code\\\":\\\"invalid_request_body\\\"}}\"}")
	fakeAgent := &autoSummarizeTestAgent{
		t:       t,
		runErrs: []error{streamErr, streamErr},
	}
	sessionAgent := newAutoSummarizeTestSessionAgent(t, env, fakeAgent, env.messages, 200_000)

	_, err = sessionAgent.Run(t.Context(), SessionAgentCall{
		Prompt:          "hello",
		SessionID:       testSession.ID,
		MaxOutputTokens: 50_000,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "context window")
	require.Equal(t, 2, fakeAgent.runCalls)
	require.Equal(t, 1, fakeAgent.summaryCalls)
}

func TestRunNormalSummarizeUsesSummarizePurpose(t *testing.T) {
	plugin.Reset()
	t.Cleanup(plugin.Reset)

	env := testEnv(t)
	purposeTracker := initCompactingPurposePlugin(t, env)
	testSession, err := env.sessions.Create(t.Context(), "normal summarize")
	require.NoError(t, err)
	createSeedHistoryMessage(t, env, testSession.ID)

	fakeAgent := &autoSummarizeTestAgent{
		t: t,
		afterStep: func() {
			_, createErr := env.messages.Create(t.Context(), testSession.ID, message.CreateMessageParams{
				Role: message.Tool,
				Parts: []message.ContentPart{
					message.ToolResult{ToolCallID: "tc-1", Name: "view", Content: strings.Repeat("x", 30000)},
				},
			})
			require.NoError(t, createErr)
		},
	}
	sessionAgent := newAutoSummarizeTestSessionAgent(t, env, fakeAgent, env.messages, 10000)

	result, err := sessionAgent.Run(t.Context(), SessionAgentCall{
		Prompt:          "hello",
		SessionID:       testSession.ID,
		MaxOutputTokens: 1000,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, fakeAgent.runCalls)
	require.Equal(t, 1, fakeAgent.summaryCalls)
	require.Contains(t, purposeTracker.messagePurposes, plugin.ChatTransformPurposeMicroCompact)
	require.Contains(t, purposeTracker.messagePurposes, plugin.ChatTransformPurposeCollapse)
	require.Contains(t, purposeTracker.messagePurposes, plugin.ChatTransformPurposeAutoCompact)
	require.Contains(t, purposeTracker.messagePurposes, plugin.ChatTransformPurposePostCompact)
	require.Equal(t, []plugin.ChatTransformPurpose{plugin.ChatTransformPurposeSummarize}, purposeTracker.purposes)

	savedSession, err := env.sessions.Get(t.Context(), testSession.ID)
	require.NoError(t, err)
	require.NotEmpty(t, savedSession.SummaryMessageID)
}

func TestRunContextWindowErrorAfterCompletedStepSummarizesWithoutAutoResume(t *testing.T) {
	plugin.Reset()
	t.Cleanup(plugin.Reset)

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "context-window after step")
	require.NoError(t, err)

	contextWindowErr := &fantasy.ProviderError{
		StatusCode: 400,
		Message:    "Your input exceeds the context window of this model.",
	}
	fakeAgent := &autoSummarizeTestAgent{
		t:            t,
		runErr:       contextWindowErr,
		errAfterStep: true,
	}
	sessionAgent := newAutoSummarizeTestSessionAgent(t, env, fakeAgent, env.messages, 200_000)

	_, err = sessionAgent.Run(t.Context(), SessionAgentCall{
		Prompt:          "hello",
		SessionID:       testSession.ID,
		MaxOutputTokens: 50_000,
	})
	require.NoError(t, err)
	require.Equal(t, 1, fakeAgent.runCalls)
	require.Equal(t, 1, fakeAgent.summaryCalls)
}

func TestSummarizeSkipsAutoCompactForTinyHistory(t *testing.T) {
	plugin.Reset()
	t.Cleanup(plugin.Reset)

	env := testEnv(t)
	purposeTracker := initCompactingPurposePlugin(t, env)
	testSession, err := env.sessions.Create(t.Context(), "tiny summarize")
	require.NoError(t, err)

	_, err = env.messages.Create(t.Context(), testSession.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "only one"}},
	})
	require.NoError(t, err)

	fakeAgent := &autoSummarizeTestAgent{t: t}
	sessionAgent := newAutoSummarizeTestSessionAgent(t, env, fakeAgent, env.messages, 10000)

	err = sessionAgent.Summarize(t.Context(), testSession.ID, nil)
	require.NoError(t, err)
	require.Equal(t, 1, fakeAgent.summaryCalls)
	require.Contains(t, purposeTracker.messagePurposes, plugin.ChatTransformPurposeMicroCompact)
	require.Contains(t, purposeTracker.messagePurposes, plugin.ChatTransformPurposeCollapse)
	require.NotContains(t, purposeTracker.messagePurposes, plugin.ChatTransformPurposeAutoCompact)
	require.NotContains(t, purposeTracker.messagePurposes, plugin.ChatTransformPurposePostCompact)
}
