package agent

import (
	"context"
	"strings"
	"sync"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// capturingStreamModel records the provider-level prompt text while
// behaving like finishStreamModel so a whole turn can complete.
type capturingStreamModel struct {
	finishStreamModel
	mu      sync.Mutex
	prompts []string
}

func (m *capturingStreamModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	m.prompts = append(m.prompts, promptText(call.Prompt))
	m.mu.Unlock()
	return m.finishStreamModel.Stream(ctx, call)
}

func (m *capturingStreamModel) capturedPrompts() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.prompts...)
}

// promptText flattens every text part of a composed provider prompt.
func promptText(p fantasy.Prompt) string {
	var sb strings.Builder
	for _, msg := range p {
		for _, part := range msg.Content {
			if text, ok := part.(fantasy.TextPart); ok {
				sb.WriteString(text.Text)
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}

func promptHookRunner(t *testing.T, command string) *hooks.Runner {
	t.Helper()
	return hooks.NewRunner([]config.HookConfig{{Command: command}}, t.TempDir(), t.TempDir())
}

// seedUserMessage records an earlier user message so Run does not kick off
// title generation, keeping the captured provider prompt deterministic.
func seedUserMessage(t *testing.T, env fakeEnv, sessionID string) {
	t.Helper()
	_, err := env.messages.Create(t.Context(), sessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "earlier question"}},
	})
	require.NoError(t, err)
}

func TestPromptHookContextIsPrependedToTheOutboundPrompt(t *testing.T) {
	t.Parallel()

	const marker = "<retrieved-context>the build uses gofumpt</retrieved-context>"

	env := testEnv(t)
	model := &capturingStreamModel{finishStreamModel: finishStreamModel{text: "done"}}
	sa := testSessionAgent(env, model, model, "system").(*sessionAgent)
	sa.promptHookRunner = promptHookRunner(t, `echo '{"context":"`+marker+`"}'`)

	sess, err := env.sessions.Create(t.Context(), "test")
	require.NoError(t, err)
	seedUserMessage(t, env, sess.ID)

	const prompt = "how should I format Go code?"
	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: prompt})
	require.NoError(t, err)

	prompts := model.capturedPrompts()
	require.Len(t, prompts, 1)
	require.Contains(t, prompts[0], marker)
	require.Contains(t, prompts[0], prompt)
	require.Less(
		t,
		strings.Index(prompts[0], marker),
		strings.Index(prompts[0], prompt),
		"hook context must be prepended before the user prompt",
	)

	// The stored transcript keeps the original prompt; only the outbound
	// copy the model sees carries the injected context.
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var storedUserTexts []string
	for _, m := range msgs {
		if m.Role == message.User {
			storedUserTexts = append(storedUserTexts, m.Content().Text)
		}
	}
	require.Contains(t, storedUserTexts, prompt)
	require.NotContains(t, strings.Join(storedUserTexts, "\n"), "the build uses gofumpt")
}

func TestPromptHookWithoutContextLeavesPromptUnchanged(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	model := &capturingStreamModel{finishStreamModel: finishStreamModel{text: "done"}}
	sa := testSessionAgent(env, model, model, "system").(*sessionAgent)
	sa.promptHookRunner = promptHookRunner(t, `echo '{}'`)

	sess, err := env.sessions.Create(t.Context(), "test")
	require.NoError(t, err)
	seedUserMessage(t, env, sess.ID)

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "plain prompt"})
	require.NoError(t, err)

	prompts := model.capturedPrompts()
	require.Len(t, prompts, 1)
	require.Contains(t, prompts[0], "plain prompt")
	require.NotContains(t, prompts[0], "retrieved-context")
}

func TestPromptWithoutHookRunnerIsUnchanged(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	model := &capturingStreamModel{finishStreamModel: finishStreamModel{text: "done"}}
	sa := testSessionAgent(env, model, model, "system").(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "test")
	require.NoError(t, err)
	seedUserMessage(t, env, sess.ID)

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "plain prompt"})
	require.NoError(t, err)

	prompts := model.capturedPrompts()
	require.Len(t, prompts, 1)
	require.Contains(t, prompts[0], "plain prompt")
	require.NotContains(t, prompts[0], "retrieved-context")
}
