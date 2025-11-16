package enum

import (
	"image/color"

	"github.com/charmbracelet/crush/internal/tui/styles"
)

// AnimationState represents the different animation states for tool calls.
// This replaces the boolean spinning field with type-safe animation states.
type AnimationState uint8

const (
	// AnimationStateNone indicates no animation should be shown
	AnimationStateNone AnimationState = iota

	// AnimationStateStatic indicates a static display without animation
	// Used for completed, failed, cancelled, or pending tools
	AnimationStateStatic

	// AnimationStateSpinner indicates a dot spinner animation
	// Used for actively running tool calls
	AnimationStateSpinner

	// AnimationStateTimer indicates a countdown timer animation
	// Used for tools awaiting user permission approval
	AnimationStateTimer

	// AnimationStateBlink indicates a blinking success animation
	// Used for recently completed successful tool calls
	AnimationStateBlink

	// AnimationStatePulse indicates a pulsing animation
	// Used for processing or transitional states
	AnimationStatePulse
)

// IsActive returns true if the animation state should display movement
func (state AnimationState) IsActive() bool {
	return state == AnimationStateSpinner ||
		state == AnimationStateTimer ||
		state == AnimationStateBlink ||
		state == AnimationStatePulse
}

// IsStatic returns true if the animation state should be static display
func (state AnimationState) IsStatic() bool {
	return state == AnimationStateNone || state == AnimationStateStatic
}

// String returns string representation of animation state
func (state AnimationState) String() string {
	switch state {
	case AnimationStateNone:
		return ""
	case AnimationStateStatic:
		return "static"
	case AnimationStateSpinner:
		return "spinner"
	case AnimationStateTimer:
		return "timer"
	case AnimationStateBlink:
		return "blink"
	case AnimationStatePulse:
		return "pulse"
	default:
		return "unknown"
	}
}

// ToIcon returns appropriate icon for animation state
func (state AnimationState) ToIcon() string {
	switch state {
	case AnimationStateNone, AnimationStateStatic:
		return ""
	case AnimationStateSpinner, AnimationStateTimer:
		return "⋯" // Loading icon
	case AnimationStateBlink:
		return "✅" // Success icon
	case AnimationStatePulse:
		return "⚡" // Processing icon
	default:
		return ""
	}
}

// ToLabel returns descriptive label for animation state
func (state AnimationState) ToLabel() string {
	switch state {
	case AnimationStateNone, AnimationStateStatic:
		return ""
	case AnimationStateSpinner:
		return "Running"
	case AnimationStateTimer:
		return "Waiting"
	case AnimationStateBlink:
		return "Completed"
	case AnimationStatePulse:
		return "Processing"
	default:
		return ""
	}
}

// isCycleColors determines if the animation should cycle colors based on animation state
func (state AnimationState) isCycleColors() (bool, error) {
	switch state {
	case AnimationStateSpinner, AnimationStatePulse:
		// Active animations should cycle colors for visual feedback
		return true, nil
	case AnimationStateStatic, AnimationStateTimer, AnimationStateBlink, AnimationStateNone:
		// Static or single-shot animations don't need color cycling
		return false, nil
	}
	return false, ErrAnimationStateUnknown
}

// toLabelColor returns the appropriate label color for the animation state
func (state AnimationState) toLabelColor() (color.Color, error) {
	t := styles.CurrentTheme()
	switch state {
	case AnimationStateSpinner, AnimationStatePulse, AnimationStateTimer:
		// Active animations use base color for high visibility
		return t.FgBase, nil
	case AnimationStateStatic, AnimationStateBlink, AnimationStateNone:
		// Static or completed states use subtle color
		return t.FgSubtle, nil
	}
	return t.Error, ErrAnimationStateUnknown
}
