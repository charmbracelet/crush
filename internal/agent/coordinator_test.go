package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/bedrock"
	"charm.land/fantasy/providers/openaicompat"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
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
func (m *mockSessionAgent) GenerateTitle(context.Context, string, string) {}

// newTestCoordinator creates a minimal coordinator for unit testing runSubAgent.
func newTestCoordinator(t *testing.T, env fakeEnv, providerID string, providerCfg config.ProviderConfig) *coordinator {
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Providers.Set(providerID, providerCfg)
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

	t.Run("cost update failure preserves output", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		agent := newMockAgent(providerID, 4096, func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
			return agentResultWithText("output before cost failure"), nil
		})

		resp, err := coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      "missing-parent-session",
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "test",
			SessionTitle:   "Test",
		})
		require.NoError(t, err)
		assert.False(t, resp.IsError)
		assert.Equal(t, "output before cost failure", resp.Content)
	})

	t.Run("response with text returns it", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		agent := newMockAgent(providerID, 4096, func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
			return agentResultWithText("the answer"), nil
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
		assert.Equal(t, "the answer", resp.Content)
	})

	t.Run("nil result returns error response", func(t *testing.T) {
		env := testEnv(t)
		coord := newTestCoordinator(t, env, providerID, providerCfg)

		parentSession, err := env.sessions.Create(t.Context(), "Parent")
		require.NoError(t, err)

		agent := newMockAgent(providerID, 4096, func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
			return nil, nil
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
		assert.Equal(t, "Sub-agent completed but produced no text output.", resp.Content)
	})

	t.Run("empty result returns error response", func(t *testing.T) {
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
		assert.True(t, resp.IsError)
		assert.Equal(t, "Sub-agent completed but produced no text output.", resp.Content)
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

func TestRunSubAgentInheritsNonInteractivePermissionPolicy(t *testing.T) {
	const providerID = "test-provider"
	providerCfg := config.ProviderConfig{ID: providerID}

	env := testEnv(t)
	coord := newTestCoordinator(t, env, providerID, providerCfg)
	permissions := permission.NewPermissionService(env.workingDir, false, nil)

	parentSession, err := env.sessions.Create(t.Context(), "Parent")
	require.NoError(t, err)
	outsidePath := t.TempDir()

	agent := newMockAgent(providerID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		assert.NotEqual(t, parentSession.ID, call.SessionID)
		assert.Equal(t, permission.RequestPolicyAutoApprove, call.PermissionPolicy)
		granted, err := permissions.Request(ctx, permission.CreatePermissionRequest{
			SessionID:  call.SessionID,
			ToolCallID: "child-call",
			ToolName:   "ls",
			Action:     "list",
			Path:       outsidePath,
		})
		if err != nil {
			return nil, err
		}
		if !granted {
			return nil, errors.New("child permission was denied")
		}
		return agentResultWithText("done"), nil
	})

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	resp, err := coord.runSubAgent(permission.WithAutoApproveRequests(ctx), subAgentParams{
		Agent:          agent,
		SessionID:      parentSession.ID,
		AgentMessageID: "msg-1",
		ToolCallID:     "call-1",
		Prompt:         "inspect outside the workspace",
		SessionTitle:   "Validator",
	})
	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.Equal(t, "done", resp.Content)
}

func TestRunSubAgentInteractiveParentDoesNotApproveChild(t *testing.T) {
	const providerID = "test-provider"
	providerCfg := config.ProviderConfig{ID: providerID}

	env := testEnv(t)
	coord := newTestCoordinator(t, env, providerID, providerCfg)
	permissions := permission.NewPermissionService(env.workingDir, false, nil)
	events := permissions.Subscribe(t.Context())
	outsidePath := t.TempDir()

	parentSession, err := env.sessions.Create(t.Context(), "Parent")
	require.NoError(t, err)

	calls := make(chan SessionAgentCall, 1)
	agent := newMockAgent(providerID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		calls <- call
		granted, err := permissions.Request(ctx, permission.CreatePermissionRequest{
			SessionID:  call.SessionID,
			ToolCallID: "child-call",
			ToolName:   "ls",
			Action:     "list",
			Path:       outsidePath,
		})
		if err != nil {
			return nil, err
		}
		if granted {
			return agentResultWithText("granted"), nil
		}
		return agentResultWithText("denied"), nil
	})

	type subAgentResult struct {
		response fantasy.ToolResponse
		err      error
	}
	result := make(chan subAgentResult, 1)
	go func() {
		response, err := coord.runSubAgent(t.Context(), subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "inspect outside the workspace",
			SessionTitle:   "Validator",
		})
		result <- subAgentResult{response: response, err: err}
	}()

	select {
	case call := <-calls:
		assert.True(t, call.NonInteractive)
		assert.Equal(t, permission.RequestPolicyPrompt, call.PermissionPolicy)
		assert.NotEqual(t, parentSession.ID, call.SessionID)
	case <-time.After(2 * time.Second):
		t.Fatal("child agent was never run")
	}

	select {
	case event := <-events:
		assert.True(t, permissions.Deny(event.Payload))
	case <-time.After(2 * time.Second):
		t.Fatal("child permission request was never published")
	}

	select {
	case got := <-result:
		require.NoError(t, got.err)
		assert.False(t, got.response.IsError)
		assert.Equal(t, "denied", got.response.Content)
	case <-time.After(2 * time.Second):
		t.Fatal("child agent did not return after permission denial")
	}
}

func TestRunSubAgentNestedDescendantInheritsPermissionPolicy(t *testing.T) {
	const providerID = "test-provider"
	providerCfg := config.ProviderConfig{ID: providerID}

	env := testEnv(t)
	coord := newTestCoordinator(t, env, providerID, providerCfg)
	permissions := permission.NewPermissionService(env.workingDir, false, nil)
	outsidePath := t.TempDir()

	parentSession, err := env.sessions.Create(t.Context(), "Parent")
	require.NoError(t, err)

	grandchildSessions := make(chan string, 1)
	grandchild := newMockAgent(providerID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		grandchildSessions <- call.SessionID
		granted, err := permissions.Request(ctx, permission.CreatePermissionRequest{
			SessionID:  call.SessionID,
			ToolCallID: "grandchild-permission",
			ToolName:   "ls",
			Action:     "list",
			Path:       outsidePath,
		})
		if err != nil {
			return nil, err
		}
		if !granted {
			return nil, errors.New("grandchild permission was denied")
		}
		return agentResultWithText("nested done"), nil
	})

	child := newMockAgent(providerID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		response, err := coord.runSubAgent(ctx, subAgentParams{
			Agent:          grandchild,
			SessionID:      call.SessionID,
			AgentMessageID: "child-msg",
			ToolCallID:     "grandchild-call",
			Prompt:         "inspect outside the workspace",
			SessionTitle:   "Nested Validator",
		})
		if err != nil {
			return nil, err
		}
		if response.IsError {
			return nil, errors.New(response.Content)
		}
		return agentResultWithText(response.Content), nil
	})

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	response, err := coord.runSubAgent(permission.WithAutoApproveRequests(ctx), subAgentParams{
		Agent:          child,
		SessionID:      parentSession.ID,
		AgentMessageID: "parent-msg",
		ToolCallID:     "child-call",
		Prompt:         "delegate validation",
		SessionTitle:   "Validator",
	})
	require.NoError(t, err)
	assert.False(t, response.IsError)
	assert.Equal(t, "nested done", response.Content)

	select {
	case grandchildSessionID := <-grandchildSessions:
		assert.NotEqual(t, parentSession.ID, grandchildSessionID)
	case <-time.After(2 * time.Second):
		t.Fatal("grandchild agent was never run")
	}
}

func TestRunSubAgentPermissionRequestCancellation(t *testing.T) {
	const providerID = "test-provider"
	providerCfg := config.ProviderConfig{ID: providerID}

	env := testEnv(t)
	coord := newTestCoordinator(t, env, providerID, providerCfg)
	permissions := permission.NewPermissionService(env.workingDir, false, nil)
	events := permissions.Subscribe(t.Context())
	outsidePath := t.TempDir()

	parentSession, err := env.sessions.Create(t.Context(), "Parent")
	require.NoError(t, err)

	agent := newMockAgent(providerID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		granted, err := permissions.Request(ctx, permission.CreatePermissionRequest{
			SessionID:  call.SessionID,
			ToolCallID: "child-call",
			ToolName:   "ls",
			Action:     "list",
			Path:       outsidePath,
		})
		if err != nil {
			return nil, err
		}
		if !granted {
			return nil, errors.New("child permission was denied")
		}
		return agentResultWithText("done"), nil
	})

	type subAgentResult struct {
		response fantasy.ToolResponse
		err      error
	}
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan subAgentResult, 1)
	go func() {
		response, err := coord.runSubAgent(ctx, subAgentParams{
			Agent:          agent,
			SessionID:      parentSession.ID,
			AgentMessageID: "msg-1",
			ToolCallID:     "call-1",
			Prompt:         "inspect outside the workspace",
			SessionTitle:   "Validator",
		})
		result <- subAgentResult{response: response, err: err}
	}()

	select {
	case <-events:
		cancel()
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("child permission request was never published")
	}

	select {
	case got := <-result:
		require.NoError(t, got.err)
		assert.True(t, got.response.IsError)
		assert.Contains(t, got.response.Content, context.Canceled.Error())
	case <-time.After(2 * time.Second):
		t.Fatal("child agent did not return after cancellation")
	}
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

func TestGetProviderOptionsReasoningEffortFallback(t *testing.T) {
	model := Model{
		CatwalkCfg: catwalk.Model{
			ID:              "glm-5.2",
			CanReason:       true,
			ReasoningLevels: []string{"high", "max"},
		},
		ModelCfg: config.SelectedModel{
			Provider: "zai",
		},
	}
	providerCfg := config.ProviderConfig{
		ID:   string(catwalk.InferenceProviderZAI),
		Type: openaicompat.Name,
	}

	opts := getProviderOptions(model, providerCfg)

	raw, ok := opts[openaicompat.Name]
	require.True(t, ok)
	parsed, ok := raw.(*openaicompat.ProviderOptions)
	require.True(t, ok)
	require.NotNil(t, parsed.ReasoningEffort)
	assert.Equal(t, "high", string(*parsed.ReasoningEffort))

	thinking, ok := parsed.ExtraBody["thinking"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "enabled", thinking["type"])
}
