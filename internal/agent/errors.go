package agent

import (
	"context"
	"errors"
)

var (
	ErrRequestCancelled = errors.New("request canceled by user")
	ErrSessionBusy      = errors.New("session is currently processing another request")
	ErrEmptyPrompt      = errors.New("prompt is empty")
	ErrSessionMissing   = errors.New("session id is missing")
)

func isCancelledErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, ErrRequestCancelled)
}
