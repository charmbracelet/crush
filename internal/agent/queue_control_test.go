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
