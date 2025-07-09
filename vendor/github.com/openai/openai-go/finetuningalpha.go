// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package openai

import (
	"github.com/openai/openai-go/option"
)

// FineTuningAlphaService contains methods and other services that help with
// interacting with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewFineTuningAlphaService] method instead.
type FineTuningAlphaService struct {
	Options []option.RequestOption
	Graders FineTuningAlphaGraderService
}

// NewFineTuningAlphaService generates a new service that applies the given options
// to each request. These options are applied after the parent client's options (if
// there is one), and before any request-specific options.
func NewFineTuningAlphaService(opts ...option.RequestOption) (r FineTuningAlphaService) {
	r = FineTuningAlphaService{}
	r.Options = opts
	r.Graders = NewFineTuningAlphaGraderService(opts...)
	return
}
