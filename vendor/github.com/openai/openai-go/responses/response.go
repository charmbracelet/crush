// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package responses

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/openai/openai-go/internal/apijson"
	"github.com/openai/openai-go/internal/apiquery"
	"github.com/openai/openai-go/internal/requestconfig"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/resp"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/shared"
	"github.com/openai/openai-go/shared/constant"
	"github.com/tidwall/gjson"
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

// A tool that controls a virtual computer. Learn more about the
// [computer tool](https://platform.openai.com/docs/guides/tools-computer-use).
type ComputerTool struct {
	// The height of the computer display.
	DisplayHeight float64 `json:"display_height,required"`
	// The width of the computer display.
	DisplayWidth float64 `json:"display_width,required"`
	// The type of computer environment to control.
	//
	// Any of "mac", "windows", "ubuntu", "browser".
	Environment ComputerToolEnvironment `json:"environment,required"`
	// The type of the computer use tool. Always `computer_use_preview`.
	Type constant.ComputerUsePreview `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		DisplayHeight resp.Field
		DisplayWidth  resp.Field
		Environment   resp.Field
		Type          resp.Field
		ExtraFields   map[string]resp.Field
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
// ComputerToolParam.IsOverridden()
func (r ComputerTool) ToParam() ComputerToolParam {
	return param.OverrideObj[ComputerToolParam](r.RawJSON())
}

// The type of computer environment to control.
type ComputerToolEnvironment string

const (
	ComputerToolEnvironmentMac     ComputerToolEnvironment = "mac"
	ComputerToolEnvironmentWindows ComputerToolEnvironment = "windows"
	ComputerToolEnvironmentUbuntu  ComputerToolEnvironment = "ubuntu"
	ComputerToolEnvironmentBrowser ComputerToolEnvironment = "browser"
)

// A tool that controls a virtual computer. Learn more about the
// [computer tool](https://platform.openai.com/docs/guides/tools-computer-use).
//
// The properties DisplayHeight, DisplayWidth, Environment, Type are required.
type ComputerToolParam struct {
	// The height of the computer display.
	DisplayHeight float64 `json:"display_height,required"`
	// The width of the computer display.
	DisplayWidth float64 `json:"display_width,required"`
	// The type of computer environment to control.
	//
	// Any of "mac", "windows", "ubuntu", "browser".
	Environment ComputerToolEnvironment `json:"environment,omitzero,required"`
	// The type of the computer use tool. Always `computer_use_preview`.
	//
	// This field can be elided, and will marshal its zero value as
	// "computer_use_preview".
	Type constant.ComputerUsePreview `json:"type,required"`
	paramObj
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ComputerToolParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r ComputerToolParam) MarshalJSON() (data []byte, err error) {
	type shadow ComputerToolParam
	return param.MarshalObject(r, (*shadow)(&r))
}

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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f EasyInputMessageParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r EasyInputMessageParam) MarshalJSON() (data []byte, err error) {
	type shadow EasyInputMessageParam
	return param.MarshalObject(r, (*shadow)(&r))
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type EasyInputMessageContentUnionParam struct {
	OfString               param.Opt[string]                    `json:",omitzero,inline"`
	OfInputItemContentList ResponseInputMessageContentListParam `json:",omitzero,inline"`
	paramUnion
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (u EasyInputMessageContentUnionParam) IsPresent() bool {
	return !param.IsOmitted(u) && !u.IsNull()
}
func (u EasyInputMessageContentUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion[EasyInputMessageContentUnionParam](u.OfString, u.OfInputItemContentList)
}

func (u *EasyInputMessageContentUnionParam) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfInputItemContentList) {
		return &u.OfInputItemContentList
	}
	return nil
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

// A tool that searches for relevant content from uploaded files. Learn more about
// the
// [file search tool](https://platform.openai.com/docs/guides/tools-file-search).
type FileSearchTool struct {
	// The type of the file search tool. Always `file_search`.
	Type constant.FileSearch `json:"type,required"`
	// The IDs of the vector stores to search.
	VectorStoreIDs []string `json:"vector_store_ids,required"`
	// A filter to apply based on file attributes.
	Filters FileSearchToolFiltersUnion `json:"filters"`
	// The maximum number of results to return. This number should be between 1 and 50
	// inclusive.
	MaxNumResults int64 `json:"max_num_results"`
	// Ranking options for search.
	RankingOptions FileSearchToolRankingOptions `json:"ranking_options"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Type           resp.Field
		VectorStoreIDs resp.Field
		Filters        resp.Field
		MaxNumResults  resp.Field
		RankingOptions resp.Field
		ExtraFields    map[string]resp.Field
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
// FileSearchToolParam.IsOverridden()
func (r FileSearchTool) ToParam() FileSearchToolParam {
	return param.OverrideObj[FileSearchToolParam](r.RawJSON())
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
		Key     resp.Field
		Type    resp.Field
		Value   resp.Field
		Filters resp.Field
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Ranker         resp.Field
		ScoreThreshold resp.Field
		ExtraFields    map[string]resp.Field
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
	// A filter to apply based on file attributes.
	Filters FileSearchToolFiltersUnionParam `json:"filters,omitzero"`
	// Ranking options for search.
	RankingOptions FileSearchToolRankingOptionsParam `json:"ranking_options,omitzero"`
	// The type of the file search tool. Always `file_search`.
	//
	// This field can be elided, and will marshal its zero value as "file_search".
	Type constant.FileSearch `json:"type,required"`
	paramObj
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f FileSearchToolParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r FileSearchToolParam) MarshalJSON() (data []byte, err error) {
	type shadow FileSearchToolParam
	return param.MarshalObject(r, (*shadow)(&r))
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type FileSearchToolFiltersUnionParam struct {
	OfComparisonFilter *shared.ComparisonFilterParam `json:",omitzero,inline"`
	OfCompoundFilter   *shared.CompoundFilterParam   `json:",omitzero,inline"`
	paramUnion
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (u FileSearchToolFiltersUnionParam) IsPresent() bool { return !param.IsOmitted(u) && !u.IsNull() }
func (u FileSearchToolFiltersUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion[FileSearchToolFiltersUnionParam](u.OfComparisonFilter, u.OfCompoundFilter)
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f FileSearchToolRankingOptionsParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r FileSearchToolRankingOptionsParam) MarshalJSON() (data []byte, err error) {
	type shadow FileSearchToolRankingOptionsParam
	return param.MarshalObject(r, (*shadow)(&r))
}

func init() {
	apijson.RegisterFieldValidator[FileSearchToolRankingOptionsParam](
		"Ranker", false, "auto", "default-2024-11-15",
	)
}

// Defines a function in your own code the model can choose to call. Learn more
// about
// [function calling](https://platform.openai.com/docs/guides/function-calling).
type FunctionTool struct {
	// The name of the function to call.
	Name string `json:"name,required"`
	// A JSON schema object describing the parameters of the function.
	Parameters map[string]interface{} `json:"parameters,required"`
	// Whether to enforce strict parameter validation. Default `true`.
	Strict bool `json:"strict,required"`
	// The type of the function tool. Always `function`.
	Type constant.Function `json:"type,required"`
	// A description of the function. Used by the model to determine whether or not to
	// call the function.
	Description string `json:"description,nullable"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Name        resp.Field
		Parameters  resp.Field
		Strict      resp.Field
		Type        resp.Field
		Description resp.Field
		ExtraFields map[string]resp.Field
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
// FunctionToolParam.IsOverridden()
func (r FunctionTool) ToParam() FunctionToolParam {
	return param.OverrideObj[FunctionToolParam](r.RawJSON())
}

// Defines a function in your own code the model can choose to call. Learn more
// about
// [function calling](https://platform.openai.com/docs/guides/function-calling).
//
// The properties Name, Parameters, Strict, Type are required.
type FunctionToolParam struct {
	// The name of the function to call.
	Name string `json:"name,required"`
	// A JSON schema object describing the parameters of the function.
	Parameters map[string]interface{} `json:"parameters,omitzero,required"`
	// Whether to enforce strict parameter validation. Default `true`.
	Strict bool `json:"strict,required"`
	// A description of the function. Used by the model to determine whether or not to
	// call the function.
	Description param.Opt[string] `json:"description,omitzero"`
	// The type of the function tool. Always `function`.
	//
	// This field can be elided, and will marshal its zero value as "function".
	Type constant.Function `json:"type,required"`
	paramObj
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f FunctionToolParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r FunctionToolParam) MarshalJSON() (data []byte, err error) {
	type shadow FunctionToolParam
	return param.MarshalObject(r, (*shadow)(&r))
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
	// Inserts a system (or developer) message as the first item in the model's
	// context.
	//
	// When using along with `previous_response_id`, the instructions from a previous
	// response will be not be carried over to the next response. This makes it simple
	// to swap out system (or developer) messages in new responses.
	Instructions string `json:"instructions,required"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,required"`
	// Model ID used to generate the response, like `gpt-4o` or `o1`. OpenAI offers a
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
	// An upper bound for the number of tokens that can be generated for a response,
	// including visible output tokens and
	// [reasoning tokens](https://platform.openai.com/docs/guides/reasoning).
	MaxOutputTokens int64 `json:"max_output_tokens,nullable"`
	// The unique ID of the previous response to the model. Use this to create
	// multi-turn conversations. Learn more about
	// [conversation state](https://platform.openai.com/docs/guides/conversation-state).
	PreviousResponseID string `json:"previous_response_id,nullable"`
	// **o-series models only**
	//
	// Configuration options for
	// [reasoning models](https://platform.openai.com/docs/guides/reasoning).
	Reasoning shared.Reasoning `json:"reasoning,nullable"`
	// The status of the response generation. One of `completed`, `failed`,
	// `in_progress`, or `incomplete`.
	//
	// Any of "completed", "failed", "in_progress", "incomplete".
	Status ResponseStatus `json:"status"`
	// Configuration options for a text response from the model. Can be plain text or
	// structured JSON data. Learn more:
	//
	// - [Text inputs and outputs](https://platform.openai.com/docs/guides/text)
	// - [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs)
	Text ResponseTextConfig `json:"text"`
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
	// A unique identifier representing your end-user, which can help OpenAI to monitor
	// and detect abuse.
	// [Learn more](https://platform.openai.com/docs/guides/safety-best-practices#end-user-ids).
	User string `json:"user"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ID                 resp.Field
		CreatedAt          resp.Field
		Error              resp.Field
		IncompleteDetails  resp.Field
		Instructions       resp.Field
		Metadata           resp.Field
		Model              resp.Field
		Object             resp.Field
		Output             resp.Field
		ParallelToolCalls  resp.Field
		Temperature        resp.Field
		ToolChoice         resp.Field
		Tools              resp.Field
		TopP               resp.Field
		MaxOutputTokens    resp.Field
		PreviousResponseID resp.Field
		Reasoning          resp.Field
		Status             resp.Field
		Text               resp.Field
		Truncation         resp.Field
		Usage              resp.Field
		User               resp.Field
		ExtraFields        map[string]resp.Field
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Reason      resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseIncompleteDetails) RawJSON() string { return r.JSON.raw }
func (r *ResponseIncompleteDetails) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseToolChoiceUnion contains all possible properties and values from
// [ToolChoiceOptions], [ToolChoiceTypes], [ToolChoiceFunction].
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
	// This field is from variant [ToolChoiceFunction].
	Name string `json:"name"`
	JSON struct {
		OfToolChoiceMode resp.Field
		Type             resp.Field
		Name             resp.Field
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

// Returns the unmodified JSON received from the API
func (u ResponseToolChoiceUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseToolChoiceUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

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
	// The type of the event. Always `response.audio.delta`.
	Type constant.ResponseAudioDelta `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Delta       resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseAudioDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseAudioDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when the audio response is complete.
type ResponseAudioDoneEvent struct {
	// The type of the event. Always `response.audio.done`.
	Type constant.ResponseAudioDone `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
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
	// The type of the event. Always `response.audio.transcript.delta`.
	Type constant.ResponseAudioTranscriptDelta `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Delta       resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseAudioTranscriptDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseAudioTranscriptDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when the full audio transcript is completed.
type ResponseAudioTranscriptDoneEvent struct {
	// The type of the event. Always `response.audio.transcript.done`.
	Type constant.ResponseAudioTranscriptDone `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseAudioTranscriptDoneEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseAudioTranscriptDoneEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a partial code snippet is added by the code interpreter.
type ResponseCodeInterpreterCallCodeDeltaEvent struct {
	// The partial code snippet added by the code interpreter.
	Delta string `json:"delta,required"`
	// The index of the output item that the code interpreter call is in progress.
	OutputIndex int64 `json:"output_index,required"`
	// The type of the event. Always `response.code_interpreter_call.code.delta`.
	Type constant.ResponseCodeInterpreterCallCodeDelta `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Delta       resp.Field
		OutputIndex resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCodeInterpreterCallCodeDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseCodeInterpreterCallCodeDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when code snippet output is finalized by the code interpreter.
type ResponseCodeInterpreterCallCodeDoneEvent struct {
	// The final code snippet output by the code interpreter.
	Code string `json:"code,required"`
	// The index of the output item that the code interpreter call is in progress.
	OutputIndex int64 `json:"output_index,required"`
	// The type of the event. Always `response.code_interpreter_call.code.done`.
	Type constant.ResponseCodeInterpreterCallCodeDone `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Code        resp.Field
		OutputIndex resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCodeInterpreterCallCodeDoneEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseCodeInterpreterCallCodeDoneEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when the code interpreter call is completed.
type ResponseCodeInterpreterCallCompletedEvent struct {
	// A tool call to run code.
	CodeInterpreterCall ResponseCodeInterpreterToolCall `json:"code_interpreter_call,required"`
	// The index of the output item that the code interpreter call is in progress.
	OutputIndex int64 `json:"output_index,required"`
	// The type of the event. Always `response.code_interpreter_call.completed`.
	Type constant.ResponseCodeInterpreterCallCompleted `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		CodeInterpreterCall resp.Field
		OutputIndex         resp.Field
		Type                resp.Field
		ExtraFields         map[string]resp.Field
		raw                 string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCodeInterpreterCallCompletedEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseCodeInterpreterCallCompletedEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a code interpreter call is in progress.
type ResponseCodeInterpreterCallInProgressEvent struct {
	// A tool call to run code.
	CodeInterpreterCall ResponseCodeInterpreterToolCall `json:"code_interpreter_call,required"`
	// The index of the output item that the code interpreter call is in progress.
	OutputIndex int64 `json:"output_index,required"`
	// The type of the event. Always `response.code_interpreter_call.in_progress`.
	Type constant.ResponseCodeInterpreterCallInProgress `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		CodeInterpreterCall resp.Field
		OutputIndex         resp.Field
		Type                resp.Field
		ExtraFields         map[string]resp.Field
		raw                 string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCodeInterpreterCallInProgressEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseCodeInterpreterCallInProgressEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when the code interpreter is actively interpreting the code snippet.
type ResponseCodeInterpreterCallInterpretingEvent struct {
	// A tool call to run code.
	CodeInterpreterCall ResponseCodeInterpreterToolCall `json:"code_interpreter_call,required"`
	// The index of the output item that the code interpreter call is in progress.
	OutputIndex int64 `json:"output_index,required"`
	// The type of the event. Always `response.code_interpreter_call.interpreting`.
	Type constant.ResponseCodeInterpreterCallInterpreting `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		CodeInterpreterCall resp.Field
		OutputIndex         resp.Field
		Type                resp.Field
		ExtraFields         map[string]resp.Field
		raw                 string
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
	// The code to run.
	Code string `json:"code,required"`
	// The results of the code interpreter tool call.
	Results []ResponseCodeInterpreterToolCallResultUnion `json:"results,required"`
	// The status of the code interpreter tool call.
	//
	// Any of "in_progress", "interpreting", "completed".
	Status ResponseCodeInterpreterToolCallStatus `json:"status,required"`
	// The type of the code interpreter tool call. Always `code_interpreter_call`.
	Type constant.CodeInterpreterCall `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ID          resp.Field
		Code        resp.Field
		Results     resp.Field
		Status      resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCodeInterpreterToolCall) RawJSON() string { return r.JSON.raw }
func (r *ResponseCodeInterpreterToolCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseCodeInterpreterToolCallResultUnion contains all possible properties and
// values from [ResponseCodeInterpreterToolCallResultLogs],
// [ResponseCodeInterpreterToolCallResultFiles].
//
// Use the [ResponseCodeInterpreterToolCallResultUnion.AsAny] method to switch on
// the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ResponseCodeInterpreterToolCallResultUnion struct {
	// This field is from variant [ResponseCodeInterpreterToolCallResultLogs].
	Logs string `json:"logs"`
	// Any of "logs", "files".
	Type string `json:"type"`
	// This field is from variant [ResponseCodeInterpreterToolCallResultFiles].
	Files []ResponseCodeInterpreterToolCallResultFilesFile `json:"files"`
	JSON  struct {
		Logs  resp.Field
		Type  resp.Field
		Files resp.Field
		raw   string
	} `json:"-"`
}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseCodeInterpreterToolCallResultUnion.AsAny().(type) {
//	case ResponseCodeInterpreterToolCallResultLogs:
//	case ResponseCodeInterpreterToolCallResultFiles:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseCodeInterpreterToolCallResultUnion) AsAny() any {
	switch u.Type {
	case "logs":
		return u.AsLogs()
	case "files":
		return u.AsFiles()
	}
	return nil
}

func (u ResponseCodeInterpreterToolCallResultUnion) AsLogs() (v ResponseCodeInterpreterToolCallResultLogs) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseCodeInterpreterToolCallResultUnion) AsFiles() (v ResponseCodeInterpreterToolCallResultFiles) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ResponseCodeInterpreterToolCallResultUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseCodeInterpreterToolCallResultUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The output of a code interpreter tool call that is text.
type ResponseCodeInterpreterToolCallResultLogs struct {
	// The logs of the code interpreter tool call.
	Logs string `json:"logs,required"`
	// The type of the code interpreter text output. Always `logs`.
	Type constant.Logs `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Logs        resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCodeInterpreterToolCallResultLogs) RawJSON() string { return r.JSON.raw }
func (r *ResponseCodeInterpreterToolCallResultLogs) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The output of a code interpreter tool call that is a file.
type ResponseCodeInterpreterToolCallResultFiles struct {
	Files []ResponseCodeInterpreterToolCallResultFilesFile `json:"files,required"`
	// The type of the code interpreter file output. Always `files`.
	Type constant.Files `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Files       resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCodeInterpreterToolCallResultFiles) RawJSON() string { return r.JSON.raw }
func (r *ResponseCodeInterpreterToolCallResultFiles) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ResponseCodeInterpreterToolCallResultFilesFile struct {
	// The ID of the file.
	FileID string `json:"file_id,required"`
	// The MIME type of the file.
	MimeType string `json:"mime_type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		FileID      resp.Field
		MimeType    resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCodeInterpreterToolCallResultFilesFile) RawJSON() string { return r.JSON.raw }
func (r *ResponseCodeInterpreterToolCallResultFilesFile) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The status of the code interpreter tool call.
type ResponseCodeInterpreterToolCallStatus string

const (
	ResponseCodeInterpreterToolCallStatusInProgress   ResponseCodeInterpreterToolCallStatus = "in_progress"
	ResponseCodeInterpreterToolCallStatusInterpreting ResponseCodeInterpreterToolCallStatus = "interpreting"
	ResponseCodeInterpreterToolCallStatusCompleted    ResponseCodeInterpreterToolCallStatus = "completed"
)

// Emitted when the model response is complete.
type ResponseCompletedEvent struct {
	// Properties of the completed response.
	Response Response `json:"response,required"`
	// The type of the event. Always `response.completed`.
	Type constant.ResponseCompleted `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Response    resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ID                  resp.Field
		Action              resp.Field
		CallID              resp.Field
		PendingSafetyChecks resp.Field
		Status              resp.Field
		Type                resp.Field
		ExtraFields         map[string]resp.Field
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
// ResponseComputerToolCallParam.IsOverridden()
func (r ResponseComputerToolCall) ToParam() ResponseComputerToolCallParam {
	return param.OverrideObj[ResponseComputerToolCallParam](r.RawJSON())
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
		Button  resp.Field
		Type    resp.Field
		X       resp.Field
		Y       resp.Field
		Path    resp.Field
		Keys    resp.Field
		ScrollX resp.Field
		ScrollY resp.Field
		Text    resp.Field
		raw     string
	} `json:"-"`
}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseComputerToolCallActionUnion.AsAny().(type) {
//	case ResponseComputerToolCallActionClick:
//	case ResponseComputerToolCallActionDoubleClick:
//	case ResponseComputerToolCallActionDrag:
//	case ResponseComputerToolCallActionKeypress:
//	case ResponseComputerToolCallActionMove:
//	case ResponseComputerToolCallActionScreenshot:
//	case ResponseComputerToolCallActionScroll:
//	case ResponseComputerToolCallActionType:
//	case ResponseComputerToolCallActionWait:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseComputerToolCallActionUnion) AsAny() any {
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Button      resp.Field
		Type        resp.Field
		X           resp.Field
		Y           resp.Field
		ExtraFields map[string]resp.Field
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Type        resp.Field
		X           resp.Field
		Y           resp.Field
		ExtraFields map[string]resp.Field
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Path        resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		X           resp.Field
		Y           resp.Field
		ExtraFields map[string]resp.Field
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Keys        resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Type        resp.Field
		X           resp.Field
		Y           resp.Field
		ExtraFields map[string]resp.Field
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Type        resp.Field
		ExtraFields map[string]resp.Field
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ScrollX     resp.Field
		ScrollY     resp.Field
		Type        resp.Field
		X           resp.Field
		Y           resp.Field
		ExtraFields map[string]resp.Field
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Text        resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Type        resp.Field
		ExtraFields map[string]resp.Field
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ID          resp.Field
		Code        resp.Field
		Message     resp.Field
		ExtraFields map[string]resp.Field
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseComputerToolCallParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r ResponseComputerToolCallParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallParam
	return param.MarshalObject(r, (*shadow)(&r))
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (u ResponseComputerToolCallActionUnionParam) IsPresent() bool {
	return !param.IsOmitted(u) && !u.IsNull()
}
func (u ResponseComputerToolCallActionUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion[ResponseComputerToolCallActionUnionParam](u.OfClick,
		u.OfDoubleClick,
		u.OfDrag,
		u.OfKeypress,
		u.OfMove,
		u.OfScreenshot,
		u.OfScroll,
		u.OfType,
		u.OfWait)
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
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseComputerToolCallActionClickParam{}),
			DiscriminatorValue: "click",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseComputerToolCallActionDoubleClickParam{}),
			DiscriminatorValue: "double_click",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseComputerToolCallActionDragParam{}),
			DiscriminatorValue: "drag",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseComputerToolCallActionKeypressParam{}),
			DiscriminatorValue: "keypress",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseComputerToolCallActionMoveParam{}),
			DiscriminatorValue: "move",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseComputerToolCallActionScreenshotParam{}),
			DiscriminatorValue: "screenshot",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseComputerToolCallActionScrollParam{}),
			DiscriminatorValue: "scroll",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseComputerToolCallActionTypeParam{}),
			DiscriminatorValue: "type",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseComputerToolCallActionWaitParam{}),
			DiscriminatorValue: "wait",
		},
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseComputerToolCallActionClickParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseComputerToolCallActionClickParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionClickParam
	return param.MarshalObject(r, (*shadow)(&r))
}

func init() {
	apijson.RegisterFieldValidator[ResponseComputerToolCallActionClickParam](
		"Button", false, "left", "right", "wheel", "back", "forward",
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseComputerToolCallActionDoubleClickParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseComputerToolCallActionDoubleClickParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionDoubleClickParam
	return param.MarshalObject(r, (*shadow)(&r))
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseComputerToolCallActionDragParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseComputerToolCallActionDragParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionDragParam
	return param.MarshalObject(r, (*shadow)(&r))
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseComputerToolCallActionDragPathParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseComputerToolCallActionDragPathParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionDragPathParam
	return param.MarshalObject(r, (*shadow)(&r))
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseComputerToolCallActionKeypressParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseComputerToolCallActionKeypressParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionKeypressParam
	return param.MarshalObject(r, (*shadow)(&r))
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseComputerToolCallActionMoveParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseComputerToolCallActionMoveParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionMoveParam
	return param.MarshalObject(r, (*shadow)(&r))
}

// A screenshot action.
//
// The property Type is required.
type ResponseComputerToolCallActionScreenshotParam struct {
	// Specifies the event type. For a screenshot action, this property is always set
	// to `screenshot`.
	//
	// This field can be elided, and will marshal its zero value as "screenshot".
	Type constant.Screenshot `json:"type,required"`
	paramObj
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseComputerToolCallActionScreenshotParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseComputerToolCallActionScreenshotParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionScreenshotParam
	return param.MarshalObject(r, (*shadow)(&r))
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseComputerToolCallActionScrollParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseComputerToolCallActionScrollParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionScrollParam
	return param.MarshalObject(r, (*shadow)(&r))
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseComputerToolCallActionTypeParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseComputerToolCallActionTypeParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionTypeParam
	return param.MarshalObject(r, (*shadow)(&r))
}

// A wait action.
//
// The property Type is required.
type ResponseComputerToolCallActionWaitParam struct {
	// Specifies the event type. For a wait action, this property is always set to
	// `wait`.
	//
	// This field can be elided, and will marshal its zero value as "wait".
	Type constant.Wait `json:"type,required"`
	paramObj
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseComputerToolCallActionWaitParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseComputerToolCallActionWaitParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallActionWaitParam
	return param.MarshalObject(r, (*shadow)(&r))
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseComputerToolCallPendingSafetyCheckParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseComputerToolCallPendingSafetyCheckParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallPendingSafetyCheckParam
	return param.MarshalObject(r, (*shadow)(&r))
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ID                       resp.Field
		CallID                   resp.Field
		Output                   resp.Field
		Type                     resp.Field
		AcknowledgedSafetyChecks resp.Field
		Status                   resp.Field
		ExtraFields              map[string]resp.Field
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ID          resp.Field
		Code        resp.Field
		Message     resp.Field
		ExtraFields map[string]resp.Field
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Type        resp.Field
		FileID      resp.Field
		ImageURL    resp.Field
		ExtraFields map[string]resp.Field
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
// ResponseComputerToolCallOutputScreenshotParam.IsOverridden()
func (r ResponseComputerToolCallOutputScreenshot) ToParam() ResponseComputerToolCallOutputScreenshotParam {
	return param.OverrideObj[ResponseComputerToolCallOutputScreenshotParam](r.RawJSON())
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseComputerToolCallOutputScreenshotParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseComputerToolCallOutputScreenshotParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseComputerToolCallOutputScreenshotParam
	return param.MarshalObject(r, (*shadow)(&r))
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
	// The type of the event. Always `response.content_part.added`.
	Type constant.ResponseContentPartAdded `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ContentIndex resp.Field
		ItemID       resp.Field
		OutputIndex  resp.Field
		Part         resp.Field
		Type         resp.Field
		ExtraFields  map[string]resp.Field
		raw          string
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
	// This field is from variant [ResponseOutputRefusal].
	Refusal string `json:"refusal"`
	JSON    struct {
		Annotations resp.Field
		Text        resp.Field
		Type        resp.Field
		Refusal     resp.Field
		raw         string
	} `json:"-"`
}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseContentPartAddedEventPartUnion.AsAny().(type) {
//	case ResponseOutputText:
//	case ResponseOutputRefusal:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseContentPartAddedEventPartUnion) AsAny() any {
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
	// The type of the event. Always `response.content_part.done`.
	Type constant.ResponseContentPartDone `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ContentIndex resp.Field
		ItemID       resp.Field
		OutputIndex  resp.Field
		Part         resp.Field
		Type         resp.Field
		ExtraFields  map[string]resp.Field
		raw          string
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
	// This field is from variant [ResponseOutputRefusal].
	Refusal string `json:"refusal"`
	JSON    struct {
		Annotations resp.Field
		Text        resp.Field
		Type        resp.Field
		Refusal     resp.Field
		raw         string
	} `json:"-"`
}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseContentPartDoneEventPartUnion.AsAny().(type) {
//	case ResponseOutputText:
//	case ResponseOutputRefusal:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseContentPartDoneEventPartUnion) AsAny() any {
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
	// The type of the event. Always `response.created`.
	Type constant.ResponseCreated `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Response    resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Code        resp.Field
		Message     resp.Field
		ExtraFields map[string]resp.Field
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
	// The type of the event. Always `error`.
	Type constant.Error `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Code        resp.Field
		Message     resp.Field
		Param       resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
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
	// The type of the event. Always `response.failed`.
	Type constant.ResponseFailed `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Response    resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
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
	// The type of the event. Always `response.file_search_call.completed`.
	Type constant.ResponseFileSearchCallCompleted `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ItemID      resp.Field
		OutputIndex resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
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
	// The type of the event. Always `response.file_search_call.in_progress`.
	Type constant.ResponseFileSearchCallInProgress `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ItemID      resp.Field
		OutputIndex resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
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
	// The type of the event. Always `response.file_search_call.searching`.
	Type constant.ResponseFileSearchCallSearching `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ItemID      resp.Field
		OutputIndex resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ID          resp.Field
		Queries     resp.Field
		Status      resp.Field
		Type        resp.Field
		Results     resp.Field
		ExtraFields map[string]resp.Field
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
// ResponseFileSearchToolCallParam.IsOverridden()
func (r ResponseFileSearchToolCall) ToParam() ResponseFileSearchToolCallParam {
	return param.OverrideObj[ResponseFileSearchToolCallParam](r.RawJSON())
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Attributes  resp.Field
		FileID      resp.Field
		Filename    resp.Field
		Score       resp.Field
		Text        resp.Field
		ExtraFields map[string]resp.Field
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
		OfString resp.Field
		OfFloat  resp.Field
		OfBool   resp.Field
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseFileSearchToolCallParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r ResponseFileSearchToolCallParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseFileSearchToolCallParam
	return param.MarshalObject(r, (*shadow)(&r))
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseFileSearchToolCallResultParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseFileSearchToolCallResultParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseFileSearchToolCallResultParam
	return param.MarshalObject(r, (*shadow)(&r))
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (u ResponseFileSearchToolCallResultAttributeUnionParam) IsPresent() bool {
	return !param.IsOmitted(u) && !u.IsNull()
}
func (u ResponseFileSearchToolCallResultAttributeUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion[ResponseFileSearchToolCallResultAttributeUnionParam](u.OfString, u.OfFloat, u.OfBool)
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
	Schema map[string]interface{} `json:"schema"`
	// This field is from variant [ResponseFormatTextJSONSchemaConfig].
	Description string `json:"description"`
	// This field is from variant [ResponseFormatTextJSONSchemaConfig].
	Name string `json:"name"`
	// This field is from variant [ResponseFormatTextJSONSchemaConfig].
	Strict bool `json:"strict"`
	JSON   struct {
		Type        resp.Field
		Schema      resp.Field
		Description resp.Field
		Name        resp.Field
		Strict      resp.Field
		raw         string
	} `json:"-"`
}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseFormatTextConfigUnion.AsAny().(type) {
//	case shared.ResponseFormatText:
//	case ResponseFormatTextJSONSchemaConfig:
//	case shared.ResponseFormatJSONObject:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseFormatTextConfigUnion) AsAny() any {
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
// ResponseFormatTextConfigUnionParam.IsOverridden()
func (r ResponseFormatTextConfigUnion) ToParam() ResponseFormatTextConfigUnionParam {
	return param.OverrideObj[ResponseFormatTextConfigUnionParam](r.RawJSON())
}

func ResponseFormatTextConfigParamOfJSONSchema(schema map[string]interface{}) ResponseFormatTextConfigUnionParam {
	var jsonSchema ResponseFormatTextJSONSchemaConfigParam
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (u ResponseFormatTextConfigUnionParam) IsPresent() bool {
	return !param.IsOmitted(u) && !u.IsNull()
}
func (u ResponseFormatTextConfigUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion[ResponseFormatTextConfigUnionParam](u.OfText, u.OfJSONSchema, u.OfJSONObject)
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
func (u ResponseFormatTextConfigUnionParam) GetSchema() map[string]interface{} {
	if vt := u.OfJSONSchema; vt != nil {
		return vt.Schema
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseFormatTextConfigUnionParam) GetDescription() *string {
	if vt := u.OfJSONSchema; vt != nil && vt.Description.IsPresent() {
		return &vt.Description.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseFormatTextConfigUnionParam) GetName() *string {
	if vt := u.OfJSONSchema; vt != nil && vt.Name.IsPresent() {
		return &vt.Name.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseFormatTextConfigUnionParam) GetStrict() *bool {
	if vt := u.OfJSONSchema; vt != nil && vt.Strict.IsPresent() {
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
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(shared.ResponseFormatTextParam{}),
			DiscriminatorValue: "text",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseFormatTextJSONSchemaConfigParam{}),
			DiscriminatorValue: "json_schema",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(shared.ResponseFormatJSONObjectParam{}),
			DiscriminatorValue: "json_object",
		},
	)
}

// JSON Schema response format. Used to generate structured JSON responses. Learn
// more about
// [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs).
type ResponseFormatTextJSONSchemaConfig struct {
	// The schema for the response format, described as a JSON Schema object. Learn how
	// to build JSON schemas [here](https://json-schema.org/).
	Schema map[string]interface{} `json:"schema,required"`
	// The type of response format being defined. Always `json_schema`.
	Type constant.JSONSchema `json:"type,required"`
	// A description of what the response format is for, used by the model to determine
	// how to respond in the format.
	Description string `json:"description"`
	// The name of the response format. Must be a-z, A-Z, 0-9, or contain underscores
	// and dashes, with a maximum length of 64.
	Name string `json:"name"`
	// Whether to enable strict schema adherence when generating the output. If set to
	// true, the model will always follow the exact schema defined in the `schema`
	// field. Only a subset of JSON Schema is supported when `strict` is `true`. To
	// learn more, read the
	// [Structured Outputs guide](https://platform.openai.com/docs/guides/structured-outputs).
	Strict bool `json:"strict,nullable"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Schema      resp.Field
		Type        resp.Field
		Description resp.Field
		Name        resp.Field
		Strict      resp.Field
		ExtraFields map[string]resp.Field
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
// ResponseFormatTextJSONSchemaConfigParam.IsOverridden()
func (r ResponseFormatTextJSONSchemaConfig) ToParam() ResponseFormatTextJSONSchemaConfigParam {
	return param.OverrideObj[ResponseFormatTextJSONSchemaConfigParam](r.RawJSON())
}

// JSON Schema response format. Used to generate structured JSON responses. Learn
// more about
// [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs).
//
// The properties Schema, Type are required.
type ResponseFormatTextJSONSchemaConfigParam struct {
	// The schema for the response format, described as a JSON Schema object. Learn how
	// to build JSON schemas [here](https://json-schema.org/).
	Schema map[string]interface{} `json:"schema,omitzero,required"`
	// Whether to enable strict schema adherence when generating the output. If set to
	// true, the model will always follow the exact schema defined in the `schema`
	// field. Only a subset of JSON Schema is supported when `strict` is `true`. To
	// learn more, read the
	// [Structured Outputs guide](https://platform.openai.com/docs/guides/structured-outputs).
	Strict param.Opt[bool] `json:"strict,omitzero"`
	// A description of what the response format is for, used by the model to determine
	// how to respond in the format.
	Description param.Opt[string] `json:"description,omitzero"`
	// The name of the response format. Must be a-z, A-Z, 0-9, or contain underscores
	// and dashes, with a maximum length of 64.
	Name param.Opt[string] `json:"name,omitzero"`
	// The type of response format being defined. Always `json_schema`.
	//
	// This field can be elided, and will marshal its zero value as "json_schema".
	Type constant.JSONSchema `json:"type,required"`
	paramObj
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseFormatTextJSONSchemaConfigParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseFormatTextJSONSchemaConfigParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseFormatTextJSONSchemaConfigParam
	return param.MarshalObject(r, (*shadow)(&r))
}

// Emitted when there is a partial function-call arguments delta.
type ResponseFunctionCallArgumentsDeltaEvent struct {
	// The function-call arguments delta that is added.
	Delta string `json:"delta,required"`
	// The ID of the output item that the function-call arguments delta is added to.
	ItemID string `json:"item_id,required"`
	// The index of the output item that the function-call arguments delta is added to.
	OutputIndex int64 `json:"output_index,required"`
	// The type of the event. Always `response.function_call_arguments.delta`.
	Type constant.ResponseFunctionCallArgumentsDelta `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Delta       resp.Field
		ItemID      resp.Field
		OutputIndex resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
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
	OutputIndex int64                                      `json:"output_index,required"`
	Type        constant.ResponseFunctionCallArgumentsDone `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Arguments   resp.Field
		ItemID      resp.Field
		OutputIndex resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Arguments   resp.Field
		CallID      resp.Field
		Name        resp.Field
		Type        resp.Field
		ID          resp.Field
		Status      resp.Field
		ExtraFields map[string]resp.Field
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
// ResponseFunctionToolCallParam.IsOverridden()
func (r ResponseFunctionToolCall) ToParam() ResponseFunctionToolCallParam {
	return param.OverrideObj[ResponseFunctionToolCallParam](r.RawJSON())
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseFunctionToolCallParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r ResponseFunctionToolCallParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseFunctionToolCallParam
	return param.MarshalObject(r, (*shadow)(&r))
}

// A tool call to run a function. See the
// [function calling guide](https://platform.openai.com/docs/guides/function-calling)
// for more information.
type ResponseFunctionToolCallItem struct {
	// The unique ID of the function tool call.
	ID string `json:"id,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ID          resp.Field
		ExtraFields map[string]resp.Field
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ID          resp.Field
		CallID      resp.Field
		Output      resp.Field
		Type        resp.Field
		Status      resp.Field
		ExtraFields map[string]resp.Field
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
	// The status of the web search tool call.
	//
	// Any of "in_progress", "searching", "completed", "failed".
	Status ResponseFunctionWebSearchStatus `json:"status,required"`
	// The type of the web search tool call. Always `web_search_call`.
	Type constant.WebSearchCall `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ID          resp.Field
		Status      resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
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
// ResponseFunctionWebSearchParam.IsOverridden()
func (r ResponseFunctionWebSearch) ToParam() ResponseFunctionWebSearchParam {
	return param.OverrideObj[ResponseFunctionWebSearchParam](r.RawJSON())
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
// The properties ID, Status, Type are required.
type ResponseFunctionWebSearchParam struct {
	// The unique ID of the web search tool call.
	ID string `json:"id,required"`
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseFunctionWebSearchParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r ResponseFunctionWebSearchParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseFunctionWebSearchParam
	return param.MarshalObject(r, (*shadow)(&r))
}

// Emitted when the response is in progress.
type ResponseInProgressEvent struct {
	// The response that is in progress.
	Response Response `json:"response,required"`
	// The type of the event. Always `response.in_progress`.
	Type constant.ResponseInProgress `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Response    resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
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
//   - `file_search_call.results`: Include the search results of the file search tool
//     call.
//   - `message.input_image.image_url`: Include image urls from the input message.
//   - `computer_call_output.output.image_url`: Include image urls from the computer
//     call output.
type ResponseIncludable string

const (
	ResponseIncludableFileSearchCallResults            ResponseIncludable = "file_search_call.results"
	ResponseIncludableMessageInputImageImageURL        ResponseIncludable = "message.input_image.image_url"
	ResponseIncludableComputerCallOutputOutputImageURL ResponseIncludable = "computer_call_output.output.image_url"
)

// An event that is emitted when a response finishes as incomplete.
type ResponseIncompleteEvent struct {
	// The response that was incomplete.
	Response Response `json:"response,required"`
	// The type of the event. Always `response.incomplete`.
	Type constant.ResponseIncomplete `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Response    resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
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
		Text     resp.Field
		Type     resp.Field
		Detail   resp.Field
		FileID   resp.Field
		ImageURL resp.Field
		FileData resp.Field
		Filename resp.Field
		raw      string
	} `json:"-"`
}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseInputContentUnion.AsAny().(type) {
//	case ResponseInputText:
//	case ResponseInputImage:
//	case ResponseInputFile:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseInputContentUnion) AsAny() any {
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
// ResponseInputContentUnionParam.IsOverridden()
func (r ResponseInputContentUnion) ToParam() ResponseInputContentUnionParam {
	return param.OverrideObj[ResponseInputContentUnionParam](r.RawJSON())
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (u ResponseInputContentUnionParam) IsPresent() bool { return !param.IsOmitted(u) && !u.IsNull() }
func (u ResponseInputContentUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion[ResponseInputContentUnionParam](u.OfInputText, u.OfInputImage, u.OfInputFile)
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
	if vt := u.OfInputImage; vt != nil && vt.ImageURL.IsPresent() {
		return &vt.ImageURL.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputContentUnionParam) GetFileData() *string {
	if vt := u.OfInputFile; vt != nil && vt.FileData.IsPresent() {
		return &vt.FileData.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputContentUnionParam) GetFilename() *string {
	if vt := u.OfInputFile; vt != nil && vt.Filename.IsPresent() {
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
	if vt := u.OfInputImage; vt != nil && vt.FileID.IsPresent() {
		return &vt.FileID.Value
	} else if vt := u.OfInputFile; vt != nil && vt.FileID.IsPresent() {
		return &vt.FileID.Value
	}
	return nil
}

func init() {
	apijson.RegisterUnion[ResponseInputContentUnionParam](
		"type",
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseInputTextParam{}),
			DiscriminatorValue: "input_text",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseInputImageParam{}),
			DiscriminatorValue: "input_image",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseInputFileParam{}),
			DiscriminatorValue: "input_file",
		},
	)
}

// A file input to the model.
type ResponseInputFile struct {
	// The type of the input item. Always `input_file`.
	Type constant.InputFile `json:"type,required"`
	// The content of the file to be sent to the model.
	FileData string `json:"file_data"`
	// The ID of the file to be sent to the model.
	FileID string `json:"file_id"`
	// The name of the file to be sent to the model.
	Filename string `json:"filename"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Type        resp.Field
		FileData    resp.Field
		FileID      resp.Field
		Filename    resp.Field
		ExtraFields map[string]resp.Field
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
// ResponseInputFileParam.IsOverridden()
func (r ResponseInputFile) ToParam() ResponseInputFileParam {
	return param.OverrideObj[ResponseInputFileParam](r.RawJSON())
}

// A file input to the model.
//
// The property Type is required.
type ResponseInputFileParam struct {
	// The content of the file to be sent to the model.
	FileData param.Opt[string] `json:"file_data,omitzero"`
	// The ID of the file to be sent to the model.
	FileID param.Opt[string] `json:"file_id,omitzero"`
	// The name of the file to be sent to the model.
	Filename param.Opt[string] `json:"filename,omitzero"`
	// The type of the input item. Always `input_file`.
	//
	// This field can be elided, and will marshal its zero value as "input_file".
	Type constant.InputFile `json:"type,required"`
	paramObj
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseInputFileParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r ResponseInputFileParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputFileParam
	return param.MarshalObject(r, (*shadow)(&r))
}

// An image input to the model. Learn about
// [image inputs](https://platform.openai.com/docs/guides/vision).
type ResponseInputImage struct {
	// The detail level of the image to be sent to the model. One of `high`, `low`, or
	// `auto`. Defaults to `auto`.
	//
	// Any of "high", "low", "auto".
	Detail ResponseInputImageDetail `json:"detail,required"`
	// The type of the input item. Always `input_image`.
	Type constant.InputImage `json:"type,required"`
	// The ID of the file to be sent to the model.
	FileID string `json:"file_id,nullable"`
	// The URL of the image to be sent to the model. A fully qualified URL or base64
	// encoded image in a data URL.
	ImageURL string `json:"image_url,nullable"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Detail      resp.Field
		Type        resp.Field
		FileID      resp.Field
		ImageURL    resp.Field
		ExtraFields map[string]resp.Field
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
// ResponseInputImageParam.IsOverridden()
func (r ResponseInputImage) ToParam() ResponseInputImageParam {
	return param.OverrideObj[ResponseInputImageParam](r.RawJSON())
}

// The detail level of the image to be sent to the model. One of `high`, `low`, or
// `auto`. Defaults to `auto`.
type ResponseInputImageDetail string

const (
	ResponseInputImageDetailHigh ResponseInputImageDetail = "high"
	ResponseInputImageDetailLow  ResponseInputImageDetail = "low"
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
	// Any of "high", "low", "auto".
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseInputImageParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r ResponseInputImageParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputImageParam
	return param.MarshalObject(r, (*shadow)(&r))
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

func ResponseInputItemParamOfWebSearchCall(id string, status ResponseFunctionWebSearchStatus) ResponseInputItemUnionParam {
	var webSearchCall ResponseFunctionWebSearchParam
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

func ResponseInputItemParamOfItemReference(id string) ResponseInputItemUnionParam {
	var itemReference ResponseInputItemItemReferenceParam
	itemReference.ID = id
	return ResponseInputItemUnionParam{OfItemReference: &itemReference}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ResponseInputItemUnionParam struct {
	OfMessage            *EasyInputMessageParam                    `json:",omitzero,inline"`
	OfInputMessage       *ResponseInputItemMessageParam            `json:",omitzero,inline"`
	OfOutputMessage      *ResponseOutputMessageParam               `json:",omitzero,inline"`
	OfFileSearchCall     *ResponseFileSearchToolCallParam          `json:",omitzero,inline"`
	OfComputerCall       *ResponseComputerToolCallParam            `json:",omitzero,inline"`
	OfComputerCallOutput *ResponseInputItemComputerCallOutputParam `json:",omitzero,inline"`
	OfWebSearchCall      *ResponseFunctionWebSearchParam           `json:",omitzero,inline"`
	OfFunctionCall       *ResponseFunctionToolCallParam            `json:",omitzero,inline"`
	OfFunctionCallOutput *ResponseInputItemFunctionCallOutputParam `json:",omitzero,inline"`
	OfReasoning          *ResponseReasoningItemParam               `json:",omitzero,inline"`
	OfItemReference      *ResponseInputItemItemReferenceParam      `json:",omitzero,inline"`
	paramUnion
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (u ResponseInputItemUnionParam) IsPresent() bool { return !param.IsOmitted(u) && !u.IsNull() }
func (u ResponseInputItemUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion[ResponseInputItemUnionParam](u.OfMessage,
		u.OfInputMessage,
		u.OfOutputMessage,
		u.OfFileSearchCall,
		u.OfComputerCall,
		u.OfComputerCallOutput,
		u.OfWebSearchCall,
		u.OfFunctionCall,
		u.OfFunctionCallOutput,
		u.OfReasoning,
		u.OfItemReference)
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
func (u ResponseInputItemUnionParam) GetAction() *ResponseComputerToolCallActionUnionParam {
	if vt := u.OfComputerCall; vt != nil {
		return &vt.Action
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
func (u ResponseInputItemUnionParam) GetArguments() *string {
	if vt := u.OfFunctionCall; vt != nil {
		return &vt.Arguments
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseInputItemUnionParam) GetName() *string {
	if vt := u.OfFunctionCall; vt != nil {
		return &vt.Name
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
	} else if vt := u.OfComputerCallOutput; vt != nil && vt.ID.IsPresent() {
		return &vt.ID.Value
	} else if vt := u.OfWebSearchCall; vt != nil {
		return (*string)(&vt.ID)
	} else if vt := u.OfFunctionCall; vt != nil && vt.ID.IsPresent() {
		return &vt.ID.Value
	} else if vt := u.OfFunctionCallOutput; vt != nil && vt.ID.IsPresent() {
		return &vt.ID.Value
	} else if vt := u.OfReasoning; vt != nil {
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
	}
	return nil
}

// Returns a subunion which exports methods to access subproperties
//
// Or use AsAny() to get the underlying value
func (u ResponseInputItemUnionParam) GetContent() (res responseInputItemUnionParamContent) {
	if vt := u.OfMessage; vt != nil {
		res.ofEasyInputMessageContentUnion = &vt.Content
	} else if vt := u.OfInputMessage; vt != nil {
		res.ofResponseInputMessageContentList = &vt.Content
	} else if vt := u.OfOutputMessage; vt != nil {
		res.ofResponseOutputMessageContent = &vt.Content
	}
	return
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type responseInputItemUnionParamContent struct {
	ofEasyInputMessageContentUnion    *EasyInputMessageContentUnionParam
	ofResponseInputMessageContentList *ResponseInputMessageContentListParam
	ofResponseOutputMessageContent    *[]ResponseOutputMessageContentUnionParam
}

// Use the following switch statement to get the type of the union:
//
//	switch u.AsAny().(type) {
//	case *string:
//	case *responses.ResponseInputMessageContentListParam:
//	case *[]responses.ResponseOutputMessageContentUnionParam:
//	default:
//	    fmt.Errorf("not present")
//	}
func (u responseInputItemUnionParamContent) AsAny() any {
	if !param.IsOmitted(u.ofEasyInputMessageContentUnion) {
		return u.ofEasyInputMessageContentUnion.asAny()
	} else if !param.IsOmitted(u.ofResponseInputMessageContentList) {
		return u.ofResponseInputMessageContentList
	} else if !param.IsOmitted(u.ofResponseOutputMessageContent) {
		return u.ofResponseOutputMessageContent
	}
	return nil
}

// Returns a subunion which exports methods to access subproperties
//
// Or use AsAny() to get the underlying value
func (u ResponseInputItemUnionParam) GetOutput() (res responseInputItemUnionParamOutput) {
	if vt := u.OfComputerCallOutput; vt != nil {
		res.ofResponseComputerToolCallOutputScreenshot = &vt.Output
	} else if vt := u.OfFunctionCallOutput; vt != nil {
		res.ofString = &vt.Output
	}
	return
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type responseInputItemUnionParamOutput struct {
	ofResponseComputerToolCallOutputScreenshot *ResponseComputerToolCallOutputScreenshotParam
	ofString                                   *string
}

// Use the following switch statement to get the type of the union:
//
//	switch u.AsAny().(type) {
//	case *responses.ResponseComputerToolCallOutputScreenshotParam:
//	case *string:
//	default:
//	    fmt.Errorf("not present")
//	}
func (u responseInputItemUnionParamOutput) AsAny() any {
	if !param.IsOmitted(u.ofResponseComputerToolCallOutputScreenshot) {
		return u.ofResponseComputerToolCallOutputScreenshot
	} else if !param.IsOmitted(u.ofString) {
		return u.ofString
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamOutput) GetType() *string {
	if vt := u.ofResponseComputerToolCallOutputScreenshot; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamOutput) GetFileID() *string {
	if vt := u.ofResponseComputerToolCallOutputScreenshot; vt != nil && vt.FileID.IsPresent() {
		return &vt.FileID.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u responseInputItemUnionParamOutput) GetImageURL() *string {
	if vt := u.ofResponseComputerToolCallOutputScreenshot; vt != nil && vt.ImageURL.IsPresent() {
		return &vt.ImageURL.Value
	}
	return nil
}

func init() {
	apijson.RegisterUnion[ResponseInputItemUnionParam](
		"type",
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(EasyInputMessageParam{}),
			DiscriminatorValue: "message",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseInputItemMessageParam{}),
			DiscriminatorValue: "message",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseOutputMessageParam{}),
			DiscriminatorValue: "message",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseFileSearchToolCallParam{}),
			DiscriminatorValue: "file_search_call",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseComputerToolCallParam{}),
			DiscriminatorValue: "computer_call",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseInputItemComputerCallOutputParam{}),
			DiscriminatorValue: "computer_call_output",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseFunctionWebSearchParam{}),
			DiscriminatorValue: "web_search_call",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseFunctionToolCallParam{}),
			DiscriminatorValue: "function_call",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseInputItemFunctionCallOutputParam{}),
			DiscriminatorValue: "function_call_output",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseReasoningItemParam{}),
			DiscriminatorValue: "reasoning",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseInputItemItemReferenceParam{}),
			DiscriminatorValue: "item_reference",
		},
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseInputItemMessageParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r ResponseInputItemMessageParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputItemMessageParam
	return param.MarshalObject(r, (*shadow)(&r))
}

func init() {
	apijson.RegisterFieldValidator[ResponseInputItemMessageParam](
		"Role", false, "user", "system", "developer",
	)
	apijson.RegisterFieldValidator[ResponseInputItemMessageParam](
		"Status", false, "in_progress", "completed", "incomplete",
	)
	apijson.RegisterFieldValidator[ResponseInputItemMessageParam](
		"Type", false, "message",
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseInputItemComputerCallOutputParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseInputItemComputerCallOutputParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputItemComputerCallOutputParam
	return param.MarshalObject(r, (*shadow)(&r))
}

func init() {
	apijson.RegisterFieldValidator[ResponseInputItemComputerCallOutputParam](
		"Status", false, "in_progress", "completed", "incomplete",
	)
}

// A pending safety check for the computer call.
//
// The properties ID, Code, Message are required.
type ResponseInputItemComputerCallOutputAcknowledgedSafetyCheckParam struct {
	// The ID of the pending safety check.
	ID string `json:"id,required"`
	// The type of the pending safety check.
	Code string `json:"code,required"`
	// Details about the pending safety check.
	Message string `json:"message,required"`
	paramObj
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseInputItemComputerCallOutputAcknowledgedSafetyCheckParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseInputItemComputerCallOutputAcknowledgedSafetyCheckParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputItemComputerCallOutputAcknowledgedSafetyCheckParam
	return param.MarshalObject(r, (*shadow)(&r))
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseInputItemFunctionCallOutputParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseInputItemFunctionCallOutputParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputItemFunctionCallOutputParam
	return param.MarshalObject(r, (*shadow)(&r))
}

func init() {
	apijson.RegisterFieldValidator[ResponseInputItemFunctionCallOutputParam](
		"Status", false, "in_progress", "completed", "incomplete",
	)
}

// An internal identifier for an item to reference.
//
// The properties ID, Type are required.
type ResponseInputItemItemReferenceParam struct {
	// The ID of the item to reference.
	ID string `json:"id,required"`
	// The type of item to reference. Always `item_reference`.
	//
	// This field can be elided, and will marshal its zero value as "item_reference".
	Type constant.ItemReference `json:"type,required"`
	paramObj
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseInputItemItemReferenceParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseInputItemItemReferenceParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputItemItemReferenceParam
	return param.MarshalObject(r, (*shadow)(&r))
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ID          resp.Field
		Content     resp.Field
		Role        resp.Field
		Status      resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Text        resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
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
// ResponseInputTextParam.IsOverridden()
func (r ResponseInputText) ToParam() ResponseInputTextParam {
	return param.OverrideObj[ResponseInputTextParam](r.RawJSON())
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseInputTextParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r ResponseInputTextParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseInputTextParam
	return param.MarshalObject(r, (*shadow)(&r))
}

// ResponseItemUnion contains all possible properties and values from
// [ResponseInputMessageItem], [ResponseOutputMessage],
// [ResponseFileSearchToolCall], [ResponseComputerToolCall],
// [ResponseComputerToolCallOutputItem], [ResponseFunctionWebSearch],
// [ResponseFunctionToolCallItem], [ResponseFunctionToolCallOutputItem].
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
	// "function_call_output".
	Type string `json:"type"`
	// This field is from variant [ResponseFileSearchToolCall].
	Queries []string `json:"queries"`
	// This field is from variant [ResponseFileSearchToolCall].
	Results []ResponseFileSearchToolCallResult `json:"results"`
	// This field is from variant [ResponseComputerToolCall].
	Action ResponseComputerToolCallActionUnion `json:"action"`
	CallID string                              `json:"call_id"`
	// This field is from variant [ResponseComputerToolCall].
	PendingSafetyChecks []ResponseComputerToolCallPendingSafetyCheck `json:"pending_safety_checks"`
	// This field is a union of [ResponseComputerToolCallOutputScreenshot], [string]
	Output ResponseItemUnionOutput `json:"output"`
	// This field is from variant [ResponseComputerToolCallOutputItem].
	AcknowledgedSafetyChecks []ResponseComputerToolCallOutputItemAcknowledgedSafetyCheck `json:"acknowledged_safety_checks"`
	// This field is from variant [ResponseFunctionToolCallItem].
	Arguments string `json:"arguments"`
	// This field is from variant [ResponseFunctionToolCallItem].
	Name string `json:"name"`
	JSON struct {
		ID                       resp.Field
		Content                  resp.Field
		Role                     resp.Field
		Status                   resp.Field
		Type                     resp.Field
		Queries                  resp.Field
		Results                  resp.Field
		Action                   resp.Field
		CallID                   resp.Field
		PendingSafetyChecks      resp.Field
		Output                   resp.Field
		AcknowledgedSafetyChecks resp.Field
		Arguments                resp.Field
		Name                     resp.Field
		raw                      string
	} `json:"-"`
}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseItemUnion.AsAny().(type) {
//	case ResponseInputMessageItem:
//	case ResponseOutputMessage:
//	case ResponseFileSearchToolCall:
//	case ResponseComputerToolCall:
//	case ResponseComputerToolCallOutputItem:
//	case ResponseFunctionWebSearch:
//	case ResponseFunctionToolCallItem:
//	case ResponseFunctionToolCallOutputItem:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseItemUnion) AsAny() any {
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
// will be valid: OfInputItemContentList OfResponseOutputMessageContent]
type ResponseItemUnionContent struct {
	// This field will be present if the value is a [ResponseInputMessageContentList]
	// instead of an object.
	OfInputItemContentList ResponseInputMessageContentList `json:",inline"`
	// This field will be present if the value is a
	// [[]ResponseOutputMessageContentUnion] instead of an object.
	OfResponseOutputMessageContent []ResponseOutputMessageContentUnion `json:",inline"`
	JSON                           struct {
		OfInputItemContentList         resp.Field
		OfResponseOutputMessageContent resp.Field
		raw                            string
	} `json:"-"`
}

func (r *ResponseItemUnionContent) UnmarshalJSON(data []byte) error {
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
		OfString resp.Field
		Type     resp.Field
		FileID   resp.Field
		ImageURL resp.Field
		raw      string
	} `json:"-"`
}

func (r *ResponseItemUnionOutput) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseOutputItemUnion contains all possible properties and values from
// [ResponseOutputMessage], [ResponseFileSearchToolCall],
// [ResponseFunctionToolCall], [ResponseFunctionWebSearch],
// [ResponseComputerToolCall], [ResponseReasoningItem].
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
	// "computer_call", "reasoning".
	Type string `json:"type"`
	// This field is from variant [ResponseFileSearchToolCall].
	Queries []string `json:"queries"`
	// This field is from variant [ResponseFileSearchToolCall].
	Results []ResponseFileSearchToolCallResult `json:"results"`
	// This field is from variant [ResponseFunctionToolCall].
	Arguments string `json:"arguments"`
	CallID    string `json:"call_id"`
	// This field is from variant [ResponseFunctionToolCall].
	Name string `json:"name"`
	// This field is from variant [ResponseComputerToolCall].
	Action ResponseComputerToolCallActionUnion `json:"action"`
	// This field is from variant [ResponseComputerToolCall].
	PendingSafetyChecks []ResponseComputerToolCallPendingSafetyCheck `json:"pending_safety_checks"`
	// This field is from variant [ResponseReasoningItem].
	Summary []ResponseReasoningItemSummary `json:"summary"`
	JSON    struct {
		ID                  resp.Field
		Content             resp.Field
		Role                resp.Field
		Status              resp.Field
		Type                resp.Field
		Queries             resp.Field
		Results             resp.Field
		Arguments           resp.Field
		CallID              resp.Field
		Name                resp.Field
		Action              resp.Field
		PendingSafetyChecks resp.Field
		Summary             resp.Field
		raw                 string
	} `json:"-"`
}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseOutputItemUnion.AsAny().(type) {
//	case ResponseOutputMessage:
//	case ResponseFileSearchToolCall:
//	case ResponseFunctionToolCall:
//	case ResponseFunctionWebSearch:
//	case ResponseComputerToolCall:
//	case ResponseReasoningItem:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseOutputItemUnion) AsAny() any {
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

// Returns the unmodified JSON received from the API
func (u ResponseOutputItemUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseOutputItemUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a new output item is added.
type ResponseOutputItemAddedEvent struct {
	// The output item that was added.
	Item ResponseOutputItemUnion `json:"item,required"`
	// The index of the output item that was added.
	OutputIndex int64 `json:"output_index,required"`
	// The type of the event. Always `response.output_item.added`.
	Type constant.ResponseOutputItemAdded `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Item        resp.Field
		OutputIndex resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
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
	// The type of the event. Always `response.output_item.done`.
	Type constant.ResponseOutputItemDone `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Item        resp.Field
		OutputIndex resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ID          resp.Field
		Content     resp.Field
		Role        resp.Field
		Status      resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
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
// ResponseOutputMessageParam.IsOverridden()
func (r ResponseOutputMessage) ToParam() ResponseOutputMessageParam {
	return param.OverrideObj[ResponseOutputMessageParam](r.RawJSON())
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
	// This field is from variant [ResponseOutputRefusal].
	Refusal string `json:"refusal"`
	JSON    struct {
		Annotations resp.Field
		Text        resp.Field
		Type        resp.Field
		Refusal     resp.Field
		raw         string
	} `json:"-"`
}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseOutputMessageContentUnion.AsAny().(type) {
//	case ResponseOutputText:
//	case ResponseOutputRefusal:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseOutputMessageContentUnion) AsAny() any {
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
	ID string `json:"id,required"`
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseOutputMessageParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r ResponseOutputMessageParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseOutputMessageParam
	return param.MarshalObject(r, (*shadow)(&r))
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ResponseOutputMessageContentUnionParam struct {
	OfOutputText *ResponseOutputTextParam    `json:",omitzero,inline"`
	OfRefusal    *ResponseOutputRefusalParam `json:",omitzero,inline"`
	paramUnion
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (u ResponseOutputMessageContentUnionParam) IsPresent() bool {
	return !param.IsOmitted(u) && !u.IsNull()
}
func (u ResponseOutputMessageContentUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion[ResponseOutputMessageContentUnionParam](u.OfOutputText, u.OfRefusal)
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
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseOutputTextParam{}),
			DiscriminatorValue: "output_text",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseOutputRefusalParam{}),
			DiscriminatorValue: "refusal",
		},
	)
}

// A refusal from the model.
type ResponseOutputRefusal struct {
	// The refusal explanationfrom the model.
	Refusal string `json:"refusal,required"`
	// The type of the refusal. Always `refusal`.
	Type constant.Refusal `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Refusal     resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
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
// ResponseOutputRefusalParam.IsOverridden()
func (r ResponseOutputRefusal) ToParam() ResponseOutputRefusalParam {
	return param.OverrideObj[ResponseOutputRefusalParam](r.RawJSON())
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseOutputRefusalParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r ResponseOutputRefusalParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseOutputRefusalParam
	return param.MarshalObject(r, (*shadow)(&r))
}

// A text output from the model.
type ResponseOutputText struct {
	// The annotations of the text output.
	Annotations []ResponseOutputTextAnnotationUnion `json:"annotations,required"`
	// The text output from the model.
	Text string `json:"text,required"`
	// The type of the output text. Always `output_text`.
	Type constant.OutputText `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Annotations resp.Field
		Text        resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
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
// ResponseOutputTextParam.IsOverridden()
func (r ResponseOutputText) ToParam() ResponseOutputTextParam {
	return param.OverrideObj[ResponseOutputTextParam](r.RawJSON())
}

// ResponseOutputTextAnnotationUnion contains all possible properties and values
// from [ResponseOutputTextAnnotationFileCitation],
// [ResponseOutputTextAnnotationURLCitation],
// [ResponseOutputTextAnnotationFilePath].
//
// Use the [ResponseOutputTextAnnotationUnion.AsAny] method to switch on the
// variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ResponseOutputTextAnnotationUnion struct {
	FileID string `json:"file_id"`
	Index  int64  `json:"index"`
	// Any of "file_citation", "url_citation", "file_path".
	Type string `json:"type"`
	// This field is from variant [ResponseOutputTextAnnotationURLCitation].
	EndIndex int64 `json:"end_index"`
	// This field is from variant [ResponseOutputTextAnnotationURLCitation].
	StartIndex int64 `json:"start_index"`
	// This field is from variant [ResponseOutputTextAnnotationURLCitation].
	Title string `json:"title"`
	// This field is from variant [ResponseOutputTextAnnotationURLCitation].
	URL  string `json:"url"`
	JSON struct {
		FileID     resp.Field
		Index      resp.Field
		Type       resp.Field
		EndIndex   resp.Field
		StartIndex resp.Field
		Title      resp.Field
		URL        resp.Field
		raw        string
	} `json:"-"`
}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseOutputTextAnnotationUnion.AsAny().(type) {
//	case ResponseOutputTextAnnotationFileCitation:
//	case ResponseOutputTextAnnotationURLCitation:
//	case ResponseOutputTextAnnotationFilePath:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseOutputTextAnnotationUnion) AsAny() any {
	switch u.Type {
	case "file_citation":
		return u.AsFileCitation()
	case "url_citation":
		return u.AsURLCitation()
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
	// The index of the file in the list of files.
	Index int64 `json:"index,required"`
	// The type of the file citation. Always `file_citation`.
	Type constant.FileCitation `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		FileID      resp.Field
		Index       resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		EndIndex    resp.Field
		StartIndex  resp.Field
		Title       resp.Field
		Type        resp.Field
		URL         resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputTextAnnotationURLCitation) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputTextAnnotationURLCitation) UnmarshalJSON(data []byte) error {
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		FileID      resp.Field
		Index       resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseOutputTextAnnotationFilePath) RawJSON() string { return r.JSON.raw }
func (r *ResponseOutputTextAnnotationFilePath) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A text output from the model.
//
// The properties Annotations, Text, Type are required.
type ResponseOutputTextParam struct {
	// The annotations of the text output.
	Annotations []ResponseOutputTextAnnotationUnionParam `json:"annotations,omitzero,required"`
	// The text output from the model.
	Text string `json:"text,required"`
	// The type of the output text. Always `output_text`.
	//
	// This field can be elided, and will marshal its zero value as "output_text".
	Type constant.OutputText `json:"type,required"`
	paramObj
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseOutputTextParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r ResponseOutputTextParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseOutputTextParam
	return param.MarshalObject(r, (*shadow)(&r))
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ResponseOutputTextAnnotationUnionParam struct {
	OfFileCitation *ResponseOutputTextAnnotationFileCitationParam `json:",omitzero,inline"`
	OfURLCitation  *ResponseOutputTextAnnotationURLCitationParam  `json:",omitzero,inline"`
	OfFilePath     *ResponseOutputTextAnnotationFilePathParam     `json:",omitzero,inline"`
	paramUnion
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (u ResponseOutputTextAnnotationUnionParam) IsPresent() bool {
	return !param.IsOmitted(u) && !u.IsNull()
}
func (u ResponseOutputTextAnnotationUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion[ResponseOutputTextAnnotationUnionParam](u.OfFileCitation, u.OfURLCitation, u.OfFilePath)
}

func (u *ResponseOutputTextAnnotationUnionParam) asAny() any {
	if !param.IsOmitted(u.OfFileCitation) {
		return u.OfFileCitation
	} else if !param.IsOmitted(u.OfURLCitation) {
		return u.OfURLCitation
	} else if !param.IsOmitted(u.OfFilePath) {
		return u.OfFilePath
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseOutputTextAnnotationUnionParam) GetEndIndex() *int64 {
	if vt := u.OfURLCitation; vt != nil {
		return &vt.EndIndex
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseOutputTextAnnotationUnionParam) GetStartIndex() *int64 {
	if vt := u.OfURLCitation; vt != nil {
		return &vt.StartIndex
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
func (u ResponseOutputTextAnnotationUnionParam) GetFileID() *string {
	if vt := u.OfFileCitation; vt != nil {
		return (*string)(&vt.FileID)
	} else if vt := u.OfFilePath; vt != nil {
		return (*string)(&vt.FileID)
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
	} else if vt := u.OfFilePath; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[ResponseOutputTextAnnotationUnionParam](
		"type",
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseOutputTextAnnotationFileCitationParam{}),
			DiscriminatorValue: "file_citation",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseOutputTextAnnotationURLCitationParam{}),
			DiscriminatorValue: "url_citation",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ResponseOutputTextAnnotationFilePathParam{}),
			DiscriminatorValue: "file_path",
		},
	)
}

// A citation to a file.
//
// The properties FileID, Index, Type are required.
type ResponseOutputTextAnnotationFileCitationParam struct {
	// The ID of the file.
	FileID string `json:"file_id,required"`
	// The index of the file in the list of files.
	Index int64 `json:"index,required"`
	// The type of the file citation. Always `file_citation`.
	//
	// This field can be elided, and will marshal its zero value as "file_citation".
	Type constant.FileCitation `json:"type,required"`
	paramObj
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseOutputTextAnnotationFileCitationParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseOutputTextAnnotationFileCitationParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseOutputTextAnnotationFileCitationParam
	return param.MarshalObject(r, (*shadow)(&r))
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseOutputTextAnnotationURLCitationParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseOutputTextAnnotationURLCitationParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseOutputTextAnnotationURLCitationParam
	return param.MarshalObject(r, (*shadow)(&r))
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseOutputTextAnnotationFilePathParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseOutputTextAnnotationFilePathParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseOutputTextAnnotationFilePathParam
	return param.MarshalObject(r, (*shadow)(&r))
}

// A description of the chain of thought used by a reasoning model while generating
// a response.
type ResponseReasoningItem struct {
	// The unique identifier of the reasoning content.
	ID string `json:"id,required"`
	// Reasoning text contents.
	Summary []ResponseReasoningItemSummary `json:"summary,required"`
	// The type of the object. Always `reasoning`.
	Type constant.Reasoning `json:"type,required"`
	// The status of the item. One of `in_progress`, `completed`, or `incomplete`.
	// Populated when items are returned via API.
	//
	// Any of "in_progress", "completed", "incomplete".
	Status ResponseReasoningItemStatus `json:"status"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ID          resp.Field
		Summary     resp.Field
		Type        resp.Field
		Status      resp.Field
		ExtraFields map[string]resp.Field
		raw         string
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
// ResponseReasoningItemParam.IsOverridden()
func (r ResponseReasoningItem) ToParam() ResponseReasoningItemParam {
	return param.OverrideObj[ResponseReasoningItemParam](r.RawJSON())
}

type ResponseReasoningItemSummary struct {
	// A short summary of the reasoning used by the model when generating the response.
	Text string `json:"text,required"`
	// The type of the object. Always `summary_text`.
	Type constant.SummaryText `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Text        resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
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
// a response.
//
// The properties ID, Summary, Type are required.
type ResponseReasoningItemParam struct {
	// The unique identifier of the reasoning content.
	ID string `json:"id,required"`
	// Reasoning text contents.
	Summary []ResponseReasoningItemSummaryParam `json:"summary,omitzero,required"`
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseReasoningItemParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r ResponseReasoningItemParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseReasoningItemParam
	return param.MarshalObject(r, (*shadow)(&r))
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseReasoningItemSummaryParam) IsPresent() bool {
	return !param.IsOmitted(f) && !f.IsNull()
}
func (r ResponseReasoningItemSummaryParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseReasoningItemSummaryParam
	return param.MarshalObject(r, (*shadow)(&r))
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
	// The type of the event. Always `response.refusal.delta`.
	Type constant.ResponseRefusalDelta `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ContentIndex resp.Field
		Delta        resp.Field
		ItemID       resp.Field
		OutputIndex  resp.Field
		Type         resp.Field
		ExtraFields  map[string]resp.Field
		raw          string
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
	// The type of the event. Always `response.refusal.done`.
	Type constant.ResponseRefusalDone `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ContentIndex resp.Field
		ItemID       resp.Field
		OutputIndex  resp.Field
		Refusal      resp.Field
		Type         resp.Field
		ExtraFields  map[string]resp.Field
		raw          string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseRefusalDoneEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseRefusalDoneEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The status of the response generation. One of `completed`, `failed`,
// `in_progress`, or `incomplete`.
type ResponseStatus string

const (
	ResponseStatusCompleted  ResponseStatus = "completed"
	ResponseStatusFailed     ResponseStatus = "failed"
	ResponseStatusInProgress ResponseStatus = "in_progress"
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
// [ResponseRefusalDeltaEvent], [ResponseRefusalDoneEvent],
// [ResponseTextAnnotationDeltaEvent], [ResponseTextDeltaEvent],
// [ResponseTextDoneEvent], [ResponseWebSearchCallCompletedEvent],
// [ResponseWebSearchCallInProgressEvent], [ResponseWebSearchCallSearchingEvent].
//
// Use the [ResponseStreamEventUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ResponseStreamEventUnion struct {
	Delta string `json:"delta"`
	// Any of "response.audio.delta", "response.audio.done",
	// "response.audio.transcript.delta", "response.audio.transcript.done",
	// "response.code_interpreter_call.code.delta",
	// "response.code_interpreter_call.code.done",
	// "response.code_interpreter_call.completed",
	// "response.code_interpreter_call.in_progress",
	// "response.code_interpreter_call.interpreting", "response.completed",
	// "response.content_part.added", "response.content_part.done", "response.created",
	// "error", "response.file_search_call.completed",
	// "response.file_search_call.in_progress", "response.file_search_call.searching",
	// "response.function_call_arguments.delta",
	// "response.function_call_arguments.done", "response.in_progress",
	// "response.failed", "response.incomplete", "response.output_item.added",
	// "response.output_item.done", "response.refusal.delta", "response.refusal.done",
	// "response.output_text.annotation.added", "response.output_text.delta",
	// "response.output_text.done", "response.web_search_call.completed",
	// "response.web_search_call.in_progress", "response.web_search_call.searching".
	Type        string `json:"type"`
	OutputIndex int64  `json:"output_index"`
	Code        string `json:"code"`
	// This field is from variant [ResponseCodeInterpreterCallCompletedEvent].
	CodeInterpreterCall ResponseCodeInterpreterToolCall `json:"code_interpreter_call"`
	// This field is from variant [ResponseCompletedEvent].
	Response     Response `json:"response"`
	ContentIndex int64    `json:"content_index"`
	ItemID       string   `json:"item_id"`
	// This field is a union of [ResponseContentPartAddedEventPartUnion],
	// [ResponseContentPartDoneEventPartUnion]
	Part ResponseStreamEventUnionPart `json:"part"`
	// This field is from variant [ResponseErrorEvent].
	Message string `json:"message"`
	// This field is from variant [ResponseErrorEvent].
	Param string `json:"param"`
	// This field is from variant [ResponseFunctionCallArgumentsDoneEvent].
	Arguments string `json:"arguments"`
	// This field is from variant [ResponseOutputItemAddedEvent].
	Item ResponseOutputItemUnion `json:"item"`
	// This field is from variant [ResponseRefusalDoneEvent].
	Refusal string `json:"refusal"`
	// This field is from variant [ResponseTextAnnotationDeltaEvent].
	Annotation ResponseTextAnnotationDeltaEventAnnotationUnion `json:"annotation"`
	// This field is from variant [ResponseTextAnnotationDeltaEvent].
	AnnotationIndex int64 `json:"annotation_index"`
	// This field is from variant [ResponseTextDoneEvent].
	Text string `json:"text"`
	JSON struct {
		Delta               resp.Field
		Type                resp.Field
		OutputIndex         resp.Field
		Code                resp.Field
		CodeInterpreterCall resp.Field
		Response            resp.Field
		ContentIndex        resp.Field
		ItemID              resp.Field
		Part                resp.Field
		Message             resp.Field
		Param               resp.Field
		Arguments           resp.Field
		Item                resp.Field
		Refusal             resp.Field
		Annotation          resp.Field
		AnnotationIndex     resp.Field
		Text                resp.Field
		raw                 string
	} `json:"-"`
}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseStreamEventUnion.AsAny().(type) {
//	case ResponseAudioDeltaEvent:
//	case ResponseAudioDoneEvent:
//	case ResponseAudioTranscriptDeltaEvent:
//	case ResponseAudioTranscriptDoneEvent:
//	case ResponseCodeInterpreterCallCodeDeltaEvent:
//	case ResponseCodeInterpreterCallCodeDoneEvent:
//	case ResponseCodeInterpreterCallCompletedEvent:
//	case ResponseCodeInterpreterCallInProgressEvent:
//	case ResponseCodeInterpreterCallInterpretingEvent:
//	case ResponseCompletedEvent:
//	case ResponseContentPartAddedEvent:
//	case ResponseContentPartDoneEvent:
//	case ResponseCreatedEvent:
//	case ResponseErrorEvent:
//	case ResponseFileSearchCallCompletedEvent:
//	case ResponseFileSearchCallInProgressEvent:
//	case ResponseFileSearchCallSearchingEvent:
//	case ResponseFunctionCallArgumentsDeltaEvent:
//	case ResponseFunctionCallArgumentsDoneEvent:
//	case ResponseInProgressEvent:
//	case ResponseFailedEvent:
//	case ResponseIncompleteEvent:
//	case ResponseOutputItemAddedEvent:
//	case ResponseOutputItemDoneEvent:
//	case ResponseRefusalDeltaEvent:
//	case ResponseRefusalDoneEvent:
//	case ResponseTextAnnotationDeltaEvent:
//	case ResponseTextDeltaEvent:
//	case ResponseTextDoneEvent:
//	case ResponseWebSearchCallCompletedEvent:
//	case ResponseWebSearchCallInProgressEvent:
//	case ResponseWebSearchCallSearchingEvent:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseStreamEventUnion) AsAny() any {
	switch u.Type {
	case "response.audio.delta":
		return u.AsResponseAudioDelta()
	case "response.audio.done":
		return u.AsResponseAudioDone()
	case "response.audio.transcript.delta":
		return u.AsResponseAudioTranscriptDelta()
	case "response.audio.transcript.done":
		return u.AsResponseAudioTranscriptDone()
	case "response.code_interpreter_call.code.delta":
		return u.AsResponseCodeInterpreterCallCodeDelta()
	case "response.code_interpreter_call.code.done":
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
	case "response.refusal.delta":
		return u.AsResponseRefusalDelta()
	case "response.refusal.done":
		return u.AsResponseRefusalDone()
	case "response.output_text.annotation.added":
		return u.AsResponseOutputTextAnnotationAdded()
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

func (u ResponseStreamEventUnion) AsResponseRefusalDelta() (v ResponseRefusalDeltaEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseRefusalDone() (v ResponseRefusalDoneEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseStreamEventUnion) AsResponseOutputTextAnnotationAdded() (v ResponseTextAnnotationDeltaEvent) {
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

// Returns the unmodified JSON received from the API
func (u ResponseStreamEventUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseStreamEventUnion) UnmarshalJSON(data []byte) error {
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
	// This field is from variant [ResponseContentPartAddedEventPartUnion],
	// [ResponseContentPartDoneEventPartUnion].
	Text string `json:"text"`
	Type string `json:"type"`
	// This field is from variant [ResponseContentPartAddedEventPartUnion],
	// [ResponseContentPartDoneEventPartUnion].
	Refusal string `json:"refusal"`
	JSON    struct {
		Annotations resp.Field
		Text        resp.Field
		Type        resp.Field
		Refusal     resp.Field
		raw         string
	} `json:"-"`
}

func (r *ResponseStreamEventUnionPart) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when a text annotation is added.
type ResponseTextAnnotationDeltaEvent struct {
	// A citation to a file.
	Annotation ResponseTextAnnotationDeltaEventAnnotationUnion `json:"annotation,required"`
	// The index of the annotation that was added.
	AnnotationIndex int64 `json:"annotation_index,required"`
	// The index of the content part that the text annotation was added to.
	ContentIndex int64 `json:"content_index,required"`
	// The ID of the output item that the text annotation was added to.
	ItemID string `json:"item_id,required"`
	// The index of the output item that the text annotation was added to.
	OutputIndex int64 `json:"output_index,required"`
	// The type of the event. Always `response.output_text.annotation.added`.
	Type constant.ResponseOutputTextAnnotationAdded `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Annotation      resp.Field
		AnnotationIndex resp.Field
		ContentIndex    resp.Field
		ItemID          resp.Field
		OutputIndex     resp.Field
		Type            resp.Field
		ExtraFields     map[string]resp.Field
		raw             string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseTextAnnotationDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseTextAnnotationDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponseTextAnnotationDeltaEventAnnotationUnion contains all possible properties
// and values from [ResponseTextAnnotationDeltaEventAnnotationFileCitation],
// [ResponseTextAnnotationDeltaEventAnnotationURLCitation],
// [ResponseTextAnnotationDeltaEventAnnotationFilePath].
//
// Use the [ResponseTextAnnotationDeltaEventAnnotationUnion.AsAny] method to switch
// on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ResponseTextAnnotationDeltaEventAnnotationUnion struct {
	FileID string `json:"file_id"`
	Index  int64  `json:"index"`
	// Any of "file_citation", "url_citation", "file_path".
	Type string `json:"type"`
	// This field is from variant
	// [ResponseTextAnnotationDeltaEventAnnotationURLCitation].
	EndIndex int64 `json:"end_index"`
	// This field is from variant
	// [ResponseTextAnnotationDeltaEventAnnotationURLCitation].
	StartIndex int64 `json:"start_index"`
	// This field is from variant
	// [ResponseTextAnnotationDeltaEventAnnotationURLCitation].
	Title string `json:"title"`
	// This field is from variant
	// [ResponseTextAnnotationDeltaEventAnnotationURLCitation].
	URL  string `json:"url"`
	JSON struct {
		FileID     resp.Field
		Index      resp.Field
		Type       resp.Field
		EndIndex   resp.Field
		StartIndex resp.Field
		Title      resp.Field
		URL        resp.Field
		raw        string
	} `json:"-"`
}

// Use the following switch statement to find the correct variant
//
//	switch variant := ResponseTextAnnotationDeltaEventAnnotationUnion.AsAny().(type) {
//	case ResponseTextAnnotationDeltaEventAnnotationFileCitation:
//	case ResponseTextAnnotationDeltaEventAnnotationURLCitation:
//	case ResponseTextAnnotationDeltaEventAnnotationFilePath:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ResponseTextAnnotationDeltaEventAnnotationUnion) AsAny() any {
	switch u.Type {
	case "file_citation":
		return u.AsFileCitation()
	case "url_citation":
		return u.AsURLCitation()
	case "file_path":
		return u.AsFilePath()
	}
	return nil
}

func (u ResponseTextAnnotationDeltaEventAnnotationUnion) AsFileCitation() (v ResponseTextAnnotationDeltaEventAnnotationFileCitation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseTextAnnotationDeltaEventAnnotationUnion) AsURLCitation() (v ResponseTextAnnotationDeltaEventAnnotationURLCitation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ResponseTextAnnotationDeltaEventAnnotationUnion) AsFilePath() (v ResponseTextAnnotationDeltaEventAnnotationFilePath) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ResponseTextAnnotationDeltaEventAnnotationUnion) RawJSON() string { return u.JSON.raw }

func (r *ResponseTextAnnotationDeltaEventAnnotationUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A citation to a file.
type ResponseTextAnnotationDeltaEventAnnotationFileCitation struct {
	// The ID of the file.
	FileID string `json:"file_id,required"`
	// The index of the file in the list of files.
	Index int64 `json:"index,required"`
	// The type of the file citation. Always `file_citation`.
	Type constant.FileCitation `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		FileID      resp.Field
		Index       resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseTextAnnotationDeltaEventAnnotationFileCitation) RawJSON() string { return r.JSON.raw }
func (r *ResponseTextAnnotationDeltaEventAnnotationFileCitation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A citation for a web resource used to generate a model response.
type ResponseTextAnnotationDeltaEventAnnotationURLCitation struct {
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		EndIndex    resp.Field
		StartIndex  resp.Field
		Title       resp.Field
		Type        resp.Field
		URL         resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseTextAnnotationDeltaEventAnnotationURLCitation) RawJSON() string { return r.JSON.raw }
func (r *ResponseTextAnnotationDeltaEventAnnotationURLCitation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A path to a file.
type ResponseTextAnnotationDeltaEventAnnotationFilePath struct {
	// The ID of the file.
	FileID string `json:"file_id,required"`
	// The index of the file in the list of files.
	Index int64 `json:"index,required"`
	// The type of the file path. Always `file_path`.
	Type constant.FilePath `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		FileID      resp.Field
		Index       resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseTextAnnotationDeltaEventAnnotationFilePath) RawJSON() string { return r.JSON.raw }
func (r *ResponseTextAnnotationDeltaEventAnnotationFilePath) UnmarshalJSON(data []byte) error {
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Format      resp.Field
		ExtraFields map[string]resp.Field
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
// ResponseTextConfigParam.IsOverridden()
func (r ResponseTextConfig) ToParam() ResponseTextConfigParam {
	return param.OverrideObj[ResponseTextConfigParam](r.RawJSON())
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseTextConfigParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r ResponseTextConfigParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseTextConfigParam
	return param.MarshalObject(r, (*shadow)(&r))
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
	// The type of the event. Always `response.output_text.delta`.
	Type constant.ResponseOutputTextDelta `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ContentIndex resp.Field
		Delta        resp.Field
		ItemID       resp.Field
		OutputIndex  resp.Field
		Type         resp.Field
		ExtraFields  map[string]resp.Field
		raw          string
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
	// The text content that is finalized.
	Text string `json:"text,required"`
	// The type of the event. Always `response.output_text.done`.
	Type constant.ResponseOutputTextDone `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ContentIndex resp.Field
		ItemID       resp.Field
		OutputIndex  resp.Field
		Text         resp.Field
		Type         resp.Field
		ExtraFields  map[string]resp.Field
		raw          string
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		InputTokens         resp.Field
		InputTokensDetails  resp.Field
		OutputTokens        resp.Field
		OutputTokensDetails resp.Field
		TotalTokens         resp.Field
		ExtraFields         map[string]resp.Field
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		CachedTokens resp.Field
		ExtraFields  map[string]resp.Field
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
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ReasoningTokens resp.Field
		ExtraFields     map[string]resp.Field
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
	// The type of the event. Always `response.web_search_call.completed`.
	Type constant.ResponseWebSearchCallCompleted `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ItemID      resp.Field
		OutputIndex resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
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
	// The type of the event. Always `response.web_search_call.in_progress`.
	Type constant.ResponseWebSearchCallInProgress `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ItemID      resp.Field
		OutputIndex resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
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
	// The type of the event. Always `response.web_search_call.searching`.
	Type constant.ResponseWebSearchCallSearching `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		ItemID      resp.Field
		OutputIndex resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseWebSearchCallSearchingEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseWebSearchCallSearchingEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToolUnion contains all possible properties and values from [FileSearchTool],
// [FunctionTool], [ComputerTool], [WebSearchTool].
//
// Use the [ToolUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ToolUnion struct {
	// Any of "file_search", "function", "computer_use_preview", nil.
	Type string `json:"type"`
	// This field is from variant [FileSearchTool].
	VectorStoreIDs []string `json:"vector_store_ids"`
	// This field is from variant [FileSearchTool].
	Filters FileSearchToolFiltersUnion `json:"filters"`
	// This field is from variant [FileSearchTool].
	MaxNumResults int64 `json:"max_num_results"`
	// This field is from variant [FileSearchTool].
	RankingOptions FileSearchToolRankingOptions `json:"ranking_options"`
	// This field is from variant [FunctionTool].
	Name string `json:"name"`
	// This field is from variant [FunctionTool].
	Parameters map[string]interface{} `json:"parameters"`
	// This field is from variant [FunctionTool].
	Strict bool `json:"strict"`
	// This field is from variant [FunctionTool].
	Description string `json:"description"`
	// This field is from variant [ComputerTool].
	DisplayHeight float64 `json:"display_height"`
	// This field is from variant [ComputerTool].
	DisplayWidth float64 `json:"display_width"`
	// This field is from variant [ComputerTool].
	Environment ComputerToolEnvironment `json:"environment"`
	// This field is from variant [WebSearchTool].
	SearchContextSize WebSearchToolSearchContextSize `json:"search_context_size"`
	// This field is from variant [WebSearchTool].
	UserLocation WebSearchToolUserLocation `json:"user_location"`
	JSON         struct {
		Type              resp.Field
		VectorStoreIDs    resp.Field
		Filters           resp.Field
		MaxNumResults     resp.Field
		RankingOptions    resp.Field
		Name              resp.Field
		Parameters        resp.Field
		Strict            resp.Field
		Description       resp.Field
		DisplayHeight     resp.Field
		DisplayWidth      resp.Field
		Environment       resp.Field
		SearchContextSize resp.Field
		UserLocation      resp.Field
		raw               string
	} `json:"-"`
}

func (u ToolUnion) AsFileSearch() (v FileSearchTool) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ToolUnion) AsFunction() (v FunctionTool) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ToolUnion) AsComputerUsePreview() (v ComputerTool) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ToolUnion) AsWebSearch() (v WebSearchTool) {
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
// ToolUnionParam.IsOverridden()
func (r ToolUnion) ToParam() ToolUnionParam {
	return param.OverrideObj[ToolUnionParam](r.RawJSON())
}

func ToolParamOfFileSearch(vectorStoreIDs []string) ToolUnionParam {
	var fileSearch FileSearchToolParam
	fileSearch.VectorStoreIDs = vectorStoreIDs
	return ToolUnionParam{OfFileSearch: &fileSearch}
}

func ToolParamOfFunction(name string, parameters map[string]interface{}, strict bool) ToolUnionParam {
	var function FunctionToolParam
	function.Name = name
	function.Parameters = parameters
	function.Strict = strict
	return ToolUnionParam{OfFunction: &function}
}

func ToolParamOfComputerUsePreview(displayHeight float64, displayWidth float64, environment ComputerToolEnvironment) ToolUnionParam {
	var computerUsePreview ComputerToolParam
	computerUsePreview.DisplayHeight = displayHeight
	computerUsePreview.DisplayWidth = displayWidth
	computerUsePreview.Environment = environment
	return ToolUnionParam{OfComputerUsePreview: &computerUsePreview}
}

func ToolParamOfWebSearch(type_ WebSearchToolType) ToolUnionParam {
	var variant WebSearchToolParam
	variant.Type = type_
	return ToolUnionParam{OfWebSearch: &variant}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ToolUnionParam struct {
	OfFileSearch         *FileSearchToolParam `json:",omitzero,inline"`
	OfFunction           *FunctionToolParam   `json:",omitzero,inline"`
	OfComputerUsePreview *ComputerToolParam   `json:",omitzero,inline"`
	OfWebSearch          *WebSearchToolParam  `json:",omitzero,inline"`
	paramUnion
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (u ToolUnionParam) IsPresent() bool { return !param.IsOmitted(u) && !u.IsNull() }
func (u ToolUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion[ToolUnionParam](u.OfFileSearch, u.OfFunction, u.OfComputerUsePreview, u.OfWebSearch)
}

func (u *ToolUnionParam) asAny() any {
	if !param.IsOmitted(u.OfFileSearch) {
		return u.OfFileSearch
	} else if !param.IsOmitted(u.OfFunction) {
		return u.OfFunction
	} else if !param.IsOmitted(u.OfComputerUsePreview) {
		return u.OfComputerUsePreview
	} else if !param.IsOmitted(u.OfWebSearch) {
		return u.OfWebSearch
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
	if vt := u.OfFileSearch; vt != nil && vt.MaxNumResults.IsPresent() {
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
func (u ToolUnionParam) GetName() *string {
	if vt := u.OfFunction; vt != nil {
		return &vt.Name
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetParameters() map[string]interface{} {
	if vt := u.OfFunction; vt != nil {
		return vt.Parameters
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetStrict() *bool {
	if vt := u.OfFunction; vt != nil {
		return &vt.Strict
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetDescription() *string {
	if vt := u.OfFunction; vt != nil && vt.Description.IsPresent() {
		return &vt.Description.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetDisplayHeight() *float64 {
	if vt := u.OfComputerUsePreview; vt != nil {
		return &vt.DisplayHeight
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetDisplayWidth() *float64 {
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
func (u ToolUnionParam) GetSearchContextSize() *string {
	if vt := u.OfWebSearch; vt != nil {
		return (*string)(&vt.SearchContextSize)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetUserLocation() *WebSearchToolUserLocationParam {
	if vt := u.OfWebSearch; vt != nil {
		return &vt.UserLocation
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetType() *string {
	if vt := u.OfFileSearch; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfFunction; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfComputerUsePreview; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfWebSearch; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[ToolUnionParam](
		"type",
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(FileSearchToolParam{}),
			DiscriminatorValue: "file_search",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(FunctionToolParam{}),
			DiscriminatorValue: "function",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(ComputerToolParam{}),
			DiscriminatorValue: "computer_use_preview",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(WebSearchToolParam{}),
			DiscriminatorValue: "web_search_preview",
		},
		apijson.UnionVariant{
			TypeFilter:         gjson.JSON,
			Type:               reflect.TypeOf(WebSearchToolParam{}),
			DiscriminatorValue: "web_search_preview_2025_03_11",
		},
	)
}

// Use this option to force the model to call a specific function.
type ToolChoiceFunction struct {
	// The name of the function to call.
	Name string `json:"name,required"`
	// For function calling, the type is always `function`.
	Type constant.Function `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Name        resp.Field
		Type        resp.Field
		ExtraFields map[string]resp.Field
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
// ToolChoiceFunctionParam.IsOverridden()
func (r ToolChoiceFunction) ToParam() ToolChoiceFunctionParam {
	return param.OverrideObj[ToolChoiceFunctionParam](r.RawJSON())
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ToolChoiceFunctionParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r ToolChoiceFunctionParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolChoiceFunctionParam
	return param.MarshalObject(r, (*shadow)(&r))
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
	//
	// Any of "file_search", "web_search_preview", "computer_use_preview",
	// "web_search_preview_2025_03_11".
	Type ToolChoiceTypesType `json:"type,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Type        resp.Field
		ExtraFields map[string]resp.Field
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
// ToolChoiceTypesParam.IsOverridden()
func (r ToolChoiceTypes) ToParam() ToolChoiceTypesParam {
	return param.OverrideObj[ToolChoiceTypesParam](r.RawJSON())
}

// The type of hosted tool the model should to use. Learn more about
// [built-in tools](https://platform.openai.com/docs/guides/tools).
//
// Allowed values are:
//
// - `file_search`
// - `web_search_preview`
// - `computer_use_preview`
type ToolChoiceTypesType string

const (
	ToolChoiceTypesTypeFileSearch                 ToolChoiceTypesType = "file_search"
	ToolChoiceTypesTypeWebSearchPreview           ToolChoiceTypesType = "web_search_preview"
	ToolChoiceTypesTypeComputerUsePreview         ToolChoiceTypesType = "computer_use_preview"
	ToolChoiceTypesTypeWebSearchPreview2025_03_11 ToolChoiceTypesType = "web_search_preview_2025_03_11"
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
	//
	// Any of "file_search", "web_search_preview", "computer_use_preview",
	// "web_search_preview_2025_03_11".
	Type ToolChoiceTypesType `json:"type,omitzero,required"`
	paramObj
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ToolChoiceTypesParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r ToolChoiceTypesParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolChoiceTypesParam
	return param.MarshalObject(r, (*shadow)(&r))
}

// This tool searches the web for relevant results to use in a response. Learn more
// about the
// [web search tool](https://platform.openai.com/docs/guides/tools-web-search).
type WebSearchTool struct {
	// The type of the web search tool. One of:
	//
	// - `web_search_preview`
	// - `web_search_preview_2025_03_11`
	//
	// Any of "web_search_preview", "web_search_preview_2025_03_11".
	Type WebSearchToolType `json:"type,required"`
	// High level guidance for the amount of context window space to use for the
	// search. One of `low`, `medium`, or `high`. `medium` is the default.
	//
	// Any of "low", "medium", "high".
	SearchContextSize WebSearchToolSearchContextSize `json:"search_context_size"`
	UserLocation      WebSearchToolUserLocation      `json:"user_location,nullable"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Type              resp.Field
		SearchContextSize resp.Field
		UserLocation      resp.Field
		ExtraFields       map[string]resp.Field
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
// WebSearchToolParam.IsOverridden()
func (r WebSearchTool) ToParam() WebSearchToolParam {
	return param.OverrideObj[WebSearchToolParam](r.RawJSON())
}

// The type of the web search tool. One of:
//
// - `web_search_preview`
// - `web_search_preview_2025_03_11`
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

type WebSearchToolUserLocation struct {
	// The type of location approximation. Always `approximate`.
	Type constant.Approximate `json:"type,required"`
	// Free text input for the city of the user, e.g. `San Francisco`.
	City string `json:"city"`
	// The two-letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1) of
	// the user, e.g. `US`.
	Country string `json:"country"`
	// Free text input for the region of the user, e.g. `California`.
	Region string `json:"region"`
	// The [IANA timezone](https://timeapi.io/documentation/iana-timezones) of the
	// user, e.g. `America/Los_Angeles`.
	Timezone string `json:"timezone"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Type        resp.Field
		City        resp.Field
		Country     resp.Field
		Region      resp.Field
		Timezone    resp.Field
		ExtraFields map[string]resp.Field
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
	// The type of the web search tool. One of:
	//
	// - `web_search_preview`
	// - `web_search_preview_2025_03_11`
	//
	// Any of "web_search_preview", "web_search_preview_2025_03_11".
	Type         WebSearchToolType              `json:"type,omitzero,required"`
	UserLocation WebSearchToolUserLocationParam `json:"user_location,omitzero"`
	// High level guidance for the amount of context window space to use for the
	// search. One of `low`, `medium`, or `high`. `medium` is the default.
	//
	// Any of "low", "medium", "high".
	SearchContextSize WebSearchToolSearchContextSize `json:"search_context_size,omitzero"`
	paramObj
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f WebSearchToolParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r WebSearchToolParam) MarshalJSON() (data []byte, err error) {
	type shadow WebSearchToolParam
	return param.MarshalObject(r, (*shadow)(&r))
}

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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f WebSearchToolUserLocationParam) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }
func (r WebSearchToolUserLocationParam) MarshalJSON() (data []byte, err error) {
	type shadow WebSearchToolUserLocationParam
	return param.MarshalObject(r, (*shadow)(&r))
}

type ResponseNewParams struct {
	// Text, image, or file inputs to the model, used to generate a response.
	//
	// Learn more:
	//
	// - [Text inputs and outputs](https://platform.openai.com/docs/guides/text)
	// - [Image inputs](https://platform.openai.com/docs/guides/images)
	// - [File inputs](https://platform.openai.com/docs/guides/pdf-files)
	// - [Conversation state](https://platform.openai.com/docs/guides/conversation-state)
	// - [Function calling](https://platform.openai.com/docs/guides/function-calling)
	Input ResponseNewParamsInputUnion `json:"input,omitzero,required"`
	// Model ID used to generate the response, like `gpt-4o` or `o1`. OpenAI offers a
	// wide range of models with different capabilities, performance characteristics,
	// and price points. Refer to the
	// [model guide](https://platform.openai.com/docs/models) to browse and compare
	// available models.
	Model shared.ResponsesModel `json:"model,omitzero,required"`
	// Inserts a system (or developer) message as the first item in the model's
	// context.
	//
	// When using along with `previous_response_id`, the instructions from a previous
	// response will be not be carried over to the next response. This makes it simple
	// to swap out system (or developer) messages in new responses.
	Instructions param.Opt[string] `json:"instructions,omitzero"`
	// An upper bound for the number of tokens that can be generated for a response,
	// including visible output tokens and
	// [reasoning tokens](https://platform.openai.com/docs/guides/reasoning).
	MaxOutputTokens param.Opt[int64] `json:"max_output_tokens,omitzero"`
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
	// An alternative to sampling with temperature, called nucleus sampling, where the
	// model considers the results of the tokens with top_p probability mass. So 0.1
	// means only the tokens comprising the top 10% probability mass are considered.
	//
	// We generally recommend altering this or `temperature` but not both.
	TopP param.Opt[float64] `json:"top_p,omitzero"`
	// A unique identifier representing your end-user, which can help OpenAI to monitor
	// and detect abuse.
	// [Learn more](https://platform.openai.com/docs/guides/safety-best-practices#end-user-ids).
	User param.Opt[string] `json:"user,omitzero"`
	// Specify additional output data to include in the model response. Currently
	// supported values are:
	//
	//   - `file_search_call.results`: Include the search results of the file search tool
	//     call.
	//   - `message.input_image.image_url`: Include image urls from the input message.
	//   - `computer_call_output.output.image_url`: Include image urls from the computer
	//     call output.
	Include []ResponseIncludable `json:"include,omitzero"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.MetadataParam `json:"metadata,omitzero"`
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

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseNewParams) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }

func (r ResponseNewParams) MarshalJSON() (data []byte, err error) {
	type shadow ResponseNewParams
	return param.MarshalObject(r, (*shadow)(&r))
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ResponseNewParamsInputUnion struct {
	OfString        param.Opt[string]  `json:",omitzero,inline"`
	OfInputItemList ResponseInputParam `json:",omitzero,inline"`
	paramUnion
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (u ResponseNewParamsInputUnion) IsPresent() bool { return !param.IsOmitted(u) && !u.IsNull() }
func (u ResponseNewParamsInputUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion[ResponseNewParamsInputUnion](u.OfString, u.OfInputItemList)
}

func (u *ResponseNewParamsInputUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfInputItemList) {
		return &u.OfInputItemList
	}
	return nil
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ResponseNewParamsToolChoiceUnion struct {
	// Check if union is this variant with !param.IsOmitted(union.OfToolChoiceMode)
	OfToolChoiceMode param.Opt[ToolChoiceOptions] `json:",omitzero,inline"`
	OfHostedTool     *ToolChoiceTypesParam        `json:",omitzero,inline"`
	OfFunctionTool   *ToolChoiceFunctionParam     `json:",omitzero,inline"`
	paramUnion
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (u ResponseNewParamsToolChoiceUnion) IsPresent() bool { return !param.IsOmitted(u) && !u.IsNull() }
func (u ResponseNewParamsToolChoiceUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion[ResponseNewParamsToolChoiceUnion](u.OfToolChoiceMode, u.OfHostedTool, u.OfFunctionTool)
}

func (u *ResponseNewParamsToolChoiceUnion) asAny() any {
	if !param.IsOmitted(u.OfToolChoiceMode) {
		return &u.OfToolChoiceMode
	} else if !param.IsOmitted(u.OfHostedTool) {
		return u.OfHostedTool
	} else if !param.IsOmitted(u.OfFunctionTool) {
		return u.OfFunctionTool
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseNewParamsToolChoiceUnion) GetName() *string {
	if vt := u.OfFunctionTool; vt != nil {
		return &vt.Name
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ResponseNewParamsToolChoiceUnion) GetType() *string {
	if vt := u.OfHostedTool; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfFunctionTool; vt != nil {
		return (*string)(&vt.Type)
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
	// Additional fields to include in the response. See the `include` parameter for
	// Response creation above for more information.
	Include []ResponseIncludable `query:"include,omitzero" json:"-"`
	paramObj
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f ResponseGetParams) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }

// URLQuery serializes [ResponseGetParams]'s query parameters as `url.Values`.
func (r ResponseGetParams) URLQuery() (v url.Values) {
	return apiquery.MarshalWithSettings(r, apiquery.QuerySettings{
		ArrayFormat:  apiquery.ArrayQueryFormatBrackets,
		NestedFormat: apiquery.NestedQueryFormatBrackets,
	})
}
