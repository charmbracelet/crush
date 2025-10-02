package agent

import (
	"context"
	"errors"
)

var (
	ErrRequestCancelled = errors.New("request canceled by user")
	ErrSessionBusy      = errors.New("session is currently processing another request")
)

func isCancelledErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, ErrRequestCancelled)
}
