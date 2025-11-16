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
func BenchmarkToolCallStateUint8Switch(b *testing.B) {
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
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, state := range states {
			switch state {
			case ToolCallStatePending:
				_ = "pending"
			case ToolCallStateRunning:
				_ = "running"
			case ToolCallStateCompleted:
				_ = "completed"
			case ToolCallStateFailed:
				_ = "failed"
			case ToolCallStateCancelled:
				_ = "cancelled"
			case ToolCallStatePermissionPending:
				_ = "permission_pending"
			case ToolCallStatePermissionApproved:
				_ = "permission_approved"
			case ToolCallStatePermissionDenied:
				_ = "permission_denied"
			default:
				_ = "unknown"
			}
		}
	}
}

// Benchmark uint8-based animation switch
func BenchmarkAnimationStateUint8Switch(b *testing.B) {
	states := []AnimationState{
		AnimationStateNone,
		AnimationStateStatic,
		AnimationStateSpinner,
		AnimationStateTimer,
		AnimationStateBlink,
		AnimationStatePulse,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, state := range states {
			switch state {
			case AnimationStateNone:
				_ = false
			case AnimationStateStatic:
				_ = false
			case AnimationStateSpinner:
				_ = true
			case AnimationStateTimer:
				_ = true
			case AnimationStateBlink:
				_ = true
			case AnimationStatePulse:
				_ = true
			default:
				_ = false
			}
		}
	}
}