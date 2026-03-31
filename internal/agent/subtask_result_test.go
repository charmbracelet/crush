package agent

import (
	"context"
	"errors"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/stretchr/testify/require"
)

type subtaskSyntheticTestAgent struct {
	t         *testing.T
	streamErr error
}

func (a *subtaskSyntheticTestAgent) Generate(context.Context, fantasy.AgentCall) (*fantasy.AgentResult, error) {
	return nil, nil
}

func (a *subtaskSyntheticTestAgent) Stream(ctx context.Context, call fantasy.AgentStreamCall) (*fantasy.AgentResult, error) {
	_, _, err := call.PrepareStep(ctx, fantasy.PrepareStepFunctionOptions{Messages: call.Messages})
	require.NoError(a.t, err)

	require.NoError(a.t, call.OnToolCall(fantasy.ToolCallContent{
		ToolCallID: "call-agent-1",
		ToolName:   AgentToolName,
		Input:      `{"prompt":"delegate"}`,
	}))

	return nil, a.streamErr
}

func TestSessionAgentCreatesCanceledSubtaskMetadataForSyntheticToolResult(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "subtask synthetic")
	require.NoError(t, err)

	agent := NewSessionAgent(SessionAgentOptions{
		LargeModel: Model{
			CatwalkCfg: catwalk.Model{ContextWindow: 10000, DefaultMaxTokens: 1000},
			ModelCfg:   config.SelectedModel{Provider: "test-provider", Model: "test-model"},
		},
		SmallModel: Model{
			CatwalkCfg: catwalk.Model{ContextWindow: 10000, DefaultMaxTokens: 1000},
			ModelCfg:   config.SelectedModel{Provider: "test-provider", Model: "test-model"},
		},
		WorkingDir: env.workingDir,
		IsYolo:     true,
		Sessions:   env.sessions,
		Messages:   env.messages,
		AgentFactory: func(fantasy.LanguageModel, ...fantasy.AgentOption) fantasy.Agent {
			return &subtaskSyntheticTestAgent{t: t, streamErr: context.Canceled}
		},
	}).(*sessionAgent)

	_, err = agent.Run(t.Context(), SessionAgentCall{
		SessionID:       sess.ID,
		Prompt:          "trigger subtask",
		MaxOutputTokens: 100,
	})
	require.ErrorIs(t, err, context.Canceled)

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)

	var found message.ToolResult
	for _, msg := range msgs {
		if msg.Role != message.Tool {
			continue
		}
		for _, tr := range msg.ToolResults() {
			if tr.ToolCallID == "call-agent-1" {
				found = tr
			}
		}
	}

	require.Equal(t, "call-agent-1", found.ToolCallID)
	require.True(t, found.IsError)
	subtask, ok := found.SubtaskResult()
	require.True(t, ok)
	require.Equal(t, message.ToolResultSubtaskStatusCanceled, subtask.Status)
	require.Equal(t, "call-agent-1", subtask.ParentToolCallID)
}

func TestSyntheticSubtaskStatusForTool(t *testing.T) {
	t.Parallel()

	status, ok := syntheticSubtaskStatusForTool(AgentToolName, true, false)
	require.True(t, ok)
	require.Equal(t, message.ToolResultSubtaskStatusCanceled, status)

	status, ok = syntheticSubtaskStatusForTool(AgentToolName, false, false)
	require.True(t, ok)
	require.Equal(t, message.ToolResultSubtaskStatusFailed, status)

	status, ok = syntheticSubtaskStatusForTool("bash", true, false)
	require.False(t, ok)
	require.Empty(t, status)

	status, ok = syntheticSubtaskStatusForTool("agentic_fetch", false, true)
	require.True(t, ok)
	require.Equal(t, message.ToolResultSubtaskStatusCanceled, status)

	require.True(t, permission.IsPermissionError(permission.ErrorPermissionDenied))
	require.False(t, errors.Is(context.Canceled, permission.ErrorPermissionDenied))
}
