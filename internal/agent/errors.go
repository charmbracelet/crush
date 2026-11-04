package agent

import "errors"

var (
	ErrRequestCancelled = errors.New("request canceled by user")
	ErrSessionBusy      = errors.New("session is currently processing another request")
	ErrEmptyPrompt      = errors.New("prompt is empty")
	ErrSessionMissing   = errors.New("session id is missing")
	// ErrSessionIdle is returned for a QueueOnly call on a session with no
	// active run, so the caller can decide what to do instead.
	ErrSessionIdle = errors.New("session is idle")
)
