package agent

import (
	"context"

	"charm.land/fantasy"
)

// stopOnCancelModel ends a stream as soon as its context is cancelled. A
// provider that has already buffered the rest of a response keeps yielding it
// from memory, where cancelling the context reaches nothing.
type stopOnCancelModel struct {
	fantasy.LanguageModel
}

// newStopOnCancelModel wraps m, returning a nil model unchanged.
func newStopOnCancelModel(m fantasy.LanguageModel) fantasy.LanguageModel {
	if m == nil {
		return m
	}
	return stopOnCancelModel{LanguageModel: m}
}

// Stream implements [fantasy.LanguageModel]. The stop is reported as an error
// part, since a quiet stop reads as a finished stream and leaves a
// half-written tool call open.
func (m stopOnCancelModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	inner, err := m.LanguageModel.Stream(ctx, call)
	if err != nil {
		return nil, err
	}
	return func(yield func(fantasy.StreamPart) bool) {
		inner(func(part fantasy.StreamPart) bool {
			if err := context.Cause(ctx); err != nil {
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: err})
				return false
			}
			return yield(part)
		})
	}, nil
}
