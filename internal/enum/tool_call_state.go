package enum

import (
	"image/color"
	"time"

	"github.com/charmbracelet/crush/internal/tui/components/anim"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/log/v2"
)

type ToolCallState uint8

const (
	// ToolCallStatePending Tool has been created but not yet started execution
	// e.g. multiple tool calls at once
	ToolCallStatePending ToolCallState = iota

	// ToolCallStatePermissionPending Tool is pending permission approval
	ToolCallStatePermissionPending

	// ToolCallStatePermissionApproved Tool has received permission approval
	ToolCallStatePermissionApproved

	// ToolCallStatePermissionDenied Tool permission was denied
	ToolCallStatePermissionDenied

	// ToolCallStateRunning Tool is actively executing
	ToolCallStateRunning

	// ToolCallStateCompleted Tool completed successfully
	ToolCallStateCompleted

	// ToolCallStateFailed Tool failed during execution
	ToolCallStateFailed

	// ToolCallStateCancelled Tool was explicitly cancelled by user
	ToolCallStateCancelled
)

func (state ToolCallState) IsFinalState() bool {
	return state == ToolCallStateCompleted ||
		state == ToolCallStateFailed ||
		state == ToolCallStateCancelled ||
		state == ToolCallStatePermissionDenied
}

func (state ToolCallState) IsNonFinalState() bool {
	return !state.IsFinalState()
}

func (state ToolCallState) ToIcon() string {
	switch state {
	case ToolCallStatePending:
		return styles.ToolPending
	case ToolCallStateRunning:
		return styles.ToolPending
	case ToolCallStatePermissionPending:
		return styles.ToolPending
	case ToolCallStatePermissionApproved:
		return styles.ToolSuccess
	case ToolCallStatePermissionDenied:
		return styles.ToolError
	case ToolCallStateCompleted:
		return styles.ToolSuccess
	case ToolCallStateCancelled:
		return styles.ToolCancel
	case ToolCallStateFailed:
		return styles.ToolError
	default:
		// In case of unknown states we also return the error Icon
		return styles.ToolError
	}
}

func (state ToolCallState) ToFgColor() color.Color {
	t := styles.CurrentTheme()
	switch state {
	case ToolCallStatePending:
		return t.FgMuted
	case ToolCallStatePermissionPending:
		return t.Paprika
	case ToolCallStatePermissionApproved:
		return t.Green
	case ToolCallStatePermissionDenied:
		return t.Error
	case ToolCallStateRunning:
		return t.GreenDark // Use darker green for active running state
	case ToolCallStateCompleted:
		return t.Green // Use bright green for successful completion
	case ToolCallStateCancelled:
		return t.FgMuted // Muted is appropriate for user-initiated cancellation
	case ToolCallStateFailed:
		return t.Error // Use error color for failed operations
	default:
		// In case of unknown states we also return the error Icon
		return t.Error
	}
}

func (state ToolCallState) ToIconColored() string {
	t := styles.CurrentTheme()
	return t.S().Base.Foreground(state.ToFgColor()).Render(state.ToIcon())
}

func (state ToolCallState) FormatToolForCopy() string {
	switch state {
	case ToolCallStatePending:
		return "Pending..."
	case ToolCallStateRunning:
		return "Running..."
	case ToolCallStatePermissionPending:
		return "Permissions..."
	case ToolCallStatePermissionApproved:
		return "Approved"
	case ToolCallStatePermissionDenied:
		return "Denied"
	case ToolCallStateCancelled:
		return "Cancelled"
	case ToolCallStateFailed:
		return "Failed"
	case ToolCallStateCompleted:
		return "" // Final states don't need status messages
	default:
		return ""
	}
}

func (state ToolCallState) RenderTUIMessage() (string, error) {
	switch state {
	case ToolCallStateFailed:
		return "Tool call failed", nil
	case ToolCallStateCancelled:
		return "Cancelled", nil
	case ToolCallStateCompleted:
		return "Done", nil
	case ToolCallStateRunning:
		return "Running", nil
	case ToolCallStatePending:
		return "Waiting for tool to start", nil
	case ToolCallStatePermissionPending:
		return "Awaiting permission...", nil
	case ToolCallStatePermissionApproved:
		return "Permission approved. Executing command", nil
	case ToolCallStatePermissionDenied:
		return "Permission denied", nil
	default:
		return "", ErrToolCallStateUnknown
	}
}

func (state ToolCallState) RenderTUIMessageColored() (string, error) {
	t := styles.CurrentTheme()
	messageBaseStyle := t.S().Base.Foreground(t.FgSubtle)
	message, err := state.RenderTUIMessage()
	if err != nil {
		return "", err
	}

	switch state {
	case ToolCallStateFailed:
		{
			messageBaseStyle = messageBaseStyle.Padding(0, 1).Background(t.Red).Foreground(t.White)
			// Error content is handled by renderer logic, not here
			// This function only provides the status message styling
		}
	case ToolCallStatePermissionApproved:
		{
			// Green background for approved permission
			messageBaseStyle = messageBaseStyle.Padding(0, 1).Background(t.Green).Foreground(t.White)
		}
	case ToolCallStatePermissionDenied:
		{
			// Red background for denied permission
			messageBaseStyle = messageBaseStyle.Padding(0, 1).Background(t.Red).Foreground(t.White)
		}
	case ToolCallStateCompleted:
		{
			// This should make the background light gray in case of ToolCallStateCompleted, see: https://github.com/charmbracelet/crush/pull/1385#issuecomment-3504123709
			messageBaseStyle = messageBaseStyle.Padding(0, 1).Background(t.BgBaseLighter).Foreground(t.FgSubtle)
		}

	}

	return messageBaseStyle.Render(message), nil
}

// ToAnimationState converts tool call state to appropriate animation state
// Enhanced for PR #1385: 5-state system with timer and blinking animations
func (state ToolCallState) ToAnimationState() AnimationState {
	switch state {
	// Permission states use timer animation for visual queue feedback
	case ToolCallStatePermissionPending:
		return AnimationStateTimer // Timer counts up every 1s while awaiting permission
	case ToolCallStatePermissionApproved:
		return AnimationStatePulse
	case ToolCallStatePermissionDenied:
		return AnimationStateStatic

	// Final states
	case ToolCallStateCompleted:
		return AnimationStateBlink // Blinks every 1s for success feedback
	case ToolCallStateFailed:
		return AnimationStateStatic
	case ToolCallStateCancelled:
		return AnimationStateStatic

	// Running states use spinner for active feedback
	case ToolCallStateRunning:
		return AnimationStateSpinner // Green dot that blinks every 1s

	// Pending state is static
	case ToolCallStatePending:
		return AnimationStateStatic // Grey dot, no animation

	default:
		return AnimationStateNone
	}
}

// ShouldShowContentForState determines if content should be displayed for a given tool state
func (state ToolCallState) ShouldShowContentForState(isNested, hasNested bool) bool {
	switch state {
	// Show content for permission states
	case ToolCallStatePermissionPending:
		return true // Show tool details while waiting for permission

	case ToolCallStatePermissionApproved:
		return true // Show content that was approved

	case ToolCallStatePermissionDenied:
		return false // [RFC] Don't show content that was denied - review this policy

	// Show content for final states (except denied)
	case ToolCallStateCompleted:
		return true // Show successful results

	case ToolCallStateFailed:
		return true // Show error content for debugging

	case ToolCallStateCancelled:
		return true // Show what was cancelled

	// Show minimal content for transitional states
	case ToolCallStatePending:
		// Don't show content until tool starts, unless it's a parent tool with nested calls
		// In that case, show the header to provide context for the nested tools
		return hasNested && !isNested

	case ToolCallStateRunning:
		return true // Show progress/running state

	default:
		// Add error logging for unknown states
		log.Error("Unknown tool state in ShouldShowContentForState:", "state", state)
		return false // Unknown states don't show content
	}
}

// String returns string representation of tool call state
func (state ToolCallState) String() string {
	switch state {
	case ToolCallStatePending:
		return "pending"
	case ToolCallStatePermissionPending:
		return "permission_pending"
	case ToolCallStatePermissionApproved:
		return "permission_approved"
	case ToolCallStatePermissionDenied:
		return "permission_denied"
	case ToolCallStateRunning:
		return "running"
	case ToolCallStateCompleted:
		return "completed"
	case ToolCallStateFailed:
		return "failed"
	case ToolCallStateCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

func (state ToolCallState) ToAnimationSettings(isNested bool) anim.Settings {
	size := 15
	t := styles.CurrentTheme()

	// Override for nested tools to ensure consistent styling
	if isNested {
		return anim.Settings{
			Size:        size / 3 * 2,
			Label:       "",
			GradColorA:  t.Primary, // based on original visibility
			GradColorB:  t.Secondary,
			LabelColor:  t.FgBase,
			CycleColors: true, // based on original cycling
		}
	}

	// Get consistent label from RenderTUIMessage for animation
	label, err := state.RenderTUIMessage()
	if err != nil {
		// Fallback to reasonable default when state rendering fails
		label = "Unknown State"
	}

	gradColors := state.ToFgColor()

	// Convert to animation state to get animation-specific properties
	animationState := state.ToAnimationState()

	// Get label color with error handling
	labelColor, err := animationState.toLabelColor()
	if err != nil {
		// Fallback to error color when state is unknown
		labelColor = t.Error
	}

	// Get cycle colors with error handling
	cycleColors, err := animationState.isCycleColors()
	if err != nil {
		// Fallback to no cycling when state is unknown
		cycleColors = false
	}

	animationSettings := anim.Settings{
		Size:        size,
		Label:       label,
		GradColorA:  gradColors,
		GradColorB:  gradColors,
		LabelColor:  labelColor,
		CycleColors: cycleColors,

		// PR #1385: Timer and blinking animation configuration
		IsTimer:          animationState == AnimationStateTimer,
		IsBlinking:       animationState == AnimationStateBlink,
		TimerInterval:    time.Second, // 1 second intervals
		BlinkingInterval: time.Second, // 1 second intervals
	}

	return animationSettings
}
