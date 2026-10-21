package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

type scriptedSummaryModel struct {
	calls func(context.Context, fantasy.Call) (fantasy.StreamResponse, error)
}

func (model *scriptedSummaryModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("unexpected generate")
}

func (model *scriptedSummaryModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	return model.calls(ctx, call)
}

func (model *scriptedSummaryModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("unexpected object generation")
}

func (model *scriptedSummaryModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("unexpected object stream")
}

func (model *scriptedSummaryModel) Provider() string { return "scripted" }
func (model *scriptedSummaryModel) Model() string    { return "scripted" }

func scriptedText(text string) fantasy.StreamResponse {
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "text"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "text", Delta: text}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "text"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop, Usage: fantasy.Usage{InputTokens: 20, OutputTokens: 10}})
	}
}

func TestSummaryRecoveryFailureModes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		summary  string
		disabled bool
		cancel   bool
		repeat   bool
	}{
		{name: "disabled", disabled: true},
		{name: "empty summary"},
		{name: "repeat overflow", summary: "Progress retained", repeat: true},
		{name: "cancel summary", cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := testEnv(t)
			session, err := env.sessions.Create(t.Context(), "existing")
			require.NoError(t, err)
			_, err = env.messages.Create(t.Context(), session.ID, message.CreateMessageParams{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "prior work"}}})
			require.NoError(t, err)
			attempts := 0
			model := &scriptedSummaryModel{calls: func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
				attempts++
				if attempts == 2 && tc.cancel {
					<-ctx.Done()
					return nil, ctx.Err()
				}
				if attempts == 2 {
					return scriptedText(tc.summary), nil
				}
				return nil, &fantasy.ProviderError{StatusCode: 400, Message: "Your input exceeds the context window of this model."}
			}}
			agent := testSessionAgent(env, model, model, "system").(*sessionAgent)
			agent.disableAutoSummarize = tc.disabled
			ctx := t.Context()
			if tc.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				defer cancel()
				model.calls = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
					attempts++
					if attempts == 2 {
						cancel()
						return nil, ctx.Err()
					}
					return nil, &fantasy.ProviderError{StatusCode: 400, Message: "Your input exceeds the context window of this model."}
				}
			}
			_, err = agent.Run(ctx, SessionAgentCall{SessionID: session.ID, Prompt: "task"})
			require.Error(t, err)
			if tc.cancel {
				require.ErrorIs(t, err, context.Canceled)
			}
			if tc.disabled {
				require.Equal(t, 1, attempts)
			} else if !tc.cancel {
				require.Equal(t, 2+boolToInt(tc.repeat), attempts)
			}
			stored, listErr := env.messages.List(t.Context(), session.ID)
			require.NoError(t, listErr)
			for _, item := range stored {
				require.False(t, item.IsSummaryMessage && strings.TrimSpace(item.Content().Text) == "")
			}
		})
	}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func TestSummaryRecoveryAfterCompletedTool(t *testing.T) {
	env := testEnv(t)
	session, err := env.sessions.Create(t.Context(), "existing")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), session.ID, message.CreateMessageParams{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "previous"}}})
	require.NoError(t, err)
	toolRuns := 0
	tool := fantasy.NewAgentTool("read_state", "Read state", func(_ context.Context, _ struct{}, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
		toolRuns++
		return fantasy.NewTextResponse("important state"), nil
	})
	attempts := 0
	model := &scriptedSummaryModel{calls: func(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
		attempts++
		switch attempts {
		case 1:
			return func(yield func(fantasy.StreamPart) bool) {
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: "call-1", ToolCallName: "read_state", ToolCallInput: "{}"}) {
					return
				}
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls, Usage: fantasy.Usage{InputTokens: 20, OutputTokens: 10}})
			}, nil
		case 2:
			return nil, &fantasy.ProviderError{StatusCode: 400, Message: "Your input exceeds the context window of this model."}
		case 3:
			return scriptedText("The state was read; finish the task."), nil
		case 4:
			return scriptedText("Finished"), nil
		default:
			return nil, errors.New("unexpected provider call")
		}
	}}
	agent := testSessionAgent(env, model, model, "system", tool)
	_, err = agent.Run(t.Context(), SessionAgentCall{SessionID: session.ID, Prompt: "inspect"})
	require.NoError(t, err)
	require.Equal(t, 4, attempts)
	require.Equal(t, 1, toolRuns)
	stored, err := env.messages.List(t.Context(), session.ID)
	require.NoError(t, err)
	count := 0
	for _, item := range stored {
		if item.Role == message.User && item.Content().Text == "inspect" {
			count++
		}
	}
	require.Equal(t, 1, count)
}

func TestQueuedCallsRetainOrderAfterFailure(t *testing.T) {
	env := testEnv(t)
	session, err := env.sessions.Create(t.Context(), "existing")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), session.ID, message.CreateMessageParams{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "prior"}}})
	require.NoError(t, err)
	firstAttempt := true
	model := &scriptedSummaryModel{calls: func(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
		if firstAttempt {
			firstAttempt = false
			return nil, errors.New("temporary failure")
		}
		return scriptedText("done"), nil
	}}
	agent := testSessionAgent(env, model, model, "system").(*sessionAgent)
	_, err = agent.Run(t.Context(), SessionAgentCall{SessionID: session.ID, Prompt: "first"})
	require.Error(t, err)
	agent.messageQueue.Set(session.ID, []SessionAgentCall{{SessionID: session.ID, Prompt: "queued"}})
	_, err = agent.Run(t.Context(), SessionAgentCall{SessionID: session.ID, Prompt: "later"})
	require.NoError(t, err)
	stored, err := env.messages.List(t.Context(), session.ID)
	require.NoError(t, err)
	var order []string
	for _, item := range stored {
		if item.Role == message.User {
			order = append(order, item.Content().Text)
		}
	}
	require.Equal(t, []string{"prior", "first", "later", "queued"}, order)
}

func TestSummaryRecoveryPreflight(t *testing.T) {
	env := testEnv(t)
	session, err := env.sessions.Create(t.Context(), "existing")
	require.NoError(t, err)
	for range 4 {
		_, err = env.messages.Create(t.Context(), session.ID, message.CreateMessageParams{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: strings.Repeat("history ", 360)}}})
		require.NoError(t, err)
	}
	attempts := 0
	model := &scriptedSummaryModel{calls: func(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
		attempts++
		if attempts < 5 {
			return scriptedText("Keep the earlier work."), nil
		}
		require.Equal(t, fantasy.NewUserMessage("Continue the previous task from the summary without repeating completed tool calls.").Content, call.Prompt[len(call.Prompt)-1].Content)
		return scriptedText("Done"), nil
	}}
	agent := testSessionAgent(env, model, model, "system").(*sessionAgent)
	large := agent.largeModel.Get()
	large.CatwalkCfg.ContextWindow = 6000
	large.CatwalkCfg.DefaultMaxTokens = 256
	agent.largeModel.Set(large)
	_, err = agent.Run(t.Context(), SessionAgentCall{SessionID: session.ID, Prompt: "continue", MaxOutputTokens: 256})
	require.NoError(t, err)
	require.Equal(t, 5, attempts)
	stored, err := env.messages.List(t.Context(), session.ID)
	require.NoError(t, err)
	count := 0
	for _, item := range stored {
		if item.Role == message.User && item.Content().Text == "continue" {
			count++
		}
	}
	require.Equal(t, 1, count)
}

func TestSummaryRecoveryFromProviderOverflow(t *testing.T) {
	env := testEnv(t)
	session, err := env.sessions.Create(t.Context(), "existing conversation")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), session.ID, message.CreateMessageParams{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "Earlier task details"}}})
	require.NoError(t, err)
	attempts := 0
	model := &scriptedSummaryModel{calls: func(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
		attempts++
		switch attempts {
		case 1:
			return nil, &fantasy.ProviderError{StatusCode: 400, Message: "Your input exceeds the context window of this model. Please adjust your input and try again."}
		case 2:
			return scriptedText("Earlier task details; continue new request."), nil
		case 3:
			require.Equal(t, fantasy.NewUserMessage("Continue the previous task from the summary without repeating completed tool calls.").Content, call.Prompt[len(call.Prompt)-1].Content)
			return scriptedText("Completed"), nil
		default:
			return nil, errors.New("unexpected request")
		}
	}}
	agent := testSessionAgent(env, model, model, "system")
	result, err := agent.Run(t.Context(), SessionAgentCall{SessionID: session.ID, Prompt: "new request"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 3, attempts)
	stored, err := env.messages.List(t.Context(), session.ID)
	require.NoError(t, err)
	userCount := 0
	summaryCount := 0
	for _, item := range stored {
		if item.Role == message.User && strings.Contains(item.Content().Text, "new request") {
			userCount++
		}
		if item.IsSummaryMessage {
			summaryCount++
		}
	}
	require.Equal(t, 1, userCount)
	require.Equal(t, 1, summaryCount)
}
