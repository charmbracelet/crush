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
// Enhanced for PR #1385: Specific cycling for different animation types
func (state AnimationState) isCycleColors() (bool, error) {
	switch state {
	case AnimationStateSpinner:
		// Running state: green dot that blinks every 1s, cycles colors
		return true, nil
	case AnimationStatePulse:
		// Processing state: pulses with color cycling
		return true, nil
	case AnimationStateTimer:
		// Awaiting permission: orange timer, no cycling (to focus on countdown)
		return false, nil
	case AnimationStateBlink:
		// Recently completed: blink success, no cycling
		return false, nil
	case AnimationStateStatic, AnimationStateNone:
		// Static states: no cycling
		return false, nil
	}
	return false, ErrAnimationStateUnknown
}

// toLabelColor returns the appropriate label color for the animation state
// Enhanced for PR #1385: Color-coded states for better UX
func (state AnimationState) toLabelColor() (color.Color, error) {
	t := styles.CurrentTheme()
	switch state {
	case AnimationStateSpinner:
		// Running state: green label for active execution
		return t.Green, nil
	case AnimationStatePulse:
		// Processing state: blue label for transitional
		return t.Blue, nil
	case AnimationStateTimer:
		// Awaiting permission: orange label (Paprika) for attention
		return t.Paprika, nil
	case AnimationStateBlink:
		// Recently completed: green label for success
		return t.Green, nil
	case AnimationStateStatic, AnimationStateNone:
		// Static states: subtle color for non-active
		return t.FgSubtle, nil
	}
	return t.Error, ErrAnimationStateUnknown
}
