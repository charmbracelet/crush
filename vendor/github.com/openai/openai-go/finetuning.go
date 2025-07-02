// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package openai

import (
	"github.com/openai/openai-go/option"
)

// FineTuningService contains methods and other services that help with interacting
// with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewFineTuningService] method instead.
type FineTuningService struct {
	Options []option.RequestOption
	Jobs    FineTuningJobService
}

// NewFineTuningService generates a new service that applies the given options to
// each request. These options are applied after the parent client's options (if
// there is one), and before any request-specific options.
func NewFineTuningService(opts ...option.RequestOption) (r FineTuningService) {
	r = FineTuningService{}
	r.Options = opts
	r.Jobs = NewFineTuningJobService(opts...)
	return
}
