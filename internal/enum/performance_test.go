package enum

import (
	"testing"
)

// Benchmark current string-based ToolCallState performance
func BenchmarkToolCallStateStringComparison(b *testing.B) {
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

// Benchmark uint8-based ToolCallState performance
func BenchmarkToolCallStateUint8Comparison(b *testing.B) {
	states := []ToolCallStateUint8{
		ToolCallStateUint8Pending,
		ToolCallStateUint8Running,
		ToolCallStateUint8Completed,
		ToolCallStateUint8Failed,
		ToolCallStateUint8Cancelled,
		ToolCallStateUint8PermissionPending,
		ToolCallStateUint8PermissionApproved,
		ToolCallStateUint8PermissionDenied,
	}
	
	target := ToolCallStateUint8Completed
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, state := range states {
			if state == target {
				break
			}
		}
	}
}

// Benchmark current string-based AnimationState performance
func BenchmarkAnimationStateStringComparison(b *testing.B) {
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

// Benchmark uint8-based AnimationState performance
func BenchmarkAnimationStateUint8Comparison(b *testing.B) {
	states := []AnimationStateUint8{
		AnimationStateUint8None,
		AnimationStateUint8Static,
		AnimationStateUint8Spinner,
		AnimationStateUint8Timer,
		AnimationStateUint8Blink,
		AnimationStateUint8Pulse,
	}
	
	target := AnimationStateUint8Spinner
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, state := range states {
			if state == target {
				break
			}
		}
	}
}

// Benchmark string-based switch statement
func BenchmarkToolCallStateStringSwitch(b *testing.B) {
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

// Benchmark uint8-based switch statement
func BenchmarkToolCallStateUint8Switch(b *testing.B) {
	states := []ToolCallStateUint8{
		ToolCallStateUint8Pending,
		ToolCallStateUint8Running,
		ToolCallStateUint8Completed,
		ToolCallStateUint8Failed,
		ToolCallStateUint8Cancelled,
		ToolCallStateUint8PermissionPending,
		ToolCallStateUint8PermissionApproved,
		ToolCallStateUint8PermissionDenied,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, state := range states {
			switch state {
			case ToolCallStateUint8Pending:
				_ = "pending"
			case ToolCallStateUint8Running:
				_ = "running"
			case ToolCallStateUint8Completed:
				_ = "completed"
			case ToolCallStateUint8Failed:
				_ = "failed"
			case ToolCallStateUint8Cancelled:
				_ = "cancelled"
			case ToolCallStateUint8PermissionPending:
				_ = "permission_pending"
			case ToolCallStateUint8PermissionApproved:
				_ = "permission_approved"
			case ToolCallStateUint8PermissionDenied:
				_ = "permission_denied"
			default:
				_ = "unknown"
			}
		}
	}
}