package agent

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"

	"charm.land/fantasy"
)

// refusalMessage is what the model and the user see once a refusal is final.
const refusalMessage = "The provider's safety classifier refused this response. Rephrase the request or try a different model."

// refusalRetriesPerTurn is how many refusals a turn retries before showing
// one. A misfire usually clears on the first retry; more mostly waits.
const refusalRetriesPerTurn = 1

type refusalBudgetKey struct{}

// withRefusalRetries gives the turn on ctx a budget of refusal retries.
func withRefusalRetries(ctx context.Context, n int32) context.Context {
	budget := new(atomic.Int32)
	budget.Store(n)
	return context.WithValue(ctx, refusalBudgetKey{}, budget)
}

// takeRefusalRetry spends one retry from ctx's budget; without a budget
// there is none to spend.
func takeRefusalRetry(ctx context.Context) bool {
	budget, ok := ctx.Value(refusalBudgetKey{}).(*atomic.Int32)
	return ok && budget.Add(-1) >= 0
}

// errRefused marks a response the provider's safety classifier stopped.
var errRefused = errors.New("response refused by the provider's safety classifier")

// refusalRetryModel turns a refused response into an error, retryable while
// the turn's refusal budget lasts, so fantasy re-requests the step and resets
// what the refused attempt streamed.
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
				retry := takeRefusalRetry(ctx)
				slog.Warn("Provider safety classifier refused the response", "retrying", retry)
				return yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: refusedError(retry)})
			}
			return yield(part)
		})
	}, nil
}

func refusedError(retryable bool) *fantasy.ProviderError {
	return &fantasy.ProviderError{
		Title:          "Refused",
		Message:        refusalMessage,
		Cause:          errRefused,
		TransientError: retryable,
	}
}
