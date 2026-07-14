package agent

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

type protocolRepairModel struct {
	autoReviewStreamModel
	calls     atomic.Int64
	mu        sync.Mutex
	toolNames [][]string
}

func (m *protocolRepairModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	callNumber := m.calls.Add(1)
	names := make([]string, 0, len(call.Tools))
	for _, tool := range call.Tools {
		names = append(names, tool.GetName())
	}
	slices.Sort(names)
	m.mu.Lock()
	m.toolNames = append(m.toolNames, names)
	m.mu.Unlock()

	text := "recovered"
	if callNumber == 1 {
		text = `<tool_code>probe_status(action="status")</tool_code>`
	}
	return func(yield func(fantasy.StreamPart) bool) {
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "text"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "text", Delta: text})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "text"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

func (m *protocolRepairModel) toolsForCall(index int) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.toolNames[index])
}

type protocolProbeInput struct {
	Action string `json:"action"`
}

func protocolProbeTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		"probe_status",
		"Reports probe status.",
		func(context.Context, protocolProbeInput, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		},
	)
}

func TestAttemptedTextToolCall(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result *fantasy.AgentResult
		want   []string
	}{
		{
			name: "recognized tool markup",
			result: &fantasy.AgentResult{Steps: []fantasy.StepResult{{Response: fantasy.Response{
				FinishReason: fantasy.FinishReasonStop,
				Content: fantasy.ResponseContent{
					fantasy.TextContent{Text: `<tool_code>probe_status(action="status")</tool_code>`},
				},
			}}}},
			want: []string{"probe_status"},
		},
		{
			name: "ordinary tool discussion",
			result: &fantasy.AgentResult{Steps: []fantasy.StepResult{{Response: fantasy.Response{
				FinishReason: fantasy.FinishReasonStop,
				Content: fantasy.ResponseContent{
					fantasy.TextContent{Text: "The probe_status tool can check the service."},
				},
			}}}},
		},
		{
			name: "unknown marked tool",
			result: &fantasy.AgentResult{Steps: []fantasy.StepResult{{Response: fantasy.Response{
				FinishReason: fantasy.FinishReasonStop,
				Content: fantasy.ResponseContent{
					fantasy.TextContent{Text: "<tool_call>invented_tool()</tool_call>"},
				},
			}}}},
		},
		{
			name: "native tool call",
			result: &fantasy.AgentResult{Steps: []fantasy.StepResult{{Response: fantasy.Response{
				FinishReason: fantasy.FinishReasonToolCalls,
				Content: fantasy.ResponseContent{
					fantasy.ToolCallContent{ToolCallID: "call-1", ToolName: "probe_status", Input: `{"action":"status"}`},
				},
			}}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, attemptedTextToolNames(tt.result, []fantasy.AgentTool{protocolProbeTool()}))
		})
	}
}

func TestAttemptedTextToolNamesDoesNotMatchNameSuffix(t *testing.T) {
	t.Parallel()

	result := &fantasy.AgentResult{Steps: []fantasy.StepResult{{Response: fantasy.Response{
		FinishReason: fantasy.FinishReasonStop,
		Content: fantasy.ResponseContent{
			fantasy.TextContent{Text: "<tool_code>mcp_probe_status()</tool_code>"},
		},
	}}}}
	require.Nil(t, attemptedTextToolNames(result, []fantasy.AgentTool{protocolProbeTool()}))
}

func TestToolProtocolRepairPreservesFullToolSurface(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	model := &protocolRepairModel{}
	agent := newRecoveryTestAgent(env, model)
	agent.SetTools([]fantasy.AgentTool{
		protocolProbeTool(),
		fantasy.NewAgentTool(
			"view",
			"View a file.",
			func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
				return fantasy.NewTextResponse("ok"), nil
			},
		),
	})
	sess, err := env.sessions.Create(t.Context(), "protocol repair")
	require.NoError(t, err)

	result, err := agent.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "Check the probe.",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(2), model.calls.Load())
	require.Equal(t, []string{"probe_status", "view"}, model.toolsForCall(0))
	require.Equal(t, []string{"probe_status", "view"}, model.toolsForCall(1))
}
