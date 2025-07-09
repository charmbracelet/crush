// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package responses

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/openai/openai-go/internal/apijson"
	"github.com/openai/openai-go/internal/apiquery"
	"github.com/openai/openai-go/internal/paramutil"
	"github.com/openai/openai-go/internal/requestconfig"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/respjson"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/shared"
	"github.com/openai/openai-go/shared/constant"
)

// ResponseService contains methods and other services that help with interacting
// with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewResponseService] method instead.
type ResponseService struct {
	Options    []option.RequestOption
	InputItems InputItemService
}

// NewResponseService generates a new service that applies the given options to
// each request. These options are applied after the parent client's options (if
// there is one), and before any request-specific options.
func NewResponseService(opts ...option.RequestOption) (r ResponseService) {
	r = ResponseService{}
	r.Options = opts
	r.InputItems = NewInputItemService(opts...)
	return
}

// Creates a model response. Provide
// [text](https://platform.openai.com/docs/guides/text) or
// [image](https://platform.openai.com/docs/guides/images) inputs to generate
// [text](https://platform.openai.com/docs/guides/text) or
// [JSON](https://platform.openai.com/docs/guides/structured-outputs) outputs. Have
// the model call your own
// [custom code](https://platform.openai.com/docs/guides/function-calling) or use
// built-in [tools](https://platform.openai.com/docs/guides/tools) like
// [web search](https://platform.openai.com/docs/guides/tools-web-search) or
// [file search](https://platform.openai.com/docs/guides/tools-file-search) to use
// your own data as input for the model's response.
func (r *ResponseService) New(ctx context.Context, body ResponseNewParams, opts ...option.RequestOption) (res *Response, err error) {
	opts = append(r.Options[:], opts...)
	path := "responses"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// Creates a model response. Provide
// [text](https://platform.openai.com/docs/guides/text) or
// [image](https://platform.openai.com/docs/guides/images) inputs to generate
// [text](https://platform.openai.com/docs/guides/text) or
// [JSON](https://platform.openai.com/docs/guides/structured-outputs) outputs. Have
// the model call your own
// [custom code](https://platform.openai.com/docs/guides/function-calling) or use
// built-in [tools](https://platform.openai.com/docs/guides/tools) like
// [web search](https://platform.openai.com/docs/guides/tools-web-search) or
// [file search](https://platform.openai.com/docs/guides/tools-file-search) to use
// your own data as input for the model's response.
func (r *ResponseService) NewStreaming(ctx context.Context, body ResponseNewParams, opts ...option.RequestOption) (stream *ssestream.Stream[ResponseStreamEventUnion]) {
	var (
		raw *http.Response
		err error
	)
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithJSONSet("stream", true)}, opts...)
	path := "responses"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &raw, opts...)
	return ssestream.NewStream[ResponseStreamEventUnion](ssestream.NewDecoder(raw), err)
}

// Retrieves a model response with the given ID.
func (r *ResponseService) Get(ctx context.Context, responseID string, query ResponseGetParams, opts ...option.RequestOption) (res *Response, err error) {
	opts = append(r.Options[:], opts...)
	if responseID == "" {
		err = errors.New("missing required response_id parameter")
		return
	}
	path := fmt.Sprintf("responses/%s", responseID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodGet, path, query, &res, opts...)
	return
}

// Retrieves a model response with the given ID.
func (r *ResponseService) GetStreaming(ctx context.Context, responseID string, query ResponseGetParams, opts ...option.RequestOption) (stream *ssestream.Stream[ResponseStreamEventUnion]) {
	var (
		raw *http.Response
		err error
	)
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithJSONSet("stream", true)}, opts...)
	if responseID == "" {
		err = errors.New("missing required response_id parameter")
		return
	}
	path := fmt.Sprintf("responses/%s", responseID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodGet, path, query, &raw, opts...)
	return ssestream.NewStream[ResponseStreamEventUnion](ssestream.NewDecoder(raw), err)
}

// Deletes a model response with the given ID.
func (r *ResponseService) Delete(ctx context.Context, responseID string, opts ...option.RequestOption) (err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("Accept", "")}, opts...)
	if responseID == "" {
		err = errors.New("missing required response_id parameter")
		return
	}
	path := fmt.Sprintf("responses/%s", responseID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodDelete, path, nil, nil, opts...)
	return
}

// Cancels a model response with the given ID. Only responses created with the
// `background` parameter set to `true` can be cancelled.
// [Learn more](https://platform.openai.com/docs/guides/background).
func (r *ResponseService) Cancel(ctx context.Context, responseID string, opts ...option.RequestOption) (res *Response, err error) {
	opts = append(r.Options[:], opts...)
	if responseID == "" {
		err = errors.New("missing required response_id parameter")
		return
	}
	path := fmt.Sprintf("responses/%s/cancel", responseID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, nil, &res, opts...)
	return
}

// A tool that controls a virtual computer. Learn more about the
// [computer tool](https://platform.openai.com/docs/guides/tools-computer-use).
type ComputerTool struct {
	// The height of the computer display.
	DisplayHeight int64 `json:"display_height,required"`
	// The width of the computer display.
	DisplayWidth int64 `json:"display_width,required"`
	// The type of computer environment to control.
	//
	// Any of "windows", "mac", "linux", "ubuntu", "browser".
	Environment ComputerToolEnvironment `json:"environment,required"`
	// The type of the computer use tool. Always `computer_use_preview`.
	Type constant.ComputerUsePreview `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		DisplayHeight respjson.Field
		DisplayWidth  respjson.Field
		Environment   respjson.Field
		Type          respjson.Field
		ExtraFields   map[string]respjson.Field
		raw           string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ComputerTool) RawJSON() string { return r.JSON.raw }
func (r *ComputerTool) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ComputerTool to a ComputerToolParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ComputerToolParam.Overrides()
func (r ComputerTool) ToParam() ComputerToolParam {
	return param.Override[ComputerToolParam](json.RawMessage(r.RawJSON()))
}

// The type of computer environment to control.
type ComputerToolEnvironment string

const (
	ComputerToolEnvironmentWindows ComputerToolEnvironment = "windows"
	ComputerToolEnvironmentMac     ComputerToolEnvironment = "mac"
	ComputerToolEnvironmentLinux   ComputerToolEnvironment = "linux"
	ComputerToolEnvironmentUbuntu  ComputerToolEnvironment = "ubuntu"
	ComputerToolEnvironmentBrowser ComputerToolEnvironment = "browser"
)

// A tool that controls a virtual computer. Learn more about the
// [computer tool](https://platform.openai.com/docs/guides/tools-computer-use).
//
// The properties DisplayHeight, DisplayWidth, Environment, Type are required.
type ComputerToolParam struct {
	// The height of the computer display.
	DisplayHeight int64 `json:"display_height,required"`
	// The width of the computer display.
	DisplayWidth int64 `json:"display_width,required"`
	// The type of computer environment to control.
	//
	// Any of "windows", "mac", "linux", "ubuntu", "browser".
	Environment ComputerToolEnvironment `json:"environment,omitzero,required"`
	// The type of the computer use tool. Always `computer_use_preview`.
	//
	// This field can be elided, and will marshal its zero value as
	// "computer_use_preview".
	Type constant.ComputerUsePreview `json:"type,required"`
	paramObj
}

func (r ComputerToolParam) MarshalJSON() (data []byte, err error) {
	type shadow ComputerToolParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ComputerToolParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A message input to the model with a role indicating instruction following
// hierarchy. Instructions given with the `developer` or `system` role take
// precedence over instructions given with the `user` role. Messages with the
// `assistant` role are presumed to have been generated by the model in previous
// interactions.
type EasyInputMessage struct {
	// Text, image, or audio input to the model, used to generate a response. Can also
	// contain previous assistant responses.
	Content EasyInputMessageContentUnion `json:"content,required"`
	// The role of the message input. One of `user`, `assistant`, `system`, or
	// `developer`.
	//
	// Any of "user", "assistant", "system", "developer".
	Role EasyInputMessageRole `json:"role,required"`
	// The type of the message input. Always `message`.
	//
	// Any of "message".
	Type EasyInputMessageType `json:"type"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Content     respjson.Field
		Role        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r EasyInputMessage) RawJSON() string { return r.JSON.raw }
func (r *EasyInputMessage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this EasyInputMessage to a EasyInputMessageParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// EasyInputMessageParam.Overrides()
func (r EasyInputMessage) ToParam() EasyInputMessageParam {
	return param.Override[EasyInputMessageParam](json.RawMessage(r.RawJSON()))
}

// EasyInputMessageContentUnion contains all possible properties and values from
// [string], [ResponseInputMessageContentList].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfString OfInputItemContentList]
type EasyInputMessageContentUnion struct {
	// This field will be present if the value is a [string] instead of an object.
	OfString string `json:",inline"`
	// This field will be present if the value is a [ResponseInputMessageContentList]
	// instead of an object.
	OfInputItemContentList ResponseInputMessageContentList `json:",inline"`
	JSON                   struct {
		OfString               respjson.Field
		OfInputItemContentList respjson.Field
		raw                    string
	} `json:"-"`
}

func (u EasyInputMessageContentUnion) AsString() (v string) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u EasyInputMessageContentUnion) AsInputItemContentList() (v ResponseInputMessageContentList) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u EasyInputMessageContentUnion) RawJSON() string { return u.JSON.raw }

func (r *EasyInputMessageContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The role of the message input. One of `user`, `assistant`, `system`, or
// `developer`.
type EasyInputMessageRole string

const (
	EasyInputMessageRoleUser      EasyInputMessageRole = "user"
	EasyInputMessageRoleAssistant EasyInputMessageRole = "assistant"
	EasyInputMessageRoleSystem    EasyInputMessageRole = "system"
	EasyInputMessageRoleDeveloper EasyInputMessageRole = "developer"
)

// The type of the message input. Always `message`.
type EasyInputMessageType string

const (
	EasyInputMessageTypeMessage EasyInputMessageType = "message"
)

// A message input to the model with a role indicating instruction following
// hierarchy. Instructions given with the `developer` or `system` role take
// precedence over instructions given with the `user` role. Messages with the
// `assistant` role are presumed to have been generated by the model in previous
// interactions.
//
// The properties Content, Role are required.
type EasyInputMessageParam struct {
	// Text, image, or audio input to the model, used to generate a response. Can also
	// contain previous assistant responses.
	Content EasyInputMessageContentUnionParam `json:"content,omitzero,required"`
	// The role of the message input. One of `user`, `assistant`, `system`, or
	// `developer`.
	//
	// Any of "user", "assistant", "system", "developer".
	Role EasyInputMessageRole `json:"role,omitzero,required"`
	// The type of the message input. Always `message`.
	//
	// Any of "message".
	Type EasyInputMessageType `json:"type,omitzero"`
	paramObj
}

func (r EasyInputMessageParam) MarshalJSON() (data []byte, err error) {
	type shadow EasyInputMessageParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *EasyInputMessageParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type EasyInputMessageContentUnionParam struct {
	OfString               param.Opt[string]                    `json:",omitzero,inline"`
	OfInputItemContentList ResponseInputMessageContentListParam `json:",omitzero,inline"`
	paramUnion
}

func (u EasyInputMessageContentUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfInputItemContentList)
}
func (u *EasyInputMessageContentUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *EasyInputMessageContentUnionParam) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfInputItemContentList) {
		return &u.OfInputItemContentList
	}
	return nil
}

// A tool that searches for relevant content from uploaded files. Learn more about
// the
// [file search tool](https://platform.openai.com/docs/guides/tools-file-search).
type FileSearchTool struct {
	// The type of the file search tool. Always `file_search`.
	Type constant.FileSearch `json:"type,required"`
	// The IDs of the vector stores to search.
	VectorStoreIDs []string `json:"vector_store_ids,required"`
	// A filter to apply.
	Filters FileSearchToolFiltersUnion `json:"filters,nullable"`
	// The maximum number of results to return. This number should be between 1 and 50
	// inclusive.
	MaxNumResults int64 `json:"max_num_results"`
	// Ranking options for search.
	RankingOptions FileSearchToolRankingOptions `json:"ranking_options"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type           respjson.Field
		VectorStoreIDs respjson.Field
		Filters        respjson.Field
		MaxNumResults  respjson.Field
		RankingOptions respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FileSearchTool) RawJSON() string { return r.JSON.raw }
func (r *FileSearchTool) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this FileSearchTool to a FileSearchToolParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// FileSearchToolParam.Overrides()
func (r FileSearchTool) ToParam() FileSearchToolParam {
	return param.Override[FileSearchToolParam](json.RawMessage(r.RawJSON()))
}

// FileSearchToolFiltersUnion contains all possible properties and values from
// [shared.ComparisonFilter], [shared.CompoundFilter].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type FileSearchToolFiltersUnion struct {
	// This field is from variant [shared.ComparisonFilter].
	Key  string `json:"key"`
	Type string `json:"type"`
	// This field is from variant [shared.ComparisonFilter].
	Value shared.ComparisonFilterValueUnion `json:"value"`
	// This field is from variant [shared.CompoundFilter].
	Filters []shared.ComparisonFilter `json:"filters"`
	JSON    struct {
		Key     respjson.Field
		Type    respjson.Field
		Value   respjson.Field
		Filters respjson.Field
		raw     string
	} `json:"-"`
}

func (u FileSearchToolFiltersUnion) AsComparisonFilter() (v shared.ComparisonFilter) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u FileSearchToolFiltersUnion) AsCompoundFilter() (v shared.CompoundFilter) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u FileSearchToolFiltersUnion) RawJSON() string { return u.JSON.raw }

func (r *FileSearchToolFiltersUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Ranking options for search.
type FileSearchToolRankingOptions struct {
	// The ranker to use for the file search.
	//
	// Any of "auto", "default-2024-11-15".
	Ranker string `json:"ranker"`
	// The score threshold for the file search, a number between 0 and 1. Numbers
	// closer to 1 will attempt to return only the most relevant results, but may
	// return fewer results.
	ScoreThreshold float64 `json:"score_threshold"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Ranker         respjson.Field
		ScoreThreshold respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FileSearchToolRankingOptions) RawJSON() string { return r.JSON.raw }
func (r *FileSearchToolRankingOptions) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A tool that searches for relevant content from uploaded files. Learn more about
// the
// [file search tool](https://platform.openai.com/docs/guides/tools-file-search).
//
// The properties Type, VectorStoreIDs are required.
type FileSearchToolParam struct {
	// The IDs of the vector stores to search.
	VectorStoreIDs []string `json:"vector_store_ids,omitzero,required"`
	// The maximum number of results to return. This number should be between 1 and 50
	// inclusive.
	MaxNumResults param.Opt[int64] `json:"max_num_results,omitzero"`
	// A filter to apply.
	Filters FileSearchToolFiltersUnionParam `json:"filters,omitzero"`
	// Ranking options for search.
	RankingOptions FileSearchToolRankingOptionsParam `json:"ranking_options,omitzero"`
	// The type of the file search tool. Always `file_search`.
	//
	// This field can be elided, and will marshal its zero value as "file_search".
	Type constant.FileSearch `json:"type,required"`
	paramObj
}

func (r FileSearchToolParam) MarshalJSON() (data []byte, err error) {
	type shadow FileSearchToolParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *FileSearchToolParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type FileSearchToolFiltersUnionParam struct {
	OfComparisonFilter *shared.ComparisonFilterParam `json:",omitzero,inline"`
	OfCompoundFilter   *shared.CompoundFilterParam   `json:",omitzero,inline"`
	paramUnion
}

func (u FileSearchToolFiltersUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfComparisonFilter, u.OfCompoundFilter)
}
func (u *FileSearchToolFiltersUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *FileSearchToolFiltersUnionParam) asAny() any {
	if !param.IsOmitted(u.OfComparisonFilter) {
		return u.OfComparisonFilter
	} else if !param.IsOmitted(u.OfCompoundFilter) {
		return u.OfCompoundFilter
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FileSearchToolFiltersUnionParam) GetKey() *string {
	if vt := u.OfComparisonFilter; vt != nil {
		return &vt.Key
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FileSearchToolFiltersUnionParam) GetValue() *shared.ComparisonFilterValueUnionParam {
	if vt := u.OfComparisonFilter; vt != nil {
		return &vt.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FileSearchToolFiltersUnionParam) GetFilters() []shared.ComparisonFilterParam {
	if vt := u.OfCompoundFilter; vt != nil {
		return vt.Filters
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FileSearchToolFiltersUnionParam) GetType() *string {
	if vt := u.OfComparisonFilter; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfCompoundFilter; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Ranking options for search.
type FileSearchToolRankingOptionsParam struct {
	// The score threshold for the file search, a number between 0 and 1. Numbers
	// closer to 1 will attempt to return only the most relevant results, but may
	// return fewer results.
	ScoreThreshold param.Opt[float64] `json:"score_threshold,omitzero"`
	// The ranker to use for the file search.
	//
	// Any of "auto", "default-2024-11-15".
	Ranker string `json:"ranker,omitzero"`
	paramObj
}

func (r FileSearchToolRankingOptionsParam) MarshalJSON() (data []byte, err error) {
	type shadow FileSearchToolRankingOptionsParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *FileSearchToolRankingOptionsParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[FileSearchToolRankingOptionsParam](
		"ranker", "auto", "default-2024-11-15",
	)
}

// Defines a function in your own code the model can choose to call. Learn more
// about
// [function calling](https://platform.openai.com/docs/guides/function-calling).
type FunctionTool struct {
	// The name of the function to call.
	Name string `json:"name,required"`
	// A JSON schema object describing the parameters of the function.
	Parameters map[string]any `json:"parameters,required"`
	// Whether to enforce strict parameter validation. Default `true`.
	Strict bool `json:"strict,required"`
	// The type of the function tool. Always `function`.
	Type constant.Function `json:"type,required"`
	// A description of the function. Used by the model to determine whether or not to
	// call the function.
	Description string `json:"description,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Name        respjson.Field
		Parameters  respjson.Field
		Strict      respjson.Field
		Type        respjson.Field
		Description respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FunctionTool) RawJSON() string { return r.JSON.raw }
func (r *FunctionTool) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this FunctionTool to a FunctionToolParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// FunctionToolParam.Overrides()
func (r FunctionTool) ToParam() FunctionToolParam {
	return param.Override[FunctionToolParam](json.RawMessage(r.RawJSON()))
}

// Defines a function in your own code the model can choose to call. Learn more
// about
// [function calling](https://platform.openai.com/docs/guides/function-calling).
//
// The properties Name, Parameters, Strict, Type are required.
type FunctionToolParam struct {
	// Whether to enforce strict parameter validation. Default `true`.
	Strict param.Opt[bool] `json:"strict,omitzero,required"`
	// A JSON schema object describing the parameters of the function.
	Parameters map[string]any `json:"parameters,omitzero,required"`
	// The name of the function to call.
	Name string `json:"name,required"`
	// A description of the function. Used by the model to determine whether or not to
	// call the function.
	Description param.Opt[string] `json:"description,omitzero"`
	// The type of the function tool. Always `function`.
	//
	// This field can be elided, and will marshal its zero value as "function".
	Type constant.Function `json:"type,required"`
	paramObj
}

func (r FunctionToolParam) MarshalJSON() (data []byte, err error) {
	type shadow FunctionToolParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *FunctionToolParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type Response struct {
	// Unique identifier for this Response.
	ID string `json:"id,required"`
	// Unix timestamp (in seconds) of when this Response was created.
	CreatedAt float64 `json:"created_at,required"`
	// An error object returned when the model fails to generate a Response.
	Error ResponseError `json:"error,required"`
	// Details about why the response is incomplete.
	IncompleteDetails ResponseIncompleteDetails `json:"incomplete_details,required"`
	// A system (or developer) message inserted into the model's context.
	//
	// When using along with `previous_response_id`, the instructions from a previous
	// response will not be carried over to the next response. This makes it simple to
	// swap out system (or developer) messages in new responses.
	Instructions ResponseInstructionsUnion `json:"instructions,required"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,required"`
	// Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI offers a
	// wide range of models with different capabilities, performance characteristics,
	// and price points. Refer to the
	// [model guide](https://platform.openai.com/docs/models) to browse and compare
	// available models.
	Model shared.ResponsesModel `json:"model,required"`
	// The object type of this resource - always set to `response`.
	Object constant.Response `json:"object,required"`
	// An array of content items generated by the model.
	//
	//   - The length and order of items in the `output` array is dependent on the
	//     model's response.
	//   - Rather than accessing the first item in the `output` array and assuming it's
	//     an `assistant` message with the content generated by the model, you might
	//     consider using the `output_text` property where supported in SDKs.
	Output []ResponseOutputItemUnion `json:"output,required"`
	// Whether to allow the model to run tool calls in parallel.
	ParallelToolCalls bool `json:"parallel_tool_calls,required"`
	// What sampling temperature to use, between 0 and 2. Higher values like 0.8 will
	// make the output more random, while lower values like 0.2 will make it more
	// focused and deterministic. We generally recommend altering this or `top_p` but
	// not both.
	Temperature float64 `json:"temperature,required"`
	// How the model should select which tool (or tools) to use when generating a
	// response. See the `tools` parameter to see how to specify which tools the model
	// can call.
	ToolChoice ResponseToolChoiceUnion `json:"tool_choice,required"`
	// An array of tools the model may call while generating a response. You can
	// specify which tool to use by setting the `tool_choice` parameter.
	//
	// The two categories of tools you can provide the model are:
	//
	//   - **Built-in tools**: Tools that are provided by OpenAI that extend the model's
	//     capabilities, like
	//     [web search](https://platform.openai.com/docs/guides/tools-web-search) or
	//     [file search](https://platform.openai.com/docs/guides/tools-file-search).
	//     Learn more about
	//     [built-in tools](https://platform.openai.com/docs/guides/tools).
	//   - **Function calls (custom tools)**: Functions that are defined by you, enabling
	//     the model to call your own code. Learn more about
	//     [function calling](https://platform.openai.com/docs/guides/function-calling).
	Tools []ToolUnion `json:"tools,required"`
	// An alternative to sampling with temperature, called nucleus sampling, where the
	// model considers the results of the tokens with top_p probability mass. So 0.1
	// means only the tokens comprising the top 10% probability mass are considered.
	//
	// We generally recommend altering this or `temperature` but not both.
	TopP float64 `json:"top_p,required"`
	// Whether to run the model response in the background.
	// [Learn more](https://platform.openai.com/docs/guides/background).
	Background bool `json:"background,nullable"`
	// An upper bound for the number of tokens that can be generated for a response,
	// including visible output tokens and
	// [reasoning tokens](https://platform.openai.com/docs/guides/reasoning).
	MaxOutputTokens int64 `json:"max_output_tokens,nullable"`
	// The maximum number of total calls to built-in tools that can be processed in a
	// response. This maximum number applies across all built-in tool calls, not per
	// individual tool. Any further attempts to call a tool by the model will be
	// ignored.
	MaxToolCalls int64 `json:"max_tool_calls,nullable"`
	// The unique ID of the previous response to the model. Use this to create
	// multi-turn conversations. Learn more about
	// [conversation state](https://platform.openai.com/docs/guides/conversation-state).
	PreviousResponseID string `json:"previous_response_id,nullable"`
	// Reference to a prompt template and its variables.
	// [Learn more](https://platform.openai.com/docs/guides/text?api-mode=responses#reusable-prompts).
	Prompt ResponsePrompt `json:"prompt,nullable"`
	// **o-series models only**
	//
	// Configuration options for
	// [reasoning models](https://platform.openai.com/docs/guides/reasoning).
	Reasoning shared.Reasoning `json:"reasoning,nullable"`
	// Specifies the processing type used for serving the request.
	//
	//   - If set to 'auto', then the request will be processed with the service tier
	//     configured in the Project settings. Unless otherwise configured, the Project
	//     will use 'default'.
	//   - If set to 'default', then the requset will be processed with the standard
	//     pricing and performance for the selected model.
	//   - If set to '[flex](https://platform.openai.com/docs/guides/flex-processing)' or
	//     'priority', then the request will be processed with the corresponding service
	//     tier. [Contact sales](https://openai.com/contact-sales) to learn more about
	//     Priority processing.
	//   - When not set, the default behavior is 'auto'.
	//
	// When the `service_tier` parameter is set, the response body will include the
	// `service_tier` value based on the processing mode actually used to serve the
	// request. This response value may be different from the value set in the
	// parameter.
	//
	// Any of "auto", "default", "flex", "scale", "priority".
	ServiceTier ResponseServiceTier `json:"service_tier,nullable"`
	// The status of the response generation. One of `completed`, `failed`,
	// `in_progress`, `cancelled`, `queued`, or `incomplete`.
	//
	// Any of "completed", "failed", "in_progress", "cancelled", "queued",
	// "incomplete".
	Status ResponseStatus `json:"status"`
	// Configuration options for a text response from the model. Can be plain text or
	// structured JSON data. Learn more:
	//
	// - [Text inputs and outputs](https://platform.openai.com/docs/guides/text)
	// - [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs)
	Text ResponseTextConfig `json:"text"`
	// An integer between 0 and 20 specifying the number of most likely tokens to
	// return at each token position, each with an associated log probability.
	TopLogprobs int64 `json:"top_logprobs,nullable"`
	// The truncation strategy to use for the model response.
	//
	//   - `auto`: If the context of this response and previous ones exceeds the model's
	//     context window size, the model will truncate the response to fit the context
	//     window by dropping input items in the middle of the conversation.
	//   - `disabled` (default): If a model response will exceed the context window size
	//     for a model, the request will fail with a 400 error.
	//
	// Any of "auto", "disabled".
	Truncation ResponseTruncation `json:"truncation,nullable"`
	// Represents token usage details including input tokens, output tokens, a
	// breakdown of output tokens, and the total tokens used.
	Usage ResponseUsage `json:"usage"`
	// A stable identifier for your end-users. Used to boost cache hit rates by better
	// bucketing similar requests and to help OpenAI detect and prevent abuse.
	// [Learn more](https://platform.openai.com/docs/guides/safety-best-practices#end-user-ids).
	User string `json:"user"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID                 respjson.Field
		CreatedAt          respjson.Field
		Error              respjson.Field
		IncompleteDetails  respjson.Field
		Instructions       respjson.Field
		Metadata           respjson.Field
		Model              respjson.Field
		Object             respjson.Field
		Output             respjson.Field
		ParallelToolCalls  respjson.Field
		Temperature        respjson.Field
		ToolChoice         respjson.Field
		Tools              respjson.Field
		TopP               respjson.Field
		Background         respjson.Field
		MaxOutputTokens    respjson.Field
		MaxToolCalls       respjson.Field
		PreviousResponseID respjson.Field
		Prompt             respjson.Field
		Reasoning          respjson.Field
		ServiceTier        respjson.Field
		Status             respjson.Field
		Text               respjson.Field
		TopLogprobs        respjson.Field
		Truncation         respjson.Field
		Usage              respjson.Field
		User               respjson.Field
		ExtraFields        map[string]respjson.Field
		raw                string
	} `json:"-"`
}

func (r Response) OutputText() string {
	var outputText strings.Builder
	for _, item := range r.Output {
		for _, content := range item.Content {
			if content.Type == "output_text" {
				outputText.WriteString(content.Text)
			}
		}
	}
	return outputText.String()
}

// Returns the unmodified JSON received from the API
func (r Response) RawJSON() string { return r.JSON.raw }
func (r *Response) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Details about why the response is incomplete.
type ResponseIncompleteDetails struct {
	// The reason why the response is incomplete.
	//
	// Any of "max_output_tokens", "content_filter".
	Reason string `json:"reason"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Reason      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseIncompleteDetails) RawJSON() string { return r.JSON.raw }
func (r *ResponseIncompleteDetails) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseInstructionsUnion contains all possible properties and values from
// [string], [[]ResponseInputItemUnion].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfString OfInputItemList]
type ResponseInstructionsUnion struct {
	// This field will be present if the value is a [string] instead of an object.
	OfString string `json:",inline"`
	// This field will be present if the value is a [[]ResponseInputItemUnion] instead
	// of an object.
	OfInputItemList []ResponseInputItemUnion `json:",inline"`
	JSON            struct {
		OfString        respjson.Field
		OfInputItemList respjson.Field
		raw             string
	} `json:"-"`
}

func (u ResponseInstructionsUnion) AsString() (v string) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInstructionsUnion) AsInputItemList() (v []ResponseInputItemUnion) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ResponseInstructionsUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseInstructionsUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseToolChoiceUnion contains all possible properties and values from
// [ToolChoiceOptions], [ToolChoiceTypes], [ToolChoiceFunction], [ToolChoiceMcp].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfToolChoiceMode]
type ResponseToolChoiceUnion struct {
	// This field will be present if the value is a [ToolChoiceOptions] instead of an
	// object.
	OfToolChoiceMode ToolChoiceOptions `json:",inline"`
	Type             string            `json:"type"`
	Name             string            `json:"name"`
	// This field is from variant [ToolChoiceMcp].
	ServerLabel string `json:"server_label"`
	JSON        struct {
		OfToolChoiceMode respjson.Field
		Type             respjson.Field
		Name             respjson.Field
		ServerLabel      respjson.Field
		raw              string
	} `json:"-"`
}

func (u ResponseToolChoiceUnion) AsToolChoiceMode() (v ToolChoiceOptions) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseToolChoiceUnion) AsHostedTool() (v ToolChoiceTypes) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseToolChoiceUnion) AsFunctionTool() (v ToolChoiceFunction) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseToolChoiceUnion) AsMcpTool() (v ToolChoiceMcp) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ResponseToolChoiceUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseToolChoiceUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Specifies the processing type used for serving the request.
//
//   - If set to 'auto', then the request will be processed with the service tier
//     configured in the Project settings. Unless otherwise configured, the Project
//     will use 'default'.
//   - If set to 'default', then the requset will be processed with the standard
//     pricing and performance for the selected model.
//   - If set to '[flex](https://platform.openai.com/docs/guides/flex-processing)' or
//     'priority', then the request will be processed with the corresponding service
//     tier. [Contact sales](https://openai.com/contact-sales) to learn more about
//     Priority processing.
//   - When not set, the default behavior is 'auto'.
//
// When the `service_tier` parameter is set, the response body will include the
// `service_tier` value based on the processing mode actually used to serve the
// request. This response value may be different from the value set in the
// parameter.
type ResponseServiceTier string

const (
	ResponseServiceTierAuto     ResponseServiceTier = "auto"
	ResponseServiceTierDefault  ResponseServiceTier = "default"
	ResponseServiceTierFlex     ResponseServiceTier = "flex"
	ResponseServiceTierScale    ResponseServiceTier = "scale"
	ResponseServiceTierPriority ResponseServiceTier = "priority"
)

// The truncation strategy to use for the model response.
//
//   - `auto`: If the context of this response and previous ones exceeds the model's
//     context window size, the model will truncate the response to fit the context
//     window by dropping input items in the middle of the conversation.
//   - `disabled` (default): If a model response will exceed the context window size
//     for a model, the request will fail with a 400 error.
type ResponseTruncation string

const (
	ResponseTruncationAuto     ResponseTruncation = "auto"
	ResponseTruncationDisabled ResponseTruncation = "disabled"
)

// Emitted when there is a partial audio response.
type ResponseAudioDeltaEvent struct {
	// A chunk of Base64 encoded response audio bytes.
	Delta string `json:"delta,required"`
	// A sequence number for this chunk of the stream response.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.audio.delta`.
	Type constant.ResponseAudioDelta `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Delta          respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseAudioDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseAudioDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when the audio response is complete.
type ResponseAudioDoneEvent struct {
	// The sequence number of the delta.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.audio.done`.
	Type constant.ResponseAudioDone `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseAudioDoneEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseAudioDoneEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when there is a partial transcript of audio.
type ResponseAudioTranscriptDeltaEvent struct {
	// The partial transcript of the audio response.
	Delta string `json:"delta,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.audio.transcript.delta`.
	Type constant.ResponseAudioTranscriptDelta `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Delta          respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseAudioTranscriptDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseAudioTranscriptDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when the full audio transcript is completed.
type ResponseAudioTranscriptDoneEvent struct {
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.audio.transcript.done`.
	Type constant.ResponseAudioTranscriptDone `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseAudioTranscriptDoneEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseAudioTranscriptDoneEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a partial code snippet is streamed by the code interpreter.
type ResponseCodeInterpreterCallCodeDeltaEvent struct {
	// The partial code snippet being streamed by the code interpreter.
	Delta string `json:"delta,required"`
	// The unique identifier of the code interpreter tool call item.
	ItemID string `json:"item_id,required"`
	// The index of the output item in the response for which the code is being
	// streamed.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event, used to order streaming events.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.code_interpreter_call_code.delta`.
	Type constant.ResponseCodeInterpreterCallCodeDelta `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Delta          respjson.Field
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCodeInterpreterCallCodeDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseCodeInterpreterCallCodeDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when the code snippet is finalized by the code interpreter.
type ResponseCodeInterpreterCallCodeDoneEvent struct {
	// The final code snippet output by the code interpreter.
	Code string `json:"code,required"`
	// The unique identifier of the code interpreter tool call item.
	ItemID string `json:"item_id,required"`
	// The index of the output item in the response for which the code is finalized.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event, used to order streaming events.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.code_interpreter_call_code.done`.
	Type constant.ResponseCodeInterpreterCallCodeDone `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Code           respjson.Field
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCodeInterpreterCallCodeDoneEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseCodeInterpreterCallCodeDoneEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when the code interpreter call is completed.
type ResponseCodeInterpreterCallCompletedEvent struct {
	// The unique identifier of the code interpreter tool call item.
	ItemID string `json:"item_id,required"`
	// The index of the output item in the response for which the code interpreter call
	// is completed.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event, used to order streaming events.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.code_interpreter_call.completed`.
	Type constant.ResponseCodeInterpreterCallCompleted `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCodeInterpreterCallCompletedEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseCodeInterpreterCallCompletedEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a code interpreter call is in progress.
type ResponseCodeInterpreterCallInProgressEvent struct {
	// The unique identifier of the code interpreter tool call item.
	ItemID string `json:"item_id,required"`
	// The index of the output item in the response for which the code interpreter call
	// is in progress.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event, used to order streaming events.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.code_interpreter_call.in_progress`.
	Type constant.ResponseCodeInterpreterCallInProgress `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCodeInterpreterCallInProgressEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseCodeInterpreterCallInProgressEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when the code interpreter is actively interpreting the code snippet.
type ResponseCodeInterpreterCallInterpretingEvent struct {
	// The unique identifier of the code interpreter tool call item.
	ItemID string `json:"item_id,required"`
	// The index of the output item in the response for which the code interpreter is
	// interpreting code.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event, used to order streaming events.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.code_interpreter_call.interpreting`.
	Type constant.ResponseCodeInterpreterCallInterpreting `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCodeInterpreterCallInterpretingEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseCodeInterpreterCallInterpretingEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A tool call to run code.
type ResponseCodeInterpreterToolCall struct {
	// The unique ID of the code interpreter tool call.
	ID string `json:"id,required"`
	// The code to run, or null if not available.
	Code string `json:"code,required"`
	// The ID of the container used to run the code.
	ContainerID string `json:"container_id,required"`
	// The outputs generated by the code interpreter, such as logs or images. Can be
	// null if no outputs are available.
	Outputs []ResponseCodeInterpreterToolCallOutputUnion `json:"outputs,required"`
	// The status of the code interpreter tool call.
	//
	// Any of "in_progress", "completed", "incomplete", "interpreting", "failed".
	Status ResponseCodeInterpreterToolCallStatus `json:"status,required"`
	// The type of the code interpreter tool call. Always `code_interpreter_call`.
	Type constant.CodeInterpreterCall `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Code        respjson.Field
		ContainerID respjson.Field
		Outputs     respjson.Field
		Status      respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCodeInterpreterToolCall) RawJSON() string { return r.JSON.raw }
func (r *ResponseCodeInterpreterToolCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponseCodeInterpreterToolCall to a
// ResponseCodeInterpreterToolCallParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseCodeInterpreterToolCallParam.Overrides()
func (r ResponseCodeInterpreterToolCall) ToParam() ResponseCodeInterpreterToolCallParam {
	return param.Override[ResponseCodeInterpreterToolCallParam](json.RawMessage(r.RawJSON()))
}

// ResponseCodeInterpreterToolCallOutputUnion contains all possible properties and
// values from [ResponseCodeInterpreterToolCallOutputLogs],
// [ResponseCodeInterpreterToolCallOutputImage].
//
// Use the [ResponseCodeInterpreterToolCallOutputUnion.AsAny] method to switch on
// the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ResponseCodeInterpreterToolCallOutputUnion struct {
	// This field is from variant [ResponseCodeInterpreterToolCallOutputLogs].
	Logs string `json:"logs"`
	// Any of "logs", "image".
	Type string `json:"type"`
	// This field is from variant [ResponseCodeInterpreterToolCallOutputImage].
	URL  string `json:"url"`
	JSON struct {
		Logs respjson.Field
		Type respjson.Field
		URL  respjson.Field
		raw  string
	} `json:"-"`
}

// anyResponseCodeInterpreterToolCallOutput is implemented by each variant of
// [ResponseCodeInterpreterToolCallOutputUnion] to add type safety for the return
// type of [ResponseCodeInterpreterToolCallOutputUnion.AsAny]
type anyResponseCodeInterpreterToolCallOutput interface {
	implResponseCodeInterpreterToolCallOutputUnion()
}

func (ResponseCodeInterpreterToolCallOutputLogs) implResponseCodeInterpreterToolCallOutputUnion()  {}
func (ResponseCodeInterpreterToolCallOutputImage) implResponseCodeInterpreterToolCallOutputUnion() {}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseCodeInterpreterToolCallOutputUnion.AsAny().(type) {
//	case responses.ResponseCodeInterpreterToolCallOutputLogs:
//	case responses.ResponseCodeInterpreterToolCallOutputImage:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseCodeInterpreterToolCallOutputUnion) AsAny() anyResponseCodeInterpreterToolCallOutput {
	switch u.Type {
	case "logs":
		return u.AsLogs()
	case "image":
		return u.AsImage()
	}
	return nil
}

func (u ResponseCodeInterpreterToolCallOutputUnion) AsLogs() (v ResponseCodeInterpreterToolCallOutputLogs) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseCodeInterpreterToolCallOutputUnion) AsImage() (v ResponseCodeInterpreterToolCallOutputImage) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ResponseCodeInterpreterToolCallOutputUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseCodeInterpreterToolCallOutputUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The logs output from the code interpreter.
type ResponseCodeInterpreterToolCallOutputLogs struct {
	// The logs output from the code interpreter.
	Logs string `json:"logs,required"`
	// The type of the output. Always 'logs'.
	Type constant.Logs `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Logs        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCodeInterpreterToolCallOutputLogs) RawJSON() string { return r.JSON.raw }
func (r *ResponseCodeInterpreterToolCallOutputLogs) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The image output from the code interpreter.
type ResponseCodeInterpreterToolCallOutputImage struct {
	// The type of the output. Always 'image'.
	Type constant.Image `json:"type,required"`
	// The URL of the image output from the code interpreter.
	URL string `json:"url,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		URL         respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCodeInterpreterToolCallOutputImage) RawJSON() string { return r.JSON.raw }
func (r *ResponseCodeInterpreterToolCallOutputImage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The status of the code interpreter tool call.
type ResponseCodeInterpreterToolCallStatus string

const (
	ResponseCodeInterpreterToolCallStatusInProgress   ResponseCodeInterpreterToolCallStatus = "in_progress"
	ResponseCodeInterpreterToolCallStatusCompleted    ResponseCodeInterpreterToolCallStatus = "completed"
	ResponseCodeInterpreterToolCallStatusIncomplete   ResponseCodeInterpreterToolCallStatus = "incomplete"
	ResponseCodeInterpreterToolCallStatusInterpreting ResponseCodeInterpreterToolCallStatus = "interpreting"
	ResponseCodeInterpreterToolCallStatusFailed       ResponseCodeInterpreterToolCallStatus = "failed"
)

// A tool call to run code.
//
// The properties ID, Code, ContainerID, Outputs, Status, Type are required.
type ResponseCodeInterpreterToolCallParam struct {
	// The code to run, or null if not available.
	Code param.Opt[string] `json:"code,omitzero,required"`
	// The outputs generated by the code interpreter, such as logs or images. Can be
	// null if no outputs are available.
	Outputs []ResponseCodeInterpreterToolCallOutputUnionParam `json:"outputs,omitzero,required"`
	// The unique ID of the code interpreter tool call.
	ID string `json:"id,required"`
	// The ID of the container used to run the code.
	ContainerID string `json:"container_id,required"`
	// The status of the code interpreter tool call.
	//
	// Any of "in_progress", "completed", "incomplete", "interpreting", "failed".
	Status ResponseCodeInterpreterToolCallStatus `json:"status,omitzero,required"`
	// The type of the code interpreter tool call. Always `code_interpreter_call`.
	//
	// This field can be elided, and will marshal its zero value as
	// "code_interpreter_call".
	Type constant.CodeInterpreterCall `json:"type,required"`
	paramObj
}

func (r ResponseCodeInterpreterToolCallParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseCodeInterpreterToolCallParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseCodeInterpreterToolCallParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ResponseCodeInterpreterToolCallOutputUnionParam struct {
	OfLogs  *ResponseCodeInterpreterToolCallOutputLogsParam  `json:",omitzero,inline"`
	OfImage *ResponseCodeInterpreterToolCallOutputImageParam `json:",omitzero,inline"`
	paramUnion
}

func (u ResponseCodeInterpreterToolCallOutputUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfLogs, u.OfImage)
}
func (u *ResponseCodeInterpreterToolCallOutputUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ResponseCodeInterpreterToolCallOutputUnionParam) asAny() any {
	if !param.IsOmitted(u.OfLogs) {
		return u.OfLogs
	} else if !param.IsOmitted(u.OfImage) {
		return u.OfImage
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseCodeInterpreterToolCallOutputUnionParam) GetLogs() *string {
	if vt := u.OfLogs; vt != nil {
		return &vt.Logs
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseCodeInterpreterToolCallOutputUnionParam) GetURL() *string {
	if vt := u.OfImage; vt != nil {
		return &vt.URL
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseCodeInterpreterToolCallOutputUnionParam) GetType() *string {
	if vt := u.OfLogs; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfImage; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[ResponseCodeInterpreterToolCallOutputUnionParam](
		"type",
		apijson.Discriminator[ResponseCodeInterpreterToolCallOutputLogsParam]("logs"),
		apijson.Discriminator[ResponseCodeInterpreterToolCallOutputImageParam]("image"),
	)
}

// The logs output from the code interpreter.
//
// The properties Logs, Type are required.
type ResponseCodeInterpreterToolCallOutputLogsParam struct {
	// The logs output from the code interpreter.
	Logs string `json:"logs,required"`
	// The type of the output. Always 'logs'.
	//
	// This field can be elided, and will marshal its zero value as "logs".
	Type constant.Logs `json:"type,required"`
	paramObj
}

func (r ResponseCodeInterpreterToolCallOutputLogsParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseCodeInterpreterToolCallOutputLogsParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseCodeInterpreterToolCallOutputLogsParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The image output from the code interpreter.
//
// The properties Type, URL are required.
type ResponseCodeInterpreterToolCallOutputImageParam struct {
	// The URL of the image output from the code interpreter.
	URL string `json:"url,required"`
	// The type of the output. Always 'image'.
	//
	// This field can be elided, and will marshal its zero value as "image".
	Type constant.Image `json:"type,required"`
	paramObj
}

func (r ResponseCodeInterpreterToolCallOutputImageParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseCodeInterpreterToolCallOutputImageParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseCodeInterpreterToolCallOutputImageParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when the model response is complete.
type ResponseCompletedEvent struct {
	// Properties of the completed response.
	Response Response `json:"response,required"`
	// The sequence number for this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.completed`.
	Type constant.ResponseCompleted `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Response       respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCompletedEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseCompletedEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A tool call to a computer use tool. See the
// [computer use guide](https://platform.openai.com/docs/guides/tools-computer-use)
// for more information.
type ResponseComputerToolCall struct {
	// The unique ID of the computer call.
	ID string `json:"id,required"`
	// A click action.
	Action ResponseComputerToolCallActionUnion `json:"action,required"`
	// An identifier used when responding to the tool call with output.
	CallID string `json:"call_id,required"`
	// The pending safety checks for the computer call.
	PendingSafetyChecks []ResponseComputerToolCallPendingSafetyCheck `json:"pending_safety_checks,required"`
	// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
	// Populated when items are returned via API.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status ResponseComputerToolCallStatus `json:"status,required"`
	// The type of the computer call. Always `computer_call`.
	//
	// Any of "computer_call".
	Type ResponseComputerToolCallType `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID                  respjson.Field
		Action              respjson.Field
		CallID              respjson.Field
		PendingSafetyChecks respjson.Field
		Status              respjson.Field
		Type                respjson.Field
		ExtraFields         map[string]respjson.Field
		raw                 string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseComputerToolCall) RawJSON() string { return r.JSON.raw }
func (r *ResponseComputerToolCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponseComputerToolCall to a
// ResponseComputerToolCallParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseComputerToolCallParam.Overrides()
func (r ResponseComputerToolCall) ToParam() ResponseComputerToolCallParam {
	return param.Override[ResponseComputerToolCallParam](json.RawMessage(r.RawJSON()))
}

// ResponseComputerToolCallActionUnion contains all possible properties and values
// from [ResponseComputerToolCallActionClick],
// [ResponseComputerToolCallActionDoubleClick],
// [ResponseComputerToolCallActionDrag], [ResponseComputerToolCallActionKeypress],
// [ResponseComputerToolCallActionMove],
// [ResponseComputerToolCallActionScreenshot],
// [ResponseComputerToolCallActionScroll], [ResponseComputerToolCallActionType],
// [ResponseComputerToolCallActionWait].
//
// Use the [ResponseComputerToolCallActionUnion.AsAny] method to switch on the
// variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ResponseComputerToolCallActionUnion struct {
	// This field is from variant [ResponseComputerToolCallActionClick].
	Button string `json:"button"`
	// Any of "click", "double_click", "drag", "keypress", "move", "screenshot",
	// "scroll", "type", "wait".
	Type string `json:"type"`
	X    int64  `json:"x"`
	Y    int64  `json:"y"`
	// This field is from variant [ResponseComputerToolCallActionDrag].
	Path []ResponseComputerToolCallActionDragPath `json:"path"`
	// This field is from variant [ResponseComputerToolCallActionKeypress].
	Keys []string `json:"keys"`
	// This field is from variant [ResponseComputerToolCallActionScroll].
	ScrollX int64 `json:"scroll_x"`
	// This field is from variant [ResponseComputerToolCallActionScroll].
	ScrollY int64 `json:"scroll_y"`
	// This field is from variant [ResponseComputerToolCallActionType].
	Text string `json:"text"`
	JSON struct {
		Button  respjson.Field
		Type    respjson.Field
		X       respjson.Field
		Y       respjson.Field
		Path    respjson.Field
		Keys    respjson.Field
		ScrollX respjson.Field
		ScrollY respjson.Field
		Text    respjson.Field
		raw     string
	} `json:"-"`
}

// anyResponseComputerToolCallAction is implemented by each variant of
// [ResponseComputerToolCallActionUnion] to add type safety for the return type of
// [ResponseComputerToolCallActionUnion.AsAny]
type anyResponseComputerToolCallAction interface {
	implResponseComputerToolCallActionUnion()
}

func (ResponseComputerToolCallActionClick) implResponseComputerToolCallActionUnion()       {}
func (ResponseComputerToolCallActionDoubleClick) implResponseComputerToolCallActionUnion() {}
func (ResponseComputerToolCallActionDrag) implResponseComputerToolCallActionUnion()        {}
func (ResponseComputerToolCallActionKeypress) implResponseComputerToolCallActionUnion()    {}
func (ResponseComputerToolCallActionMove) implResponseComputerToolCallActionUnion()        {}
func (ResponseComputerToolCallActionScreenshot) implResponseComputerToolCallActionUnion()  {}
func (ResponseComputerToolCallActionScroll) implResponseComputerToolCallActionUnion()      {}
func (ResponseComputerToolCallActionType) implResponseComputerToolCallActionUnion()        {}
func (ResponseComputerToolCallActionWait) implResponseComputerToolCallActionUnion()        {}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseComputerToolCallActionUnion.AsAny().(type) {
//	case responses.ResponseComputerToolCallActionClick:
//	case responses.ResponseComputerToolCallActionDoubleClick:
//	case responses.ResponseComputerToolCallActionDrag:
//	case responses.ResponseComputerToolCallActionKeypress:
//	case responses.ResponseComputerToolCallActionMove:
//	case responses.ResponseComputerToolCallActionScreenshot:
//	case responses.ResponseComputerToolCallActionScroll:
//	case responses.ResponseComputerToolCallActionType:
//	case responses.ResponseComputerToolCallActionWait:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseComputerToolCallActionUnion) AsAny() anyResponseComputerToolCallAction {
	switch u.Type {
	case "click":
		return u.AsClick()
	case "double_click":
		return u.AsDoubleClick()
	case "drag":
		return u.AsDrag()
	case "keypress":
		return u.AsKeypress()
	case "move":
		return u.AsMove()
	case "screenshot":
		return u.AsScreenshot()
	case "scroll":
		return u.AsScroll()
	case "type":
		return u.AsType()
	case "wait":
		return u.AsWait()
	}
	return nil
}

func (u ResponseComputerToolCallActionUnion) AsClick() (v ResponseComputerToolCallActionClick) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseComputerToolCallActionUnion) AsDoubleClick() (v ResponseComputerToolCallActionDoubleClick) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseComputerToolCallActionUnion) AsDrag() (v ResponseComputerToolCallActionDrag) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseComputerToolCallActionUnion) AsKeypress() (v ResponseComputerToolCallActionKeypress) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseComputerToolCallActionUnion) AsMove() (v ResponseComputerToolCallActionMove) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseComputerToolCallActionUnion) AsScreenshot() (v ResponseComputerToolCallActionScreenshot) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseComputerToolCallActionUnion) AsScroll() (v ResponseComputerToolCallActionScroll) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseComputerToolCallActionUnion) AsType() (v ResponseComputerToolCallActionType) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseComputerToolCallActionUnion) AsWait() (v ResponseComputerToolCallActionWait) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ResponseComputerToolCallActionUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseComputerToolCallActionUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A click action.
type ResponseComputerToolCallActionClick struct {
	// Indicates which mouse button was pressed during the click. One of `left`,
	// `right`, `wheel`, `back`, or `forward`.
	//
	// Any of "left", "right", "wheel", "back", "forward".
	Button string `json:"button,required"`
	// Specifies the event type. For a click action, this property is always set to
	// `click`.
	Type constant.Click `json:"type,required"`
	// The x-coordinate where the click occurred.
	X int64 `json:"x,required"`
	// The y-coordinate where the click occurred.
	Y int64 `json:"y,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Button      respjson.Field
		Type        respjson.Field
		X           respjson.Field
		Y           respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseComputerToolCallActionClick) RawJSON() string { return r.JSON.raw }
func (r *ResponseComputerToolCallActionClick) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A double click action.
type ResponseComputerToolCallActionDoubleClick struct {
	// Specifies the event type. For a double click action, this property is always set
	// to `double_click`.
	Type constant.DoubleClick `json:"type,required"`
	// The x-coordinate where the double click occurred.
	X int64 `json:"x,required"`
	// The y-coordinate where the double click occurred.
	Y int64 `json:"y,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		X           respjson.Field
		Y           respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseComputerToolCallActionDoubleClick) RawJSON() string { return r.JSON.raw }
func (r *ResponseComputerToolCallActionDoubleClick) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A drag action.
type ResponseComputerToolCallActionDrag struct {
	// An array of coordinates representing the path of the drag action. Coordinates
	// will appear as an array of objects, eg
	//
	// ```
	// [
	//
	//	{ x: 100, y: 200 },
	//	{ x: 200, y: 300 }
	//
	// ]
	// ```
	Path []ResponseComputerToolCallActionDragPath `json:"path,required"`
	// Specifies the event type. For a drag action, this property is always set to
	// `drag`.
	Type constant.Drag `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Path        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseComputerToolCallActionDrag) RawJSON() string { return r.JSON.raw }
func (r *ResponseComputerToolCallActionDrag) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A series of x/y coordinate pairs in the drag path.
type ResponseComputerToolCallActionDragPath struct {
	// The x-coordinate.
	X int64 `json:"x,required"`
	// The y-coordinate.
	Y int64 `json:"y,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		X           respjson.Field
		Y           respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseComputerToolCallActionDragPath) RawJSON() string { return r.JSON.raw }
func (r *ResponseComputerToolCallActionDragPath) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A collection of keypresses the model would like to perform.
type ResponseComputerToolCallActionKeypress struct {
	// The combination of keys the model is requesting to be pressed. This is an array
	// of strings, each representing a key.
	Keys []string `json:"keys,required"`
	// Specifies the event type. For a keypress action, this property is always set to
	// `keypress`.
	Type constant.Keypress `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Keys        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseComputerToolCallActionKeypress) RawJSON() string { return r.JSON.raw }
func (r *ResponseComputerToolCallActionKeypress) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A mouse move action.
type ResponseComputerToolCallActionMove struct {
	// Specifies the event type. For a move action, this property is always set to
	// `move`.
	Type constant.Move `json:"type,required"`
	// The x-coordinate to move to.
	X int64 `json:"x,required"`
	// The y-coordinate to move to.
	Y int64 `json:"y,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		X           respjson.Field
		Y           respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseComputerToolCallActionMove) RawJSON() string { return r.JSON.raw }
func (r *ResponseComputerToolCallActionMove) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A screenshot action.
type ResponseComputerToolCallActionScreenshot struct {
	// Specifies the event type. For a screenshot action, this property is always set
	// to `screenshot`.
	Type constant.Screenshot `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseComputerToolCallActionScreenshot) RawJSON() string { return r.JSON.raw }
func (r *ResponseComputerToolCallActionScreenshot) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A scroll action.
type ResponseComputerToolCallActionScroll struct {
	// The horizontal scroll distance.
	ScrollX int64 `json:"scroll_x,required"`
	// The vertical scroll distance.
	ScrollY int64 `json:"scroll_y,required"`
	// Specifies the event type. For a scroll action, this property is always set to
	// `scroll`.
	Type constant.Scroll `json:"type,required"`
	// The x-coordinate where the scroll occurred.
	X int64 `json:"x,required"`
	// The y-coordinate where the scroll occurred.
	Y int64 `json:"y,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ScrollX     respjson.Field
		ScrollY     respjson.Field
		Type        respjson.Field
		X           respjson.Field
		Y           respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseComputerToolCallActionScroll) RawJSON() string { return r.JSON.raw }
func (r *ResponseComputerToolCallActionScroll) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// An action to type in text.
type ResponseComputerToolCallActionType struct {
	// The text to type.
	Text string `json:"text,required"`
	// Specifies the event type. For a type action, this property is always set to
	// `type`.
	Type constant.Type `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Text        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseComputerToolCallActionType) RawJSON() string { return r.JSON.raw }
func (r *ResponseComputerToolCallActionType) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A wait action.
type ResponseComputerToolCallActionWait struct {
	// Specifies the event type. For a wait action, this property is always set to
	// `wait`.
	Type constant.Wait `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseComputerToolCallActionWait) RawJSON() string { return r.JSON.raw }
func (r *ResponseComputerToolCallActionWait) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A pending safety check for the computer call.
type ResponseComputerToolCallPendingSafetyCheck struct {
	// The ID of the pending safety check.
	ID string `json:"id,required"`
	// The type of the pending safety check.
	Code string `json:"code,required"`
	// Details about the pending safety check.
	Message string `json:"message,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Code        respjson.Field
		Message     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseComputerToolCallPendingSafetyCheck) RawJSON() string { return r.JSON.raw }
func (r *ResponseComputerToolCallPendingSafetyCheck) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
// Populated when items are returned via API.
type ResponseComputerToolCallStatus string

const (
	ResponseComputerToolCallStatusInProgress ResponseComputerToolCallStatus = "in_progress"
	ResponseComputerToolCallStatusCompleted  ResponseComputerToolCallStatus = "completed"
	ResponseComputerToolCallStatusIncomplete ResponseComputerToolCallStatus = "incomplete"
)

// The type of the computer call. Always `computer_call`.
type ResponseComputerToolCallType string

const (
	ResponseComputerToolCallTypeComputerCall ResponseComputerToolCallType = "computer_call"
)

// A tool call to a computer use tool. See the
// [computer use guide](https://platform.openai.com/docs/guides/tools-computer-use)
// for more information.
//
// The properties ID, Action, CallID, PendingSafetyChecks, Status, Type are
// required.
type ResponseComputerToolCallParam struct {
	// The unique ID of the computer call.
	ID string `json:"id,required"`
	// A click action.
	Action ResponseComputerToolCallActionUnionParam `json:"action,omitzero,required"`
	// An identifier used when responding to the tool call with output.
	CallID string `json:"call_id,required"`
	// The pending safety checks for the computer call.
	PendingSafetyChecks []ResponseComputerToolCallPendingSafetyCheckParam `json:"pending_safety_checks,omitzero,required"`
	// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
	// Populated when items are returned via API.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status ResponseComputerToolCallStatus `json:"status,omitzero,required"`
	// The type of the computer call. Always `computer_call`.
	//
	// Any of "computer_call".
	Type ResponseComputerToolCallType `json:"type,omitzero,required"`
	paramObj
}

func (r ResponseComputerToolCallParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseComputerToolCallParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ResponseComputerToolCallActionUnionParam struct {
	OfClick       *ResponseComputerToolCallActionClickParam       `json:",omitzero,inline"`
	OfDoubleClick *ResponseComputerToolCallActionDoubleClickParam `json:",omitzero,inline"`
	OfDrag        *ResponseComputerToolCallActionDragParam        `json:",omitzero,inline"`
	OfKeypress    *ResponseComputerToolCallActionKeypressParam    `json:",omitzero,inline"`
	OfMove        *ResponseComputerToolCallActionMoveParam        `json:",omitzero,inline"`
	OfScreenshot  *ResponseComputerToolCallActionScreenshotParam  `json:",omitzero,inline"`
	OfScroll      *ResponseComputerToolCallActionScrollParam      `json:",omitzero,inline"`
	OfType        *ResponseComputerToolCallActionTypeParam        `json:",omitzero,inline"`
	OfWait        *ResponseComputerToolCallActionWaitParam        `json:",omitzero,inline"`
	paramUnion
}

func (u ResponseComputerToolCallActionUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfClick,
		u.OfDoubleClick,
		u.OfDrag,
		u.OfKeypress,
		u.OfMove,
		u.OfScreenshot,
		u.OfScroll,
		u.OfType,
		u.OfWait)
}
func (u *ResponseComputerToolCallActionUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ResponseComputerToolCallActionUnionParam) asAny() any {
	if !param.IsOmitted(u.OfClick) {
		return u.OfClick
	} else if !param.IsOmitted(u.OfDoubleClick) {
		return u.OfDoubleClick
	} else if !param.IsOmitted(u.OfDrag) {
		return u.OfDrag
	} else if !param.IsOmitted(u.OfKeypress) {
		return u.OfKeypress
	} else if !param.IsOmitted(u.OfMove) {
		return u.OfMove
	} else if !param.IsOmitted(u.OfScreenshot) {
		return u.OfScreenshot
	} else if !param.IsOmitted(u.OfScroll) {
		return u.OfScroll
	} else if !param.IsOmitted(u.OfType) {
		return u.OfType
	} else if !param.IsOmitted(u.OfWait) {
		return u.OfWait
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseComputerToolCallActionUnionParam) GetButton() *string {
	if vt := u.OfClick; vt != nil {
		return &vt.Button
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseComputerToolCallActionUnionParam) GetPath() []ResponseComputerToolCallActionDragPathParam {
	if vt := u.OfDrag; vt != nil {
		return vt.Path
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseComputerToolCallActionUnionParam) GetKeys() []string {
	if vt := u.OfKeypress; vt != nil {
		return vt.Keys
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseComputerToolCallActionUnionParam) GetScrollX() *int64 {
	if vt := u.OfScroll; vt != nil {
		return &vt.ScrollX
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseComputerToolCallActionUnionParam) GetScrollY() *int64 {
	if vt := u.OfScroll; vt != nil {
		return &vt.ScrollY
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseComputerToolCallActionUnionParam) GetText() *string {
	if vt := u.OfType; vt != nil {
		return &vt.Text
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseComputerToolCallActionUnionParam) GetType() *string {
	if vt := u.OfClick; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfDoubleClick; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfDrag; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfKeypress; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfMove; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfScreenshot; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfScroll; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfType; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfWait; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseComputerToolCallActionUnionParam) GetX() *int64 {
	if vt := u.OfClick; vt != nil {
		return (*int64)(&vt.X)
	} else if vt := u.OfDoubleClick; vt != nil {
		return (*int64)(&vt.X)
	} else if vt := u.OfMove; vt != nil {
		return (*int64)(&vt.X)
	} else if vt := u.OfScroll; vt != nil {
		return (*int64)(&vt.X)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseComputerToolCallActionUnionParam) GetY() *int64 {
	if vt := u.OfClick; vt != nil {
		return (*int64)(&vt.Y)
	} else if vt := u.OfDoubleClick; vt != nil {
		return (*int64)(&vt.Y)
	} else if vt := u.OfMove; vt != nil {
		return (*int64)(&vt.Y)
	} else if vt := u.OfScroll; vt != nil {
		return (*int64)(&vt.Y)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[ResponseComputerToolCallActionUnionParam](
		"type",
		apijson.Discriminator[ResponseComputerToolCallActionClickParam]("click"),
		apijson.Discriminator[ResponseComputerToolCallActionDoubleClickParam]("double_click"),
		apijson.Discriminator[ResponseComputerToolCallActionDragParam]("drag"),
		apijson.Discriminator[ResponseComputerToolCallActionKeypressParam]("keypress"),
		apijson.Discriminator[ResponseComputerToolCallActionMoveParam]("move"),
		apijson.Discriminator[ResponseComputerToolCallActionScreenshotParam]("screenshot"),
		apijson.Discriminator[ResponseComputerToolCallActionScrollParam]("scroll"),
		apijson.Discriminator[ResponseComputerToolCallActionTypeParam]("type"),
		apijson.Discriminator[ResponseComputerToolCallActionWaitParam]("wait"),
	)
}

// A click action.
//
// The properties Button, Type, X, Y are required.
type ResponseComputerToolCallActionClickParam struct {
	// Indicates which mouse button was pressed during the click. One of `left`,
	// `right`, `wheel`, `back`, or `forward`.
	//
	// Any of "left", "right", "wheel", "back", "forward".
	Button string `json:"button,omitzero,required"`
	// The x-coordinate where the click occurred.
	X int64 `json:"x,required"`
	// The y-coordinate where the click occurred.
	Y int64 `json:"y,required"`
	// Specifies the event type. For a click action, this property is always set to
	// `click`.
	//
	// This field can be elided, and will marshal its zero value as "click".
	Type constant.Click `json:"type,required"`
	paramObj
}

func (r ResponseComputerToolCallActionClickParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionClickParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseComputerToolCallActionClickParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[ResponseComputerToolCallActionClickParam](
		"button", "left", "right", "wheel", "back", "forward",
	)
}

// A double click action.
//
// The properties Type, X, Y are required.
type ResponseComputerToolCallActionDoubleClickParam struct {
	// The x-coordinate where the double click occurred.
	X int64 `json:"x,required"`
	// The y-coordinate where the double click occurred.
	Y int64 `json:"y,required"`
	// Specifies the event type. For a double click action, this property is always set
	// to `double_click`.
	//
	// This field can be elided, and will marshal its zero value as "double_click".
	Type constant.DoubleClick `json:"type,required"`
	paramObj
}

func (r ResponseComputerToolCallActionDoubleClickParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionDoubleClickParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseComputerToolCallActionDoubleClickParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A drag action.
//
// The properties Path, Type are required.
type ResponseComputerToolCallActionDragParam struct {
	// An array of coordinates representing the path of the drag action. Coordinates
	// will appear as an array of objects, eg
	//
	// ```
	// [
	//
	//	{ x: 100, y: 200 },
	//	{ x: 200, y: 300 }
	//
	// ]
	// ```
	Path []ResponseComputerToolCallActionDragPathParam `json:"path,omitzero,required"`
	// Specifies the event type. For a drag action, this property is always set to
	// `drag`.
	//
	// This field can be elided, and will marshal its zero value as "drag".
	Type constant.Drag `json:"type,required"`
	paramObj
}

func (r ResponseComputerToolCallActionDragParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionDragParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseComputerToolCallActionDragParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A series of x/y coordinate pairs in the drag path.
//
// The properties X, Y are required.
type ResponseComputerToolCallActionDragPathParam struct {
	// The x-coordinate.
	X int64 `json:"x,required"`
	// The y-coordinate.
	Y int64 `json:"y,required"`
	paramObj
}

func (r ResponseComputerToolCallActionDragPathParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionDragPathParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseComputerToolCallActionDragPathParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A collection of keypresses the model would like to perform.
//
// The properties Keys, Type are required.
type ResponseComputerToolCallActionKeypressParam struct {
	// The combination of keys the model is requesting to be pressed. This is an array
	// of strings, each representing a key.
	Keys []string `json:"keys,omitzero,required"`
	// Specifies the event type. For a keypress action, this property is always set to
	// `keypress`.
	//
	// This field can be elided, and will marshal its zero value as "keypress".
	Type constant.Keypress `json:"type,required"`
	paramObj
}

func (r ResponseComputerToolCallActionKeypressParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionKeypressParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseComputerToolCallActionKeypressParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A mouse move action.
//
// The properties Type, X, Y are required.
type ResponseComputerToolCallActionMoveParam struct {
	// The x-coordinate to move to.
	X int64 `json:"x,required"`
	// The y-coordinate to move to.
	Y int64 `json:"y,required"`
	// Specifies the event type. For a move action, this property is always set to
	// `move`.
	//
	// This field can be elided, and will marshal its zero value as "move".
	Type constant.Move `json:"type,required"`
	paramObj
}

func (r ResponseComputerToolCallActionMoveParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionMoveParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseComputerToolCallActionMoveParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func NewResponseComputerToolCallActionScreenshotParam() ResponseComputerToolCallActionScreenshotParam {
	return ResponseComputerToolCallActionScreenshotParam{
		Type: "screenshot",
	}
}

// A screenshot action.
//
// This struct has a constant value, construct it with
// [NewResponseComputerToolCallActionScreenshotParam].
type ResponseComputerToolCallActionScreenshotParam struct {
	// Specifies the event type. For a screenshot action, this property is always set
	// to `screenshot`.
	Type constant.Screenshot `json:"type,required"`
	paramObj
}

func (r ResponseComputerToolCallActionScreenshotParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionScreenshotParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseComputerToolCallActionScreenshotParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A scroll action.
//
// The properties ScrollX, ScrollY, Type, X, Y are required.
type ResponseComputerToolCallActionScrollParam struct {
	// The horizontal scroll distance.
	ScrollX int64 `json:"scroll_x,required"`
	// The vertical scroll distance.
	ScrollY int64 `json:"scroll_y,required"`
	// The x-coordinate where the scroll occurred.
	X int64 `json:"x,required"`
	// The y-coordinate where the scroll occurred.
	Y int64 `json:"y,required"`
	// Specifies the event type. For a scroll action, this property is always set to
	// `scroll`.
	//
	// This field can be elided, and will marshal its zero value as "scroll".
	Type constant.Scroll `json:"type,required"`
	paramObj
}

func (r ResponseComputerToolCallActionScrollParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionScrollParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseComputerToolCallActionScrollParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// An action to type in text.
//
// The properties Text, Type are required.
type ResponseComputerToolCallActionTypeParam struct {
	// The text to type.
	Text string `json:"text,required"`
	// Specifies the event type. For a type action, this property is always set to
	// `type`.
	//
	// This field can be elided, and will marshal its zero value as "type".
	Type constant.Type `json:"type,required"`
	paramObj
}

func (r ResponseComputerToolCallActionTypeParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionTypeParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseComputerToolCallActionTypeParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func NewResponseComputerToolCallActionWaitParam() ResponseComputerToolCallActionWaitParam {
	return ResponseComputerToolCallActionWaitParam{
		Type: "wait",
	}
}

// A wait action.
//
// This struct has a constant value, construct it with
// [NewResponseComputerToolCallActionWaitParam].
type ResponseComputerToolCallActionWaitParam struct {
	// Specifies the event type. For a wait action, this property is always set to
	// `wait`.
	Type constant.Wait `json:"type,required"`
	paramObj
}

func (r ResponseComputerToolCallActionWaitParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionWaitParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseComputerToolCallActionWaitParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A pending safety check for the computer call.
//
// The properties ID, Code, Message are required.
type ResponseComputerToolCallPendingSafetyCheckParam struct {
	// The ID of the pending safety check.
	ID string `json:"id,required"`
	// The type of the pending safety check.
	Code string `json:"code,required"`
	// Details about the pending safety check.
	Message string `json:"message,required"`
	paramObj
}

func (r ResponseComputerToolCallPendingSafetyCheckParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallPendingSafetyCheckParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseComputerToolCallPendingSafetyCheckParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ResponseComputerToolCallOutputItem struct {
	// The unique ID of the computer call tool output.
	ID string `json:"id,required"`
	// The ID of the computer tool call that produced the output.
	CallID string `json:"call_id,required"`
	// A computer screenshot image used with the computer use tool.
	Output ResponseComputerToolCallOutputScreenshot `json:"output,required"`
	// The type of the computer tool call output. Always `computer_call_output`.
	Type constant.ComputerCallOutput `json:"type,required"`
	// The safety checks reported by the API that have been acknowledged by the
	// developer.
	AcknowledgedSafetyChecks []ResponseComputerToolCallOutputItemAcknowledgedSafetyCheck `json:"acknowledged_safety_checks"`
	// The status of the message input. One of `in_progress`, `completed`, or
	// `incomplete`. Populated when input items are returned via API.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status ResponseComputerToolCallOutputItemStatus `json:"status"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID                       respjson.Field
		CallID                   respjson.Field
		Output                   respjson.Field
		Type                     respjson.Field
		AcknowledgedSafetyChecks respjson.Field
		Status                   respjson.Field
		ExtraFields              map[string]respjson.Field
		raw                      string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseComputerToolCallOutputItem) RawJSON() string { return r.JSON.raw }
func (r *ResponseComputerToolCallOutputItem) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A pending safety check for the computer call.
type ResponseComputerToolCallOutputItemAcknowledgedSafetyCheck struct {
	// The ID of the pending safety check.
	ID string `json:"id,required"`
	// The type of the pending safety check.
	Code string `json:"code,required"`
	// Details about the pending safety check.
	Message string `json:"message,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Code        respjson.Field
		Message     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseComputerToolCallOutputItemAcknowledgedSafetyCheck) RawJSON() string {
	return r.JSON.raw
}
func (r *ResponseComputerToolCallOutputItemAcknowledgedSafetyCheck) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The status of the message input. One of `in_progress`, `completed`, or
// `incomplete`. Populated when input items are returned via API.
type ResponseComputerToolCallOutputItemStatus string

const (
	ResponseComputerToolCallOutputItemStatusInProgress ResponseComputerToolCallOutputItemStatus = "in_progress"
	ResponseComputerToolCallOutputItemStatusCompleted  ResponseComputerToolCallOutputItemStatus = "completed"
	ResponseComputerToolCallOutputItemStatusIncomplete ResponseComputerToolCallOutputItemStatus = "incomplete"
)

// A computer screenshot image used with the computer use tool.
type ResponseComputerToolCallOutputScreenshot struct {
	// Specifies the event type. For a computer screenshot, this property is always set
	// to `computer_screenshot`.
	Type constant.ComputerScreenshot `json:"type,required"`
	// The identifier of an uploaded file that contains the screenshot.
	FileID string `json:"file_id"`
	// The URL of the screenshot image.
	ImageURL string `json:"image_url"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		FileID      respjson.Field
		ImageURL    respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseComputerToolCallOutputScreenshot) RawJSON() string { return r.JSON.raw }
func (r *ResponseComputerToolCallOutputScreenshot) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponseComputerToolCallOutputScreenshot to a
// ResponseComputerToolCallOutputScreenshotParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseComputerToolCallOutputScreenshotParam.Overrides()
func (r ResponseComputerToolCallOutputScreenshot) ToParam() ResponseComputerToolCallOutputScreenshotParam {
	return param.Override[ResponseComputerToolCallOutputScreenshotParam](json.RawMessage(r.RawJSON()))
}

// A computer screenshot image used with the computer use tool.
//
// The property Type is required.
type ResponseComputerToolCallOutputScreenshotParam struct {
	// The identifier of an uploaded file that contains the screenshot.
	FileID param.Opt[string] `json:"file_id,omitzero"`
	// The URL of the screenshot image.
	ImageURL param.Opt[string] `json:"image_url,omitzero"`
	// Specifies the event type. For a computer screenshot, this property is always set
	// to `computer_screenshot`.
	//
	// This field can be elided, and will marshal its zero value as
	// "computer_screenshot".
	Type constant.ComputerScreenshot `json:"type,required"`
	paramObj
}

func (r ResponseComputerToolCallOutputScreenshotParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallOutputScreenshotParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseComputerToolCallOutputScreenshotParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a new content part is added.
type ResponseContentPartAddedEvent struct {
	// The index of the content part that was added.
	ContentIndex int64 `json:"content_index,required"`
	// The ID of the output item that the content part was added to.
	ItemID string `json:"item_id,required"`
	// The index of the output item that the content part was added to.
	OutputIndex int64 `json:"output_index,required"`
	// The content part that was added.
	Part ResponseContentPartAddedEventPartUnion `json:"part,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.content_part.added`.
	Type constant.ResponseContentPartAdded `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ContentIndex   respjson.Field
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		Part           respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseContentPartAddedEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseContentPartAddedEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseContentPartAddedEventPartUnion contains all possible properties and
// values from [ResponseOutputText], [ResponseOutputRefusal].
//
// Use the [ResponseContentPartAddedEventPartUnion.AsAny] method to switch on the
// variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ResponseContentPartAddedEventPartUnion struct {
	// This field is from variant [ResponseOutputText].
	Annotations []ResponseOutputTextAnnotationUnion `json:"annotations"`
	// This field is from variant [ResponseOutputText].
	Text string `json:"text"`
	// Any of "output_text", "refusal".
	Type string `json:"type"`
	// This field is from variant [ResponseOutputText].
	Logprobs []ResponseOutputTextLogprob `json:"logprobs"`
	// This field is from variant [ResponseOutputRefusal].
	Refusal string `json:"refusal"`
	JSON    struct {
		Annotations respjson.Field
		Text        respjson.Field
		Type        respjson.Field
		Logprobs    respjson.Field
		Refusal     respjson.Field
		raw         string
	} `json:"-"`
}

// anyResponseContentPartAddedEventPart is implemented by each variant of
// [ResponseContentPartAddedEventPartUnion] to add type safety for the return type
// of [ResponseContentPartAddedEventPartUnion.AsAny]
type anyResponseContentPartAddedEventPart interface {
	implResponseContentPartAddedEventPartUnion()
}

func (ResponseOutputText) implResponseContentPartAddedEventPartUnion()    {}
func (ResponseOutputRefusal) implResponseContentPartAddedEventPartUnion() {}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseContentPartAddedEventPartUnion.AsAny().(type) {
//	case responses.ResponseOutputText:
//	case responses.ResponseOutputRefusal:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseContentPartAddedEventPartUnion) AsAny() anyResponseContentPartAddedEventPart {
	switch u.Type {
	case "output_text":
		return u.AsOutputText()
	case "refusal":
		return u.AsRefusal()
	}
	return nil
}

func (u ResponseContentPartAddedEventPartUnion) AsOutputText() (v ResponseOutputText) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseContentPartAddedEventPartUnion) AsRefusal() (v ResponseOutputRefusal) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ResponseContentPartAddedEventPartUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseContentPartAddedEventPartUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a content part is done.
type ResponseContentPartDoneEvent struct {
	// The index of the content part that is done.
	ContentIndex int64 `json:"content_index,required"`
	// The ID of the output item that the content part was added to.
	ItemID string `json:"item_id,required"`
	// The index of the output item that the content part was added to.
	OutputIndex int64 `json:"output_index,required"`
	// The content part that is done.
	Part ResponseContentPartDoneEventPartUnion `json:"part,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.content_part.done`.
	Type constant.ResponseContentPartDone `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ContentIndex   respjson.Field
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		Part           respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseContentPartDoneEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseContentPartDoneEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseContentPartDoneEventPartUnion contains all possible properties and
// values from [ResponseOutputText], [ResponseOutputRefusal].
//
// Use the [ResponseContentPartDoneEventPartUnion.AsAny] method to switch on the
// variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ResponseContentPartDoneEventPartUnion struct {
	// This field is from variant [ResponseOutputText].
	Annotations []ResponseOutputTextAnnotationUnion `json:"annotations"`
	// This field is from variant [ResponseOutputText].
	Text string `json:"text"`
	// Any of "output_text", "refusal".
	Type string `json:"type"`
	// This field is from variant [ResponseOutputText].
	Logprobs []ResponseOutputTextLogprob `json:"logprobs"`
	// This field is from variant [ResponseOutputRefusal].
	Refusal string `json:"refusal"`
	JSON    struct {
		Annotations respjson.Field
		Text        respjson.Field
		Type        respjson.Field
		Logprobs    respjson.Field
		Refusal     respjson.Field
		raw         string
	} `json:"-"`
}

// anyResponseContentPartDoneEventPart is implemented by each variant of
// [ResponseContentPartDoneEventPartUnion] to add type safety for the return type
// of [ResponseContentPartDoneEventPartUnion.AsAny]
type anyResponseContentPartDoneEventPart interface {
	implResponseContentPartDoneEventPartUnion()
}

func (ResponseOutputText) implResponseContentPartDoneEventPartUnion()    {}
func (ResponseOutputRefusal) implResponseContentPartDoneEventPartUnion() {}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseContentPartDoneEventPartUnion.AsAny().(type) {
//	case responses.ResponseOutputText:
//	case responses.ResponseOutputRefusal:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseContentPartDoneEventPartUnion) AsAny() anyResponseContentPartDoneEventPart {
	switch u.Type {
	case "output_text":
		return u.AsOutputText()
	case "refusal":
		return u.AsRefusal()
	}
	return nil
}

func (u ResponseContentPartDoneEventPartUnion) AsOutputText() (v ResponseOutputText) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseContentPartDoneEventPartUnion) AsRefusal() (v ResponseOutputRefusal) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ResponseContentPartDoneEventPartUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseContentPartDoneEventPartUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// An event that is emitted when a response is created.
type ResponseCreatedEvent struct {
	// The response that was created.
	Response Response `json:"response,required"`
	// The sequence number for this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.created`.
	Type constant.ResponseCreated `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Response       respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCreatedEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseCreatedEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// An error object returned when the model fails to generate a Response.
type ResponseError struct {
	// The error code for the response.
	//
	// Any of "server_error", "rate_limit_exceeded", "invalid_prompt",
	// "vector_store_timeout", "invalid_image", "invalid_image_format",
	// "invalid_base64_image", "invalid_image_url", "image_too_large",
	// "image_too_small", "image_parse_error", "image_content_policy_violation",
	// "invalid_image_mode", "image_file_too_large", "unsupported_image_media_type",
	// "empty_image_file", "failed_to_download_image", "image_file_not_found".
	Code ResponseErrorCode `json:"code,required"`
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
func (r ResponseError) RawJSON() string { return r.JSON.raw }
func (r *ResponseError) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The error code for the response.
type ResponseErrorCode string

const (
	ResponseErrorCodeServerError                 ResponseErrorCode = "server_error"
	ResponseErrorCodeRateLimitExceeded           ResponseErrorCode = "rate_limit_exceeded"
	ResponseErrorCodeInvalidPrompt               ResponseErrorCode = "invalid_prompt"
	ResponseErrorCodeVectorStoreTimeout          ResponseErrorCode = "vector_store_timeout"
	ResponseErrorCodeInvalidImage                ResponseErrorCode = "invalid_image"
	ResponseErrorCodeInvalidImageFormat          ResponseErrorCode = "invalid_image_format"
	ResponseErrorCodeInvalidBase64Image          ResponseErrorCode = "invalid_base64_image"
	ResponseErrorCodeInvalidImageURL             ResponseErrorCode = "invalid_image_url"
	ResponseErrorCodeImageTooLarge               ResponseErrorCode = "image_too_large"
	ResponseErrorCodeImageTooSmall               ResponseErrorCode = "image_too_small"
	ResponseErrorCodeImageParseError             ResponseErrorCode = "image_parse_error"
	ResponseErrorCodeImageContentPolicyViolation ResponseErrorCode = "image_content_policy_violation"
	ResponseErrorCodeInvalidImageMode            ResponseErrorCode = "invalid_image_mode"
	ResponseErrorCodeImageFileTooLarge           ResponseErrorCode = "image_file_too_large"
	ResponseErrorCodeUnsupportedImageMediaType   ResponseErrorCode = "unsupported_image_media_type"
	ResponseErrorCodeEmptyImageFile              ResponseErrorCode = "empty_image_file"
	ResponseErrorCodeFailedToDownloadImage       ResponseErrorCode = "failed_to_download_image"
	ResponseErrorCodeImageFileNotFound           ResponseErrorCode = "image_file_not_found"
)

// Emitted when an error occurs.
type ResponseErrorEvent struct {
	// The error code.
	Code string `json:"code,required"`
	// The error message.
	Message string `json:"message,required"`
	// The error parameter.
	Param string `json:"param,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `error`.
	Type constant.Error `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Code           respjson.Field
		Message        respjson.Field
		Param          respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseErrorEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseErrorEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// An event that is emitted when a response fails.
type ResponseFailedEvent struct {
	// The response that failed.
	Response Response `json:"response,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.failed`.
	Type constant.ResponseFailed `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Response       respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFailedEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseFailedEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a file search call is completed (results found).
type ResponseFileSearchCallCompletedEvent struct {
	// The ID of the output item that the file search call is initiated.
	ItemID string `json:"item_id,required"`
	// The index of the output item that the file search call is initiated.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.file_search_call.completed`.
	Type constant.ResponseFileSearchCallCompleted `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFileSearchCallCompletedEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseFileSearchCallCompletedEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a file search call is initiated.
type ResponseFileSearchCallInProgressEvent struct {
	// The ID of the output item that the file search call is initiated.
	ItemID string `json:"item_id,required"`
	// The index of the output item that the file search call is initiated.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.file_search_call.in_progress`.
	Type constant.ResponseFileSearchCallInProgress `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFileSearchCallInProgressEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseFileSearchCallInProgressEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a file search is currently searching.
type ResponseFileSearchCallSearchingEvent struct {
	// The ID of the output item that the file search call is initiated.
	ItemID string `json:"item_id,required"`
	// The index of the output item that the file search call is searching.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.file_search_call.searching`.
	Type constant.ResponseFileSearchCallSearching `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFileSearchCallSearchingEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseFileSearchCallSearchingEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The results of a file search tool call. See the
// [file search guide](https://platform.openai.com/docs/guides/tools-file-search)
// for more information.
type ResponseFileSearchToolCall struct {
	// The unique ID of the file search tool call.
	ID string `json:"id,required"`
	// The queries used to search for files.
	Queries []string `json:"queries,required"`
	// The status of the file search tool call. One of `in_progress`, `searching`,
	// `incomplete` or `failed`,
	//
	// Any of "in_progress", "searching", "completed", "incomplete", "failed".
	Status ResponseFileSearchToolCallStatus `json:"status,required"`
	// The type of the file search tool call. Always `file_search_call`.
	Type constant.FileSearchCall `json:"type,required"`
	// The results of the file search tool call.
	Results []ResponseFileSearchToolCallResult `json:"results,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Queries     respjson.Field
		Status      respjson.Field
		Type        respjson.Field
		Results     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFileSearchToolCall) RawJSON() string { return r.JSON.raw }
func (r *ResponseFileSearchToolCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponseFileSearchToolCall to a
// ResponseFileSearchToolCallParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseFileSearchToolCallParam.Overrides()
func (r ResponseFileSearchToolCall) ToParam() ResponseFileSearchToolCallParam {
	return param.Override[ResponseFileSearchToolCallParam](json.RawMessage(r.RawJSON()))
}

// The status of the file search tool call. One of `in_progress`, `searching`,
// `incomplete` or `failed`,
type ResponseFileSearchToolCallStatus string

const (
	ResponseFileSearchToolCallStatusInProgress ResponseFileSearchToolCallStatus = "in_progress"
	ResponseFileSearchToolCallStatusSearching  ResponseFileSearchToolCallStatus = "searching"
	ResponseFileSearchToolCallStatusCompleted  ResponseFileSearchToolCallStatus = "completed"
	ResponseFileSearchToolCallStatusIncomplete ResponseFileSearchToolCallStatus = "incomplete"
	ResponseFileSearchToolCallStatusFailed     ResponseFileSearchToolCallStatus = "failed"
)

type ResponseFileSearchToolCallResult struct {
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard. Keys are strings with a maximum
	// length of 64 characters. Values are strings with a maximum length of 512
	// characters, booleans, or numbers.
	Attributes map[string]ResponseFileSearchToolCallResultAttributeUnion `json:"attributes,nullable"`
	// The unique ID of the file.
	FileID string `json:"file_id"`
	// The name of the file.
	Filename string `json:"filename"`
	// The relevance score of the file - a value between 0 and 1.
	Score float64 `json:"score"`
	// The text that was retrieved from the file.
	Text string `json:"text"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Attributes  respjson.Field
		FileID      respjson.Field
		Filename    respjson.Field
		Score       respjson.Field
		Text        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFileSearchToolCallResult) RawJSON() string { return r.JSON.raw }
func (r *ResponseFileSearchToolCallResult) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseFileSearchToolCallResultAttributeUnion contains all possible properties
// and values from [string], [float64], [bool].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfString OfFloat OfBool]
type ResponseFileSearchToolCallResultAttributeUnion struct {
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

func (u ResponseFileSearchToolCallResultAttributeUnion) AsString() (v string) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseFileSearchToolCallResultAttributeUnion) AsFloat() (v float64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseFileSearchToolCallResultAttributeUnion) AsBool() (v bool) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ResponseFileSearchToolCallResultAttributeUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseFileSearchToolCallResultAttributeUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The results of a file search tool call. See the
// [file search guide](https://platform.openai.com/docs/guides/tools-file-search)
// for more information.
//
// The properties ID, Queries, Status, Type are required.
type ResponseFileSearchToolCallParam struct {
	// The unique ID of the file search tool call.
	ID string `json:"id,required"`
	// The queries used to search for files.
	Queries []string `json:"queries,omitzero,required"`
	// The status of the file search tool call. One of `in_progress`, `searching`,
	// `incomplete` or `failed`,
	//
	// Any of "in_progress", "searching", "completed", "incomplete", "failed".
	Status ResponseFileSearchToolCallStatus `json:"status,omitzero,required"`
	// The results of the file search tool call.
	Results []ResponseFileSearchToolCallResultParam `json:"results,omitzero"`
	// The type of the file search tool call. Always `file_search_call`.
	//
	// This field can be elided, and will marshal its zero value as "file_search_call".
	Type constant.FileSearchCall `json:"type,required"`
	paramObj
}

func (r ResponseFileSearchToolCallParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseFileSearchToolCallParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseFileSearchToolCallParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ResponseFileSearchToolCallResultParam struct {
	// The unique ID of the file.
	FileID param.Opt[string] `json:"file_id,omitzero"`
	// The name of the file.
	Filename param.Opt[string] `json:"filename,omitzero"`
	// The relevance score of the file - a value between 0 and 1.
	Score param.Opt[float64] `json:"score,omitzero"`
	// The text that was retrieved from the file.
	Text param.Opt[string] `json:"text,omitzero"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard. Keys are strings with a maximum
	// length of 64 characters. Values are strings with a maximum length of 512
	// characters, booleans, or numbers.
	Attributes map[string]ResponseFileSearchToolCallResultAttributeUnionParam `json:"attributes,omitzero"`
	paramObj
}

func (r ResponseFileSearchToolCallResultParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseFileSearchToolCallResultParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseFileSearchToolCallResultParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ResponseFileSearchToolCallResultAttributeUnionParam struct {
	OfString param.Opt[string]  `json:",omitzero,inline"`
	OfFloat  param.Opt[float64] `json:",omitzero,inline"`
	OfBool   param.Opt[bool]    `json:",omitzero,inline"`
	paramUnion
}

func (u ResponseFileSearchToolCallResultAttributeUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfFloat, u.OfBool)
}
func (u *ResponseFileSearchToolCallResultAttributeUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ResponseFileSearchToolCallResultAttributeUnionParam) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfFloat) {
		return &u.OfFloat.Value
	} else if !param.IsOmitted(u.OfBool) {
		return &u.OfBool.Value
	}
	return nil
}

// ResponseFormatTextConfigUnion contains all possible properties and values from
// [shared.ResponseFormatText], [ResponseFormatTextJSONSchemaConfig],
// [shared.ResponseFormatJSONObject].
//
// Use the [ResponseFormatTextConfigUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ResponseFormatTextConfigUnion struct {
	// Any of "text", "json_schema", "json_object".
	Type string `json:"type"`
	// This field is from variant [ResponseFormatTextJSONSchemaConfig].
	Name string `json:"name"`
	// This field is from variant [ResponseFormatTextJSONSchemaConfig].
	Schema map[string]any `json:"schema"`
	// This field is from variant [ResponseFormatTextJSONSchemaConfig].
	Description string `json:"description"`
	// This field is from variant [ResponseFormatTextJSONSchemaConfig].
	Strict bool `json:"strict"`
	JSON   struct {
		Type        respjson.Field
		Name        respjson.Field
		Schema      respjson.Field
		Description respjson.Field
		Strict      respjson.Field
		raw         string
	} `json:"-"`
}

// anyResponseFormatTextConfig is implemented by each variant of
// [ResponseFormatTextConfigUnion] to add type safety for the return type of
// [ResponseFormatTextConfigUnion.AsAny]
type anyResponseFormatTextConfig interface {
	ImplResponseFormatTextConfigUnion()
}

func (ResponseFormatTextJSONSchemaConfig) ImplResponseFormatTextConfigUnion() {}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseFormatTextConfigUnion.AsAny().(type) {
//	case shared.ResponseFormatText:
//	case responses.ResponseFormatTextJSONSchemaConfig:
//	case shared.ResponseFormatJSONObject:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseFormatTextConfigUnion) AsAny() anyResponseFormatTextConfig {
	switch u.Type {
	case "text":
		return u.AsText()
	case "json_schema":
		return u.AsJSONSchema()
	case "json_object":
		return u.AsJSONObject()
	}
	return nil
}

func (u ResponseFormatTextConfigUnion) AsText() (v shared.ResponseFormatText) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseFormatTextConfigUnion) AsJSONSchema() (v ResponseFormatTextJSONSchemaConfig) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseFormatTextConfigUnion) AsJSONObject() (v shared.ResponseFormatJSONObject) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ResponseFormatTextConfigUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseFormatTextConfigUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponseFormatTextConfigUnion to a
// ResponseFormatTextConfigUnionParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseFormatTextConfigUnionParam.Overrides()
func (r ResponseFormatTextConfigUnion) ToParam() ResponseFormatTextConfigUnionParam {
	return param.Override[ResponseFormatTextConfigUnionParam](json.RawMessage(r.RawJSON()))
}

func ResponseFormatTextConfigParamOfJSONSchema(name string, schema map[string]any) ResponseFormatTextConfigUnionParam {
	var jsonSchema ResponseFormatTextJSONSchemaConfigParam
	jsonSchema.Name = name
	jsonSchema.Schema = schema
	return ResponseFormatTextConfigUnionParam{OfJSONSchema: &jsonSchema}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ResponseFormatTextConfigUnionParam struct {
	OfText       *shared.ResponseFormatTextParam          `json:",omitzero,inline"`
	OfJSONSchema *ResponseFormatTextJSONSchemaConfigParam `json:",omitzero,inline"`
	OfJSONObject *shared.ResponseFormatJSONObjectParam    `json:",omitzero,inline"`
	paramUnion
}

func (u ResponseFormatTextConfigUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfText, u.OfJSONSchema, u.OfJSONObject)
}
func (u *ResponseFormatTextConfigUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ResponseFormatTextConfigUnionParam) asAny() any {
	if !param.IsOmitted(u.OfText) {
		return u.OfText
	} else if !param.IsOmitted(u.OfJSONSchema) {
		return u.OfJSONSchema
	} else if !param.IsOmitted(u.OfJSONObject) {
		return u.OfJSONObject
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseFormatTextConfigUnionParam) GetName() *string {
	if vt := u.OfJSONSchema; vt != nil {
		return &vt.Name
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseFormatTextConfigUnionParam) GetSchema() map[string]any {
	if vt := u.OfJSONSchema; vt != nil {
		return vt.Schema
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseFormatTextConfigUnionParam) GetDescription() *string {
	if vt := u.OfJSONSchema; vt != nil && vt.Description.Valid() {
		return &vt.Description.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseFormatTextConfigUnionParam) GetStrict() *bool {
	if vt := u.OfJSONSchema; vt != nil && vt.Strict.Valid() {
		return &vt.Strict.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseFormatTextConfigUnionParam) GetType() *string {
	if vt := u.OfText; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfJSONSchema; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfJSONObject; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[ResponseFormatTextConfigUnionParam](
		"type",
		apijson.Discriminator[shared.ResponseFormatTextParam]("text"),
		apijson.Discriminator[ResponseFormatTextJSONSchemaConfigParam]("json_schema"),
		apijson.Discriminator[shared.ResponseFormatJSONObjectParam]("json_object"),
	)
}

// JSON Schema response format. Used to generate structured JSON responses. Learn
// more about
// [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs).
type ResponseFormatTextJSONSchemaConfig struct {
	// The name of the response format. Must be a-z, A-Z, 0-9, or contain underscores
	// and dashes, with a maximum length of 64.
	Name string `json:"name,required"`
	// The schema for the response format, described as a JSON Schema object. Learn how
	// to build JSON schemas [here](https://json-schema.org/).
	Schema map[string]any `json:"schema,required"`
	// The type of response format being defined. Always `json_schema`.
	Type constant.JSONSchema `json:"type,required"`
	// A description of what the response format is for, used by the model to determine
	// how to respond in the format.
	Description string `json:"description"`
	// Whether to enable strict schema adherence when generating the output. If set to
	// true, the model will always follow the exact schema defined in the `schema`
	// field. Only a subset of JSON Schema is supported when `strict` is `true`. To
	// learn more, read the
	// [Structured Outputs guide](https://platform.openai.com/docs/guides/structured-outputs).
	Strict bool `json:"strict,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Name        respjson.Field
		Schema      respjson.Field
		Type        respjson.Field
		Description respjson.Field
		Strict      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFormatTextJSONSchemaConfig) RawJSON() string { return r.JSON.raw }
func (r *ResponseFormatTextJSONSchemaConfig) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponseFormatTextJSONSchemaConfig to a
// ResponseFormatTextJSONSchemaConfigParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseFormatTextJSONSchemaConfigParam.Overrides()
func (r ResponseFormatTextJSONSchemaConfig) ToParam() ResponseFormatTextJSONSchemaConfigParam {
	return param.Override[ResponseFormatTextJSONSchemaConfigParam](json.RawMessage(r.RawJSON()))
}

// JSON Schema response format. Used to generate structured JSON responses. Learn
// more about
// [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs).
//
// The properties Name, Schema, Type are required.
type ResponseFormatTextJSONSchemaConfigParam struct {
	// The name of the response format. Must be a-z, A-Z, 0-9, or contain underscores
	// and dashes, with a maximum length of 64.
	Name string `json:"name,required"`
	// The schema for the response format, described as a JSON Schema object. Learn how
	// to build JSON schemas [here](https://json-schema.org/).
	Schema map[string]any `json:"schema,omitzero,required"`
	// Whether to enable strict schema adherence when generating the output. If set to
	// true, the model will always follow the exact schema defined in the `schema`
	// field. Only a subset of JSON Schema is supported when `strict` is `true`. To
	// learn more, read the
	// [Structured Outputs guide](https://platform.openai.com/docs/guides/structured-outputs).
	Strict param.Opt[bool] `json:"strict,omitzero"`
	// A description of what the response format is for, used by the model to determine
	// how to respond in the format.
	Description param.Opt[string] `json:"description,omitzero"`
	// The type of response format being defined. Always `json_schema`.
	//
	// This field can be elided, and will marshal its zero value as "json_schema".
	Type constant.JSONSchema `json:"type,required"`
	paramObj
}

func (r ResponseFormatTextJSONSchemaConfigParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseFormatTextJSONSchemaConfigParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseFormatTextJSONSchemaConfigParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when there is a partial function-call arguments delta.
type ResponseFunctionCallArgumentsDeltaEvent struct {
	// The function-call arguments delta that is added.
	Delta string `json:"delta,required"`
	// The ID of the output item that the function-call arguments delta is added to.
	ItemID string `json:"item_id,required"`
	// The index of the output item that the function-call arguments delta is added to.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.function_call_arguments.delta`.
	Type constant.ResponseFunctionCallArgumentsDelta `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Delta          respjson.Field
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFunctionCallArgumentsDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseFunctionCallArgumentsDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when function-call arguments are finalized.
type ResponseFunctionCallArgumentsDoneEvent struct {
	// The function-call arguments.
	Arguments string `json:"arguments,required"`
	// The ID of the item.
	ItemID string `json:"item_id,required"`
	// The index of the output item.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event.
	SequenceNumber int64                                      `json:"sequence_number,required"`
	Type           constant.ResponseFunctionCallArgumentsDone `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Arguments      respjson.Field
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFunctionCallArgumentsDoneEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseFunctionCallArgumentsDoneEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A tool call to run a function. See the
// [function calling guide](https://platform.openai.com/docs/guides/function-calling)
// for more information.
type ResponseFunctionToolCall struct {
	// A JSON string of the arguments to pass to the function.
	Arguments string `json:"arguments,required"`
	// The unique ID of the function tool call generated by the model.
	CallID string `json:"call_id,required"`
	// The name of the function to run.
	Name string `json:"name,required"`
	// The type of the function tool call. Always `function_call`.
	Type constant.FunctionCall `json:"type,required"`
	// The unique ID of the function tool call.
	ID string `json:"id"`
	// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
	// Populated when items are returned via API.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status ResponseFunctionToolCallStatus `json:"status"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Arguments   respjson.Field
		CallID      respjson.Field
		Name        respjson.Field
		Type        respjson.Field
		ID          respjson.Field
		Status      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFunctionToolCall) RawJSON() string { return r.JSON.raw }
func (r *ResponseFunctionToolCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponseFunctionToolCall to a
// ResponseFunctionToolCallParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseFunctionToolCallParam.Overrides()
func (r ResponseFunctionToolCall) ToParam() ResponseFunctionToolCallParam {
	return param.Override[ResponseFunctionToolCallParam](json.RawMessage(r.RawJSON()))
}

// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
// Populated when items are returned via API.
type ResponseFunctionToolCallStatus string

const (
	ResponseFunctionToolCallStatusInProgress ResponseFunctionToolCallStatus = "in_progress"
	ResponseFunctionToolCallStatusCompleted  ResponseFunctionToolCallStatus = "completed"
	ResponseFunctionToolCallStatusIncomplete ResponseFunctionToolCallStatus = "incomplete"
)

// A tool call to run a function. See the
// [function calling guide](https://platform.openai.com/docs/guides/function-calling)
// for more information.
//
// The properties Arguments, CallID, Name, Type are required.
type ResponseFunctionToolCallParam struct {
	// A JSON string of the arguments to pass to the function.
	Arguments string `json:"arguments,required"`
	// The unique ID of the function tool call generated by the model.
	CallID string `json:"call_id,required"`
	// The name of the function to run.
	Name string `json:"name,required"`
	// The unique ID of the function tool call.
	ID param.Opt[string] `json:"id,omitzero"`
	// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
	// Populated when items are returned via API.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status ResponseFunctionToolCallStatus `json:"status,omitzero"`
	// The type of the function tool call. Always `function_call`.
	//
	// This field can be elided, and will marshal its zero value as "function_call".
	Type constant.FunctionCall `json:"type,required"`
	paramObj
}

func (r ResponseFunctionToolCallParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseFunctionToolCallParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseFunctionToolCallParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A tool call to run a function. See the
// [function calling guide](https://platform.openai.com/docs/guides/function-calling)
// for more information.
type ResponseFunctionToolCallItem struct {
	// The unique ID of the function tool call.
	ID string `json:"id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
	ResponseFunctionToolCall
}

// Returns the unmodified JSON received from the API
func (r ResponseFunctionToolCallItem) RawJSON() string { return r.JSON.raw }
func (r *ResponseFunctionToolCallItem) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ResponseFunctionToolCallOutputItem struct {
	// The unique ID of the function call tool output.
	ID string `json:"id,required"`
	// The unique ID of the function tool call generated by the model.
	CallID string `json:"call_id,required"`
	// A JSON string of the output of the function tool call.
	Output string `json:"output,required"`
	// The type of the function tool call output. Always `function_call_output`.
	Type constant.FunctionCallOutput `json:"type,required"`
	// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
	// Populated when items are returned via API.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status ResponseFunctionToolCallOutputItemStatus `json:"status"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		CallID      respjson.Field
		Output      respjson.Field
		Type        respjson.Field
		Status      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFunctionToolCallOutputItem) RawJSON() string { return r.JSON.raw }
func (r *ResponseFunctionToolCallOutputItem) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
// Populated when items are returned via API.
type ResponseFunctionToolCallOutputItemStatus string

const (
	ResponseFunctionToolCallOutputItemStatusInProgress ResponseFunctionToolCallOutputItemStatus = "in_progress"
	ResponseFunctionToolCallOutputItemStatusCompleted  ResponseFunctionToolCallOutputItemStatus = "completed"
	ResponseFunctionToolCallOutputItemStatusIncomplete ResponseFunctionToolCallOutputItemStatus = "incomplete"
)

// The results of a web search tool call. See the
// [web search guide](https://platform.openai.com/docs/guides/tools-web-search) for
// more information.
type ResponseFunctionWebSearch struct {
	// The unique ID of the web search tool call.
	ID string `json:"id,required"`
	// An object describing the specific action taken in this web search call. Includes
	// details on how the model used the web (search, open_page, find).
	Action ResponseFunctionWebSearchActionUnion `json:"action,required"`
	// The status of the web search tool call.
	//
	// Any of "in_progress", "searching", "completed", "failed".
	Status ResponseFunctionWebSearchStatus `json:"status,required"`
	// The type of the web search tool call. Always `web_search_call`.
	Type constant.WebSearchCall `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Action      respjson.Field
		Status      respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFunctionWebSearch) RawJSON() string { return r.JSON.raw }
func (r *ResponseFunctionWebSearch) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponseFunctionWebSearch to a
// ResponseFunctionWebSearchParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseFunctionWebSearchParam.Overrides()
func (r ResponseFunctionWebSearch) ToParam() ResponseFunctionWebSearchParam {
	return param.Override[ResponseFunctionWebSearchParam](json.RawMessage(r.RawJSON()))
}

// ResponseFunctionWebSearchActionUnion contains all possible properties and values
// from [ResponseFunctionWebSearchActionSearch],
// [ResponseFunctionWebSearchActionOpenPage],
// [ResponseFunctionWebSearchActionFind].
//
// Use the [ResponseFunctionWebSearchActionUnion.AsAny] method to switch on the
// variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ResponseFunctionWebSearchActionUnion struct {
	// This field is from variant [ResponseFunctionWebSearchActionSearch].
	Query string `json:"query"`
	// Any of "search", "open_page", "find".
	Type string `json:"type"`
	URL  string `json:"url"`
	// This field is from variant [ResponseFunctionWebSearchActionFind].
	Pattern string `json:"pattern"`
	JSON    struct {
		Query   respjson.Field
		Type    respjson.Field
		URL     respjson.Field
		Pattern respjson.Field
		raw     string
	} `json:"-"`
}

// anyResponseFunctionWebSearchAction is implemented by each variant of
// [ResponseFunctionWebSearchActionUnion] to add type safety for the return type of
// [ResponseFunctionWebSearchActionUnion.AsAny]
type anyResponseFunctionWebSearchAction interface {
	implResponseFunctionWebSearchActionUnion()
}

func (ResponseFunctionWebSearchActionSearch) implResponseFunctionWebSearchActionUnion()   {}
func (ResponseFunctionWebSearchActionOpenPage) implResponseFunctionWebSearchActionUnion() {}
func (ResponseFunctionWebSearchActionFind) implResponseFunctionWebSearchActionUnion()     {}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseFunctionWebSearchActionUnion.AsAny().(type) {
//	case responses.ResponseFunctionWebSearchActionSearch:
//	case responses.ResponseFunctionWebSearchActionOpenPage:
//	case responses.ResponseFunctionWebSearchActionFind:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseFunctionWebSearchActionUnion) AsAny() anyResponseFunctionWebSearchAction {
	switch u.Type {
	case "search":
		return u.AsSearch()
	case "open_page":
		return u.AsOpenPage()
	case "find":
		return u.AsFind()
	}
	return nil
}

func (u ResponseFunctionWebSearchActionUnion) AsSearch() (v ResponseFunctionWebSearchActionSearch) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseFunctionWebSearchActionUnion) AsOpenPage() (v ResponseFunctionWebSearchActionOpenPage) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseFunctionWebSearchActionUnion) AsFind() (v ResponseFunctionWebSearchActionFind) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ResponseFunctionWebSearchActionUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseFunctionWebSearchActionUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Action type "search" - Performs a web search query.
type ResponseFunctionWebSearchActionSearch struct {
	// The search query.
	Query string `json:"query,required"`
	// The action type.
	Type constant.Search `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Query       respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFunctionWebSearchActionSearch) RawJSON() string { return r.JSON.raw }
func (r *ResponseFunctionWebSearchActionSearch) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Action type "open_page" - Opens a specific URL from search results.
type ResponseFunctionWebSearchActionOpenPage struct {
	// The action type.
	Type constant.OpenPage `json:"type,required"`
	// The URL opened by the model.
	URL string `json:"url,required" format:"uri"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		URL         respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFunctionWebSearchActionOpenPage) RawJSON() string { return r.JSON.raw }
func (r *ResponseFunctionWebSearchActionOpenPage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Action type "find": Searches for a pattern within a loaded page.
type ResponseFunctionWebSearchActionFind struct {
	// The pattern or text to search for within the page.
	Pattern string `json:"pattern,required"`
	// The action type.
	Type constant.Find `json:"type,required"`
	// The URL of the page searched for the pattern.
	URL string `json:"url,required" format:"uri"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Pattern     respjson.Field
		Type        respjson.Field
		URL         respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFunctionWebSearchActionFind) RawJSON() string { return r.JSON.raw }
func (r *ResponseFunctionWebSearchActionFind) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The status of the web search tool call.
type ResponseFunctionWebSearchStatus string

const (
	ResponseFunctionWebSearchStatusInProgress ResponseFunctionWebSearchStatus = "in_progress"
	ResponseFunctionWebSearchStatusSearching  ResponseFunctionWebSearchStatus = "searching"
	ResponseFunctionWebSearchStatusCompleted  ResponseFunctionWebSearchStatus = "completed"
	ResponseFunctionWebSearchStatusFailed     ResponseFunctionWebSearchStatus = "failed"
)

// The results of a web search tool call. See the
// [web search guide](https://platform.openai.com/docs/guides/tools-web-search) for
// more information.
//
// The properties ID, Action, Status, Type are required.
type ResponseFunctionWebSearchParam struct {
	// The unique ID of the web search tool call.
	ID string `json:"id,required"`
	// An object describing the specific action taken in this web search call. Includes
	// details on how the model used the web (search, open_page, find).
	Action ResponseFunctionWebSearchActionUnionParam `json:"action,omitzero,required"`
	// The status of the web search tool call.
	//
	// Any of "in_progress", "searching", "completed", "failed".
	Status ResponseFunctionWebSearchStatus `json:"status,omitzero,required"`
	// The type of the web search tool call. Always `web_search_call`.
	//
	// This field can be elided, and will marshal its zero value as "web_search_call".
	Type constant.WebSearchCall `json:"type,required"`
	paramObj
}

func (r ResponseFunctionWebSearchParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseFunctionWebSearchParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseFunctionWebSearchParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ResponseFunctionWebSearchActionUnionParam struct {
	OfSearch   *ResponseFunctionWebSearchActionSearchParam   `json:",omitzero,inline"`
	OfOpenPage *ResponseFunctionWebSearchActionOpenPageParam `json:",omitzero,inline"`
	OfFind     *ResponseFunctionWebSearchActionFindParam     `json:",omitzero,inline"`
	paramUnion
}

func (u ResponseFunctionWebSearchActionUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfSearch, u.OfOpenPage, u.OfFind)
}
func (u *ResponseFunctionWebSearchActionUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ResponseFunctionWebSearchActionUnionParam) asAny() any {
	if !param.IsOmitted(u.OfSearch) {
		return u.OfSearch
	} else if !param.IsOmitted(u.OfOpenPage) {
		return u.OfOpenPage
	} else if !param.IsOmitted(u.OfFind) {
		return u.OfFind
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseFunctionWebSearchActionUnionParam) GetQuery() *string {
	if vt := u.OfSearch; vt != nil {
		return &vt.Query
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseFunctionWebSearchActionUnionParam) GetPattern() *string {
	if vt := u.OfFind; vt != nil {
		return &vt.Pattern
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseFunctionWebSearchActionUnionParam) GetType() *string {
	if vt := u.OfSearch; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfOpenPage; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfFind; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseFunctionWebSearchActionUnionParam) GetURL() *string {
	if vt := u.OfOpenPage; vt != nil {
		return (*string)(&vt.URL)
	} else if vt := u.OfFind; vt != nil {
		return (*string)(&vt.URL)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[ResponseFunctionWebSearchActionUnionParam](
		"type",
		apijson.Discriminator[ResponseFunctionWebSearchActionSearchParam]("search"),
		apijson.Discriminator[ResponseFunctionWebSearchActionOpenPageParam]("open_page"),
		apijson.Discriminator[ResponseFunctionWebSearchActionFindParam]("find"),
	)
}

// Action type "search" - Performs a web search query.
//
// The properties Query, Type are required.
type ResponseFunctionWebSearchActionSearchParam struct {
	// The search query.
	Query string `json:"query,required"`
	// The action type.
	//
	// This field can be elided, and will marshal its zero value as "search".
	Type constant.Search `json:"type,required"`
	paramObj
}

func (r ResponseFunctionWebSearchActionSearchParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseFunctionWebSearchActionSearchParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseFunctionWebSearchActionSearchParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Action type "open_page" - Opens a specific URL from search results.
//
// The properties Type, URL are required.
type ResponseFunctionWebSearchActionOpenPageParam struct {
	// The URL opened by the model.
	URL string `json:"url,required" format:"uri"`
	// The action type.
	//
	// This field can be elided, and will marshal its zero value as "open_page".
	Type constant.OpenPage `json:"type,required"`
	paramObj
}

func (r ResponseFunctionWebSearchActionOpenPageParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseFunctionWebSearchActionOpenPageParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseFunctionWebSearchActionOpenPageParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Action type "find": Searches for a pattern within a loaded page.
//
// The properties Pattern, Type, URL are required.
type ResponseFunctionWebSearchActionFindParam struct {
	// The pattern or text to search for within the page.
	Pattern string `json:"pattern,required"`
	// The URL of the page searched for the pattern.
	URL string `json:"url,required" format:"uri"`
	// The action type.
	//
	// This field can be elided, and will marshal its zero value as "find".
	Type constant.Find `json:"type,required"`
	paramObj
}

func (r ResponseFunctionWebSearchActionFindParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseFunctionWebSearchActionFindParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseFunctionWebSearchActionFindParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when an image generation tool call has completed and the final image is
// available.
type ResponseImageGenCallCompletedEvent struct {
	// The unique identifier of the image generation item being processed.
	ItemID string `json:"item_id,required"`
	// The index of the output item in the response's output array.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always 'response.image_generation_call.completed'.
	Type constant.ResponseImageGenerationCallCompleted `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseImageGenCallCompletedEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseImageGenCallCompletedEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when an image generation tool call is actively generating an image
// (intermediate state).
type ResponseImageGenCallGeneratingEvent struct {
	// The unique identifier of the image generation item being processed.
	ItemID string `json:"item_id,required"`
	// The index of the output item in the response's output array.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of the image generation item being processed.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always 'response.image_generation_call.generating'.
	Type constant.ResponseImageGenerationCallGenerating `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseImageGenCallGeneratingEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseImageGenCallGeneratingEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when an image generation tool call is in progress.
type ResponseImageGenCallInProgressEvent struct {
	// The unique identifier of the image generation item being processed.
	ItemID string `json:"item_id,required"`
	// The index of the output item in the response's output array.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of the image generation item being processed.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always 'response.image_generation_call.in_progress'.
	Type constant.ResponseImageGenerationCallInProgress `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseImageGenCallInProgressEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseImageGenCallInProgressEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a partial image is available during image generation streaming.
type ResponseImageGenCallPartialImageEvent struct {
	// The unique identifier of the image generation item being processed.
	ItemID string `json:"item_id,required"`
	// The index of the output item in the response's output array.
	OutputIndex int64 `json:"output_index,required"`
	// Base64-encoded partial image data, suitable for rendering as an image.
	PartialImageB64 string `json:"partial_image_b64,required"`
	// 0-based index for the partial image (backend is 1-based, but this is 0-based for
	// the user).
	PartialImageIndex int64 `json:"partial_image_index,required"`
	// The sequence number of the image generation item being processed.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always 'response.image_generation_call.partial_image'.
	Type constant.ResponseImageGenerationCallPartialImage `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ItemID            respjson.Field
		OutputIndex       respjson.Field
		PartialImageB64   respjson.Field
		PartialImageIndex respjson.Field
		SequenceNumber    respjson.Field
		Type              respjson.Field
		ExtraFields       map[string]respjson.Field
		raw               string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseImageGenCallPartialImageEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseImageGenCallPartialImageEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when the response is in progress.
type ResponseInProgressEvent struct {
	// The response that is in progress.
	Response Response `json:"response,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.in_progress`.
	Type constant.ResponseInProgress `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Response       respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseInProgressEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseInProgressEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Specify additional output data to include in the model response. Currently
// supported values are:
//
//   - `code_interpreter_call.outputs`: Includes the outputs of python code execution
//     in code interpreter tool call items.
//   - `computer_call_output.output.image_url`: Include image urls from the computer
//     call output.
//   - `file_search_call.results`: Include the search results of the file search tool
//     call.
//   - `message.input_image.image_url`: Include image urls from the input message.
//   - `message.output_text.logprobs`: Include logprobs with assistant messages.
//   - `reasoning.encrypted_content`: Includes an encrypted version of reasoning
//     tokens in reasoning item outputs. This enables reasoning items to be used in
//     multi-turn conversations when using the Responses API statelessly (like when
//     the `store` parameter is set to `false`, or when an organization is enrolled
//     in the zero data retention program).
type ResponseIncludable string

const (
	ResponseIncludableCodeInterpreterCallOutputs       ResponseIncludable = "code_interpreter_call.outputs"
	ResponseIncludableComputerCallOutputOutputImageURL ResponseIncludable = "computer_call_output.output.image_url"
	ResponseIncludableFileSearchCallResults            ResponseIncludable = "file_search_call.results"
	ResponseIncludableMessageInputImageImageURL        ResponseIncludable = "message.input_image.image_url"
	ResponseIncludableMessageOutputTextLogprobs        ResponseIncludable = "message.output_text.logprobs"
	ResponseIncludableReasoningEncryptedContent        ResponseIncludable = "reasoning.encrypted_content"
)

// An event that is emitted when a response finishes as incomplete.
type ResponseIncompleteEvent struct {
	// The response that was incomplete.
	Response Response `json:"response,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.incomplete`.
	Type constant.ResponseIncomplete `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Response       respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseIncompleteEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseIncompleteEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ResponseInputParam []ResponseInputItemUnionParam

// ResponseInputContentUnion contains all possible properties and values from
// [ResponseInputText], [ResponseInputImage], [ResponseInputFile].
//
// Use the [ResponseInputContentUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ResponseInputContentUnion struct {
	// This field is from variant [ResponseInputText].
	Text string `json:"text"`
	// Any of "input_text", "input_image", "input_file".
	Type string `json:"type"`
	// This field is from variant [ResponseInputImage].
	Detail ResponseInputImageDetail `json:"detail"`
	FileID string                   `json:"file_id"`
	// This field is from variant [ResponseInputImage].
	ImageURL string `json:"image_url"`
	// This field is from variant [ResponseInputFile].
	FileData string `json:"file_data"`
	// This field is from variant [ResponseInputFile].
	Filename string `json:"filename"`
	JSON     struct {
		Text     respjson.Field
		Type     respjson.Field
		Detail   respjson.Field
		FileID   respjson.Field
		ImageURL respjson.Field
		FileData respjson.Field
		Filename respjson.Field
		raw      string
	} `json:"-"`
}

// anyResponseInputContent is implemented by each variant of
// [ResponseInputContentUnion] to add type safety for the return type of
// [ResponseInputContentUnion.AsAny]
type anyResponseInputContent interface {
	implResponseInputContentUnion()
}

func (ResponseInputText) implResponseInputContentUnion()  {}
func (ResponseInputImage) implResponseInputContentUnion() {}
func (ResponseInputFile) implResponseInputContentUnion()  {}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseInputContentUnion.AsAny().(type) {
//	case responses.ResponseInputText:
//	case responses.ResponseInputImage:
//	case responses.ResponseInputFile:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseInputContentUnion) AsAny() anyResponseInputContent {
	switch u.Type {
	case "input_text":
		return u.AsInputText()
	case "input_image":
		return u.AsInputImage()
	case "input_file":
		return u.AsInputFile()
	}
	return nil
}

func (u ResponseInputContentUnion) AsInputText() (v ResponseInputText) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputContentUnion) AsInputImage() (v ResponseInputImage) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputContentUnion) AsInputFile() (v ResponseInputFile) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ResponseInputContentUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseInputContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponseInputContentUnion to a
// ResponseInputContentUnionParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseInputContentUnionParam.Overrides()
func (r ResponseInputContentUnion) ToParam() ResponseInputContentUnionParam {
	return param.Override[ResponseInputContentUnionParam](json.RawMessage(r.RawJSON()))
}

func ResponseInputContentParamOfInputText(text string) ResponseInputContentUnionParam {
	var inputText ResponseInputTextParam
	inputText.Text = text
	return ResponseInputContentUnionParam{OfInputText: &inputText}
}

func ResponseInputContentParamOfInputImage(detail ResponseInputImageDetail) ResponseInputContentUnionParam {
	var inputImage ResponseInputImageParam
	inputImage.Detail = detail
	return ResponseInputContentUnionParam{OfInputImage: &inputImage}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ResponseInputContentUnionParam struct {
	OfInputText  *ResponseInputTextParam  `json:",omitzero,inline"`
	OfInputImage *ResponseInputImageParam `json:",omitzero,inline"`
	OfInputFile  *ResponseInputFileParam  `json:",omitzero,inline"`
	paramUnion
}

func (u ResponseInputContentUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfInputText, u.OfInputImage, u.OfInputFile)
}
func (u *ResponseInputContentUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ResponseInputContentUnionParam) asAny() any {
	if !param.IsOmitted(u.OfInputText) {
		return u.OfInputText
	} else if !param.IsOmitted(u.OfInputImage) {
		return u.OfInputImage
	} else if !param.IsOmitted(u.OfInputFile) {
		return u.OfInputFile
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputContentUnionParam) GetText() *string {
	if vt := u.OfInputText; vt != nil {
		return &vt.Text
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputContentUnionParam) GetDetail() *string {
	if vt := u.OfInputImage; vt != nil {
		return (*string)(&vt.Detail)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputContentUnionParam) GetImageURL() *string {
	if vt := u.OfInputImage; vt != nil && vt.ImageURL.Valid() {
		return &vt.ImageURL.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputContentUnionParam) GetFileData() *string {
	if vt := u.OfInputFile; vt != nil && vt.FileData.Valid() {
		return &vt.FileData.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputContentUnionParam) GetFilename() *string {
	if vt := u.OfInputFile; vt != nil && vt.Filename.Valid() {
		return &vt.Filename.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputContentUnionParam) GetType() *string {
	if vt := u.OfInputText; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfInputImage; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfInputFile; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputContentUnionParam) GetFileID() *string {
	if vt := u.OfInputImage; vt != nil && vt.FileID.Valid() {
		return &vt.FileID.Value
	} else if vt := u.OfInputFile; vt != nil && vt.FileID.Valid() {
		return &vt.FileID.Value
	}
	return nil
}

func init() {
	apijson.RegisterUnion[ResponseInputContentUnionParam](
		"type",
		apijson.Discriminator[ResponseInputTextParam]("input_text"),
		apijson.Discriminator[ResponseInputImageParam]("input_image"),
		apijson.Discriminator[ResponseInputFileParam]("input_file"),
	)
}

// A file input to the model.
type ResponseInputFile struct {
	// The type of the input item. Always `input_file`.
	Type constant.InputFile `json:"type,required"`
	// The content of the file to be sent to the model.
	FileData string `json:"file_data"`
	// The ID of the file to be sent to the model.
	FileID string `json:"file_id,nullable"`
	// The name of the file to be sent to the model.
	Filename string `json:"filename"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		FileData    respjson.Field
		FileID      respjson.Field
		Filename    respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseInputFile) RawJSON() string { return r.JSON.raw }
func (r *ResponseInputFile) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponseInputFile to a ResponseInputFileParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseInputFileParam.Overrides()
func (r ResponseInputFile) ToParam() ResponseInputFileParam {
	return param.Override[ResponseInputFileParam](json.RawMessage(r.RawJSON()))
}

// A file input to the model.
//
// The property Type is required.
type ResponseInputFileParam struct {
	// The ID of the file to be sent to the model.
	FileID param.Opt[string] `json:"file_id,omitzero"`
	// The content of the file to be sent to the model.
	FileData param.Opt[string] `json:"file_data,omitzero"`
	// The name of the file to be sent to the model.
	Filename param.Opt[string] `json:"filename,omitzero"`
	// The type of the input item. Always `input_file`.
	//
	// This field can be elided, and will marshal its zero value as "input_file".
	Type constant.InputFile `json:"type,required"`
	paramObj
}

func (r ResponseInputFileParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputFileParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseInputFileParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// An image input to the model. Learn about
// [image inputs](https://platform.openai.com/docs/guides/vision).
type ResponseInputImage struct {
	// The detail level of the image to be sent to the model. One of `high`, `low`, or
	// `auto`. Defaults to `auto`.
	//
	// Any of "low", "high", "auto".
	Detail ResponseInputImageDetail `json:"detail,required"`
	// The type of the input item. Always `input_image`.
	Type constant.InputImage `json:"type,required"`
	// The ID of the file to be sent to the model.
	FileID string `json:"file_id,nullable"`
	// The URL of the image to be sent to the model. A fully qualified URL or base64
	// encoded image in a data URL.
	ImageURL string `json:"image_url,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Detail      respjson.Field
		Type        respjson.Field
		FileID      respjson.Field
		ImageURL    respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseInputImage) RawJSON() string { return r.JSON.raw }
func (r *ResponseInputImage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponseInputImage to a ResponseInputImageParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseInputImageParam.Overrides()
func (r ResponseInputImage) ToParam() ResponseInputImageParam {
	return param.Override[ResponseInputImageParam](json.RawMessage(r.RawJSON()))
}

// The detail level of the image to be sent to the model. One of `high`, `low`, or
// `auto`. Defaults to `auto`.
type ResponseInputImageDetail string

const (
	ResponseInputImageDetailLow  ResponseInputImageDetail = "low"
	ResponseInputImageDetailHigh ResponseInputImageDetail = "high"
	ResponseInputImageDetailAuto ResponseInputImageDetail = "auto"
)

// An image input to the model. Learn about
// [image inputs](https://platform.openai.com/docs/guides/vision).
//
// The properties Detail, Type are required.
type ResponseInputImageParam struct {
	// The detail level of the image to be sent to the model. One of `high`, `low`, or
	// `auto`. Defaults to `auto`.
	//
	// Any of "low", "high", "auto".
	Detail ResponseInputImageDetail `json:"detail,omitzero,required"`
	// The ID of the file to be sent to the model.
	FileID param.Opt[string] `json:"file_id,omitzero"`
	// The URL of the image to be sent to the model. A fully qualified URL or base64
	// encoded image in a data URL.
	ImageURL param.Opt[string] `json:"image_url,omitzero"`
	// The type of the input item. Always `input_image`.
	//
	// This field can be elided, and will marshal its zero value as "input_image".
	Type constant.InputImage `json:"type,required"`
	paramObj
}

func (r ResponseInputImageParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputImageParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseInputImageParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseInputItemUnion contains all possible properties and values from
// [EasyInputMessage], [ResponseInputItemMessage], [ResponseOutputMessage],
// [ResponseFileSearchToolCall], [ResponseComputerToolCall],
// [ResponseInputItemComputerCallOutput], [ResponseFunctionWebSearch],
// [ResponseFunctionToolCall], [ResponseInputItemFunctionCallOutput],
// [ResponseReasoningItem], [ResponseInputItemImageGenerationCall],
// [ResponseCodeInterpreterToolCall], [ResponseInputItemLocalShellCall],
// [ResponseInputItemLocalShellCallOutput], [ResponseInputItemMcpListTools],
// [ResponseInputItemMcpApprovalRequest], [ResponseInputItemMcpApprovalResponse],
// [ResponseInputItemMcpCall], [ResponseInputItemItemReference].
//
// Use the [ResponseInputItemUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ResponseInputItemUnion struct {
	// This field is a union of [EasyInputMessageContentUnion],
	// [ResponseInputMessageContentList], [[]ResponseOutputMessageContentUnion]
	Content ResponseInputItemUnionContent `json:"content"`
	Role    string                        `json:"role"`
	// Any of "message", "message", "message", "file_search_call", "computer_call",
	// "computer_call_output", "web_search_call", "function_call",
	// "function_call_output", "reasoning", "image_generation_call",
	// "code_interpreter_call", "local_shell_call", "local_shell_call_output",
	// "mcp_list_tools", "mcp_approval_request", "mcp_approval_response", "mcp_call",
	// "item_reference".
	Type   string `json:"type"`
	Status string `json:"status"`
	ID     string `json:"id"`
	// This field is from variant [ResponseFileSearchToolCall].
	Queries []string `json:"queries"`
	// This field is from variant [ResponseFileSearchToolCall].
	Results []ResponseFileSearchToolCallResult `json:"results"`
	// This field is a union of [ResponseComputerToolCallActionUnion],
	// [ResponseFunctionWebSearchActionUnion], [ResponseInputItemLocalShellCallAction]
	Action ResponseInputItemUnionAction `json:"action"`
	CallID string                       `json:"call_id"`
	// This field is from variant [ResponseComputerToolCall].
	PendingSafetyChecks []ResponseComputerToolCallPendingSafetyCheck `json:"pending_safety_checks"`
	// This field is a union of [ResponseComputerToolCallOutputScreenshot], [string],
	// [string], [string]
	Output ResponseInputItemUnionOutput `json:"output"`
	// This field is from variant [ResponseInputItemComputerCallOutput].
	AcknowledgedSafetyChecks []ResponseInputItemComputerCallOutputAcknowledgedSafetyCheck `json:"acknowledged_safety_checks"`
	Arguments                string                                                       `json:"arguments"`
	Name                     string                                                       `json:"name"`
	// This field is from variant [ResponseReasoningItem].
	Summary []ResponseReasoningItemSummary `json:"summary"`
	// This field is from variant [ResponseReasoningItem].
	EncryptedContent string `json:"encrypted_content"`
	// This field is from variant [ResponseInputItemImageGenerationCall].
	Result string `json:"result"`
	// This field is from variant [ResponseCodeInterpreterToolCall].
	Code string `json:"code"`
	// This field is from variant [ResponseCodeInterpreterToolCall].
	ContainerID string `json:"container_id"`
	// This field is from variant [ResponseCodeInterpreterToolCall].
	Outputs     []ResponseCodeInterpreterToolCallOutputUnion `json:"outputs"`
	ServerLabel string                                       `json:"server_label"`
	// This field is from variant [ResponseInputItemMcpListTools].
	Tools []ResponseInputItemMcpListToolsTool `json:"tools"`
	Error string                              `json:"error"`
	// This field is from variant [ResponseInputItemMcpApprovalResponse].
	ApprovalRequestID string `json:"approval_request_id"`
	// This field is from variant [ResponseInputItemMcpApprovalResponse].
	Approve bool `json:"approve"`
	// This field is from variant [ResponseInputItemMcpApprovalResponse].
	Reason string `json:"reason"`
	JSON   struct {
		Content                  respjson.Field
		Role                     respjson.Field
		Type                     respjson.Field
		Status                   respjson.Field
		ID                       respjson.Field
		Queries                  respjson.Field
		Results                  respjson.Field
		Action                   respjson.Field
		CallID                   respjson.Field
		PendingSafetyChecks      respjson.Field
		Output                   respjson.Field
		AcknowledgedSafetyChecks respjson.Field
		Arguments                respjson.Field
		Name                     respjson.Field
		Summary                  respjson.Field
		EncryptedContent         respjson.Field
		Result                   respjson.Field
		Code                     respjson.Field
		ContainerID              respjson.Field
		Outputs                  respjson.Field
		ServerLabel              respjson.Field
		Tools                    respjson.Field
		Error                    respjson.Field
		ApprovalRequestID        respjson.Field
		Approve                  respjson.Field
		Reason                   respjson.Field
		raw                      string
	} `json:"-"`
}

// anyResponseInputItem is implemented by each variant of [ResponseInputItemUnion]
// to add type safety for the return type of [ResponseInputItemUnion.AsAny]
type anyResponseInputItem interface {
	implResponseInputItemUnion()
}

func (EasyInputMessage) implResponseInputItemUnion()                      {}
func (ResponseInputItemMessage) implResponseInputItemUnion()              {}
func (ResponseOutputMessage) implResponseInputItemUnion()                 {}
func (ResponseFileSearchToolCall) implResponseInputItemUnion()            {}
func (ResponseComputerToolCall) implResponseInputItemUnion()              {}
func (ResponseInputItemComputerCallOutput) implResponseInputItemUnion()   {}
func (ResponseFunctionWebSearch) implResponseInputItemUnion()             {}
func (ResponseFunctionToolCall) implResponseInputItemUnion()              {}
func (ResponseInputItemFunctionCallOutput) implResponseInputItemUnion()   {}
func (ResponseReasoningItem) implResponseInputItemUnion()                 {}
func (ResponseInputItemImageGenerationCall) implResponseInputItemUnion()  {}
func (ResponseCodeInterpreterToolCall) implResponseInputItemUnion()       {}
func (ResponseInputItemLocalShellCall) implResponseInputItemUnion()       {}
func (ResponseInputItemLocalShellCallOutput) implResponseInputItemUnion() {}
func (ResponseInputItemMcpListTools) implResponseInputItemUnion()         {}
func (ResponseInputItemMcpApprovalRequest) implResponseInputItemUnion()   {}
func (ResponseInputItemMcpApprovalResponse) implResponseInputItemUnion()  {}
func (ResponseInputItemMcpCall) implResponseInputItemUnion()              {}
func (ResponseInputItemItemReference) implResponseInputItemUnion()        {}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseInputItemUnion.AsAny().(type) {
//	case responses.EasyInputMessage:
//	case responses.ResponseInputItemMessage:
//	case responses.ResponseOutputMessage:
//	case responses.ResponseFileSearchToolCall:
//	case responses.ResponseComputerToolCall:
//	case responses.ResponseInputItemComputerCallOutput:
//	case responses.ResponseFunctionWebSearch:
//	case responses.ResponseFunctionToolCall:
//	case responses.ResponseInputItemFunctionCallOutput:
//	case responses.ResponseReasoningItem:
//	case responses.ResponseInputItemImageGenerationCall:
//	case responses.ResponseCodeInterpreterToolCall:
//	case responses.ResponseInputItemLocalShellCall:
//	case responses.ResponseInputItemLocalShellCallOutput:
//	case responses.ResponseInputItemMcpListTools:
//	case responses.ResponseInputItemMcpApprovalRequest:
//	case responses.ResponseInputItemMcpApprovalResponse:
//	case responses.ResponseInputItemMcpCall:
//	case responses.ResponseInputItemItemReference:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseInputItemUnion) AsAny() anyResponseInputItem {
	switch u.Type {
	case "message":
		return u.AsOutputMessage()
	case "file_search_call":
		return u.AsFileSearchCall()
	case "computer_call":
		return u.AsComputerCall()
	case "computer_call_output":
		return u.AsComputerCallOutput()
	case "web_search_call":
		return u.AsWebSearchCall()
	case "function_call":
		return u.AsFunctionCall()
	case "function_call_output":
		return u.AsFunctionCallOutput()
	case "reasoning":
		return u.AsReasoning()
	case "image_generation_call":
		return u.AsImageGenerationCall()
	case "code_interpreter_call":
		return u.AsCodeInterpreterCall()
	case "local_shell_call":
		return u.AsLocalShellCall()
	case "local_shell_call_output":
		return u.AsLocalShellCallOutput()
	case "mcp_list_tools":
		return u.AsMcpListTools()
	case "mcp_approval_request":
		return u.AsMcpApprovalRequest()
	case "mcp_approval_response":
		return u.AsMcpApprovalResponse()
	case "mcp_call":
		return u.AsMcpCall()
	case "item_reference":
		return u.AsItemReference()
	}
	return nil
}

func (u ResponseInputItemUnion) AsMessage() (v EasyInputMessage) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputItemUnion) AsInputMessage() (v ResponseInputItemMessage) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputItemUnion) AsOutputMessage() (v ResponseOutputMessage) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputItemUnion) AsFileSearchCall() (v ResponseFileSearchToolCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputItemUnion) AsComputerCall() (v ResponseComputerToolCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputItemUnion) AsComputerCallOutput() (v ResponseInputItemComputerCallOutput) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputItemUnion) AsWebSearchCall() (v ResponseFunctionWebSearch) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputItemUnion) AsFunctionCall() (v ResponseFunctionToolCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputItemUnion) AsFunctionCallOutput() (v ResponseInputItemFunctionCallOutput) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputItemUnion) AsReasoning() (v ResponseReasoningItem) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputItemUnion) AsImageGenerationCall() (v ResponseInputItemImageGenerationCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputItemUnion) AsCodeInterpreterCall() (v ResponseCodeInterpreterToolCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputItemUnion) AsLocalShellCall() (v ResponseInputItemLocalShellCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputItemUnion) AsLocalShellCallOutput() (v ResponseInputItemLocalShellCallOutput) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputItemUnion) AsMcpListTools() (v ResponseInputItemMcpListTools) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputItemUnion) AsMcpApprovalRequest() (v ResponseInputItemMcpApprovalRequest) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputItemUnion) AsMcpApprovalResponse() (v ResponseInputItemMcpApprovalResponse) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputItemUnion) AsMcpCall() (v ResponseInputItemMcpCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseInputItemUnion) AsItemReference() (v ResponseInputItemItemReference) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ResponseInputItemUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseInputItemUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseInputItemUnionContent is an implicit subunion of
// [ResponseInputItemUnion]. ResponseInputItemUnionContent provides convenient
// access to the sub-properties of the union.
//
// For type safety it is recommended to directly use a variant of the
// [ResponseInputItemUnion].
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfString OfInputItemContentList
// OfResponseOutputMessageContentArray]
type ResponseInputItemUnionContent struct {
	// This field will be present if the value is a [string] instead of an object.
	OfString string `json:",inline"`
	// This field will be present if the value is a [ResponseInputMessageContentList]
	// instead of an object.
	OfInputItemContentList ResponseInputMessageContentList `json:",inline"`
	// This field will be present if the value is a
	// [[]ResponseOutputMessageContentUnion] instead of an object.
	OfResponseOutputMessageContentArray []ResponseOutputMessageContentUnion `json:",inline"`
	JSON                                struct {
		OfString                            respjson.Field
		OfInputItemContentList              respjson.Field
		OfResponseOutputMessageContentArray respjson.Field
		raw                                 string
	} `json:"-"`
}

func (r *ResponseInputItemUnionContent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseInputItemUnionAction is an implicit subunion of
// [ResponseInputItemUnion]. ResponseInputItemUnionAction provides convenient
// access to the sub-properties of the union.
//
// For type safety it is recommended to directly use a variant of the
// [ResponseInputItemUnion].
type ResponseInputItemUnionAction struct {
	// This field is from variant [ResponseComputerToolCallActionUnion].
	Button string `json:"button"`
	Type   string `json:"type"`
	X      int64  `json:"x"`
	Y      int64  `json:"y"`
	// This field is from variant [ResponseComputerToolCallActionUnion].
	Path []ResponseComputerToolCallActionDragPath `json:"path"`
	// This field is from variant [ResponseComputerToolCallActionUnion].
	Keys []string `json:"keys"`
	// This field is from variant [ResponseComputerToolCallActionUnion].
	ScrollX int64 `json:"scroll_x"`
	// This field is from variant [ResponseComputerToolCallActionUnion].
	ScrollY int64 `json:"scroll_y"`
	// This field is from variant [ResponseComputerToolCallActionUnion].
	Text string `json:"text"`
	// This field is from variant [ResponseFunctionWebSearchActionUnion].
	Query string `json:"query"`
	URL   string `json:"url"`
	// This field is from variant [ResponseFunctionWebSearchActionUnion].
	Pattern string `json:"pattern"`
	// This field is from variant [ResponseInputItemLocalShellCallAction].
	Command []string `json:"command"`
	// This field is from variant [ResponseInputItemLocalShellCallAction].
	Env map[string]string `json:"env"`
	// This field is from variant [ResponseInputItemLocalShellCallAction].
	TimeoutMs int64 `json:"timeout_ms"`
	// This field is from variant [ResponseInputItemLocalShellCallAction].
	User string `json:"user"`
	// This field is from variant [ResponseInputItemLocalShellCallAction].
	WorkingDirectory string `json:"working_directory"`
	JSON             struct {
		Button           respjson.Field
		Type             respjson.Field
		X                respjson.Field
		Y                respjson.Field
		Path             respjson.Field
		Keys             respjson.Field
		ScrollX          respjson.Field
		ScrollY          respjson.Field
		Text             respjson.Field
		Query            respjson.Field
		URL              respjson.Field
		Pattern          respjson.Field
		Command          respjson.Field
		Env              respjson.Field
		TimeoutMs        respjson.Field
		User             respjson.Field
		WorkingDirectory respjson.Field
		raw              string
	} `json:"-"`
}

func (r *ResponseInputItemUnionAction) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseInputItemUnionOutput is an implicit subunion of
// [ResponseInputItemUnion]. ResponseInputItemUnionOutput provides convenient
// access to the sub-properties of the union.
//
// For type safety it is recommended to directly use a variant of the
// [ResponseInputItemUnion].
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfString]
type ResponseInputItemUnionOutput struct {
	// This field will be present if the value is a [string] instead of an object.
	OfString string `json:",inline"`
	// This field is from variant [ResponseComputerToolCallOutputScreenshot].
	Type constant.ComputerScreenshot `json:"type"`
	// This field is from variant [ResponseComputerToolCallOutputScreenshot].
	FileID string `json:"file_id"`
	// This field is from variant [ResponseComputerToolCallOutputScreenshot].
	ImageURL string `json:"image_url"`
	JSON     struct {
		OfString respjson.Field
		Type     respjson.Field
		FileID   respjson.Field
		ImageURL respjson.Field
		raw      string
	} `json:"-"`
}

func (r *ResponseInputItemUnionOutput) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponseInputItemUnion to a ResponseInputItemUnionParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseInputItemUnionParam.Overrides()
func (r ResponseInputItemUnion) ToParam() ResponseInputItemUnionParam {
	return param.Override[ResponseInputItemUnionParam](json.RawMessage(r.RawJSON()))
}

// A message input to the model with a role indicating instruction following
// hierarchy. Instructions given with the `developer` or `system` role take
// precedence over instructions given with the `user` role.
type ResponseInputItemMessage struct {
	// A list of one or many input items to the model, containing different content
	// types.
	Content ResponseInputMessageContentList `json:"content,required"`
	// The role of the message input. One of `user`, `system`, or `developer`.
	//
	// Any of "user", "system", "developer".
	Role string `json:"role,required"`
	// The status of item. One of `in_progress`, `completed`, or `incomplete`.
	// Populated when items are returned via API.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status string `json:"status"`
	// The type of the message input. Always set to `message`.
	//
	// Any of "message".
	Type string `json:"type"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Content     respjson.Field
		Role        respjson.Field
		Status      respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseInputItemMessage) RawJSON() string { return r.JSON.raw }
func (r *ResponseInputItemMessage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The output of a computer tool call.
type ResponseInputItemComputerCallOutput struct {
	// The ID of the computer tool call that produced the output.
	CallID string `json:"call_id,required"`
	// A computer screenshot image used with the computer use tool.
	Output ResponseComputerToolCallOutputScreenshot `json:"output,required"`
	// The type of the computer tool call output. Always `computer_call_output`.
	Type constant.ComputerCallOutput `json:"type,required"`
	// The ID of the computer tool call output.
	ID string `json:"id,nullable"`
	// The safety checks reported by the API that have been acknowledged by the
	// developer.
	AcknowledgedSafetyChecks []ResponseInputItemComputerCallOutputAcknowledgedSafetyCheck `json:"acknowledged_safety_checks,nullable"`
	// The status of the message input. One of `in_progress`, `completed`, or
	// `incomplete`. Populated when input items are returned via API.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status string `json:"status,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		CallID                   respjson.Field
		Output                   respjson.Field
		Type                     respjson.Field
		ID                       respjson.Field
		AcknowledgedSafetyChecks respjson.Field
		Status                   respjson.Field
		ExtraFields              map[string]respjson.Field
		raw                      string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseInputItemComputerCallOutput) RawJSON() string { return r.JSON.raw }
func (r *ResponseInputItemComputerCallOutput) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A pending safety check for the computer call.
type ResponseInputItemComputerCallOutputAcknowledgedSafetyCheck struct {
	// The ID of the pending safety check.
	ID string `json:"id,required"`
	// The type of the pending safety check.
	Code string `json:"code,nullable"`
	// Details about the pending safety check.
	Message string `json:"message,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Code        respjson.Field
		Message     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseInputItemComputerCallOutputAcknowledgedSafetyCheck) RawJSON() string {
	return r.JSON.raw
}
func (r *ResponseInputItemComputerCallOutputAcknowledgedSafetyCheck) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The output of a function tool call.
type ResponseInputItemFunctionCallOutput struct {
	// The unique ID of the function tool call generated by the model.
	CallID string `json:"call_id,required"`
	// A JSON string of the output of the function tool call.
	Output string `json:"output,required"`
	// The type of the function tool call output. Always `function_call_output`.
	Type constant.FunctionCallOutput `json:"type,required"`
	// The unique ID of the function tool call output. Populated when this item is
	// returned via API.
	ID string `json:"id,nullable"`
	// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
	// Populated when items are returned via API.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status string `json:"status,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		CallID      respjson.Field
		Output      respjson.Field
		Type        respjson.Field
		ID          respjson.Field
		Status      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseInputItemFunctionCallOutput) RawJSON() string { return r.JSON.raw }
func (r *ResponseInputItemFunctionCallOutput) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// An image generation request made by the model.
type ResponseInputItemImageGenerationCall struct {
	// The unique ID of the image generation call.
	ID string `json:"id,required"`
	// The generated image encoded in base64.
	Result string `json:"result,required"`
	// The status of the image generation call.
	//
	// Any of "in_progress", "completed", "generating", "failed".
	Status string `json:"status,required"`
	// The type of the image generation call. Always `image_generation_call`.
	Type constant.ImageGenerationCall `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Result      respjson.Field
		Status      respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseInputItemImageGenerationCall) RawJSON() string { return r.JSON.raw }
func (r *ResponseInputItemImageGenerationCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A tool call to run a command on the local shell.
type ResponseInputItemLocalShellCall struct {
	// The unique ID of the local shell call.
	ID string `json:"id,required"`
	// Execute a shell command on the server.
	Action ResponseInputItemLocalShellCallAction `json:"action,required"`
	// The unique ID of the local shell tool call generated by the model.
	CallID string `json:"call_id,required"`
	// The status of the local shell call.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status string `json:"status,required"`
	// The type of the local shell call. Always `local_shell_call`.
	Type constant.LocalShellCall `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Action      respjson.Field
		CallID      respjson.Field
		Status      respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseInputItemLocalShellCall) RawJSON() string { return r.JSON.raw }
func (r *ResponseInputItemLocalShellCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Execute a shell command on the server.
type ResponseInputItemLocalShellCallAction struct {
	// The command to run.
	Command []string `json:"command,required"`
	// Environment variables to set for the command.
	Env map[string]string `json:"env,required"`
	// The type of the local shell action. Always `exec`.
	Type constant.Exec `json:"type,required"`
	// Optional timeout in milliseconds for the command.
	TimeoutMs int64 `json:"timeout_ms,nullable"`
	// Optional user to run the command as.
	User string `json:"user,nullable"`
	// Optional working directory to run the command in.
	WorkingDirectory string `json:"working_directory,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Command          respjson.Field
		Env              respjson.Field
		Type             respjson.Field
		TimeoutMs        respjson.Field
		User             respjson.Field
		WorkingDirectory respjson.Field
		ExtraFields      map[string]respjson.Field
		raw              string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseInputItemLocalShellCallAction) RawJSON() string { return r.JSON.raw }
func (r *ResponseInputItemLocalShellCallAction) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The output of a local shell tool call.
type ResponseInputItemLocalShellCallOutput struct {
	// The unique ID of the local shell tool call generated by the model.
	ID string `json:"id,required"`
	// A JSON string of the output of the local shell tool call.
	Output string `json:"output,required"`
	// The type of the local shell tool call output. Always `local_shell_call_output`.
	Type constant.LocalShellCallOutput `json:"type,required"`
	// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status string `json:"status,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Output      respjson.Field
		Type        respjson.Field
		Status      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseInputItemLocalShellCallOutput) RawJSON() string { return r.JSON.raw }
func (r *ResponseInputItemLocalShellCallOutput) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A list of tools available on an MCP server.
type ResponseInputItemMcpListTools struct {
	// The unique ID of the list.
	ID string `json:"id,required"`
	// The label of the MCP server.
	ServerLabel string `json:"server_label,required"`
	// The tools available on the server.
	Tools []ResponseInputItemMcpListToolsTool `json:"tools,required"`
	// The type of the item. Always `mcp_list_tools`.
	Type constant.McpListTools `json:"type,required"`
	// Error message if the server could not list tools.
	Error string `json:"error,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ServerLabel respjson.Field
		Tools       respjson.Field
		Type        respjson.Field
		Error       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseInputItemMcpListTools) RawJSON() string { return r.JSON.raw }
func (r *ResponseInputItemMcpListTools) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A tool available on an MCP server.
type ResponseInputItemMcpListToolsTool struct {
	// The JSON schema describing the tool's input.
	InputSchema any `json:"input_schema,required"`
	// The name of the tool.
	Name string `json:"name,required"`
	// Additional annotations about the tool.
	Annotations any `json:"annotations,nullable"`
	// The description of the tool.
	Description string `json:"description,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		InputSchema respjson.Field
		Name        respjson.Field
		Annotations respjson.Field
		Description respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseInputItemMcpListToolsTool) RawJSON() string { return r.JSON.raw }
func (r *ResponseInputItemMcpListToolsTool) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A request for human approval of a tool invocation.
type ResponseInputItemMcpApprovalRequest struct {
	// The unique ID of the approval request.
	ID string `json:"id,required"`
	// A JSON string of arguments for the tool.
	Arguments string `json:"arguments,required"`
	// The name of the tool to run.
	Name string `json:"name,required"`
	// The label of the MCP server making the request.
	ServerLabel string `json:"server_label,required"`
	// The type of the item. Always `mcp_approval_request`.
	Type constant.McpApprovalRequest `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Arguments   respjson.Field
		Name        respjson.Field
		ServerLabel respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseInputItemMcpApprovalRequest) RawJSON() string { return r.JSON.raw }
func (r *ResponseInputItemMcpApprovalRequest) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A response to an MCP approval request.
type ResponseInputItemMcpApprovalResponse struct {
	// The ID of the approval request being answered.
	ApprovalRequestID string `json:"approval_request_id,required"`
	// Whether the request was approved.
	Approve bool `json:"approve,required"`
	// The type of the item. Always `mcp_approval_response`.
	Type constant.McpApprovalResponse `json:"type,required"`
	// The unique ID of the approval response
	ID string `json:"id,nullable"`
	// Optional reason for the decision.
	Reason string `json:"reason,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ApprovalRequestID respjson.Field
		Approve           respjson.Field
		Type              respjson.Field
		ID                respjson.Field
		Reason            respjson.Field
		ExtraFields       map[string]respjson.Field
		raw               string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseInputItemMcpApprovalResponse) RawJSON() string { return r.JSON.raw }
func (r *ResponseInputItemMcpApprovalResponse) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// An invocation of a tool on an MCP server.
type ResponseInputItemMcpCall struct {
	// The unique ID of the tool call.
	ID string `json:"id,required"`
	// A JSON string of the arguments passed to the tool.
	Arguments string `json:"arguments,required"`
	// The name of the tool that was run.
	Name string `json:"name,required"`
	// The label of the MCP server running the tool.
	ServerLabel string `json:"server_label,required"`
	// The type of the item. Always `mcp_call`.
	Type constant.McpCall `json:"type,required"`
	// The error from the tool call, if any.
	Error string `json:"error,nullable"`
	// The output from the tool call.
	Output string `json:"output,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Arguments   respjson.Field
		Name        respjson.Field
		ServerLabel respjson.Field
		Type        respjson.Field
		Error       respjson.Field
		Output      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseInputItemMcpCall) RawJSON() string { return r.JSON.raw }
func (r *ResponseInputItemMcpCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// An internal identifier for an item to reference.
type ResponseInputItemItemReference struct {
	// The ID of the item to reference.
	ID string `json:"id,required"`
	// The type of item to reference. Always `item_reference`.
	//
	// Any of "item_reference".
	Type string `json:"type,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseInputItemItemReference) RawJSON() string { return r.JSON.raw }
func (r *ResponseInputItemItemReference) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func ResponseInputItemParamOfMessage[T string | ResponseInputMessageContentListParam](content T, role EasyInputMessageRole) ResponseInputItemUnionParam {
	var message EasyInputMessageParam
	switch v := any(content).(type) {
	case string:
		message.Content.OfString = param.NewOpt(v)
	case ResponseInputMessageContentListParam:
		message.Content.OfInputItemContentList = v
	}
	message.Role = role
	return ResponseInputItemUnionParam{OfMessage: &message}
}

func ResponseInputItemParamOfInputMessage(content ResponseInputMessageContentListParam, role string) ResponseInputItemUnionParam {
	var message ResponseInputItemMessageParam
	message.Content = content
	message.Role = role
	return ResponseInputItemUnionParam{OfInputMessage: &message}
}

func ResponseInputItemParamOfOutputMessage(content []ResponseOutputMessageContentUnionParam, id string, status ResponseOutputMessageStatus) ResponseInputItemUnionParam {
	var message ResponseOutputMessageParam
	message.Content = content
	message.ID = id
	message.Status = status
	return ResponseInputItemUnionParam{OfOutputMessage: &message}
}

func ResponseInputItemParamOfFileSearchCall(id string, queries []string, status ResponseFileSearchToolCallStatus) ResponseInputItemUnionParam {
	var fileSearchCall ResponseFileSearchToolCallParam
	fileSearchCall.ID = id
	fileSearchCall.Queries = queries
	fileSearchCall.Status = status
	return ResponseInputItemUnionParam{OfFileSearchCall: &fileSearchCall}
}

func ResponseInputItemParamOfComputerCallOutput(callID string, output ResponseComputerToolCallOutputScreenshotParam) ResponseInputItemUnionParam {
	var computerCallOutput ResponseInputItemComputerCallOutputParam
	computerCallOutput.CallID = callID
	computerCallOutput.Output = output
	return ResponseInputItemUnionParam{OfComputerCallOutput: &computerCallOutput}
}

func ResponseInputItemParamOfWebSearchCall[
	T ResponseFunctionWebSearchActionSearchParam | ResponseFunctionWebSearchActionOpenPageParam | ResponseFunctionWebSearchActionFindParam,
](action T, id string, status ResponseFunctionWebSearchStatus) ResponseInputItemUnionParam {
	var webSearchCall ResponseFunctionWebSearchParam
	switch v := any(action).(type) {
	case ResponseFunctionWebSearchActionSearchParam:
		webSearchCall.Action.OfSearch = &v
	case ResponseFunctionWebSearchActionOpenPageParam:
		webSearchCall.Action.OfOpenPage = &v
	case ResponseFunctionWebSearchActionFindParam:
		webSearchCall.Action.OfFind = &v
	}
	webSearchCall.ID = id
	webSearchCall.Status = status
	return ResponseInputItemUnionParam{OfWebSearchCall: &webSearchCall}
}

func ResponseInputItemParamOfFunctionCall(arguments string, callID string, name string) ResponseInputItemUnionParam {
	var functionCall ResponseFunctionToolCallParam
	functionCall.Arguments = arguments
	functionCall.CallID = callID
	functionCall.Name = name
	return ResponseInputItemUnionParam{OfFunctionCall: &functionCall}
}

func ResponseInputItemParamOfFunctionCallOutput(callID string, output string) ResponseInputItemUnionParam {
	var functionCallOutput ResponseInputItemFunctionCallOutputParam
	functionCallOutput.CallID = callID
	functionCallOutput.Output = output
	return ResponseInputItemUnionParam{OfFunctionCallOutput: &functionCallOutput}
}

func ResponseInputItemParamOfReasoning(id string, summary []ResponseReasoningItemSummaryParam) ResponseInputItemUnionParam {
	var reasoning ResponseReasoningItemParam
	reasoning.ID = id
	reasoning.Summary = summary
	return ResponseInputItemUnionParam{OfReasoning: &reasoning}
}

func ResponseInputItemParamOfImageGenerationCall(id string, result string, status string) ResponseInputItemUnionParam {
	var imageGenerationCall ResponseInputItemImageGenerationCallParam
	imageGenerationCall.ID = id
	imageGenerationCall.Result = param.NewOpt(result)
	imageGenerationCall.Status = status
	return ResponseInputItemUnionParam{OfImageGenerationCall: &imageGenerationCall}
}

func ResponseInputItemParamOfLocalShellCallOutput(id string, output string) ResponseInputItemUnionParam {
	var localShellCallOutput ResponseInputItemLocalShellCallOutputParam
	localShellCallOutput.ID = id
	localShellCallOutput.Output = output
	return ResponseInputItemUnionParam{OfLocalShellCallOutput: &localShellCallOutput}
}

func ResponseInputItemParamOfMcpListTools(id string, serverLabel string, tools []ResponseInputItemMcpListToolsToolParam) ResponseInputItemUnionParam {
	var mcpListTools ResponseInputItemMcpListToolsParam
	mcpListTools.ID = id
	mcpListTools.ServerLabel = serverLabel
	mcpListTools.Tools = tools
	return ResponseInputItemUnionParam{OfMcpListTools: &mcpListTools}
}

func ResponseInputItemParamOfMcpApprovalResponse(approvalRequestID string, approve bool) ResponseInputItemUnionParam {
	var mcpApprovalResponse ResponseInputItemMcpApprovalResponseParam
	mcpApprovalResponse.ApprovalRequestID = approvalRequestID
	mcpApprovalResponse.Approve = approve
	return ResponseInputItemUnionParam{OfMcpApprovalResponse: &mcpApprovalResponse}
}

func ResponseInputItemParamOfItemReference(id string) ResponseInputItemUnionParam {
	var itemReference ResponseInputItemItemReferenceParam
	itemReference.ID = id
	return ResponseInputItemUnionParam{OfItemReference: &itemReference}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ResponseInputItemUnionParam struct {
	OfMessage              *EasyInputMessageParam                      `json:",omitzero,inline"`
	OfInputMessage         *ResponseInputItemMessageParam              `json:",omitzero,inline"`
	OfOutputMessage        *ResponseOutputMessageParam                 `json:",omitzero,inline"`
	OfFileSearchCall       *ResponseFileSearchToolCallParam            `json:",omitzero,inline"`
	OfComputerCall         *ResponseComputerToolCallParam              `json:",omitzero,inline"`
	OfComputerCallOutput   *ResponseInputItemComputerCallOutputParam   `json:",omitzero,inline"`
	OfWebSearchCall        *ResponseFunctionWebSearchParam             `json:",omitzero,inline"`
	OfFunctionCall         *ResponseFunctionToolCallParam              `json:",omitzero,inline"`
	OfFunctionCallOutput   *ResponseInputItemFunctionCallOutputParam   `json:",omitzero,inline"`
	OfReasoning            *ResponseReasoningItemParam                 `json:",omitzero,inline"`
	OfImageGenerationCall  *ResponseInputItemImageGenerationCallParam  `json:",omitzero,inline"`
	OfCodeInterpreterCall  *ResponseCodeInterpreterToolCallParam       `json:",omitzero,inline"`
	OfLocalShellCall       *ResponseInputItemLocalShellCallParam       `json:",omitzero,inline"`
	OfLocalShellCallOutput *ResponseInputItemLocalShellCallOutputParam `json:",omitzero,inline"`
	OfMcpListTools         *ResponseInputItemMcpListToolsParam         `json:",omitzero,inline"`
	OfMcpApprovalRequest   *ResponseInputItemMcpApprovalRequestParam   `json:",omitzero,inline"`
	OfMcpApprovalResponse  *ResponseInputItemMcpApprovalResponseParam  `json:",omitzero,inline"`
	OfMcpCall              *ResponseInputItemMcpCallParam              `json:",omitzero,inline"`
	OfItemReference        *ResponseInputItemItemReferenceParam        `json:",omitzero,inline"`
	paramUnion
}

func (u ResponseInputItemUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfMessage,
		u.OfInputMessage,
		u.OfOutputMessage,
		u.OfFileSearchCall,
		u.OfComputerCall,
		u.OfComputerCallOutput,
		u.OfWebSearchCall,
		u.OfFunctionCall,
		u.OfFunctionCallOutput,
		u.OfReasoning,
		u.OfImageGenerationCall,
		u.OfCodeInterpreterCall,
		u.OfLocalShellCall,
		u.OfLocalShellCallOutput,
		u.OfMcpListTools,
		u.OfMcpApprovalRequest,
		u.OfMcpApprovalResponse,
		u.OfMcpCall,
		u.OfItemReference)
}
func (u *ResponseInputItemUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ResponseInputItemUnionParam) asAny() any {
	if !param.IsOmitted(u.OfMessage) {
		return u.OfMessage
	} else if !param.IsOmitted(u.OfInputMessage) {
		return u.OfInputMessage
	} else if !param.IsOmitted(u.OfOutputMessage) {
		return u.OfOutputMessage
	} else if !param.IsOmitted(u.OfFileSearchCall) {
		return u.OfFileSearchCall
	} else if !param.IsOmitted(u.OfComputerCall) {
		return u.OfComputerCall
	} else if !param.IsOmitted(u.OfComputerCallOutput) {
		return u.OfComputerCallOutput
	} else if !param.IsOmitted(u.OfWebSearchCall) {
		return u.OfWebSearchCall
	} else if !param.IsOmitted(u.OfFunctionCall) {
		return u.OfFunctionCall
	} else if !param.IsOmitted(u.OfFunctionCallOutput) {
		return u.OfFunctionCallOutput
	} else if !param.IsOmitted(u.OfReasoning) {
		return u.OfReasoning
	} else if !param.IsOmitted(u.OfImageGenerationCall) {
		return u.OfImageGenerationCall
	} else if !param.IsOmitted(u.OfCodeInterpreterCall) {
		return u.OfCodeInterpreterCall
	} else if !param.IsOmitted(u.OfLocalShellCall) {
		return u.OfLocalShellCall
	} else if !param.IsOmitted(u.OfLocalShellCallOutput) {
		return u.OfLocalShellCallOutput
	} else if !param.IsOmitted(u.OfMcpListTools) {
		return u.OfMcpListTools
	} else if !param.IsOmitted(u.OfMcpApprovalRequest) {
		return u.OfMcpApprovalRequest
	} else if !param.IsOmitted(u.OfMcpApprovalResponse) {
		return u.OfMcpApprovalResponse
	} else if !param.IsOmitted(u.OfMcpCall) {
		return u.OfMcpCall
	} else if !param.IsOmitted(u.OfItemReference) {
		return u.OfItemReference
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetQueries() []string {
	if vt := u.OfFileSearchCall; vt != nil {
		return vt.Queries
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetResults() []ResponseFileSearchToolCallResultParam {
	if vt := u.OfFileSearchCall; vt != nil {
		return vt.Results
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetPendingSafetyChecks() []ResponseComputerToolCallPendingSafetyCheckParam {
	if vt := u.OfComputerCall; vt != nil {
		return vt.PendingSafetyChecks
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetAcknowledgedSafetyChecks() []ResponseInputItemComputerCallOutputAcknowledgedSafetyCheckParam {
	if vt := u.OfComputerCallOutput; vt != nil {
		return vt.AcknowledgedSafetyChecks
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetSummary() []ResponseReasoningItemSummaryParam {
	if vt := u.OfReasoning; vt != nil {
		return vt.Summary
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetEncryptedContent() *string {
	if vt := u.OfReasoning; vt != nil && vt.EncryptedContent.Valid() {
		return &vt.EncryptedContent.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetResult() *string {
	if vt := u.OfImageGenerationCall; vt != nil && vt.Result.Valid() {
		return &vt.Result.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetCode() *string {
	if vt := u.OfCodeInterpreterCall; vt != nil && vt.Code.Valid() {
		return &vt.Code.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetContainerID() *string {
	if vt := u.OfCodeInterpreterCall; vt != nil {
		return &vt.ContainerID
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetOutputs() []ResponseCodeInterpreterToolCallOutputUnionParam {
	if vt := u.OfCodeInterpreterCall; vt != nil {
		return vt.Outputs
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetTools() []ResponseInputItemMcpListToolsToolParam {
	if vt := u.OfMcpListTools; vt != nil {
		return vt.Tools
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetApprovalRequestID() *string {
	if vt := u.OfMcpApprovalResponse; vt != nil {
		return &vt.ApprovalRequestID
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetApprove() *bool {
	if vt := u.OfMcpApprovalResponse; vt != nil {
		return &vt.Approve
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetReason() *string {
	if vt := u.OfMcpApprovalResponse; vt != nil && vt.Reason.Valid() {
		return &vt.Reason.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetRole() *string {
	if vt := u.OfMessage; vt != nil {
		return (*string)(&vt.Role)
	} else if vt := u.OfInputMessage; vt != nil {
		return (*string)(&vt.Role)
	} else if vt := u.OfOutputMessage; vt != nil {
		return (*string)(&vt.Role)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetType() *string {
	if vt := u.OfMessage; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfInputMessage; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfOutputMessage; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfFileSearchCall; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfComputerCall; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfComputerCallOutput; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfWebSearchCall; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfFunctionCall; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfFunctionCallOutput; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfReasoning; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfImageGenerationCall; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfCodeInterpreterCall; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfLocalShellCall; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfLocalShellCallOutput; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfMcpListTools; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfMcpApprovalRequest; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfMcpApprovalResponse; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfMcpCall; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfItemReference; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetStatus() *string {
	if vt := u.OfInputMessage; vt != nil {
		return (*string)(&vt.Status)
	} else if vt := u.OfOutputMessage; vt != nil {
		return (*string)(&vt.Status)
	} else if vt := u.OfFileSearchCall; vt != nil {
		return (*string)(&vt.Status)
	} else if vt := u.OfComputerCall; vt != nil {
		return (*string)(&vt.Status)
	} else if vt := u.OfComputerCallOutput; vt != nil {
		return (*string)(&vt.Status)
	} else if vt := u.OfWebSearchCall; vt != nil {
		return (*string)(&vt.Status)
	} else if vt := u.OfFunctionCall; vt != nil {
		return (*string)(&vt.Status)
	} else if vt := u.OfFunctionCallOutput; vt != nil {
		return (*string)(&vt.Status)
	} else if vt := u.OfReasoning; vt != nil {
		return (*string)(&vt.Status)
	} else if vt := u.OfImageGenerationCall; vt != nil {
		return (*string)(&vt.Status)
	} else if vt := u.OfCodeInterpreterCall; vt != nil {
		return (*string)(&vt.Status)
	} else if vt := u.OfLocalShellCall; vt != nil {
		return (*string)(&vt.Status)
	} else if vt := u.OfLocalShellCallOutput; vt != nil {
		return (*string)(&vt.Status)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetID() *string {
	if vt := u.OfOutputMessage; vt != nil {
		return (*string)(&vt.ID)
	} else if vt := u.OfFileSearchCall; vt != nil {
		return (*string)(&vt.ID)
	} else if vt := u.OfComputerCall; vt != nil {
		return (*string)(&vt.ID)
	} else if vt := u.OfComputerCallOutput; vt != nil && vt.ID.Valid() {
		return &vt.ID.Value
	} else if vt := u.OfWebSearchCall; vt != nil {
		return (*string)(&vt.ID)
	} else if vt := u.OfFunctionCall; vt != nil && vt.ID.Valid() {
		return &vt.ID.Value
	} else if vt := u.OfFunctionCallOutput; vt != nil && vt.ID.Valid() {
		return &vt.ID.Value
	} else if vt := u.OfReasoning; vt != nil {
		return (*string)(&vt.ID)
	} else if vt := u.OfImageGenerationCall; vt != nil {
		return (*string)(&vt.ID)
	} else if vt := u.OfCodeInterpreterCall; vt != nil {
		return (*string)(&vt.ID)
	} else if vt := u.OfLocalShellCall; vt != nil {
		return (*string)(&vt.ID)
	} else if vt := u.OfLocalShellCallOutput; vt != nil {
		return (*string)(&vt.ID)
	} else if vt := u.OfMcpListTools; vt != nil {
		return (*string)(&vt.ID)
	} else if vt := u.OfMcpApprovalRequest; vt != nil {
		return (*string)(&vt.ID)
	} else if vt := u.OfMcpApprovalResponse; vt != nil && vt.ID.Valid() {
		return &vt.ID.Value
	} else if vt := u.OfMcpCall; vt != nil {
		return (*string)(&vt.ID)
	} else if vt := u.OfItemReference; vt != nil {
		return (*string)(&vt.ID)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetCallID() *string {
	if vt := u.OfComputerCall; vt != nil {
		return (*string)(&vt.CallID)
	} else if vt := u.OfComputerCallOutput; vt != nil {
		return (*string)(&vt.CallID)
	} else if vt := u.OfFunctionCall; vt != nil {
		return (*string)(&vt.CallID)
	} else if vt := u.OfFunctionCallOutput; vt != nil {
		return (*string)(&vt.CallID)
	} else if vt := u.OfLocalShellCall; vt != nil {
		return (*string)(&vt.CallID)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetArguments() *string {
	if vt := u.OfFunctionCall; vt != nil {
		return (*string)(&vt.Arguments)
	} else if vt := u.OfMcpApprovalRequest; vt != nil {
		return (*string)(&vt.Arguments)
	} else if vt := u.OfMcpCall; vt != nil {
		return (*string)(&vt.Arguments)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetName() *string {
	if vt := u.OfFunctionCall; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfMcpApprovalRequest; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfMcpCall; vt != nil {
		return (*string)(&vt.Name)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetServerLabel() *string {
	if vt := u.OfMcpListTools; vt != nil {
		return (*string)(&vt.ServerLabel)
	} else if vt := u.OfMcpApprovalRequest; vt != nil {
		return (*string)(&vt.ServerLabel)
	} else if vt := u.OfMcpCall; vt != nil {
		return (*string)(&vt.ServerLabel)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetError() *string {
	if vt := u.OfMcpListTools; vt != nil && vt.Error.Valid() {
		return &vt.Error.Value
	} else if vt := u.OfMcpCall; vt != nil && vt.Error.Valid() {
		return &vt.Error.Value
	}
	return nil
}

// Returns a subunion which exports methods to access subproperties
//
// Or use AsAny() to get the underlying value
func (u ResponseInputItemUnionParam) GetContent() (res responseInputItemUnionParamContent) {
	if vt := u.OfMessage; vt != nil {
		res.any = vt.Content.asAny()
	} else if vt := u.OfInputMessage; vt != nil {
		res.any = &vt.Content
	} else if vt := u.OfOutputMessage; vt != nil {
		res.any = &vt.Content
	}
	return
}

// Can have the runtime types [*string], [*ResponseInputMessageContentListParam],
// [\*[]ResponseOutputMessageContentUnionParam]
type responseInputItemUnionParamContent struct{ any }

// Use the following switch statement to get the type of the union:
//
//	switch u.AsAny().(type) {
//	case *string:
//	case *responses.ResponseInputMessageContentListParam:
//	case *[]responses.ResponseOutputMessageContentUnionParam:
//	default:
//	    fmt.Errorf("not present")
//	}
func (u responseInputItemUnionParamContent) AsAny() any { return u.any }

// Returns a subunion which exports methods to access subproperties
//
// Or use AsAny() to get the underlying value
func (u ResponseInputItemUnionParam) GetAction() (res responseInputItemUnionParamAction) {
	if vt := u.OfComputerCall; vt != nil {
		res.any = vt.Action.asAny()
	} else if vt := u.OfWebSearchCall; vt != nil {
		res.any = vt.Action.asAny()
	} else if vt := u.OfLocalShellCall; vt != nil {
		res.any = &vt.Action
	}
	return
}

// Can have the runtime types [*ResponseComputerToolCallActionClickParam],
// [*ResponseComputerToolCallActionDoubleClickParam],
// [*ResponseComputerToolCallActionDragParam],
// [*ResponseComputerToolCallActionKeypressParam],
// [*ResponseComputerToolCallActionMoveParam],
// [*ResponseComputerToolCallActionScreenshotParam],
// [*ResponseComputerToolCallActionScrollParam],
// [*ResponseComputerToolCallActionTypeParam],
// [*ResponseComputerToolCallActionWaitParam],
// [*ResponseFunctionWebSearchActionSearchParam],
// [*ResponseFunctionWebSearchActionOpenPageParam],
// [*ResponseFunctionWebSearchActionFindParam],
// [*ResponseInputItemLocalShellCallActionParam]
type responseInputItemUnionParamAction struct{ any }

// Use the following switch statement to get the type of the union:
//
//	switch u.AsAny().(type) {
//	case *responses.ResponseComputerToolCallActionClickParam:
//	case *responses.ResponseComputerToolCallActionDoubleClickParam:
//	case *responses.ResponseComputerToolCallActionDragParam:
//	case *responses.ResponseComputerToolCallActionKeypressParam:
//	case *responses.ResponseComputerToolCallActionMoveParam:
//	case *responses.ResponseComputerToolCallActionScreenshotParam:
//	case *responses.ResponseComputerToolCallActionScrollParam:
//	case *responses.ResponseComputerToolCallActionTypeParam:
//	case *responses.ResponseComputerToolCallActionWaitParam:
//	case *responses.ResponseFunctionWebSearchActionSearchParam:
//	case *responses.ResponseFunctionWebSearchActionOpenPageParam:
//	case *responses.ResponseFunctionWebSearchActionFindParam:
//	case *responses.ResponseInputItemLocalShellCallActionParam:
//	default:
//	    fmt.Errorf("not present")
//	}
func (u responseInputItemUnionParamAction) AsAny() any { return u.any }

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamAction) GetButton() *string {
	switch vt := u.any.(type) {
	case *ResponseComputerToolCallActionUnionParam:
		return vt.GetButton()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamAction) GetPath() []ResponseComputerToolCallActionDragPathParam {
	switch vt := u.any.(type) {
	case *ResponseComputerToolCallActionUnionParam:
		return vt.GetPath()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamAction) GetKeys() []string {
	switch vt := u.any.(type) {
	case *ResponseComputerToolCallActionUnionParam:
		return vt.GetKeys()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamAction) GetScrollX() *int64 {
	switch vt := u.any.(type) {
	case *ResponseComputerToolCallActionUnionParam:
		return vt.GetScrollX()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamAction) GetScrollY() *int64 {
	switch vt := u.any.(type) {
	case *ResponseComputerToolCallActionUnionParam:
		return vt.GetScrollY()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamAction) GetText() *string {
	switch vt := u.any.(type) {
	case *ResponseComputerToolCallActionUnionParam:
		return vt.GetText()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamAction) GetQuery() *string {
	switch vt := u.any.(type) {
	case *ResponseFunctionWebSearchActionUnionParam:
		return vt.GetQuery()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamAction) GetPattern() *string {
	switch vt := u.any.(type) {
	case *ResponseFunctionWebSearchActionUnionParam:
		return vt.GetPattern()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamAction) GetCommand() []string {
	switch vt := u.any.(type) {
	case *ResponseInputItemLocalShellCallActionParam:
		return vt.Command
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamAction) GetEnv() map[string]string {
	switch vt := u.any.(type) {
	case *ResponseInputItemLocalShellCallActionParam:
		return vt.Env
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamAction) GetTimeoutMs() *int64 {
	switch vt := u.any.(type) {
	case *ResponseInputItemLocalShellCallActionParam:
		return paramutil.AddrIfPresent(vt.TimeoutMs)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamAction) GetUser() *string {
	switch vt := u.any.(type) {
	case *ResponseInputItemLocalShellCallActionParam:
		return paramutil.AddrIfPresent(vt.User)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamAction) GetWorkingDirectory() *string {
	switch vt := u.any.(type) {
	case *ResponseInputItemLocalShellCallActionParam:
		return paramutil.AddrIfPresent(vt.WorkingDirectory)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamAction) GetType() *string {
	switch vt := u.any.(type) {
	case *ResponseComputerToolCallActionUnionParam:
		return vt.GetType()
	case *ResponseFunctionWebSearchActionUnionParam:
		return vt.GetType()
	case *ResponseInputItemLocalShellCallActionParam:
		return (*string)(&vt.Type)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamAction) GetX() *int64 {
	switch vt := u.any.(type) {
	case *ResponseComputerToolCallActionUnionParam:
		return vt.GetX()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamAction) GetY() *int64 {
	switch vt := u.any.(type) {
	case *ResponseComputerToolCallActionUnionParam:
		return vt.GetY()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamAction) GetURL() *string {
	switch vt := u.any.(type) {
	case *ResponseFunctionWebSearchActionUnionParam:
		return vt.GetURL()
	}
	return nil
}

// Returns a subunion which exports methods to access subproperties
//
// Or use AsAny() to get the underlying value
func (u ResponseInputItemUnionParam) GetOutput() (res responseInputItemUnionParamOutput) {
	if vt := u.OfComputerCallOutput; vt != nil {
		res.any = &vt.Output
	} else if vt := u.OfFunctionCallOutput; vt != nil {
		res.any = &vt.Output
	} else if vt := u.OfLocalShellCallOutput; vt != nil {
		res.any = &vt.Output
	} else if vt := u.OfMcpCall; vt != nil && vt.Output.Valid() {
		res.any = &vt.Output.Value
	}
	return
}

// Can have the runtime types [*ResponseComputerToolCallOutputScreenshotParam],
// [*string]
type responseInputItemUnionParamOutput struct{ any }

// Use the following switch statement to get the type of the union:
//
//	switch u.AsAny().(type) {
//	case *responses.ResponseComputerToolCallOutputScreenshotParam:
//	case *string:
//	default:
//	    fmt.Errorf("not present")
//	}
func (u responseInputItemUnionParamOutput) AsAny() any { return u.any }

func init() {
	apijson.RegisterUnion[ResponseInputItemUnionParam](
		"type",
		apijson.Discriminator[EasyInputMessageParam]("message"),
		apijson.Discriminator[ResponseInputItemMessageParam]("message"),
		apijson.Discriminator[ResponseOutputMessageParam]("message"),
		apijson.Discriminator[ResponseFileSearchToolCallParam]("file_search_call"),
		apijson.Discriminator[ResponseComputerToolCallParam]("computer_call"),
		apijson.Discriminator[ResponseInputItemComputerCallOutputParam]("computer_call_output"),
		apijson.Discriminator[ResponseFunctionWebSearchParam]("web_search_call"),
		apijson.Discriminator[ResponseFunctionToolCallParam]("function_call"),
		apijson.Discriminator[ResponseInputItemFunctionCallOutputParam]("function_call_output"),
		apijson.Discriminator[ResponseReasoningItemParam]("reasoning"),
		apijson.Discriminator[ResponseInputItemImageGenerationCallParam]("image_generation_call"),
		apijson.Discriminator[ResponseCodeInterpreterToolCallParam]("code_interpreter_call"),
		apijson.Discriminator[ResponseInputItemLocalShellCallParam]("local_shell_call"),
		apijson.Discriminator[ResponseInputItemLocalShellCallOutputParam]("local_shell_call_output"),
		apijson.Discriminator[ResponseInputItemMcpListToolsParam]("mcp_list_tools"),
		apijson.Discriminator[ResponseInputItemMcpApprovalRequestParam]("mcp_approval_request"),
		apijson.Discriminator[ResponseInputItemMcpApprovalResponseParam]("mcp_approval_response"),
		apijson.Discriminator[ResponseInputItemMcpCallParam]("mcp_call"),
		apijson.Discriminator[ResponseInputItemItemReferenceParam]("item_reference"),
	)
}

// A message input to the model with a role indicating instruction following
// hierarchy. Instructions given with the `developer` or `system` role take
// precedence over instructions given with the `user` role.
//
// The properties Content, Role are required.
type ResponseInputItemMessageParam struct {
	// A list of one or many input items to the model, containing different content
	// types.
	Content ResponseInputMessageContentListParam `json:"content,omitzero,required"`
	// The role of the message input. One of `user`, `system`, or `developer`.
	//
	// Any of "user", "system", "developer".
	Role string `json:"role,omitzero,required"`
	// The status of item. One of `in_progress`, `completed`, or `incomplete`.
	// Populated when items are returned via API.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status string `json:"status,omitzero"`
	// The type of the message input. Always set to `message`.
	//
	// Any of "message".
	Type string `json:"type,omitzero"`
	paramObj
}

func (r ResponseInputItemMessageParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputItemMessageParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseInputItemMessageParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[ResponseInputItemMessageParam](
		"role", "user", "system", "developer",
	)
	apijson.RegisterFieldValidator[ResponseInputItemMessageParam](
		"status", "in_progress", "completed", "incomplete",
	)
	apijson.RegisterFieldValidator[ResponseInputItemMessageParam](
		"type", "message",
	)
}

// The output of a computer tool call.
//
// The properties CallID, Output, Type are required.
type ResponseInputItemComputerCallOutputParam struct {
	// The ID of the computer tool call that produced the output.
	CallID string `json:"call_id,required"`
	// A computer screenshot image used with the computer use tool.
	Output ResponseComputerToolCallOutputScreenshotParam `json:"output,omitzero,required"`
	// The ID of the computer tool call output.
	ID param.Opt[string] `json:"id,omitzero"`
	// The safety checks reported by the API that have been acknowledged by the
	// developer.
	AcknowledgedSafetyChecks []ResponseInputItemComputerCallOutputAcknowledgedSafetyCheckParam `json:"acknowledged_safety_checks,omitzero"`
	// The status of the message input. One of `in_progress`, `completed`, or
	// `incomplete`. Populated when input items are returned via API.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status string `json:"status,omitzero"`
	// The type of the computer tool call output. Always `computer_call_output`.
	//
	// This field can be elided, and will marshal its zero value as
	// "computer_call_output".
	Type constant.ComputerCallOutput `json:"type,required"`
	paramObj
}

func (r ResponseInputItemComputerCallOutputParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputItemComputerCallOutputParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseInputItemComputerCallOutputParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[ResponseInputItemComputerCallOutputParam](
		"status", "in_progress", "completed", "incomplete",
	)
}

// A pending safety check for the computer call.
//
// The property ID is required.
type ResponseInputItemComputerCallOutputAcknowledgedSafetyCheckParam struct {
	// The ID of the pending safety check.
	ID string `json:"id,required"`
	// The type of the pending safety check.
	Code param.Opt[string] `json:"code,omitzero"`
	// Details about the pending safety check.
	Message param.Opt[string] `json:"message,omitzero"`
	paramObj
}

func (r ResponseInputItemComputerCallOutputAcknowledgedSafetyCheckParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputItemComputerCallOutputAcknowledgedSafetyCheckParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseInputItemComputerCallOutputAcknowledgedSafetyCheckParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The output of a function tool call.
//
// The properties CallID, Output, Type are required.
type ResponseInputItemFunctionCallOutputParam struct {
	// The unique ID of the function tool call generated by the model.
	CallID string `json:"call_id,required"`
	// A JSON string of the output of the function tool call.
	Output string `json:"output,required"`
	// The unique ID of the function tool call output. Populated when this item is
	// returned via API.
	ID param.Opt[string] `json:"id,omitzero"`
	// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
	// Populated when items are returned via API.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status string `json:"status,omitzero"`
	// The type of the function tool call output. Always `function_call_output`.
	//
	// This field can be elided, and will marshal its zero value as
	// "function_call_output".
	Type constant.FunctionCallOutput `json:"type,required"`
	paramObj
}

func (r ResponseInputItemFunctionCallOutputParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputItemFunctionCallOutputParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseInputItemFunctionCallOutputParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[ResponseInputItemFunctionCallOutputParam](
		"status", "in_progress", "completed", "incomplete",
	)
}

// An image generation request made by the model.
//
// The properties ID, Result, Status, Type are required.
type ResponseInputItemImageGenerationCallParam struct {
	// The generated image encoded in base64.
	Result param.Opt[string] `json:"result,omitzero,required"`
	// The unique ID of the image generation call.
	ID string `json:"id,required"`
	// The status of the image generation call.
	//
	// Any of "in_progress", "completed", "generating", "failed".
	Status string `json:"status,omitzero,required"`
	// The type of the image generation call. Always `image_generation_call`.
	//
	// This field can be elided, and will marshal its zero value as
	// "image_generation_call".
	Type constant.ImageGenerationCall `json:"type,required"`
	paramObj
}

func (r ResponseInputItemImageGenerationCallParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputItemImageGenerationCallParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseInputItemImageGenerationCallParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[ResponseInputItemImageGenerationCallParam](
		"status", "in_progress", "completed", "generating", "failed",
	)
}

// A tool call to run a command on the local shell.
//
// The properties ID, Action, CallID, Status, Type are required.
type ResponseInputItemLocalShellCallParam struct {
	// The unique ID of the local shell call.
	ID string `json:"id,required"`
	// Execute a shell command on the server.
	Action ResponseInputItemLocalShellCallActionParam `json:"action,omitzero,required"`
	// The unique ID of the local shell tool call generated by the model.
	CallID string `json:"call_id,required"`
	// The status of the local shell call.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status string `json:"status,omitzero,required"`
	// The type of the local shell call. Always `local_shell_call`.
	//
	// This field can be elided, and will marshal its zero value as "local_shell_call".
	Type constant.LocalShellCall `json:"type,required"`
	paramObj
}

func (r ResponseInputItemLocalShellCallParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputItemLocalShellCallParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseInputItemLocalShellCallParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[ResponseInputItemLocalShellCallParam](
		"status", "in_progress", "completed", "incomplete",
	)
}

// Execute a shell command on the server.
//
// The properties Command, Env, Type are required.
type ResponseInputItemLocalShellCallActionParam struct {
	// The command to run.
	Command []string `json:"command,omitzero,required"`
	// Environment variables to set for the command.
	Env map[string]string `json:"env,omitzero,required"`
	// Optional timeout in milliseconds for the command.
	TimeoutMs param.Opt[int64] `json:"timeout_ms,omitzero"`
	// Optional user to run the command as.
	User param.Opt[string] `json:"user,omitzero"`
	// Optional working directory to run the command in.
	WorkingDirectory param.Opt[string] `json:"working_directory,omitzero"`
	// The type of the local shell action. Always `exec`.
	//
	// This field can be elided, and will marshal its zero value as "exec".
	Type constant.Exec `json:"type,required"`
	paramObj
}

func (r ResponseInputItemLocalShellCallActionParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputItemLocalShellCallActionParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseInputItemLocalShellCallActionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The output of a local shell tool call.
//
// The properties ID, Output, Type are required.
type ResponseInputItemLocalShellCallOutputParam struct {
	// The unique ID of the local shell tool call generated by the model.
	ID string `json:"id,required"`
	// A JSON string of the output of the local shell tool call.
	Output string `json:"output,required"`
	// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status string `json:"status,omitzero"`
	// The type of the local shell tool call output. Always `local_shell_call_output`.
	//
	// This field can be elided, and will marshal its zero value as
	// "local_shell_call_output".
	Type constant.LocalShellCallOutput `json:"type,required"`
	paramObj
}

func (r ResponseInputItemLocalShellCallOutputParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputItemLocalShellCallOutputParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseInputItemLocalShellCallOutputParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[ResponseInputItemLocalShellCallOutputParam](
		"status", "in_progress", "completed", "incomplete",
	)
}

// A list of tools available on an MCP server.
//
// The properties ID, ServerLabel, Tools, Type are required.
type ResponseInputItemMcpListToolsParam struct {
	// The unique ID of the list.
	ID string `json:"id,required"`
	// The label of the MCP server.
	ServerLabel string `json:"server_label,required"`
	// The tools available on the server.
	Tools []ResponseInputItemMcpListToolsToolParam `json:"tools,omitzero,required"`
	// Error message if the server could not list tools.
	Error param.Opt[string] `json:"error,omitzero"`
	// The type of the item. Always `mcp_list_tools`.
	//
	// This field can be elided, and will marshal its zero value as "mcp_list_tools".
	Type constant.McpListTools `json:"type,required"`
	paramObj
}

func (r ResponseInputItemMcpListToolsParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputItemMcpListToolsParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseInputItemMcpListToolsParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A tool available on an MCP server.
//
// The properties InputSchema, Name are required.
type ResponseInputItemMcpListToolsToolParam struct {
	// The JSON schema describing the tool's input.
	InputSchema any `json:"input_schema,omitzero,required"`
	// The name of the tool.
	Name string `json:"name,required"`
	// The description of the tool.
	Description param.Opt[string] `json:"description,omitzero"`
	// Additional annotations about the tool.
	Annotations any `json:"annotations,omitzero"`
	paramObj
}

func (r ResponseInputItemMcpListToolsToolParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputItemMcpListToolsToolParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseInputItemMcpListToolsToolParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A request for human approval of a tool invocation.
//
// The properties ID, Arguments, Name, ServerLabel, Type are required.
type ResponseInputItemMcpApprovalRequestParam struct {
	// The unique ID of the approval request.
	ID string `json:"id,required"`
	// A JSON string of arguments for the tool.
	Arguments string `json:"arguments,required"`
	// The name of the tool to run.
	Name string `json:"name,required"`
	// The label of the MCP server making the request.
	ServerLabel string `json:"server_label,required"`
	// The type of the item. Always `mcp_approval_request`.
	//
	// This field can be elided, and will marshal its zero value as
	// "mcp_approval_request".
	Type constant.McpApprovalRequest `json:"type,required"`
	paramObj
}

func (r ResponseInputItemMcpApprovalRequestParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputItemMcpApprovalRequestParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseInputItemMcpApprovalRequestParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A response to an MCP approval request.
//
// The properties ApprovalRequestID, Approve, Type are required.
type ResponseInputItemMcpApprovalResponseParam struct {
	// The ID of the approval request being answered.
	ApprovalRequestID string `json:"approval_request_id,required"`
	// Whether the request was approved.
	Approve bool `json:"approve,required"`
	// The unique ID of the approval response
	ID param.Opt[string] `json:"id,omitzero"`
	// Optional reason for the decision.
	Reason param.Opt[string] `json:"reason,omitzero"`
	// The type of the item. Always `mcp_approval_response`.
	//
	// This field can be elided, and will marshal its zero value as
	// "mcp_approval_response".
	Type constant.McpApprovalResponse `json:"type,required"`
	paramObj
}

func (r ResponseInputItemMcpApprovalResponseParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputItemMcpApprovalResponseParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseInputItemMcpApprovalResponseParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// An invocation of a tool on an MCP server.
//
// The properties ID, Arguments, Name, ServerLabel, Type are required.
type ResponseInputItemMcpCallParam struct {
	// The unique ID of the tool call.
	ID string `json:"id,required"`
	// A JSON string of the arguments passed to the tool.
	Arguments string `json:"arguments,required"`
	// The name of the tool that was run.
	Name string `json:"name,required"`
	// The label of the MCP server running the tool.
	ServerLabel string `json:"server_label,required"`
	// The error from the tool call, if any.
	Error param.Opt[string] `json:"error,omitzero"`
	// The output from the tool call.
	Output param.Opt[string] `json:"output,omitzero"`
	// The type of the item. Always `mcp_call`.
	//
	// This field can be elided, and will marshal its zero value as "mcp_call".
	Type constant.McpCall `json:"type,required"`
	paramObj
}

func (r ResponseInputItemMcpCallParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputItemMcpCallParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseInputItemMcpCallParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// An internal identifier for an item to reference.
//
// The property ID is required.
type ResponseInputItemItemReferenceParam struct {
	// The ID of the item to reference.
	ID string `json:"id,required"`
	// The type of item to reference. Always `item_reference`.
	//
	// Any of "item_reference".
	Type string `json:"type,omitzero"`
	paramObj
}

func (r ResponseInputItemItemReferenceParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputItemItemReferenceParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseInputItemItemReferenceParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[ResponseInputItemItemReferenceParam](
		"type", "item_reference",
	)
}

type ResponseInputMessageContentList []ResponseInputContentUnion

type ResponseInputMessageContentListParam []ResponseInputContentUnionParam

type ResponseInputMessageItem struct {
	// The unique ID of the message input.
	ID string `json:"id,required"`
	// A list of one or many input items to the model, containing different content
	// types.
	Content ResponseInputMessageContentList `json:"content,required"`
	// The role of the message input. One of `user`, `system`, or `developer`.
	//
	// Any of "user", "system", "developer".
	Role ResponseInputMessageItemRole `json:"role,required"`
	// The status of item. One of `in_progress`, `completed`, or `incomplete`.
	// Populated when items are returned via API.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status ResponseInputMessageItemStatus `json:"status"`
	// The type of the message input. Always set to `message`.
	//
	// Any of "message".
	Type ResponseInputMessageItemType `json:"type"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Content     respjson.Field
		Role        respjson.Field
		Status      respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseInputMessageItem) RawJSON() string { return r.JSON.raw }
func (r *ResponseInputMessageItem) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The role of the message input. One of `user`, `system`, or `developer`.
type ResponseInputMessageItemRole string

const (
	ResponseInputMessageItemRoleUser      ResponseInputMessageItemRole = "user"
	ResponseInputMessageItemRoleSystem    ResponseInputMessageItemRole = "system"
	ResponseInputMessageItemRoleDeveloper ResponseInputMessageItemRole = "developer"
)

// The status of item. One of `in_progress`, `completed`, or `incomplete`.
// Populated when items are returned via API.
type ResponseInputMessageItemStatus string

const (
	ResponseInputMessageItemStatusInProgress ResponseInputMessageItemStatus = "in_progress"
	ResponseInputMessageItemStatusCompleted  ResponseInputMessageItemStatus = "completed"
	ResponseInputMessageItemStatusIncomplete ResponseInputMessageItemStatus = "incomplete"
)

// The type of the message input. Always set to `message`.
type ResponseInputMessageItemType string

const (
	ResponseInputMessageItemTypeMessage ResponseInputMessageItemType = "message"
)

// A text input to the model.
type ResponseInputText struct {
	// The text input to the model.
	Text string `json:"text,required"`
	// The type of the input item. Always `input_text`.
	Type constant.InputText `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Text        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseInputText) RawJSON() string { return r.JSON.raw }
func (r *ResponseInputText) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponseInputText to a ResponseInputTextParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseInputTextParam.Overrides()
func (r ResponseInputText) ToParam() ResponseInputTextParam {
	return param.Override[ResponseInputTextParam](json.RawMessage(r.RawJSON()))
}

// A text input to the model.
//
// The properties Text, Type are required.
type ResponseInputTextParam struct {
	// The text input to the model.
	Text string `json:"text,required"`
	// The type of the input item. Always `input_text`.
	//
	// This field can be elided, and will marshal its zero value as "input_text".
	Type constant.InputText `json:"type,required"`
	paramObj
}

func (r ResponseInputTextParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputTextParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseInputTextParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseItemUnion contains all possible properties and values from
// [ResponseInputMessageItem], [ResponseOutputMessage],
// [ResponseFileSearchToolCall], [ResponseComputerToolCall],
// [ResponseComputerToolCallOutputItem], [ResponseFunctionWebSearch],
// [ResponseFunctionToolCallItem], [ResponseFunctionToolCallOutputItem],
// [ResponseItemImageGenerationCall], [ResponseCodeInterpreterToolCall],
// [ResponseItemLocalShellCall], [ResponseItemLocalShellCallOutput],
// [ResponseItemMcpListTools], [ResponseItemMcpApprovalRequest],
// [ResponseItemMcpApprovalResponse], [ResponseItemMcpCall].
//
// Use the [ResponseItemUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ResponseItemUnion struct {
	ID string `json:"id"`
	// This field is a union of [ResponseInputMessageContentList],
	// [[]ResponseOutputMessageContentUnion]
	Content ResponseItemUnionContent `json:"content"`
	Role    string                   `json:"role"`
	Status  string                   `json:"status"`
	// Any of "message", "message", "file_search_call", "computer_call",
	// "computer_call_output", "web_search_call", "function_call",
	// "function_call_output", "image_generation_call", "code_interpreter_call",
	// "local_shell_call", "local_shell_call_output", "mcp_list_tools",
	// "mcp_approval_request", "mcp_approval_response", "mcp_call".
	Type string `json:"type"`
	// This field is from variant [ResponseFileSearchToolCall].
	Queries []string `json:"queries"`
	// This field is from variant [ResponseFileSearchToolCall].
	Results []ResponseFileSearchToolCallResult `json:"results"`
	// This field is a union of [ResponseComputerToolCallActionUnion],
	// [ResponseFunctionWebSearchActionUnion], [ResponseItemLocalShellCallAction]
	Action ResponseItemUnionAction `json:"action"`
	CallID string                  `json:"call_id"`
	// This field is from variant [ResponseComputerToolCall].
	PendingSafetyChecks []ResponseComputerToolCallPendingSafetyCheck `json:"pending_safety_checks"`
	// This field is a union of [ResponseComputerToolCallOutputScreenshot], [string],
	// [string], [string]
	Output ResponseItemUnionOutput `json:"output"`
	// This field is from variant [ResponseComputerToolCallOutputItem].
	AcknowledgedSafetyChecks []ResponseComputerToolCallOutputItemAcknowledgedSafetyCheck `json:"acknowledged_safety_checks"`
	Arguments                string                                                      `json:"arguments"`
	Name                     string                                                      `json:"name"`
	// This field is from variant [ResponseItemImageGenerationCall].
	Result string `json:"result"`
	// This field is from variant [ResponseCodeInterpreterToolCall].
	Code string `json:"code"`
	// This field is from variant [ResponseCodeInterpreterToolCall].
	ContainerID string `json:"container_id"`
	// This field is from variant [ResponseCodeInterpreterToolCall].
	Outputs     []ResponseCodeInterpreterToolCallOutputUnion `json:"outputs"`
	ServerLabel string                                       `json:"server_label"`
	// This field is from variant [ResponseItemMcpListTools].
	Tools []ResponseItemMcpListToolsTool `json:"tools"`
	Error string                         `json:"error"`
	// This field is from variant [ResponseItemMcpApprovalResponse].
	ApprovalRequestID string `json:"approval_request_id"`
	// This field is from variant [ResponseItemMcpApprovalResponse].
	Approve bool `json:"approve"`
	// This field is from variant [ResponseItemMcpApprovalResponse].
	Reason string `json:"reason"`
	JSON   struct {
		ID                       respjson.Field
		Content                  respjson.Field
		Role                     respjson.Field
		Status                   respjson.Field
		Type                     respjson.Field
		Queries                  respjson.Field
		Results                  respjson.Field
		Action                   respjson.Field
		CallID                   respjson.Field
		PendingSafetyChecks      respjson.Field
		Output                   respjson.Field
		AcknowledgedSafetyChecks respjson.Field
		Arguments                respjson.Field
		Name                     respjson.Field
		Result                   respjson.Field
		Code                     respjson.Field
		ContainerID              respjson.Field
		Outputs                  respjson.Field
		ServerLabel              respjson.Field
		Tools                    respjson.Field
		Error                    respjson.Field
		ApprovalRequestID        respjson.Field
		Approve                  respjson.Field
		Reason                   respjson.Field
		raw                      string
	} `json:"-"`
}

// anyResponseItem is implemented by each variant of [ResponseItemUnion] to add
// type safety for the return type of [ResponseItemUnion.AsAny]
type anyResponseItem interface {
	implResponseItemUnion()
}

func (ResponseInputMessageItem) implResponseItemUnion()           {}
func (ResponseOutputMessage) implResponseItemUnion()              {}
func (ResponseFileSearchToolCall) implResponseItemUnion()         {}
func (ResponseComputerToolCall) implResponseItemUnion()           {}
func (ResponseComputerToolCallOutputItem) implResponseItemUnion() {}
func (ResponseFunctionWebSearch) implResponseItemUnion()          {}
func (ResponseFunctionToolCallItem) implResponseItemUnion()       {}
func (ResponseFunctionToolCallOutputItem) implResponseItemUnion() {}
func (ResponseItemImageGenerationCall) implResponseItemUnion()    {}
func (ResponseCodeInterpreterToolCall) implResponseItemUnion()    {}
func (ResponseItemLocalShellCall) implResponseItemUnion()         {}
func (ResponseItemLocalShellCallOutput) implResponseItemUnion()   {}
func (ResponseItemMcpListTools) implResponseItemUnion()           {}
func (ResponseItemMcpApprovalRequest) implResponseItemUnion()     {}
func (ResponseItemMcpApprovalResponse) implResponseItemUnion()    {}
func (ResponseItemMcpCall) implResponseItemUnion()                {}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseItemUnion.AsAny().(type) {
//	case responses.ResponseInputMessageItem:
//	case responses.ResponseOutputMessage:
//	case responses.ResponseFileSearchToolCall:
//	case responses.ResponseComputerToolCall:
//	case responses.ResponseComputerToolCallOutputItem:
//	case responses.ResponseFunctionWebSearch:
//	case responses.ResponseFunctionToolCallItem:
//	case responses.ResponseFunctionToolCallOutputItem:
//	case responses.ResponseItemImageGenerationCall:
//	case responses.ResponseCodeInterpreterToolCall:
//	case responses.ResponseItemLocalShellCall:
//	case responses.ResponseItemLocalShellCallOutput:
//	case responses.ResponseItemMcpListTools:
//	case responses.ResponseItemMcpApprovalRequest:
//	case responses.ResponseItemMcpApprovalResponse:
//	case responses.ResponseItemMcpCall:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseItemUnion) AsAny() anyResponseItem {
	switch u.Type {
	case "message":
		return u.AsOutputMessage()
	case "file_search_call":
		return u.AsFileSearchCall()
	case "computer_call":
		return u.AsComputerCall()
	case "computer_call_output":
		return u.AsComputerCallOutput()
	case "web_search_call":
		return u.AsWebSearchCall()
	case "function_call":
		return u.AsFunctionCall()
	case "function_call_output":
		return u.AsFunctionCallOutput()
	case "image_generation_call":
		return u.AsImageGenerationCall()
	case "code_interpreter_call":
		return u.AsCodeInterpreterCall()
	case "local_shell_call":
		return u.AsLocalShellCall()
	case "local_shell_call_output":
		return u.AsLocalShellCallOutput()
	case "mcp_list_tools":
		return u.AsMcpListTools()
	case "mcp_approval_request":
		return u.AsMcpApprovalRequest()
	case "mcp_approval_response":
		return u.AsMcpApprovalResponse()
	case "mcp_call":
		return u.AsMcpCall()
	}
	return nil
}

func (u ResponseItemUnion) AsMessage() (v ResponseInputMessageItem) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseItemUnion) AsOutputMessage() (v ResponseOutputMessage) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseItemUnion) AsFileSearchCall() (v ResponseFileSearchToolCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseItemUnion) AsComputerCall() (v ResponseComputerToolCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseItemUnion) AsComputerCallOutput() (v ResponseComputerToolCallOutputItem) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseItemUnion) AsWebSearchCall() (v ResponseFunctionWebSearch) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseItemUnion) AsFunctionCall() (v ResponseFunctionToolCallItem) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseItemUnion) AsFunctionCallOutput() (v ResponseFunctionToolCallOutputItem) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseItemUnion) AsImageGenerationCall() (v ResponseItemImageGenerationCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseItemUnion) AsCodeInterpreterCall() (v ResponseCodeInterpreterToolCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseItemUnion) AsLocalShellCall() (v ResponseItemLocalShellCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseItemUnion) AsLocalShellCallOutput() (v ResponseItemLocalShellCallOutput) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseItemUnion) AsMcpListTools() (v ResponseItemMcpListTools) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseItemUnion) AsMcpApprovalRequest() (v ResponseItemMcpApprovalRequest) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseItemUnion) AsMcpApprovalResponse() (v ResponseItemMcpApprovalResponse) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseItemUnion) AsMcpCall() (v ResponseItemMcpCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ResponseItemUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseItemUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseItemUnionContent is an implicit subunion of [ResponseItemUnion].
// ResponseItemUnionContent provides convenient access to the sub-properties of the
// union.
//
// For type safety it is recommended to directly use a variant of the
// [ResponseItemUnion].
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfInputItemContentList OfResponseOutputMessageContentArray]
type ResponseItemUnionContent struct {
	// This field will be present if the value is a [ResponseInputMessageContentList]
	// instead of an object.
	OfInputItemContentList ResponseInputMessageContentList `json:",inline"`
	// This field will be present if the value is a
	// [[]ResponseOutputMessageContentUnion] instead of an object.
	OfResponseOutputMessageContentArray []ResponseOutputMessageContentUnion `json:",inline"`
	JSON                                struct {
		OfInputItemContentList              respjson.Field
		OfResponseOutputMessageContentArray respjson.Field
		raw                                 string
	} `json:"-"`
}

func (r *ResponseItemUnionContent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseItemUnionAction is an implicit subunion of [ResponseItemUnion].
// ResponseItemUnionAction provides convenient access to the sub-properties of the
// union.
//
// For type safety it is recommended to directly use a variant of the
// [ResponseItemUnion].
type ResponseItemUnionAction struct {
	// This field is from variant [ResponseComputerToolCallActionUnion].
	Button string `json:"button"`
	Type   string `json:"type"`
	X      int64  `json:"x"`
	Y      int64  `json:"y"`
	// This field is from variant [ResponseComputerToolCallActionUnion].
	Path []ResponseComputerToolCallActionDragPath `json:"path"`
	// This field is from variant [ResponseComputerToolCallActionUnion].
	Keys []string `json:"keys"`
	// This field is from variant [ResponseComputerToolCallActionUnion].
	ScrollX int64 `json:"scroll_x"`
	// This field is from variant [ResponseComputerToolCallActionUnion].
	ScrollY int64 `json:"scroll_y"`
	// This field is from variant [ResponseComputerToolCallActionUnion].
	Text string `json:"text"`
	// This field is from variant [ResponseFunctionWebSearchActionUnion].
	Query string `json:"query"`
	URL   string `json:"url"`
	// This field is from variant [ResponseFunctionWebSearchActionUnion].
	Pattern string `json:"pattern"`
	// This field is from variant [ResponseItemLocalShellCallAction].
	Command []string `json:"command"`
	// This field is from variant [ResponseItemLocalShellCallAction].
	Env map[string]string `json:"env"`
	// This field is from variant [ResponseItemLocalShellCallAction].
	TimeoutMs int64 `json:"timeout_ms"`
	// This field is from variant [ResponseItemLocalShellCallAction].
	User string `json:"user"`
	// This field is from variant [ResponseItemLocalShellCallAction].
	WorkingDirectory string `json:"working_directory"`
	JSON             struct {
		Button           respjson.Field
		Type             respjson.Field
		X                respjson.Field
		Y                respjson.Field
		Path             respjson.Field
		Keys             respjson.Field
		ScrollX          respjson.Field
		ScrollY          respjson.Field
		Text             respjson.Field
		Query            respjson.Field
		URL              respjson.Field
		Pattern          respjson.Field
		Command          respjson.Field
		Env              respjson.Field
		TimeoutMs        respjson.Field
		User             respjson.Field
		WorkingDirectory respjson.Field
		raw              string
	} `json:"-"`
}

func (r *ResponseItemUnionAction) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseItemUnionOutput is an implicit subunion of [ResponseItemUnion].
// ResponseItemUnionOutput provides convenient access to the sub-properties of the
// union.
//
// For type safety it is recommended to directly use a variant of the
// [ResponseItemUnion].
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfString]
type ResponseItemUnionOutput struct {
	// This field will be present if the value is a [string] instead of an object.
	OfString string `json:",inline"`
	// This field is from variant [ResponseComputerToolCallOutputScreenshot].
	Type constant.ComputerScreenshot `json:"type"`
	// This field is from variant [ResponseComputerToolCallOutputScreenshot].
	FileID string `json:"file_id"`
	// This field is from variant [ResponseComputerToolCallOutputScreenshot].
	ImageURL string `json:"image_url"`
	JSON     struct {
		OfString respjson.Field
		Type     respjson.Field
		FileID   respjson.Field
		ImageURL respjson.Field
		raw      string
	} `json:"-"`
}

func (r *ResponseItemUnionOutput) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// An image generation request made by the model.
type ResponseItemImageGenerationCall struct {
	// The unique ID of the image generation call.
	ID string `json:"id,required"`
	// The generated image encoded in base64.
	Result string `json:"result,required"`
	// The status of the image generation call.
	//
	// Any of "in_progress", "completed", "generating", "failed".
	Status string `json:"status,required"`
	// The type of the image generation call. Always `image_generation_call`.
	Type constant.ImageGenerationCall `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Result      respjson.Field
		Status      respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseItemImageGenerationCall) RawJSON() string { return r.JSON.raw }
func (r *ResponseItemImageGenerationCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A tool call to run a command on the local shell.
type ResponseItemLocalShellCall struct {
	// The unique ID of the local shell call.
	ID string `json:"id,required"`
	// Execute a shell command on the server.
	Action ResponseItemLocalShellCallAction `json:"action,required"`
	// The unique ID of the local shell tool call generated by the model.
	CallID string `json:"call_id,required"`
	// The status of the local shell call.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status string `json:"status,required"`
	// The type of the local shell call. Always `local_shell_call`.
	Type constant.LocalShellCall `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Action      respjson.Field
		CallID      respjson.Field
		Status      respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseItemLocalShellCall) RawJSON() string { return r.JSON.raw }
func (r *ResponseItemLocalShellCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Execute a shell command on the server.
type ResponseItemLocalShellCallAction struct {
	// The command to run.
	Command []string `json:"command,required"`
	// Environment variables to set for the command.
	Env map[string]string `json:"env,required"`
	// The type of the local shell action. Always `exec`.
	Type constant.Exec `json:"type,required"`
	// Optional timeout in milliseconds for the command.
	TimeoutMs int64 `json:"timeout_ms,nullable"`
	// Optional user to run the command as.
	User string `json:"user,nullable"`
	// Optional working directory to run the command in.
	WorkingDirectory string `json:"working_directory,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Command          respjson.Field
		Env              respjson.Field
		Type             respjson.Field
		TimeoutMs        respjson.Field
		User             respjson.Field
		WorkingDirectory respjson.Field
		ExtraFields      map[string]respjson.Field
		raw              string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseItemLocalShellCallAction) RawJSON() string { return r.JSON.raw }
func (r *ResponseItemLocalShellCallAction) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The output of a local shell tool call.
type ResponseItemLocalShellCallOutput struct {
	// The unique ID of the local shell tool call generated by the model.
	ID string `json:"id,required"`
	// A JSON string of the output of the local shell tool call.
	Output string `json:"output,required"`
	// The type of the local shell tool call output. Always `local_shell_call_output`.
	Type constant.LocalShellCallOutput `json:"type,required"`
	// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status string `json:"status,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Output      respjson.Field
		Type        respjson.Field
		Status      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseItemLocalShellCallOutput) RawJSON() string { return r.JSON.raw }
func (r *ResponseItemLocalShellCallOutput) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A list of tools available on an MCP server.
type ResponseItemMcpListTools struct {
	// The unique ID of the list.
	ID string `json:"id,required"`
	// The label of the MCP server.
	ServerLabel string `json:"server_label,required"`
	// The tools available on the server.
	Tools []ResponseItemMcpListToolsTool `json:"tools,required"`
	// The type of the item. Always `mcp_list_tools`.
	Type constant.McpListTools `json:"type,required"`
	// Error message if the server could not list tools.
	Error string `json:"error,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ServerLabel respjson.Field
		Tools       respjson.Field
		Type        respjson.Field
		Error       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseItemMcpListTools) RawJSON() string { return r.JSON.raw }
func (r *ResponseItemMcpListTools) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A tool available on an MCP server.
type ResponseItemMcpListToolsTool struct {
	// The JSON schema describing the tool's input.
	InputSchema any `json:"input_schema,required"`
	// The name of the tool.
	Name string `json:"name,required"`
	// Additional annotations about the tool.
	Annotations any `json:"annotations,nullable"`
	// The description of the tool.
	Description string `json:"description,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		InputSchema respjson.Field
		Name        respjson.Field
		Annotations respjson.Field
		Description respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseItemMcpListToolsTool) RawJSON() string { return r.JSON.raw }
func (r *ResponseItemMcpListToolsTool) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A request for human approval of a tool invocation.
type ResponseItemMcpApprovalRequest struct {
	// The unique ID of the approval request.
	ID string `json:"id,required"`
	// A JSON string of arguments for the tool.
	Arguments string `json:"arguments,required"`
	// The name of the tool to run.
	Name string `json:"name,required"`
	// The label of the MCP server making the request.
	ServerLabel string `json:"server_label,required"`
	// The type of the item. Always `mcp_approval_request`.
	Type constant.McpApprovalRequest `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Arguments   respjson.Field
		Name        respjson.Field
		ServerLabel respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseItemMcpApprovalRequest) RawJSON() string { return r.JSON.raw }
func (r *ResponseItemMcpApprovalRequest) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A response to an MCP approval request.
type ResponseItemMcpApprovalResponse struct {
	// The unique ID of the approval response
	ID string `json:"id,required"`
	// The ID of the approval request being answered.
	ApprovalRequestID string `json:"approval_request_id,required"`
	// Whether the request was approved.
	Approve bool `json:"approve,required"`
	// The type of the item. Always `mcp_approval_response`.
	Type constant.McpApprovalResponse `json:"type,required"`
	// Optional reason for the decision.
	Reason string `json:"reason,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID                respjson.Field
		ApprovalRequestID respjson.Field
		Approve           respjson.Field
		Type              respjson.Field
		Reason            respjson.Field
		ExtraFields       map[string]respjson.Field
		raw               string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseItemMcpApprovalResponse) RawJSON() string { return r.JSON.raw }
func (r *ResponseItemMcpApprovalResponse) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// An invocation of a tool on an MCP server.
type ResponseItemMcpCall struct {
	// The unique ID of the tool call.
	ID string `json:"id,required"`
	// A JSON string of the arguments passed to the tool.
	Arguments string `json:"arguments,required"`
	// The name of the tool that was run.
	Name string `json:"name,required"`
	// The label of the MCP server running the tool.
	ServerLabel string `json:"server_label,required"`
	// The type of the item. Always `mcp_call`.
	Type constant.McpCall `json:"type,required"`
	// The error from the tool call, if any.
	Error string `json:"error,nullable"`
	// The output from the tool call.
	Output string `json:"output,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Arguments   respjson.Field
		Name        respjson.Field
		ServerLabel respjson.Field
		Type        respjson.Field
		Error       respjson.Field
		Output      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseItemMcpCall) RawJSON() string { return r.JSON.raw }
func (r *ResponseItemMcpCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when there is a delta (partial update) to the arguments of an MCP tool
// call.
type ResponseMcpCallArgumentsDeltaEvent struct {
	// The partial update to the arguments for the MCP tool call.
	Delta any `json:"delta,required"`
	// The unique identifier of the MCP tool call item being processed.
	ItemID string `json:"item_id,required"`
	// The index of the output item in the response's output array.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always 'response.mcp_call.arguments_delta'.
	Type constant.ResponseMcpCallArgumentsDelta `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Delta          respjson.Field
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseMcpCallArgumentsDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseMcpCallArgumentsDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when the arguments for an MCP tool call are finalized.
type ResponseMcpCallArgumentsDoneEvent struct {
	// The finalized arguments for the MCP tool call.
	Arguments any `json:"arguments,required"`
	// The unique identifier of the MCP tool call item being processed.
	ItemID string `json:"item_id,required"`
	// The index of the output item in the response's output array.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always 'response.mcp_call.arguments_done'.
	Type constant.ResponseMcpCallArgumentsDone `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Arguments      respjson.Field
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseMcpCallArgumentsDoneEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseMcpCallArgumentsDoneEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when an MCP tool call has completed successfully.
type ResponseMcpCallCompletedEvent struct {
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always 'response.mcp_call.completed'.
	Type constant.ResponseMcpCallCompleted `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseMcpCallCompletedEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseMcpCallCompletedEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when an MCP tool call has failed.
type ResponseMcpCallFailedEvent struct {
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always 'response.mcp_call.failed'.
	Type constant.ResponseMcpCallFailed `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseMcpCallFailedEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseMcpCallFailedEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when an MCP tool call is in progress.
type ResponseMcpCallInProgressEvent struct {
	// The unique identifier of the MCP tool call item being processed.
	ItemID string `json:"item_id,required"`
	// The index of the output item in the response's output array.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always 'response.mcp_call.in_progress'.
	Type constant.ResponseMcpCallInProgress `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseMcpCallInProgressEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseMcpCallInProgressEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when the list of available MCP tools has been successfully retrieved.
type ResponseMcpListToolsCompletedEvent struct {
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always 'response.mcp_list_tools.completed'.
	Type constant.ResponseMcpListToolsCompleted `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseMcpListToolsCompletedEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseMcpListToolsCompletedEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when the attempt to list available MCP tools has failed.
type ResponseMcpListToolsFailedEvent struct {
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always 'response.mcp_list_tools.failed'.
	Type constant.ResponseMcpListToolsFailed `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseMcpListToolsFailedEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseMcpListToolsFailedEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when the system is in the process of retrieving the list of available
// MCP tools.
type ResponseMcpListToolsInProgressEvent struct {
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always 'response.mcp_list_tools.in_progress'.
	Type constant.ResponseMcpListToolsInProgress `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseMcpListToolsInProgressEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseMcpListToolsInProgressEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseOutputItemUnion contains all possible properties and values from
// [ResponseOutputMessage], [ResponseFileSearchToolCall],
// [ResponseFunctionToolCall], [ResponseFunctionWebSearch],
// [ResponseComputerToolCall], [ResponseReasoningItem],
// [ResponseOutputItemImageGenerationCall], [ResponseCodeInterpreterToolCall],
// [ResponseOutputItemLocalShellCall], [ResponseOutputItemMcpCall],
// [ResponseOutputItemMcpListTools], [ResponseOutputItemMcpApprovalRequest].
//
// Use the [ResponseOutputItemUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ResponseOutputItemUnion struct {
	ID string `json:"id"`
	// This field is from variant [ResponseOutputMessage].
	Content []ResponseOutputMessageContentUnion `json:"content"`
	// This field is from variant [ResponseOutputMessage].
	Role   constant.Assistant `json:"role"`
	Status string             `json:"status"`
	// Any of "message", "file_search_call", "function_call", "web_search_call",
	// "computer_call", "reasoning", "image_generation_call", "code_interpreter_call",
	// "local_shell_call", "mcp_call", "mcp_list_tools", "mcp_approval_request".
	Type string `json:"type"`
	// This field is from variant [ResponseFileSearchToolCall].
	Queries []string `json:"queries"`
	// This field is from variant [ResponseFileSearchToolCall].
	Results   []ResponseFileSearchToolCallResult `json:"results"`
	Arguments string                             `json:"arguments"`
	CallID    string                             `json:"call_id"`
	Name      string                             `json:"name"`
	// This field is a union of [ResponseFunctionWebSearchActionUnion],
	// [ResponseComputerToolCallActionUnion], [ResponseOutputItemLocalShellCallAction]
	Action ResponseOutputItemUnionAction `json:"action"`
	// This field is from variant [ResponseComputerToolCall].
	PendingSafetyChecks []ResponseComputerToolCallPendingSafetyCheck `json:"pending_safety_checks"`
	// This field is from variant [ResponseReasoningItem].
	Summary []ResponseReasoningItemSummary `json:"summary"`
	// This field is from variant [ResponseReasoningItem].
	EncryptedContent string `json:"encrypted_content"`
	// This field is from variant [ResponseOutputItemImageGenerationCall].
	Result string `json:"result"`
	// This field is from variant [ResponseCodeInterpreterToolCall].
	Code string `json:"code"`
	// This field is from variant [ResponseCodeInterpreterToolCall].
	ContainerID string `json:"container_id"`
	// This field is from variant [ResponseCodeInterpreterToolCall].
	Outputs     []ResponseCodeInterpreterToolCallOutputUnion `json:"outputs"`
	ServerLabel string                                       `json:"server_label"`
	Error       string                                       `json:"error"`
	// This field is from variant [ResponseOutputItemMcpCall].
	Output string `json:"output"`
	// This field is from variant [ResponseOutputItemMcpListTools].
	Tools []ResponseOutputItemMcpListToolsTool `json:"tools"`
	JSON  struct {
		ID                  respjson.Field
		Content             respjson.Field
		Role                respjson.Field
		Status              respjson.Field
		Type                respjson.Field
		Queries             respjson.Field
		Results             respjson.Field
		Arguments           respjson.Field
		CallID              respjson.Field
		Name                respjson.Field
		Action              respjson.Field
		PendingSafetyChecks respjson.Field
		Summary             respjson.Field
		EncryptedContent    respjson.Field
		Result              respjson.Field
		Code                respjson.Field
		ContainerID         respjson.Field
		Outputs             respjson.Field
		ServerLabel         respjson.Field
		Error               respjson.Field
		Output              respjson.Field
		Tools               respjson.Field
		raw                 string
	} `json:"-"`
}

// anyResponseOutputItem is implemented by each variant of
// [ResponseOutputItemUnion] to add type safety for the return type of
// [ResponseOutputItemUnion.AsAny]
type anyResponseOutputItem interface {
	implResponseOutputItemUnion()
}

func (ResponseOutputMessage) implResponseOutputItemUnion()                 {}
func (ResponseFileSearchToolCall) implResponseOutputItemUnion()            {}
func (ResponseFunctionToolCall) implResponseOutputItemUnion()              {}
func (ResponseFunctionWebSearch) implResponseOutputItemUnion()             {}
func (ResponseComputerToolCall) implResponseOutputItemUnion()              {}
func (ResponseReasoningItem) implResponseOutputItemUnion()                 {}
func (ResponseOutputItemImageGenerationCall) implResponseOutputItemUnion() {}
func (ResponseCodeInterpreterToolCall) implResponseOutputItemUnion()       {}
func (ResponseOutputItemLocalShellCall) implResponseOutputItemUnion()      {}
func (ResponseOutputItemMcpCall) implResponseOutputItemUnion()             {}
func (ResponseOutputItemMcpListTools) implResponseOutputItemUnion()        {}
func (ResponseOutputItemMcpApprovalRequest) implResponseOutputItemUnion()  {}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseOutputItemUnion.AsAny().(type) {
//	case responses.ResponseOutputMessage:
//	case responses.ResponseFileSearchToolCall:
//	case responses.ResponseFunctionToolCall:
//	case responses.ResponseFunctionWebSearch:
//	case responses.ResponseComputerToolCall:
//	case responses.ResponseReasoningItem:
//	case responses.ResponseOutputItemImageGenerationCall:
//	case responses.ResponseCodeInterpreterToolCall:
//	case responses.ResponseOutputItemLocalShellCall:
//	case responses.ResponseOutputItemMcpCall:
//	case responses.ResponseOutputItemMcpListTools:
//	case responses.ResponseOutputItemMcpApprovalRequest:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseOutputItemUnion) AsAny() anyResponseOutputItem {
	switch u.Type {
	case "message":
		return u.AsMessage()
	case "file_search_call":
		return u.AsFileSearchCall()
	case "function_call":
		return u.AsFunctionCall()
	case "web_search_call":
		return u.AsWebSearchCall()
	case "computer_call":
		return u.AsComputerCall()
	case "reasoning":
		return u.AsReasoning()
	case "image_generation_call":
		return u.AsImageGenerationCall()
	case "code_interpreter_call":
		return u.AsCodeInterpreterCall()
	case "local_shell_call":
		return u.AsLocalShellCall()
	case "mcp_call":
		return u.AsMcpCall()
	case "mcp_list_tools":
		return u.AsMcpListTools()
	case "mcp_approval_request":
		return u.AsMcpApprovalRequest()
	}
	return nil
}

func (u ResponseOutputItemUnion) AsMessage() (v ResponseOutputMessage) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseOutputItemUnion) AsFileSearchCall() (v ResponseFileSearchToolCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseOutputItemUnion) AsFunctionCall() (v ResponseFunctionToolCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseOutputItemUnion) AsWebSearchCall() (v ResponseFunctionWebSearch) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseOutputItemUnion) AsComputerCall() (v ResponseComputerToolCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseOutputItemUnion) AsReasoning() (v ResponseReasoningItem) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseOutputItemUnion) AsImageGenerationCall() (v ResponseOutputItemImageGenerationCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseOutputItemUnion) AsCodeInterpreterCall() (v ResponseCodeInterpreterToolCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseOutputItemUnion) AsLocalShellCall() (v ResponseOutputItemLocalShellCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseOutputItemUnion) AsMcpCall() (v ResponseOutputItemMcpCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseOutputItemUnion) AsMcpListTools() (v ResponseOutputItemMcpListTools) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseOutputItemUnion) AsMcpApprovalRequest() (v ResponseOutputItemMcpApprovalRequest) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ResponseOutputItemUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseOutputItemUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseOutputItemUnionAction is an implicit subunion of
// [ResponseOutputItemUnion]. ResponseOutputItemUnionAction provides convenient
// access to the sub-properties of the union.
//
// For type safety it is recommended to directly use a variant of the
// [ResponseOutputItemUnion].
type ResponseOutputItemUnionAction struct {
	// This field is from variant [ResponseFunctionWebSearchActionUnion].
	Query string `json:"query"`
	Type  string `json:"type"`
	URL   string `json:"url"`
	// This field is from variant [ResponseFunctionWebSearchActionUnion].
	Pattern string `json:"pattern"`
	// This field is from variant [ResponseComputerToolCallActionUnion].
	Button string `json:"button"`
	X      int64  `json:"x"`
	Y      int64  `json:"y"`
	// This field is from variant [ResponseComputerToolCallActionUnion].
	Path []ResponseComputerToolCallActionDragPath `json:"path"`
	// This field is from variant [ResponseComputerToolCallActionUnion].
	Keys []string `json:"keys"`
	// This field is from variant [ResponseComputerToolCallActionUnion].
	ScrollX int64 `json:"scroll_x"`
	// This field is from variant [ResponseComputerToolCallActionUnion].
	ScrollY int64 `json:"scroll_y"`
	// This field is from variant [ResponseComputerToolCallActionUnion].
	Text string `json:"text"`
	// This field is from variant [ResponseOutputItemLocalShellCallAction].
	Command []string `json:"command"`
	// This field is from variant [ResponseOutputItemLocalShellCallAction].
	Env map[string]string `json:"env"`
	// This field is from variant [ResponseOutputItemLocalShellCallAction].
	TimeoutMs int64 `json:"timeout_ms"`
	// This field is from variant [ResponseOutputItemLocalShellCallAction].
	User string `json:"user"`
	// This field is from variant [ResponseOutputItemLocalShellCallAction].
	WorkingDirectory string `json:"working_directory"`
	JSON             struct {
		Query            respjson.Field
		Type             respjson.Field
		URL              respjson.Field
		Pattern          respjson.Field
		Button           respjson.Field
		X                respjson.Field
		Y                respjson.Field
		Path             respjson.Field
		Keys             respjson.Field
		ScrollX          respjson.Field
		ScrollY          respjson.Field
		Text             respjson.Field
		Command          respjson.Field
		Env              respjson.Field
		TimeoutMs        respjson.Field
		User             respjson.Field
		WorkingDirectory respjson.Field
		raw              string
	} `json:"-"`
}

func (r *ResponseOutputItemUnionAction) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// An image generation request made by the model.
type ResponseOutputItemImageGenerationCall struct {
	// The unique ID of the image generation call.
	ID string `json:"id,required"`
	// The generated image encoded in base64.
	Result string `json:"result,required"`
	// The status of the image generation call.
	//
	// Any of "in_progress", "completed", "generating", "failed".
	Status string `json:"status,required"`
	// The type of the image generation call. Always `image_generation_call`.
	Type constant.ImageGenerationCall `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Result      respjson.Field
		Status      respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputItemImageGenerationCall) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputItemImageGenerationCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A tool call to run a command on the local shell.
type ResponseOutputItemLocalShellCall struct {
	// The unique ID of the local shell call.
	ID string `json:"id,required"`
	// Execute a shell command on the server.
	Action ResponseOutputItemLocalShellCallAction `json:"action,required"`
	// The unique ID of the local shell tool call generated by the model.
	CallID string `json:"call_id,required"`
	// The status of the local shell call.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status string `json:"status,required"`
	// The type of the local shell call. Always `local_shell_call`.
	Type constant.LocalShellCall `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Action      respjson.Field
		CallID      respjson.Field
		Status      respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputItemLocalShellCall) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputItemLocalShellCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Execute a shell command on the server.
type ResponseOutputItemLocalShellCallAction struct {
	// The command to run.
	Command []string `json:"command,required"`
	// Environment variables to set for the command.
	Env map[string]string `json:"env,required"`
	// The type of the local shell action. Always `exec`.
	Type constant.Exec `json:"type,required"`
	// Optional timeout in milliseconds for the command.
	TimeoutMs int64 `json:"timeout_ms,nullable"`
	// Optional user to run the command as.
	User string `json:"user,nullable"`
	// Optional working directory to run the command in.
	WorkingDirectory string `json:"working_directory,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Command          respjson.Field
		Env              respjson.Field
		Type             respjson.Field
		TimeoutMs        respjson.Field
		User             respjson.Field
		WorkingDirectory respjson.Field
		ExtraFields      map[string]respjson.Field
		raw              string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputItemLocalShellCallAction) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputItemLocalShellCallAction) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// An invocation of a tool on an MCP server.
type ResponseOutputItemMcpCall struct {
	// The unique ID of the tool call.
	ID string `json:"id,required"`
	// A JSON string of the arguments passed to the tool.
	Arguments string `json:"arguments,required"`
	// The name of the tool that was run.
	Name string `json:"name,required"`
	// The label of the MCP server running the tool.
	ServerLabel string `json:"server_label,required"`
	// The type of the item. Always `mcp_call`.
	Type constant.McpCall `json:"type,required"`
	// The error from the tool call, if any.
	Error string `json:"error,nullable"`
	// The output from the tool call.
	Output string `json:"output,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Arguments   respjson.Field
		Name        respjson.Field
		ServerLabel respjson.Field
		Type        respjson.Field
		Error       respjson.Field
		Output      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputItemMcpCall) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputItemMcpCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A list of tools available on an MCP server.
type ResponseOutputItemMcpListTools struct {
	// The unique ID of the list.
	ID string `json:"id,required"`
	// The label of the MCP server.
	ServerLabel string `json:"server_label,required"`
	// The tools available on the server.
	Tools []ResponseOutputItemMcpListToolsTool `json:"tools,required"`
	// The type of the item. Always `mcp_list_tools`.
	Type constant.McpListTools `json:"type,required"`
	// Error message if the server could not list tools.
	Error string `json:"error,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ServerLabel respjson.Field
		Tools       respjson.Field
		Type        respjson.Field
		Error       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputItemMcpListTools) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputItemMcpListTools) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A tool available on an MCP server.
type ResponseOutputItemMcpListToolsTool struct {
	// The JSON schema describing the tool's input.
	InputSchema any `json:"input_schema,required"`
	// The name of the tool.
	Name string `json:"name,required"`
	// Additional annotations about the tool.
	Annotations any `json:"annotations,nullable"`
	// The description of the tool.
	Description string `json:"description,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		InputSchema respjson.Field
		Name        respjson.Field
		Annotations respjson.Field
		Description respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputItemMcpListToolsTool) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputItemMcpListToolsTool) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A request for human approval of a tool invocation.
type ResponseOutputItemMcpApprovalRequest struct {
	// The unique ID of the approval request.
	ID string `json:"id,required"`
	// A JSON string of arguments for the tool.
	Arguments string `json:"arguments,required"`
	// The name of the tool to run.
	Name string `json:"name,required"`
	// The label of the MCP server making the request.
	ServerLabel string `json:"server_label,required"`
	// The type of the item. Always `mcp_approval_request`.
	Type constant.McpApprovalRequest `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Arguments   respjson.Field
		Name        respjson.Field
		ServerLabel respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputItemMcpApprovalRequest) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputItemMcpApprovalRequest) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a new output item is added.
type ResponseOutputItemAddedEvent struct {
	// The output item that was added.
	Item ResponseOutputItemUnion `json:"item,required"`
	// The index of the output item that was added.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.output_item.added`.
	Type constant.ResponseOutputItemAdded `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Item           respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputItemAddedEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputItemAddedEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when an output item is marked done.
type ResponseOutputItemDoneEvent struct {
	// The output item that was marked done.
	Item ResponseOutputItemUnion `json:"item,required"`
	// The index of the output item that was marked done.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.output_item.done`.
	Type constant.ResponseOutputItemDone `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Item           respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputItemDoneEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputItemDoneEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// An output message from the model.
type ResponseOutputMessage struct {
	// The unique ID of the output message.
	ID string `json:"id,required"`
	// The content of the output message.
	Content []ResponseOutputMessageContentUnion `json:"content,required"`
	// The role of the output message. Always `assistant`.
	Role constant.Assistant `json:"role,required"`
	// The status of the message input. One of `in_progress`, `completed`, or
	// `incomplete`. Populated when input items are returned via API.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status ResponseOutputMessageStatus `json:"status,required"`
	// The type of the output message. Always `message`.
	Type constant.Message `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Content     respjson.Field
		Role        respjson.Field
		Status      respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputMessage) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputMessage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponseOutputMessage to a ResponseOutputMessageParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseOutputMessageParam.Overrides()
func (r ResponseOutputMessage) ToParam() ResponseOutputMessageParam {
	return param.Override[ResponseOutputMessageParam](json.RawMessage(r.RawJSON()))
}

// ResponseOutputMessageContentUnion contains all possible properties and values
// from [ResponseOutputText], [ResponseOutputRefusal].
//
// Use the [ResponseOutputMessageContentUnion.AsAny] method to switch on the
// variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ResponseOutputMessageContentUnion struct {
	// This field is from variant [ResponseOutputText].
	Annotations []ResponseOutputTextAnnotationUnion `json:"annotations"`
	// This field is from variant [ResponseOutputText].
	Text string `json:"text"`
	// Any of "output_text", "refusal".
	Type string `json:"type"`
	// This field is from variant [ResponseOutputText].
	Logprobs []ResponseOutputTextLogprob `json:"logprobs"`
	// This field is from variant [ResponseOutputRefusal].
	Refusal string `json:"refusal"`
	JSON    struct {
		Annotations respjson.Field
		Text        respjson.Field
		Type        respjson.Field
		Logprobs    respjson.Field
		Refusal     respjson.Field
		raw         string
	} `json:"-"`
}

// anyResponseOutputMessageContent is implemented by each variant of
// [ResponseOutputMessageContentUnion] to add type safety for the return type of
// [ResponseOutputMessageContentUnion.AsAny]
type anyResponseOutputMessageContent interface {
	implResponseOutputMessageContentUnion()
}

func (ResponseOutputText) implResponseOutputMessageContentUnion()    {}
func (ResponseOutputRefusal) implResponseOutputMessageContentUnion() {}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseOutputMessageContentUnion.AsAny().(type) {
//	case responses.ResponseOutputText:
//	case responses.ResponseOutputRefusal:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseOutputMessageContentUnion) AsAny() anyResponseOutputMessageContent {
	switch u.Type {
	case "output_text":
		return u.AsOutputText()
	case "refusal":
		return u.AsRefusal()
	}
	return nil
}

func (u ResponseOutputMessageContentUnion) AsOutputText() (v ResponseOutputText) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseOutputMessageContentUnion) AsRefusal() (v ResponseOutputRefusal) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ResponseOutputMessageContentUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseOutputMessageContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The status of the message input. One of `in_progress`, `completed`, or
// `incomplete`. Populated when input items are returned via API.
type ResponseOutputMessageStatus string

const (
	ResponseOutputMessageStatusInProgress ResponseOutputMessageStatus = "in_progress"
	ResponseOutputMessageStatusCompleted  ResponseOutputMessageStatus = "completed"
	ResponseOutputMessageStatusIncomplete ResponseOutputMessageStatus = "incomplete"
)

// An output message from the model.
//
// The properties ID, Content, Role, Status, Type are required.
type ResponseOutputMessageParam struct {
	// The unique ID of the output message.
	ID string `json:"id,omitzero,required"`
	// The content of the output message.
	Content []ResponseOutputMessageContentUnionParam `json:"content,omitzero,required"`
	// The status of the message input. One of `in_progress`, `completed`, or
	// `incomplete`. Populated when input items are returned via API.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status ResponseOutputMessageStatus `json:"status,omitzero,required"`
	// The role of the output message. Always `assistant`.
	//
	// This field can be elided, and will marshal its zero value as "assistant".
	Role constant.Assistant `json:"role,required"`
	// The type of the output message. Always `message`.
	//
	// This field can be elided, and will marshal its zero value as "message".
	Type constant.Message `json:"type,required"`
	paramObj
}

func (r ResponseOutputMessageParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseOutputMessageParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseOutputMessageParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ResponseOutputMessageContentUnionParam struct {
	OfOutputText *ResponseOutputTextParam    `json:",omitzero,inline"`
	OfRefusal    *ResponseOutputRefusalParam `json:",omitzero,inline"`
	paramUnion
}

func (u ResponseOutputMessageContentUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfOutputText, u.OfRefusal)
}
func (u *ResponseOutputMessageContentUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ResponseOutputMessageContentUnionParam) asAny() any {
	if !param.IsOmitted(u.OfOutputText) {
		return u.OfOutputText
	} else if !param.IsOmitted(u.OfRefusal) {
		return u.OfRefusal
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseOutputMessageContentUnionParam) GetAnnotations() []ResponseOutputTextAnnotationUnionParam {
	if vt := u.OfOutputText; vt != nil {
		return vt.Annotations
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseOutputMessageContentUnionParam) GetText() *string {
	if vt := u.OfOutputText; vt != nil {
		return &vt.Text
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseOutputMessageContentUnionParam) GetLogprobs() []ResponseOutputTextLogprobParam {
	if vt := u.OfOutputText; vt != nil {
		return vt.Logprobs
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseOutputMessageContentUnionParam) GetRefusal() *string {
	if vt := u.OfRefusal; vt != nil {
		return &vt.Refusal
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseOutputMessageContentUnionParam) GetType() *string {
	if vt := u.OfOutputText; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfRefusal; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[ResponseOutputMessageContentUnionParam](
		"type",
		apijson.Discriminator[ResponseOutputTextParam]("output_text"),
		apijson.Discriminator[ResponseOutputRefusalParam]("refusal"),
	)
}

// A refusal from the model.
type ResponseOutputRefusal struct {
	// The refusal explanationfrom the model.
	Refusal string `json:"refusal,required"`
	// The type of the refusal. Always `refusal`.
	Type constant.Refusal `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Refusal     respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputRefusal) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputRefusal) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponseOutputRefusal to a ResponseOutputRefusalParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseOutputRefusalParam.Overrides()
func (r ResponseOutputRefusal) ToParam() ResponseOutputRefusalParam {
	return param.Override[ResponseOutputRefusalParam](json.RawMessage(r.RawJSON()))
}

// A refusal from the model.
//
// The properties Refusal, Type are required.
type ResponseOutputRefusalParam struct {
	// The refusal explanationfrom the model.
	Refusal string `json:"refusal,required"`
	// The type of the refusal. Always `refusal`.
	//
	// This field can be elided, and will marshal its zero value as "refusal".
	Type constant.Refusal `json:"type,required"`
	paramObj
}

func (r ResponseOutputRefusalParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseOutputRefusalParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseOutputRefusalParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A text output from the model.
type ResponseOutputText struct {
	// The annotations of the text output.
	Annotations []ResponseOutputTextAnnotationUnion `json:"annotations,required"`
	// The text output from the model.
	Text string `json:"text,required"`
	// The type of the output text. Always `output_text`.
	Type     constant.OutputText         `json:"type,required"`
	Logprobs []ResponseOutputTextLogprob `json:"logprobs"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Annotations respjson.Field
		Text        respjson.Field
		Type        respjson.Field
		Logprobs    respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputText) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputText) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponseOutputText to a ResponseOutputTextParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseOutputTextParam.Overrides()
func (r ResponseOutputText) ToParam() ResponseOutputTextParam {
	return param.Override[ResponseOutputTextParam](json.RawMessage(r.RawJSON()))
}

// ResponseOutputTextAnnotationUnion contains all possible properties and values
// from [ResponseOutputTextAnnotationFileCitation],
// [ResponseOutputTextAnnotationURLCitation],
// [ResponseOutputTextAnnotationContainerFileCitation],
// [ResponseOutputTextAnnotationFilePath].
//
// Use the [ResponseOutputTextAnnotationUnion.AsAny] method to switch on the
// variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ResponseOutputTextAnnotationUnion struct {
	FileID   string `json:"file_id"`
	Filename string `json:"filename"`
	Index    int64  `json:"index"`
	// Any of "file_citation", "url_citation", "container_file_citation", "file_path".
	Type       string `json:"type"`
	EndIndex   int64  `json:"end_index"`
	StartIndex int64  `json:"start_index"`
	// This field is from variant [ResponseOutputTextAnnotationURLCitation].
	Title string `json:"title"`
	// This field is from variant [ResponseOutputTextAnnotationURLCitation].
	URL string `json:"url"`
	// This field is from variant [ResponseOutputTextAnnotationContainerFileCitation].
	ContainerID string `json:"container_id"`
	JSON        struct {
		FileID      respjson.Field
		Filename    respjson.Field
		Index       respjson.Field
		Type        respjson.Field
		EndIndex    respjson.Field
		StartIndex  respjson.Field
		Title       respjson.Field
		URL         respjson.Field
		ContainerID respjson.Field
		raw         string
	} `json:"-"`
}

// anyResponseOutputTextAnnotation is implemented by each variant of
// [ResponseOutputTextAnnotationUnion] to add type safety for the return type of
// [ResponseOutputTextAnnotationUnion.AsAny]
type anyResponseOutputTextAnnotation interface {
	implResponseOutputTextAnnotationUnion()
}

func (ResponseOutputTextAnnotationFileCitation) implResponseOutputTextAnnotationUnion()          {}
func (ResponseOutputTextAnnotationURLCitation) implResponseOutputTextAnnotationUnion()           {}
func (ResponseOutputTextAnnotationContainerFileCitation) implResponseOutputTextAnnotationUnion() {}
func (ResponseOutputTextAnnotationFilePath) implResponseOutputTextAnnotationUnion()              {}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseOutputTextAnnotationUnion.AsAny().(type) {
//	case responses.ResponseOutputTextAnnotationFileCitation:
//	case responses.ResponseOutputTextAnnotationURLCitation:
//	case responses.ResponseOutputTextAnnotationContainerFileCitation:
//	case responses.ResponseOutputTextAnnotationFilePath:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseOutputTextAnnotationUnion) AsAny() anyResponseOutputTextAnnotation {
	switch u.Type {
	case "file_citation":
		return u.AsFileCitation()
	case "url_citation":
		return u.AsURLCitation()
	case "container_file_citation":
		return u.AsContainerFileCitation()
	case "file_path":
		return u.AsFilePath()
	}
	return nil
}

func (u ResponseOutputTextAnnotationUnion) AsFileCitation() (v ResponseOutputTextAnnotationFileCitation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseOutputTextAnnotationUnion) AsURLCitation() (v ResponseOutputTextAnnotationURLCitation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseOutputTextAnnotationUnion) AsContainerFileCitation() (v ResponseOutputTextAnnotationContainerFileCitation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseOutputTextAnnotationUnion) AsFilePath() (v ResponseOutputTextAnnotationFilePath) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ResponseOutputTextAnnotationUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseOutputTextAnnotationUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A citation to a file.
type ResponseOutputTextAnnotationFileCitation struct {
	// The ID of the file.
	FileID string `json:"file_id,required"`
	// The filename of the file cited.
	Filename string `json:"filename,required"`
	// The index of the file in the list of files.
	Index int64 `json:"index,required"`
	// The type of the file citation. Always `file_citation`.
	Type constant.FileCitation `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		FileID      respjson.Field
		Filename    respjson.Field
		Index       respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputTextAnnotationFileCitation) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputTextAnnotationFileCitation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A citation for a web resource used to generate a model response.
type ResponseOutputTextAnnotationURLCitation struct {
	// The index of the last character of the URL citation in the message.
	EndIndex int64 `json:"end_index,required"`
	// The index of the first character of the URL citation in the message.
	StartIndex int64 `json:"start_index,required"`
	// The title of the web resource.
	Title string `json:"title,required"`
	// The type of the URL citation. Always `url_citation`.
	Type constant.URLCitation `json:"type,required"`
	// The URL of the web resource.
	URL string `json:"url,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		EndIndex    respjson.Field
		StartIndex  respjson.Field
		Title       respjson.Field
		Type        respjson.Field
		URL         respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputTextAnnotationURLCitation) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputTextAnnotationURLCitation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A citation for a container file used to generate a model response.
type ResponseOutputTextAnnotationContainerFileCitation struct {
	// The ID of the container file.
	ContainerID string `json:"container_id,required"`
	// The index of the last character of the container file citation in the message.
	EndIndex int64 `json:"end_index,required"`
	// The ID of the file.
	FileID string `json:"file_id,required"`
	// The filename of the container file cited.
	Filename string `json:"filename,required"`
	// The index of the first character of the container file citation in the message.
	StartIndex int64 `json:"start_index,required"`
	// The type of the container file citation. Always `container_file_citation`.
	Type constant.ContainerFileCitation `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ContainerID respjson.Field
		EndIndex    respjson.Field
		FileID      respjson.Field
		Filename    respjson.Field
		StartIndex  respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputTextAnnotationContainerFileCitation) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputTextAnnotationContainerFileCitation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A path to a file.
type ResponseOutputTextAnnotationFilePath struct {
	// The ID of the file.
	FileID string `json:"file_id,required"`
	// The index of the file in the list of files.
	Index int64 `json:"index,required"`
	// The type of the file path. Always `file_path`.
	Type constant.FilePath `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		FileID      respjson.Field
		Index       respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputTextAnnotationFilePath) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputTextAnnotationFilePath) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The log probability of a token.
type ResponseOutputTextLogprob struct {
	Token       string                                `json:"token,required"`
	Bytes       []int64                               `json:"bytes,required"`
	Logprob     float64                               `json:"logprob,required"`
	TopLogprobs []ResponseOutputTextLogprobTopLogprob `json:"top_logprobs,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Token       respjson.Field
		Bytes       respjson.Field
		Logprob     respjson.Field
		TopLogprobs respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputTextLogprob) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputTextLogprob) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The top log probability of a token.
type ResponseOutputTextLogprobTopLogprob struct {
	Token   string  `json:"token,required"`
	Bytes   []int64 `json:"bytes,required"`
	Logprob float64 `json:"logprob,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Token       respjson.Field
		Bytes       respjson.Field
		Logprob     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputTextLogprobTopLogprob) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputTextLogprobTopLogprob) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A text output from the model.
//
// The properties Annotations, Text, Type are required.
type ResponseOutputTextParam struct {
	// The annotations of the text output.
	Annotations []ResponseOutputTextAnnotationUnionParam `json:"annotations,omitzero,required"`
	// The text output from the model.
	Text     string                           `json:"text,required"`
	Logprobs []ResponseOutputTextLogprobParam `json:"logprobs,omitzero"`
	// The type of the output text. Always `output_text`.
	//
	// This field can be elided, and will marshal its zero value as "output_text".
	Type constant.OutputText `json:"type,required"`
	paramObj
}

func (r ResponseOutputTextParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseOutputTextParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseOutputTextParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ResponseOutputTextAnnotationUnionParam struct {
	OfFileCitation          *ResponseOutputTextAnnotationFileCitationParam          `json:",omitzero,inline"`
	OfURLCitation           *ResponseOutputTextAnnotationURLCitationParam           `json:",omitzero,inline"`
	OfContainerFileCitation *ResponseOutputTextAnnotationContainerFileCitationParam `json:",omitzero,inline"`
	OfFilePath              *ResponseOutputTextAnnotationFilePathParam              `json:",omitzero,inline"`
	paramUnion
}

func (u ResponseOutputTextAnnotationUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfFileCitation, u.OfURLCitation, u.OfContainerFileCitation, u.OfFilePath)
}
func (u *ResponseOutputTextAnnotationUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ResponseOutputTextAnnotationUnionParam) asAny() any {
	if !param.IsOmitted(u.OfFileCitation) {
		return u.OfFileCitation
	} else if !param.IsOmitted(u.OfURLCitation) {
		return u.OfURLCitation
	} else if !param.IsOmitted(u.OfContainerFileCitation) {
		return u.OfContainerFileCitation
	} else if !param.IsOmitted(u.OfFilePath) {
		return u.OfFilePath
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseOutputTextAnnotationUnionParam) GetTitle() *string {
	if vt := u.OfURLCitation; vt != nil {
		return &vt.Title
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseOutputTextAnnotationUnionParam) GetURL() *string {
	if vt := u.OfURLCitation; vt != nil {
		return &vt.URL
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseOutputTextAnnotationUnionParam) GetContainerID() *string {
	if vt := u.OfContainerFileCitation; vt != nil {
		return &vt.ContainerID
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseOutputTextAnnotationUnionParam) GetFileID() *string {
	if vt := u.OfFileCitation; vt != nil {
		return (*string)(&vt.FileID)
	} else if vt := u.OfContainerFileCitation; vt != nil {
		return (*string)(&vt.FileID)
	} else if vt := u.OfFilePath; vt != nil {
		return (*string)(&vt.FileID)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseOutputTextAnnotationUnionParam) GetFilename() *string {
	if vt := u.OfFileCitation; vt != nil {
		return (*string)(&vt.Filename)
	} else if vt := u.OfContainerFileCitation; vt != nil {
		return (*string)(&vt.Filename)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseOutputTextAnnotationUnionParam) GetIndex() *int64 {
	if vt := u.OfFileCitation; vt != nil {
		return (*int64)(&vt.Index)
	} else if vt := u.OfFilePath; vt != nil {
		return (*int64)(&vt.Index)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseOutputTextAnnotationUnionParam) GetType() *string {
	if vt := u.OfFileCitation; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfURLCitation; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfContainerFileCitation; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfFilePath; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseOutputTextAnnotationUnionParam) GetEndIndex() *int64 {
	if vt := u.OfURLCitation; vt != nil {
		return (*int64)(&vt.EndIndex)
	} else if vt := u.OfContainerFileCitation; vt != nil {
		return (*int64)(&vt.EndIndex)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseOutputTextAnnotationUnionParam) GetStartIndex() *int64 {
	if vt := u.OfURLCitation; vt != nil {
		return (*int64)(&vt.StartIndex)
	} else if vt := u.OfContainerFileCitation; vt != nil {
		return (*int64)(&vt.StartIndex)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[ResponseOutputTextAnnotationUnionParam](
		"type",
		apijson.Discriminator[ResponseOutputTextAnnotationFileCitationParam]("file_citation"),
		apijson.Discriminator[ResponseOutputTextAnnotationURLCitationParam]("url_citation"),
		apijson.Discriminator[ResponseOutputTextAnnotationContainerFileCitationParam]("container_file_citation"),
		apijson.Discriminator[ResponseOutputTextAnnotationFilePathParam]("file_path"),
	)
}

// A citation to a file.
//
// The properties FileID, Filename, Index, Type are required.
type ResponseOutputTextAnnotationFileCitationParam struct {
	// The ID of the file.
	FileID string `json:"file_id,required"`
	// The filename of the file cited.
	Filename string `json:"filename,required"`
	// The index of the file in the list of files.
	Index int64 `json:"index,required"`
	// The type of the file citation. Always `file_citation`.
	//
	// This field can be elided, and will marshal its zero value as "file_citation".
	Type constant.FileCitation `json:"type,required"`
	paramObj
}

func (r ResponseOutputTextAnnotationFileCitationParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseOutputTextAnnotationFileCitationParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseOutputTextAnnotationFileCitationParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A citation for a web resource used to generate a model response.
//
// The properties EndIndex, StartIndex, Title, Type, URL are required.
type ResponseOutputTextAnnotationURLCitationParam struct {
	// The index of the last character of the URL citation in the message.
	EndIndex int64 `json:"end_index,required"`
	// The index of the first character of the URL citation in the message.
	StartIndex int64 `json:"start_index,required"`
	// The title of the web resource.
	Title string `json:"title,required"`
	// The URL of the web resource.
	URL string `json:"url,required"`
	// The type of the URL citation. Always `url_citation`.
	//
	// This field can be elided, and will marshal its zero value as "url_citation".
	Type constant.URLCitation `json:"type,required"`
	paramObj
}

func (r ResponseOutputTextAnnotationURLCitationParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseOutputTextAnnotationURLCitationParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseOutputTextAnnotationURLCitationParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A citation for a container file used to generate a model response.
//
// The properties ContainerID, EndIndex, FileID, Filename, StartIndex, Type are
// required.
type ResponseOutputTextAnnotationContainerFileCitationParam struct {
	// The ID of the container file.
	ContainerID string `json:"container_id,required"`
	// The index of the last character of the container file citation in the message.
	EndIndex int64 `json:"end_index,required"`
	// The ID of the file.
	FileID string `json:"file_id,required"`
	// The filename of the container file cited.
	Filename string `json:"filename,required"`
	// The index of the first character of the container file citation in the message.
	StartIndex int64 `json:"start_index,required"`
	// The type of the container file citation. Always `container_file_citation`.
	//
	// This field can be elided, and will marshal its zero value as
	// "container_file_citation".
	Type constant.ContainerFileCitation `json:"type,required"`
	paramObj
}

func (r ResponseOutputTextAnnotationContainerFileCitationParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseOutputTextAnnotationContainerFileCitationParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseOutputTextAnnotationContainerFileCitationParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A path to a file.
//
// The properties FileID, Index, Type are required.
type ResponseOutputTextAnnotationFilePathParam struct {
	// The ID of the file.
	FileID string `json:"file_id,required"`
	// The index of the file in the list of files.
	Index int64 `json:"index,required"`
	// The type of the file path. Always `file_path`.
	//
	// This field can be elided, and will marshal its zero value as "file_path".
	Type constant.FilePath `json:"type,required"`
	paramObj
}

func (r ResponseOutputTextAnnotationFilePathParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseOutputTextAnnotationFilePathParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseOutputTextAnnotationFilePathParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The log probability of a token.
//
// The properties Token, Bytes, Logprob, TopLogprobs are required.
type ResponseOutputTextLogprobParam struct {
	Token       string                                     `json:"token,required"`
	Bytes       []int64                                    `json:"bytes,omitzero,required"`
	Logprob     float64                                    `json:"logprob,required"`
	TopLogprobs []ResponseOutputTextLogprobTopLogprobParam `json:"top_logprobs,omitzero,required"`
	paramObj
}

func (r ResponseOutputTextLogprobParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseOutputTextLogprobParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseOutputTextLogprobParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The top log probability of a token.
//
// The properties Token, Bytes, Logprob are required.
type ResponseOutputTextLogprobTopLogprobParam struct {
	Token   string  `json:"token,required"`
	Bytes   []int64 `json:"bytes,omitzero,required"`
	Logprob float64 `json:"logprob,required"`
	paramObj
}

func (r ResponseOutputTextLogprobTopLogprobParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseOutputTextLogprobTopLogprobParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseOutputTextLogprobTopLogprobParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when an annotation is added to output text content.
type ResponseOutputTextAnnotationAddedEvent struct {
	// The annotation object being added. (See annotation schema for details.)
	Annotation any `json:"annotation,required"`
	// The index of the annotation within the content part.
	AnnotationIndex int64 `json:"annotation_index,required"`
	// The index of the content part within the output item.
	ContentIndex int64 `json:"content_index,required"`
	// The unique identifier of the item to which the annotation is being added.
	ItemID string `json:"item_id,required"`
	// The index of the output item in the response's output array.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always 'response.output_text_annotation.added'.
	Type constant.ResponseOutputTextAnnotationAdded `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Annotation      respjson.Field
		AnnotationIndex respjson.Field
		ContentIndex    respjson.Field
		ItemID          respjson.Field
		OutputIndex     respjson.Field
		SequenceNumber  respjson.Field
		Type            respjson.Field
		ExtraFields     map[string]respjson.Field
		raw             string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputTextAnnotationAddedEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputTextAnnotationAddedEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Reference to a prompt template and its variables.
// [Learn more](https://platform.openai.com/docs/guides/text?api-mode=responses#reusable-prompts).
type ResponsePrompt struct {
	// The unique identifier of the prompt template to use.
	ID string `json:"id,required"`
	// Optional map of values to substitute in for variables in your prompt. The
	// substitution values can either be strings, or other Response input types like
	// images or files.
	Variables map[string]ResponsePromptVariableUnion `json:"variables,nullable"`
	// Optional version of the prompt template.
	Version string `json:"version,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Variables   respjson.Field
		Version     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponsePrompt) RawJSON() string { return r.JSON.raw }
func (r *ResponsePrompt) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponsePrompt to a ResponsePromptParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponsePromptParam.Overrides()
func (r ResponsePrompt) ToParam() ResponsePromptParam {
	return param.Override[ResponsePromptParam](json.RawMessage(r.RawJSON()))
}

// ResponsePromptVariableUnion contains all possible properties and values from
// [string], [ResponseInputText], [ResponseInputImage], [ResponseInputFile].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfString]
type ResponsePromptVariableUnion struct {
	// This field will be present if the value is a [string] instead of an object.
	OfString string `json:",inline"`
	// This field is from variant [ResponseInputText].
	Text string `json:"text"`
	Type string `json:"type"`
	// This field is from variant [ResponseInputImage].
	Detail ResponseInputImageDetail `json:"detail"`
	FileID string                   `json:"file_id"`
	// This field is from variant [ResponseInputImage].
	ImageURL string `json:"image_url"`
	// This field is from variant [ResponseInputFile].
	FileData string `json:"file_data"`
	// This field is from variant [ResponseInputFile].
	Filename string `json:"filename"`
	JSON     struct {
		OfString respjson.Field
		Text     respjson.Field
		Type     respjson.Field
		Detail   respjson.Field
		FileID   respjson.Field
		ImageURL respjson.Field
		FileData respjson.Field
		Filename respjson.Field
		raw      string
	} `json:"-"`
}

func (u ResponsePromptVariableUnion) AsString() (v string) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponsePromptVariableUnion) AsInputText() (v ResponseInputText) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponsePromptVariableUnion) AsInputImage() (v ResponseInputImage) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponsePromptVariableUnion) AsInputFile() (v ResponseInputFile) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ResponsePromptVariableUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponsePromptVariableUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Reference to a prompt template and its variables.
// [Learn more](https://platform.openai.com/docs/guides/text?api-mode=responses#reusable-prompts).
//
// The property ID is required.
type ResponsePromptParam struct {
	// The unique identifier of the prompt template to use.
	ID string `json:"id,required"`
	// Optional version of the prompt template.
	Version param.Opt[string] `json:"version,omitzero"`
	// Optional map of values to substitute in for variables in your prompt. The
	// substitution values can either be strings, or other Response input types like
	// images or files.
	Variables map[string]ResponsePromptVariableUnionParam `json:"variables,omitzero"`
	paramObj
}

func (r ResponsePromptParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponsePromptParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponsePromptParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ResponsePromptVariableUnionParam struct {
	OfString     param.Opt[string]        `json:",omitzero,inline"`
	OfInputText  *ResponseInputTextParam  `json:",omitzero,inline"`
	OfInputImage *ResponseInputImageParam `json:",omitzero,inline"`
	OfInputFile  *ResponseInputFileParam  `json:",omitzero,inline"`
	paramUnion
}

func (u ResponsePromptVariableUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfInputText, u.OfInputImage, u.OfInputFile)
}
func (u *ResponsePromptVariableUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ResponsePromptVariableUnionParam) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfInputText) {
		return u.OfInputText
	} else if !param.IsOmitted(u.OfInputImage) {
		return u.OfInputImage
	} else if !param.IsOmitted(u.OfInputFile) {
		return u.OfInputFile
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponsePromptVariableUnionParam) GetText() *string {
	if vt := u.OfInputText; vt != nil {
		return &vt.Text
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponsePromptVariableUnionParam) GetDetail() *string {
	if vt := u.OfInputImage; vt != nil {
		return (*string)(&vt.Detail)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponsePromptVariableUnionParam) GetImageURL() *string {
	if vt := u.OfInputImage; vt != nil && vt.ImageURL.Valid() {
		return &vt.ImageURL.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponsePromptVariableUnionParam) GetFileData() *string {
	if vt := u.OfInputFile; vt != nil && vt.FileData.Valid() {
		return &vt.FileData.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponsePromptVariableUnionParam) GetFilename() *string {
	if vt := u.OfInputFile; vt != nil && vt.Filename.Valid() {
		return &vt.Filename.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponsePromptVariableUnionParam) GetType() *string {
	if vt := u.OfInputText; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfInputImage; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfInputFile; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponsePromptVariableUnionParam) GetFileID() *string {
	if vt := u.OfInputImage; vt != nil && vt.FileID.Valid() {
		return &vt.FileID.Value
	} else if vt := u.OfInputFile; vt != nil && vt.FileID.Valid() {
		return &vt.FileID.Value
	}
	return nil
}

// Emitted when a response is queued and waiting to be processed.
type ResponseQueuedEvent struct {
	// The full response object that is queued.
	Response Response `json:"response,required"`
	// The sequence number for this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always 'response.queued'.
	Type constant.ResponseQueued `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Response       respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseQueuedEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseQueuedEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when there is a delta (partial update) to the reasoning content.
type ResponseReasoningDeltaEvent struct {
	// The index of the reasoning content part within the output item.
	ContentIndex int64 `json:"content_index,required"`
	// The partial update to the reasoning content.
	Delta any `json:"delta,required"`
	// The unique identifier of the item for which reasoning is being updated.
	ItemID string `json:"item_id,required"`
	// The index of the output item in the response's output array.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always 'response.reasoning.delta'.
	Type constant.ResponseReasoningDelta `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ContentIndex   respjson.Field
		Delta          respjson.Field
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseReasoningDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseReasoningDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when the reasoning content is finalized for an item.
type ResponseReasoningDoneEvent struct {
	// The index of the reasoning content part within the output item.
	ContentIndex int64 `json:"content_index,required"`
	// The unique identifier of the item for which reasoning is finalized.
	ItemID string `json:"item_id,required"`
	// The index of the output item in the response's output array.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The finalized reasoning text.
	Text string `json:"text,required"`
	// The type of the event. Always 'response.reasoning.done'.
	Type constant.ResponseReasoningDone `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ContentIndex   respjson.Field
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Text           respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseReasoningDoneEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseReasoningDoneEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A description of the chain of thought used by a reasoning model while generating
// a response. Be sure to include these items in your `input` to the Responses API
// for subsequent turns of a conversation if you are manually
// [managing context](https://platform.openai.com/docs/guides/conversation-state).
type ResponseReasoningItem struct {
	// The unique identifier of the reasoning content.
	ID string `json:"id,required"`
	// Reasoning text contents.
	Summary []ResponseReasoningItemSummary `json:"summary,required"`
	// The type of the object. Always `reasoning`.
	Type constant.Reasoning `json:"type,required"`
	// The encrypted content of the reasoning item - populated when a response is
	// generated with `reasoning.encrypted_content` in the `include` parameter.
	EncryptedContent string `json:"encrypted_content,nullable"`
	// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
	// Populated when items are returned via API.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status ResponseReasoningItemStatus `json:"status"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID               respjson.Field
		Summary          respjson.Field
		Type             respjson.Field
		EncryptedContent respjson.Field
		Status           respjson.Field
		ExtraFields      map[string]respjson.Field
		raw              string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseReasoningItem) RawJSON() string { return r.JSON.raw }
func (r *ResponseReasoningItem) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponseReasoningItem to a ResponseReasoningItemParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseReasoningItemParam.Overrides()
func (r ResponseReasoningItem) ToParam() ResponseReasoningItemParam {
	return param.Override[ResponseReasoningItemParam](json.RawMessage(r.RawJSON()))
}

type ResponseReasoningItemSummary struct {
	// A short summary of the reasoning used by the model when generating the response.
	Text string `json:"text,required"`
	// The type of the object. Always `summary_text`.
	Type constant.SummaryText `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Text        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseReasoningItemSummary) RawJSON() string { return r.JSON.raw }
func (r *ResponseReasoningItemSummary) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
// Populated when items are returned via API.
type ResponseReasoningItemStatus string

const (
	ResponseReasoningItemStatusInProgress ResponseReasoningItemStatus = "in_progress"
	ResponseReasoningItemStatusCompleted  ResponseReasoningItemStatus = "completed"
	ResponseReasoningItemStatusIncomplete ResponseReasoningItemStatus = "incomplete"
)

// A description of the chain of thought used by a reasoning model while generating
// a response. Be sure to include these items in your `input` to the Responses API
// for subsequent turns of a conversation if you are manually
// [managing context](https://platform.openai.com/docs/guides/conversation-state).
//
// The properties ID, Summary, Type are required.
type ResponseReasoningItemParam struct {
	// The unique identifier of the reasoning content.
	ID string `json:"id,required"`
	// Reasoning text contents.
	Summary []ResponseReasoningItemSummaryParam `json:"summary,omitzero,required"`
	// The encrypted content of the reasoning item - populated when a response is
	// generated with `reasoning.encrypted_content` in the `include` parameter.
	EncryptedContent param.Opt[string] `json:"encrypted_content,omitzero"`
	// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
	// Populated when items are returned via API.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status ResponseReasoningItemStatus `json:"status,omitzero"`
	// The type of the object. Always `reasoning`.
	//
	// This field can be elided, and will marshal its zero value as "reasoning".
	Type constant.Reasoning `json:"type,required"`
	paramObj
}

func (r ResponseReasoningItemParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseReasoningItemParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseReasoningItemParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Text, Type are required.
type ResponseReasoningItemSummaryParam struct {
	// A short summary of the reasoning used by the model when generating the response.
	Text string `json:"text,required"`
	// The type of the object. Always `summary_text`.
	//
	// This field can be elided, and will marshal its zero value as "summary_text".
	Type constant.SummaryText `json:"type,required"`
	paramObj
}

func (r ResponseReasoningItemSummaryParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseReasoningItemSummaryParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseReasoningItemSummaryParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when there is a delta (partial update) to the reasoning summary content.
type ResponseReasoningSummaryDeltaEvent struct {
	// The partial update to the reasoning summary content.
	Delta any `json:"delta,required"`
	// The unique identifier of the item for which the reasoning summary is being
	// updated.
	ItemID string `json:"item_id,required"`
	// The index of the output item in the response's output array.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The index of the summary part within the output item.
	SummaryIndex int64 `json:"summary_index,required"`
	// The type of the event. Always 'response.reasoning_summary.delta'.
	Type constant.ResponseReasoningSummaryDelta `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Delta          respjson.Field
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		SummaryIndex   respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseReasoningSummaryDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseReasoningSummaryDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when the reasoning summary content is finalized for an item.
type ResponseReasoningSummaryDoneEvent struct {
	// The unique identifier of the item for which the reasoning summary is finalized.
	ItemID string `json:"item_id,required"`
	// The index of the output item in the response's output array.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The index of the summary part within the output item.
	SummaryIndex int64 `json:"summary_index,required"`
	// The finalized reasoning summary text.
	Text string `json:"text,required"`
	// The type of the event. Always 'response.reasoning_summary.done'.
	Type constant.ResponseReasoningSummaryDone `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		SummaryIndex   respjson.Field
		Text           respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseReasoningSummaryDoneEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseReasoningSummaryDoneEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a new reasoning summary part is added.
type ResponseReasoningSummaryPartAddedEvent struct {
	// The ID of the item this summary part is associated with.
	ItemID string `json:"item_id,required"`
	// The index of the output item this summary part is associated with.
	OutputIndex int64 `json:"output_index,required"`
	// The summary part that was added.
	Part ResponseReasoningSummaryPartAddedEventPart `json:"part,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The index of the summary part within the reasoning summary.
	SummaryIndex int64 `json:"summary_index,required"`
	// The type of the event. Always `response.reasoning_summary_part.added`.
	Type constant.ResponseReasoningSummaryPartAdded `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		Part           respjson.Field
		SequenceNumber respjson.Field
		SummaryIndex   respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseReasoningSummaryPartAddedEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseReasoningSummaryPartAddedEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The summary part that was added.
type ResponseReasoningSummaryPartAddedEventPart struct {
	// The text of the summary part.
	Text string `json:"text,required"`
	// The type of the summary part. Always `summary_text`.
	Type constant.SummaryText `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Text        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseReasoningSummaryPartAddedEventPart) RawJSON() string { return r.JSON.raw }
func (r *ResponseReasoningSummaryPartAddedEventPart) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a reasoning summary part is completed.
type ResponseReasoningSummaryPartDoneEvent struct {
	// The ID of the item this summary part is associated with.
	ItemID string `json:"item_id,required"`
	// The index of the output item this summary part is associated with.
	OutputIndex int64 `json:"output_index,required"`
	// The completed summary part.
	Part ResponseReasoningSummaryPartDoneEventPart `json:"part,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The index of the summary part within the reasoning summary.
	SummaryIndex int64 `json:"summary_index,required"`
	// The type of the event. Always `response.reasoning_summary_part.done`.
	Type constant.ResponseReasoningSummaryPartDone `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		Part           respjson.Field
		SequenceNumber respjson.Field
		SummaryIndex   respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseReasoningSummaryPartDoneEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseReasoningSummaryPartDoneEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The completed summary part.
type ResponseReasoningSummaryPartDoneEventPart struct {
	// The text of the summary part.
	Text string `json:"text,required"`
	// The type of the summary part. Always `summary_text`.
	Type constant.SummaryText `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Text        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseReasoningSummaryPartDoneEventPart) RawJSON() string { return r.JSON.raw }
func (r *ResponseReasoningSummaryPartDoneEventPart) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a delta is added to a reasoning summary text.
type ResponseReasoningSummaryTextDeltaEvent struct {
	// The text delta that was added to the summary.
	Delta string `json:"delta,required"`
	// The ID of the item this summary text delta is associated with.
	ItemID string `json:"item_id,required"`
	// The index of the output item this summary text delta is associated with.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The index of the summary part within the reasoning summary.
	SummaryIndex int64 `json:"summary_index,required"`
	// The type of the event. Always `response.reasoning_summary_text.delta`.
	Type constant.ResponseReasoningSummaryTextDelta `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Delta          respjson.Field
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		SummaryIndex   respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseReasoningSummaryTextDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseReasoningSummaryTextDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a reasoning summary text is completed.
type ResponseReasoningSummaryTextDoneEvent struct {
	// The ID of the item this summary text is associated with.
	ItemID string `json:"item_id,required"`
	// The index of the output item this summary text is associated with.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The index of the summary part within the reasoning summary.
	SummaryIndex int64 `json:"summary_index,required"`
	// The full text of the completed reasoning summary.
	Text string `json:"text,required"`
	// The type of the event. Always `response.reasoning_summary_text.done`.
	Type constant.ResponseReasoningSummaryTextDone `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		SummaryIndex   respjson.Field
		Text           respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseReasoningSummaryTextDoneEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseReasoningSummaryTextDoneEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when there is a partial refusal text.
type ResponseRefusalDeltaEvent struct {
	// The index of the content part that the refusal text is added to.
	ContentIndex int64 `json:"content_index,required"`
	// The refusal text that is added.
	Delta string `json:"delta,required"`
	// The ID of the output item that the refusal text is added to.
	ItemID string `json:"item_id,required"`
	// The index of the output item that the refusal text is added to.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.refusal.delta`.
	Type constant.ResponseRefusalDelta `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ContentIndex   respjson.Field
		Delta          respjson.Field
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseRefusalDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseRefusalDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when refusal text is finalized.
type ResponseRefusalDoneEvent struct {
	// The index of the content part that the refusal text is finalized.
	ContentIndex int64 `json:"content_index,required"`
	// The ID of the output item that the refusal text is finalized.
	ItemID string `json:"item_id,required"`
	// The index of the output item that the refusal text is finalized.
	OutputIndex int64 `json:"output_index,required"`
	// The refusal text that is finalized.
	Refusal string `json:"refusal,required"`
	// The sequence number of this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.refusal.done`.
	Type constant.ResponseRefusalDone `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ContentIndex   respjson.Field
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		Refusal        respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseRefusalDoneEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseRefusalDoneEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The status of the response generation. One of `completed`, `failed`,
// `in_progress`, `cancelled`, `queued`, or `incomplete`.
type ResponseStatus string

const (
	ResponseStatusCompleted  ResponseStatus = "completed"
	ResponseStatusFailed     ResponseStatus = "failed"
	ResponseStatusInProgress ResponseStatus = "in_progress"
	ResponseStatusCancelled  ResponseStatus = "cancelled"
	ResponseStatusQueued     ResponseStatus = "queued"
	ResponseStatusIncomplete ResponseStatus = "incomplete"
)

// ResponseStreamEventUnion contains all possible properties and values from
// [ResponseAudioDeltaEvent], [ResponseAudioDoneEvent],
// [ResponseAudioTranscriptDeltaEvent], [ResponseAudioTranscriptDoneEvent],
// [ResponseCodeInterpreterCallCodeDeltaEvent],
// [ResponseCodeInterpreterCallCodeDoneEvent],
// [ResponseCodeInterpreterCallCompletedEvent],
// [ResponseCodeInterpreterCallInProgressEvent],
// [ResponseCodeInterpreterCallInterpretingEvent], [ResponseCompletedEvent],
// [ResponseContentPartAddedEvent], [ResponseContentPartDoneEvent],
// [ResponseCreatedEvent], [ResponseErrorEvent],
// [ResponseFileSearchCallCompletedEvent], [ResponseFileSearchCallInProgressEvent],
// [ResponseFileSearchCallSearchingEvent],
// [ResponseFunctionCallArgumentsDeltaEvent],
// [ResponseFunctionCallArgumentsDoneEvent], [ResponseInProgressEvent],
// [ResponseFailedEvent], [ResponseIncompleteEvent],
// [ResponseOutputItemAddedEvent], [ResponseOutputItemDoneEvent],
// [ResponseReasoningSummaryPartAddedEvent],
// [ResponseReasoningSummaryPartDoneEvent],
// [ResponseReasoningSummaryTextDeltaEvent],
// [ResponseReasoningSummaryTextDoneEvent], [ResponseRefusalDeltaEvent],
// [ResponseRefusalDoneEvent], [ResponseTextDeltaEvent], [ResponseTextDoneEvent],
// [ResponseWebSearchCallCompletedEvent], [ResponseWebSearchCallInProgressEvent],
// [ResponseWebSearchCallSearchingEvent], [ResponseImageGenCallCompletedEvent],
// [ResponseImageGenCallGeneratingEvent], [ResponseImageGenCallInProgressEvent],
// [ResponseImageGenCallPartialImageEvent], [ResponseMcpCallArgumentsDeltaEvent],
// [ResponseMcpCallArgumentsDoneEvent], [ResponseMcpCallCompletedEvent],
// [ResponseMcpCallFailedEvent], [ResponseMcpCallInProgressEvent],
// [ResponseMcpListToolsCompletedEvent], [ResponseMcpListToolsFailedEvent],
// [ResponseMcpListToolsInProgressEvent], [ResponseOutputTextAnnotationAddedEvent],
// [ResponseQueuedEvent], [ResponseReasoningDeltaEvent],
// [ResponseReasoningDoneEvent], [ResponseReasoningSummaryDeltaEvent],
// [ResponseReasoningSummaryDoneEvent].
//
// Use the [ResponseStreamEventUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ResponseStreamEventUnion struct {
	// This field is a union of [string], [string], [string], [string], [string],
	// [string], [string], [any], [any], [any]
	Delta          ResponseStreamEventUnionDelta `json:"delta"`
	SequenceNumber int64                         `json:"sequence_number"`
	// Any of "response.audio.delta", "response.audio.done",
	// "response.audio.transcript.delta", "response.audio.transcript.done",
	// "response.code_interpreter_call_code.delta",
	// "response.code_interpreter_call_code.done",
	// "response.code_interpreter_call.completed",
	// "response.code_interpreter_call.in_progress",
	// "response.code_interpreter_call.interpreting", "response.completed",
	// "response.content_part.added", "response.content_part.done", "response.created",
	// "error", "response.file_search_call.completed",
	// "response.file_search_call.in_progress", "response.file_search_call.searching",
	// "response.function_call_arguments.delta",
	// "response.function_call_arguments.done", "response.in_progress",
	// "response.failed", "response.incomplete", "response.output_item.added",
	// "response.output_item.done", "response.reasoning_summary_part.added",
	// "response.reasoning_summary_part.done", "response.reasoning_summary_text.delta",
	// "response.reasoning_summary_text.done", "response.refusal.delta",
	// "response.refusal.done", "response.output_text.delta",
	// "response.output_text.done", "response.web_search_call.completed",
	// "response.web_search_call.in_progress", "response.web_search_call.searching",
	// "response.image_generation_call.completed",
	// "response.image_generation_call.generating",
	// "response.image_generation_call.in_progress",
	// "response.image_generation_call.partial_image",
	// "response.mcp_call.arguments_delta", "response.mcp_call.arguments_done",
	// "response.mcp_call.completed", "response.mcp_call.failed",
	// "response.mcp_call.in_progress", "response.mcp_list_tools.completed",
	// "response.mcp_list_tools.failed", "response.mcp_list_tools.in_progress",
	// "response.output_text_annotation.added", "response.queued",
	// "response.reasoning.delta", "response.reasoning.done",
	// "response.reasoning_summary.delta", "response.reasoning_summary.done".
	Type        string `json:"type"`
	ItemID      string `json:"item_id"`
	OutputIndex int64  `json:"output_index"`
	Code        string `json:"code"`
	// This field is from variant [ResponseCompletedEvent].
	Response     Response `json:"response"`
	ContentIndex int64    `json:"content_index"`
	// This field is a union of [ResponseContentPartAddedEventPartUnion],
	// [ResponseContentPartDoneEventPartUnion],
	// [ResponseReasoningSummaryPartAddedEventPart],
	// [ResponseReasoningSummaryPartDoneEventPart]
	Part ResponseStreamEventUnionPart `json:"part"`
	// This field is from variant [ResponseErrorEvent].
	Message string `json:"message"`
	// This field is from variant [ResponseErrorEvent].
	Param string `json:"param"`
	// This field is a union of [string], [any]
	Arguments ResponseStreamEventUnionArguments `json:"arguments"`
	// This field is from variant [ResponseOutputItemAddedEvent].
	Item         ResponseOutputItemUnion `json:"item"`
	SummaryIndex int64                   `json:"summary_index"`
	Text         string                  `json:"text"`
	// This field is from variant [ResponseRefusalDoneEvent].
	Refusal string `json:"refusal"`
	// This field is from variant [ResponseImageGenCallPartialImageEvent].
	PartialImageB64 string `json:"partial_image_b64"`
	// This field is from variant [ResponseImageGenCallPartialImageEvent].
	PartialImageIndex int64 `json:"partial_image_index"`
	// This field is from variant [ResponseOutputTextAnnotationAddedEvent].
	Annotation any `json:"annotation"`
	// This field is from variant [ResponseOutputTextAnnotationAddedEvent].
	AnnotationIndex int64 `json:"annotation_index"`
	JSON            struct {
		Delta             respjson.Field
		SequenceNumber    respjson.Field
		Type              respjson.Field
		ItemID            respjson.Field
		OutputIndex       respjson.Field
		Code              respjson.Field
		Response          respjson.Field
		ContentIndex      respjson.Field
		Part              respjson.Field
		Message           respjson.Field
		Param             respjson.Field
		Arguments         respjson.Field
		Item              respjson.Field
		SummaryIndex      respjson.Field
		Text              respjson.Field
		Refusal           respjson.Field
		PartialImageB64   respjson.Field
		PartialImageIndex respjson.Field
		Annotation        respjson.Field
		AnnotationIndex   respjson.Field
		raw               string
	} `json:"-"`
}

// anyResponseStreamEvent is implemented by each variant of
// [ResponseStreamEventUnion] to add type safety for the return type of
// [ResponseStreamEventUnion.AsAny]
type anyResponseStreamEvent interface {
	implResponseStreamEventUnion()
}

func (ResponseAudioDeltaEvent) implResponseStreamEventUnion()                      {}
func (ResponseAudioDoneEvent) implResponseStreamEventUnion()                       {}
func (ResponseAudioTranscriptDeltaEvent) implResponseStreamEventUnion()            {}
func (ResponseAudioTranscriptDoneEvent) implResponseStreamEventUnion()             {}
func (ResponseCodeInterpreterCallCodeDeltaEvent) implResponseStreamEventUnion()    {}
func (ResponseCodeInterpreterCallCodeDoneEvent) implResponseStreamEventUnion()     {}
func (ResponseCodeInterpreterCallCompletedEvent) implResponseStreamEventUnion()    {}
func (ResponseCodeInterpreterCallInProgressEvent) implResponseStreamEventUnion()   {}
func (ResponseCodeInterpreterCallInterpretingEvent) implResponseStreamEventUnion() {}
func (ResponseCompletedEvent) implResponseStreamEventUnion()                       {}
func (ResponseContentPartAddedEvent) implResponseStreamEventUnion()                {}
func (ResponseContentPartDoneEvent) implResponseStreamEventUnion()                 {}
func (ResponseCreatedEvent) implResponseStreamEventUnion()                         {}
func (ResponseErrorEvent) implResponseStreamEventUnion()                           {}
func (ResponseFileSearchCallCompletedEvent) implResponseStreamEventUnion()         {}
func (ResponseFileSearchCallInProgressEvent) implResponseStreamEventUnion()        {}
func (ResponseFileSearchCallSearchingEvent) implResponseStreamEventUnion()         {}
func (ResponseFunctionCallArgumentsDeltaEvent) implResponseStreamEventUnion()      {}
func (ResponseFunctionCallArgumentsDoneEvent) implResponseStreamEventUnion()       {}
func (ResponseInProgressEvent) implResponseStreamEventUnion()                      {}
func (ResponseFailedEvent) implResponseStreamEventUnion()                          {}
func (ResponseIncompleteEvent) implResponseStreamEventUnion()                      {}
func (ResponseOutputItemAddedEvent) implResponseStreamEventUnion()                 {}
func (ResponseOutputItemDoneEvent) implResponseStreamEventUnion()                  {}
func (ResponseReasoningSummaryPartAddedEvent) implResponseStreamEventUnion()       {}
func (ResponseReasoningSummaryPartDoneEvent) implResponseStreamEventUnion()        {}
func (ResponseReasoningSummaryTextDeltaEvent) implResponseStreamEventUnion()       {}
func (ResponseReasoningSummaryTextDoneEvent) implResponseStreamEventUnion()        {}
func (ResponseRefusalDeltaEvent) implResponseStreamEventUnion()                    {}
func (ResponseRefusalDoneEvent) implResponseStreamEventUnion()                     {}
func (ResponseTextDeltaEvent) implResponseStreamEventUnion()                       {}
func (ResponseTextDoneEvent) implResponseStreamEventUnion()                        {}
func (ResponseWebSearchCallCompletedEvent) implResponseStreamEventUnion()          {}
func (ResponseWebSearchCallInProgressEvent) implResponseStreamEventUnion()         {}
func (ResponseWebSearchCallSearchingEvent) implResponseStreamEventUnion()          {}
func (ResponseImageGenCallCompletedEvent) implResponseStreamEventUnion()           {}
func (ResponseImageGenCallGeneratingEvent) implResponseStreamEventUnion()          {}
func (ResponseImageGenCallInProgressEvent) implResponseStreamEventUnion()          {}
func (ResponseImageGenCallPartialImageEvent) implResponseStreamEventUnion()        {}
func (ResponseMcpCallArgumentsDeltaEvent) implResponseStreamEventUnion()           {}
func (ResponseMcpCallArgumentsDoneEvent) implResponseStreamEventUnion()            {}
func (ResponseMcpCallCompletedEvent) implResponseStreamEventUnion()                {}
func (ResponseMcpCallFailedEvent) implResponseStreamEventUnion()                   {}
func (ResponseMcpCallInProgressEvent) implResponseStreamEventUnion()               {}
func (ResponseMcpListToolsCompletedEvent) implResponseStreamEventUnion()           {}
func (ResponseMcpListToolsFailedEvent) implResponseStreamEventUnion()              {}
func (ResponseMcpListToolsInProgressEvent) implResponseStreamEventUnion()          {}
func (ResponseOutputTextAnnotationAddedEvent) implResponseStreamEventUnion()       {}
func (ResponseQueuedEvent) implResponseStreamEventUnion()                          {}
func (ResponseReasoningDeltaEvent) implResponseStreamEventUnion()                  {}
func (ResponseReasoningDoneEvent) implResponseStreamEventUnion()                   {}
func (ResponseReasoningSummaryDeltaEvent) implResponseStreamEventUnion()           {}
func (ResponseReasoningSummaryDoneEvent) implResponseStreamEventUnion()            {}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseStreamEventUnion.AsAny().(type) {
//	case responses.ResponseAudioDeltaEvent:
//	case responses.ResponseAudioDoneEvent:
//	case responses.ResponseAudioTranscriptDeltaEvent:
//	case responses.ResponseAudioTranscriptDoneEvent:
//	case responses.ResponseCodeInterpreterCallCodeDeltaEvent:
//	case responses.ResponseCodeInterpreterCallCodeDoneEvent:
//	case responses.ResponseCodeInterpreterCallCompletedEvent:
//	case responses.ResponseCodeInterpreterCallInProgressEvent:
//	case responses.ResponseCodeInterpreterCallInterpretingEvent:
//	case responses.ResponseCompletedEvent:
//	case responses.ResponseContentPartAddedEvent:
//	case responses.ResponseContentPartDoneEvent:
//	case responses.ResponseCreatedEvent:
//	case responses.ResponseErrorEvent:
//	case responses.ResponseFileSearchCallCompletedEvent:
//	case responses.ResponseFileSearchCallInProgressEvent:
//	case responses.ResponseFileSearchCallSearchingEvent:
//	case responses.ResponseFunctionCallArgumentsDeltaEvent:
//	case responses.ResponseFunctionCallArgumentsDoneEvent:
//	case responses.ResponseInProgressEvent:
//	case responses.ResponseFailedEvent:
//	case responses.ResponseIncompleteEvent:
//	case responses.ResponseOutputItemAddedEvent:
//	case responses.ResponseOutputItemDoneEvent:
//	case responses.ResponseReasoningSummaryPartAddedEvent:
//	case responses.ResponseReasoningSummaryPartDoneEvent:
//	case responses.ResponseReasoningSummaryTextDeltaEvent:
//	case responses.ResponseReasoningSummaryTextDoneEvent:
//	case responses.ResponseRefusalDeltaEvent:
//	case responses.ResponseRefusalDoneEvent:
//	case responses.ResponseTextDeltaEvent:
//	case responses.ResponseTextDoneEvent:
//	case responses.ResponseWebSearchCallCompletedEvent:
//	case responses.ResponseWebSearchCallInProgressEvent:
//	case responses.ResponseWebSearchCallSearchingEvent:
//	case responses.ResponseImageGenCallCompletedEvent:
//	case responses.ResponseImageGenCallGeneratingEvent:
//	case responses.ResponseImageGenCallInProgressEvent:
//	case responses.ResponseImageGenCallPartialImageEvent:
//	case responses.ResponseMcpCallArgumentsDeltaEvent:
//	case responses.ResponseMcpCallArgumentsDoneEvent:
//	case responses.ResponseMcpCallCompletedEvent:
//	case responses.ResponseMcpCallFailedEvent:
//	case responses.ResponseMcpCallInProgressEvent:
//	case responses.ResponseMcpListToolsCompletedEvent:
//	case responses.ResponseMcpListToolsFailedEvent:
//	case responses.ResponseMcpListToolsInProgressEvent:
//	case responses.ResponseOutputTextAnnotationAddedEvent:
//	case responses.ResponseQueuedEvent:
//	case responses.ResponseReasoningDeltaEvent:
//	case responses.ResponseReasoningDoneEvent:
//	case responses.ResponseReasoningSummaryDeltaEvent:
//	case responses.ResponseReasoningSummaryDoneEvent:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseStreamEventUnion) AsAny() anyResponseStreamEvent {
	switch u.Type {
	case "response.audio.delta":
		return u.AsResponseAudioDelta()
	case "response.audio.done":
		return u.AsResponseAudioDone()
	case "response.audio.transcript.delta":
		return u.AsResponseAudioTranscriptDelta()
	case "response.audio.transcript.done":
		return u.AsResponseAudioTranscriptDone()
	case "response.code_interpreter_call_code.delta":
		return u.AsResponseCodeInterpreterCallCodeDelta()
	case "response.code_interpreter_call_code.done":
		return u.AsResponseCodeInterpreterCallCodeDone()
	case "response.code_interpreter_call.completed":
		return u.AsResponseCodeInterpreterCallCompleted()
	case "response.code_interpreter_call.in_progress":
		return u.AsResponseCodeInterpreterCallInProgress()
	case "response.code_interpreter_call.interpreting":
		return u.AsResponseCodeInterpreterCallInterpreting()
	case "response.completed":
		return u.AsResponseCompleted()
	case "response.content_part.added":
		return u.AsResponseContentPartAdded()
	case "response.content_part.done":
		return u.AsResponseContentPartDone()
	case "response.created":
		return u.AsResponseCreated()
	case "error":
		return u.AsError()
	case "response.file_search_call.completed":
		return u.AsResponseFileSearchCallCompleted()
	case "response.file_search_call.in_progress":
		return u.AsResponseFileSearchCallInProgress()
	case "response.file_search_call.searching":
		return u.AsResponseFileSearchCallSearching()
	case "response.function_call_arguments.delta":
		return u.AsResponseFunctionCallArgumentsDelta()
	case "response.function_call_arguments.done":
		return u.AsResponseFunctionCallArgumentsDone()
	case "response.in_progress":
		return u.AsResponseInProgress()
	case "response.failed":
		return u.AsResponseFailed()
	case "response.incomplete":
		return u.AsResponseIncomplete()
	case "response.output_item.added":
		return u.AsResponseOutputItemAdded()
	case "response.output_item.done":
		return u.AsResponseOutputItemDone()
	case "response.reasoning_summary_part.added":
		return u.AsResponseReasoningSummaryPartAdded()
	case "response.reasoning_summary_part.done":
		return u.AsResponseReasoningSummaryPartDone()
	case "response.reasoning_summary_text.delta":
		return u.AsResponseReasoningSummaryTextDelta()
	case "response.reasoning_summary_text.done":
		return u.AsResponseReasoningSummaryTextDone()
	case "response.refusal.delta":
		return u.AsResponseRefusalDelta()
	case "response.refusal.done":
		return u.AsResponseRefusalDone()
	case "response.output_text.delta":
		return u.AsResponseOutputTextDelta()
	case "response.output_text.done":
		return u.AsResponseOutputTextDone()
	case "response.web_search_call.completed":
		return u.AsResponseWebSearchCallCompleted()
	case "response.web_search_call.in_progress":
		return u.AsResponseWebSearchCallInProgress()
	case "response.web_search_call.searching":
		return u.AsResponseWebSearchCallSearching()
	case "response.image_generation_call.completed":
		return u.AsResponseImageGenerationCallCompleted()
	case "response.image_generation_call.generating":
		return u.AsResponseImageGenerationCallGenerating()
	case "response.image_generation_call.in_progress":
		return u.AsResponseImageGenerationCallInProgress()
	case "response.image_generation_call.partial_image":
		return u.AsResponseImageGenerationCallPartialImage()
	case "response.mcp_call.arguments_delta":
		return u.AsResponseMcpCallArgumentsDelta()
	case "response.mcp_call.arguments_done":
		return u.AsResponseMcpCallArgumentsDone()
	case "response.mcp_call.completed":
		return u.AsResponseMcpCallCompleted()
	case "response.mcp_call.failed":
		return u.AsResponseMcpCallFailed()
	case "response.mcp_call.in_progress":
		return u.AsResponseMcpCallInProgress()
	case "response.mcp_list_tools.completed":
		return u.AsResponseMcpListToolsCompleted()
	case "response.mcp_list_tools.failed":
		return u.AsResponseMcpListToolsFailed()
	case "response.mcp_list_tools.in_progress":
		return u.AsResponseMcpListToolsInProgress()
	case "response.output_text_annotation.added":
		return u.AsResponseOutputTextAnnotationAdded()
	case "response.queued":
		return u.AsResponseQueued()
	case "response.reasoning.delta":
		return u.AsResponseReasoningDelta()
	case "response.reasoning.done":
		return u.AsResponseReasoningDone()
	case "response.reasoning_summary.delta":
		return u.AsResponseReasoningSummaryDelta()
	case "response.reasoning_summary.done":
		return u.AsResponseReasoningSummaryDone()
	}
	return nil
}

func (u ResponseStreamEventUnion) AsResponseAudioDelta() (v ResponseAudioDeltaEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseAudioDone() (v ResponseAudioDoneEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseAudioTranscriptDelta() (v ResponseAudioTranscriptDeltaEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseAudioTranscriptDone() (v ResponseAudioTranscriptDoneEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseCodeInterpreterCallCodeDelta() (v ResponseCodeInterpreterCallCodeDeltaEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseCodeInterpreterCallCodeDone() (v ResponseCodeInterpreterCallCodeDoneEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseCodeInterpreterCallCompleted() (v ResponseCodeInterpreterCallCompletedEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseCodeInterpreterCallInProgress() (v ResponseCodeInterpreterCallInProgressEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseCodeInterpreterCallInterpreting() (v ResponseCodeInterpreterCallInterpretingEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseCompleted() (v ResponseCompletedEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseContentPartAdded() (v ResponseContentPartAddedEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseContentPartDone() (v ResponseContentPartDoneEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseCreated() (v ResponseCreatedEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsError() (v ResponseErrorEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseFileSearchCallCompleted() (v ResponseFileSearchCallCompletedEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseFileSearchCallInProgress() (v ResponseFileSearchCallInProgressEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseFileSearchCallSearching() (v ResponseFileSearchCallSearchingEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseFunctionCallArgumentsDelta() (v ResponseFunctionCallArgumentsDeltaEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseFunctionCallArgumentsDone() (v ResponseFunctionCallArgumentsDoneEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseInProgress() (v ResponseInProgressEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseFailed() (v ResponseFailedEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseIncomplete() (v ResponseIncompleteEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseOutputItemAdded() (v ResponseOutputItemAddedEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseOutputItemDone() (v ResponseOutputItemDoneEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseReasoningSummaryPartAdded() (v ResponseReasoningSummaryPartAddedEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseReasoningSummaryPartDone() (v ResponseReasoningSummaryPartDoneEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseReasoningSummaryTextDelta() (v ResponseReasoningSummaryTextDeltaEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseReasoningSummaryTextDone() (v ResponseReasoningSummaryTextDoneEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseRefusalDelta() (v ResponseRefusalDeltaEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseRefusalDone() (v ResponseRefusalDoneEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseOutputTextDelta() (v ResponseTextDeltaEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseOutputTextDone() (v ResponseTextDoneEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseWebSearchCallCompleted() (v ResponseWebSearchCallCompletedEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseWebSearchCallInProgress() (v ResponseWebSearchCallInProgressEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseWebSearchCallSearching() (v ResponseWebSearchCallSearchingEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseImageGenerationCallCompleted() (v ResponseImageGenCallCompletedEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseImageGenerationCallGenerating() (v ResponseImageGenCallGeneratingEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseImageGenerationCallInProgress() (v ResponseImageGenCallInProgressEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseImageGenerationCallPartialImage() (v ResponseImageGenCallPartialImageEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseMcpCallArgumentsDelta() (v ResponseMcpCallArgumentsDeltaEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseMcpCallArgumentsDone() (v ResponseMcpCallArgumentsDoneEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseMcpCallCompleted() (v ResponseMcpCallCompletedEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseMcpCallFailed() (v ResponseMcpCallFailedEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseMcpCallInProgress() (v ResponseMcpCallInProgressEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseMcpListToolsCompleted() (v ResponseMcpListToolsCompletedEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseMcpListToolsFailed() (v ResponseMcpListToolsFailedEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseMcpListToolsInProgress() (v ResponseMcpListToolsInProgressEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseOutputTextAnnotationAdded() (v ResponseOutputTextAnnotationAddedEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseQueued() (v ResponseQueuedEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseReasoningDelta() (v ResponseReasoningDeltaEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseReasoningDone() (v ResponseReasoningDoneEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseReasoningSummaryDelta() (v ResponseReasoningSummaryDeltaEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseReasoningSummaryDone() (v ResponseReasoningSummaryDoneEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ResponseStreamEventUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseStreamEventUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseStreamEventUnionDelta is an implicit subunion of
// [ResponseStreamEventUnion]. ResponseStreamEventUnionDelta provides convenient
// access to the sub-properties of the union.
//
// For type safety it is recommended to directly use a variant of the
// [ResponseStreamEventUnion].
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfString OfResponseReasoningSummaryDeltaEventDelta]
type ResponseStreamEventUnionDelta struct {
	// This field will be present if the value is a [string] instead of an object.
	OfString string `json:",inline"`
	// This field will be present if the value is a [any] instead of an object.
	OfResponseReasoningSummaryDeltaEventDelta any `json:",inline"`
	JSON                                      struct {
		OfString                                  respjson.Field
		OfResponseReasoningSummaryDeltaEventDelta respjson.Field
		raw                                       string
	} `json:"-"`
}

func (r *ResponseStreamEventUnionDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseStreamEventUnionPart is an implicit subunion of
// [ResponseStreamEventUnion]. ResponseStreamEventUnionPart provides convenient
// access to the sub-properties of the union.
//
// For type safety it is recommended to directly use a variant of the
// [ResponseStreamEventUnion].
type ResponseStreamEventUnionPart struct {
	// This field is from variant [ResponseContentPartAddedEventPartUnion],
	// [ResponseContentPartDoneEventPartUnion].
	Annotations []ResponseOutputTextAnnotationUnion `json:"annotations"`
	Text        string                              `json:"text"`
	Type        string                              `json:"type"`
	// This field is from variant [ResponseContentPartAddedEventPartUnion],
	// [ResponseContentPartDoneEventPartUnion].
	Logprobs []ResponseOutputTextLogprob `json:"logprobs"`
	// This field is from variant [ResponseContentPartAddedEventPartUnion],
	// [ResponseContentPartDoneEventPartUnion].
	Refusal string `json:"refusal"`
	JSON    struct {
		Annotations respjson.Field
		Text        respjson.Field
		Type        respjson.Field
		Logprobs    respjson.Field
		Refusal     respjson.Field
		raw         string
	} `json:"-"`
}

func (r *ResponseStreamEventUnionPart) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseStreamEventUnionArguments is an implicit subunion of
// [ResponseStreamEventUnion]. ResponseStreamEventUnionArguments provides
// convenient access to the sub-properties of the union.
//
// For type safety it is recommended to directly use a variant of the
// [ResponseStreamEventUnion].
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfString OfResponseMcpCallArgumentsDoneEventArguments]
type ResponseStreamEventUnionArguments struct {
	// This field will be present if the value is a [string] instead of an object.
	OfString string `json:",inline"`
	// This field will be present if the value is a [any] instead of an object.
	OfResponseMcpCallArgumentsDoneEventArguments any `json:",inline"`
	JSON                                         struct {
		OfString                                     respjson.Field
		OfResponseMcpCallArgumentsDoneEventArguments respjson.Field
		raw                                          string
	} `json:"-"`
}

func (r *ResponseStreamEventUnionArguments) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Configuration options for a text response from the model. Can be plain text or
// structured JSON data. Learn more:
//
// - [Text inputs and outputs](https://platform.openai.com/docs/guides/text)
// - [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs)
type ResponseTextConfig struct {
	// An object specifying the format that the model must output.
	//
	// Configuring `{ "type": "json_schema" }` enables Structured Outputs, which
	// ensures the model will match your supplied JSON schema. Learn more in the
	// [Structured Outputs guide](https://platform.openai.com/docs/guides/structured-outputs).
	//
	// The default format is `{ "type": "text" }` with no additional options.
	//
	// **Not recommended for gpt-4o and newer models:**
	//
	// Setting to `{ "type": "json_object" }` enables the older JSON mode, which
	// ensures the message the model generates is valid JSON. Using `json_schema` is
	// preferred for models that support it.
	Format ResponseFormatTextConfigUnion `json:"format"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Format      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseTextConfig) RawJSON() string { return r.JSON.raw }
func (r *ResponseTextConfig) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponseTextConfig to a ResponseTextConfigParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseTextConfigParam.Overrides()
func (r ResponseTextConfig) ToParam() ResponseTextConfigParam {
	return param.Override[ResponseTextConfigParam](json.RawMessage(r.RawJSON()))
}

// Configuration options for a text response from the model. Can be plain text or
// structured JSON data. Learn more:
//
// - [Text inputs and outputs](https://platform.openai.com/docs/guides/text)
// - [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs)
type ResponseTextConfigParam struct {
	// An object specifying the format that the model must output.
	//
	// Configuring `{ "type": "json_schema" }` enables Structured Outputs, which
	// ensures the model will match your supplied JSON schema. Learn more in the
	// [Structured Outputs guide](https://platform.openai.com/docs/guides/structured-outputs).
	//
	// The default format is `{ "type": "text" }` with no additional options.
	//
	// **Not recommended for gpt-4o and newer models:**
	//
	// Setting to `{ "type": "json_object" }` enables the older JSON mode, which
	// ensures the message the model generates is valid JSON. Using `json_schema` is
	// preferred for models that support it.
	Format ResponseFormatTextConfigUnionParam `json:"format,omitzero"`
	paramObj
}

func (r ResponseTextConfigParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseTextConfigParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseTextConfigParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when there is an additional text delta.
type ResponseTextDeltaEvent struct {
	// The index of the content part that the text delta was added to.
	ContentIndex int64 `json:"content_index,required"`
	// The text delta that was added.
	Delta string `json:"delta,required"`
	// The ID of the output item that the text delta was added to.
	ItemID string `json:"item_id,required"`
	// The index of the output item that the text delta was added to.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number for this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.output_text.delta`.
	Type constant.ResponseOutputTextDelta `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ContentIndex   respjson.Field
		Delta          respjson.Field
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseTextDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseTextDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when text content is finalized.
type ResponseTextDoneEvent struct {
	// The index of the content part that the text content is finalized.
	ContentIndex int64 `json:"content_index,required"`
	// The ID of the output item that the text content is finalized.
	ItemID string `json:"item_id,required"`
	// The index of the output item that the text content is finalized.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number for this event.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The text content that is finalized.
	Text string `json:"text,required"`
	// The type of the event. Always `response.output_text.done`.
	Type constant.ResponseOutputTextDone `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ContentIndex   respjson.Field
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Text           respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseTextDoneEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseTextDoneEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Represents token usage details including input tokens, output tokens, a
// breakdown of output tokens, and the total tokens used.
type ResponseUsage struct {
	// The number of input tokens.
	InputTokens int64 `json:"input_tokens,required"`
	// A detailed breakdown of the input tokens.
	InputTokensDetails ResponseUsageInputTokensDetails `json:"input_tokens_details,required"`
	// The number of output tokens.
	OutputTokens int64 `json:"output_tokens,required"`
	// A detailed breakdown of the output tokens.
	OutputTokensDetails ResponseUsageOutputTokensDetails `json:"output_tokens_details,required"`
	// The total number of tokens used.
	TotalTokens int64 `json:"total_tokens,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		InputTokens         respjson.Field
		InputTokensDetails  respjson.Field
		OutputTokens        respjson.Field
		OutputTokensDetails respjson.Field
		TotalTokens         respjson.Field
		ExtraFields         map[string]respjson.Field
		raw                 string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseUsage) RawJSON() string { return r.JSON.raw }
func (r *ResponseUsage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A detailed breakdown of the input tokens.
type ResponseUsageInputTokensDetails struct {
	// The number of tokens that were retrieved from the cache.
	// [More on prompt caching](https://platform.openai.com/docs/guides/prompt-caching).
	CachedTokens int64 `json:"cached_tokens,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		CachedTokens respjson.Field
		ExtraFields  map[string]respjson.Field
		raw          string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseUsageInputTokensDetails) RawJSON() string { return r.JSON.raw }
func (r *ResponseUsageInputTokensDetails) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A detailed breakdown of the output tokens.
type ResponseUsageOutputTokensDetails struct {
	// The number of reasoning tokens.
	ReasoningTokens int64 `json:"reasoning_tokens,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ReasoningTokens respjson.Field
		ExtraFields     map[string]respjson.Field
		raw             string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseUsageOutputTokensDetails) RawJSON() string { return r.JSON.raw }
func (r *ResponseUsageOutputTokensDetails) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a web search call is completed.
type ResponseWebSearchCallCompletedEvent struct {
	// Unique ID for the output item associated with the web search call.
	ItemID string `json:"item_id,required"`
	// The index of the output item that the web search call is associated with.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of the web search call being processed.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.web_search_call.completed`.
	Type constant.ResponseWebSearchCallCompleted `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseWebSearchCallCompletedEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseWebSearchCallCompletedEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a web search call is initiated.
type ResponseWebSearchCallInProgressEvent struct {
	// Unique ID for the output item associated with the web search call.
	ItemID string `json:"item_id,required"`
	// The index of the output item that the web search call is associated with.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of the web search call being processed.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.web_search_call.in_progress`.
	Type constant.ResponseWebSearchCallInProgress `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseWebSearchCallInProgressEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseWebSearchCallInProgressEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a web search call is executing.
type ResponseWebSearchCallSearchingEvent struct {
	// Unique ID for the output item associated with the web search call.
	ItemID string `json:"item_id,required"`
	// The index of the output item that the web search call is associated with.
	OutputIndex int64 `json:"output_index,required"`
	// The sequence number of the web search call being processed.
	SequenceNumber int64 `json:"sequence_number,required"`
	// The type of the event. Always `response.web_search_call.searching`.
	Type constant.ResponseWebSearchCallSearching `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ItemID         respjson.Field
		OutputIndex    respjson.Field
		SequenceNumber respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseWebSearchCallSearchingEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseWebSearchCallSearchingEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToolUnion contains all possible properties and values from [FunctionTool],
// [FileSearchTool], [WebSearchTool], [ComputerTool], [ToolMcp],
// [ToolCodeInterpreter], [ToolImageGeneration], [ToolLocalShell].
//
// Use the [ToolUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ToolUnion struct {
	// This field is from variant [FunctionTool].
	Name string `json:"name"`
	// This field is from variant [FunctionTool].
	Parameters map[string]any `json:"parameters"`
	// This field is from variant [FunctionTool].
	Strict bool `json:"strict"`
	// Any of "function", "file_search", nil, "computer_use_preview", "mcp",
	// "code_interpreter", "image_generation", "local_shell".
	Type string `json:"type"`
	// This field is from variant [FunctionTool].
	Description string `json:"description"`
	// This field is from variant [FileSearchTool].
	VectorStoreIDs []string `json:"vector_store_ids"`
	// This field is from variant [FileSearchTool].
	Filters FileSearchToolFiltersUnion `json:"filters"`
	// This field is from variant [FileSearchTool].
	MaxNumResults int64 `json:"max_num_results"`
	// This field is from variant [FileSearchTool].
	RankingOptions FileSearchToolRankingOptions `json:"ranking_options"`
	// This field is from variant [WebSearchTool].
	SearchContextSize WebSearchToolSearchContextSize `json:"search_context_size"`
	// This field is from variant [WebSearchTool].
	UserLocation WebSearchToolUserLocation `json:"user_location"`
	// This field is from variant [ComputerTool].
	DisplayHeight int64 `json:"display_height"`
	// This field is from variant [ComputerTool].
	DisplayWidth int64 `json:"display_width"`
	// This field is from variant [ComputerTool].
	Environment ComputerToolEnvironment `json:"environment"`
	// This field is from variant [ToolMcp].
	ServerLabel string `json:"server_label"`
	// This field is from variant [ToolMcp].
	ServerURL string `json:"server_url"`
	// This field is from variant [ToolMcp].
	AllowedTools ToolMcpAllowedToolsUnion `json:"allowed_tools"`
	// This field is from variant [ToolMcp].
	Headers map[string]string `json:"headers"`
	// This field is from variant [ToolMcp].
	RequireApproval ToolMcpRequireApprovalUnion `json:"require_approval"`
	// This field is from variant [ToolCodeInterpreter].
	Container ToolCodeInterpreterContainerUnion `json:"container"`
	// This field is from variant [ToolImageGeneration].
	Background string `json:"background"`
	// This field is from variant [ToolImageGeneration].
	InputImageMask ToolImageGenerationInputImageMask `json:"input_image_mask"`
	// This field is from variant [ToolImageGeneration].
	Model string `json:"model"`
	// This field is from variant [ToolImageGeneration].
	Moderation string `json:"moderation"`
	// This field is from variant [ToolImageGeneration].
	OutputCompression int64 `json:"output_compression"`
	// This field is from variant [ToolImageGeneration].
	OutputFormat string `json:"output_format"`
	// This field is from variant [ToolImageGeneration].
	PartialImages int64 `json:"partial_images"`
	// This field is from variant [ToolImageGeneration].
	Quality string `json:"quality"`
	// This field is from variant [ToolImageGeneration].
	Size string `json:"size"`
	JSON struct {
		Name              respjson.Field
		Parameters        respjson.Field
		Strict            respjson.Field
		Type              respjson.Field
		Description       respjson.Field
		VectorStoreIDs    respjson.Field
		Filters           respjson.Field
		MaxNumResults     respjson.Field
		RankingOptions    respjson.Field
		SearchContextSize respjson.Field
		UserLocation      respjson.Field
		DisplayHeight     respjson.Field
		DisplayWidth      respjson.Field
		Environment       respjson.Field
		ServerLabel       respjson.Field
		ServerURL         respjson.Field
		AllowedTools      respjson.Field
		Headers           respjson.Field
		RequireApproval   respjson.Field
		Container         respjson.Field
		Background        respjson.Field
		InputImageMask    respjson.Field
		Model             respjson.Field
		Moderation        respjson.Field
		OutputCompression respjson.Field
		OutputFormat      respjson.Field
		PartialImages     respjson.Field
		Quality           respjson.Field
		Size              respjson.Field
		raw               string
	} `json:"-"`
}

func (u ToolUnion) AsFunction() (v FunctionTool) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ToolUnion) AsFileSearch() (v FileSearchTool) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ToolUnion) AsWebSearchPreview() (v WebSearchTool) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ToolUnion) AsComputerUsePreview() (v ComputerTool) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ToolUnion) AsMcp() (v ToolMcp) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ToolUnion) AsCodeInterpreter() (v ToolCodeInterpreter) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ToolUnion) AsImageGeneration() (v ToolImageGeneration) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ToolUnion) AsLocalShell() (v ToolLocalShell) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ToolUnion) RawJSON() string { return u.JSON.raw }

func (r *ToolUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ToolUnion to a ToolUnionParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ToolUnionParam.Overrides()
func (r ToolUnion) ToParam() ToolUnionParam {
	return param.Override[ToolUnionParam](json.RawMessage(r.RawJSON()))
}

// Give the model access to additional tools via remote Model Context Protocol
// (MCP) servers.
// [Learn more about MCP](https://platform.openai.com/docs/guides/tools-remote-mcp).
type ToolMcp struct {
	// A label for this MCP server, used to identify it in tool calls.
	ServerLabel string `json:"server_label,required"`
	// The URL for the MCP server.
	ServerURL string `json:"server_url,required"`
	// The type of the MCP tool. Always `mcp`.
	Type constant.Mcp `json:"type,required"`
	// List of allowed tool names or a filter object.
	AllowedTools ToolMcpAllowedToolsUnion `json:"allowed_tools,nullable"`
	// Optional HTTP headers to send to the MCP server. Use for authentication or other
	// purposes.
	Headers map[string]string `json:"headers,nullable"`
	// Specify which of the MCP server's tools require approval.
	RequireApproval ToolMcpRequireApprovalUnion `json:"require_approval,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ServerLabel     respjson.Field
		ServerURL       respjson.Field
		Type            respjson.Field
		AllowedTools    respjson.Field
		Headers         respjson.Field
		RequireApproval respjson.Field
		ExtraFields     map[string]respjson.Field
		raw             string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ToolMcp) RawJSON() string { return r.JSON.raw }
func (r *ToolMcp) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToolMcpAllowedToolsUnion contains all possible properties and values from
// [[]string], [ToolMcpAllowedToolsMcpAllowedToolsFilter].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfMcpAllowedTools]
type ToolMcpAllowedToolsUnion struct {
	// This field will be present if the value is a [[]string] instead of an object.
	OfMcpAllowedTools []string `json:",inline"`
	// This field is from variant [ToolMcpAllowedToolsMcpAllowedToolsFilter].
	ToolNames []string `json:"tool_names"`
	JSON      struct {
		OfMcpAllowedTools respjson.Field
		ToolNames         respjson.Field
		raw               string
	} `json:"-"`
}

func (u ToolMcpAllowedToolsUnion) AsMcpAllowedTools() (v []string) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ToolMcpAllowedToolsUnion) AsMcpAllowedToolsFilter() (v ToolMcpAllowedToolsMcpAllowedToolsFilter) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ToolMcpAllowedToolsUnion) RawJSON() string { return u.JSON.raw }

func (r *ToolMcpAllowedToolsUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A filter object to specify which tools are allowed.
type ToolMcpAllowedToolsMcpAllowedToolsFilter struct {
	// List of allowed tool names.
	ToolNames []string `json:"tool_names"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ToolNames   respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ToolMcpAllowedToolsMcpAllowedToolsFilter) RawJSON() string { return r.JSON.raw }
func (r *ToolMcpAllowedToolsMcpAllowedToolsFilter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToolMcpRequireApprovalUnion contains all possible properties and values from
// [ToolMcpRequireApprovalMcpToolApprovalFilter], [string].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfMcpToolApprovalSetting]
type ToolMcpRequireApprovalUnion struct {
	// This field will be present if the value is a [string] instead of an object.
	OfMcpToolApprovalSetting string `json:",inline"`
	// This field is from variant [ToolMcpRequireApprovalMcpToolApprovalFilter].
	Always ToolMcpRequireApprovalMcpToolApprovalFilterAlways `json:"always"`
	// This field is from variant [ToolMcpRequireApprovalMcpToolApprovalFilter].
	Never ToolMcpRequireApprovalMcpToolApprovalFilterNever `json:"never"`
	JSON  struct {
		OfMcpToolApprovalSetting respjson.Field
		Always                   respjson.Field
		Never                    respjson.Field
		raw                      string
	} `json:"-"`
}

func (u ToolMcpRequireApprovalUnion) AsMcpToolApprovalFilter() (v ToolMcpRequireApprovalMcpToolApprovalFilter) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ToolMcpRequireApprovalUnion) AsMcpToolApprovalSetting() (v string) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ToolMcpRequireApprovalUnion) RawJSON() string { return u.JSON.raw }

func (r *ToolMcpRequireApprovalUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ToolMcpRequireApprovalMcpToolApprovalFilter struct {
	// A list of tools that always require approval.
	Always ToolMcpRequireApprovalMcpToolApprovalFilterAlways `json:"always"`
	// A list of tools that never require approval.
	Never ToolMcpRequireApprovalMcpToolApprovalFilterNever `json:"never"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Always      respjson.Field
		Never       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ToolMcpRequireApprovalMcpToolApprovalFilter) RawJSON() string { return r.JSON.raw }
func (r *ToolMcpRequireApprovalMcpToolApprovalFilter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A list of tools that always require approval.
type ToolMcpRequireApprovalMcpToolApprovalFilterAlways struct {
	// List of tools that require approval.
	ToolNames []string `json:"tool_names"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ToolNames   respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ToolMcpRequireApprovalMcpToolApprovalFilterAlways) RawJSON() string { return r.JSON.raw }
func (r *ToolMcpRequireApprovalMcpToolApprovalFilterAlways) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A list of tools that never require approval.
type ToolMcpRequireApprovalMcpToolApprovalFilterNever struct {
	// List of tools that do not require approval.
	ToolNames []string `json:"tool_names"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ToolNames   respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ToolMcpRequireApprovalMcpToolApprovalFilterNever) RawJSON() string { return r.JSON.raw }
func (r *ToolMcpRequireApprovalMcpToolApprovalFilterNever) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Specify a single approval policy for all tools. One of `always` or `never`. When
// set to `always`, all tools will require approval. When set to `never`, all tools
// will not require approval.
type ToolMcpRequireApprovalMcpToolApprovalSetting string

const (
	ToolMcpRequireApprovalMcpToolApprovalSettingAlways ToolMcpRequireApprovalMcpToolApprovalSetting = "always"
	ToolMcpRequireApprovalMcpToolApprovalSettingNever  ToolMcpRequireApprovalMcpToolApprovalSetting = "never"
)

// A tool that runs Python code to help generate a response to a prompt.
type ToolCodeInterpreter struct {
	// The code interpreter container. Can be a container ID or an object that
	// specifies uploaded file IDs to make available to your code.
	Container ToolCodeInterpreterContainerUnion `json:"container,required"`
	// The type of the code interpreter tool. Always `code_interpreter`.
	Type constant.CodeInterpreter `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Container   respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ToolCodeInterpreter) RawJSON() string { return r.JSON.raw }
func (r *ToolCodeInterpreter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToolCodeInterpreterContainerUnion contains all possible properties and values
// from [string], [ToolCodeInterpreterContainerCodeInterpreterContainerAuto].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfString]
type ToolCodeInterpreterContainerUnion struct {
	// This field will be present if the value is a [string] instead of an object.
	OfString string `json:",inline"`
	// This field is from variant
	// [ToolCodeInterpreterContainerCodeInterpreterContainerAuto].
	Type constant.Auto `json:"type"`
	// This field is from variant
	// [ToolCodeInterpreterContainerCodeInterpreterContainerAuto].
	FileIDs []string `json:"file_ids"`
	JSON    struct {
		OfString respjson.Field
		Type     respjson.Field
		FileIDs  respjson.Field
		raw      string
	} `json:"-"`
}

func (u ToolCodeInterpreterContainerUnion) AsString() (v string) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ToolCodeInterpreterContainerUnion) AsCodeInterpreterContainerAuto() (v ToolCodeInterpreterContainerCodeInterpreterContainerAuto) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ToolCodeInterpreterContainerUnion) RawJSON() string { return u.JSON.raw }

func (r *ToolCodeInterpreterContainerUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Configuration for a code interpreter container. Optionally specify the IDs of
// the files to run the code on.
type ToolCodeInterpreterContainerCodeInterpreterContainerAuto struct {
	// Always `auto`.
	Type constant.Auto `json:"type,required"`
	// An optional list of uploaded files to make available to your code.
	FileIDs []string `json:"file_ids"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		FileIDs     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ToolCodeInterpreterContainerCodeInterpreterContainerAuto) RawJSON() string { return r.JSON.raw }
func (r *ToolCodeInterpreterContainerCodeInterpreterContainerAuto) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A tool that generates images using a model like `gpt-image-1`.
type ToolImageGeneration struct {
	// The type of the image generation tool. Always `image_generation`.
	Type constant.ImageGeneration `json:"type,required"`
	// Background type for the generated image. One of `transparent`, `opaque`, or
	// `auto`. Default: `auto`.
	//
	// Any of "transparent", "opaque", "auto".
	Background string `json:"background"`
	// Optional mask for inpainting. Contains `image_url` (string, optional) and
	// `file_id` (string, optional).
	InputImageMask ToolImageGenerationInputImageMask `json:"input_image_mask"`
	// The image generation model to use. Default: `gpt-image-1`.
	//
	// Any of "gpt-image-1".
	Model string `json:"model"`
	// Moderation level for the generated image. Default: `auto`.
	//
	// Any of "auto", "low".
	Moderation string `json:"moderation"`
	// Compression level for the output image. Default: 100.
	OutputCompression int64 `json:"output_compression"`
	// The output format of the generated image. One of `png`, `webp`, or `jpeg`.
	// Default: `png`.
	//
	// Any of "png", "webp", "jpeg".
	OutputFormat string `json:"output_format"`
	// Number of partial images to generate in streaming mode, from 0 (default value)
	// to 3.
	PartialImages int64 `json:"partial_images"`
	// The quality of the generated image. One of `low`, `medium`, `high`, or `auto`.
	// Default: `auto`.
	//
	// Any of "low", "medium", "high", "auto".
	Quality string `json:"quality"`
	// The size of the generated image. One of `1024x1024`, `1024x1536`, `1536x1024`,
	// or `auto`. Default: `auto`.
	//
	// Any of "1024x1024", "1024x1536", "1536x1024", "auto".
	Size string `json:"size"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type              respjson.Field
		Background        respjson.Field
		InputImageMask    respjson.Field
		Model             respjson.Field
		Moderation        respjson.Field
		OutputCompression respjson.Field
		OutputFormat      respjson.Field
		PartialImages     respjson.Field
		Quality           respjson.Field
		Size              respjson.Field
		ExtraFields       map[string]respjson.Field
		raw               string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ToolImageGeneration) RawJSON() string { return r.JSON.raw }
func (r *ToolImageGeneration) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Optional mask for inpainting. Contains `image_url` (string, optional) and
// `file_id` (string, optional).
type ToolImageGenerationInputImageMask struct {
	// File ID for the mask image.
	FileID string `json:"file_id"`
	// Base64-encoded mask image.
	ImageURL string `json:"image_url"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		FileID      respjson.Field
		ImageURL    respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ToolImageGenerationInputImageMask) RawJSON() string { return r.JSON.raw }
func (r *ToolImageGenerationInputImageMask) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A tool that allows the model to execute shell commands in a local environment.
type ToolLocalShell struct {
	// The type of the local shell tool. Always `local_shell`.
	Type constant.LocalShell `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ToolLocalShell) RawJSON() string { return r.JSON.raw }
func (r *ToolLocalShell) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func ToolParamOfFunction(name string, parameters map[string]any, strict bool) ToolUnionParam {
	var function FunctionToolParam
	function.Name = name
	function.Parameters = parameters
	function.Strict = param.NewOpt(strict)
	return ToolUnionParam{OfFunction: &function}
}

func ToolParamOfFileSearch(vectorStoreIDs []string) ToolUnionParam {
	var fileSearch FileSearchToolParam
	fileSearch.VectorStoreIDs = vectorStoreIDs
	return ToolUnionParam{OfFileSearch: &fileSearch}
}

func ToolParamOfWebSearchPreview(type_ WebSearchToolType) ToolUnionParam {
	var variant WebSearchToolParam
	variant.Type = type_
	return ToolUnionParam{OfWebSearchPreview: &variant}
}

func ToolParamOfComputerUsePreview(displayHeight int64, displayWidth int64, environment ComputerToolEnvironment) ToolUnionParam {
	var computerUsePreview ComputerToolParam
	computerUsePreview.DisplayHeight = displayHeight
	computerUsePreview.DisplayWidth = displayWidth
	computerUsePreview.Environment = environment
	return ToolUnionParam{OfComputerUsePreview: &computerUsePreview}
}

func ToolParamOfMcp(serverLabel string, serverURL string) ToolUnionParam {
	var mcp ToolMcpParam
	mcp.ServerLabel = serverLabel
	mcp.ServerURL = serverURL
	return ToolUnionParam{OfMcp: &mcp}
}

func ToolParamOfCodeInterpreter[
	T string | ToolCodeInterpreterContainerCodeInterpreterContainerAutoParam,
](container T) ToolUnionParam {
	var codeInterpreter ToolCodeInterpreterParam
	switch v := any(container).(type) {
	case string:
		codeInterpreter.Container.OfString = param.NewOpt(v)
	case ToolCodeInterpreterContainerCodeInterpreterContainerAutoParam:
		codeInterpreter.Container.OfCodeInterpreterContainerAuto = &v
	}
	return ToolUnionParam{OfCodeInterpreter: &codeInterpreter}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ToolUnionParam struct {
	OfFunction           *FunctionToolParam        `json:",omitzero,inline"`
	OfFileSearch         *FileSearchToolParam      `json:",omitzero,inline"`
	OfWebSearchPreview   *WebSearchToolParam       `json:",omitzero,inline"`
	OfComputerUsePreview *ComputerToolParam        `json:",omitzero,inline"`
	OfMcp                *ToolMcpParam             `json:",omitzero,inline"`
	OfCodeInterpreter    *ToolCodeInterpreterParam `json:",omitzero,inline"`
	OfImageGeneration    *ToolImageGenerationParam `json:",omitzero,inline"`
	OfLocalShell         *ToolLocalShellParam      `json:",omitzero,inline"`
	paramUnion
}

func (u ToolUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfFunction,
		u.OfFileSearch,
		u.OfWebSearchPreview,
		u.OfComputerUsePreview,
		u.OfMcp,
		u.OfCodeInterpreter,
		u.OfImageGeneration,
		u.OfLocalShell)
}
func (u *ToolUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ToolUnionParam) asAny() any {
	if !param.IsOmitted(u.OfFunction) {
		return u.OfFunction
	} else if !param.IsOmitted(u.OfFileSearch) {
		return u.OfFileSearch
	} else if !param.IsOmitted(u.OfWebSearchPreview) {
		return u.OfWebSearchPreview
	} else if !param.IsOmitted(u.OfComputerUsePreview) {
		return u.OfComputerUsePreview
	} else if !param.IsOmitted(u.OfMcp) {
		return u.OfMcp
	} else if !param.IsOmitted(u.OfCodeInterpreter) {
		return u.OfCodeInterpreter
	} else if !param.IsOmitted(u.OfImageGeneration) {
		return u.OfImageGeneration
	} else if !param.IsOmitted(u.OfLocalShell) {
		return u.OfLocalShell
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetName() *string {
	if vt := u.OfFunction; vt != nil {
		return &vt.Name
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetParameters() map[string]any {
	if vt := u.OfFunction; vt != nil {
		return vt.Parameters
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetStrict() *bool {
	if vt := u.OfFunction; vt != nil && vt.Strict.Valid() {
		return &vt.Strict.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetDescription() *string {
	if vt := u.OfFunction; vt != nil && vt.Description.Valid() {
		return &vt.Description.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetVectorStoreIDs() []string {
	if vt := u.OfFileSearch; vt != nil {
		return vt.VectorStoreIDs
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetFilters() *FileSearchToolFiltersUnionParam {
	if vt := u.OfFileSearch; vt != nil {
		return &vt.Filters
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetMaxNumResults() *int64 {
	if vt := u.OfFileSearch; vt != nil && vt.MaxNumResults.Valid() {
		return &vt.MaxNumResults.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetRankingOptions() *FileSearchToolRankingOptionsParam {
	if vt := u.OfFileSearch; vt != nil {
		return &vt.RankingOptions
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetSearchContextSize() *string {
	if vt := u.OfWebSearchPreview; vt != nil {
		return (*string)(&vt.SearchContextSize)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetUserLocation() *WebSearchToolUserLocationParam {
	if vt := u.OfWebSearchPreview; vt != nil {
		return &vt.UserLocation
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetDisplayHeight() *int64 {
	if vt := u.OfComputerUsePreview; vt != nil {
		return &vt.DisplayHeight
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetDisplayWidth() *int64 {
	if vt := u.OfComputerUsePreview; vt != nil {
		return &vt.DisplayWidth
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetEnvironment() *string {
	if vt := u.OfComputerUsePreview; vt != nil {
		return (*string)(&vt.Environment)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetServerLabel() *string {
	if vt := u.OfMcp; vt != nil {
		return &vt.ServerLabel
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetServerURL() *string {
	if vt := u.OfMcp; vt != nil {
		return &vt.ServerURL
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetAllowedTools() *ToolMcpAllowedToolsUnionParam {
	if vt := u.OfMcp; vt != nil {
		return &vt.AllowedTools
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetHeaders() map[string]string {
	if vt := u.OfMcp; vt != nil {
		return vt.Headers
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetRequireApproval() *ToolMcpRequireApprovalUnionParam {
	if vt := u.OfMcp; vt != nil {
		return &vt.RequireApproval
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetContainer() *ToolCodeInterpreterContainerUnionParam {
	if vt := u.OfCodeInterpreter; vt != nil {
		return &vt.Container
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetBackground() *string {
	if vt := u.OfImageGeneration; vt != nil {
		return &vt.Background
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetInputImageMask() *ToolImageGenerationInputImageMaskParam {
	if vt := u.OfImageGeneration; vt != nil {
		return &vt.InputImageMask
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetModel() *string {
	if vt := u.OfImageGeneration; vt != nil {
		return &vt.Model
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetModeration() *string {
	if vt := u.OfImageGeneration; vt != nil {
		return &vt.Moderation
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetOutputCompression() *int64 {
	if vt := u.OfImageGeneration; vt != nil && vt.OutputCompression.Valid() {
		return &vt.OutputCompression.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetOutputFormat() *string {
	if vt := u.OfImageGeneration; vt != nil {
		return &vt.OutputFormat
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetPartialImages() *int64 {
	if vt := u.OfImageGeneration; vt != nil && vt.PartialImages.Valid() {
		return &vt.PartialImages.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetQuality() *string {
	if vt := u.OfImageGeneration; vt != nil {
		return &vt.Quality
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetSize() *string {
	if vt := u.OfImageGeneration; vt != nil {
		return &vt.Size
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetType() *string {
	if vt := u.OfFunction; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfFileSearch; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfWebSearchPreview; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfComputerUsePreview; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfMcp; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfCodeInterpreter; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfImageGeneration; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfLocalShell; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[ToolUnionParam](
		"type",
		apijson.Discriminator[FunctionToolParam]("function"),
		apijson.Discriminator[FileSearchToolParam]("file_search"),
		apijson.Discriminator[WebSearchToolParam]("web_search_preview"),
		apijson.Discriminator[WebSearchToolParam]("web_search_preview_2025_03_11"),
		apijson.Discriminator[ComputerToolParam]("computer_use_preview"),
		apijson.Discriminator[ToolMcpParam]("mcp"),
		apijson.Discriminator[ToolCodeInterpreterParam]("code_interpreter"),
		apijson.Discriminator[ToolImageGenerationParam]("image_generation"),
		apijson.Discriminator[ToolLocalShellParam]("local_shell"),
	)
}

// Give the model access to additional tools via remote Model Context Protocol
// (MCP) servers.
// [Learn more about MCP](https://platform.openai.com/docs/guides/tools-remote-mcp).
//
// The properties ServerLabel, ServerURL, Type are required.
type ToolMcpParam struct {
	// A label for this MCP server, used to identify it in tool calls.
	ServerLabel string `json:"server_label,required"`
	// The URL for the MCP server.
	ServerURL string `json:"server_url,required"`
	// List of allowed tool names or a filter object.
	AllowedTools ToolMcpAllowedToolsUnionParam `json:"allowed_tools,omitzero"`
	// Optional HTTP headers to send to the MCP server. Use for authentication or other
	// purposes.
	Headers map[string]string `json:"headers,omitzero"`
	// Specify which of the MCP server's tools require approval.
	RequireApproval ToolMcpRequireApprovalUnionParam `json:"require_approval,omitzero"`
	// The type of the MCP tool. Always `mcp`.
	//
	// This field can be elided, and will marshal its zero value as "mcp".
	Type constant.Mcp `json:"type,required"`
	paramObj
}

func (r ToolMcpParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolMcpParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolMcpParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ToolMcpAllowedToolsUnionParam struct {
	OfMcpAllowedTools       []string                                       `json:",omitzero,inline"`
	OfMcpAllowedToolsFilter *ToolMcpAllowedToolsMcpAllowedToolsFilterParam `json:",omitzero,inline"`
	paramUnion
}

func (u ToolMcpAllowedToolsUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfMcpAllowedTools, u.OfMcpAllowedToolsFilter)
}
func (u *ToolMcpAllowedToolsUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ToolMcpAllowedToolsUnionParam) asAny() any {
	if !param.IsOmitted(u.OfMcpAllowedTools) {
		return &u.OfMcpAllowedTools
	} else if !param.IsOmitted(u.OfMcpAllowedToolsFilter) {
		return u.OfMcpAllowedToolsFilter
	}
	return nil
}

// A filter object to specify which tools are allowed.
type ToolMcpAllowedToolsMcpAllowedToolsFilterParam struct {
	// List of allowed tool names.
	ToolNames []string `json:"tool_names,omitzero"`
	paramObj
}

func (r ToolMcpAllowedToolsMcpAllowedToolsFilterParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolMcpAllowedToolsMcpAllowedToolsFilterParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolMcpAllowedToolsMcpAllowedToolsFilterParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ToolMcpRequireApprovalUnionParam struct {
	OfMcpToolApprovalFilter *ToolMcpRequireApprovalMcpToolApprovalFilterParam `json:",omitzero,inline"`
	// Check if union is this variant with
	// !param.IsOmitted(union.OfMcpToolApprovalSetting)
	OfMcpToolApprovalSetting param.Opt[string] `json:",omitzero,inline"`
	paramUnion
}

func (u ToolMcpRequireApprovalUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfMcpToolApprovalFilter, u.OfMcpToolApprovalSetting)
}
func (u *ToolMcpRequireApprovalUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ToolMcpRequireApprovalUnionParam) asAny() any {
	if !param.IsOmitted(u.OfMcpToolApprovalFilter) {
		return u.OfMcpToolApprovalFilter
	} else if !param.IsOmitted(u.OfMcpToolApprovalSetting) {
		return &u.OfMcpToolApprovalSetting
	}
	return nil
}

type ToolMcpRequireApprovalMcpToolApprovalFilterParam struct {
	// A list of tools that always require approval.
	Always ToolMcpRequireApprovalMcpToolApprovalFilterAlwaysParam `json:"always,omitzero"`
	// A list of tools that never require approval.
	Never ToolMcpRequireApprovalMcpToolApprovalFilterNeverParam `json:"never,omitzero"`
	paramObj
}

func (r ToolMcpRequireApprovalMcpToolApprovalFilterParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolMcpRequireApprovalMcpToolApprovalFilterParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolMcpRequireApprovalMcpToolApprovalFilterParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A list of tools that always require approval.
type ToolMcpRequireApprovalMcpToolApprovalFilterAlwaysParam struct {
	// List of tools that require approval.
	ToolNames []string `json:"tool_names,omitzero"`
	paramObj
}

func (r ToolMcpRequireApprovalMcpToolApprovalFilterAlwaysParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolMcpRequireApprovalMcpToolApprovalFilterAlwaysParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolMcpRequireApprovalMcpToolApprovalFilterAlwaysParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A list of tools that never require approval.
type ToolMcpRequireApprovalMcpToolApprovalFilterNeverParam struct {
	// List of tools that do not require approval.
	ToolNames []string `json:"tool_names,omitzero"`
	paramObj
}

func (r ToolMcpRequireApprovalMcpToolApprovalFilterNeverParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolMcpRequireApprovalMcpToolApprovalFilterNeverParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolMcpRequireApprovalMcpToolApprovalFilterNeverParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A tool that runs Python code to help generate a response to a prompt.
//
// The properties Container, Type are required.
type ToolCodeInterpreterParam struct {
	// The code interpreter container. Can be a container ID or an object that
	// specifies uploaded file IDs to make available to your code.
	Container ToolCodeInterpreterContainerUnionParam `json:"container,omitzero,required"`
	// The type of the code interpreter tool. Always `code_interpreter`.
	//
	// This field can be elided, and will marshal its zero value as "code_interpreter".
	Type constant.CodeInterpreter `json:"type,required"`
	paramObj
}

func (r ToolCodeInterpreterParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolCodeInterpreterParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolCodeInterpreterParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ToolCodeInterpreterContainerUnionParam struct {
	OfString                       param.Opt[string]                                              `json:",omitzero,inline"`
	OfCodeInterpreterContainerAuto *ToolCodeInterpreterContainerCodeInterpreterContainerAutoParam `json:",omitzero,inline"`
	paramUnion
}

func (u ToolCodeInterpreterContainerUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfCodeInterpreterContainerAuto)
}
func (u *ToolCodeInterpreterContainerUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ToolCodeInterpreterContainerUnionParam) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfCodeInterpreterContainerAuto) {
		return u.OfCodeInterpreterContainerAuto
	}
	return nil
}

// Configuration for a code interpreter container. Optionally specify the IDs of
// the files to run the code on.
//
// The property Type is required.
type ToolCodeInterpreterContainerCodeInterpreterContainerAutoParam struct {
	// An optional list of uploaded files to make available to your code.
	FileIDs []string `json:"file_ids,omitzero"`
	// Always `auto`.
	//
	// This field can be elided, and will marshal its zero value as "auto".
	Type constant.Auto `json:"type,required"`
	paramObj
}

func (r ToolCodeInterpreterContainerCodeInterpreterContainerAutoParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolCodeInterpreterContainerCodeInterpreterContainerAutoParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolCodeInterpreterContainerCodeInterpreterContainerAutoParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A tool that generates images using a model like `gpt-image-1`.
//
// The property Type is required.
type ToolImageGenerationParam struct {
	// Compression level for the output image. Default: 100.
	OutputCompression param.Opt[int64] `json:"output_compression,omitzero"`
	// Number of partial images to generate in streaming mode, from 0 (default value)
	// to 3.
	PartialImages param.Opt[int64] `json:"partial_images,omitzero"`
	// Background type for the generated image. One of `transparent`, `opaque`, or
	// `auto`. Default: `auto`.
	//
	// Any of "transparent", "opaque", "auto".
	Background string `json:"background,omitzero"`
	// Optional mask for inpainting. Contains `image_url` (string, optional) and
	// `file_id` (string, optional).
	InputImageMask ToolImageGenerationInputImageMaskParam `json:"input_image_mask,omitzero"`
	// The image generation model to use. Default: `gpt-image-1`.
	//
	// Any of "gpt-image-1".
	Model string `json:"model,omitzero"`
	// Moderation level for the generated image. Default: `auto`.
	//
	// Any of "auto", "low".
	Moderation string `json:"moderation,omitzero"`
	// The output format of the generated image. One of `png`, `webp`, or `jpeg`.
	// Default: `png`.
	//
	// Any of "png", "webp", "jpeg".
	OutputFormat string `json:"output_format,omitzero"`
	// The quality of the generated image. One of `low`, `medium`, `high`, or `auto`.
	// Default: `auto`.
	//
	// Any of "low", "medium", "high", "auto".
	Quality string `json:"quality,omitzero"`
	// The size of the generated image. One of `1024x1024`, `1024x1536`, `1536x1024`,
	// or `auto`. Default: `auto`.
	//
	// Any of "1024x1024", "1024x1536", "1536x1024", "auto".
	Size string `json:"size,omitzero"`
	// The type of the image generation tool. Always `image_generation`.
	//
	// This field can be elided, and will marshal its zero value as "image_generation".
	Type constant.ImageGeneration `json:"type,required"`
	paramObj
}

func (r ToolImageGenerationParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolImageGenerationParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolImageGenerationParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[ToolImageGenerationParam](
		"background", "transparent", "opaque", "auto",
	)
	apijson.RegisterFieldValidator[ToolImageGenerationParam](
		"model", "gpt-image-1",
	)
	apijson.RegisterFieldValidator[ToolImageGenerationParam](
		"moderation", "auto", "low",
	)
	apijson.RegisterFieldValidator[ToolImageGenerationParam](
		"output_format", "png", "webp", "jpeg",
	)
	apijson.RegisterFieldValidator[ToolImageGenerationParam](
		"quality", "low", "medium", "high", "auto",
	)
	apijson.RegisterFieldValidator[ToolImageGenerationParam](
		"size", "1024x1024", "1024x1536", "1536x1024", "auto",
	)
}

// Optional mask for inpainting. Contains `image_url` (string, optional) and
// `file_id` (string, optional).
type ToolImageGenerationInputImageMaskParam struct {
	// File ID for the mask image.
	FileID param.Opt[string] `json:"file_id,omitzero"`
	// Base64-encoded mask image.
	ImageURL param.Opt[string] `json:"image_url,omitzero"`
	paramObj
}

func (r ToolImageGenerationInputImageMaskParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolImageGenerationInputImageMaskParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolImageGenerationInputImageMaskParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func NewToolLocalShellParam() ToolLocalShellParam {
	return ToolLocalShellParam{
		Type: "local_shell",
	}
}

// A tool that allows the model to execute shell commands in a local environment.
//
// This struct has a constant value, construct it with [NewToolLocalShellParam].
type ToolLocalShellParam struct {
	// The type of the local shell tool. Always `local_shell`.
	Type constant.LocalShell `json:"type,required"`
	paramObj
}

func (r ToolLocalShellParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolLocalShellParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolLocalShellParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Use this option to force the model to call a specific function.
type ToolChoiceFunction struct {
	// The name of the function to call.
	Name string `json:"name,required"`
	// For function calling, the type is always `function`.
	Type constant.Function `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Name        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ToolChoiceFunction) RawJSON() string { return r.JSON.raw }
func (r *ToolChoiceFunction) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ToolChoiceFunction to a ToolChoiceFunctionParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ToolChoiceFunctionParam.Overrides()
func (r ToolChoiceFunction) ToParam() ToolChoiceFunctionParam {
	return param.Override[ToolChoiceFunctionParam](json.RawMessage(r.RawJSON()))
}

// Use this option to force the model to call a specific function.
//
// The properties Name, Type are required.
type ToolChoiceFunctionParam struct {
	// The name of the function to call.
	Name string `json:"name,required"`
	// For function calling, the type is always `function`.
	//
	// This field can be elided, and will marshal its zero value as "function".
	Type constant.Function `json:"type,required"`
	paramObj
}

func (r ToolChoiceFunctionParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolChoiceFunctionParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolChoiceFunctionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Use this option to force the model to call a specific tool on a remote MCP
// server.
type ToolChoiceMcp struct {
	// The label of the MCP server to use.
	ServerLabel string `json:"server_label,required"`
	// For MCP tools, the type is always `mcp`.
	Type constant.Mcp `json:"type,required"`
	// The name of the tool to call on the server.
	Name string `json:"name,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ServerLabel respjson.Field
		Type        respjson.Field
		Name        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ToolChoiceMcp) RawJSON() string { return r.JSON.raw }
func (r *ToolChoiceMcp) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ToolChoiceMcp to a ToolChoiceMcpParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ToolChoiceMcpParam.Overrides()
func (r ToolChoiceMcp) ToParam() ToolChoiceMcpParam {
	return param.Override[ToolChoiceMcpParam](json.RawMessage(r.RawJSON()))
}

// Use this option to force the model to call a specific tool on a remote MCP
// server.
//
// The properties ServerLabel, Type are required.
type ToolChoiceMcpParam struct {
	// The label of the MCP server to use.
	ServerLabel string `json:"server_label,required"`
	// The name of the tool to call on the server.
	Name param.Opt[string] `json:"name,omitzero"`
	// For MCP tools, the type is always `mcp`.
	//
	// This field can be elided, and will marshal its zero value as "mcp".
	Type constant.Mcp `json:"type,required"`
	paramObj
}

func (r ToolChoiceMcpParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolChoiceMcpParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolChoiceMcpParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Controls which (if any) tool is called by the model.
//
// `none` means the model will not call any tool and instead generates a message.
//
// `auto` means the model can pick between generating a message or calling one or
// more tools.
//
// `required` means the model must call one or more tools.
type ToolChoiceOptions string

const (
	ToolChoiceOptionsNone     ToolChoiceOptions = "none"
	ToolChoiceOptionsAuto     ToolChoiceOptions = "auto"
	ToolChoiceOptionsRequired ToolChoiceOptions = "required"
)

// Indicates that the model should use a built-in tool to generate a response.
// [Learn more about built-in tools](https://platform.openai.com/docs/guides/tools).
type ToolChoiceTypes struct {
	// The type of hosted tool the model should to use. Learn more about
	// [built-in tools](https://platform.openai.com/docs/guides/tools).
	//
	// Allowed values are:
	//
	// - `file_search`
	// - `web_search_preview`
	// - `computer_use_preview`
	// - `code_interpreter`
	// - `image_generation`
	//
	// Any of "file_search", "web_search_preview", "computer_use_preview",
	// "web_search_preview_2025_03_11", "image_generation", "code_interpreter".
	Type ToolChoiceTypesType `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ToolChoiceTypes) RawJSON() string { return r.JSON.raw }
func (r *ToolChoiceTypes) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ToolChoiceTypes to a ToolChoiceTypesParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ToolChoiceTypesParam.Overrides()
func (r ToolChoiceTypes) ToParam() ToolChoiceTypesParam {
	return param.Override[ToolChoiceTypesParam](json.RawMessage(r.RawJSON()))
}

// The type of hosted tool the model should to use. Learn more about
// [built-in tools](https://platform.openai.com/docs/guides/tools).
//
// Allowed values are:
//
// - `file_search`
// - `web_search_preview`
// - `computer_use_preview`
// - `code_interpreter`
// - `image_generation`
type ToolChoiceTypesType string

const (
	ToolChoiceTypesTypeFileSearch                 ToolChoiceTypesType = "file_search"
	ToolChoiceTypesTypeWebSearchPreview           ToolChoiceTypesType = "web_search_preview"
	ToolChoiceTypesTypeComputerUsePreview         ToolChoiceTypesType = "computer_use_preview"
	ToolChoiceTypesTypeWebSearchPreview2025_03_11 ToolChoiceTypesType = "web_search_preview_2025_03_11"
	ToolChoiceTypesTypeImageGeneration            ToolChoiceTypesType = "image_generation"
	ToolChoiceTypesTypeCodeInterpreter            ToolChoiceTypesType = "code_interpreter"
)

// Indicates that the model should use a built-in tool to generate a response.
// [Learn more about built-in tools](https://platform.openai.com/docs/guides/tools).
//
// The property Type is required.
type ToolChoiceTypesParam struct {
	// The type of hosted tool the model should to use. Learn more about
	// [built-in tools](https://platform.openai.com/docs/guides/tools).
	//
	// Allowed values are:
	//
	// - `file_search`
	// - `web_search_preview`
	// - `computer_use_preview`
	// - `code_interpreter`
	// - `image_generation`
	//
	// Any of "file_search", "web_search_preview", "computer_use_preview",
	// "web_search_preview_2025_03_11", "image_generation", "code_interpreter".
	Type ToolChoiceTypesType `json:"type,omitzero,required"`
	paramObj
}

func (r ToolChoiceTypesParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolChoiceTypesParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolChoiceTypesParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// This tool searches the web for relevant results to use in a response. Learn more
// about the
// [web search tool](https://platform.openai.com/docs/guides/tools-web-search).
type WebSearchTool struct {
	// The type of the web search tool. One of `web_search_preview` or
	// `web_search_preview_2025_03_11`.
	//
	// Any of "web_search_preview", "web_search_preview_2025_03_11".
	Type WebSearchToolType `json:"type,required"`
	// High level guidance for the amount of context window space to use for the
	// search. One of `low`, `medium`, or `high`. `medium` is the default.
	//
	// Any of "low", "medium", "high".
	SearchContextSize WebSearchToolSearchContextSize `json:"search_context_size"`
	// The user's location.
	UserLocation WebSearchToolUserLocation `json:"user_location,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type              respjson.Field
		SearchContextSize respjson.Field
		UserLocation      respjson.Field
		ExtraFields       map[string]respjson.Field
		raw               string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r WebSearchTool) RawJSON() string { return r.JSON.raw }
func (r *WebSearchTool) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this WebSearchTool to a WebSearchToolParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// WebSearchToolParam.Overrides()
func (r WebSearchTool) ToParam() WebSearchToolParam {
	return param.Override[WebSearchToolParam](json.RawMessage(r.RawJSON()))
}

// The type of the web search tool. One of `web_search_preview` or
// `web_search_preview_2025_03_11`.
type WebSearchToolType string

const (
	WebSearchToolTypeWebSearchPreview           WebSearchToolType = "web_search_preview"
	WebSearchToolTypeWebSearchPreview2025_03_11 WebSearchToolType = "web_search_preview_2025_03_11"
)

// High level guidance for the amount of context window space to use for the
// search. One of `low`, `medium`, or `high`. `medium` is the default.
type WebSearchToolSearchContextSize string

const (
	WebSearchToolSearchContextSizeLow    WebSearchToolSearchContextSize = "low"
	WebSearchToolSearchContextSizeMedium WebSearchToolSearchContextSize = "medium"
	WebSearchToolSearchContextSizeHigh   WebSearchToolSearchContextSize = "high"
)

// The user's location.
type WebSearchToolUserLocation struct {
	// The type of location approximation. Always `approximate`.
	Type constant.Approximate `json:"type,required"`
	// Free text input for the city of the user, e.g. `San Francisco`.
	City string `json:"city,nullable"`
	// The two-letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1) of
	// the user, e.g. `US`.
	Country string `json:"country,nullable"`
	// Free text input for the region of the user, e.g. `California`.
	Region string `json:"region,nullable"`
	// The [IANA timezone](https://timeapi.io/documentation/iana-timezones) of the
	// user, e.g. `America/Los_Angeles`.
	Timezone string `json:"timezone,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		City        respjson.Field
		Country     respjson.Field
		Region      respjson.Field
		Timezone    respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r WebSearchToolUserLocation) RawJSON() string { return r.JSON.raw }
func (r *WebSearchToolUserLocation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// This tool searches the web for relevant results to use in a response. Learn more
// about the
// [web search tool](https://platform.openai.com/docs/guides/tools-web-search).
//
// The property Type is required.
type WebSearchToolParam struct {
	// The type of the web search tool. One of `web_search_preview` or
	// `web_search_preview_2025_03_11`.
	//
	// Any of "web_search_preview", "web_search_preview_2025_03_11".
	Type WebSearchToolType `json:"type,omitzero,required"`
	// The user's location.
	UserLocation WebSearchToolUserLocationParam `json:"user_location,omitzero"`
	// High level guidance for the amount of context window space to use for the
	// search. One of `low`, `medium`, or `high`. `medium` is the default.
	//
	// Any of "low", "medium", "high".
	SearchContextSize WebSearchToolSearchContextSize `json:"search_context_size,omitzero"`
	paramObj
}

func (r WebSearchToolParam) MarshalJSON() (data []byte, err error) {
	type shadow WebSearchToolParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *WebSearchToolParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The user's location.
//
// The property Type is required.
type WebSearchToolUserLocationParam struct {
	// Free text input for the city of the user, e.g. `San Francisco`.
	City param.Opt[string] `json:"city,omitzero"`
	// The two-letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1) of
	// the user, e.g. `US`.
	Country param.Opt[string] `json:"country,omitzero"`
	// Free text input for the region of the user, e.g. `California`.
	Region param.Opt[string] `json:"region,omitzero"`
	// The [IANA timezone](https://timeapi.io/documentation/iana-timezones) of the
	// user, e.g. `America/Los_Angeles`.
	Timezone param.Opt[string] `json:"timezone,omitzero"`
	// The type of location approximation. Always `approximate`.
	//
	// This field can be elided, and will marshal its zero value as "approximate".
	Type constant.Approximate `json:"type,required"`
	paramObj
}

func (r WebSearchToolUserLocationParam) MarshalJSON() (data []byte, err error) {
	type shadow WebSearchToolUserLocationParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *WebSearchToolUserLocationParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ResponseNewParams struct {
	// Whether to run the model response in the background.
	// [Learn more](https://platform.openai.com/docs/guides/background).
	Background param.Opt[bool] `json:"background,omitzero"`
	// A system (or developer) message inserted into the model's context.
	//
	// When using along with `previous_response_id`, the instructions from a previous
	// response will not be carried over to the next response. This makes it simple to
	// swap out system (or developer) messages in new responses.
	Instructions param.Opt[string] `json:"instructions,omitzero"`
	// An upper bound for the number of tokens that can be generated for a response,
	// including visible output tokens and
	// [reasoning tokens](https://platform.openai.com/docs/guides/reasoning).
	MaxOutputTokens param.Opt[int64] `json:"max_output_tokens,omitzero"`
	// The maximum number of total calls to built-in tools that can be processed in a
	// response. This maximum number applies across all built-in tool calls, not per
	// individual tool. Any further attempts to call a tool by the model will be
	// ignored.
	MaxToolCalls param.Opt[int64] `json:"max_tool_calls,omitzero"`
	// Whether to allow the model to run tool calls in parallel.
	ParallelToolCalls param.Opt[bool] `json:"parallel_tool_calls,omitzero"`
	// The unique ID of the previous response to the model. Use this to create
	// multi-turn conversations. Learn more about
	// [conversation state](https://platform.openai.com/docs/guides/conversation-state).
	PreviousResponseID param.Opt[string] `json:"previous_response_id,omitzero"`
	// Whether to store the generated model response for later retrieval via API.
	Store param.Opt[bool] `json:"store,omitzero"`
	// What sampling temperature to use, between 0 and 2. Higher values like 0.8 will
	// make the output more random, while lower values like 0.2 will make it more
	// focused and deterministic. We generally recommend altering this or `top_p` but
	// not both.
	Temperature param.Opt[float64] `json:"temperature,omitzero"`
	// An integer between 0 and 20 specifying the number of most likely tokens to
	// return at each token position, each with an associated log probability.
	TopLogprobs param.Opt[int64] `json:"top_logprobs,omitzero"`
	// An alternative to sampling with temperature, called nucleus sampling, where the
	// model considers the results of the tokens with top_p probability mass. So 0.1
	// means only the tokens comprising the top 10% probability mass are considered.
	//
	// We generally recommend altering this or `temperature` but not both.
	TopP param.Opt[float64] `json:"top_p,omitzero"`
	// A stable identifier for your end-users. Used to boost cache hit rates by better
	// bucketing similar requests and to help OpenAI detect and prevent abuse.
	// [Learn more](https://platform.openai.com/docs/guides/safety-best-practices#end-user-ids).
	User param.Opt[string] `json:"user,omitzero"`
	// Specify additional output data to include in the model response. Currently
	// supported values are:
	//
	//   - `code_interpreter_call.outputs`: Includes the outputs of python code execution
	//     in code interpreter tool call items.
	//   - `computer_call_output.output.image_url`: Include image urls from the computer
	//     call output.
	//   - `file_search_call.results`: Include the search results of the file search tool
	//     call.
	//   - `message.input_image.image_url`: Include image urls from the input message.
	//   - `message.output_text.logprobs`: Include logprobs with assistant messages.
	//   - `reasoning.encrypted_content`: Includes an encrypted version of reasoning
	//     tokens in reasoning item outputs. This enables reasoning items to be used in
	//     multi-turn conversations when using the Responses API statelessly (like when
	//     the `store` parameter is set to `false`, or when an organization is enrolled
	//     in the zero data retention program).
	Include []ResponseIncludable `json:"include,omitzero"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,omitzero"`
	// Reference to a prompt template and its variables.
	// [Learn more](https://platform.openai.com/docs/guides/text?api-mode=responses#reusable-prompts).
	Prompt ResponsePromptParam `json:"prompt,omitzero"`
	// Specifies the processing type used for serving the request.
	//
	//   - If set to 'auto', then the request will be processed with the service tier
	//     configured in the Project settings. Unless otherwise configured, the Project
	//     will use 'default'.
	//   - If set to 'default', then the requset will be processed with the standard
	//     pricing and performance for the selected model.
	//   - If set to '[flex](https://platform.openai.com/docs/guides/flex-processing)' or
	//     'priority', then the request will be processed with the corresponding service
	//     tier. [Contact sales](https://openai.com/contact-sales) to learn more about
	//     Priority processing.
	//   - When not set, the default behavior is 'auto'.
	//
	// When the `service_tier` parameter is set, the response body will include the
	// `service_tier` value based on the processing mode actually used to serve the
	// request. This response value may be different from the value set in the
	// parameter.
	//
	// Any of "auto", "default", "flex", "scale", "priority".
	ServiceTier ResponseNewParamsServiceTier `json:"service_tier,omitzero"`
	// The truncation strategy to use for the model response.
	//
	//   - `auto`: If the context of this response and previous ones exceeds the model's
	//     context window size, the model will truncate the response to fit the context
	//     window by dropping input items in the middle of the conversation.
	//   - `disabled` (default): If a model response will exceed the context window size
	//     for a model, the request will fail with a 400 error.
	//
	// Any of "auto", "disabled".
	Truncation ResponseNewParamsTruncation `json:"truncation,omitzero"`
	// Text, image, or file inputs to the model, used to generate a response.
	//
	// Learn more:
	//
	// - [Text inputs and outputs](https://platform.openai.com/docs/guides/text)
	// - [Image inputs](https://platform.openai.com/docs/guides/images)
	// - [File inputs](https://platform.openai.com/docs/guides/pdf-files)
	// - [Conversation state](https://platform.openai.com/docs/guides/conversation-state)
	// - [Function calling](https://platform.openai.com/docs/guides/function-calling)
	Input ResponseNewParamsInputUnion `json:"input,omitzero"`
	// Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI offers a
	// wide range of models with different capabilities, performance characteristics,
	// and price points. Refer to the
	// [model guide](https://platform.openai.com/docs/models) to browse and compare
	// available models.
	Model shared.ResponsesModel `json:"model,omitzero"`
	// **o-series models only**
	//
	// Configuration options for
	// [reasoning models](https://platform.openai.com/docs/guides/reasoning).
	Reasoning shared.ReasoningParam `json:"reasoning,omitzero"`
	// Configuration options for a text response from the model. Can be plain text or
	// structured JSON data. Learn more:
	//
	// - [Text inputs and outputs](https://platform.openai.com/docs/guides/text)
	// - [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs)
	Text ResponseTextConfigParam `json:"text,omitzero"`
	// How the model should select which tool (or tools) to use when generating a
	// response. See the `tools` parameter to see how to specify which tools the model
	// can call.
	ToolChoice ResponseNewParamsToolChoiceUnion `json:"tool_choice,omitzero"`
	// An array of tools the model may call while generating a response. You can
	// specify which tool to use by setting the `tool_choice` parameter.
	//
	// The two categories of tools you can provide the model are:
	//
	//   - **Built-in tools**: Tools that are provided by OpenAI that extend the model's
	//     capabilities, like
	//     [web search](https://platform.openai.com/docs/guides/tools-web-search) or
	//     [file search](https://platform.openai.com/docs/guides/tools-file-search).
	//     Learn more about
	//     [built-in tools](https://platform.openai.com/docs/guides/tools).
	//   - **Function calls (custom tools)**: Functions that are defined by you, enabling
	//     the model to call your own code. Learn more about
	//     [function calling](https://platform.openai.com/docs/guides/function-calling).
	Tools []ToolUnionParam `json:"tools,omitzero"`
	paramObj
}

func (r ResponseNewParams) MarshalJSON() (data []byte, err error) {
	type shadow ResponseNewParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseNewParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ResponseNewParamsInputUnion struct {
	OfString        param.Opt[string]  `json:",omitzero,inline"`
	OfInputItemList ResponseInputParam `json:",omitzero,inline"`
	paramUnion
}

func (u ResponseNewParamsInputUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfInputItemList)
}
func (u *ResponseNewParamsInputUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ResponseNewParamsInputUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfInputItemList) {
		return &u.OfInputItemList
	}
	return nil
}

// Specifies the processing type used for serving the request.
//
//   - If set to 'auto', then the request will be processed with the service tier
//     configured in the Project settings. Unless otherwise configured, the Project
//     will use 'default'.
//   - If set to 'default', then the requset will be processed with the standard
//     pricing and performance for the selected model.
//   - If set to '[flex](https://platform.openai.com/docs/guides/flex-processing)' or
//     'priority', then the request will be processed with the corresponding service
//     tier. [Contact sales](https://openai.com/contact-sales) to learn more about
//     Priority processing.
//   - When not set, the default behavior is 'auto'.
//
// When the `service_tier` parameter is set, the response body will include the
// `service_tier` value based on the processing mode actually used to serve the
// request. This response value may be different from the value set in the
// parameter.
type ResponseNewParamsServiceTier string

const (
	ResponseNewParamsServiceTierAuto     ResponseNewParamsServiceTier = "auto"
	ResponseNewParamsServiceTierDefault  ResponseNewParamsServiceTier = "default"
	ResponseNewParamsServiceTierFlex     ResponseNewParamsServiceTier = "flex"
	ResponseNewParamsServiceTierScale    ResponseNewParamsServiceTier = "scale"
	ResponseNewParamsServiceTierPriority ResponseNewParamsServiceTier = "priority"
)

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ResponseNewParamsToolChoiceUnion struct {
	// Check if union is this variant with !param.IsOmitted(union.OfToolChoiceMode)
	OfToolChoiceMode param.Opt[ToolChoiceOptions] `json:",omitzero,inline"`
	OfHostedTool     *ToolChoiceTypesParam        `json:",omitzero,inline"`
	OfFunctionTool   *ToolChoiceFunctionParam     `json:",omitzero,inline"`
	OfMcpTool        *ToolChoiceMcpParam          `json:",omitzero,inline"`
	paramUnion
}

func (u ResponseNewParamsToolChoiceUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfToolChoiceMode, u.OfHostedTool, u.OfFunctionTool, u.OfMcpTool)
}
func (u *ResponseNewParamsToolChoiceUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ResponseNewParamsToolChoiceUnion) asAny() any {
	if !param.IsOmitted(u.OfToolChoiceMode) {
		return &u.OfToolChoiceMode
	} else if !param.IsOmitted(u.OfHostedTool) {
		return u.OfHostedTool
	} else if !param.IsOmitted(u.OfFunctionTool) {
		return u.OfFunctionTool
	} else if !param.IsOmitted(u.OfMcpTool) {
		return u.OfMcpTool
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseNewParamsToolChoiceUnion) GetServerLabel() *string {
	if vt := u.OfMcpTool; vt != nil {
		return &vt.ServerLabel
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseNewParamsToolChoiceUnion) GetType() *string {
	if vt := u.OfHostedTool; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfFunctionTool; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfMcpTool; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseNewParamsToolChoiceUnion) GetName() *string {
	if vt := u.OfFunctionTool; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfMcpTool; vt != nil && vt.Name.Valid() {
		return &vt.Name.Value
	}
	return nil
}

// The truncation strategy to use for the model response.
//
//   - `auto`: If the context of this response and previous ones exceeds the model's
//     context window size, the model will truncate the response to fit the context
//     window by dropping input items in the middle of the conversation.
//   - `disabled` (default): If a model response will exceed the context window size
//     for a model, the request will fail with a 400 error.
type ResponseNewParamsTruncation string

const (
	ResponseNewParamsTruncationAuto     ResponseNewParamsTruncation = "auto"
	ResponseNewParamsTruncationDisabled ResponseNewParamsTruncation = "disabled"
)

type ResponseGetParams struct {
	// The sequence number of the event after which to start streaming.
	StartingAfter param.Opt[int64] `query:"starting_after,omitzero" json:"-"`
	// Additional fields to include in the response. See the `include` parameter for
	// Response creation above for more information.
	Include []ResponseIncludable `query:"include,omitzero" json:"-"`
	paramObj
}

// URLQuery serializes [ResponseGetParams]'s query parameters as `url.Values`.
func (r ResponseGetParams) URLQuery() (v url.Values, err error) {
	return apiquery.MarshalWithSettings(r, apiquery.QuerySettings{
		ArrayFormat:  apiquery.ArrayQueryFormatBrackets,
		NestedFormat: apiquery.NestedQueryFormatBrackets,
	})
}
