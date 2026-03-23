package agent

import (
	"context"
	"errors"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"github.com/charmbracelet/crush/internal/config"
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
	return &coordinator{
		cfg:      cfg,
		sessions: env.sessions,
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
		assert.Equal(t, "error generating response", resp.Content)
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
		model := Model{
			CatwalkCfg: catwalk.Model{
				ID:        "claude-opus-4-6",
				CanReason: true,
			},
			ModelCfg: config.SelectedModel{
				Think: true,
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
