package enum

// AnimationState represents the different animation states for tool calls.
// This replaces the boolean spinning field with type-safe animation states.
type AnimationState string

const (
	// AnimationStateNone indicates no animation should be shown
	AnimationStateNone AnimationState = ""

	// AnimationStateStatic indicates a static display without animation
	// Used for completed, failed, cancelled, or pending tools
	AnimationStateStatic AnimationState = "static"

	// AnimationStateSpinner indicates a dot spinner animation
	// Used for actively running tool calls
	AnimationStateSpinner AnimationState = "spinner"

	// AnimationStateTimer indicates a countdown timer animation
	// Used for tools awaiting user permission approval
	AnimationStateTimer AnimationState = "timer"

	// AnimationStateBlink indicates a blinking success animation
	// Used for recently completed successful tool calls
	AnimationStateBlink AnimationState = "blink"

	// AnimationStatePulse indicates a pulsing animation
	// Used for processing or transitional states
	AnimationStatePulse AnimationState = "pulse"
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

// String returns the string representation of the animation state
func (state AnimationState) String() string {
	return string(state)
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
	case AnimationStateNone:
		return ""
	case AnimationStateStatic:
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
