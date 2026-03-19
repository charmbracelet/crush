package agent

import (
	"context"
	"errors"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/plugin"
	"github.com/stretchr/testify/require"
)

type chatHookTestPlugin struct {
	name  string
	hooks plugin.Hooks
}

func (p *chatHookTestPlugin) Name() string { return p.name }

func (p *chatHookTestPlugin) Init(context.Context, plugin.PluginInput) (plugin.Hooks, error) {
	return p.hooks, nil
}

type chatHookTestAgent struct {
	t            *testing.T
	responseText string
	streamErr    error
	callCount    int
	lastCall     fantasy.AgentStreamCall
	prepared     []fantasy.Message
}

func (a *chatHookTestAgent) Generate(context.Context, fantasy.AgentCall) (*fantasy.AgentResult, error) {
	return nil, nil
}

func (a *chatHookTestAgent) Stream(ctx context.Context, call fantasy.AgentStreamCall) (*fantasy.AgentResult, error) {
	a.callCount++
	a.lastCall = call

	if call.PrepareStep != nil {
		_, prepared, err := call.PrepareStep(ctx, fantasy.PrepareStepFunctionOptions{Messages: call.Messages})
		require.NoError(a.t, err)
		a.prepared = prepared.Messages
	}

	if a.streamErr != nil {
		return nil, a.streamErr
	}

	if call.OnTextDelta != nil {
		require.NoError(a.t, call.OnTextDelta("text-1", a.responseText))
	}
	if call.OnStepFinish != nil {
		require.NoError(a.t, call.OnStepFinish(fantasy.StepResult{
			Response: fantasy.Response{FinishReason: fantasy.FinishReasonStop},
		}))
	}

	return &fantasy.AgentResult{}, nil
}

func createAgentWithHooksForTest(env fakeEnv, fakeAgent fantasy.Agent) SessionAgent {
	model := Model{
		CatwalkCfg: catwalk.Model{
			ContextWindow:    200000,
			DefaultMaxTokens: 1000,
		},
		ModelCfg: config.SelectedModel{
			Model:    "claude-sonnet-4",
			Provider: "anthropic",
		},
	}

	return NewSessionAgent(SessionAgentOptions{
		LargeModel:   model,
		SmallModel:   model,
		SystemPrompt: "",
		IsYolo:       true,
		Sessions:     env.sessions,
		Messages:     env.messages,
		AgentFactory: func(model fantasy.LanguageModel, opts ...fantasy.AgentOption) fantasy.Agent {
			return fakeAgent
		},
	})
}

func createSeedHistoryMessage(t *testing.T, env fakeEnv, sessionID string) {
	_, err := env.messages.Create(t.Context(), sessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "seed history"}},
	})
	require.NoError(t, err)
}

func TestSessionAgentRunTriggersChatHooks(t *testing.T) {
	plugin.Reset()
	t.Cleanup(plugin.Reset)

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "chat hooks")
	require.NoError(t, err)
	createSeedHistoryMessage(t, env, testSession.ID)

	var beforeInputs []plugin.ChatBeforeRequestInput
	var afterInputs []plugin.ChatAfterResponseInput
	plugin.Register(&chatHookTestPlugin{
		name: "chat-hooks-plugin",
		hooks: plugin.Hooks{
			ChatBeforeRequest: func(ctx context.Context, input plugin.ChatBeforeRequestInput) error {
				beforeInputs = append(beforeInputs, input)
				return nil
			},
			ChatAfterResponse: func(ctx context.Context, input plugin.ChatAfterResponseInput) error {
				afterInputs = append(afterInputs, input)
				return nil
			},
		},
	})
	err = plugin.Init(context.Background(), plugin.PluginInput{WorkingDir: env.workingDir})
	require.NoError(t, err)

	fakeAgent := &chatHookTestAgent{t: t, responseText: "ok"}
	sessionAgent := createAgentWithHooksForTest(env, fakeAgent)

	result, err := sessionAgent.Run(t.Context(), SessionAgentCall{
		Prompt:          "hello",
		SessionID:       testSession.ID,
		MaxOutputTokens: 1000,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, fakeAgent.callCount)
	require.Len(t, beforeInputs, 1)
	require.Len(t, afterInputs, 1)

	before := beforeInputs[0]
	require.Equal(t, testSession.ID, before.SessionID)
	require.Equal(t, "session", before.Agent)
	require.Equal(t, "anthropic", before.Model.ProviderID)
	require.Equal(t, "claude-sonnet-4", before.Model.ModelID)
	require.Equal(t, "hello", before.Message.Content().Text)

	after := afterInputs[0]
	require.Equal(t, testSession.ID, after.SessionID)
	require.Equal(t, "session", after.Agent)
	require.Equal(t, "anthropic", after.Model.ProviderID)
	require.Equal(t, "claude-sonnet-4", after.Model.ModelID)
	require.NotNil(t, after.Result)
	require.NoError(t, after.Error)
}

func TestSessionAgentRunReturnsErrorWhenChatBeforeHookFails(t *testing.T) {
	plugin.Reset()
	t.Cleanup(plugin.Reset)

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "chat before fails")
	require.NoError(t, err)
	createSeedHistoryMessage(t, env, testSession.ID)

	hookErr := errors.New("chat before failed")
	plugin.Register(&chatHookTestPlugin{
		name: "chat-before-failure-plugin",
		hooks: plugin.Hooks{
			ChatBeforeRequest: func(ctx context.Context, input plugin.ChatBeforeRequestInput) error {
				return hookErr
			},
		},
	})
	err = plugin.Init(context.Background(), plugin.PluginInput{WorkingDir: env.workingDir})
	require.NoError(t, err)

	fakeAgent := &chatHookTestAgent{t: t, responseText: "ok"}
	sessionAgent := createAgentWithHooksForTest(env, fakeAgent)

	result, err := sessionAgent.Run(t.Context(), SessionAgentCall{
		Prompt:          "hello",
		SessionID:       testSession.ID,
		MaxOutputTokens: 1000,
	})
	require.ErrorIs(t, err, hookErr)
	require.Nil(t, result)
	require.Equal(t, 0, fakeAgent.callCount)
}

func TestSessionAgentRunReturnsErrorWhenChatAfterHookFails(t *testing.T) {
	plugin.Reset()
	t.Cleanup(plugin.Reset)

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "chat after fails")
	require.NoError(t, err)
	createSeedHistoryMessage(t, env, testSession.ID)

	hookErr := errors.New("chat after failed")
	plugin.Register(&chatHookTestPlugin{
		name: "chat-after-failure-plugin",
		hooks: plugin.Hooks{
			ChatAfterResponse: func(ctx context.Context, input plugin.ChatAfterResponseInput) error {
				return hookErr
			},
		},
	})
	err = plugin.Init(context.Background(), plugin.PluginInput{WorkingDir: env.workingDir})
	require.NoError(t, err)

	fakeAgent := &chatHookTestAgent{t: t, responseText: "ok"}
	sessionAgent := createAgentWithHooksForTest(env, fakeAgent)

	result, err := sessionAgent.Run(t.Context(), SessionAgentCall{
		Prompt:          "hello",
		SessionID:       testSession.ID,
		MaxOutputTokens: 1000,
	})
	require.ErrorIs(t, err, hookErr)
	require.Nil(t, result)
	require.Equal(t, 1, fakeAgent.callCount)
}

func TestSessionAgentRunReturnsCombinedErrorWhenStreamAndChatAfterHookFail(t *testing.T) {
	plugin.Reset()
	t.Cleanup(plugin.Reset)

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "chat after and stream fail")
	require.NoError(t, err)
	createSeedHistoryMessage(t, env, testSession.ID)

	hookErr := errors.New("chat after failed")
	streamErr := errors.New("stream failed")
	plugin.Register(&chatHookTestPlugin{
		name: "chat-after-combined-failure-plugin",
		hooks: plugin.Hooks{
			ChatAfterResponse: func(ctx context.Context, input plugin.ChatAfterResponseInput) error {
				return hookErr
			},
		},
	})
	err = plugin.Init(context.Background(), plugin.PluginInput{WorkingDir: env.workingDir})
	require.NoError(t, err)

	fakeAgent := &chatHookTestAgent{t: t, streamErr: streamErr}
	sessionAgent := createAgentWithHooksForTest(env, fakeAgent)

	result, err := sessionAgent.Run(t.Context(), SessionAgentCall{
		Prompt:          "hello",
		SessionID:       testSession.ID,
		MaxOutputTokens: 1000,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "stream error")
	require.ErrorContains(t, err, "hook error")
	require.ErrorIs(t, err, streamErr)
	require.ErrorIs(t, err, hookErr)
	require.Nil(t, result)
	require.Equal(t, 1, fakeAgent.callCount)
}

func TestSessionAgentRunAppliesChatTransforms(t *testing.T) {
	plugin.Reset()
	t.Cleanup(plugin.Reset)

	env := testEnv(t)
	testSession, err := env.sessions.Create(t.Context(), "chat transforms")
	require.NoError(t, err)
	createSeedHistoryMessage(t, env, testSession.ID)

	plugin.Register(&chatHookTestPlugin{
		name: "chat-transforms-plugin",
		hooks: plugin.Hooks{
			ChatMessagesTransform: func(ctx context.Context, input plugin.ChatMessagesTransformInput, output *plugin.ChatMessagesTransformOutput) error {
				output.Messages = append(output.Messages, message.Message{
					Role:  message.User,
					Parts: []message.ContentPart{message.TextContent{Text: "plugin compacted history"}},
				})
				return nil
			},
			ChatSystemTransform: func(ctx context.Context, input plugin.ChatSystemTransformInput, output *plugin.ChatSystemTransformOutput) error {
				output.Prefix = "plugin prefix"
				return nil
			},
		},
	})
	err = plugin.Init(context.Background(), plugin.PluginInput{WorkingDir: env.workingDir})
	require.NoError(t, err)

	fakeAgent := &chatHookTestAgent{t: t, responseText: "ok"}
	sessionAgent := createAgentWithHooksForTest(env, fakeAgent)

	result, err := sessionAgent.Run(t.Context(), SessionAgentCall{
		Prompt:          "hello",
		SessionID:       testSession.ID,
		MaxOutputTokens: 1000,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	foundTransformedMessage := false
	for _, msg := range fakeAgent.lastCall.Messages {
		for _, part := range msg.Content {
			if textPart, ok := part.(fantasy.TextPart); ok && textPart.Text == "plugin compacted history" {
				foundTransformedMessage = true
			}
		}
	}
	require.True(t, foundTransformedMessage)
	require.NotEmpty(t, fakeAgent.prepared)
	first := fakeAgent.prepared[0]
	require.Equal(t, fantasy.MessageRoleSystem, first.Role)
	require.Len(t, first.Content, 1)
	textPart, ok := first.Content[0].(fantasy.TextPart)
	require.True(t, ok)
	require.Equal(t, "plugin prefix", textPart.Text)
}
