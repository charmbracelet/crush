// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package openai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/openai/openai-go/internal/apijson"
	"github.com/openai/openai-go/internal/apiquery"
	"github.com/openai/openai-go/internal/requestconfig"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/pagination"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/respjson"
)

// ContainerService contains methods and other services that help with interacting
// with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewContainerService] method instead.
type ContainerService struct {
	Options []option.RequestOption
	Files   ContainerFileService
}

// NewContainerService generates a new service that applies the given options to
// each request. These options are applied after the parent client's options (if
// there is one), and before any request-specific options.
func NewContainerService(opts ...option.RequestOption) (r ContainerService) {
	r = ContainerService{}
	r.Options = opts
	r.Files = NewContainerFileService(opts...)
	return
}

// Create Container
func (r *ContainerService) New(ctx context.Context, body ContainerNewParams, opts ...option.RequestOption) (res *ContainerNewResponse, err error) {
	opts = append(r.Options[:], opts...)
	path := "containers"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// Retrieve Container
func (r *ContainerService) Get(ctx context.Context, containerID string, opts ...option.RequestOption) (res *ContainerGetResponse, err error) {
	opts = append(r.Options[:], opts...)
	if containerID == "" {
		err = errors.New("missing required container_id parameter")
		return
	}
	path := fmt.Sprintf("containers/%s", containerID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// List Containers
func (r *ContainerService) List(ctx context.Context, query ContainerListParams, opts ...option.RequestOption) (res *pagination.CursorPage[ContainerListResponse], err error) {
	var raw *http.Response
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithResponseInto(&raw)}, opts...)
	path := "containers"
	cfg, err := requestconfig.NewRequestConfig(ctx, http.MethodGet, path, query, &res, opts...)
	if err != nil {
		return nil, err
	}
	err = cfg.Execute()
	if err != nil {
		return nil, err
	}
	res.SetPageConfig(cfg, raw)
	return res, nil
}

// List Containers
func (r *ContainerService) ListAutoPaging(ctx context.Context, query ContainerListParams, opts ...option.RequestOption) *pagination.CursorPageAutoPager[ContainerListResponse] {
	return pagination.NewCursorPageAutoPager(r.List(ctx, query, opts...))
}

// Delete Container
func (r *ContainerService) Delete(ctx context.Context, containerID string, opts ...option.RequestOption) (err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("Accept", "")}, opts...)
	if containerID == "" {
		err = errors.New("missing required container_id parameter")
		return
	}
	path := fmt.Sprintf("containers/%s", containerID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodDelete, path, nil, nil, opts...)
	return
}

type ContainerNewResponse struct {
	// Unique identifier for the container.
	ID string `json:"id,required"`
	// Unix timestamp (in seconds) when the container was created.
	CreatedAt int64 `json:"created_at,required"`
	// Name of the container.
	Name string `json:"name,required"`
	// The type of this object.
	Object string `json:"object,required"`
	// Status of the container (e.g., active, deleted).
	Status string `json:"status,required"`
	// The container will expire after this time period. The anchor is the reference
	// point for the expiration. The minutes is the number of minutes after the anchor
	// before the container expires.
	ExpiresAfter ContainerNewResponseExpiresAfter `json:"expires_after"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID           respjson.Field
		CreatedAt    respjson.Field
		Name         respjson.Field
		Object       respjson.Field
		Status       respjson.Field
		ExpiresAfter respjson.Field
		ExtraFields  map[string]respjson.Field
		raw          string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ContainerNewResponse) RawJSON() string { return r.JSON.raw }
func (r *ContainerNewResponse) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The container will expire after this time period. The anchor is the reference
// point for the expiration. The minutes is the number of minutes after the anchor
// before the container expires.
type ContainerNewResponseExpiresAfter struct {
	// The reference point for the expiration.
	//
	// Any of "last_active_at".
	Anchor string `json:"anchor"`
	// The number of minutes after the anchor before the container expires.
	Minutes int64 `json:"minutes"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Anchor      respjson.Field
		Minutes     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ContainerNewResponseExpiresAfter) RawJSON() string { return r.JSON.raw }
func (r *ContainerNewResponseExpiresAfter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ContainerGetResponse struct {
	// Unique identifier for the container.
	ID string `json:"id,required"`
	// Unix timestamp (in seconds) when the container was created.
	CreatedAt int64 `json:"created_at,required"`
	// Name of the container.
	Name string `json:"name,required"`
	// The type of this object.
	Object string `json:"object,required"`
	// Status of the container (e.g., active, deleted).
	Status string `json:"status,required"`
	// The container will expire after this time period. The anchor is the reference
	// point for the expiration. The minutes is the number of minutes after the anchor
	// before the container expires.
	ExpiresAfter ContainerGetResponseExpiresAfter `json:"expires_after"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID           respjson.Field
		CreatedAt    respjson.Field
		Name         respjson.Field
		Object       respjson.Field
		Status       respjson.Field
		ExpiresAfter respjson.Field
		ExtraFields  map[string]respjson.Field
		raw          string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ContainerGetResponse) RawJSON() string { return r.JSON.raw }
func (r *ContainerGetResponse) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The container will expire after this time period. The anchor is the reference
// point for the expiration. The minutes is the number of minutes after the anchor
// before the container expires.
type ContainerGetResponseExpiresAfter struct {
	// The reference point for the expiration.
	//
	// Any of "last_active_at".
	Anchor string `json:"anchor"`
	// The number of minutes after the anchor before the container expires.
	Minutes int64 `json:"minutes"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Anchor      respjson.Field
		Minutes     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ContainerGetResponseExpiresAfter) RawJSON() string { return r.JSON.raw }
func (r *ContainerGetResponseExpiresAfter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ContainerListResponse struct {
	// Unique identifier for the container.
	ID string `json:"id,required"`
	// Unix timestamp (in seconds) when the container was created.
	CreatedAt int64 `json:"created_at,required"`
	// Name of the container.
	Name string `json:"name,required"`
	// The type of this object.
	Object string `json:"object,required"`
	// Status of the container (e.g., active, deleted).
	Status string `json:"status,required"`
	// The container will expire after this time period. The anchor is the reference
	// point for the expiration. The minutes is the number of minutes after the anchor
	// before the container expires.
	ExpiresAfter ContainerListResponseExpiresAfter `json:"expires_after"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID           respjson.Field
		CreatedAt    respjson.Field
		Name         respjson.Field
		Object       respjson.Field
		Status       respjson.Field
		ExpiresAfter respjson.Field
		ExtraFields  map[string]respjson.Field
		raw          string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ContainerListResponse) RawJSON() string { return r.JSON.raw }
func (r *ContainerListResponse) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The container will expire after this time period. The anchor is the reference
// point for the expiration. The minutes is the number of minutes after the anchor
// before the container expires.
type ContainerListResponseExpiresAfter struct {
	// The reference point for the expiration.
	//
	// Any of "last_active_at".
	Anchor string `json:"anchor"`
	// The number of minutes after the anchor before the container expires.
	Minutes int64 `json:"minutes"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Anchor      respjson.Field
		Minutes     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ContainerListResponseExpiresAfter) RawJSON() string { return r.JSON.raw }
func (r *ContainerListResponseExpiresAfter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ContainerNewParams struct {
	// Name of the container to create.
	Name string `json:"name,required"`
	// Container expiration time in seconds relative to the 'anchor' time.
	ExpiresAfter ContainerNewParamsExpiresAfter `json:"expires_after,omitzero"`
	// IDs of files to copy to the container.
	FileIDs []string `json:"file_ids,omitzero"`
	paramObj
}

func (r ContainerNewParams) MarshalJSON() (data []byte, err error) {
	type shadow ContainerNewParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ContainerNewParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Container expiration time in seconds relative to the 'anchor' time.
//
// The properties Anchor, Minutes are required.
type ContainerNewParamsExpiresAfter struct {
	// Time anchor for the expiration time. Currently only 'last_active_at' is
	// supported.
	//
	// Any of "last_active_at".
	Anchor  string `json:"anchor,omitzero,required"`
	Minutes int64  `json:"minutes,required"`
	paramObj
}

func (r ContainerNewParamsExpiresAfter) MarshalJSON() (data []byte, err error) {
	type shadow ContainerNewParamsExpiresAfter
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ContainerNewParamsExpiresAfter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[ContainerNewParamsExpiresAfter](
		"anchor", "last_active_at",
	)
}

type ContainerListParams struct {
	// A cursor for use in pagination. `after` is an object ID that defines your place
	// in the list. For instance, if you make a list request and receive 100 objects,
	// ending with obj_foo, your subsequent call can include after=obj_foo in order to
	// fetch the next page of the list.
	After param.Opt[string] `query:"after,omitzero" json:"-"`
	// A limit on the number of objects to be returned. Limit can range between 1 and
	// 100, and the default is 20.
	Limit param.Opt[int64] `query:"limit,omitzero" json:"-"`
	// Sort order by the `created_at` timestamp of the objects. `asc` for ascending
	// order and `desc` for descending order.
	//
	// Any of "asc", "desc".
	Order ContainerListParamsOrder `query:"order,omitzero" json:"-"`
	paramObj
}

// URLQuery serializes [ContainerListParams]'s query parameters as `url.Values`.
func (r ContainerListParams) URLQuery() (v url.Values, err error) {
	return apiquery.MarshalWithSettings(r, apiquery.QuerySettings{
		ArrayFormat:  apiquery.ArrayQueryFormatBrackets,
		NestedFormat: apiquery.NestedQueryFormatBrackets,
	})
}

// Sort order by the `created_at` timestamp of the objects. `asc` for ascending
// order and `desc` for descending order.
type ContainerListParamsOrder string

const (
	ContainerListParamsOrderAsc  ContainerListParamsOrder = "asc"
	ContainerListParamsOrderDesc ContainerListParamsOrder = "desc"
)
