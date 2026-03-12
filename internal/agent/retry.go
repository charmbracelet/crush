package agent

import (
	"context"
	"errors"
	"io"

	"charm.land/fantasy"
)

// retryableStreamModel wraps a fantasy.LanguageModel and converts bare
// retryable network errors (such as io.ErrUnexpectedEOF) inside stream parts
// into *fantasy.ProviderError so the fantasy library's built-in retry
// mechanism can recognize and retry them.
type retryableStreamModel struct {
	fantasy.LanguageModel
}

// Stream implements fantasy.LanguageModel.
func (m retryableStreamModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	stream, err := m.LanguageModel.Stream(ctx, call)
	if err != nil {
		return nil, wrapRetryableNetworkErr(err)
	}
	return func(yield func(fantasy.StreamPart) bool) {
		sawToolUse := false
		stream(func(part fantasy.StreamPart) bool {
			if isToolStreamPart(part.Type) {
				sawToolUse = true
			}
			if !sawToolUse && part.Type == fantasy.StreamPartTypeError && part.Error != nil {
				part.Error = wrapRetryableNetworkErr(part.Error)
			}
			return yield(part)
		})
	}, nil
}

func isToolStreamPart(partType fantasy.StreamPartType) bool {
	switch partType {
	case fantasy.StreamPartTypeToolInputStart,
		fantasy.StreamPartTypeToolInputDelta,
		fantasy.StreamPartTypeToolInputEnd,
		fantasy.StreamPartTypeToolCall,
		fantasy.StreamPartTypeToolResult:
		return true
	default:
		return false
	}
}

// wrapRetryableNetworkErr wraps known retryable network errors into
// *fantasy.ProviderError so the fantasy retry mechanism can recognize them as
// retryable. If the error is not a known retryable network error, it is
// returned unchanged.
func wrapRetryableNetworkErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return &fantasy.ProviderError{
			Title:   "network error",
			Message: err.Error(),
			Cause:   err,
		}
	}
	return err
}
