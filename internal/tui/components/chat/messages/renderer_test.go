package messages

import (
	"testing"
	"github.com/charmbracelet/crush/internal/enum"
)

func TestShouldShowContentForStateLogic(t *testing.T) {
	// Test the integration of ShouldShowContentForState with renderWithParams
	tests := []struct {
		name           string
		state          enum.ToolCallState
		isNested       bool
		hasNestedTools  bool
		expectContent  bool
	}{
		{
			name:          "Pending top-level without children should not show content",
			state:         enum.ToolCallStatePending,
			isNested:      false,
			hasNestedTools: false,
			expectContent:  false,
		},
		{
			name:          "Pending top-level with children should show content",
			state:         enum.ToolCallStatePending,
			isNested:      false,
			hasNestedTools: true,
			expectContent:  true,
		},
		{
			name:          "Permission denied should not show content",
			state:         enum.ToolCallStatePermissionDenied,
			isNested:      false,
			hasNestedTools: false,
			expectContent:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the ShouldShowContentForState logic directly
			shouldShow := tt.state.ShouldShowContentForState(tt.isNested, tt.hasNestedTools)
			
			if shouldShow != tt.expectContent {
				t.Errorf("ShouldShowContentForState(%s, isNested=%v, hasNested=%v) = %v, want %v",
					tt.state, tt.isNested, tt.hasNestedTools, shouldShow, tt.expectContent)
			}
		})
	}
}

func TestStateVisibilityEdgeCases(t *testing.T) {
	// Test edge cases for state visibility logic
	
	t.Run("All states with nested=false hasNested=false", func(t *testing.T) {
		states := []enum.ToolCallState{
			enum.ToolCallStatePending,
			enum.ToolCallStateRunning,
			enum.ToolCallStateCompleted,
			enum.ToolCallStateFailed,
			enum.ToolCallStateCancelled,
			enum.ToolCallStatePermissionPending,
			enum.ToolCallStatePermissionApproved,
			enum.ToolCallStatePermissionDenied,
		}
		
		expected := []bool{false, true, true, true, true, true, true, false}
		
		for i, state := range states {
			got := state.ShouldShowContentForState(false, false)
			if got != expected[i] {
				t.Errorf("State %s: got %v, want %v", state, got, expected[i])
			}
		}
	})
	
	t.Run("Pending state variations", func(t *testing.T) {
		// Test all combinations of isNested/hasNested for Pending state
		combinations := []struct{ isNested, hasNested, want bool }{
			{false, false, false}, // top-level, no children: don't show
			{false, true,  true},  // top-level, has children: show (context)
			{true,  false, false}, // nested, no children: don't show  
			{true,  true,  false}, // nested, has children: don't show
		}
		
		for _, combo := range combinations {
			got := enum.ToolCallStatePending.ShouldShowContentForState(combo.isNested, combo.hasNested)
			if got != combo.want {
				t.Errorf("Pending(isNested=%v, hasNested=%v) = %v, want %v",
					combo.isNested, combo.hasNested, got, combo.want)
			}
		}
	})
}