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