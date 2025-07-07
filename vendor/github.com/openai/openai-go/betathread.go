// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/openai/openai-go/internal/apijson"
	"github.com/openai/openai-go/internal/requestconfig"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/respjson"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/shared"
	"github.com/openai/openai-go/shared/constant"
)

// BetaThreadService contains methods and other services that help with interacting
// with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewBetaThreadService] method instead.
//
// Deprecated: The Assistants API is deprecated in favor of the Responses API
type BetaThreadService struct {
	Options []option.RequestOption
	// Deprecated: The Assistants API is deprecated in favor of the Responses API
	Runs BetaThreadRunService
	// Deprecated: The Assistants API is deprecated in favor of the Responses API
	Messages BetaThreadMessageService
}

// NewBetaThreadService generates a new service that applies the given options to
// each request. These options are applied after the parent client's options (if
// there is one), and before any request-specific options.
func NewBetaThreadService(opts ...option.RequestOption) (r BetaThreadService) {
	r = BetaThreadService{}
	r.Options = opts
	r.Runs = NewBetaThreadRunService(opts...)
	r.Messages = NewBetaThreadMessageService(opts...)
	return
}

// Create a thread.
//
// Deprecated: The Assistants API is deprecated in favor of the Responses API
func (r *BetaThreadService) New(ctx context.Context, body BetaThreadNewParams, opts ...option.RequestOption) (res *Thread, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	path := "threads"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// Retrieves a thread.
//
// Deprecated: The Assistants API is deprecated in favor of the Responses API
func (r *BetaThreadService) Get(ctx context.Context, threadID string, opts ...option.RequestOption) (res *Thread, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	if threadID == "" {
		err = errors.New("missing required thread_id parameter")
		return
	}
	path := fmt.Sprintf("threads/%s", threadID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// Modifies a thread.
//
// Deprecated: The Assistants API is deprecated in favor of the Responses API
func (r *BetaThreadService) Update(ctx context.Context, threadID string, body BetaThreadUpdateParams, opts ...option.RequestOption) (res *Thread, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	if threadID == "" {
		err = errors.New("missing required thread_id parameter")
		return
	}
	path := fmt.Sprintf("threads/%s", threadID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// Delete a thread.
//
// Deprecated: The Assistants API is deprecated in favor of the Responses API
func (r *BetaThreadService) Delete(ctx context.Context, threadID string, opts ...option.RequestOption) (res *ThreadDeleted, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	if threadID == "" {
		err = errors.New("missing required thread_id parameter")
		return
	}
	path := fmt.Sprintf("threads/%s", threadID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodDelete, path, nil, &res, opts...)
	return
}

// Create a thread and run it in one request.
//
// Deprecated: The Assistants API is deprecated in favor of the Responses API
func (r *BetaThreadService) NewAndRun(ctx context.Context, body BetaThreadNewAndRunParams, opts ...option.RequestOption) (res *Run, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	path := "threads/runs"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// Create a thread and run it in one request.
//
// Deprecated: The Assistants API is deprecated in favor of the Responses API
func (r *BetaThreadService) NewAndRunStreaming(ctx context.Context, body BetaThreadNewAndRunParams, opts ...option.RequestOption) (stream *ssestream.Stream[AssistantStreamEventUnion]) {
	var (
		raw *http.Response
		err error
	)
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2"), option.WithJSONSet("stream", true)}, opts...)
	path := "threads/runs"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &raw, opts...)
	return ssestream.NewStream[AssistantStreamEventUnion](ssestream.NewDecoder(raw), err)
}

// AssistantResponseFormatOptionUnion contains all possible properties and values
// from [constant.Auto], [shared.ResponseFormatText],
// [shared.ResponseFormatJSONObject], [shared.ResponseFormatJSONSchema].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfAuto]
type AssistantResponseFormatOptionUnion struct {
	// This field will be present if the value is a [constant.Auto] instead of an
	// object.
	OfAuto constant.Auto `json:",inline"`
	Type   string        `json:"type"`
	// This field is from variant [shared.ResponseFormatJSONSchema].
	JSONSchema shared.ResponseFormatJSONSchemaJSONSchema `json:"json_schema"`
	JSON       struct {
		OfAuto     respjson.Field
		Type       respjson.Field
		JSONSchema respjson.Field
		raw        string
	} `json:"-"`
}

func (u AssistantResponseFormatOptionUnion) AsAuto() (v constant.Auto) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantResponseFormatOptionUnion) AsText() (v shared.ResponseFormatText) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantResponseFormatOptionUnion) AsJSONObject() (v shared.ResponseFormatJSONObject) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantResponseFormatOptionUnion) AsJSONSchema() (v shared.ResponseFormatJSONSchema) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u AssistantResponseFormatOptionUnion) RawJSON() string { return u.JSON.raw }

func (r *AssistantResponseFormatOptionUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this AssistantResponseFormatOptionUnion to a
// AssistantResponseFormatOptionUnionParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// AssistantResponseFormatOptionUnionParam.Overrides()
func (r AssistantResponseFormatOptionUnion) ToParam() AssistantResponseFormatOptionUnionParam {
	return param.Override[AssistantResponseFormatOptionUnionParam](json.RawMessage(r.RawJSON()))
}

func AssistantResponseFormatOptionParamOfAuto() AssistantResponseFormatOptionUnionParam {
	return AssistantResponseFormatOptionUnionParam{OfAuto: constant.ValueOf[constant.Auto]()}
}

func AssistantResponseFormatOptionParamOfJSONSchema(jsonSchema shared.ResponseFormatJSONSchemaJSONSchemaParam) AssistantResponseFormatOptionUnionParam {
	var variant shared.ResponseFormatJSONSchemaParam
	variant.JSONSchema = jsonSchema
	return AssistantResponseFormatOptionUnionParam{OfJSONSchema: &variant}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type AssistantResponseFormatOptionUnionParam struct {
	// Construct this variant with constant.ValueOf[constant.Auto]()
	OfAuto       constant.Auto                         `json:",omitzero,inline"`
	OfText       *shared.ResponseFormatTextParam       `json:",omitzero,inline"`
	OfJSONObject *shared.ResponseFormatJSONObjectParam `json:",omitzero,inline"`
	OfJSONSchema *shared.ResponseFormatJSONSchemaParam `json:",omitzero,inline"`
	paramUnion
}

func (u AssistantResponseFormatOptionUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfText, u.OfJSONObject, u.OfJSONSchema)
}
func (u *AssistantResponseFormatOptionUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *AssistantResponseFormatOptionUnionParam) asAny() any {
	if !param.IsOmitted(u.OfAuto) {
		return &u.OfAuto
	} else if !param.IsOmitted(u.OfText) {
		return u.OfText
	} else if !param.IsOmitted(u.OfJSONObject) {
		return u.OfJSONObject
	} else if !param.IsOmitted(u.OfJSONSchema) {
		return u.OfJSONSchema
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u AssistantResponseFormatOptionUnionParam) GetJSONSchema() *shared.ResponseFormatJSONSchemaJSONSchemaParam {
	if vt := u.OfJSONSchema; vt != nil {
		return &vt.JSONSchema
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u AssistantResponseFormatOptionUnionParam) GetType() *string {
	if vt := u.OfText; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfJSONObject; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfJSONSchema; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Specifies a tool the model should use. Use to force the model to call a specific
// tool.
type AssistantToolChoice struct {
	// The type of the tool. If type is `function`, the function name must be set
	//
	// Any of "function", "code_interpreter", "file_search".
	Type     AssistantToolChoiceType     `json:"type,required"`
	Function AssistantToolChoiceFunction `json:"function"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		Function    respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantToolChoice) RawJSON() string { return r.JSON.raw }
func (r *AssistantToolChoice) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this AssistantToolChoice to a AssistantToolChoiceParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// AssistantToolChoiceParam.Overrides()
func (r AssistantToolChoice) ToParam() AssistantToolChoiceParam {
	return param.Override[AssistantToolChoiceParam](json.RawMessage(r.RawJSON()))
}

// The type of the tool. If type is `function`, the function name must be set
type AssistantToolChoiceType string

const (
	AssistantToolChoiceTypeFunction        AssistantToolChoiceType = "function"
	AssistantToolChoiceTypeCodeInterpreter AssistantToolChoiceType = "code_interpreter"
	AssistantToolChoiceTypeFileSearch      AssistantToolChoiceType = "file_search"
)

// Specifies a tool the model should use. Use to force the model to call a specific
// tool.
//
// The property Type is required.
type AssistantToolChoiceParam struct {
	// The type of the tool. If type is `function`, the function name must be set
	//
	// Any of "function", "code_interpreter", "file_search".
	Type     AssistantToolChoiceType          `json:"type,omitzero,required"`
	Function AssistantToolChoiceFunctionParam `json:"function,omitzero"`
	paramObj
}

func (r AssistantToolChoiceParam) MarshalJSON() (data []byte, err error) {
	type shadow AssistantToolChoiceParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *AssistantToolChoiceParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type AssistantToolChoiceFunction struct {
	// The name of the function to call.
	Name string `json:"name,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Name        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantToolChoiceFunction) RawJSON() string { return r.JSON.raw }
func (r *AssistantToolChoiceFunction) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this AssistantToolChoiceFunction to a
// AssistantToolChoiceFunctionParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// AssistantToolChoiceFunctionParam.Overrides()
func (r AssistantToolChoiceFunction) ToParam() AssistantToolChoiceFunctionParam {
	return param.Override[AssistantToolChoiceFunctionParam](json.RawMessage(r.RawJSON()))
}

// The property Name is required.
type AssistantToolChoiceFunctionParam struct {
	// The name of the function to call.
	Name string `json:"name,required"`
	paramObj
}

func (r AssistantToolChoiceFunctionParam) MarshalJSON() (data []byte, err error) {
	type shadow AssistantToolChoiceFunctionParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *AssistantToolChoiceFunctionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// AssistantToolChoiceOptionUnion contains all possible properties and values from
// [string], [AssistantToolChoice].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfAuto]
type AssistantToolChoiceOptionUnion struct {
	// This field will be present if the value is a [string] instead of an object.
	OfAuto string `json:",inline"`
	// This field is from variant [AssistantToolChoice].
	Type AssistantToolChoiceType `json:"type"`
	// This field is from variant [AssistantToolChoice].
	Function AssistantToolChoiceFunction `json:"function"`
	JSON     struct {
		OfAuto   respjson.Field
		Type     respjson.Field
		Function respjson.Field
		raw      string
	} `json:"-"`
}

func (u AssistantToolChoiceOptionUnion) AsAuto() (v string) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantToolChoiceOptionUnion) AsAssistantToolChoice() (v AssistantToolChoice) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u AssistantToolChoiceOptionUnion) RawJSON() string { return u.JSON.raw }

func (r *AssistantToolChoiceOptionUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this AssistantToolChoiceOptionUnion to a
// AssistantToolChoiceOptionUnionParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// AssistantToolChoiceOptionUnionParam.Overrides()
func (r AssistantToolChoiceOptionUnion) ToParam() AssistantToolChoiceOptionUnionParam {
	return param.Override[AssistantToolChoiceOptionUnionParam](json.RawMessage(r.RawJSON()))
}

// `none` means the model will not call any tools and instead generates a message.
// `auto` means the model can pick between generating a message or calling one or
// more tools. `required` means the model must call one or more tools before
// responding to the user.
type AssistantToolChoiceOptionAuto string

const (
	AssistantToolChoiceOptionAutoNone     AssistantToolChoiceOptionAuto = "none"
	AssistantToolChoiceOptionAutoAuto     AssistantToolChoiceOptionAuto = "auto"
	AssistantToolChoiceOptionAutoRequired AssistantToolChoiceOptionAuto = "required"
)

func AssistantToolChoiceOptionParamOfAssistantToolChoice(type_ AssistantToolChoiceType) AssistantToolChoiceOptionUnionParam {
	var variant AssistantToolChoiceParam
	variant.Type = type_
	return AssistantToolChoiceOptionUnionParam{OfAssistantToolChoice: &variant}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type AssistantToolChoiceOptionUnionParam struct {
	// Check if union is this variant with !param.IsOmitted(union.OfAuto)
	OfAuto                param.Opt[string]         `json:",omitzero,inline"`
	OfAssistantToolChoice *AssistantToolChoiceParam `json:",omitzero,inline"`
	paramUnion
}

func (u AssistantToolChoiceOptionUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfAssistantToolChoice)
}
func (u *AssistantToolChoiceOptionUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *AssistantToolChoiceOptionUnionParam) asAny() any {
	if !param.IsOmitted(u.OfAuto) {
		return &u.OfAuto
	} else if !param.IsOmitted(u.OfAssistantToolChoice) {
		return u.OfAssistantToolChoice
	}
	return nil
}

// Represents a thread that contains
// [messages](https://platform.openai.com/docs/api-reference/messages).
type Thread struct {
	// The identifier, which can be referenced in API endpoints.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) for when the thread was created.
	CreatedAt int64 `json:"created_at,required"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,required"`
	// The object type, which is always `thread`.
	Object constant.Thread `json:"object,required"`
	// A set of resources that are made available to the assistant's tools in this
	// thread. The resources are specific to the type of tool. For example, the
	// `code_interpreter` tool requires a list of file IDs, while the `file_search`
	// tool requires a list of vector store IDs.
	ToolResources ThreadToolResources `json:"tool_resources,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID            respjson.Field
		CreatedAt     respjson.Field
		Metadata      respjson.Field
		Object        respjson.Field
		ToolResources respjson.Field
		ExtraFields   map[string]respjson.Field
		raw           string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r Thread) RawJSON() string { return r.JSON.raw }
func (r *Thread) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A set of resources that are made available to the assistant's tools in this
// thread. The resources are specific to the type of tool. For example, the
// `code_interpreter` tool requires a list of file IDs, while the `file_search`
// tool requires a list of vector store IDs.
type ThreadToolResources struct {
	CodeInterpreter ThreadToolResourcesCodeInterpreter `json:"code_interpreter"`
	FileSearch      ThreadToolResourcesFileSearch      `json:"file_search"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		CodeInterpreter respjson.Field
		FileSearch      respjson.Field
		ExtraFields     map[string]respjson.Field
		raw             string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ThreadToolResources) RawJSON() string { return r.JSON.raw }
func (r *ThreadToolResources) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ThreadToolResourcesCodeInterpreter struct {
	// A list of [file](https://platform.openai.com/docs/api-reference/files) IDs made
	// available to the `code_interpreter` tool. There can be a maximum of 20 files
	// associated with the tool.
	FileIDs []string `json:"file_ids"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		FileIDs     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ThreadToolResourcesCodeInterpreter) RawJSON() string { return r.JSON.raw }
func (r *ThreadToolResourcesCodeInterpreter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ThreadToolResourcesFileSearch struct {
	// The
	// [vector store](https://platform.openai.com/docs/api-reference/vector-stores/object)
	// attached to this thread. There can be a maximum of 1 vector store attached to
	// the thread.
	VectorStoreIDs []string `json:"vector_store_ids"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		VectorStoreIDs respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ThreadToolResourcesFileSearch) RawJSON() string { return r.JSON.raw }
func (r *ThreadToolResourcesFileSearch) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ThreadDeleted struct {
	ID      string                 `json:"id,required"`
	Deleted bool                   `json:"deleted,required"`
	Object  constant.ThreadDeleted `json:"object,required"`
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
func (r ThreadDeleted) RawJSON() string { return r.JSON.raw }
func (r *ThreadDeleted) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaThreadNewParams struct {
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,omitzero"`
	// A set of resources that are made available to the assistant's tools in this
	// thread. The resources are specific to the type of tool. For example, the
	// `code_interpreter` tool requires a list of file IDs, while the `file_search`
	// tool requires a list of vector store IDs.
	ToolResources BetaThreadNewParamsToolResources `json:"tool_resources,omitzero"`
	// A list of [messages](https://platform.openai.com/docs/api-reference/messages) to
	// start the thread with.
	Messages []BetaThreadNewParamsMessage `json:"messages,omitzero"`
	paramObj
}

func (r BetaThreadNewParams) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Content, Role are required.
type BetaThreadNewParamsMessage struct {
	// The text contents of the message.
	Content BetaThreadNewParamsMessageContentUnion `json:"content,omitzero,required"`
	// The role of the entity that is creating the message. Allowed values include:
	//
	//   - `user`: Indicates the message is sent by an actual user and should be used in
	//     most cases to represent user-generated messages.
	//   - `assistant`: Indicates the message is generated by the assistant. Use this
	//     value to insert messages from the assistant into the conversation.
	//
	// Any of "user", "assistant".
	Role string `json:"role,omitzero,required"`
	// A list of files attached to the message, and the tools they should be added to.
	Attachments []BetaThreadNewParamsMessageAttachment `json:"attachments,omitzero"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,omitzero"`
	paramObj
}

func (r BetaThreadNewParamsMessage) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewParamsMessage
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewParamsMessage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[BetaThreadNewParamsMessage](
		"role", "user", "assistant",
	)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaThreadNewParamsMessageContentUnion struct {
	OfString              param.Opt[string]              `json:",omitzero,inline"`
	OfArrayOfContentParts []MessageContentPartParamUnion `json:",omitzero,inline"`
	paramUnion
}

func (u BetaThreadNewParamsMessageContentUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfArrayOfContentParts)
}
func (u *BetaThreadNewParamsMessageContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaThreadNewParamsMessageContentUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfArrayOfContentParts) {
		return &u.OfArrayOfContentParts
	}
	return nil
}

type BetaThreadNewParamsMessageAttachment struct {
	// The ID of the file to attach to the message.
	FileID param.Opt[string] `json:"file_id,omitzero"`
	// The tools to add this file to.
	Tools []BetaThreadNewParamsMessageAttachmentToolUnion `json:"tools,omitzero"`
	paramObj
}

func (r BetaThreadNewParamsMessageAttachment) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewParamsMessageAttachment
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewParamsMessageAttachment) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaThreadNewParamsMessageAttachmentToolUnion struct {
	OfCodeInterpreter *CodeInterpreterToolParam                           `json:",omitzero,inline"`
	OfFileSearch      *BetaThreadNewParamsMessageAttachmentToolFileSearch `json:",omitzero,inline"`
	paramUnion
}

func (u BetaThreadNewParamsMessageAttachmentToolUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfCodeInterpreter, u.OfFileSearch)
}
func (u *BetaThreadNewParamsMessageAttachmentToolUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaThreadNewParamsMessageAttachmentToolUnion) asAny() any {
	if !param.IsOmitted(u.OfCodeInterpreter) {
		return u.OfCodeInterpreter
	} else if !param.IsOmitted(u.OfFileSearch) {
		return u.OfFileSearch
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaThreadNewParamsMessageAttachmentToolUnion) GetType() *string {
	if vt := u.OfCodeInterpreter; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfFileSearch; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[BetaThreadNewParamsMessageAttachmentToolUnion](
		"type",
		apijson.Discriminator[CodeInterpreterToolParam]("code_interpreter"),
		apijson.Discriminator[BetaThreadNewParamsMessageAttachmentToolFileSearch]("file_search"),
	)
}

func NewBetaThreadNewParamsMessageAttachmentToolFileSearch() BetaThreadNewParamsMessageAttachmentToolFileSearch {
	return BetaThreadNewParamsMessageAttachmentToolFileSearch{
		Type: "file_search",
	}
}

// This struct has a constant value, construct it with
// [NewBetaThreadNewParamsMessageAttachmentToolFileSearch].
type BetaThreadNewParamsMessageAttachmentToolFileSearch struct {
	// The type of tool being defined: `file_search`
	Type constant.FileSearch `json:"type,required"`
	paramObj
}

func (r BetaThreadNewParamsMessageAttachmentToolFileSearch) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewParamsMessageAttachmentToolFileSearch
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewParamsMessageAttachmentToolFileSearch) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A set of resources that are made available to the assistant's tools in this
// thread. The resources are specific to the type of tool. For example, the
// `code_interpreter` tool requires a list of file IDs, while the `file_search`
// tool requires a list of vector store IDs.
type BetaThreadNewParamsToolResources struct {
	CodeInterpreter BetaThreadNewParamsToolResourcesCodeInterpreter `json:"code_interpreter,omitzero"`
	FileSearch      BetaThreadNewParamsToolResourcesFileSearch      `json:"file_search,omitzero"`
	paramObj
}

func (r BetaThreadNewParamsToolResources) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewParamsToolResources
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewParamsToolResources) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaThreadNewParamsToolResourcesCodeInterpreter struct {
	// A list of [file](https://platform.openai.com/docs/api-reference/files) IDs made
	// available to the `code_interpreter` tool. There can be a maximum of 20 files
	// associated with the tool.
	FileIDs []string `json:"file_ids,omitzero"`
	paramObj
}

func (r BetaThreadNewParamsToolResourcesCodeInterpreter) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewParamsToolResourcesCodeInterpreter
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewParamsToolResourcesCodeInterpreter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaThreadNewParamsToolResourcesFileSearch struct {
	// The
	// [vector store](https://platform.openai.com/docs/api-reference/vector-stores/object)
	// attached to this thread. There can be a maximum of 1 vector store attached to
	// the thread.
	VectorStoreIDs []string `json:"vector_store_ids,omitzero"`
	// A helper to create a
	// [vector store](https://platform.openai.com/docs/api-reference/vector-stores/object)
	// with file_ids and attach it to this thread. There can be a maximum of 1 vector
	// store attached to the thread.
	VectorStores []BetaThreadNewParamsToolResourcesFileSearchVectorStore `json:"vector_stores,omitzero"`
	paramObj
}

func (r BetaThreadNewParamsToolResourcesFileSearch) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewParamsToolResourcesFileSearch
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewParamsToolResourcesFileSearch) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaThreadNewParamsToolResourcesFileSearchVectorStore struct {
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,omitzero"`
	// The chunking strategy used to chunk the file(s). If not set, will use the `auto`
	// strategy.
	ChunkingStrategy BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyUnion `json:"chunking_strategy,omitzero"`
	// A list of [file](https://platform.openai.com/docs/api-reference/files) IDs to
	// add to the vector store. There can be a maximum of 10000 files in a vector
	// store.
	FileIDs []string `json:"file_ids,omitzero"`
	paramObj
}

func (r BetaThreadNewParamsToolResourcesFileSearchVectorStore) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewParamsToolResourcesFileSearchVectorStore
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewParamsToolResourcesFileSearchVectorStore) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyUnion struct {
	OfAuto   *BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto   `json:",omitzero,inline"`
	OfStatic *BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStatic `json:",omitzero,inline"`
	paramUnion
}

func (u BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfStatic)
}
func (u *BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyUnion) asAny() any {
	if !param.IsOmitted(u.OfAuto) {
		return u.OfAuto
	} else if !param.IsOmitted(u.OfStatic) {
		return u.OfStatic
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyUnion) GetStatic() *BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStaticStatic {
	if vt := u.OfStatic; vt != nil {
		return &vt.Static
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyUnion) GetType() *string {
	if vt := u.OfAuto; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfStatic; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyUnion](
		"type",
		apijson.Discriminator[BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto]("auto"),
		apijson.Discriminator[BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStatic]("static"),
	)
}

func NewBetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto() BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto {
	return BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto{
		Type: "auto",
	}
}

// The default strategy. This strategy currently uses a `max_chunk_size_tokens` of
// `800` and `chunk_overlap_tokens` of `400`.
//
// This struct has a constant value, construct it with
// [NewBetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto].
type BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto struct {
	// Always `auto`.
	Type constant.Auto `json:"type,required"`
	paramObj
}

func (r BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Static, Type are required.
type BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStatic struct {
	Static BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStaticStatic `json:"static,omitzero,required"`
	// Always `static`.
	//
	// This field can be elided, and will marshal its zero value as "static".
	Type constant.Static `json:"type,required"`
	paramObj
}

func (r BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStatic) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStatic
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStatic) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties ChunkOverlapTokens, MaxChunkSizeTokens are required.
type BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStaticStatic struct {
	// The number of tokens that overlap between chunks. The default value is `400`.
	//
	// Note that the overlap must not exceed half of `max_chunk_size_tokens`.
	ChunkOverlapTokens int64 `json:"chunk_overlap_tokens,required"`
	// The maximum number of tokens in each chunk. The default value is `800`. The
	// minimum value is `100` and the maximum value is `4096`.
	MaxChunkSizeTokens int64 `json:"max_chunk_size_tokens,required"`
	paramObj
}

func (r BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStaticStatic) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStaticStatic
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStaticStatic) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaThreadUpdateParams struct {
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,omitzero"`
	// A set of resources that are made available to the assistant's tools in this
	// thread. The resources are specific to the type of tool. For example, the
	// `code_interpreter` tool requires a list of file IDs, while the `file_search`
	// tool requires a list of vector store IDs.
	ToolResources BetaThreadUpdateParamsToolResources `json:"tool_resources,omitzero"`
	paramObj
}

func (r BetaThreadUpdateParams) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadUpdateParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadUpdateParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A set of resources that are made available to the assistant's tools in this
// thread. The resources are specific to the type of tool. For example, the
// `code_interpreter` tool requires a list of file IDs, while the `file_search`
// tool requires a list of vector store IDs.
type BetaThreadUpdateParamsToolResources struct {
	CodeInterpreter BetaThreadUpdateParamsToolResourcesCodeInterpreter `json:"code_interpreter,omitzero"`
	FileSearch      BetaThreadUpdateParamsToolResourcesFileSearch      `json:"file_search,omitzero"`
	paramObj
}

func (r BetaThreadUpdateParamsToolResources) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadUpdateParamsToolResources
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadUpdateParamsToolResources) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaThreadUpdateParamsToolResourcesCodeInterpreter struct {
	// A list of [file](https://platform.openai.com/docs/api-reference/files) IDs made
	// available to the `code_interpreter` tool. There can be a maximum of 20 files
	// associated with the tool.
	FileIDs []string `json:"file_ids,omitzero"`
	paramObj
}

func (r BetaThreadUpdateParamsToolResourcesCodeInterpreter) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadUpdateParamsToolResourcesCodeInterpreter
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadUpdateParamsToolResourcesCodeInterpreter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaThreadUpdateParamsToolResourcesFileSearch struct {
	// The
	// [vector store](https://platform.openai.com/docs/api-reference/vector-stores/object)
	// attached to this thread. There can be a maximum of 1 vector store attached to
	// the thread.
	VectorStoreIDs []string `json:"vector_store_ids,omitzero"`
	paramObj
}

func (r BetaThreadUpdateParamsToolResourcesFileSearch) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadUpdateParamsToolResourcesFileSearch
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadUpdateParamsToolResourcesFileSearch) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaThreadNewAndRunParams struct {
	// The ID of the
	// [assistant](https://platform.openai.com/docs/api-reference/assistants) to use to
	// execute this run.
	AssistantID string `json:"assistant_id,required"`
	// Override the default system message of the assistant. This is useful for
	// modifying the behavior on a per-run basis.
	Instructions param.Opt[string] `json:"instructions,omitzero"`
	// The maximum number of completion tokens that may be used over the course of the
	// run. The run will make a best effort to use only the number of completion tokens
	// specified, across multiple turns of the run. If the run exceeds the number of
	// completion tokens specified, the run will end with status `incomplete`. See
	// `incomplete_details` for more info.
	MaxCompletionTokens param.Opt[int64] `json:"max_completion_tokens,omitzero"`
	// The maximum number of prompt tokens that may be used over the course of the run.
	// The run will make a best effort to use only the number of prompt tokens
	// specified, across multiple turns of the run. If the run exceeds the number of
	// prompt tokens specified, the run will end with status `incomplete`. See
	// `incomplete_details` for more info.
	MaxPromptTokens param.Opt[int64] `json:"max_prompt_tokens,omitzero"`
	// What sampling temperature to use, between 0 and 2. Higher values like 0.8 will
	// make the output more random, while lower values like 0.2 will make it more
	// focused and deterministic.
	Temperature param.Opt[float64] `json:"temperature,omitzero"`
	// An alternative to sampling with temperature, called nucleus sampling, where the
	// model considers the results of the tokens with top_p probability mass. So 0.1
	// means only the tokens comprising the top 10% probability mass are considered.
	//
	// We generally recommend altering this or temperature but not both.
	TopP param.Opt[float64] `json:"top_p,omitzero"`
	// Whether to enable
	// [parallel function calling](https://platform.openai.com/docs/guides/function-calling#configuring-parallel-function-calling)
	// during tool use.
	ParallelToolCalls param.Opt[bool] `json:"parallel_tool_calls,omitzero"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,omitzero"`
	// The ID of the [Model](https://platform.openai.com/docs/api-reference/models) to
	// be used to execute this run. If a value is provided here, it will override the
	// model associated with the assistant. If not, the model associated with the
	// assistant will be used.
	Model shared.ChatModel `json:"model,omitzero"`
	// A set of resources that are used by the assistant's tools. The resources are
	// specific to the type of tool. For example, the `code_interpreter` tool requires
	// a list of file IDs, while the `file_search` tool requires a list of vector store
	// IDs.
	ToolResources BetaThreadNewAndRunParamsToolResources `json:"tool_resources,omitzero"`
	// Override the tools the assistant can use for this run. This is useful for
	// modifying the behavior on a per-run basis.
	Tools []AssistantToolUnionParam `json:"tools,omitzero"`
	// Controls for how a thread will be truncated prior to the run. Use this to
	// control the intial context window of the run.
	TruncationStrategy BetaThreadNewAndRunParamsTruncationStrategy `json:"truncation_strategy,omitzero"`
	// Specifies the format that the model must output. Compatible with
	// [GPT-4o](https://platform.openai.com/docs/models#gpt-4o),
	// [GPT-4 Turbo](https://platform.openai.com/docs/models#gpt-4-turbo-and-gpt-4),
	// and all GPT-3.5 Turbo models since `gpt-3.5-turbo-1106`.
	//
	// Setting to `{ "type": "json_schema", "json_schema": {...} }` enables Structured
	// Outputs which ensures the model will match your supplied JSON schema. Learn more
	// in the
	// [Structured Outputs guide](https://platform.openai.com/docs/guides/structured-outputs).
	//
	// Setting to `{ "type": "json_object" }` enables JSON mode, which ensures the
	// message the model generates is valid JSON.
	//
	// **Important:** when using JSON mode, you **must** also instruct the model to
	// produce JSON yourself via a system or user message. Without this, the model may
	// generate an unending stream of whitespace until the generation reaches the token
	// limit, resulting in a long-running and seemingly "stuck" request. Also note that
	// the message content may be partially cut off if `finish_reason="length"`, which
	// indicates the generation exceeded `max_tokens` or the conversation exceeded the
	// max context length.
	ResponseFormat AssistantResponseFormatOptionUnionParam `json:"response_format,omitzero"`
	// Options to create a new thread. If no thread is provided when running a request,
	// an empty thread will be created.
	Thread BetaThreadNewAndRunParamsThread `json:"thread,omitzero"`
	// Controls which (if any) tool is called by the model. `none` means the model will
	// not call any tools and instead generates a message. `auto` is the default value
	// and means the model can pick between generating a message or calling one or more
	// tools. `required` means the model must call one or more tools before responding
	// to the user. Specifying a particular tool like `{"type": "file_search"}` or
	// `{"type": "function", "function": {"name": "my_function"}}` forces the model to
	// call that tool.
	ToolChoice AssistantToolChoiceOptionUnionParam `json:"tool_choice,omitzero"`
	paramObj
}

func (r BetaThreadNewAndRunParams) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewAndRunParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewAndRunParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Options to create a new thread. If no thread is provided when running a request,
// an empty thread will be created.
type BetaThreadNewAndRunParamsThread struct {
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,omitzero"`
	// A set of resources that are made available to the assistant's tools in this
	// thread. The resources are specific to the type of tool. For example, the
	// `code_interpreter` tool requires a list of file IDs, while the `file_search`
	// tool requires a list of vector store IDs.
	ToolResources BetaThreadNewAndRunParamsThreadToolResources `json:"tool_resources,omitzero"`
	// A list of [messages](https://platform.openai.com/docs/api-reference/messages) to
	// start the thread with.
	Messages []BetaThreadNewAndRunParamsThreadMessage `json:"messages,omitzero"`
	paramObj
}

func (r BetaThreadNewAndRunParamsThread) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewAndRunParamsThread
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewAndRunParamsThread) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Content, Role are required.
type BetaThreadNewAndRunParamsThreadMessage struct {
	// The text contents of the message.
	Content BetaThreadNewAndRunParamsThreadMessageContentUnion `json:"content,omitzero,required"`
	// The role of the entity that is creating the message. Allowed values include:
	//
	//   - `user`: Indicates the message is sent by an actual user and should be used in
	//     most cases to represent user-generated messages.
	//   - `assistant`: Indicates the message is generated by the assistant. Use this
	//     value to insert messages from the assistant into the conversation.
	//
	// Any of "user", "assistant".
	Role string `json:"role,omitzero,required"`
	// A list of files attached to the message, and the tools they should be added to.
	Attachments []BetaThreadNewAndRunParamsThreadMessageAttachment `json:"attachments,omitzero"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,omitzero"`
	paramObj
}

func (r BetaThreadNewAndRunParamsThreadMessage) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewAndRunParamsThreadMessage
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewAndRunParamsThreadMessage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[BetaThreadNewAndRunParamsThreadMessage](
		"role", "user", "assistant",
	)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaThreadNewAndRunParamsThreadMessageContentUnion struct {
	OfString              param.Opt[string]              `json:",omitzero,inline"`
	OfArrayOfContentParts []MessageContentPartParamUnion `json:",omitzero,inline"`
	paramUnion
}

func (u BetaThreadNewAndRunParamsThreadMessageContentUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfArrayOfContentParts)
}
func (u *BetaThreadNewAndRunParamsThreadMessageContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaThreadNewAndRunParamsThreadMessageContentUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfArrayOfContentParts) {
		return &u.OfArrayOfContentParts
	}
	return nil
}

type BetaThreadNewAndRunParamsThreadMessageAttachment struct {
	// The ID of the file to attach to the message.
	FileID param.Opt[string] `json:"file_id,omitzero"`
	// The tools to add this file to.
	Tools []BetaThreadNewAndRunParamsThreadMessageAttachmentToolUnion `json:"tools,omitzero"`
	paramObj
}

func (r BetaThreadNewAndRunParamsThreadMessageAttachment) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewAndRunParamsThreadMessageAttachment
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewAndRunParamsThreadMessageAttachment) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaThreadNewAndRunParamsThreadMessageAttachmentToolUnion struct {
	OfCodeInterpreter *CodeInterpreterToolParam                                       `json:",omitzero,inline"`
	OfFileSearch      *BetaThreadNewAndRunParamsThreadMessageAttachmentToolFileSearch `json:",omitzero,inline"`
	paramUnion
}

func (u BetaThreadNewAndRunParamsThreadMessageAttachmentToolUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfCodeInterpreter, u.OfFileSearch)
}
func (u *BetaThreadNewAndRunParamsThreadMessageAttachmentToolUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaThreadNewAndRunParamsThreadMessageAttachmentToolUnion) asAny() any {
	if !param.IsOmitted(u.OfCodeInterpreter) {
		return u.OfCodeInterpreter
	} else if !param.IsOmitted(u.OfFileSearch) {
		return u.OfFileSearch
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaThreadNewAndRunParamsThreadMessageAttachmentToolUnion) GetType() *string {
	if vt := u.OfCodeInterpreter; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfFileSearch; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[BetaThreadNewAndRunParamsThreadMessageAttachmentToolUnion](
		"type",
		apijson.Discriminator[CodeInterpreterToolParam]("code_interpreter"),
		apijson.Discriminator[BetaThreadNewAndRunParamsThreadMessageAttachmentToolFileSearch]("file_search"),
	)
}

func NewBetaThreadNewAndRunParamsThreadMessageAttachmentToolFileSearch() BetaThreadNewAndRunParamsThreadMessageAttachmentToolFileSearch {
	return BetaThreadNewAndRunParamsThreadMessageAttachmentToolFileSearch{
		Type: "file_search",
	}
}

// This struct has a constant value, construct it with
// [NewBetaThreadNewAndRunParamsThreadMessageAttachmentToolFileSearch].
type BetaThreadNewAndRunParamsThreadMessageAttachmentToolFileSearch struct {
	// The type of tool being defined: `file_search`
	Type constant.FileSearch `json:"type,required"`
	paramObj
}

func (r BetaThreadNewAndRunParamsThreadMessageAttachmentToolFileSearch) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewAndRunParamsThreadMessageAttachmentToolFileSearch
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewAndRunParamsThreadMessageAttachmentToolFileSearch) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A set of resources that are made available to the assistant's tools in this
// thread. The resources are specific to the type of tool. For example, the
// `code_interpreter` tool requires a list of file IDs, while the `file_search`
// tool requires a list of vector store IDs.
type BetaThreadNewAndRunParamsThreadToolResources struct {
	CodeInterpreter BetaThreadNewAndRunParamsThreadToolResourcesCodeInterpreter `json:"code_interpreter,omitzero"`
	FileSearch      BetaThreadNewAndRunParamsThreadToolResourcesFileSearch      `json:"file_search,omitzero"`
	paramObj
}

func (r BetaThreadNewAndRunParamsThreadToolResources) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewAndRunParamsThreadToolResources
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewAndRunParamsThreadToolResources) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaThreadNewAndRunParamsThreadToolResourcesCodeInterpreter struct {
	// A list of [file](https://platform.openai.com/docs/api-reference/files) IDs made
	// available to the `code_interpreter` tool. There can be a maximum of 20 files
	// associated with the tool.
	FileIDs []string `json:"file_ids,omitzero"`
	paramObj
}

func (r BetaThreadNewAndRunParamsThreadToolResourcesCodeInterpreter) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewAndRunParamsThreadToolResourcesCodeInterpreter
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewAndRunParamsThreadToolResourcesCodeInterpreter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaThreadNewAndRunParamsThreadToolResourcesFileSearch struct {
	// The
	// [vector store](https://platform.openai.com/docs/api-reference/vector-stores/object)
	// attached to this thread. There can be a maximum of 1 vector store attached to
	// the thread.
	VectorStoreIDs []string `json:"vector_store_ids,omitzero"`
	// A helper to create a
	// [vector store](https://platform.openai.com/docs/api-reference/vector-stores/object)
	// with file_ids and attach it to this thread. There can be a maximum of 1 vector
	// store attached to the thread.
	VectorStores []BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStore `json:"vector_stores,omitzero"`
	paramObj
}

func (r BetaThreadNewAndRunParamsThreadToolResourcesFileSearch) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewAndRunParamsThreadToolResourcesFileSearch
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewAndRunParamsThreadToolResourcesFileSearch) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStore struct {
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,omitzero"`
	// The chunking strategy used to chunk the file(s). If not set, will use the `auto`
	// strategy.
	ChunkingStrategy BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyUnion `json:"chunking_strategy,omitzero"`
	// A list of [file](https://platform.openai.com/docs/api-reference/files) IDs to
	// add to the vector store. There can be a maximum of 10000 files in a vector
	// store.
	FileIDs []string `json:"file_ids,omitzero"`
	paramObj
}

func (r BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStore) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStore
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStore) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyUnion struct {
	OfAuto   *BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyAuto   `json:",omitzero,inline"`
	OfStatic *BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyStatic `json:",omitzero,inline"`
	paramUnion
}

func (u BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfStatic)
}
func (u *BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyUnion) asAny() any {
	if !param.IsOmitted(u.OfAuto) {
		return u.OfAuto
	} else if !param.IsOmitted(u.OfStatic) {
		return u.OfStatic
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyUnion) GetStatic() *BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyStaticStatic {
	if vt := u.OfStatic; vt != nil {
		return &vt.Static
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyUnion) GetType() *string {
	if vt := u.OfAuto; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfStatic; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyUnion](
		"type",
		apijson.Discriminator[BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyAuto]("auto"),
		apijson.Discriminator[BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyStatic]("static"),
	)
}

func NewBetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyAuto() BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyAuto {
	return BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyAuto{
		Type: "auto",
	}
}

// The default strategy. This strategy currently uses a `max_chunk_size_tokens` of
// `800` and `chunk_overlap_tokens` of `400`.
//
// This struct has a constant value, construct it with
// [NewBetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyAuto].
type BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyAuto struct {
	// Always `auto`.
	Type constant.Auto `json:"type,required"`
	paramObj
}

func (r BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyAuto) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyAuto
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyAuto) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Static, Type are required.
type BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyStatic struct {
	Static BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyStaticStatic `json:"static,omitzero,required"`
	// Always `static`.
	//
	// This field can be elided, and will marshal its zero value as "static".
	Type constant.Static `json:"type,required"`
	paramObj
}

func (r BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyStatic) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyStatic
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyStatic) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties ChunkOverlapTokens, MaxChunkSizeTokens are required.
type BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyStaticStatic struct {
	// The number of tokens that overlap between chunks. The default value is `400`.
	//
	// Note that the overlap must not exceed half of `max_chunk_size_tokens`.
	ChunkOverlapTokens int64 `json:"chunk_overlap_tokens,required"`
	// The maximum number of tokens in each chunk. The default value is `800`. The
	// minimum value is `100` and the maximum value is `4096`.
	MaxChunkSizeTokens int64 `json:"max_chunk_size_tokens,required"`
	paramObj
}

func (r BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyStaticStatic) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyStaticStatic
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewAndRunParamsThreadToolResourcesFileSearchVectorStoreChunkingStrategyStaticStatic) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A set of resources that are used by the assistant's tools. The resources are
// specific to the type of tool. For example, the `code_interpreter` tool requires
// a list of file IDs, while the `file_search` tool requires a list of vector store
// IDs.
type BetaThreadNewAndRunParamsToolResources struct {
	CodeInterpreter BetaThreadNewAndRunParamsToolResourcesCodeInterpreter `json:"code_interpreter,omitzero"`
	FileSearch      BetaThreadNewAndRunParamsToolResourcesFileSearch      `json:"file_search,omitzero"`
	paramObj
}

func (r BetaThreadNewAndRunParamsToolResources) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewAndRunParamsToolResources
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewAndRunParamsToolResources) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaThreadNewAndRunParamsToolResourcesCodeInterpreter struct {
	// A list of [file](https://platform.openai.com/docs/api-reference/files) IDs made
	// available to the `code_interpreter` tool. There can be a maximum of 20 files
	// associated with the tool.
	FileIDs []string `json:"file_ids,omitzero"`
	paramObj
}

func (r BetaThreadNewAndRunParamsToolResourcesCodeInterpreter) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewAndRunParamsToolResourcesCodeInterpreter
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewAndRunParamsToolResourcesCodeInterpreter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaThreadNewAndRunParamsToolResourcesFileSearch struct {
	// The ID of the
	// [vector store](https://platform.openai.com/docs/api-reference/vector-stores/object)
	// attached to this assistant. There can be a maximum of 1 vector store attached to
	// the assistant.
	VectorStoreIDs []string `json:"vector_store_ids,omitzero"`
	paramObj
}

func (r BetaThreadNewAndRunParamsToolResourcesFileSearch) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewAndRunParamsToolResourcesFileSearch
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewAndRunParamsToolResourcesFileSearch) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Controls for how a thread will be truncated prior to the run. Use this to
// control the intial context window of the run.
//
// The property Type is required.
type BetaThreadNewAndRunParamsTruncationStrategy struct {
	// The truncation strategy to use for the thread. The default is `auto`. If set to
	// `last_messages`, the thread will be truncated to the n most recent messages in
	// the thread. When set to `auto`, messages in the middle of the thread will be
	// dropped to fit the context length of the model, `max_prompt_tokens`.
	//
	// Any of "auto", "last_messages".
	Type string `json:"type,omitzero,required"`
	// The number of most recent messages from the thread when constructing the context
	// for the run.
	LastMessages param.Opt[int64] `json:"last_messages,omitzero"`
	paramObj
}

func (r BetaThreadNewAndRunParamsTruncationStrategy) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadNewAndRunParamsTruncationStrategy
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadNewAndRunParamsTruncationStrategy) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[BetaThreadNewAndRunParamsTruncationStrategy](
		"type", "auto", "last_messages",
	)
}
