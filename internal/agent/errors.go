package agent

import (
	"errors"
)

// Define error types locally since we're using the standard errors package
var (
	ErrRequestCancelled = errors.New("request cancelled")
	ErrSessionBusy      = errors.New("session busy")
	ErrEmptyPrompt      = errors.New("empty prompt")
	ErrSessionMissing   = errors.New("session missing")
)
