package agent

import (
	"context"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

type queueTestAgent struct{}

func (queueTestAgent) Generate(context.Context, fantasy.AgentCall) (*fantasy.AgentResult, error) {
	return &fantasy.AgentResult{}, nil
}

func (queueTestAgent) Stream(context.Context, fantasy.AgentStreamCall) (*fantasy.AgentResult, error) {
	return &fantasy.AgentResult{}, nil
}

type queuePrepareTestAgent struct {
	t                 *testing.T
	afterFirstPrepare func()
}

func (queuePrepareTestAgent) Generate(context.Context, fantasy.AgentCall) (*fantasy.AgentResult, error) {
	return &fantasy.AgentResult{}, nil
}

func (a *queuePrepareTestAgent) Stream(ctx context.Context, call fantasy.AgentStreamCall) (*fantasy.AgentResult, error) {
	preparedCtx, prepared, err := call.PrepareStep(ctx, fantasy.PrepareStepFunctionOptions{Messages: call.Messages})
	require.NoError(a.t, err)

	if a.afterFirstPrepare != nil {
		a.afterFirstPrepare()
		a.afterFirstPrepare = nil
	}

	_, _, err = call.PrepareStep(preparedCtx, fantasy.PrepareStepFunctionOptions{Messages: prepared.Messages})
	require.NoError(a.t, err)

	if call.OnTextDelta != nil {
		require.NoError(a.t, call.OnTextDelta("reply", "ok"))
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

func newQueueControlTestAgent(env fakeEnv) *sessionAgent {
	return &sessionAgent{
		largeModel:         csync.NewValue(Model{CatwalkCfg: catwalk.Model{}, ModelCfg: config.SelectedModel{}}),
		smallModel:         csync.NewValue(Model{CatwalkCfg: catwalk.Model{}, ModelCfg: config.SelectedModel{}}),
		systemPromptPrefix: csync.NewValue(""),
		systemPrompt:       csync.NewValue(""),
		tools:              csync.NewSlice[fantasy.AgentTool](),
		agentFactory: func(fantasy.LanguageModel, ...fantasy.AgentOption) fantasy.Agent {
			return queueTestAgent{}
		},
		sessions:       env.sessions,
		messages:       env.messages,
		messageQueue:   csync.NewMap[string, []SessionAgentCall](),
		activeRequests: csync.NewMap[string, context.CancelFunc](),
		pausedQueues:   csync.NewMap[string, bool](),
	}
}

func newQueuePrepareTestSessionAgent(env fakeEnv, fakeAgent fantasy.Agent) *sessionAgent {
	model := Model{
		CatwalkCfg: catwalk.Model{
			ContextWindow:    10000,
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
		Messages:     env.messages,
		AgentFactory: func(fantasy.LanguageModel, ...fantasy.AgentOption) fantasy.Agent {
			return fakeAgent
		},
	}).(*sessionAgent)
}

func TestResumeQueueStartsNextPromptWhenIdle(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	a := newQueueControlTestAgent(env)

	sess, err := env.sessions.Create(t.Context(), "queue resume")
	require.NoError(t, err)

	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "seed"},
		},
	})
	require.NoError(t, err)

	a.messageQueue.Set(sess.ID, []SessionAgentCall{{
		SessionID: sess.ID,
		Prompt:    "queued prompt",
	}})
	a.pausedQueues.Set(sess.ID, true)

	a.ResumeQueue(sess.ID)

	require.Eventually(t, func() bool {
		if a.QueuedPrompts(sess.ID) != 0 || a.IsSessionBusy(sess.ID) {
			return false
		}
		msgs, listErr := env.messages.List(t.Context(), sess.ID)
		if listErr != nil {
			return false
		}
		for _, msg := range msgs {
			if msg.Role == message.User && msg.Content().Text == "queued prompt" {
				return true
			}
		}
		return false
	}, time.Second, 20*time.Millisecond)
	require.False(t, a.IsQueuePaused(sess.ID))
}

func TestResumeQueueDoesNotStartWhenBusy(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	a := newQueueControlTestAgent(env)

	sess, err := env.sessions.Create(t.Context(), "queue busy")
	require.NoError(t, err)

	a.messageQueue.Set(sess.ID, []SessionAgentCall{{
		SessionID: sess.ID,
		Prompt:    "queued",
	}})
	a.pausedQueues.Set(sess.ID, true)
	a.activeRequests.Set(sess.ID, func() {})

	a.ResumeQueue(sess.ID)

	require.Equal(t, 1, a.QueuedPrompts(sess.ID))
	require.False(t, a.IsQueuePaused(sess.ID))
}

func TestCancelClearsQueuePauseState(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	a := newQueueControlTestAgent(env)

	sess, err := env.sessions.Create(t.Context(), "queue cancel")
	require.NoError(t, err)

	a.messageQueue.Set(sess.ID, []SessionAgentCall{{
		SessionID: sess.ID,
		Prompt:    "queued",
	}})
	a.pausedQueues.Set(sess.ID, true)

	a.Cancel(sess.ID)

	require.Equal(t, 0, a.QueuedPrompts(sess.ID))
	require.False(t, a.IsQueuePaused(sess.ID))
}

func TestRemoveQueuedPromptClearsPauseWhenQueueEmpties(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	a := newQueueControlTestAgent(env)

	sess, err := env.sessions.Create(t.Context(), "queue remove")
	require.NoError(t, err)

	a.messageQueue.Set(sess.ID, []SessionAgentCall{{
		SessionID: sess.ID,
		Prompt:    "queued",
	}})
	a.pausedQueues.Set(sess.ID, true)

	removed := a.RemoveQueuedPrompt(sess.ID, 0)
	require.True(t, removed)
	require.Equal(t, 0, a.QueuedPrompts(sess.ID))
	require.False(t, a.IsQueuePaused(sess.ID))
}

func TestQueuedPromptWaitsForCurrentRunByDefault(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	var sessionAgent *sessionAgent
	testAgent := &queuePrepareTestAgent{t: t}
	sessionAgent = newQueuePrepareTestSessionAgent(env, testAgent)

	sess, err := env.sessions.Create(t.Context(), "queue waits")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "seed"},
		},
	})
	require.NoError(t, err)

	sessionAgent.PauseQueue(sess.ID)
	testAgent.afterFirstPrepare = func() {
		_, runErr := sessionAgent.Run(context.Background(), SessionAgentCall{
			SessionID:       sess.ID,
			Prompt:          "queued later",
			MaxOutputTokens: 1000,
		})
		require.NoError(t, runErr)
	}

	result, err := sessionAgent.Run(t.Context(), SessionAgentCall{
		SessionID:       sess.ID,
		Prompt:          "run now",
		MaxOutputTokens: 1000,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Equal(t, 1, sessionAgent.QueuedPrompts(sess.ID))

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	for _, msg := range msgs {
		if msg.Role == message.User && msg.Content().Text == "queued later" {
			t.Fatalf("queued prompt was merged into the active run")
		}
	}
}

func TestJoinActiveRunQueuedPromptMergesIntoCurrentRun(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	var sessionAgent *sessionAgent
	testAgent := &queuePrepareTestAgent{t: t}
	sessionAgent = newQueuePrepareTestSessionAgent(env, testAgent)

	sess, err := env.sessions.Create(t.Context(), "queue joins run")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "seed"},
		},
	})
	require.NoError(t, err)

	sessionAgent.PauseQueue(sess.ID)
	testAgent.afterFirstPrepare = func() {
		_, runErr := sessionAgent.Run(context.Background(), SessionAgentCall{
			SessionID:       sess.ID,
			Prompt:          "join now",
			JoinActiveRun:   true,
			MaxOutputTokens: 1000,
		})
		require.NoError(t, runErr)
	}

	result, err := sessionAgent.Run(t.Context(), SessionAgentCall{
		SessionID:       sess.ID,
		Prompt:          "run now",
		MaxOutputTokens: 1000,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Equal(t, 0, sessionAgent.QueuedPrompts(sess.ID))

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	foundJoinedPrompt := false
	for _, msg := range msgs {
		if msg.Role == message.User && msg.Content().Text == "join now" {
			foundJoinedPrompt = true
			break
		}
	}
	require.True(t, foundJoinedPrompt)
}
