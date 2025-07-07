// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package openai

import (
	"github.com/openai/openai-go/option"
)

// FineTuningCheckpointService contains methods and other services that help with
// interacting with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewFineTuningCheckpointService] method instead.
type FineTuningCheckpointService struct {
	Options     []option.RequestOption
	Permissions FineTuningCheckpointPermissionService
}

// NewFineTuningCheckpointService generates a new service that applies the given
// options to each request. These options are applied after the parent client's
// options (if there is one), and before any request-specific options.
func NewFineTuningCheckpointService(opts ...option.RequestOption) (r FineTuningCheckpointService) {
	r = FineTuningCheckpointService{}
	r.Options = opts
	r.Permissions = NewFineTuningCheckpointPermissionService(opts...)
	return
}
