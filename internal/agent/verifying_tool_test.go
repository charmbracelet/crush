package agent

import (
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestWrapToolsWithVerification(t *testing.T) {
	t.Parallel()

	inputs := []fantasy.AgentTool{
		&fakeTool{name: "edit"},
		&fakeTool{name: "write"},
		&fakeTool{name: "multiedit"},
		&fakeTool{name: "lsp_rename"},
		&fakeTool{name: "lsp_replace_symbol"},
		&fakeTool{name: "bash"},
		&fakeTool{name: "view"},
	}

	out := wrapToolsWithVerification(inputs, nil, t.TempDir(), nil)
	require.Len(t, out, len(inputs))
	for i, tool := range inputs {
		wrapped, isWrapped := out[i].(*verifyingTool)
		require.Equal(t, tools.WriteToolNames[tool.Info().Name], isWrapped,
			"tool %q wrap = %v, want %v", tool.Info().Name, isWrapped, tools.WriteToolNames[tool.Info().Name])
		if isWrapped && tool.Info().Name == "lsp_rename" {
			require.True(t, wrapped.projectWide, "rename refreshes all open files")
		}
	}
}

func TestVerifyingTool_UnverifiedWhenNoClientHandlesFile(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "edit", resp: fantasy.NewTextResponse("ok")}
	tool := &verifyingTool{inner: inner, lspManager: nil, workingDir: t.TempDir()}

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{
		ID:    "call-1",
		Name:  "edit",
		Input: `{"file_path":"/tmp/x.go","old_string":"a","new_string":"b"}`,
	})
	require.NoError(t, err)
	require.True(t, inner.called)

	var meta struct {
		Verification []message.VerificationCheck `json:"verification"`
	}
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.Len(t, meta.Verification, 1)
	require.Equal(t, "diagnostics", meta.Verification[0].Check)
	require.Equal(t, message.VerificationUnverified, meta.Verification[0].State)
}

func TestVerifyingTool_ErrorResponseSkipsVerification(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "edit", resp: fantasy.NewTextErrorResponse("edit failed")}
	tool := &verifyingTool{inner: inner, lspManager: nil, workingDir: t.TempDir()}

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{
		ID:    "call-1",
		Name:  "edit",
		Input: `{"file_path":"/tmp/x.go"}`,
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Empty(t, resp.Metadata, "failed mutation must not carry verification metadata")
}

func TestVerifyingTool_MissingPathPassesThrough(t *testing.T) {
	t.Parallel()

	inner := &fakeTool{name: "edit", resp: fantasy.NewTextResponse("ok")}
	tool := &verifyingTool{inner: inner, lspManager: nil, workingDir: t.TempDir()}

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{
		ID:    "call-1",
		Name:  "edit",
		Input: `{"old_string":"a"}`,
	})
	require.NoError(t, err)
	require.True(t, inner.called)
	require.Empty(t, resp.Metadata)
}

func TestMergeVerificationMetadata_ComposesWithHookKey(t *testing.T) {
	t.Parallel()

	existing := `{"hook":{"hook_count":1,"decision":"allow"}}`
	merged := mergeVerificationMetadata(existing, []message.VerificationCheck{{
		Check: "diagnostics",
		State: message.VerificationPassed,
	}})

	var meta struct {
		Hook         map[string]any              `json:"hook"`
		Verification []message.VerificationCheck `json:"verification"`
	}
	require.NoError(t, json.Unmarshal([]byte(merged), &meta))
	require.Equal(t, float64(1), meta.Hook["hook_count"], "hook key must survive the verification merge")
	require.Len(t, meta.Verification, 1)
	require.Equal(t, message.VerificationPassed, meta.Verification[0].State)
}

func TestMergeVerificationMetadata_EmptyExisting(t *testing.T) {
	t.Parallel()

	merged := mergeVerificationMetadata("", []message.VerificationCheck{{
		Check: "diagnostics",
		State: message.VerificationFailed,
	}})

	var meta struct {
		Verification []message.VerificationCheck `json:"verification"`
	}
	require.NoError(t, json.Unmarshal([]byte(merged), &meta))
	require.Len(t, meta.Verification, 1)
	require.Equal(t, message.VerificationFailed, meta.Verification[0].State)
}
