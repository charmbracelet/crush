package enum

import "errors"

var (
	ErrUnknownToolCallState  = errors.New("unknown tool call state")
	ErrUnknownAnimationState = errors.New("unknown animation state")
)
