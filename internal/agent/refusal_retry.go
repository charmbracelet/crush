package agent

import (
	"context"
	"errors"
	"log/slog"

	"charm.land/fantasy"
)

// refusalMessage is what the model and the user see once every retry of a
// refused response has been refused too.
const refusalMessage = "The provider's safety classifier refused this response on every retry. Rephrase the request or try a different model."

// errRefused marks a response the provider's safety classifier stopped.
var errRefused = errors.New("response refused by the provider's safety classifier")

// refusalRetryModel turns a refused response into a retryable error, so
// fantasy re-requests the step with its usual backoff and resets what the
// refused attempt streamed. Safety classifiers misfire often enough on
// legitimate work that a fresh sample usually gets through.
type refusalRetryModel struct {
	fantasy.LanguageModel
}

func newRefusalRetryModel(m fantasy.LanguageModel) fantasy.LanguageModel {
	if m == nil {
		return nil
	}
	return refusalRetryModel{LanguageModel: m}
}

// Stream implements [fantasy.LanguageModel].
func (m refusalRetryModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	inner, err := m.LanguageModel.Stream(ctx, call)
	if err != nil {
		return nil, err
	}
	return func(yield func(fantasy.StreamPart) bool) {
		inner(func(part fantasy.StreamPart) bool {
			if part.Type == fantasy.StreamPartTypeFinish && part.FinishReason == fantasy.FinishReasonContentFilter {
				slog.Warn("Provider safety classifier refused the response, retrying")
				return yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: refusedError()})
			}
			return yield(part)
		})
	}, nil
}

func refusedError() *fantasy.ProviderError {
	return &fantasy.ProviderError{
		Title:          "Refused",
		Message:        refusalMessage,
		Cause:          errRefused,
		TransientError: true,
	}
}
