package agent

import (
	"context"
	"strings"
	"sync"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// recordingLanguageModel records every call it receives so tests can assert
// on the request the ephemeral path builds.
type recordingLanguageModel struct {
	mu    sync.Mutex
	calls []fantasy.Call
	reply string
}

func (m *recordingLanguageModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{}, nil
}

func (m *recordingLanguageModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	m.calls = append(m.calls, call)
	m.mu.Unlock()
	reply := m.reply
	return func(yield func(fantasy.StreamPart) bool) {
		if reply == "" {
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "0"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "0", Delta: reply})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "0"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

func (m *recordingLanguageModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return &fantasy.ObjectResponse{}, nil
}

func (m *recordingLanguageModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, nil
}

func (m *recordingLanguageModel) Provider() string { return "recording" }
func (m *recordingLanguageModel) Model() string    { return "recording-model" }

func (m *recordingLanguageModel) lastCall(t *testing.T) fantasy.Call {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	require.NotEmpty(t, m.calls, "the model must have been called")
	return m.calls[len(m.calls)-1]
}

// TestEphemeralAnswersWithSideQuestionPrompt pins the fix for side questions
// that came back empty: the ephemeral call used to send the coding-agent
// system prompt, which tells the model to go read files and run commands,
// while registering no tools. Models answered with a promise to look around
// (or with nothing at all) instead of answering.
func TestEphemeralAnswersWithSideQuestionPrompt(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	model := &recordingLanguageModel{reply: "You asked about the pooling option."}
	agent := testSessionAgent(env, model, model, "CODER PROMPT: use your tools to inspect the workspace")

	sess, err := env.sessions.Create(t.Context(), "New Session")
	require.NoError(t, err)

	res, err := agent.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "what was the pooling option?",
		Ephemeral: true,
	})
	require.NoError(t, err)
	require.Equal(t, "You asked about the pooling option.", subAgentOutput(res))

	call := model.lastCall(t)
	require.Empty(t, call.Tools, "a side question must not offer tools")

	require.NotEmpty(t, call.Prompt)
	system, ok := fantasy.AsMessagePart[fantasy.TextPart](call.Prompt[0].Content[0])
	require.True(t, ok, "the first message must be the system prompt")
	require.NotContains(t, system.Text, "CODER PROMPT")
	require.Contains(t, strings.ToLower(system.Text), "side question")
}

// TestEphemeralKeepsConversationHistory guards the reason side questions are
// useful: the model still sees the conversation it is being asked about.
func TestEphemeralKeepsConversationHistory(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	model := &recordingLanguageModel{reply: "answer"}
	agent := testSessionAgent(env, model, model, "system prompt")

	sess, err := env.sessions.Create(t.Context(), "New Session")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "we picked the pooled connection"}},
	})
	require.NoError(t, err)

	_, err = agent.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "what was the pooling option?",
		Ephemeral: true,
	})
	require.NoError(t, err)

	call := model.lastCall(t)
	var joined strings.Builder
	for _, msg := range call.Prompt {
		for _, part := range msg.Content {
			if text, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok {
				joined.WriteString(text.Text)
				joined.WriteString("\n")
			}
		}
	}
	require.Contains(t, joined.String(), "we picked the pooled connection")
	require.Contains(t, joined.String(), "what was the pooling option?")
}
