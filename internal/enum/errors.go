package enum

import "errors"

var (
	ErrToolCallStateUnknown  = errors.New("unknown tool call state")
	ErrAnimationStateUnknown = errors.New("unknown animation state")
)
