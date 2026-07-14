package agent

import (
	"context"
	"sync/atomic"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

type goalLifecycleModel struct {
	autoReviewStreamModel
	calls atomic.Int64
}

func (m *goalLifecycleModel) Stream(_ context.Context, _ fantasy.Call) (fantasy.StreamResponse, error) {
	callNumber := m.calls.Add(1)
	return func(yield func(fantasy.StreamPart) bool) {
		switch callNumber {
		case 1:
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "progress"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "progress", Delta: "Output was truncated."})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "progress"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonLength})
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

func TestTerminalGoalStatusRequiresSuccessfulToolResult(t *testing.T) {
	t.Parallel()

	complete := makeStep(
		[]fantasy.ToolCallContent{{ToolCallID: "goal-1", ToolName: tools.GoalStatusToolName, Input: `{"status":"complete","summary":"tests pass"}`}},
		[]fantasy.ToolResultContent{{ToolCallID: "goal-1", ToolName: tools.GoalStatusToolName, Result: fantasy.ToolResultOutputContentText{Text: "Goal complete"}}},
	)
	status, ok := terminalGoalStatus([]fantasy.StepResult{complete})
	require.True(t, ok)
	require.Equal(t, "complete", status)

	failed := makeStep(
		[]fantasy.ToolCallContent{{ToolCallID: "goal-2", ToolName: tools.GoalStatusToolName, Input: `{"status":"complete","summary":"tests pass"}`}},
		[]fantasy.ToolResultContent{{ToolCallID: "goal-2", ToolName: tools.GoalStatusToolName, Result: fantasy.ToolResultOutputContentError{Error: assertError("invalid")}}},
	)
	_, ok = terminalGoalStatus([]fantasy.StepResult{failed})
	require.False(t, ok)
}

func TestGoalNeedsContinuationOnlyForOutputLength(t *testing.T) {
	t.Parallel()

	require.False(t, goalNeedsContinuation(nil))
	require.False(t, goalNeedsContinuation([]fantasy.StepResult{{
		Response: fantasy.Response{FinishReason: fantasy.FinishReasonStop},
	}}))
	require.True(t, goalNeedsContinuation([]fantasy.StepResult{{
		Response: fantasy.Response{FinishReason: fantasy.FinishReasonLength},
	}}))
}

func TestPrepareGoalContinuationPreservesIntentAndObservesFailures(t *testing.T) {
	t.Parallel()

	failedStep := makeStep(
		[]fantasy.ToolCallContent{{ToolCallID: "bash-1", ToolName: "bash"}},
		[]fantasy.ToolResultContent{{ToolCallID: "bash-1", ToolName: "bash", Result: fantasy.ToolResultOutputContentText{Text: "command failed\nExit code 1"}}},
	)
	call := prepareGoalContinuation(SessionAgentCall{
		Prompt:         "fix the failing build",
		originalIntent: "fix the failing build",
		goalMode:       true,
	}, []fantasy.StepResult{failedStep})

	require.Equal(t, "fix the failing build", call.Prompt)
	require.True(t, call.skipUserMessage)
	require.Equal(t, 1, call.goalIteration)
	require.Contains(t, call.TransientContext, `"tool":"bash"`)
	require.Contains(t, call.TransientContext, "Exit code 1")
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
		TransientContext: goalModeContext(0, nil),
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(2), model.calls.Load())
	require.Equal(t, 0, agent.QueuedPrompts(session.ID))
	status, ok := terminalGoalStatus(result.Steps)
	require.True(t, ok)
	require.Equal(t, "complete", status)

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
