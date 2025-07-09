// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package openai

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/openai/openai-go/internal/requestconfig"
	"github.com/openai/openai-go/option"
)

// ContainerFileContentService contains methods and other services that help with
// interacting with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewContainerFileContentService] method instead.
type ContainerFileContentService struct {
	Options []option.RequestOption
}

// NewContainerFileContentService generates a new service that applies the given
// options to each request. These options are applied after the parent client's
// options (if there is one), and before any request-specific options.
func NewContainerFileContentService(opts ...option.RequestOption) (r ContainerFileContentService) {
	r = ContainerFileContentService{}
	r.Options = opts
	return
}

// Retrieve Container File Content
func (r *ContainerFileContentService) Get(ctx context.Context, containerID string, fileID string, opts ...option.RequestOption) (res *http.Response, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("Accept", "application/binary")}, opts...)
	if containerID == "" {
		err = errors.New("missing required container_id parameter")
		return
	}
	if fileID == "" {
		err = errors.New("missing required file_id parameter")
		return
	}
	path := fmt.Sprintf("containers/%s/files/%s/content", containerID, fileID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}
