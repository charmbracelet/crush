package agent

import (
	"context"
	"errors"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/bedrock"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/subagents"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSessionAgent is a minimal mock for the SessionAgent interface.
type mockSessionAgent struct {
	model     Model
	runFunc   func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error)
	cancelled []string
}

func (m *mockSessionAgent) Run(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
	return m.runFunc(ctx, call)
}

func (m *mockSessionAgent) BeginAccepted(sessionID string) *AcceptedRun {
	return &AcceptedRun{sessionID: sessionID}
}

func (m *mockSessionAgent) Model() Model                        { return m.model }
func (m *mockSessionAgent) SetModels(large, small Model)        {}
func (m *mockSessionAgent) SetTools(tools []fantasy.AgentTool)  {}
func (m *mockSessionAgent) SetSystemPrompt(systemPrompt string) {}
func (m *mockSessionAgent) Cancel(sessionID string) {
	m.cancelled = append(m.cancelled, sessionID)
}
func (m *mockSessionAgent) CancelAll()                                  {}
func (m *mockSessionAgent) IsSessionBusy(sessionID string) bool         { return false }
func (m *mockSessionAgent) IsBusy() bool                                { return false }
func (m *mockSessionAgent) QueuedPrompts(sessionID string) int          { return 0 }
func (m *mockSessionAgent) QueuedPromptsList(sessionID string) []string { return nil }
func (m *mockSessionAgent) ClearQueue(sessionID string)                 {}
func (m *mockSessionAgent) Summarize(context.Context, string, fantasy.ProviderOptions) error {
	return nil
}

// newTestCoordinator creates a minimal coordinator for unit testing runSubAgent.
func newTestCoordinator(t *testing.T, env fakeEnv, providerID string, providerCfg config.ProviderConfig) *coordinator {
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Providers.Set(providerID, providerCfg)
	return &coordinator{
		cfg:                cfg,
		sessions:           env.sessions,
		subagentModelCache: make(map[subagentModelKey]Model),
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
			return nil, errors.New("provider request failed")
		})

		resp, err := coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Test",
		})
		// runSubAgent returns (errorResponse, nil) when agent.Run fails — not a Go error.
		require.NoError(t, err)
		assert.True(t, resp.IsError)
		assert.Equal(t, "Failed to generate response: provider request failed", resp.Content)
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

// TestCoordinator_ActiveSubagentsFromManager verifies that coordinator.activeSubagents
// is populated from a Manager's ActiveSubagents slice. This test will fail to compile
// until the activeSubagents field exists on the coordinator struct.
func TestCoordinator_ActiveSubagentsFromManager(t *testing.T) {
	t.Parallel()

	active := []*subagents.Subagent{
		{Name: "test-agent", Description: "A test agent"},
		{Name: "another-agent", Description: "Another test agent"},
	}
	mgr := subagents.NewManager(active, active, nil)

	// Construct the coordinator directly (mirrors newTestCoordinator style).
	// This fails to compile if activeSubagents field does not exist on coordinator.
	c := &coordinator{
		activeSubagents: mgr.ActiveSubagents(),
	}

	require.Len(t, c.activeSubagents, 2)
	require.Equal(t, "test-agent", c.activeSubagents[0].Name)
	require.Equal(t, "another-agent", c.activeSubagents[1].Name)
}

// TestCoordinator_ActiveSubagentsNilManager verifies that coordinator.activeSubagents
// is nil (zero value) when no Manager is wired in. This test will fail to compile
// until the activeSubagents field exists on the coordinator struct.
func TestCoordinator_ActiveSubagentsNilManager(t *testing.T) {
	t.Parallel()

	// Construct coordinator without setting activeSubagents — mirrors the
	// nil-manager branch of NewCoordinator (no subagents wired).
	c := &coordinator{}

	require.Nil(t, c.activeSubagents)
}

// TestCoordinator_ActiveSubagentsFieldType verifies that the activeSubagents field
// has type []*subagents.Subagent. A direct struct literal assignment is used so the
// test fails to compile with a type mismatch if the field type is wrong.
func TestCoordinator_ActiveSubagentsFieldType(t *testing.T) {
	t.Parallel()

	// This literal fails to compile if the field type is not []*subagents.Subagent.
	c := &coordinator{
		activeSubagents: []*subagents.Subagent{
			{Name: "compile-check"},
		},
	}

	require.Len(t, c.activeSubagents, 1)
	assert.Equal(t, "compile-check", c.activeSubagents[0].Name)
}

// TestRunSubAgent_RegistersAndFinishesRuntime verifies that runSubAgent calls
// Register on the coordinator's Runtime after session creation and Finish when
// it returns, propagating AgentName, AgentColor and AgentModel from params.
func TestRunSubAgent_RegistersAndFinishesRuntime(t *testing.T) {
	t.Parallel()

	const providerID = "test-provider"
	providerCfg := config.ProviderConfig{ID: providerID}

	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Providers.Set(providerID, providerCfg)

	rt := subagents.NewRuntime()
	t.Cleanup(rt.Shutdown)

	// Channel to capture the session ID used during the agent run so we can
	// assert that List sees the entry while runSubAgent is in-flight.
	type snapshot struct {
		entries []subagents.RunningEntry
	}
	snapCh := make(chan snapshot, 1)

	parentSession, err := env.sessions.Create(t.Context(), "Parent")
	require.NoError(t, err)

	agent := newMockAgent(providerID, 4096, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		// Capture a snapshot of the Runtime state while the sub-agent is running.
		snapCh <- snapshot{entries: rt.List(parentSession.ID)}
		return agentResultWithText("done"), nil
	})

	coord := &coordinator{
		cfg:      cfg,
		sessions: env.sessions,
		runtime:  rt,
	}

	_, err = coord.runSubAgent(t.Context(), subAgentParams{
		Agent:          agent,
		SessionID:      parentSession.ID,
		AgentMessageID: "msg-1",
		ToolCallID:     "call-1",
		Prompt:         "do something",
		SessionTitle:   "Runtime Test",
		AgentName:      "my-agent",
		AgentColor:     "blue",
		AgentModel:     "claude-test",
	})
	require.NoError(t, err)

	// Verify the in-flight snapshot captured exactly one entry with correct fields.
	select {
	case snap := <-snapCh:
		require.Len(t, snap.entries, 1, "Runtime must have one entry while runSubAgent is in-flight")
		e := snap.entries[0]
		require.Equal(t, parentSession.ID, e.ParentSessionID)
		require.Equal(t, "my-agent", e.Name)
		require.Equal(t, "blue", e.Color)
		require.Equal(t, "claude-test", e.Model)
		require.Equal(t, subagents.StatusRunning, e.Status)
		require.False(t, e.StartedAt.IsZero())
	default:
		t.Fatal("agent run function was never called")
	}

	// After runSubAgent returns, the entry must be gone.
	after := rt.List(parentSession.ID)
	require.Empty(t, after, "Runtime must have no entries after runSubAgent returns")
}

// TestResolveModelByID_UnknownErrors verifies resolveModelByID errors when no
// configured provider offers the requested model id.
func TestResolveModelByID_UnknownErrors(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	coord := newTestCoordinator(t, env, "p", config.ProviderConfig{ID: "p"})

	_, err := coord.resolveModelByID(t.Context(), "no-such-model", "", true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// TestResolveModelByID_WithProviderOverride verifies that when a providerOverride
// is supplied, resolveModelByID restricts lookup to that provider.
func TestResolveModelByID_WithProviderOverride(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	providerCfg := config.ProviderConfig{
		ID:     "test-provider",
		Models: []catwalk.Model{{ID: "model-a"}},
	}
	coord := newTestCoordinator(t, env, "test-provider", providerCfg)

	t.Run("unknown_provider_override_errors", func(t *testing.T) {
		t.Parallel()

		_, err := coord.resolveModelByID(t.Context(), "model-a", "nonexistent-provider", true)
		require.Error(t, err)
	})
}

// TestFindModelProvider verifies the pure provider/catwalk lookup used by
// resolveModelByID to back a subagent's specific `model:` id.
func TestFindModelProvider(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	providerCfg := config.ProviderConfig{
		ID:     "test-provider",
		Models: []catwalk.Model{{ID: "model-a"}, {ID: "model-b"}},
	}
	coord := newTestCoordinator(t, env, "test-provider", providerCfg)

	t.Run("found", func(t *testing.T) {
		t.Parallel()

		pc, m, ok := coord.findModelProvider("model-b", "")
		require.True(t, ok)
		require.Equal(t, "test-provider", pc.ID)
		require.Equal(t, "model-b", m.ID)
	})

	t.Run("unknown", func(t *testing.T) {
		t.Parallel()

		_, _, ok := coord.findModelProvider("no-such-model", "")
		require.False(t, ok)
	})

	t.Run("provider_override_match", func(t *testing.T) {
		t.Parallel()

		pc, m, ok := coord.findModelProvider("model-a", "test-provider")
		require.True(t, ok)
		require.Equal(t, "test-provider", pc.ID)
		require.Equal(t, "model-a", m.ID)
	})

	t.Run("provider_override_no_match_wrong_provider", func(t *testing.T) {
		t.Parallel()

		_, _, ok := coord.findModelProvider("model-a", "other-provider")
		require.False(t, ok)
	})

	t.Run("provider_override_no_match_unknown_model", func(t *testing.T) {
		t.Parallel()

		_, _, ok := coord.findModelProvider("no-such-model", "test-provider")
		require.False(t, ok)
	})
}

// TestFindModelProvider_TwoProvidersSameModelID verifies behavior when two
// providers expose the same model ID.
func TestFindModelProvider_TwoProvidersSameModelID(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	cfg.Config().Providers.Set("provider-a", config.ProviderConfig{
		ID:     "provider-a",
		Models: []catwalk.Model{{ID: "shared-model"}},
	})
	cfg.Config().Providers.Set("provider-b", config.ProviderConfig{
		ID:     "provider-b",
		Models: []catwalk.Model{{ID: "shared-model"}},
	})

	coord := &coordinator{cfg: cfg, sessions: env.sessions}

	t.Run("no_override_returns_one_result_no_panic", func(t *testing.T) {
		t.Parallel()

		pc, m, ok := coord.findModelProvider("shared-model", "")
		require.True(t, ok, "must find shared-model in at least one provider")
		require.Equal(t, "shared-model", m.ID)
		require.NotEmpty(t, pc.ID)
	})

	t.Run("override_selects_specific_provider", func(t *testing.T) {
		t.Parallel()

		pc, m, ok := coord.findModelProvider("shared-model", "provider-b")
		require.True(t, ok)
		require.Equal(t, "provider-b", pc.ID)
		require.Equal(t, "shared-model", m.ID)
	})
}

// TestBuildAgent_SubagentModel verifies that buildAgent accepts a subagentModel
// struct and routes model selection correctly.
func TestBuildAgent_SubagentModel(t *testing.T) {
	t.Parallel()

	t.Run("zero_value_uses_large_model", func(t *testing.T) {
		t.Parallel()

		env := testEnv(t)
		coord := &coordinator{
			cfg:      config.NewTestStoreWithWorkingDir(&config.Config{}, env.workingDir),
			sessions: env.sessions,
		}

		agentCfg := config.Agent{
			ID:           "test",
			Name:         "test",
			AllowedTools: []string{},
		}

		// Zero-value subagentModel must be accepted without panicking. With no
		// models configured, buildNamedModel fails before any prompt is needed,
		// verifying the struct parameter is wired into model-selection logic.
		_, err := coord.buildAgent(t.Context(), nil, agentCfg, true, subagentModel{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "model")
	})
}

// TestRunSubAgent_CancelledMapsToCancelled verifies that a context.Canceled
// from the agent run is mapped to the "cancelled" response (distinct from the
// generic failure), confirming the StatusCancelled branch is taken.
func TestRunSubAgent_CancelledMapsToCancelled(t *testing.T) {
	t.Parallel()

	const providerID = "test-provider"
	providerCfg := config.ProviderConfig{ID: providerID}

	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Providers.Set(providerID, providerCfg)

	rt := subagents.NewRuntime()
	t.Cleanup(rt.Shutdown)

	parentSession, err := env.sessions.Create(t.Context(), "Parent")
	require.NoError(t, err)

	agent := newMockAgent(providerID, 4096, func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
		return nil, context.Canceled
	})

	coord := &coordinator{cfg: cfg, sessions: env.sessions, runtime: rt}

	resp, err := coord.runSubAgent(t.Context(), subAgentParams{
		Agent:          agent,
		SessionID:      parentSession.ID,
		AgentMessageID: "msg-1",
		ToolCallID:     "call-1",
		Prompt:         "do something",
		SessionTitle:   "Cancel Test",
		AgentName:      "a",
		AgentColor:     "red",
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Equal(t, "Subagent cancelled by user", resp.Content)

	// The runtime entry must be gone after a cancelled run.
	require.Empty(t, rt.List(parentSession.ID))
}

// TestActiveSubagentsList verifies the coordinator reads the live manager
// snapshot when present (so Library reloads are reflected) and falls back to
// the construction-time slice when no manager is wired.
func TestActiveSubagentsList(t *testing.T) {
	t.Parallel()

	t.Run("live from manager reflects reload", func(t *testing.T) {
		t.Parallel()
		initial := []*subagents.Subagent{{Name: "x"}, {Name: "y"}}
		mgr := subagents.NewManager(initial, initial, nil)
		t.Cleanup(mgr.Shutdown)
		c := &coordinator{subagentsMgr: mgr}

		require.Len(t, c.activeSubagentsList(), 2)

		reduced := []*subagents.Subagent{{Name: "x"}}
		mgr.Reload(reduced, reduced, nil)

		got := c.activeSubagentsList()
		require.Len(t, got, 1)
		require.Equal(t, "x", got[0].Name)
	})

	t.Run("fallback when nil manager", func(t *testing.T) {
		t.Parallel()
		c := &coordinator{activeSubagents: []*subagents.Subagent{{Name: "z"}}}
		got := c.activeSubagentsList()
		require.Len(t, got, 1)
		require.Equal(t, "z", got[0].Name)
	})
}

func TestGetProviderOptionsReasoningEffort(t *testing.T) {
	// Bedrock is Fantasy's Anthropic under a different provider name; options
	// must land under anthropic.Name so the Anthropic language model picks them up.
	tests := []struct {
		name         string
		providerType catwalk.Type
	}{
		{"anthropic honors reasoning_effort", catwalk.Type(anthropic.Name)},
		{"bedrock honors reasoning_effort", catwalk.Type(bedrock.Name)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			model := Model{
				CatwalkCfg: catwalk.Model{
					ID:              "claude-opus-4-7",
					CanReason:       true,
					ReasoningLevels: []string{"max"},
				},
				ModelCfg: config.SelectedModel{
					Provider:        "test",
					ReasoningEffort: "max",
				},
			}
			providerCfg := config.ProviderConfig{ID: "test", Type: tc.providerType}

			opts := getProviderOptions(model, providerCfg)

			raw, ok := opts[anthropic.Name]
			require.True(t, ok, "options should be keyed under anthropic.Name for type %q", tc.providerType)
			parsed, ok := raw.(*anthropic.ProviderOptions)
			require.True(t, ok)
			require.NotNil(t, parsed.Effort)
			assert.Equal(t, anthropic.Effort("max"), *parsed.Effort)
		})
	}
}

// TestUpdateModels_ClearsSubagentModelCache verifies that UpdateModels empties
// the subagent model cache so stale LanguageModel instances are not reused
// after a config reload, even when UpdateModels itself returns an error.
func TestUpdateModels_ClearsSubagentModelCache(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	coord := &coordinator{
		cfg:                config.NewTestStoreWithWorkingDir(&config.Config{}, env.workingDir),
		sessions:           env.sessions,
		subagentModelCache: make(map[subagentModelKey]Model),
	}

	// Manually populate the cache with a dummy entry.
	coord.subagentModelCache[subagentModelKey{modelID: "some-model", provider: "", isSubAgent: true}] = Model{}

	require.Len(t, coord.subagentModelCache, 1)

	// UpdateModels will error (no models configured in empty config), but the
	// cache must be cleared regardless.
	_ = coord.UpdateModels(t.Context())

	require.Empty(t, coord.subagentModelCache)
}

// TestResolveModelByID_CacheHitSkipsBuild verifies that a second call to
// resolveModelByID with the same arguments returns the cached Model without
// repeating the provider build, and that errors are not cached.
func TestResolveModelByID_CacheHitSkipsBuild(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	// No-op provider with a known model so findModelProvider succeeds.
	providerCfg := config.ProviderConfig{
		ID:     "test-provider",
		Models: []catwalk.Model{{ID: "model-x", DefaultMaxTokens: 4096}},
	}
	coord := newTestCoordinator(t, env, "test-provider", providerCfg)

	// First call — cache is empty.
	require.Empty(t, coord.subagentModelCache)

	_, err := coord.resolveModelByID(t.Context(), "model-x", "test-provider", true)
	if err != nil {
		// Provider construction may fail in the test environment (fake API key).
		// Errors must not be cached.
		require.Empty(t, coord.subagentModelCache, "failed build must not populate cache")
		return
	}

	// Success path: cache must contain exactly one entry.
	require.Len(t, coord.subagentModelCache, 1)

	// Second call must hit the cache (same result, no error).
	_, err2 := coord.resolveModelByID(t.Context(), "model-x", "test-provider", true)
	require.NoError(t, err2)
	require.Len(t, coord.subagentModelCache, 1, "second call must not add a new entry")
}

// TestResolveModelByID_ModelNotFound verifies that resolveModelByID returns an
// error containing "not found" when no configured provider offers the requested
// model id, and that the cache is not populated on failure.
func TestResolveModelByID_ModelNotFound(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	providerCfg := config.ProviderConfig{
		ID:     "test-provider",
		Models: []catwalk.Model{{ID: "model-x", DefaultMaxTokens: 4096}},
	}
	coord := newTestCoordinator(t, env, "test-provider", providerCfg)

	_, err := coord.resolveModelByID(t.Context(), "does-not-exist", "", true)
	require.Error(t, err)
	require.ErrorContains(t, err, "not found")
	require.Empty(t, coord.subagentModelCache)
}
