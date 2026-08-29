package agent

import "errors"

var (
	ErrRequestCancelled = errors.New("request canceled by user")
	ErrSessionBusy      = errors.New("session is currently processing another request")
	ErrEmptyPrompt      = errors.New("prompt is empty")
	ErrSessionMissing   = errors.New("session id is missing")
	// ErrToolCallsNotRun reports a turn that ended with tool calls the
	// provider never let us run: the response was cut off before the
	// calls could be dispatched, so they produced no results. The turn
	// must fail rather than report success with a broken transcript.
	ErrToolCallsNotRun = errors.New("the model's response was cut off before its tool calls could run")
	// ErrToolResultsMissing reports a turn that ended with tool calls
	// that have no stored result for another reason: the tools ran, but
	// writing a result failed and fantasy discards that error. The
	// transcript is repaired, and the turn fails rather than report a
	// success the stored history does not support.
	ErrToolResultsMissing = errors.New("the turn ended with tool calls that have no result")
)
