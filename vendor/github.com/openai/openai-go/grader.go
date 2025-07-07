// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package openai

import (
	"github.com/openai/openai-go/option"
)

// GraderService contains methods and other services that help with interacting
// with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewGraderService] method instead.
type GraderService struct {
	Options      []option.RequestOption
	GraderModels GraderGraderModelService
}

// NewGraderService generates a new service that applies the given options to each
// request. These options are applied after the parent client's options (if there
// is one), and before any request-specific options.
func NewGraderService(opts ...option.RequestOption) (r GraderService) {
	r = GraderService{}
	r.Options = opts
	r.GraderModels = NewGraderGraderModelService(opts...)
	return
}
