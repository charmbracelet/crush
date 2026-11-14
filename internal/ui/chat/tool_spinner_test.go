package chat

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// A tool call with a result is not still running, whether or not it was ever
// marked finished.
func TestToolItemStopsSpinningOnceItHasAResult(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	result := &message.ToolResult{
		ToolCallID: "tc1",
		Name:       "write",
		Content:    "the model ran out of output tokens",
		IsError:    true,
	}

	tests := []struct {
		name     string
		toolCall message.ToolCall
		result   *message.ToolResult
		want     bool
	}{
		{
			name:     "running call with no result spins",
			toolCall: message.ToolCall{ID: "tc1", Name: "write", Input: "{}", Finished: false},
			want:     true,
		},
		{
			name:     "unfinished call with a result does not spin",
			toolCall: message.ToolCall{ID: "tc1", Name: "write", Input: "{}", Finished: false},
			result:   result,
			want:     false,
		},
		{
			name:     "finished call with a result does not spin",
			toolCall: message.ToolCall{ID: "tc1", Name: "write", Input: "{}", Finished: true},
			result:   result,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			item := NewToolMessageItem(&sty, "msg", tt.toolCall, tt.result, false, "")
			spinner, ok := item.(interface{ isSpinning() bool })
			require.True(t, ok, "tool items must expose their spinning state")
			require.Equal(t, tt.want, spinner.isSpinning())
		})
	}
}
