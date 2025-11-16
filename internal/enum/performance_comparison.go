package enum

// Experimental uint8-based ToolCallState for performance testing
// NOT FOR PRODUCTION USE - for benchmarking only
type ToolCallStateUint8 uint8

const (
	ToolCallStateUint8Pending ToolCallStateUint8 = iota
	ToolCallStateUint8PermissionPending
	ToolCallStateUint8PermissionApproved
	ToolCallStateUint8PermissionDenied
	ToolCallStateUint8Running
	ToolCallStateUint8Completed
	ToolCallStateUint8Failed
	ToolCallStateUint8Cancelled
)

// Experimental uint8-based AnimationState for performance testing
// NOT FOR PRODUCTION USE - for benchmarking only
type AnimationStateUint8 uint8

const (
	AnimationStateUint8None AnimationStateUint8 = iota
	AnimationStateUint8Static
	AnimationStateUint8Spinner
	AnimationStateUint8Timer
	AnimationStateUint8Blink
	AnimationStateUint8Pulse
)

// String conversion methods for uint8 versions
func (s ToolCallStateUint8) String() string {
	switch s {
	case ToolCallStateUint8Pending:
		return "pending"
	case ToolCallStateUint8PermissionPending:
		return "permission_pending"
	case ToolCallStateUint8PermissionApproved:
		return "permission_approved"
	case ToolCallStateUint8PermissionDenied:
		return "permission_denied"
	case ToolCallStateUint8Running:
		return "running"
	case ToolCallStateUint8Completed:
		return "completed"
	case ToolCallStateUint8Failed:
		return "failed"
	case ToolCallStateUint8Cancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

func (s AnimationStateUint8) String() string {
	switch s {
	case AnimationStateUint8None:
		return ""
	case AnimationStateUint8Static:
		return "static"
	case AnimationStateUint8Spinner:
		return "spinner"
	case AnimationStateUint8Timer:
		return "timer"
	case AnimationStateUint8Blink:
		return "blink"
	case AnimationStateUint8Pulse:
		return "pulse"
	default:
		return "unknown"
	}
}