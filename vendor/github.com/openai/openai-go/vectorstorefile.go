// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package openai

import (
	"context"
	"encoding/json"
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
	"github.com/openai/openai-go/shared/constant"
)

// VectorStoreFileService contains methods and other services that help with
// interacting with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewVectorStoreFileService] method instead.
type VectorStoreFileService struct {
	Options []option.RequestOption
}

// NewVectorStoreFileService generates a new service that applies the given options
// to each request. These options are applied after the parent client's options (if
// there is one), and before any request-specific options.
func NewVectorStoreFileService(opts ...option.RequestOption) (r VectorStoreFileService) {
	r = VectorStoreFileService{}
	r.Options = opts
	return
}

// Create a vector store file by attaching a
// [File](https://platform.openai.com/docs/api-reference/files) to a
// [vector store](https://platform.openai.com/docs/api-reference/vector-stores/object).
func (r *VectorStoreFileService) New(ctx context.Context, vectorStoreID string, body VectorStoreFileNewParams, opts ...option.RequestOption) (res *VectorStoreFile, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	if vectorStoreID == "" {
		err = errors.New("missing required vector_store_id parameter")
		return
	}
	path := fmt.Sprintf("vector_stores/%s/files", vectorStoreID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// Create a vector store file by attaching a
// [File](https://platform.openai.com/docs/api-reference/files) to a
// [vector store](https://platform.openai.com/docs/api-reference/vector-stores/object).
//
// Polls the API and blocks until the task is complete.
// Default polling interval is 1 second.
func (r *VectorStoreFileService) NewAndPoll(ctx context.Context, vectorStoreId string, body VectorStoreFileNewParams, pollIntervalMs int, opts ...option.RequestOption) (res *VectorStoreFile, err error) {
	file, err := r.New(ctx, vectorStoreId, body, opts...)
	if err != nil {
		return nil, err
	}
	return r.PollStatus(ctx, vectorStoreId, file.ID, pollIntervalMs, opts...)
}

// Upload a file to the `files` API and then attach it to the given vector store.
//
// Note the file will be asynchronously processed (you can use the alternative
// polling helper method to wait for processing to complete).
func (r *VectorStoreFileService) Upload(ctx context.Context, vectorStoreID string, body FileNewParams, opts ...option.RequestOption) (*VectorStoreFile, error) {
	filesService := NewFileService(r.Options...)
	fileObj, err := filesService.New(ctx, body, opts...)
	if err != nil {
		return nil, err
	}
	return r.New(ctx, vectorStoreID, VectorStoreFileNewParams{
		FileID: fileObj.ID,
	}, opts...)
}

// Add a file to a vector store and poll until processing is complete.
// Default polling interval is 1 second.
func (r *VectorStoreFileService) UploadAndPoll(ctx context.Context, vectorStoreID string, body FileNewParams, pollIntervalMs int, opts ...option.RequestOption) (*VectorStoreFile, error) {
	res, err := r.Upload(ctx, vectorStoreID, body, opts...)
	if err != nil {
		return nil, err
	}
	return r.PollStatus(ctx, vectorStoreID, res.ID, pollIntervalMs, opts...)
}

// Retrieves a vector store file.
func (r *VectorStoreFileService) Get(ctx context.Context, vectorStoreID string, fileID string, opts ...option.RequestOption) (res *VectorStoreFile, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	if vectorStoreID == "" {
		err = errors.New("missing required vector_store_id parameter")
		return
	}
	if fileID == "" {
		err = errors.New("missing required file_id parameter")
		return
	}
	path := fmt.Sprintf("vector_stores/%s/files/%s", vectorStoreID, fileID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// Update attributes on a vector store file.
func (r *VectorStoreFileService) Update(ctx context.Context, vectorStoreID string, fileID string, body VectorStoreFileUpdateParams, opts ...option.RequestOption) (res *VectorStoreFile, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	if vectorStoreID == "" {
		err = errors.New("missing required vector_store_id parameter")
		return
	}
	if fileID == "" {
		err = errors.New("missing required file_id parameter")
		return
	}
	path := fmt.Sprintf("vector_stores/%s/files/%s", vectorStoreID, fileID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// Returns a list of vector store files.
func (r *VectorStoreFileService) List(ctx context.Context, vectorStoreID string, query VectorStoreFileListParams, opts ...option.RequestOption) (res *pagination.CursorPage[VectorStoreFile], err error) {
	var raw *http.Response
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2"), option.WithResponseInto(&raw)}, opts...)
	if vectorStoreID == "" {
		err = errors.New("missing required vector_store_id parameter")
		return
	}
	path := fmt.Sprintf("vector_stores/%s/files", vectorStoreID)
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

// Returns a list of vector store files.
func (r *VectorStoreFileService) ListAutoPaging(ctx context.Context, vectorStoreID string, query VectorStoreFileListParams, opts ...option.RequestOption) *pagination.CursorPageAutoPager[VectorStoreFile] {
	return pagination.NewCursorPageAutoPager(r.List(ctx, vectorStoreID, query, opts...))
}

// Delete a vector store file. This will remove the file from the vector store but
// the file itself will not be deleted. To delete the file, use the
// [delete file](https://platform.openai.com/docs/api-reference/files/delete)
// endpoint.
func (r *VectorStoreFileService) Delete(ctx context.Context, vectorStoreID string, fileID string, opts ...option.RequestOption) (res *VectorStoreFileDeleted, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	if vectorStoreID == "" {
		err = errors.New("missing required vector_store_id parameter")
		return
	}
	if fileID == "" {
		err = errors.New("missing required file_id parameter")
		return
	}
	path := fmt.Sprintf("vector_stores/%s/files/%s", vectorStoreID, fileID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodDelete, path, nil, &res, opts...)
	return
}

// Retrieve the parsed contents of a vector store file.
func (r *VectorStoreFileService) Content(ctx context.Context, vectorStoreID string, fileID string, opts ...option.RequestOption) (res *pagination.Page[VectorStoreFileContentResponse], err error) {
	var raw *http.Response
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2"), option.WithResponseInto(&raw)}, opts...)
	if vectorStoreID == "" {
		err = errors.New("missing required vector_store_id parameter")
		return
	}
	if fileID == "" {
		err = errors.New("missing required file_id parameter")
		return
	}
	path := fmt.Sprintf("vector_stores/%s/files/%s/content", vectorStoreID, fileID)
	cfg, err := requestconfig.NewRequestConfig(ctx, http.MethodGet, path, nil, &res, opts...)
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

// Retrieve the parsed contents of a vector store file.
func (r *VectorStoreFileService) ContentAutoPaging(ctx context.Context, vectorStoreID string, fileID string, opts ...option.RequestOption) *pagination.PageAutoPager[VectorStoreFileContentResponse] {
	return pagination.NewPageAutoPager(r.Content(ctx, vectorStoreID, fileID, opts...))
}

// A list of files attached to a vector store.
type VectorStoreFile struct {
	// The identifier, which can be referenced in API endpoints.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) for when the vector store file was created.
	CreatedAt int64 `json:"created_at,required"`
	// The last error associated with this vector store file. Will be `null` if there
	// are no errors.
	LastError VectorStoreFileLastError `json:"last_error,required"`
	// The object type, which is always `vector_store.file`.
	Object constant.VectorStoreFile `json:"object,required"`
	// The status of the vector store file, which can be either `in_progress`,
	// `completed`, `cancelled`, or `failed`. The status `completed` indicates that the
	// vector store file is ready for use.
	//
	// Any of "in_progress", "completed", "cancelled", "failed".
	Status VectorStoreFileStatus `json:"status,required"`
	// The total vector store usage in bytes. Note that this may be different from the
	// original file size.
	UsageBytes int64 `json:"usage_bytes,required"`
	// The ID of the
	// [vector store](https://platform.openai.com/docs/api-reference/vector-stores/object)
	// that the [File](https://platform.openai.com/docs/api-reference/files) is
	// attached to.
	VectorStoreID string `json:"vector_store_id,required"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard. Keys are strings with a maximum
	// length of 64 characters. Values are strings with a maximum length of 512
	// characters, booleans, or numbers.
	Attributes map[string]VectorStoreFileAttributeUnion `json:"attributes,nullable"`
	// The strategy used to chunk the file.
	ChunkingStrategy FileChunkingStrategyUnion `json:"chunking_strategy"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID               respjson.Field
		CreatedAt        respjson.Field
		LastError        respjson.Field
		Object           respjson.Field
		Status           respjson.Field
		UsageBytes       respjson.Field
		VectorStoreID    respjson.Field
		Attributes       respjson.Field
		ChunkingStrategy respjson.Field
		ExtraFields      map[string]respjson.Field
		raw              string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r VectorStoreFile) RawJSON() string { return r.JSON.raw }
func (r *VectorStoreFile) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The last error associated with this vector store file. Will be `null` if there
// are no errors.
type VectorStoreFileLastError struct {
	// One of `server_error` or `rate_limit_exceeded`.
	//
	// Any of "server_error", "unsupported_file", "invalid_file".
	Code string `json:"code,required"`
	// A human-readable description of the error.
	Message string `json:"message,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Code        respjson.Field
		Message     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r VectorStoreFileLastError) RawJSON() string { return r.JSON.raw }
func (r *VectorStoreFileLastError) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The status of the vector store file, which can be either `in_progress`,
// `completed`, `cancelled`, or `failed`. The status `completed` indicates that the
// vector store file is ready for use.
type VectorStoreFileStatus string

const (
	VectorStoreFileStatusInProgress VectorStoreFileStatus = "in_progress"
	VectorStoreFileStatusCompleted  VectorStoreFileStatus = "completed"
	VectorStoreFileStatusCancelled  VectorStoreFileStatus = "cancelled"
	VectorStoreFileStatusFailed     VectorStoreFileStatus = "failed"
)

// VectorStoreFileAttributeUnion contains all possible properties and values from
// [string], [float64], [bool].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfString OfFloat OfBool]
type VectorStoreFileAttributeUnion struct {
	// This field will be present if the value is a [string] instead of an object.
	OfString string `json:",inline"`
	// This field will be present if the value is a [float64] instead of an object.
	OfFloat float64 `json:",inline"`
	// This field will be present if the value is a [bool] instead of an object.
	OfBool bool `json:",inline"`
	JSON   struct {
		OfString respjson.Field
		OfFloat  respjson.Field
		OfBool   respjson.Field
		raw      string
	} `json:"-"`
}

func (u VectorStoreFileAttributeUnion) AsString() (v string) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u VectorStoreFileAttributeUnion) AsFloat() (v float64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u VectorStoreFileAttributeUnion) AsBool() (v bool) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u VectorStoreFileAttributeUnion) RawJSON() string { return u.JSON.raw }

func (r *VectorStoreFileAttributeUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type VectorStoreFileDeleted struct {
	ID      string                          `json:"id,required"`
	Deleted bool                            `json:"deleted,required"`
	Object  constant.VectorStoreFileDeleted `json:"object,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Deleted     respjson.Field
		Object      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r VectorStoreFileDeleted) RawJSON() string { return r.JSON.raw }
func (r *VectorStoreFileDeleted) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type VectorStoreFileContentResponse struct {
	// The text content
	Text string `json:"text"`
	// The content type (currently only `"text"`)
	Type string `json:"type"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Text        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r VectorStoreFileContentResponse) RawJSON() string { return r.JSON.raw }
func (r *VectorStoreFileContentResponse) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type VectorStoreFileNewParams struct {
	// A [File](https://platform.openai.com/docs/api-reference/files) ID that the
	// vector store should use. Useful for tools like `file_search` that can access
	// files.
	FileID string `json:"file_id,required"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard. Keys are strings with a maximum
	// length of 64 characters. Values are strings with a maximum length of 512
	// characters, booleans, or numbers.
	Attributes map[string]VectorStoreFileNewParamsAttributeUnion `json:"attributes,omitzero"`
	// The chunking strategy used to chunk the file(s). If not set, will use the `auto`
	// strategy. Only applicable if `file_ids` is non-empty.
	ChunkingStrategy FileChunkingStrategyParamUnion `json:"chunking_strategy,omitzero"`
	paramObj
}

func (r VectorStoreFileNewParams) MarshalJSON() (data []byte, err error) {
	type shadow VectorStoreFileNewParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *VectorStoreFileNewParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type VectorStoreFileNewParamsAttributeUnion struct {
	OfString param.Opt[string]  `json:",omitzero,inline"`
	OfFloat  param.Opt[float64] `json:",omitzero,inline"`
	OfBool   param.Opt[bool]    `json:",omitzero,inline"`
	paramUnion
}

func (u VectorStoreFileNewParamsAttributeUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfFloat, u.OfBool)
}
func (u *VectorStoreFileNewParamsAttributeUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *VectorStoreFileNewParamsAttributeUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfFloat) {
		return &u.OfFloat.Value
	} else if !param.IsOmitted(u.OfBool) {
		return &u.OfBool.Value
	}
	return nil
}

type VectorStoreFileUpdateParams struct {
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard. Keys are strings with a maximum
	// length of 64 characters. Values are strings with a maximum length of 512
	// characters, booleans, or numbers.
	Attributes map[string]VectorStoreFileUpdateParamsAttributeUnion `json:"attributes,omitzero,required"`
	paramObj
}

func (r VectorStoreFileUpdateParams) MarshalJSON() (data []byte, err error) {
	type shadow VectorStoreFileUpdateParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *VectorStoreFileUpdateParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type VectorStoreFileUpdateParamsAttributeUnion struct {
	OfString param.Opt[string]  `json:",omitzero,inline"`
	OfFloat  param.Opt[float64] `json:",omitzero,inline"`
	OfBool   param.Opt[bool]    `json:",omitzero,inline"`
	paramUnion
}

func (u VectorStoreFileUpdateParamsAttributeUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfFloat, u.OfBool)
}
func (u *VectorStoreFileUpdateParamsAttributeUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *VectorStoreFileUpdateParamsAttributeUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfFloat) {
		return &u.OfFloat.Value
	} else if !param.IsOmitted(u.OfBool) {
		return &u.OfBool.Value
	}
	return nil
}

type VectorStoreFileListParams struct {
	// A cursor for use in pagination. `after` is an object ID that defines your place
	// in the list. For instance, if you make a list request and receive 100 objects,
	// ending with obj_foo, your subsequent call can include after=obj_foo in order to
	// fetch the next page of the list.
	After param.Opt[string] `query:"after,omitzero" json:"-"`
	// A cursor for use in pagination. `before` is an object ID that defines your place
	// in the list. For instance, if you make a list request and receive 100 objects,
	// starting with obj_foo, your subsequent call can include before=obj_foo in order
	// to fetch the previous page of the list.
	Before param.Opt[string] `query:"before,omitzero" json:"-"`
	// A limit on the number of objects to be returned. Limit can range between 1 and
	// 100, and the default is 20.
	Limit param.Opt[int64] `query:"limit,omitzero" json:"-"`
	// Filter by file status. One of `in_progress`, `completed`, `failed`, `cancelled`.
	//
	// Any of "in_progress", "completed", "failed", "cancelled".
	Filter VectorStoreFileListParamsFilter `query:"filter,omitzero" json:"-"`
	// Sort order by the `created_at` timestamp of the objects. `asc` for ascending
	// order and `desc` for descending order.
	//
	// Any of "asc", "desc".
	Order VectorStoreFileListParamsOrder `query:"order,omitzero" json:"-"`
	paramObj
}

// URLQuery serializes [VectorStoreFileListParams]'s query parameters as
// `url.Values`.
func (r VectorStoreFileListParams) URLQuery() (v url.Values, err error) {
	return apiquery.MarshalWithSettings(r, apiquery.QuerySettings{
		ArrayFormat:  apiquery.ArrayQueryFormatBrackets,
		NestedFormat: apiquery.NestedQueryFormatBrackets,
	})
}

// Filter by file status. One of `in_progress`, `completed`, `failed`, `cancelled`.
type VectorStoreFileListParamsFilter string

const (
	VectorStoreFileListParamsFilterInProgress VectorStoreFileListParamsFilter = "in_progress"
	VectorStoreFileListParamsFilterCompleted  VectorStoreFileListParamsFilter = "completed"
	VectorStoreFileListParamsFilterFailed     VectorStoreFileListParamsFilter = "failed"
	VectorStoreFileListParamsFilterCancelled  VectorStoreFileListParamsFilter = "cancelled"
)

// Sort order by the `created_at` timestamp of the objects. `asc` for ascending
// order and `desc` for descending order.
type VectorStoreFileListParamsOrder string

const (
	VectorStoreFileListParamsOrderAsc  VectorStoreFileListParamsOrder = "asc"
	VectorStoreFileListParamsOrderDesc VectorStoreFileListParamsOrder = "desc"
)
