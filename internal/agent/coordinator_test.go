package agent

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"unsafe"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openaicompat"
	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSessionAgent is a minimal mock for the SessionAgent interface.
type mockSessionAgent struct {
	model        Model
	runFunc      func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error)
	estimateFunc func(ctx context.Context, sessionID string, model Model) (int64, error)
	summarizeErr error
	summarized   []string
	cancelled    []string
}

func (m *mockSessionAgent) Run(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
	return m.runFunc(ctx, call)
}

func (m *mockSessionAgent) EstimateSessionPromptTokensForModel(ctx context.Context, sessionID string, model Model) (int64, error) {
	if m.estimateFunc == nil {
		return 0, nil
	}
	return m.estimateFunc(ctx, sessionID, model)
}

func (m *mockSessionAgent) Model() Model                                    { return m.model }
func (m *mockSessionAgent) SetModels(large, small Model)                    {}
func (m *mockSessionAgent) SetTools(tools []fantasy.AgentTool)              {}
func (m *mockSessionAgent) SetSystemPrompt(systemPrompt string)             {}
func (m *mockSessionAgent) SetSystemPromptPrefix(systemPromptPrefix string) {}
func (m *mockSessionAgent) Cancel(sessionID string) {
	m.cancelled = append(m.cancelled, sessionID)
}
func (m *mockSessionAgent) CancelAll()                                          {}
func (m *mockSessionAgent) IsSessionBusy(sessionID string) bool                 { return false }
func (m *mockSessionAgent) IsBusy() bool                                        { return false }
func (m *mockSessionAgent) QueuedPrompts(sessionID string) int                  { return 0 }
func (m *mockSessionAgent) QueuedPromptsList(sessionID string) []string         { return nil }
func (m *mockSessionAgent) RemoveQueuedPrompt(sessionID string, index int) bool { return false }
func (m *mockSessionAgent) ClearQueue(sessionID string)                         {}
func (m *mockSessionAgent) PauseQueue(sessionID string)                         {}
func (m *mockSessionAgent) ResumeQueue(sessionID string)                        {}
func (m *mockSessionAgent) IsQueuePaused(sessionID string) bool                 { return false }
func (m *mockSessionAgent) PrioritizeQueuedPrompt(sessionID string, index int) bool {
	return false
}
func (m *mockSessionAgent) Summarize(context.Context, string, fantasy.ProviderOptions) error {
	if m.summarizeErr != nil {
		return m.summarizeErr
	}
	m.summarized = append(m.summarized, "summarized")
	return nil
}

// newTestCoordinator creates a minimal coordinator for unit testing runSubAgent.
func newTestCoordinator(t *testing.T, env fakeEnv, providerID string, providerCfg config.ProviderConfig) *coordinator {
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Providers.Set(providerID, providerCfg)
	env.sessions.SetDefaultPermissionMode(session.PermissionModeDefault)
	return &coordinator{
		cfg:      cfg,
		sessions: env.sessions,
		messages: env.messages,
	}
}

// newMockAgent creates a mockSessionAgent with the given provider and run function.
func newMockAgent(providerID string, maxTokens int64, runFunc func(context.Context, SessionAgentCall) (*fantasy.AgentResult, error)) *mockSessionAgent {
	return &mockSessionAgent{
		model: Model{
			CatwalkCfg: catwalk.Model{
				DefaultMaxTokens: maxTokens,
			},
			ModelCfg: config.SelectedModel{
				Provider: providerID,
			},
		},
		runFunc: runFunc,
	}
}

// agentResultWithText creates a minimal AgentResult with the given text response.
func agentResultWithText(text string) *fantasy.AgentResult {
	return &fantasy.AgentResult{
		Response: fantasy.Response{
			Content: fantasy.ResponseContent{
				fantasy.TextContent{Text: text},
			},
		},
	}
}

func TestRunSubAgent(t *testing.T) {
	const providerID = "test-provider"
	providerCfg := config.ProviderConfig{ID: providerID}

	t.Run("happy path", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		agent := newMockAgent(providerID, 4096, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			assert.Equal(t, "do something", call.Prompt)
			assert.Equal(t, int64(4096), call.MaxOutputTokens)
			return agentResultWithText("done"), nil
		})

		resp, err := coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "do something",
			SessionTitle:   "Test Session",
		})
		require.NoError(t, err)
		assert.Equal(t, "done", resp.Content)
		assert.False(t, resp.IsError)
	})

	t.Run("auto mode blocks delegation when handoff review cannot run", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)
		coord.cfg.Config().Models[config.SelectedModelTypeAutoClassifier] = config.SelectedModel{
			Provider: "missing-provider",
			Model:    "missing-model",
		}

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)
		_, err = env.sessions.UpdatePermissionMode(t.Context(), parentSession.ID, session.PermissionModeAuto)
		require.NoError(t, err)

		agent := newMockAgent(providerID, 4096, func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
			t.Fatalf("subagent should not run when auto delegation review blocks")
			return nil, nil
		})

		resp, err := coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "do something",
			SessionTitle:   "Test Session",
		})
		require.NoError(t, err)
		assert.True(t, resp.IsError)
		assert.Equal(t, "Auto Mode blocked subagent delegation because the handoff review failed.", resp.Content)
	})

	t.Run("ModelCfg.MaxTokens overrides default", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		agent := &mockSessionAgent{
			model: Model{
				CatwalkCfg: catwalk.Model{
					DefaultMaxTokens: 4096,
				},
				ModelCfg: config.SelectedModel{
					Provider:  providerID,
					MaxTokens: 8192,
				},
			},
			runFunc: func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
				assert.Equal(t, int64(8192), call.MaxOutputTokens)
				return agentResultWithText("ok"), nil
			},
		}

		resp, err := coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Test",
		})
		require.NoError(t, err)
		assert.Equal(t, "ok", resp.Content)
	})

	t.Run("session creation failure with canceled context", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		agent := newMockAgent(providerID, 4096, nil)

		// Use a canceled context to trigger CreateTaskSession failure.
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err = coord.runSubAgent(ctx, subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Test",
		})
		require.Error(t, err)
	})

	t.Run("provider not configured", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		// Agent references a provider that doesn't exist in config.
		agent := newMockAgent("unknown-provider", 4096, nil)

		_, err = coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Test",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "model provider not configured")
	})

	t.Run("agent run error returns error response", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		agent := newMockAgent(providerID, 4096, func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
			return nil, errors.New("agent exploded")
		})

		resp, err := coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Test",
		})
		// runSubAgent returns (errorResponse, nil) when agent.Run fails 鈥?not a Go error.
		require.NoError(t, err)
		assert.True(t, resp.IsError)
		assert.Equal(t, "agent exploded", resp.Content)
	})

	t.Run("agent run error prefers persisted child assistant error details", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		agent := newMockAgent(providerID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			assistant, createErr := env.messages.Create(ctx, call.SessionID, message.CreateMessageParams{
				Role: message.Assistant,
			})
			require.NoError(t, createErr)

			assistant.AddFinish(message.FinishReasonError, "Network error", "stream idle timeout: no data received for 45s")
			require.NoError(t, env.messages.Update(ctx, assistant))

			return nil, errors.New("agent exploded")
		})

		resp, err := coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Test",
		})
		require.NoError(t, err)
		assert.True(t, resp.IsError)
		assert.Equal(t, "stream idle timeout: no data received for 45s", resp.Content)
	})

	t.Run("falls back to persisted child session assistant content", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		agent := newMockAgent(providerID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			_, err := env.messages.Create(ctx, call.SessionID, message.CreateMessageParams{
				Role: message.Assistant,
				Parts: []message.ContentPart{
					message.TextContent{Text: "persisted final answer"},
					message.Finish{Reason: message.FinishReasonEndTurn},
				},
			})
			require.NoError(t, err)
			return &fantasy.AgentResult{}, nil
		})

		resp, err := coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Test",
		})
		require.NoError(t, err)
		assert.False(t, resp.IsError)
		assert.Equal(t, "persisted final answer", resp.Content)
	})

	t.Run("returns guidance text when neither result nor child session has content", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		agent := newMockAgent(providerID, 4096, func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
			return &fantasy.AgentResult{}, nil
		})

		resp, err := coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Test",
		})
		require.NoError(t, err)
		assert.False(t, resp.IsError)
		assert.Contains(t, resp.Content, "Subagent completed with no textual response")
	})

	t.Run("does not fall back to earlier assistant text when latest assistant is empty", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		agent := newMockAgent(providerID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			_, err := env.messages.Create(ctx, call.SessionID, message.CreateMessageParams{
				Role: message.Assistant,
				Parts: []message.ContentPart{
					message.TextContent{Text: "earlier text"},
					message.Finish{Reason: message.FinishReasonToolUse},
				},
			})
			require.NoError(t, err)

			_, err = env.messages.Create(ctx, call.SessionID, message.CreateMessageParams{
				Role: message.Assistant,
				Parts: []message.ContentPart{
					message.Finish{Reason: message.FinishReasonEndTurn},
				},
			})
			require.NoError(t, err)

			return &fantasy.AgentResult{}, nil
		})

		resp, err := coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Test",
		})
		require.NoError(t, err)
		assert.False(t, resp.IsError)
		assert.Contains(t, resp.Content, "Subagent completed with no textual response")
	})

	t.Run("session setup callback is invoked", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		var setupCalledWith string
		agent := newMockAgent(providerID, 4096, func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
			return agentResultWithText("ok"), nil
		})

		_, err = coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Test",
			SessionSetup: func(sessionID string) {
				setupCalledWith = sessionID
			},
		})
		require.NoError(t, err)
		assert.NotEmpty(t, setupCalledWith, "SessionSetup should have been called")
	})

	t.Run("clears inherited parent runtime config before subagent run", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		parentRuntimeConfig := sessionAgentRuntimeConfig{
			MaxOutputTokens: 1234,
		}

		agent := newMockAgent(providerID, 4096, func(ctx context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
			got, ok := ctx.Value(sessionAgentRuntimeConfigContextKey{}).(*sessionAgentRuntimeConfig)
			require.True(t, ok, "subagent context should carry an explicit runtime config override marker")
			require.Nil(t, got, "subagent must not inherit the parent agent runtime config")
			return agentResultWithText("ok"), nil
		})

		_, err = coord.runSubAgent(
			context.WithValue(t.Context(), sessionAgentRuntimeConfigContextKey{}, parentRuntimeConfig),
			subAgentParams{
				Agent:          agent,
				SessionID:      parentSession.ID,
				AgentMessageID: "msg-1",
				ToolCallID:     "call-1",
				Prompt:         "test",
				SessionTitle:   "Test",
			},
		)
		require.NoError(t, err)
	})

	t.Run("cost propagation to parent session", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		agent := newMockAgent(providerID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			// Simulate the agent incurring cost by updating the child session.
			childSession, err := env.sessions.Get(ctx, call.SessionID)
			if err != nil {
				return nil, err
			}
			childSession.Cost = 0.05
			_, err = env.sessions.Save(ctx, childSession)
			if err != nil {
				return nil, err
			}
			return agentResultWithText("ok"), nil
		})

		_, err = coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Test",
		})
		require.NoError(t, err)

		updated, err := env.sessions.Get(t.Context(), parentSession.ID)
		require.NoError(t, err)
		assert.InDelta(t, 0.05, updated.Cost, 1e-9)
	})
}

func TestPrepareModelSwitch(t *testing.T) {
	const providerID = "test-provider"
	providerCfg := config.ProviderConfig{
		ID: providerID,
		Models: []catwalk.Model{
			{ID: "big", ContextWindow: 1_000_000, DefaultMaxTokens: 32_000},
			{ID: "small", ContextWindow: 200_000, DefaultMaxTokens: 8_000},
		},
	}

	t.Run("summarizes before switching to smaller active model", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)
		coord.cfg.Config().Agents[config.AgentCoder] = config.Agent{Name: config.AgentCoder, Model: config.SelectedModelTypeLarge}

		sess, err := env.sessions.Create(t.Context(), "switch")
		require.NoError(t, err)

		estimates := []int64{250_000, 50_000}
		agent := newMockAgent(providerID, 32_000, func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
			return agentResultWithText("ok"), nil
		})
		agent.model.CatwalkCfg.ContextWindow = 1_000_000
		agent.estimateFunc = func(_ context.Context, sessionID string, model Model) (int64, error) {
			require.Equal(t, sess.ID, sessionID)
			require.Equal(t, "small", model.ModelCfg.Model)
			v := estimates[0]
			estimates = estimates[1:]
			return v, nil
		}
		coord.currentAgent = agent

		err = coord.PrepareModelSwitch(t.Context(), sess.ID, config.SelectedModelTypeLarge, config.SelectedModel{Provider: providerID, Model: "small"})
		require.NoError(t, err)
		require.Len(t, agent.summarized, 1)
	})

	t.Run("fails when summarization cannot shrink session enough", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)
		coord.cfg.Config().Agents[config.AgentCoder] = config.Agent{Name: config.AgentCoder, Model: config.SelectedModelTypeLarge}

		sess, err := env.sessions.Create(t.Context(), "switch")
		require.NoError(t, err)

		agent := newMockAgent(providerID, 32_000, func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
			return agentResultWithText("ok"), nil
		})
		agent.model.CatwalkCfg.ContextWindow = 1_000_000
		agent.estimateFunc = func(_ context.Context, _ string, _ Model) (int64, error) {
			return 250_000, nil
		}
		coord.currentAgent = agent

		err = coord.PrepareModelSwitch(t.Context(), sess.ID, config.SelectedModelTypeLarge, config.SelectedModel{Provider: providerID, Model: "small"})
		require.ErrorContains(t, err, "still too large")
		require.Len(t, agent.summarized, 1)
	})

	t.Run("ignores inactive model slot switches", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)
		coord.cfg.Config().Agents[config.AgentCoder] = config.Agent{Name: config.AgentCoder, Model: config.SelectedModelTypeLarge}

		sess, err := env.sessions.Create(t.Context(), "switch")
		require.NoError(t, err)

		agent := newMockAgent(providerID, 32_000, func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
			return agentResultWithText("ok"), nil
		})
		agent.estimateFunc = func(_ context.Context, _ string, _ Model) (int64, error) {
			t.Fatal("estimate should not be called for inactive model slot")
			return 0, nil
		}
		coord.currentAgent = agent

		err = coord.PrepareModelSwitch(t.Context(), sess.ID, config.SelectedModelTypeSmall, config.SelectedModel{Provider: providerID, Model: "small"})
		require.NoError(t, err)
		require.Empty(t, agent.summarized)
	})
}

func TestUpdateParentSessionCost(t *testing.T) {
	t.Run("accumulates cost correctly", func(t *testing.T) {
		env := testEnv(t)
		cfg, err := config.Init(env.workingDir, "", false)
		require.NoError(t, err)
		coord := &coordinator{cfg: cfg, sessions: env.sessions}

		parent, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		child, err := env.sessions.CreateTaskSession(t.Context(), "tool-1", parent.ID, "Child")
		require.NoError(t, err)

		// Set child cost.
		child.Cost = 0.10
		_, err = env.sessions.Save(t.Context(), child)
		require.NoError(t, err)

		err = coord.updateParentSessionCost(t.Context(), child.ID, parent.ID)
		require.NoError(t, err)

		updated, err := env.sessions.Get(t.Context(), parent.ID)
		require.NoError(t, err)
		assert.InDelta(t, 0.10, updated.Cost, 1e-9)
	})

	t.Run("accumulates multiple child costs", func(t *testing.T) {
		env := testEnv(t)
		cfg, err := config.Init(env.workingDir, "", false)
		require.NoError(t, err)
		coord := &coordinator{cfg: cfg, sessions: env.sessions}

		parent, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		child1, err := env.sessions.CreateTaskSession(t.Context(), "tool-1", parent.ID, "Child1")
		require.NoError(t, err)
		child1.Cost = 0.05
		_, err = env.sessions.Save(t.Context(), child1)
		require.NoError(t, err)

		child2, err := env.sessions.CreateTaskSession(t.Context(), "tool-2", parent.ID, "Child2")
		require.NoError(t, err)
		child2.Cost = 0.03
		_, err = env.sessions.Save(t.Context(), child2)
		require.NoError(t, err)

		err = coord.updateParentSessionCost(t.Context(), child1.ID, parent.ID)
		require.NoError(t, err)
		err = coord.updateParentSessionCost(t.Context(), child2.ID, parent.ID)
		require.NoError(t, err)

		updated, err := env.sessions.Get(t.Context(), parent.ID)
		require.NoError(t, err)
		assert.InDelta(t, 0.08, updated.Cost, 1e-9)
	})

	t.Run("child session not found", func(t *testing.T) {
		env := testEnv(t)
		cfg, err := config.Init(env.workingDir, "", false)
		require.NoError(t, err)
		coord := &coordinator{cfg: cfg, sessions: env.sessions}

		parent, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		err = coord.updateParentSessionCost(t.Context(), "non-existent", parent.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get child session")
	})

	t.Run("parent session not found", func(t *testing.T) {
		env := testEnv(t)
		cfg, err := config.Init(env.workingDir, "", false)
		require.NoError(t, err)
		coord := &coordinator{cfg: cfg, sessions: env.sessions}

		parent, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)
		child, err := env.sessions.CreateTaskSession(t.Context(), "tool-1", parent.ID, "Child")
		require.NoError(t, err)

		err = coord.updateParentSessionCost(t.Context(), child.ID, "non-existent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get parent session")
	})

	t.Run("zero cost handled correctly", func(t *testing.T) {
		env := testEnv(t)
		cfg, err := config.Init(env.workingDir, "", false)
		require.NoError(t, err)
		coord := &coordinator{cfg: cfg, sessions: env.sessions}

		parent, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)
		child, err := env.sessions.CreateTaskSession(t.Context(), "tool-1", parent.ID, "Child")
		require.NoError(t, err)

		err = coord.updateParentSessionCost(t.Context(), child.ID, parent.ID)
		require.NoError(t, err)

		updated, err := env.sessions.Get(t.Context(), parent.ID)
		require.NoError(t, err)
		assert.InDelta(t, 0.0, updated.Cost, 1e-9)
	})
}

func TestMergeCallOptions_AnthropicThinkingCompatibility(t *testing.T) {
	t.Run("claude 4.6 uses effort without budget thinking", func(t *testing.T) {
		model := Model{
			CatwalkCfg: catwalk.Model{
				ID:                     "claude-sonnet-4.6",
				CanReason:              true,
				DefaultReasoningEffort: "high",
			},
		}
		cfg := config.ProviderConfig{
			Type: anthropic.Name,
		}

		options, _, _, _, _, _ := mergeCallOptions(model, cfg)
		anthropicOpts, ok := options[anthropic.Name].(*anthropic.ProviderOptions)
		require.True(t, ok)
		require.NotNil(t, anthropicOpts)
		require.Equal(t, anthropic.Effort("high"), *anthropicOpts.Effort)
		// Claude 4.6 with effort uses adaptive thinking (SDK handles this)
		require.Nil(t, anthropicOpts.Thinking)
	})

	t.Run("claude opus 4.6 with think flag uses high effort", func(t *testing.T) {
		think := true
		model := Model{
			CatwalkCfg: catwalk.Model{
				ID:        "claude-opus-4-6",
				CanReason: true,
			},
			ModelCfg: config.SelectedModel{
				Think: &think,
			},
		}
		cfg := config.ProviderConfig{
			Type: anthropic.Name,
		}

		options, _, _, _, _, _ := mergeCallOptions(model, cfg)
		anthropicOpts, ok := options[anthropic.Name].(*anthropic.ProviderOptions)
		require.True(t, ok)
		require.NotNil(t, anthropicOpts)
		require.Equal(t, anthropic.Effort("high"), *anthropicOpts.Effort)
		require.Nil(t, anthropicOpts.Thinking)
	})

	t.Run("older claude uses budget thinking without effort", func(t *testing.T) {
		model := Model{
			CatwalkCfg: catwalk.Model{
				ID:                     "claude-sonnet-4",
				CanReason:              true,
				DefaultReasoningEffort: "high",
			},
		}
		cfg := config.ProviderConfig{
			Type: anthropic.Name,
		}

		options, _, _, _, _, _ := mergeCallOptions(model, cfg)
		anthropicOpts, ok := options[anthropic.Name].(*anthropic.ProviderOptions)
		require.True(t, ok)
		require.NotNil(t, anthropicOpts)
		require.Nil(t, anthropicOpts.Effort)
		require.NotNil(t, anthropicOpts.Thinking)
		require.Equal(t, int64(28672), anthropicOpts.Thinking.BudgetTokens)
	})

	t.Run("canReason model enables thinking by default", func(t *testing.T) {
		model := Model{
			CatwalkCfg: catwalk.Model{
				ID:        "claude-sonnet-4",
				CanReason: true,
			},
		}
		cfg := config.ProviderConfig{
			Type: anthropic.Name,
		}

		options, _, _, _, _, _ := mergeCallOptions(model, cfg)
		anthropicOpts, ok := options[anthropic.Name].(*anthropic.ProviderOptions)
		require.True(t, ok)
		require.NotNil(t, anthropicOpts)
		require.Nil(t, anthropicOpts.Effort)
		require.NotNil(t, anthropicOpts.Thinking)
		require.Equal(t, int64(28672), anthropicOpts.Thinking.BudgetTokens)
	})

	t.Run("thinking is disabled when Think is explicitly false", func(t *testing.T) {
		think := false
		model := Model{
			CatwalkCfg: catwalk.Model{
				ID:        "claude-sonnet-4",
				CanReason: true,
			},
			ModelCfg: config.SelectedModel{
				Think: &think,
			},
		}
		cfg := config.ProviderConfig{
			Type: anthropic.Name,
		}

		options, _, _, _, _, _ := mergeCallOptions(model, cfg)
		anthropicOpts, ok := options[anthropic.Name].(*anthropic.ProviderOptions)
		require.True(t, ok)
		require.NotNil(t, anthropicOpts)
		require.Nil(t, anthropicOpts.Effort)
		require.Nil(t, anthropicOpts.Thinking)
	})

	t.Run("thinking is disabled for claude 4.6 when Think is explicitly false", func(t *testing.T) {
		think := false
		model := Model{
			CatwalkCfg: catwalk.Model{
				ID:        "claude-sonnet-4.6",
				CanReason: true,
			},
			ModelCfg: config.SelectedModel{
				Think: &think,
			},
		}
		cfg := config.ProviderConfig{
			Type: anthropic.Name,
		}

		options, _, _, _, _, _ := mergeCallOptions(model, cfg)
		anthropicOpts, ok := options[anthropic.Name].(*anthropic.ProviderOptions)
		require.True(t, ok)
		require.NotNil(t, anthropicOpts)
		require.Nil(t, anthropicOpts.Effort)
		require.Nil(t, anthropicOpts.Thinking)
	})

	t.Run("isAnthropicThinking returns true for any CanReason model", func(t *testing.T) {
		for _, id := range []string{"claude-sonnet-4", "claude-sonnet-4.6", "kimi-k2.5"} {
			require.True(t, isAnthropicThinking(catwalk.Model{ID: id, CanReason: true}), id)
		}
	})

	t.Run("isAnthropicThinking returns false when CanReason is false", func(t *testing.T) {
		require.False(t, isAnthropicThinking(catwalk.Model{ID: "claude-sonnet-4", CanReason: false}))
	})

	t.Run("claude 4.6 canReason enables effort by default", func(t *testing.T) {
		model := Model{
			CatwalkCfg: catwalk.Model{
				ID:        "claude-sonnet-4.6",
				CanReason: true,
			},
		}
		cfg := config.ProviderConfig{
			Type: anthropic.Name,
		}

		options, _, _, _, _, _ := mergeCallOptions(model, cfg)
		anthropicOpts, ok := options[anthropic.Name].(*anthropic.ProviderOptions)
		require.True(t, ok)
		require.NotNil(t, anthropicOpts)
		require.Equal(t, anthropic.Effort("high"), *anthropicOpts.Effort)
		require.Nil(t, anthropicOpts.Thinking)
	})
}

func TestMergeCallOptions_ThinkDisabledAllProviders(t *testing.T) {
	t.Parallel()

	think := false

	t.Run("openai: no reasoning_effort when Think is false", func(t *testing.T) {
		t.Parallel()
		// Use a non-responses model ID so mergeCallOptions returns *ProviderOptions.
		model := Model{
			CatwalkCfg: catwalk.Model{
				ID:        "custom-reasoning-model",
				CanReason: true,
			},
			ModelCfg: config.SelectedModel{Think: &think},
		}
		options, _, _, _, _, _ := mergeCallOptions(model, config.ProviderConfig{Type: openai.Name})
		opts, ok := options[openai.Name].(*openai.ProviderOptions)
		require.True(t, ok)
		require.Nil(t, opts.ReasoningEffort)
	})

	t.Run("openai: reasoning_effort set by default when Think is nil", func(t *testing.T) {
		t.Parallel()
		// Use a non-responses model ID so mergeCallOptions returns *ProviderOptions.
		model := Model{
			CatwalkCfg: catwalk.Model{
				ID:        "custom-reasoning-model",
				CanReason: true,
			},
		}
		options, _, _, _, _, _ := mergeCallOptions(model, config.ProviderConfig{Type: openai.Name})
		opts, ok := options[openai.Name].(*openai.ProviderOptions)
		require.True(t, ok)
		require.NotNil(t, opts.ReasoningEffort)
		require.Equal(t, openai.ReasoningEffortHigh, *opts.ReasoningEffort)
	})

	t.Run("openai-compat: no reasoning_effort when Think is false", func(t *testing.T) {
		t.Parallel()
		model := Model{
			CatwalkCfg: catwalk.Model{
				ID:        "some-reasoning-model",
				CanReason: true,
			},
			ModelCfg: config.SelectedModel{Think: &think},
		}
		options, _, _, _, _, _ := mergeCallOptions(model, config.ProviderConfig{Type: openaicompat.Name})
		opts, ok := options[openaicompat.Name].(*openaicompat.ProviderOptions)
		require.True(t, ok)
		require.Nil(t, opts.ReasoningEffort)
	})
}

func TestMergeCallOptions_ThinkDisabledClearsProviderOptions(t *testing.T) {
	t.Parallel()

	think := false

	t.Run("anthropic: Think=false clears effort set in provider config", func(t *testing.T) {
		t.Parallel()
		model := Model{
			CatwalkCfg: catwalk.Model{
				ID:        "claude-sonnet-4",
				CanReason: true,
			},
			ModelCfg: config.SelectedModel{Think: &think},
		}
		// Provider config has effort pre-set.
		cfg := config.ProviderConfig{
			Type:            anthropic.Name,
			ProviderOptions: map[string]any{"effort": "high"},
		}

		options, _, _, _, _, _ := mergeCallOptions(model, cfg)
		anthropicOpts, ok := options[anthropic.Name].(*anthropic.ProviderOptions)
		require.True(t, ok)
		require.Nil(t, anthropicOpts.Effort)
		require.Nil(t, anthropicOpts.Thinking)
	})

	t.Run("openai: Think=false clears reasoning_effort set in provider config", func(t *testing.T) {
		t.Parallel()
		model := Model{
			CatwalkCfg: catwalk.Model{
				ID:        "custom-reasoning-model",
				CanReason: true,
			},
			ModelCfg: config.SelectedModel{Think: &think},
		}
		cfg := config.ProviderConfig{
			Type:            openai.Name,
			ProviderOptions: map[string]any{"reasoning_effort": "high"},
		}

		options, _, _, _, _, _ := mergeCallOptions(model, cfg)
		opts, ok := options[openai.Name].(*openai.ProviderOptions)
		require.True(t, ok)
		require.Nil(t, opts.ReasoningEffort)
	})

	t.Run("openai-compat: Think=false clears reasoning_effort set in provider config", func(t *testing.T) {
		t.Parallel()
		model := Model{
			CatwalkCfg: catwalk.Model{
				ID:        "custom-reasoning-model",
				CanReason: true,
			},
			ModelCfg: config.SelectedModel{Think: &think},
		}
		cfg := config.ProviderConfig{
			Type:            openaicompat.Name,
			ProviderOptions: map[string]any{"reasoning_effort": "high"},
		}

		options, _, _, _, _, _ := mergeCallOptions(model, cfg)
		opts, ok := options[openaicompat.Name].(*openaicompat.ProviderOptions)
		require.True(t, ok)
		require.Nil(t, opts.ReasoningEffort)
	})
}

func TestWrapOpenAIStreamingHTTPClient(t *testing.T) {
	t.Parallel()

	t.Run("uses websocket wrapper when enabled", func(t *testing.T) {
		t.Parallel()

		wrapped := wrapOpenAIStreamingHTTPClient(nil, true)
		require.NotNil(t, wrapped)

		transportType := reflect.TypeOf(wrapped.Transport).String()
		require.Equal(t, "*httpext.activityTrackingTransport", transportType)

		activityValue := reflect.ValueOf(wrapped.Transport).Elem()
		baseField := activityValue.FieldByName("base")
		require.True(t, baseField.IsValid())
		base := reflect.NewAt(baseField.Type(), unsafe.Pointer(baseField.UnsafeAddr())).Elem().Interface().(http.RoundTripper)
		require.Equal(t, "httpext.openAIResponsesWebSocketTransport", reflect.TypeOf(base).String())
	})

	t.Run("uses activity wrapper only when disabled", func(t *testing.T) {
		t.Parallel()

		wrapped := wrapOpenAIStreamingHTTPClient(nil, false)
		require.NotNil(t, wrapped)

		transportType := reflect.TypeOf(wrapped.Transport).String()
		require.Equal(t, "*httpext.activityTrackingTransport", transportType)

		activityValue := reflect.ValueOf(wrapped.Transport).Elem()
		baseField := activityValue.FieldByName("base")
		require.True(t, baseField.IsValid())
		base := reflect.NewAt(baseField.Type(), unsafe.Pointer(baseField.UnsafeAddr())).Elem().Interface().(http.RoundTripper)
		require.Same(t, http.DefaultTransport, base)
	})
}

func TestBuildProvider_PreservesHyperBaseURL(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	coord := newTestCoordinator(t, env, hyper.Name, config.ProviderConfig{
		ID:      hyper.Name,
		Type:    hyper.Name,
		BaseURL: "https://hyper.example.test/api/v1/fantasy",
		APIKey:  "test-key",
	})

	provider, err := coord.buildProvider(config.ProviderConfig{
		ID:      hyper.Name,
		Type:    hyper.Name,
		BaseURL: "https://hyper.example.test/api/v1/fantasy",
		APIKey:  "test-key",
	}, catwalk.Model{}, false, false)
	require.NoError(t, err)
	require.Equal(t, "https://hyper.example.test/api/v1/fantasy", hyperProviderBaseURL(t, provider))
}

func hyperProviderBaseURL(t *testing.T, provider fantasy.Provider) string {
	t.Helper()

	value := reflect.ValueOf(provider)
	require.Equal(t, reflect.Pointer, value.Kind())

	optionsField := value.Elem().FieldByName("options")
	require.True(t, optionsField.IsValid())

	optionsValue := reflect.NewAt(optionsField.Type(), unsafe.Pointer(optionsField.UnsafeAddr())).Elem()
	baseURLField := optionsValue.FieldByName("baseURL")
	require.True(t, baseURLField.IsValid())

	return reflect.NewAt(baseURLField.Type(), unsafe.Pointer(baseURLField.UnsafeAddr())).Elem().String()
}
