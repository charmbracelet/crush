package proto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestRunActivityProtoRoundTripOmitsResultContent(t *testing.T) {
	t.Parallel()

	original := notify.RunActivity{
		SessionID: "session",
		RunID:     "run",
		MessageID: "assistant",
		Type:      notify.RunActivityToolFinished,
		ToolCall: message.ToolCall{
			ID:       "call",
			Name:     "view",
			Input:    `{"file_path":"main.go"}`,
			Finished: true,
		},
		ToolResult: message.ToolResult{
			ToolCallID: "call",
			Name:       "view",
			Content:    "result body",
			Data:       "result data",
			Metadata:   "result metadata",
			IsError:    true,
		},
		Cancelled: true,
	}

	wire := RunActivityToProto(original)
	require.Empty(t, wire.ToolResult.Content)
	require.Empty(t, wire.ToolResult.Data)
	require.Empty(t, wire.ToolResult.Metadata)

	roundTrip := RunActivityFromProto(wire)
	original.ToolResult.Content = ""
	original.ToolResult.Data = ""
	original.ToolResult.Metadata = ""
	require.Equal(t, original, roundTrip)
}

func TestRunCompleteActivitiesJSON(t *testing.T) {
	t.Parallel()

	complete := RunComplete{
		SessionID: "session",
		RunID:     "run",
		Activities: []RunActivity{{
			SessionID:        "session",
			RunID:            "run",
			MessageID:        "assistant",
			Type:             RunActivityThinkingFinished,
			ThinkingDuration: 2 * time.Second,
		}},
	}
	encoded, err := json.Marshal(complete)
	require.NoError(t, err)
	var decoded RunComplete
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, complete, decoded)
}
