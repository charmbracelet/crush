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
	"github.com/openai/openai-go/shared"
	"github.com/openai/openai-go/shared/constant"
)

// VectorStoreService contains methods and other services that help with
// interacting with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewVectorStoreService] method instead.
type VectorStoreService struct {
	Options     []option.RequestOption
	Files       VectorStoreFileService
	FileBatches VectorStoreFileBatchService
}

// NewVectorStoreService generates a new service that applies the given options to
// each request. These options are applied after the parent client's options (if
// there is one), and before any request-specific options.
func NewVectorStoreService(opts ...option.RequestOption) (r VectorStoreService) {
	r = VectorStoreService{}
	r.Options = opts
	r.Files = NewVectorStoreFileService(opts...)
	r.FileBatches = NewVectorStoreFileBatchService(opts...)
	return
}

// Create a vector store.
func (r *VectorStoreService) New(ctx context.Context, body VectorStoreNewParams, opts ...option.RequestOption) (res *VectorStore, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	path := "vector_stores"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// Retrieves a vector store.
func (r *VectorStoreService) Get(ctx context.Context, vectorStoreID string, opts ...option.RequestOption) (res *VectorStore, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	if vectorStoreID == "" {
		err = errors.New("missing required vector_store_id parameter")
		return
	}
	path := fmt.Sprintf("vector_stores/%s", vectorStoreID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// Modifies a vector store.
func (r *VectorStoreService) Update(ctx context.Context, vectorStoreID string, body VectorStoreUpdateParams, opts ...option.RequestOption) (res *VectorStore, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	if vectorStoreID == "" {
		err = errors.New("missing required vector_store_id parameter")
		return
	}
	path := fmt.Sprintf("vector_stores/%s", vectorStoreID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// Returns a list of vector stores.
func (r *VectorStoreService) List(ctx context.Context, query VectorStoreListParams, opts ...option.RequestOption) (res *pagination.CursorPage[VectorStore], err error) {
	var raw *http.Response
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2"), option.WithResponseInto(&raw)}, opts...)
	path := "vector_stores"
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

// Returns a list of vector stores.
func (r *VectorStoreService) ListAutoPaging(ctx context.Context, query VectorStoreListParams, opts ...option.RequestOption) *pagination.CursorPageAutoPager[VectorStore] {
	return pagination.NewCursorPageAutoPager(r.List(ctx, query, opts...))
}

// Delete a vector store.
func (r *VectorStoreService) Delete(ctx context.Context, vectorStoreID string, opts ...option.RequestOption) (res *VectorStoreDeleted, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	if vectorStoreID == "" {
		err = errors.New("missing required vector_store_id parameter")
		return
	}
	path := fmt.Sprintf("vector_stores/%s", vectorStoreID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodDelete, path, nil, &res, opts...)
	return
}

// Search a vector store for relevant chunks based on a query and file attributes
// filter.
func (r *VectorStoreService) Search(ctx context.Context, vectorStoreID string, body VectorStoreSearchParams, opts ...option.RequestOption) (res *pagination.Page[VectorStoreSearchResponse], err error) {
	var raw *http.Response
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2"), option.WithResponseInto(&raw)}, opts...)
	if vectorStoreID == "" {
		err = errors.New("missing required vector_store_id parameter")
		return
	}
	path := fmt.Sprintf("vector_stores/%s/search", vectorStoreID)
	cfg, err := requestconfig.NewRequestConfig(ctx, http.MethodPost, path, body, &res, opts...)
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

// Search a vector store for relevant chunks based on a query and file attributes
// filter.
func (r *VectorStoreService) SearchAutoPaging(ctx context.Context, vectorStoreID string, body VectorStoreSearchParams, opts ...option.RequestOption) *pagination.PageAutoPager[VectorStoreSearchResponse] {
	return pagination.NewPageAutoPager(r.Search(ctx, vectorStoreID, body, opts...))
}

func NewAutoFileChunkingStrategyParam() AutoFileChunkingStrategyParam {
	return AutoFileChunkingStrategyParam{
		Type: "auto",
	}
}

// The default strategy. This strategy currently uses a `max_chunk_size_tokens` of
// `800` and `chunk_overlap_tokens` of `400`.
//
// This struct has a constant value, construct it with
// [NewAutoFileChunkingStrategyParam].
type AutoFileChunkingStrategyParam struct {
	// Always `auto`.
	Type constant.Auto `json:"type,required"`
	paramObj
}

func (r AutoFileChunkingStrategyParam) MarshalJSON() (data []byte, err error) {
	type shadow AutoFileChunkingStrategyParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *AutoFileChunkingStrategyParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// FileChunkingStrategyUnion contains all possible properties and values from
// [StaticFileChunkingStrategyObject], [OtherFileChunkingStrategyObject].
//
// Use the [FileChunkingStrategyUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type FileChunkingStrategyUnion struct {
	// This field is from variant [StaticFileChunkingStrategyObject].
	Static StaticFileChunkingStrategy `json:"static"`
	// Any of "static", "other".
	Type string `json:"type"`
	JSON struct {
		Static respjson.Field
		Type   respjson.Field
		raw    string
	} `json:"-"`
}

// anyFileChunkingStrategy is implemented by each variant of
// [FileChunkingStrategyUnion] to add type safety for the return type of
// [FileChunkingStrategyUnion.AsAny]
type anyFileChunkingStrategy interface {
	implFileChunkingStrategyUnion()
}

func (StaticFileChunkingStrategyObject) implFileChunkingStrategyUnion() {}
func (OtherFileChunkingStrategyObject) implFileChunkingStrategyUnion()  {}

// Use the following switch statement to find the correct variant
//
//	switch variant := FileChunkingStrategyUnion.AsAny().(type) {
//	case openai.StaticFileChunkingStrategyObject:
//	case openai.OtherFileChunkingStrategyObject:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u FileChunkingStrategyUnion) AsAny() anyFileChunkingStrategy {
	switch u.Type {
	case "static":
		return u.AsStatic()
	case "other":
		return u.AsOther()
	}
	return nil
}

func (u FileChunkingStrategyUnion) AsStatic() (v StaticFileChunkingStrategyObject) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u FileChunkingStrategyUnion) AsOther() (v OtherFileChunkingStrategyObject) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u FileChunkingStrategyUnion) RawJSON() string { return u.JSON.raw }

func (r *FileChunkingStrategyUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func FileChunkingStrategyParamOfStatic(static StaticFileChunkingStrategyParam) FileChunkingStrategyParamUnion {
	var variant StaticFileChunkingStrategyObjectParam
	variant.Static = static
	return FileChunkingStrategyParamUnion{OfStatic: &variant}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type FileChunkingStrategyParamUnion struct {
	OfAuto   *AutoFileChunkingStrategyParam         `json:",omitzero,inline"`
	OfStatic *StaticFileChunkingStrategyObjectParam `json:",omitzero,inline"`
	paramUnion
}

func (u FileChunkingStrategyParamUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfStatic)
}
func (u *FileChunkingStrategyParamUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *FileChunkingStrategyParamUnion) asAny() any {
	if !param.IsOmitted(u.OfAuto) {
		return u.OfAuto
	} else if !param.IsOmitted(u.OfStatic) {
		return u.OfStatic
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FileChunkingStrategyParamUnion) GetStatic() *StaticFileChunkingStrategyParam {
	if vt := u.OfStatic; vt != nil {
		return &vt.Static
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FileChunkingStrategyParamUnion) GetType() *string {
	if vt := u.OfAuto; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfStatic; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[FileChunkingStrategyParamUnion](
		"type",
		apijson.Discriminator[AutoFileChunkingStrategyParam]("auto"),
		apijson.Discriminator[StaticFileChunkingStrategyObjectParam]("static"),
	)
}

// This is returned when the chunking strategy is unknown. Typically, this is
// because the file was indexed before the `chunking_strategy` concept was
// introduced in the API.
type OtherFileChunkingStrategyObject struct {
	// Always `other`.
	Type constant.Other `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r OtherFileChunkingStrategyObject) RawJSON() string { return r.JSON.raw }
func (r *OtherFileChunkingStrategyObject) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type StaticFileChunkingStrategy struct {
	// The number of tokens that overlap between chunks. The default value is `400`.
	//
	// Note that the overlap must not exceed half of `max_chunk_size_tokens`.
	ChunkOverlapTokens int64 `json:"chunk_overlap_tokens,required"`
	// The maximum number of tokens in each chunk. The default value is `800`. The
	// minimum value is `100` and the maximum value is `4096`.
	MaxChunkSizeTokens int64 `json:"max_chunk_size_tokens,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ChunkOverlapTokens respjson.Field
		MaxChunkSizeTokens respjson.Field
		ExtraFields        map[string]respjson.Field
		raw                string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r StaticFileChunkingStrategy) RawJSON() string { return r.JSON.raw }
func (r *StaticFileChunkingStrategy) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this StaticFileChunkingStrategy to a
// StaticFileChunkingStrategyParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// StaticFileChunkingStrategyParam.Overrides()
func (r StaticFileChunkingStrategy) ToParam() StaticFileChunkingStrategyParam {
	return param.Override[StaticFileChunkingStrategyParam](json.RawMessage(r.RawJSON()))
}

// The properties ChunkOverlapTokens, MaxChunkSizeTokens are required.
type StaticFileChunkingStrategyParam struct {
	// The number of tokens that overlap between chunks. The default value is `400`.
	//
	// Note that the overlap must not exceed half of `max_chunk_size_tokens`.
	ChunkOverlapTokens int64 `json:"chunk_overlap_tokens,required"`
	// The maximum number of tokens in each chunk. The default value is `800`. The
	// minimum value is `100` and the maximum value is `4096`.
	MaxChunkSizeTokens int64 `json:"max_chunk_size_tokens,required"`
	paramObj
}

func (r StaticFileChunkingStrategyParam) MarshalJSON() (data []byte, err error) {
	type shadow StaticFileChunkingStrategyParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *StaticFileChunkingStrategyParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type StaticFileChunkingStrategyObject struct {
	Static StaticFileChunkingStrategy `json:"static,required"`
	// Always `static`.
	Type constant.Static `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Static      respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r StaticFileChunkingStrategyObject) RawJSON() string { return r.JSON.raw }
func (r *StaticFileChunkingStrategyObject) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Customize your own chunking strategy by setting chunk size and chunk overlap.
//
// The properties Static, Type are required.
type StaticFileChunkingStrategyObjectParam struct {
	Static StaticFileChunkingStrategyParam `json:"static,omitzero,required"`
	// Always `static`.
	//
	// This field can be elided, and will marshal its zero value as "static".
	Type constant.Static `json:"type,required"`
	paramObj
}

func (r StaticFileChunkingStrategyObjectParam) MarshalJSON() (data []byte, err error) {
	type shadow StaticFileChunkingStrategyObjectParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *StaticFileChunkingStrategyObjectParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A vector store is a collection of processed files can be used by the
// `file_search` tool.
type VectorStore struct {
	// The identifier, which can be referenced in API endpoints.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) for when the vector store was created.
	CreatedAt  int64                 `json:"created_at,required"`
	FileCounts VectorStoreFileCounts `json:"file_counts,required"`
	// The Unix timestamp (in seconds) for when the vector store was last active.
	LastActiveAt int64 `json:"last_active_at,required"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,required"`
	// The name of the vector store.
	Name string `json:"name,required"`
	// The object type, which is always `vector_store`.
	Object constant.VectorStore `json:"object,required"`
	// The status of the vector store, which can be either `expired`, `in_progress`, or
	// `completed`. A status of `completed` indicates that the vector store is ready
	// for use.
	//
	// Any of "expired", "in_progress", "completed".
	Status VectorStoreStatus `json:"status,required"`
	// The total number of bytes used by the files in the vector store.
	UsageBytes int64 `json:"usage_bytes,required"`
	// The expiration policy for a vector store.
	ExpiresAfter VectorStoreExpiresAfter `json:"expires_after"`
	// The Unix timestamp (in seconds) for when the vector store will expire.
	ExpiresAt int64 `json:"expires_at,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID           respjson.Field
		CreatedAt    respjson.Field
		FileCounts   respjson.Field
		LastActiveAt respjson.Field
		Metadata     respjson.Field
		Name         respjson.Field
		Object       respjson.Field
		Status       respjson.Field
		UsageBytes   respjson.Field
		ExpiresAfter respjson.Field
		ExpiresAt    respjson.Field
		ExtraFields  map[string]respjson.Field
		raw          string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r VectorStore) RawJSON() string { return r.JSON.raw }
func (r *VectorStore) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type VectorStoreFileCounts struct {
	// The number of files that were cancelled.
	Cancelled int64 `json:"cancelled,required"`
	// The number of files that have been successfully processed.
	Completed int64 `json:"completed,required"`
	// The number of files that have failed to process.
	Failed int64 `json:"failed,required"`
	// The number of files that are currently being processed.
	InProgress int64 `json:"in_progress,required"`
	// The total number of files.
	Total int64 `json:"total,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Cancelled   respjson.Field
		Completed   respjson.Field
		Failed      respjson.Field
		InProgress  respjson.Field
		Total       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r VectorStoreFileCounts) RawJSON() string { return r.JSON.raw }
func (r *VectorStoreFileCounts) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The status of the vector store, which can be either `expired`, `in_progress`, or
// `completed`. A status of `completed` indicates that the vector store is ready
// for use.
type VectorStoreStatus string

const (
	VectorStoreStatusExpired    VectorStoreStatus = "expired"
	VectorStoreStatusInProgress VectorStoreStatus = "in_progress"
	VectorStoreStatusCompleted  VectorStoreStatus = "completed"
)

// The expiration policy for a vector store.
type VectorStoreExpiresAfter struct {
	// Anchor timestamp after which the expiration policy applies. Supported anchors:
	// `last_active_at`.
	Anchor constant.LastActiveAt `json:"anchor,required"`
	// The number of days after the anchor time that the vector store will expire.
	Days int64 `json:"days,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Anchor      respjson.Field
		Days        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r VectorStoreExpiresAfter) RawJSON() string { return r.JSON.raw }
func (r *VectorStoreExpiresAfter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type VectorStoreDeleted struct {
	ID      string                      `json:"id,required"`
	Deleted bool                        `json:"deleted,required"`
	Object  constant.VectorStoreDeleted `json:"object,required"`
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
func (r VectorStoreDeleted) RawJSON() string { return r.JSON.raw }
func (r *VectorStoreDeleted) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type VectorStoreSearchResponse struct {
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard. Keys are strings with a maximum
	// length of 64 characters. Values are strings with a maximum length of 512
	// characters, booleans, or numbers.
	Attributes map[string]VectorStoreSearchResponseAttributeUnion `json:"attributes,required"`
	// Content chunks from the file.
	Content []VectorStoreSearchResponseContent `json:"content,required"`
	// The ID of the vector store file.
	FileID string `json:"file_id,required"`
	// The name of the vector store file.
	Filename string `json:"filename,required"`
	// The similarity score for the result.
	Score float64 `json:"score,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Attributes  respjson.Field
		Content     respjson.Field
		FileID      respjson.Field
		Filename    respjson.Field
		Score       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r VectorStoreSearchResponse) RawJSON() string { return r.JSON.raw }
func (r *VectorStoreSearchResponse) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// VectorStoreSearchResponseAttributeUnion contains all possible properties and
// values from [string], [float64], [bool].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfString OfFloat OfBool]
type VectorStoreSearchResponseAttributeUnion struct {
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

func (u VectorStoreSearchResponseAttributeUnion) AsString() (v string) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u VectorStoreSearchResponseAttributeUnion) AsFloat() (v float64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u VectorStoreSearchResponseAttributeUnion) AsBool() (v bool) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u VectorStoreSearchResponseAttributeUnion) RawJSON() string { return u.JSON.raw }

func (r *VectorStoreSearchResponseAttributeUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type VectorStoreSearchResponseContent struct {
	// The text content returned from search.
	Text string `json:"text,required"`
	// The type of content.
	//
	// Any of "text".
	Type string `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Text        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r VectorStoreSearchResponseContent) RawJSON() string { return r.JSON.raw }
func (r *VectorStoreSearchResponseContent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type VectorStoreNewParams struct {
	// The name of the vector store.
	Name param.Opt[string] `json:"name,omitzero"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,omitzero"`
	// The chunking strategy used to chunk the file(s). If not set, will use the `auto`
	// strategy. Only applicable if `file_ids` is non-empty.
	ChunkingStrategy FileChunkingStrategyParamUnion `json:"chunking_strategy,omitzero"`
	// The expiration policy for a vector store.
	ExpiresAfter VectorStoreNewParamsExpiresAfter `json:"expires_after,omitzero"`
	// A list of [File](https://platform.openai.com/docs/api-reference/files) IDs that
	// the vector store should use. Useful for tools like `file_search` that can access
	// files.
	FileIDs []string `json:"file_ids,omitzero"`
	paramObj
}

func (r VectorStoreNewParams) MarshalJSON() (data []byte, err error) {
	type shadow VectorStoreNewParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *VectorStoreNewParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The expiration policy for a vector store.
//
// The properties Anchor, Days are required.
type VectorStoreNewParamsExpiresAfter struct {
	// The number of days after the anchor time that the vector store will expire.
	Days int64 `json:"days,required"`
	// Anchor timestamp after which the expiration policy applies. Supported anchors:
	// `last_active_at`.
	//
	// This field can be elided, and will marshal its zero value as "last_active_at".
	Anchor constant.LastActiveAt `json:"anchor,required"`
	paramObj
}

func (r VectorStoreNewParamsExpiresAfter) MarshalJSON() (data []byte, err error) {
	type shadow VectorStoreNewParamsExpiresAfter
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *VectorStoreNewParamsExpiresAfter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type VectorStoreUpdateParams struct {
	// The name of the vector store.
	Name param.Opt[string] `json:"name,omitzero"`
	// The expiration policy for a vector store.
	ExpiresAfter VectorStoreUpdateParamsExpiresAfter `json:"expires_after,omitzero"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,omitzero"`
	paramObj
}

func (r VectorStoreUpdateParams) MarshalJSON() (data []byte, err error) {
	type shadow VectorStoreUpdateParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *VectorStoreUpdateParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The expiration policy for a vector store.
//
// The properties Anchor, Days are required.
type VectorStoreUpdateParamsExpiresAfter struct {
	// The number of days after the anchor time that the vector store will expire.
	Days int64 `json:"days,required"`
	// Anchor timestamp after which the expiration policy applies. Supported anchors:
	// `last_active_at`.
	//
	// This field can be elided, and will marshal its zero value as "last_active_at".
	Anchor constant.LastActiveAt `json:"anchor,required"`
	paramObj
}

func (r VectorStoreUpdateParamsExpiresAfter) MarshalJSON() (data []byte, err error) {
	type shadow VectorStoreUpdateParamsExpiresAfter
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *VectorStoreUpdateParamsExpiresAfter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type VectorStoreListParams struct {
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
	// Sort order by the `created_at` timestamp of the objects. `asc` for ascending
	// order and `desc` for descending order.
	//
	// Any of "asc", "desc".
	Order VectorStoreListParamsOrder `query:"order,omitzero" json:"-"`
	paramObj
}

// URLQuery serializes [VectorStoreListParams]'s query parameters as `url.Values`.
func (r VectorStoreListParams) URLQuery() (v url.Values, err error) {
	return apiquery.MarshalWithSettings(r, apiquery.QuerySettings{
		ArrayFormat:  apiquery.ArrayQueryFormatBrackets,
		NestedFormat: apiquery.NestedQueryFormatBrackets,
	})
}

// Sort order by the `created_at` timestamp of the objects. `asc` for ascending
// order and `desc` for descending order.
type VectorStoreListParamsOrder string

const (
	VectorStoreListParamsOrderAsc  VectorStoreListParamsOrder = "asc"
	VectorStoreListParamsOrderDesc VectorStoreListParamsOrder = "desc"
)

type VectorStoreSearchParams struct {
	// A query string for a search
	Query VectorStoreSearchParamsQueryUnion `json:"query,omitzero,required"`
	// The maximum number of results to return. This number should be between 1 and 50
	// inclusive.
	MaxNumResults param.Opt[int64] `json:"max_num_results,omitzero"`
	// Whether to rewrite the natural language query for vector search.
	RewriteQuery param.Opt[bool] `json:"rewrite_query,omitzero"`
	// A filter to apply based on file attributes.
	Filters VectorStoreSearchParamsFiltersUnion `json:"filters,omitzero"`
	// Ranking options for search.
	RankingOptions VectorStoreSearchParamsRankingOptions `json:"ranking_options,omitzero"`
	paramObj
}

func (r VectorStoreSearchParams) MarshalJSON() (data []byte, err error) {
	type shadow VectorStoreSearchParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *VectorStoreSearchParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type VectorStoreSearchParamsQueryUnion struct {
	OfString      param.Opt[string] `json:",omitzero,inline"`
	OfStringArray []string          `json:",omitzero,inline"`
	paramUnion
}

func (u VectorStoreSearchParamsQueryUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfStringArray)
}
func (u *VectorStoreSearchParamsQueryUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *VectorStoreSearchParamsQueryUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfStringArray) {
		return &u.OfStringArray
	}
	return nil
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type VectorStoreSearchParamsFiltersUnion struct {
	OfComparisonFilter *shared.ComparisonFilterParam `json:",omitzero,inline"`
	OfCompoundFilter   *shared.CompoundFilterParam   `json:",omitzero,inline"`
	paramUnion
}

func (u VectorStoreSearchParamsFiltersUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfComparisonFilter, u.OfCompoundFilter)
}
func (u *VectorStoreSearchParamsFiltersUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *VectorStoreSearchParamsFiltersUnion) asAny() any {
	if !param.IsOmitted(u.OfComparisonFilter) {
		return u.OfComparisonFilter
	} else if !param.IsOmitted(u.OfCompoundFilter) {
		return u.OfCompoundFilter
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u VectorStoreSearchParamsFiltersUnion) GetKey() *string {
	if vt := u.OfComparisonFilter; vt != nil {
		return &vt.Key
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u VectorStoreSearchParamsFiltersUnion) GetValue() *shared.ComparisonFilterValueUnionParam {
	if vt := u.OfComparisonFilter; vt != nil {
		return &vt.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u VectorStoreSearchParamsFiltersUnion) GetFilters() []shared.ComparisonFilterParam {
	if vt := u.OfCompoundFilter; vt != nil {
		return vt.Filters
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u VectorStoreSearchParamsFiltersUnion) GetType() *string {
	if vt := u.OfComparisonFilter; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfCompoundFilter; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Ranking options for search.
type VectorStoreSearchParamsRankingOptions struct {
	ScoreThreshold param.Opt[float64] `json:"score_threshold,omitzero"`
	// Any of "auto", "default-2024-11-15".
	Ranker string `json:"ranker,omitzero"`
	paramObj
}

func (r VectorStoreSearchParamsRankingOptions) MarshalJSON() (data []byte, err error) {
	type shadow VectorStoreSearchParamsRankingOptions
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *VectorStoreSearchParamsRankingOptions) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[VectorStoreSearchParamsRankingOptions](
		"ranker", "auto", "default-2024-11-15",
	)
}
