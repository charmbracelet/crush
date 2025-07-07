// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package openai

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"

	"github.com/openai/openai-go/internal/apiform"
	"github.com/openai/openai-go/internal/apijson"
	"github.com/openai/openai-go/internal/apiquery"
	"github.com/openai/openai-go/internal/requestconfig"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/pagination"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/respjson"
	"github.com/openai/openai-go/shared/constant"
)

// ContainerFileService contains methods and other services that help with
// interacting with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewContainerFileService] method instead.
type ContainerFileService struct {
	Options []option.RequestOption
	Content ContainerFileContentService
}

// NewContainerFileService generates a new service that applies the given options
// to each request. These options are applied after the parent client's options (if
// there is one), and before any request-specific options.
func NewContainerFileService(opts ...option.RequestOption) (r ContainerFileService) {
	r = ContainerFileService{}
	r.Options = opts
	r.Content = NewContainerFileContentService(opts...)
	return
}

// Create a Container File
//
// You can send either a multipart/form-data request with the raw file content, or
// a JSON request with a file ID.
func (r *ContainerFileService) New(ctx context.Context, containerID string, body ContainerFileNewParams, opts ...option.RequestOption) (res *ContainerFileNewResponse, err error) {
	opts = append(r.Options[:], opts...)
	if containerID == "" {
		err = errors.New("missing required container_id parameter")
		return
	}
	path := fmt.Sprintf("containers/%s/files", containerID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// Retrieve Container File
func (r *ContainerFileService) Get(ctx context.Context, containerID string, fileID string, opts ...option.RequestOption) (res *ContainerFileGetResponse, err error) {
	opts = append(r.Options[:], opts...)
	if containerID == "" {
		err = errors.New("missing required container_id parameter")
		return
	}
	if fileID == "" {
		err = errors.New("missing required file_id parameter")
		return
	}
	path := fmt.Sprintf("containers/%s/files/%s", containerID, fileID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// List Container files
func (r *ContainerFileService) List(ctx context.Context, containerID string, query ContainerFileListParams, opts ...option.RequestOption) (res *pagination.CursorPage[ContainerFileListResponse], err error) {
	var raw *http.Response
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithResponseInto(&raw)}, opts...)
	if containerID == "" {
		err = errors.New("missing required container_id parameter")
		return
	}
	path := fmt.Sprintf("containers/%s/files", containerID)
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

// List Container files
func (r *ContainerFileService) ListAutoPaging(ctx context.Context, containerID string, query ContainerFileListParams, opts ...option.RequestOption) *pagination.CursorPageAutoPager[ContainerFileListResponse] {
	return pagination.NewCursorPageAutoPager(r.List(ctx, containerID, query, opts...))
}

// Delete Container File
func (r *ContainerFileService) Delete(ctx context.Context, containerID string, fileID string, opts ...option.RequestOption) (err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("Accept", "")}, opts...)
	if containerID == "" {
		err = errors.New("missing required container_id parameter")
		return
	}
	if fileID == "" {
		err = errors.New("missing required file_id parameter")
		return
	}
	path := fmt.Sprintf("containers/%s/files/%s", containerID, fileID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodDelete, path, nil, nil, opts...)
	return
}

type ContainerFileNewResponse struct {
	// Unique identifier for the file.
	ID string `json:"id,required"`
	// Size of the file in bytes.
	Bytes int64 `json:"bytes,required"`
	// The container this file belongs to.
	ContainerID string `json:"container_id,required"`
	// Unix timestamp (in seconds) when the file was created.
	CreatedAt int64 `json:"created_at,required"`
	// The type of this object (`container.file`).
	Object constant.ContainerFile `json:"object,required"`
	// Path of the file in the container.
	Path string `json:"path,required"`
	// Source of the file (e.g., `user`, `assistant`).
	Source string `json:"source,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Bytes       respjson.Field
		ContainerID respjson.Field
		CreatedAt   respjson.Field
		Object      respjson.Field
		Path        respjson.Field
		Source      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ContainerFileNewResponse) RawJSON() string { return r.JSON.raw }
func (r *ContainerFileNewResponse) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ContainerFileGetResponse struct {
	// Unique identifier for the file.
	ID string `json:"id,required"`
	// Size of the file in bytes.
	Bytes int64 `json:"bytes,required"`
	// The container this file belongs to.
	ContainerID string `json:"container_id,required"`
	// Unix timestamp (in seconds) when the file was created.
	CreatedAt int64 `json:"created_at,required"`
	// The type of this object (`container.file`).
	Object constant.ContainerFile `json:"object,required"`
	// Path of the file in the container.
	Path string `json:"path,required"`
	// Source of the file (e.g., `user`, `assistant`).
	Source string `json:"source,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Bytes       respjson.Field
		ContainerID respjson.Field
		CreatedAt   respjson.Field
		Object      respjson.Field
		Path        respjson.Field
		Source      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ContainerFileGetResponse) RawJSON() string { return r.JSON.raw }
func (r *ContainerFileGetResponse) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ContainerFileListResponse struct {
	// Unique identifier for the file.
	ID string `json:"id,required"`
	// Size of the file in bytes.
	Bytes int64 `json:"bytes,required"`
	// The container this file belongs to.
	ContainerID string `json:"container_id,required"`
	// Unix timestamp (in seconds) when the file was created.
	CreatedAt int64 `json:"created_at,required"`
	// The type of this object (`container.file`).
	Object constant.ContainerFile `json:"object,required"`
	// Path of the file in the container.
	Path string `json:"path,required"`
	// Source of the file (e.g., `user`, `assistant`).
	Source string `json:"source,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Bytes       respjson.Field
		ContainerID respjson.Field
		CreatedAt   respjson.Field
		Object      respjson.Field
		Path        respjson.Field
		Source      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ContainerFileListResponse) RawJSON() string { return r.JSON.raw }
func (r *ContainerFileListResponse) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ContainerFileNewParams struct {
	// Name of the file to create.
	FileID param.Opt[string] `json:"file_id,omitzero"`
	// The File object (not file name) to be uploaded.
	File io.Reader `json:"file,omitzero" format:"binary"`
	paramObj
}

func (r ContainerFileNewParams) MarshalMultipart() (data []byte, contentType string, err error) {
	buf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buf)
	err = apiform.MarshalRoot(r, writer)
	if err == nil {
		err = apiform.WriteExtras(writer, r.ExtraFields())
	}
	if err != nil {
		writer.Close()
		return nil, "", err
	}
	err = writer.Close()
	if err != nil {
		return nil, "", err
	}
	return buf.Bytes(), writer.FormDataContentType(), nil
}

type ContainerFileListParams struct {
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
	Order ContainerFileListParamsOrder `query:"order,omitzero" json:"-"`
	paramObj
}

// URLQuery serializes [ContainerFileListParams]'s query parameters as
// `url.Values`.
func (r ContainerFileListParams) URLQuery() (v url.Values, err error) {
	return apiquery.MarshalWithSettings(r, apiquery.QuerySettings{
		ArrayFormat:  apiquery.ArrayQueryFormatBrackets,
		NestedFormat: apiquery.NestedQueryFormatBrackets,
	})
}

// Sort order by the `created_at` timestamp of the objects. `asc` for ascending
// order and `desc` for descending order.
type ContainerFileListParamsOrder string

const (
	ContainerFileListParamsOrderAsc  ContainerFileListParamsOrder = "asc"
	ContainerFileListParamsOrderDesc ContainerFileListParamsOrder = "desc"
)
