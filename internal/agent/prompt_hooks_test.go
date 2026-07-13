package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

type capturingStreamModel struct {
	text string

	mu      sync.Mutex
	prompts [][]fantasy.Message
}

func TestCapabilityAwareHookContext(t *testing.T) {
	t.Parallel()

	t.Run("keeps available MCP reference unchanged", func(t *testing.T) {
		contextText := "Use mcp_context7_resolve-library-id before editing."
		require.Equal(t, contextText, capabilityAwareHookContext(contextText, []string{"mcp_context7_resolve-library-id"}))
	})

	t.Run("warns and forbids shell fallback for unavailable MCP", func(t *testing.T) {
		got := capabilityAwareHookContext(
			"Use mcp_sequential-thinking_sequentialthinking before editing.",
			[]string{"bash", "view"},
		)
		require.Contains(t, got, "Referenced tools are unavailable")
		require.Contains(t, got, "mcp_sequential-thinking_sequentialthinking")
		require.Contains(t, got, "Do not invoke these names through Bash")
	})
}

func (m *capturingStreamModel) Provider() string { return "fake" }
func (m *capturingStreamModel) Model() string    { return "fake-model" }

func (m *capturingStreamModel) record(call fantasy.Call) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prompts = append(m.prompts, cloneFantasyMessages([]fantasy.Message(call.Prompt)))
}

func (m *capturingStreamModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	m.record(call)
	return &fantasy.Response{
		Content:      fantasy.ResponseContent{fantasy.TextContent{Text: m.text}},
		FinishReason: fantasy.FinishReasonStop,
	}, nil
}

func (m *capturingStreamModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.record(call)
	text := m.text
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: text}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

func (m *capturingStreamModel) GenerateObject(ctx context.Context, call fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *capturingStreamModel) StreamObject(ctx context.Context, call fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *capturingStreamModel) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.prompts)
}

func (m *capturingStreamModel) allPromptText() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fantasyMessagesText(m.prompts...)
}

func (m *capturingStreamModel) promptsSnapshot() [][]fantasy.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([][]fantasy.Message, len(m.prompts))
	for i, prompt := range m.prompts {
		result[i] = cloneFantasyMessages(prompt)
	}
	return result
}

func newUserPromptRunner(t *testing.T, cmd string) *hooks.Runner {
	t.Helper()
	cfg := &config.Config{
		Hooks: map[string][]config.HookConfig{
			hooks.EventUserPromptSubmit: {{Command: cmd}},
		},
	}
	require.NoError(t, cfg.ValidateHooks())
	return hooks.NewRunner(cfg.Hooks[hooks.EventUserPromptSubmit], t.TempDir(), t.TempDir())
}

func newPromptHookTestAgent(t *testing.T, runner *hooks.Runner) (*sessionAgent, fakeEnv, *capturingStreamModel) {
	t.Helper()
	env := testEnv(t)
	model := &capturingStreamModel{text: "done"}
	sa := testSessionAgent(env, model, model, "system").(*sessionAgent)
	sa.userPromptHooks = runner
	return sa, env, model
}

func fantasyMessagesText(groups ...[]fantasy.Message) string {
	var sb strings.Builder
	for _, messages := range groups {
		for _, msg := range messages {
			for _, part := range msg.Content {
				if textPart, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok {
					sb.WriteString(textPart.Text)
					sb.WriteString("\n")
				}
			}
		}
	}
	return sb.String()
}

func userMessages(t *testing.T, env fakeEnv, sessionID string) []message.Message {
	t.Helper()
	msgs, err := env.messages.List(t.Context(), sessionID)
	require.NoError(t, err)
	var users []message.Message
	for _, msg := range msgs {
		if msg.Role == message.User {
			users = append(users, msg)
		}
	}
	return users
}

func TestRun_UserPromptSubmitDenyDoesNotPersistOrCallModel(t *testing.T) {
	t.Parallel()
	runner := newUserPromptRunner(t, `printf '%s\n' '{"decision":"deny","reason":"blocked secret"}'`)
	sa, env, model := newPromptHookTestAgent(t, runner)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	result, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "secret prompt"})
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "blocked secret")
	require.Empty(t, userMessages(t, env, sess.ID))
	require.Equal(t, 0, model.callCount(), "denied prompts must not reach title generation or streaming")
}

func TestRun_UserPromptSubmitUpdatedPromptIsPersisted(t *testing.T) {
	t.Parallel()
	runner := newUserPromptRunner(t, `printf '%s\n' '{"updated_prompt":"safe prompt","context":"transient policy context"}'`)
	sa, env, model := newPromptHookTestAgent(t, runner)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	result, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "secret prompt"})
	require.NoError(t, err)
	require.NotNil(t, result)

	users := userMessages(t, env, sess.ID)
	require.Len(t, users, 1)
	require.Equal(t, "safe prompt", users[0].Content().String())
	require.NotContains(t, users[0].Content().String(), "secret prompt")
	require.NotContains(t, users[0].Content().String(), "transient policy context")

	allText := model.allPromptText()
	require.NotContains(t, allText, "secret prompt")
	require.Contains(t, allText, "safe prompt")
	require.Contains(t, allText, "transient policy context")
}

func TestRun_UserPromptSubmitAppliesToFoldedQueuedPrompts(t *testing.T) {
	t.Parallel()
	runner := newUserPromptRunner(t, `payload=$(cat); case "$payload" in *queued-secret*) printf '%s\n' '{"updated_prompt":"queued-safe","context":"queued transient context"}' ;; *) printf '%s\n' '{}' ;; esac`)
	sa, env, model := newPromptHookTestAgent(t, runner)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	sa.enqueueCall(SessionAgentCall{SessionID: sess.ID, Prompt: "queued-secret"})

	result, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "main prompt"})
	require.NoError(t, err)
	require.NotNil(t, result)

	users := userMessages(t, env, sess.ID)
	require.Len(t, users, 2)
	require.Equal(t, "main prompt", users[0].Content().String())
	require.Equal(t, "queued-safe", users[1].Content().String())

	allText := model.allPromptText()
	require.NotContains(t, allText, "queued-secret")
	require.Contains(t, allText, "queued-safe")
	require.Contains(t, allText, "queued transient context")
}

func TestRun_UserPromptSubmitDenyBlocksFoldedQueuedPromptBeforePersistence(t *testing.T) {
	t.Parallel()
	runner := newUserPromptRunner(t, `payload=$(cat); case "$payload" in *queued-secret*) printf '%s\n' '{"decision":"deny","reason":"blocked queued"}' ;; *) printf '%s\n' '{}' ;; esac`)
	sa, env, _ := newPromptHookTestAgent(t, runner)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	sa.enqueueCall(SessionAgentCall{SessionID: sess.ID, Prompt: "queued-secret"})

	result, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "main prompt"})
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "blocked queued")

	users := userMessages(t, env, sess.ID)
	require.Len(t, users, 1)
	require.Equal(t, "main prompt", users[0].Content().String())
}

func TestRun_SystemPromptPrefixMergesIntoSingleSystemMessage(t *testing.T) {
	t.Parallel()
	sa, env, model := newPromptHookTestAgent(t, nil)
	sa.largePromptPrefix.Set("provider prefix")

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	result, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hello"})
	require.NoError(t, err)
	require.NotNil(t, result)

	var prompt []fantasy.Message
	for _, candidate := range model.promptsSnapshot() {
		if strings.Contains(fantasyMessagesText(candidate), "provider prefix") {
			prompt = candidate
			break
		}
	}
	require.NotEmpty(t, prompt)
	require.Equal(t, fantasy.MessageRoleSystem, prompt[0].Role)

	var systemMessages []fantasy.Message
	for _, msg := range prompt {
		if msg.Role == fantasy.MessageRoleSystem {
			systemMessages = append(systemMessages, msg)
		}
	}
	require.Len(t, systemMessages, 1)

	systemText := fantasyMessagesText(systemMessages)
	require.Contains(t, systemText, "provider prefix")
	require.Contains(t, systemText, "system")
}
