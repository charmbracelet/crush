package agent

import (
	"context"
	"sync/atomic"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/goal"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

type goalLifecycleModel struct {
	autoReviewStreamModel
	calls       atomic.Int64
	firstFinish fantasy.FinishReason
}

func (m *goalLifecycleModel) Stream(_ context.Context, _ fantasy.Call) (fantasy.StreamResponse, error) {
	callNumber := m.calls.Add(1)
	return func(yield func(fantasy.StreamPart) bool) {
		switch callNumber {
		case 1:
			finish := m.firstFinish
			if finish == "" {
				finish = fantasy.FinishReasonLength
			}
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "progress"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "progress", Delta: "Work remains."})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "progress"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: finish})
		case 2:
			input := `{"status":"complete","summary":"verification passed"}`
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputStart, ID: "goal-1", ToolCallName: tools.GoalStatusToolName})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputDelta, ID: "goal-1", Delta: input})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputEnd, ID: "goal-1"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: "goal-1", ToolCallName: tools.GoalStatusToolName, ToolCallInput: input})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
		default:
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "done"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "done", Delta: "Verified and complete."})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "done"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
		}
	}, nil
}

func TestGoalModePreservesGoalAfterNormalStop(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	model := &goalLifecycleModel{firstFinish: fantasy.FinishReasonStop}
	titleModel := &autoReviewStreamModel{text: "title"}
	modelConfig := catwalk.Model{ContextWindow: 200_000, DefaultMaxTokens: 10_000}
	configured := Model{
		Model:      model,
		CatwalkCfg: modelConfig,
		ModelCfg:   config.SelectedModel{Provider: model.Provider(), Model: model.Model()},
	}
	title := Model{
		Model:      titleModel,
		CatwalkCfg: modelConfig,
		ModelCfg:   config.SelectedModel{Provider: titleModel.Provider(), Model: titleModel.Model()},
	}
	agent := NewSessionAgent(SessionAgentOptions{
		Models:       SessionAgentModels{Large: configured, Small: title, Summary: configured},
		SystemPrompt: "system",
		IsYolo:       true,
		Sessions:     env.sessions,
		Messages:     env.messages,
		Tools:        []fantasy.AgentTool{tools.NewGoalStatusTool()},
	}).(*sessionAgent)
	session, err := env.sessions.Create(t.Context(), "goal normal stop")
	require.NoError(t, err)
	state := goal.Start("finish despite a normal model stop")

	result, err := agent.Run(t.Context(), SessionAgentCall{
		SessionID:        session.ID,
		Prompt:           state.Objective,
		originalIntent:   state.Objective,
		goalMode:         true,
		goalState:        state,
		TransientContext: goal.Context(state, nil),
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(1), model.calls.Load())
	status, _, ok := terminalGoalStatus(result.Steps)
	require.False(t, ok)
	require.Empty(t, status)
	saved, err := env.sessions.Get(t.Context(), session.ID)
	require.NoError(t, err)
	require.Equal(t, goal.StatusPaused, saved.Goal.Status)
	require.Contains(t, saved.Goal.Summary, "goal preserved")
}

func TestTerminalGoalStatusRequiresSuccessfulToolResult(t *testing.T) {
	t.Parallel()

	complete := makeStep(
		[]fantasy.ToolCallContent{{ToolCallID: "goal-1", ToolName: tools.GoalStatusToolName, Input: `{"status":"complete","summary":"tests pass"}`}},
		[]fantasy.ToolResultContent{{ToolCallID: "goal-1", ToolName: tools.GoalStatusToolName, Result: fantasy.ToolResultOutputContentText{Text: "Goal complete"}}},
	)
	status, summary, ok := terminalGoalStatus([]fantasy.StepResult{complete})
	require.True(t, ok)
	require.Equal(t, goal.StatusComplete, status)
	require.Equal(t, "tests pass", summary)

	failed := makeStep(
		[]fantasy.ToolCallContent{{ToolCallID: "goal-2", ToolName: tools.GoalStatusToolName, Input: `{"status":"complete","summary":"tests pass"}`}},
		[]fantasy.ToolResultContent{{ToolCallID: "goal-2", ToolName: tools.GoalStatusToolName, Result: fantasy.ToolResultOutputContentError{Error: assertError("invalid")}}},
	)
	_, _, ok = terminalGoalStatus([]fantasy.StepResult{failed})
	require.False(t, ok)
}

func TestPrepareGoalContinuationPreservesIntentAndObservesFailures(t *testing.T) {
	t.Parallel()

	failedStep := makeStep(
		[]fantasy.ToolCallContent{{ToolCallID: "bash-1", ToolName: "bash"}},
		[]fantasy.ToolResultContent{{ToolCallID: "bash-1", ToolName: "bash", Result: fantasy.ToolResultOutputContentText{Text: "command failed\nExit code 1"}}},
	)
	state := goal.Start("fix the failing build").NextTurn()
	call := prepareGoalContinuation(SessionAgentCall{
		Prompt:         "fix the failing build",
		originalIntent: "fix the failing build",
		goalMode:       true,
	}, state, []fantasy.StepResult{failedStep})

	require.Equal(t, "fix the failing build", call.Prompt)
	require.True(t, call.skipUserMessage)
	require.Equal(t, state, call.goalState)
	require.Contains(t, call.TransientContext, `"tool":"bash"`)
	require.Contains(t, call.TransientContext, "Exit code 1")
	require.Contains(t, call.TransientContext, "Do not repeat the previous response")
}

func TestGoalModeContinuesAfterOutputLengthUntilTerminalStatus(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	model := &goalLifecycleModel{}
	titleModel := &autoReviewStreamModel{text: "title"}
	modelConfig := catwalk.Model{ContextWindow: 200_000, DefaultMaxTokens: 10_000}
	configured := Model{
		Model:      model,
		CatwalkCfg: modelConfig,
		ModelCfg:   config.SelectedModel{Provider: model.Provider(), Model: model.Model()},
	}
	title := Model{
		Model:      titleModel,
		CatwalkCfg: modelConfig,
		ModelCfg:   config.SelectedModel{Provider: titleModel.Provider(), Model: titleModel.Model()},
	}
	agent := NewSessionAgent(SessionAgentOptions{
		Models:       SessionAgentModels{Large: configured, Small: title, Summary: configured},
		SystemPrompt: "system",
		IsYolo:       true,
		Sessions:     env.sessions,
		Messages:     env.messages,
		Tools:        []fantasy.AgentTool{tools.NewGoalStatusTool()},
	}).(*sessionAgent)
	session, err := env.sessions.Create(t.Context(), "goal")
	require.NoError(t, err)

	result, err := agent.Run(t.Context(), SessionAgentCall{
		SessionID:        session.ID,
		Prompt:           "finish and verify the task",
		originalIntent:   "finish and verify the task",
		goalMode:         true,
		goalState:        goal.Start("finish and verify the task"),
		TransientContext: goal.Context(goal.Start("finish and verify the task"), nil),
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(2), model.calls.Load())
	require.Equal(t, 0, agent.QueuedPrompts(session.ID))
	status, _, ok := terminalGoalStatus(result.Steps)
	require.True(t, ok)
	require.Equal(t, goal.StatusComplete, status)
	saved, err := env.sessions.Get(t.Context(), session.ID)
	require.NoError(t, err)
	require.Equal(t, goal.StatusComplete, saved.Goal.Status)
	require.Equal(t, "verification passed", saved.Goal.Summary)
	require.Equal(t, 1, saved.Goal.Turns)

	messages, err := env.messages.List(t.Context(), session.ID)
	require.NoError(t, err)
	userMessages := 0
	for _, msg := range messages {
		if msg.Role == message.User {
			userMessages++
		}
	}
	require.Equal(t, 1, userMessages, "internal Goal continuations must not duplicate the user prompt")
}

type assertError string

func (e assertError) Error() string { return string(e) }
