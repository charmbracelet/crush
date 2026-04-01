package agent

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

type taskGraphMockSessionAgent struct {
	model   Model
	runFunc func(context.Context, SessionAgentCall) (*fantasy.AgentResult, error)
}

func (m *taskGraphMockSessionAgent) Run(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
	if m.runFunc != nil {
		return m.runFunc(ctx, call)
	}
	return &fantasy.AgentResult{}, nil
}

func (m *taskGraphMockSessionAgent) EstimateSessionPromptTokensForModel(context.Context, string, Model) (int64, error) {
	return 0, nil
}

func (m *taskGraphMockSessionAgent) Model() Model                                    { return m.model }
func (m *taskGraphMockSessionAgent) SetModels(large, small Model)                    {}
func (m *taskGraphMockSessionAgent) SetTools(tools []fantasy.AgentTool)              {}
func (m *taskGraphMockSessionAgent) SetSystemPrompt(systemPrompt string)             {}
func (m *taskGraphMockSessionAgent) SetSystemPromptPrefix(systemPromptPrefix string) {}
func (m *taskGraphMockSessionAgent) Cancel(sessionID string)                         {}
func (m *taskGraphMockSessionAgent) CancelAll()                                      {}
func (m *taskGraphMockSessionAgent) IsSessionBusy(sessionID string) bool             { return false }
func (m *taskGraphMockSessionAgent) IsBusy() bool                                    { return false }
func (m *taskGraphMockSessionAgent) QueuedPrompts(sessionID string) int              { return 0 }
func (m *taskGraphMockSessionAgent) QueuedPromptsList(sessionID string) []string     { return nil }
func (m *taskGraphMockSessionAgent) RemoveQueuedPrompt(sessionID string, index int) bool {
	return false
}
func (m *taskGraphMockSessionAgent) ClearQueue(sessionID string)  {}
func (m *taskGraphMockSessionAgent) PauseQueue(sessionID string)  {}
func (m *taskGraphMockSessionAgent) ResumeQueue(sessionID string) {}
func (m *taskGraphMockSessionAgent) IsQueuePaused(sessionID string) bool {
	return false
}
func (m *taskGraphMockSessionAgent) PrioritizeQueuedPrompt(sessionID string, index int) bool {
	return false
}
func (m *taskGraphMockSessionAgent) Summarize(context.Context, string, fantasy.ProviderOptions) error {
	return nil
}

func TestRunTaskGraphDirect_ParallelAndDependencies(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	coord := &coordinator{cfg: cfg}

	var running int32
	var maxRunning int32
	var runMu sync.Mutex
	runOrder := make([]string, 0)
	completed := make(map[string]bool)

	coord.subAgentFactory = func(_ context.Context, requestedType string) (SessionAgent, config.Agent, error) {
		taskID := requestedType
		agent := &taskGraphMockSessionAgent{
			model: Model{
				CatwalkCfg: catwalk.Model{DefaultMaxTokens: 1000},
				ModelCfg:   config.SelectedModel{Provider: "test-provider", Model: "test-model"},
			},
			runFunc: func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
				current := atomic.AddInt32(&running, 1)
				for {
					prev := atomic.LoadInt32(&maxRunning)
					if current <= prev {
						break
					}
					if atomic.CompareAndSwapInt32(&maxRunning, prev, current) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
				runMu.Lock()
				runOrder = append(runOrder, taskID)
				completed[taskID] = true
				runMu.Unlock()
				atomic.AddInt32(&running, -1)
				return &fantasy.AgentResult{Response: fantasy.Response{Content: fantasy.ResponseContent{fantasy.TextContent{Text: "ok"}}}}, nil
			},
		}
		return agent, config.Agent{ID: taskID, Description: taskID, Mode: config.AgentModeSubagent}, nil
	}

	coord.subAgentScheduler = func(_ context.Context, params subAgentParams) (fantasy.ToolResponse, error) {
		_, err := params.Agent.Run(context.Background(), SessionAgentCall{Prompt: params.Prompt})
		if err != nil {
			return fantasy.NewTextErrorResponse(err.Error()), nil
		}
		return withSubtaskToolResponseMetadata(
			fantasy.NewTextResponse("done"),
			params.ToolCallID,
			params.AgentMessageID+"$$"+params.ToolCallID,
			message.ToolResultSubtaskStatusCompleted,
		), nil
	}

	resp, err := coord.runTaskGraphDirect(t.Context(), taskGraphParams{
		SessionID:      "session-1",
		AgentMessageID: "msg-1",
		ToolCallID:     "call-1",
		Tasks: []taskGraphTask{
			{ID: "a", Prompt: "task-a", SubagentType: "a"},
			{ID: "b", Prompt: "task-b", SubagentType: "b"},
			{ID: "c", Prompt: "task-c", SubagentType: "c", DependsOn: []string{"a", "b"}},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.GreaterOrEqual(t, atomic.LoadInt32(&maxRunning), int32(2))

	runMu.Lock()
	defer runMu.Unlock()
	require.Len(t, runOrder, 3)
	idxC := -1
	for i, id := range runOrder {
		if id == "c" {
			idxC = i
			break
		}
	}
	require.Equal(t, 2, idxC)
	_, hasReducer := message.ParseToolResultReducer(resp.Metadata)
	require.True(t, hasReducer)
}

func TestRunTaskGraphDirect_PropagatesFailureToDependents(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	coord := &coordinator{cfg: cfg}

	coord.subAgentFactory = func(_ context.Context, requestedType string) (SessionAgent, config.Agent, error) {
		agent := &taskGraphMockSessionAgent{
			model: Model{CatwalkCfg: catwalk.Model{DefaultMaxTokens: 1000}, ModelCfg: config.SelectedModel{Provider: "test-provider", Model: "test-model"}},
			runFunc: func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
				return &fantasy.AgentResult{Response: fantasy.Response{Content: fantasy.ResponseContent{fantasy.TextContent{Text: "ok"}}}}, nil
			},
		}
		return agent, config.Agent{ID: requestedType, Description: requestedType, Mode: config.AgentModeSubagent}, nil
	}

	coord.subAgentScheduler = func(_ context.Context, params subAgentParams) (fantasy.ToolResponse, error) {
		if params.ToolCallID == "call-graph::root" {
			return withSubtaskToolResponseMetadata(
				fantasy.NewTextErrorResponse("root failed"),
				params.ToolCallID,
				"",
				message.ToolResultSubtaskStatusFailed,
			), nil
		}
		return withSubtaskToolResponseMetadata(
			fantasy.NewTextResponse("ok"),
			params.ToolCallID,
			"child",
			message.ToolResultSubtaskStatusCompleted,
		), nil
	}

	resp, err := coord.runTaskGraphDirect(t.Context(), taskGraphParams{
		SessionID:      "session-1",
		AgentMessageID: "msg-1",
		ToolCallID:     "call-graph",
		Tasks: []taskGraphTask{
			{ID: "root", Prompt: "root", SubagentType: "general"},
			{ID: "child", Prompt: "child", SubagentType: "general", DependsOn: []string{"root"}},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "root: failed")
	require.Contains(t, resp.Content, "child: canceled")
	reducerMeta, ok := message.ParseToolResultReducer(resp.Metadata)
	require.True(t, ok)
	require.Equal(t, "low", reducerMeta.Confidence)
	require.NotEmpty(t, reducerMeta.Risks)
}

func TestRunTaskGraphDirect_SingleTaskKeepsSubtaskMetadata(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	coord := &coordinator{cfg: cfg}

	coord.subAgentFactory = func(_ context.Context, requestedType string) (SessionAgent, config.Agent, error) {
		agent := &taskGraphMockSessionAgent{
			model: Model{CatwalkCfg: catwalk.Model{DefaultMaxTokens: 1000}, ModelCfg: config.SelectedModel{Provider: "test-provider", Model: "test-model"}},
		}
		return agent, config.Agent{ID: requestedType, Description: requestedType, Mode: config.AgentModeSubagent}, nil
	}
	coord.subAgentScheduler = func(_ context.Context, params subAgentParams) (fantasy.ToolResponse, error) {
		return withSubtaskToolResponseMetadata(
			fantasy.NewTextResponse("ok"),
			params.ToolCallID,
			"child-session-1",
			message.ToolResultSubtaskStatusCompleted,
		), nil
	}

	resp, err := coord.runTaskGraphDirect(t.Context(), taskGraphParams{
		SessionID:      "session-1",
		AgentMessageID: "msg-1",
		ToolCallID:     "call-1",
		Tasks: []taskGraphTask{
			{ID: "only", Prompt: "only", SubagentType: "general"},
		},
	})
	require.NoError(t, err)
	subtask, ok := message.ParseToolResultSubtaskResult(resp.Metadata)
	require.True(t, ok)
	require.Equal(t, "child-session-1", subtask.ChildSessionID)
	require.Equal(t, message.ToolResultSubtaskStatusCompleted, subtask.Status)
	reducerMeta, hasReducer := message.ParseToolResultReducer(resp.Metadata)
	require.True(t, hasReducer)
	require.Equal(t, "high", reducerMeta.Confidence)
}

func TestRunTaskGraphDirect_InvalidGraphReturnsToolError(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	coord := &coordinator{cfg: cfg}

	resp, err := coord.runTaskGraphDirect(t.Context(), taskGraphParams{
		SessionID:      "session-1",
		AgentMessageID: "msg-1",
		ToolCallID:     "call-1",
		Tasks: []taskGraphTask{
			{ID: "a", Prompt: "a", SubagentType: "general", DependsOn: []string{"missing"}},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, `depends on missing task "missing"`)
}

func TestRunTaskGraphUsesInjectedScheduler(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	coord := &coordinator{cfg: cfg}

	called := false
	coord.taskGraphScheduler = func(_ context.Context, params taskGraphParams) (fantasy.ToolResponse, error) {
		called = true
		return fantasy.NewTextResponse(fmt.Sprintf("scheduled-%d", len(params.Tasks))), nil
	}

	resp, err := coord.runTaskGraph(t.Context(), taskGraphParams{Tasks: []taskGraphTask{{ID: "a"}}})
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, "scheduled-1", resp.Content)
}
