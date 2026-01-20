package trajectory

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestExportSession(t *testing.T) {
	t.Parallel()

	now := time.Now().Unix()

	sess := session.Session{
		ID:               "test-session-123",
		Title:            "Test Session",
		PromptTokens:     1000,
		CompletionTokens: 500,
		Cost:             0.05,
	}

	messages := []message.Message{
		{
			ID:        "msg-1",
			SessionID: sess.ID,
			Role:      message.User,
			Parts:     []message.ContentPart{message.TextContent{Text: "Hello, can you help me?"}},
			CreatedAt: now,
		},
		{
			ID:        "msg-2",
			SessionID: sess.ID,
			Role:      message.Assistant,
			Parts: []message.ContentPart{
				message.ReasoningContent{Thinking: "User is asking for help. I should respond helpfully."},
				message.TextContent{Text: "Of course! How can I assist you today?"},
			},
			Model:     "claude-sonnet-4-20250514",
			CreatedAt: now + 1,
		},
		{
			ID:        "msg-3",
			SessionID: sess.ID,
			Role:      message.User,
			Parts:     []message.ContentPart{message.TextContent{Text: "List files in the current directory"}},
			CreatedAt: now + 2,
		},
		{
			ID:        "msg-4",
			SessionID: sess.ID,
			Role:      message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: "I'll list the files for you."},
				message.ToolCall{
					ID:    "call-123",
					Name:  "ls",
					Input: `{"path": "."}`,
				},
			},
			Model:     "claude-sonnet-4-20250514",
			CreatedAt: now + 3,
		},
		{
			ID:        "msg-5",
			SessionID: sess.ID,
			Role:      message.Tool,
			Parts: []message.ContentPart{
				message.ToolResult{
					ToolCallID: "call-123",
					Name:       "ls",
					Content:    "file1.go\nfile2.go\nREADME.md",
				},
			},
			CreatedAt: now + 4,
		},
		{
			ID:        "msg-6",
			SessionID: sess.ID,
			Role:      message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: "Here are the files: file1.go, file2.go, README.md"},
			},
			Model:     "claude-sonnet-4-20250514",
			CreatedAt: now + 5,
		},
	}

	traj, err := ExportSession(sess, messages, "Crush", "1.0.0", "claude-sonnet-4-20250514")
	require.NoError(t, err)

	// Verify root structure.
	require.Equal(t, "ATIF-v1.4", traj.SchemaVersion)
	require.Equal(t, "test-session-123", traj.SessionID)
	require.Equal(t, "Crush", traj.Agent.Name)
	require.Equal(t, "1.0.0", traj.Agent.Version)
	require.Equal(t, "claude-sonnet-4-20250514", traj.Agent.ModelName)

	// Verify steps (tool results are attached to agent steps, not separate).
	require.Len(t, traj.Steps, 5)

	// Step 1: User message.
	require.Equal(t, 1, traj.Steps[0].StepID)
	require.Equal(t, "user", traj.Steps[0].Source)
	require.Equal(t, "Hello, can you help me?", traj.Steps[0].Message)
	require.Empty(t, traj.Steps[0].ToolCalls)
	require.Nil(t, traj.Steps[0].Observation)

	// Step 2: Assistant with reasoning.
	require.Equal(t, 2, traj.Steps[1].StepID)
	require.Equal(t, "agent", traj.Steps[1].Source)
	require.Equal(t, "Of course! How can I assist you today?", traj.Steps[1].Message)
	require.Equal(t, "User is asking for help. I should respond helpfully.", traj.Steps[1].ReasoningContent)

	// Step 3: User message.
	require.Equal(t, 3, traj.Steps[2].StepID)
	require.Equal(t, "user", traj.Steps[2].Source)

	// Step 4: Assistant with tool call AND observation (tool result attached).
	require.Equal(t, 4, traj.Steps[3].StepID)
	require.Equal(t, "agent", traj.Steps[3].Source)
	require.Len(t, traj.Steps[3].ToolCalls, 1)
	require.Equal(t, "call-123", traj.Steps[3].ToolCalls[0].ToolCallID)
	require.Equal(t, "ls", traj.Steps[3].ToolCalls[0].FunctionName)
	require.NotNil(t, traj.Steps[3].ToolCalls[0].Arguments)
	// Observation attached to the same agent step.
	require.NotNil(t, traj.Steps[3].Observation)
	require.Len(t, traj.Steps[3].Observation.Results, 1)
	require.Equal(t, "call-123", traj.Steps[3].Observation.Results[0].SourceCallID)
	require.Contains(t, traj.Steps[3].Observation.Results[0].Content, "file1.go")

	// Step 5: Final assistant response.
	require.Equal(t, 5, traj.Steps[4].StepID)
	require.Equal(t, "agent", traj.Steps[4].Source)
	require.Equal(t, "Here are the files: file1.go, file2.go, README.md", traj.Steps[4].Message)

	// Verify final metrics.
	require.NotNil(t, traj.FinalMetrics)
	require.Equal(t, int64(1000), traj.FinalMetrics.TotalPromptTokens)
	require.Equal(t, int64(500), traj.FinalMetrics.TotalCompletionTokens)
	require.Equal(t, 5, traj.FinalMetrics.TotalSteps)
	require.InDelta(t, 0.05, traj.FinalMetrics.TotalCostUSD, 0.001)

	// Verify timestamps are ISO 8601.
	for _, step := range traj.Steps {
		_, err := time.Parse(time.RFC3339, step.Timestamp)
		require.NoError(t, err, "step %d has invalid timestamp: %s", step.StepID, step.Timestamp)
	}

	// Verify JSON marshaling works.
	data, err := json.MarshalIndent(traj, "", "  ")
	require.NoError(t, err)
	require.Contains(t, string(data), `"schema_version": "ATIF-v1.4"`)
}

func TestExportSession_EmptyMessages(t *testing.T) {
	t.Parallel()

	sess := session.Session{
		ID:    "empty-session",
		Title: "Empty",
	}

	traj, err := ExportSession(sess, nil, "Crush", "1.0.0", "")
	require.NoError(t, err)
	require.Empty(t, traj.Steps)
	require.Nil(t, traj.FinalMetrics)
}

func TestExportSession_ToolCallArgumentsParsing(t *testing.T) {
	t.Parallel()

	sess := session.Session{ID: "tool-args-session"}
	now := time.Now().Unix()

	messages := []message.Message{
		{
			ID:        "msg-1",
			SessionID: sess.ID,
			Role:      message.Assistant,
			Parts: []message.ContentPart{
				message.ToolCall{
					ID:    "call-1",
					Name:  "edit",
					Input: `{"file_path": "/tmp/test.go", "old_string": "foo", "new_string": "bar"}`,
				},
			},
			CreatedAt: now,
		},
	}

	traj, err := ExportSession(sess, messages, "Crush", "1.0.0", "test-model")
	require.NoError(t, err)
	require.Len(t, traj.Steps, 1)
	require.Len(t, traj.Steps[0].ToolCalls, 1)

	// Arguments should be parsed as JSON object.
	args, ok := traj.Steps[0].ToolCalls[0].Arguments.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "/tmp/test.go", args["file_path"])
}

func TestExportSession_ToolError(t *testing.T) {
	t.Parallel()

	sess := session.Session{ID: "error-session"}
	now := time.Now().Unix()

	messages := []message.Message{
		{
			ID:        "msg-1",
			SessionID: sess.ID,
			Role:      message.Assistant,
			Parts: []message.ContentPart{
				message.ToolCall{
					ID:    "call-1",
					Name:  "bash",
					Input: `{"command": "foobar"}`,
				},
			},
			CreatedAt: now,
		},
		{
			ID:        "msg-2",
			SessionID: sess.ID,
			Role:      message.Tool,
			Parts: []message.ContentPart{
				message.ToolResult{
					ToolCallID: "call-1",
					Name:       "bash",
					Content:    "command not found: foobar",
					IsError:    true,
				},
			},
			CreatedAt: now + 1,
		},
	}

	traj, err := ExportSession(sess, messages, "Crush", "1.0.0", "")
	require.NoError(t, err)
	require.Len(t, traj.Steps, 1)
	require.Equal(t, "agent", traj.Steps[0].Source)
	require.NotNil(t, traj.Steps[0].Observation)
	require.Len(t, traj.Steps[0].Observation.Results, 1)
	require.Equal(t, "command not found: foobar", traj.Steps[0].Observation.Results[0].Content)
}
