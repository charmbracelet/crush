package enum

import (
	"testing"
)

// Benchmark uint8-based ToolCallState comparison
func BenchmarkToolCallStateUint8Comparison(b *testing.B) {
	states := []ToolCallState{
		ToolCallStatePending,
		ToolCallStateRunning,
		ToolCallStateCompleted,
		ToolCallStateFailed,
		ToolCallStateCancelled,
		ToolCallStatePermissionPending,
		ToolCallStatePermissionApproved,
		ToolCallStatePermissionDenied,
	}
	
	target := ToolCallStateCompleted
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, state := range states {
			if state == target {
				break
			}
		}
	}
}

// Benchmark uint8-based AnimationState comparison
func BenchmarkAnimationStateUint8Comparison(b *testing.B) {
	states := []AnimationState{
		AnimationStateNone,
		AnimationStateStatic,
		AnimationStateSpinner,
		AnimationStateTimer,
		AnimationStateBlink,
		AnimationStatePulse,
	}
	
	target := AnimationStateSpinner
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, state := range states {
			if state == target {
				break
			}
		}
	}
}

// Benchmark uint8-based switch statement
