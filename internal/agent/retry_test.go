package agent

import (
	"context"
	"errors"
	"io"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

type stubLanguageModel struct {
	stream func(context.Context, fantasy.Call) (fantasy.StreamResponse, error)
}

func (m stubLanguageModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	panic("unexpected Generate call")
}

func (m stubLanguageModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	return m.stream(ctx, call)
}

func (m stubLanguageModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	panic("unexpected GenerateObject call")
}

func (m stubLanguageModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	panic("unexpected StreamObject call")
}

func (m stubLanguageModel) Provider() string {
	return "test"
}

func (m stubLanguageModel) Model() string {
	return "test"
}

func TestRetryableStreamModelWrapsUnexpectedEOFBeforeToolCall(t *testing.T) {
	t.Parallel()

	model := retryableStreamModel{stubLanguageModel{
		stream: func(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
			return func(yield func(fantasy.StreamPart) bool) {
				yield(fantasy.StreamPart{
					Type:  fantasy.StreamPartTypeError,
					Error: io.ErrUnexpectedEOF,
				})
			}, nil
		},
	}}

	stream, err := model.Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)

	var gotErr error
	stream(func(part fantasy.StreamPart) bool {
		gotErr = part.Error
		return true
	})

	var providerErr *fantasy.ProviderError
	require.ErrorAs(t, gotErr, &providerErr)
	require.ErrorIs(t, providerErr.Cause, io.ErrUnexpectedEOF)
}

func TestRetryableStreamModelDoesNotWrapUnexpectedEOFAfterToolCall(t *testing.T) {
	t.Parallel()

	model := retryableStreamModel{stubLanguageModel{
		stream: func(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
			return func(yield func(fantasy.StreamPart) bool) {
				if !yield(fantasy.StreamPart{
					Type:         fantasy.StreamPartTypeToolCall,
					ID:           "tool-1",
					ToolCallName: "bash",
				}) {
					return
				}
				yield(fantasy.StreamPart{
					Type:  fantasy.StreamPartTypeError,
					Error: io.ErrUnexpectedEOF,
				})
			}, nil
		},
	}}

	stream, err := model.Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)

	var gotErr error
	stream(func(part fantasy.StreamPart) bool {
		if part.Type == fantasy.StreamPartTypeError {
			gotErr = part.Error
		}
		return true
	})

	require.ErrorIs(t, gotErr, io.ErrUnexpectedEOF)
	var providerErr *fantasy.ProviderError
	require.False(t, errors.As(gotErr, &providerErr), "tool-call failures must not become retryable provider errors")
}

func TestRetryableStreamModelDoesNotWrapUnexpectedEOFAfterToolInputStart(t *testing.T) {
	t.Parallel()

	model := retryableStreamModel{stubLanguageModel{
		stream: func(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
			return func(yield func(fantasy.StreamPart) bool) {
				if !yield(fantasy.StreamPart{
					Type:         fantasy.StreamPartTypeToolInputStart,
					ID:           "tool-1",
					ToolCallName: "ls",
				}) {
					return
				}
				yield(fantasy.StreamPart{
					Type:  fantasy.StreamPartTypeError,
					Error: io.ErrUnexpectedEOF,
				})
			}, nil
		},
	}}

	stream, err := model.Stream(t.Context(), fantasy.Call{})
	require.NoError(t, err)

	var gotErr error
	stream(func(part fantasy.StreamPart) bool {
		if part.Type == fantasy.StreamPartTypeError {
			gotErr = part.Error
		}
		return true
	})

	require.ErrorIs(t, gotErr, io.ErrUnexpectedEOF)
	var providerErr *fantasy.ProviderError
	require.False(t, errors.As(gotErr, &providerErr), "tool-input failures must not become retryable provider errors")
}
