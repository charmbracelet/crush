package agent

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/mailbox"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
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
	rootSession, err := env.sessions.Create(context.Background(), "taskgraph-parallel")
	require.NoError(t, err)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	coord := &coordinator{cfg: cfg, mailbox: mailbox.NewService(), sessions: env.sessions}

	var running int32
	var maxRunning int32
	var runMu sync.Mutex
	runOrder := make([]string, 0)

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
				runMu.Unlock()
				atomic.AddInt32(&running, -1)
				return &fantasy.AgentResult{Response: fantasy.Response{Content: fantasy.ResponseContent{fantasy.TextContent{Text: "ok"}}}}, nil
			},
		}
		return agent, config.Agent{ID: taskID, Description: taskID, Mode: config.AgentModeSubagent}, nil
	}

	coord.subAgentScheduler = func(ctx context.Context, params subAgentParams) (fantasy.ToolResponse, error) {
		_, err := params.Agent.Run(ctx, SessionAgentCall{Prompt: params.Prompt})
		if err != nil {
			return fantasy.NewTextErrorResponse(err.Error()), nil
		}
		return withSubtaskToolResponseMetadata(
			fantasy.NewTextResponse("done"),
			params.ToolCallID,
			params.AgentMessageID+"$$"+params.ToolCallID,
			params.ParentMessageID,
			message.ToolResultSubtaskStatusCompleted,
		), nil
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		_, _ = coord.mailbox.Send("call-1", "c", "prioritize final checks")
	}()

	resp, err := coord.runTaskGraphDirect(t.Context(), taskGraphParams{
		SessionID:      rootSession.ID,
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
	require.Contains(t, resp.Metadata, "mailbox_id")
	require.Contains(t, resp.Content, "Task outputs:")
	require.Contains(t, resp.Content, "- c (completed): done")

	loadedSession, err := env.sessions.Get(context.Background(), rootSession.ID)
	require.NoError(t, err)
	require.Len(t, loadedSession.Todos, 3)
	var todoC session.Todo
	for _, todo := range loadedSession.Todos {
		if todo.ID == "c" {
			todoC = todo
			break
		}
	}
	require.Equal(t, session.TodoStatusCompleted, todoC.Status)
	require.Contains(t, todoC.Content, "mailbox:prioritize final checks")

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
	reducerMeta, hasReducer := message.ParseToolResultReducer(resp.Metadata)
	require.True(t, hasReducer)
	require.Empty(t, reducerMeta.PatchPlan)
	require.Empty(t, reducerMeta.TestResults)
	require.Empty(t, reducerMeta.FollowupQuestions)
}

func TestRunTaskGraphDirect_PropagatesFailureToDependents(t *testing.T) {
	env := testEnv(t)
	rootSession, err := env.sessions.Create(context.Background(), "taskgraph-failure")
	require.NoError(t, err)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	coord := &coordinator{cfg: cfg, mailbox: mailbox.NewService(), sessions: env.sessions}

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
				params.ParentMessageID,
				message.ToolResultSubtaskStatusFailed,
			), nil
		}
		return withSubtaskToolResponseMetadata(
			fantasy.NewTextResponse("ok"),
			params.ToolCallID,
			"child",
			params.ParentMessageID,
			message.ToolResultSubtaskStatusCompleted,
		), nil
	}

	resp, err := coord.runTaskGraphDirect(t.Context(), taskGraphParams{
		SessionID:      rootSession.ID,
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
	require.Empty(t, reducerMeta.PatchPlan)
	require.Empty(t, reducerMeta.TestResults)
	require.Empty(t, reducerMeta.FollowupQuestions)

	loadedSession, err := env.sessions.Get(context.Background(), rootSession.ID)
	require.NoError(t, err)
	require.Len(t, loadedSession.Todos, 2)
	statuses := map[string]session.TodoStatus{}
	for _, todo := range loadedSession.Todos {
		statuses[todo.ID] = todo.Status
	}
	require.Equal(t, session.TodoStatusFailed, statuses["root"])
	require.Equal(t, session.TodoStatusCanceled, statuses["child"])
}

func TestRunTaskGraphDirect_SingleTaskKeepsSubtaskMetadata(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	coord := &coordinator{cfg: cfg, mailbox: mailbox.NewService(), sessions: env.sessions}

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
			params.ParentMessageID,
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
	coord := &coordinator{cfg: cfg, mailbox: mailbox.NewService(), sessions: env.sessions}

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

func TestRunTaskGraphDirect_HonorsMaxConcurrentPerAgent(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	maxConcurrent := 1
	cfg.Config().Agents[config.AgentGeneral] = config.Agent{
		ID:          config.AgentGeneral,
		Description: "general",
		Mode:        config.AgentModeSubagent,
		TaskGovernance: &config.TaskGovernance{
			MaxConcurrent: &maxConcurrent,
		},
	}
	coord := &coordinator{cfg: cfg, mailbox: mailbox.NewService(), sessions: env.sessions}

	var running int32
	var maxRunning int32
	coord.subAgentFactory = func(_ context.Context, requestedType string) (SessionAgent, config.Agent, error) {
		agent := &taskGraphMockSessionAgent{
			model: Model{CatwalkCfg: catwalk.Model{DefaultMaxTokens: 1000}, ModelCfg: config.SelectedModel{Provider: "test-provider", Model: "test-model"}},
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
				atomic.AddInt32(&running, -1)
				return &fantasy.AgentResult{Response: fantasy.Response{Content: fantasy.ResponseContent{fantasy.TextContent{Text: "ok"}}}}, nil
			},
		}
		return agent, cfg.Config().Agents[config.AgentGeneral], nil
	}
	coord.subAgentScheduler = func(ctx context.Context, params subAgentParams) (fantasy.ToolResponse, error) {
		_, err := params.Agent.Run(ctx, SessionAgentCall{Prompt: params.Prompt})
		require.NoError(t, err)
		return withSubtaskToolResponseMetadata(fantasy.NewTextResponse("ok"), params.ToolCallID, params.ToolCallID, params.ParentMessageID, message.ToolResultSubtaskStatusCompleted), nil
	}

	resp, err := coord.runTaskGraphDirect(t.Context(), taskGraphParams{
		SessionID:      "session-1",
		AgentMessageID: "msg-1",
		ToolCallID:     "call-1",
		Tasks: []taskGraphTask{
			{ID: "a", Prompt: "task-a", SubagentType: "general"},
			{ID: "b", Prompt: "task-b", SubagentType: "general"},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Equal(t, int32(1), atomic.LoadInt32(&maxRunning))
}

func TestRunTaskGraphDirect_ReadyQueueStartsDependentsBeforePeerRootsFinish(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	coord := &coordinator{cfg: cfg, mailbox: mailbox.NewService(), sessions: env.sessions}

	var eventMu sync.Mutex
	events := make([]string, 0, 6)
	record := func(event string) {
		eventMu.Lock()
		events = append(events, event)
		eventMu.Unlock()
	}

	coord.subAgentFactory = func(_ context.Context, requestedType string) (SessionAgent, config.Agent, error) {
		agent := &taskGraphMockSessionAgent{
			model: Model{CatwalkCfg: catwalk.Model{DefaultMaxTokens: 1000}, ModelCfg: config.SelectedModel{Provider: "test-provider", Model: "test-model"}},
			runFunc: func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
				record(requestedType + "-start")
				switch requestedType {
				case "a":
					time.Sleep(20 * time.Millisecond)
					record("a-done")
				case "b":
					time.Sleep(120 * time.Millisecond)
					record("b-done")
				case "c":
					record("c-done")
				}
				return &fantasy.AgentResult{Response: fantasy.Response{Content: fantasy.ResponseContent{fantasy.TextContent{Text: "ok"}}}}, nil
			},
		}
		return agent, config.Agent{ID: requestedType, Description: requestedType, Mode: config.AgentModeSubagent}, nil
	}
	coord.subAgentScheduler = func(ctx context.Context, params subAgentParams) (fantasy.ToolResponse, error) {
		_, err := params.Agent.Run(ctx, SessionAgentCall{Prompt: params.Prompt})
		require.NoError(t, err)
		return withSubtaskToolResponseMetadata(
			fantasy.NewTextResponse("ok"),
			params.ToolCallID,
			params.ToolCallID,
			params.ParentMessageID,
			message.ToolResultSubtaskStatusCompleted,
		), nil
	}

	resp, err := coord.runTaskGraphDirect(t.Context(), taskGraphParams{
		SessionID:      "session-1",
		AgentMessageID: "msg-1",
		ToolCallID:     "call-ready",
		Tasks: []taskGraphTask{
			{ID: "a", Prompt: "task-a", SubagentType: "a"},
			{ID: "b", Prompt: "task-b", SubagentType: "b"},
			{ID: "c", Prompt: "task-c", SubagentType: "c", DependsOn: []string{"a"}},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)

	eventMu.Lock()
	defer eventMu.Unlock()
	require.Contains(t, events, "c-start")
	require.Contains(t, events, "b-done")
	cIdx := slices.Index(events, "c-start")
	bDoneIdx := slices.Index(events, "b-done")
	require.NotEqual(t, -1, cIdx)
	require.NotEqual(t, -1, bDoneIdx)
	require.Less(t, cIdx, bDoneIdx)
}

func TestRunTaskGraphDirect_RetriesFailuresWithinBudget(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	retryBudget := 1
	cfg.Config().Agents[config.AgentGeneral] = config.Agent{
		ID:          config.AgentGeneral,
		Description: "general",
		Mode:        config.AgentModeSubagent,
		TaskGovernance: &config.TaskGovernance{
			RetryBudget: &retryBudget,
		},
	}
	coord := &coordinator{cfg: cfg, mailbox: mailbox.NewService(), sessions: env.sessions}

	var attempts int32
	coord.subAgentFactory = func(_ context.Context, requestedType string) (SessionAgent, config.Agent, error) {
		return &taskGraphMockSessionAgent{model: Model{CatwalkCfg: catwalk.Model{DefaultMaxTokens: 1000}, ModelCfg: config.SelectedModel{Provider: "test-provider", Model: "test-model"}}}, cfg.Config().Agents[config.AgentGeneral], nil
	}
	coord.subAgentScheduler = func(_ context.Context, params subAgentParams) (fantasy.ToolResponse, error) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt == 1 {
			return withSubtaskToolResponseMetadata(fantasy.NewTextErrorResponse("try again"), params.ToolCallID, "", params.ParentMessageID, message.ToolResultSubtaskStatusFailed), nil
		}
		return withSubtaskToolResponseMetadata(fantasy.NewTextResponse("ok"), params.ToolCallID, params.ToolCallID, params.ParentMessageID, message.ToolResultSubtaskStatusCompleted), nil
	}

	resp, err := coord.runTaskGraphDirect(t.Context(), taskGraphParams{
		SessionID:      "session-1",
		AgentMessageID: "msg-1",
		ToolCallID:     "call-1",
		Tasks: []taskGraphTask{
			{ID: "a", Prompt: "task-a", SubagentType: "general"},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Equal(t, int32(2), atomic.LoadInt32(&attempts))
}

func TestRunTaskGraphDirect_TimesOutTaskAttempts(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	timeoutSeconds := 1
	cfg.Config().Agents[config.AgentGeneral] = config.Agent{
		ID:          config.AgentGeneral,
		Description: "general",
		Mode:        config.AgentModeSubagent,
		TaskGovernance: &config.TaskGovernance{
			TimeoutSeconds: &timeoutSeconds,
		},
	}
	coord := &coordinator{cfg: cfg, mailbox: mailbox.NewService(), sessions: env.sessions}

	coord.subAgentFactory = func(_ context.Context, requestedType string) (SessionAgent, config.Agent, error) {
		agent := &taskGraphMockSessionAgent{
			model: Model{CatwalkCfg: catwalk.Model{DefaultMaxTokens: 1000}, ModelCfg: config.SelectedModel{Provider: "test-provider", Model: "test-model"}},
			runFunc: func(ctx context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
		}
		return agent, cfg.Config().Agents[config.AgentGeneral], nil
	}
	coord.subAgentScheduler = func(ctx context.Context, params subAgentParams) (fantasy.ToolResponse, error) {
		_, err := params.Agent.Run(ctx, SessionAgentCall{Prompt: params.Prompt})
		if err != nil {
			return fantasy.NewTextErrorResponse(err.Error()), nil
		}
		return withSubtaskToolResponseMetadata(fantasy.NewTextResponse("ok"), params.ToolCallID, params.ToolCallID, params.ParentMessageID, message.ToolResultSubtaskStatusCompleted), nil
	}

	resp, err := coord.runTaskGraphDirect(t.Context(), taskGraphParams{
		SessionID:      "session-1",
		AgentMessageID: "msg-1",
		ToolCallID:     "call-1",
		Tasks: []taskGraphTask{
			{ID: "a", Prompt: "task-a", SubagentType: "general"},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "a: failed")
}

func TestRunTaskGraphDirect_FailFastStopsLaterLayers(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	failFast := true
	cfg.Config().Agents[config.AgentGeneral] = config.Agent{
		ID:          config.AgentGeneral,
		Description: "general",
		Mode:        config.AgentModeSubagent,
		TaskGovernance: &config.TaskGovernance{
			FailFast: &failFast,
		},
	}
	coord := &coordinator{cfg: cfg, mailbox: mailbox.NewService(), sessions: env.sessions}

	coord.subAgentFactory = func(_ context.Context, requestedType string) (SessionAgent, config.Agent, error) {
		agent := &taskGraphMockSessionAgent{model: Model{CatwalkCfg: catwalk.Model{DefaultMaxTokens: 1000}, ModelCfg: config.SelectedModel{Provider: "test-provider", Model: "test-model"}}}
		return agent, cfg.Config().Agents[config.AgentGeneral], nil
	}
	var attempts int32
	coord.subAgentScheduler = func(_ context.Context, params subAgentParams) (fantasy.ToolResponse, error) {
		atomic.AddInt32(&attempts, 1)
		if params.ToolCallID == "call-1::a" {
			time.Sleep(10 * time.Millisecond)
			return withSubtaskToolResponseMetadata(fantasy.NewTextErrorResponse("boom"), params.ToolCallID, "", params.ParentMessageID, message.ToolResultSubtaskStatusFailed), nil
		}
		return withSubtaskToolResponseMetadata(fantasy.NewTextResponse("ok"), params.ToolCallID, params.ToolCallID, params.ParentMessageID, message.ToolResultSubtaskStatusCompleted), nil
	}

	resp, err := coord.runTaskGraphDirect(t.Context(), taskGraphParams{
		SessionID:      "session-1",
		AgentMessageID: "msg-1",
		ToolCallID:     "call-1",
		Tasks: []taskGraphTask{
			{ID: "a", Prompt: "task-a", SubagentType: "general"},
			{ID: "b", Prompt: "task-b", SubagentType: "general"},
			{ID: "c", Prompt: "task-c", SubagentType: "general", DependsOn: []string{"a", "b"}},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Equal(t, int32(2), atomic.LoadInt32(&attempts))
	require.Contains(t, resp.Content, "a: failed")
	require.Contains(t, resp.Content, "c: canceled")
}

func TestRunTaskGraphDirect_HonorsGraphTimeout(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	graphTimeout := 1
	cfg.Config().Agents[config.AgentGeneral] = config.Agent{
		ID:          config.AgentGeneral,
		Description: "general",
		Mode:        config.AgentModeSubagent,
		TaskGovernance: &config.TaskGovernance{
			GraphTimeoutSeconds: &graphTimeout,
		},
	}
	coord := &coordinator{cfg: cfg, mailbox: mailbox.NewService(), sessions: env.sessions}

	coord.subAgentFactory = func(_ context.Context, requestedType string) (SessionAgent, config.Agent, error) {
		agent := &taskGraphMockSessionAgent{
			model: Model{CatwalkCfg: catwalk.Model{DefaultMaxTokens: 1000}, ModelCfg: config.SelectedModel{Provider: "test-provider", Model: "test-model"}},
			runFunc: func(ctx context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
		}
		return agent, cfg.Config().Agents[config.AgentGeneral], nil
	}
	coord.subAgentScheduler = func(ctx context.Context, params subAgentParams) (fantasy.ToolResponse, error) {
		_, err := params.Agent.Run(ctx, SessionAgentCall{Prompt: params.Prompt})
		if err != nil {
			return fantasy.NewTextErrorResponse(err.Error()), nil
		}
		return withSubtaskToolResponseMetadata(fantasy.NewTextResponse("ok"), params.ToolCallID, params.ToolCallID, params.ParentMessageID, message.ToolResultSubtaskStatusCompleted), nil
	}

	resp, err := coord.runTaskGraphDirect(t.Context(), taskGraphParams{
		SessionID:      "session-1",
		AgentMessageID: "msg-1",
		ToolCallID:     "call-1",
		Tasks: []taskGraphTask{
			{ID: "a", Prompt: "task-a", SubagentType: "general"},
			{ID: "b", Prompt: "task-b", SubagentType: "general"},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "failed")
}

func TestRunTaskGraphUsesInjectedScheduler(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	coord := &coordinator{cfg: cfg, mailbox: mailbox.NewService(), sessions: env.sessions}

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

func TestRunTaskGraphDirect_MailboxStopCancelsTask(t *testing.T) {
	env := testEnv(t)
	rootSession, err := env.sessions.Create(context.Background(), "taskgraph-stop")
	require.NoError(t, err)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	retryBudget := 1
	cfg.Config().Agents[config.AgentGeneral] = config.Agent{
		ID:          config.AgentGeneral,
		Description: "general",
		Mode:        config.AgentModeSubagent,
		TaskGovernance: &config.TaskGovernance{
			RetryBudget: &retryBudget,
		},
	}
	coord := &coordinator{cfg: cfg, mailbox: mailbox.NewService(), sessions: env.sessions}

	coord.subAgentFactory = func(_ context.Context, requestedType string) (SessionAgent, config.Agent, error) {
		agent := &taskGraphMockSessionAgent{
			model: Model{CatwalkCfg: catwalk.Model{DefaultMaxTokens: 1000}, ModelCfg: config.SelectedModel{Provider: "test-provider", Model: "test-model"}},
			runFunc: func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
				time.Sleep(20 * time.Millisecond)
				return &fantasy.AgentResult{Response: fantasy.Response{Content: fantasy.ResponseContent{fantasy.TextContent{Text: "ok"}}}}, nil
			},
		}
		return agent, cfg.Config().Agents[config.AgentGeneral], nil
	}
	attempts := int32(0)
	coord.subAgentScheduler = func(_ context.Context, params subAgentParams) (fantasy.ToolResponse, error) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt == 1 {
			time.Sleep(30 * time.Millisecond)
			return withSubtaskToolResponseMetadata(
				fantasy.NewTextErrorResponse("retry"),
				params.ToolCallID,
				"",
				params.ParentMessageID,
				message.ToolResultSubtaskStatusFailed,
			), nil
		}
		return withSubtaskToolResponseMetadata(
			fantasy.NewTextResponse("ok"),
			params.ToolCallID,
			"child",
			params.ParentMessageID,
			message.ToolResultSubtaskStatusCompleted,
		), nil
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		_, _ = coord.mailbox.Stop("call-stop", "a", "halted by parent")
	}()
	resp, err := coord.runTaskGraphDirect(t.Context(), taskGraphParams{
		SessionID:      rootSession.ID,
		AgentMessageID: "msg-1",
		ToolCallID:     "call-stop",
		Tasks: []taskGraphTask{
			{ID: "a", Prompt: "run", SubagentType: "general"},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "a: canceled")
	require.GreaterOrEqual(t, atomic.LoadInt32(&attempts), int32(1))

	loadedSession, err := env.sessions.Get(context.Background(), rootSession.ID)
	require.NoError(t, err)
	require.Len(t, loadedSession.Todos, 1)
	require.Equal(t, session.TodoStatusCanceled, loadedSession.Todos[0].Status)
}

func TestRunTaskGraphDirect_TruncatesTaskOutputsForModel(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	coord := &coordinator{cfg: cfg, mailbox: mailbox.NewService(), sessions: env.sessions}

	coord.subAgentFactory = func(_ context.Context, requestedType string) (SessionAgent, config.Agent, error) {
		agent := &taskGraphMockSessionAgent{
			model: Model{CatwalkCfg: catwalk.Model{DefaultMaxTokens: 1000}, ModelCfg: config.SelectedModel{Provider: "test-provider", Model: "test-model"}},
		}
		return agent, config.Agent{ID: requestedType, Description: requestedType, Mode: config.AgentModeSubagent}, nil
	}
	coord.subAgentScheduler = func(_ context.Context, params subAgentParams) (fantasy.ToolResponse, error) {
		big := strings.Repeat("x", taskGraphOutputPerTaskCharsLimit+200)
		return withSubtaskToolResponseMetadata(
			fantasy.NewTextResponse(big),
			params.ToolCallID,
			"child",
			params.ParentMessageID,
			message.ToolResultSubtaskStatusCompleted,
		), nil
	}

	resp, err := coord.runTaskGraphDirect(t.Context(), taskGraphParams{
		SessionID:      "session-1",
		AgentMessageID: "msg-1",
		ToolCallID:     "call-1",
		Tasks: []taskGraphTask{
			{ID: "a", Prompt: "task-a", SubagentType: "general", Description: "alpha"},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "Task outputs:")
	require.Contains(t, resp.Content, "[truncated]")
}

func TestTaskGraphPromptWithMailboxMessagesAddsOmissionNotice(t *testing.T) {
	messages := make([]string, 0, taskGraphMailboxMessagesLimit+2)
	for i := 0; i < taskGraphMailboxMessagesLimit+2; i++ {
		messages = append(messages, fmt.Sprintf("message-%d", i))
	}
	prompt := taskGraphPromptWithMailboxMessages("run", messages)
	require.Contains(t, prompt, "earlier mailbox message(s) omitted")
}

func TestTaskGraphPromptWithMailboxMessagesKeepsUTF8WhenTrimmed(t *testing.T) {
	message := strings.Repeat("你", taskGraphMailboxPromptCharsLimit+20)
	prompt := taskGraphPromptWithMailboxMessages("run", []string{message})
	require.True(t, utf8.ValidString(prompt))
	require.Contains(t, prompt, "…")
}

func TestRunTaskGraphDirect_RecoversFromTaskPanic(t *testing.T) {
	env := testEnv(t)
	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	coord := &coordinator{cfg: cfg, mailbox: mailbox.NewService(), sessions: env.sessions}

	coord.subAgentFactory = func(_ context.Context, requestedType string) (SessionAgent, config.Agent, error) {
		agent := &taskGraphMockSessionAgent{
			model: Model{CatwalkCfg: catwalk.Model{DefaultMaxTokens: 1000}, ModelCfg: config.SelectedModel{Provider: "test-provider", Model: "test-model"}},
			runFunc: func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
				return &fantasy.AgentResult{Response: fantasy.Response{Content: fantasy.ResponseContent{fantasy.TextContent{Text: "ok"}}}}, nil
			},
		}
		return agent, config.Agent{ID: requestedType, Description: requestedType, Mode: config.AgentModeSubagent}, nil
	}

	coord.subAgentScheduler = func(_ context.Context, _ subAgentParams) (fantasy.ToolResponse, error) {
		panic("scheduler crashed")
	}

	resp, err := coord.runTaskGraphDirect(t.Context(), taskGraphParams{
		SessionID:      "session-1",
		AgentMessageID: "msg-1",
		ToolCallID:     "call-panic",
		Tasks: []taskGraphTask{
			{ID: "a", Prompt: "task-a", SubagentType: "general"},
			{ID: "b", Prompt: "task-b", SubagentType: "general"},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "a: failed")
	require.Contains(t, resp.Content, "b: failed")
}
