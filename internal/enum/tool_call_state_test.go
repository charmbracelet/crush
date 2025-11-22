package enum

import (
	"testing"
)

func TestShouldShowContentForState(t *testing.T) {
	tests := []struct {
		name      string
		state     ToolCallState
		isNested  bool
		hasNested bool
		want      bool
	}{
		// Pending state: only show for top-level tools with nested calls
		{
			name:      "Pending top-level with no children",
			state:     ToolCallStatePending,
			isNested:  false,
			hasNested: false,
			want:      false,
		},
		{
			name:      "Pending top-level with children",
			state:     ToolCallStatePending,
			isNested:  false,
			hasNested: true,
			want:      true, // Show header to provide context for nested tools
		},
		{
			name:      "Nested pending with no children",
			state:     ToolCallStatePending,
			isNested:  true,
			hasNested: false,
			want:      false,
		},
		{
			name:      "Nested pending with children",
			state:     ToolCallStatePending,
			isNested:  true,
			hasNested: true,
			want:      false,
		},

		// Running state: always show
		{
			name:      "Running top-level",
			state:     ToolCallStateRunning,
			isNested:  false,
			hasNested: false,
			want:      true,
		},
		{
			name:      "Running nested",
			state:     ToolCallStateRunning,
			isNested:  true,
			hasNested: false,
			want:      true,
		},

		// Completed state: always show
		{
			name:      "Completed top-level",
			state:     ToolCallStateCompleted,
			isNested:  false,
			hasNested: false,
			want:      true,
		},

		// Failed state: always show
		{
			name:      "Failed top-level",
			state:     ToolCallStateFailed,
			isNested:  false,
			hasNested: false,
			want:      true,
		},

		// Cancelled state: always show
		{
			name:      "Cancelled top-level",
			state:     ToolCallStateCancelled,
			isNested:  false,
			hasNested: false,
			want:      true,
		},

		// Permission states
		{
			name:      "Permission Pending",
			state:     ToolCallStatePermissionPending,
			isNested:  false,
			hasNested: false,
			want:      true,
		},
		{
			name:      "Permission Approved",
			state:     ToolCallStatePermissionApproved,
			isNested:  false,
			hasNested: false,
			want:      true,
		},
		{
			name:      "Permission Denied",
			state:     ToolCallStatePermissionDenied,
			isNested:  false,
			hasNested: false,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.state.ShouldShowContentForState(tt.isNested, tt.hasNested)
			if got != tt.want {
				t.Errorf("ShouldShowContentForState() = %v, want %v", got, tt.want)
			}
		})
	}
}
