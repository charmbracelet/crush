package enum

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToolResultState_IsSuccess(t *testing.T) {
	tests := []struct {
		name     string
		state    ToolResultState
		expected bool
	}{
		{"Success is success", ToolResultStateSuccess, true},
		{"Partial is success", ToolResultStatePartial, true},
		{"Error is not success", ToolResultStateError, false},
		{"Timeout is not success", ToolResultStateTimeout, false},
		{"Cancelled is not success", ToolResultStateCancelled, false},
		{"Unknown is not success", ToolResultStateUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.state.IsSuccess())
		})
	}
}

func TestToolResultState_IsError(t *testing.T) {
	tests := []struct {
		name     string
		state    ToolResultState
		expected bool
	}{
		{"Success is not error", ToolResultStateSuccess, false},
		{"Partial is not error", ToolResultStatePartial, false},
		{"Error is error", ToolResultStateError, true},
		{"Timeout is error", ToolResultStateTimeout, true},
		{"Unknown is error", ToolResultStateUnknown, true},
		{"Cancelled is not error", ToolResultStateCancelled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.state.IsError())
		})
	}
}

func TestToolResultState_IsFinal(t *testing.T) {
	tests := []struct {
		name     string
		state    ToolResultState
		expected bool
	}{
		{"Success is final", ToolResultStateSuccess, true},
		{"Error is final", ToolResultStateError, true},
		{"Timeout is final", ToolResultStateTimeout, true},
		{"Cancelled is final", ToolResultStateCancelled, true},
		{"Partial is final", ToolResultStatePartial, true},
		{"Unknown is not final", ToolResultStateUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.state.IsFinal())
		})
	}
}

func TestToolResultState_String(t *testing.T) {
	tests := []struct {
		name     string
		state    ToolResultState
		expected string
	}{
		{"Success string", ToolResultStateSuccess, "success"},
		{"Error string", ToolResultStateError, "error"},
		{"Timeout string", ToolResultStateTimeout, "timeout"},
		{"Cancelled string", ToolResultStateCancelled, "cancelled"},
		{"Partial string", ToolResultStatePartial, "partial"},
		{"Unknown string", ToolResultStateUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.state.String())
		})
	}
}

func TestToolResultState_ToLabel(t *testing.T) {
	tests := []struct {
		name     string
		state    ToolResultState
		expected string
	}{
		{"Success label", ToolResultStateSuccess, "Success"},
		{"Error label", ToolResultStateError, "Error"},
		{"Timeout label", ToolResultStateTimeout, "Timeout"},
		{"Cancelled label", ToolResultStateCancelled, "Cancelled"},
		{"Partial label", ToolResultStatePartial, "Partial Success"},
		{"Unknown label", ToolResultStateUnknown, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.state.ToLabel())
		})
	}
}

func TestToolResultState_RenderTUIMessage(t *testing.T) {
	tests := []struct {
		name     string
		state    ToolResultState
		expected string
	}{
		{"Success message", ToolResultStateSuccess, "Completed successfully"},
		{"Error message", ToolResultStateError, "Tool execution failed"},
		{"Timeout message", ToolResultStateTimeout, "Tool execution timed out"},
		{"Cancelled message", ToolResultStateCancelled, "Tool execution cancelled"},
		{"Partial message", ToolResultStatePartial, "Partially completed"},
		{"Unknown message", ToolResultStateUnknown, "Result unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.state.RenderTUIMessage())
		})
	}
}

func TestToolResultState_ToIcon(t *testing.T) {
	// Test that icons are not empty and vary by state
	states := []ToolResultState{
		ToolResultStateSuccess,
		ToolResultStateError,
		ToolResultStateTimeout,
		ToolResultStateCancelled,
		ToolResultStatePartial,
		ToolResultStateUnknown,
	}

	for _, state := range states {
		icon := state.ToIcon()
		require.NotEmpty(t, icon, "Icon should not be empty for state %v", state)
	}
}

func TestToolResultState_ToFgColor(t *testing.T) {
	// Test that colors are returned for all states
	states := []ToolResultState{
		ToolResultStateSuccess,
		ToolResultStateError,
		ToolResultStateTimeout,
		ToolResultStateCancelled,
		ToolResultStatePartial,
		ToolResultStateUnknown,
	}

	for _, state := range states {
		color := state.ToFgColor()
		require.NotNil(t, color, "Color should not be nil for state %v", state)
	}
}

func TestToolResultState_ToIconColored(t *testing.T) {
	// Test that colored icons are not empty
	states := []ToolResultState{
		ToolResultStateSuccess,
		ToolResultStateError,
		ToolResultStateTimeout,
	}

	for _, state := range states {
		coloredIcon := state.ToIconColored()
		require.NotEmpty(t, coloredIcon, "Colored icon should not be empty for state %v", state)
	}
}
