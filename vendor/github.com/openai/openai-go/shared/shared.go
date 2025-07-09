// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package shared

import (
	"encoding/json"

	"github.com/openai/openai-go/internal/apijson"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/respjson"
	"github.com/openai/openai-go/shared/constant"
)

// aliased to make [param.APIUnion] private when embedding
type paramUnion = param.APIUnion
type paramObj = param.APIObject

type ChatModel = string
type ResponsesModel = string

// aliased to make [param.APIObject] private when embedding

const (
	ChatModelGPT4_1                           ChatModel = "gpt-4.1"
	ChatModelGPT4_1Mini                       ChatModel = "gpt-4.1-mini"
	ChatModelGPT4_1Nano                       ChatModel = "gpt-4.1-nano"
	ChatModelGPT4_1_2025_04_14                ChatModel = "gpt-4.1-2025-04-14"
	ChatModelGPT4_1Mini2025_04_14             ChatModel = "gpt-4.1-mini-2025-04-14"
	ChatModelGPT4_1Nano2025_04_14             ChatModel = "gpt-4.1-nano-2025-04-14"
	ChatModelO4Mini                           ChatModel = "o4-mini"
	ChatModelO4Mini2025_04_16                 ChatModel = "o4-mini-2025-04-16"
	ChatModelO3                               ChatModel = "o3"
	ChatModelO3_2025_04_16                    ChatModel = "o3-2025-04-16"
	ChatModelO3Mini                           ChatModel = "o3-mini"
	ChatModelO3Mini2025_01_31                 ChatModel = "o3-mini-2025-01-31"
	ChatModelO1                               ChatModel = "o1"
	ChatModelO1_2024_12_17                    ChatModel = "o1-2024-12-17"
	ChatModelO1Preview                        ChatModel = "o1-preview"
	ChatModelO1Preview2024_09_12              ChatModel = "o1-preview-2024-09-12"
	ChatModelO1Mini                           ChatModel = "o1-mini"
	ChatModelO1Mini2024_09_12                 ChatModel = "o1-mini-2024-09-12"
	ChatModelGPT4o                            ChatModel = "gpt-4o"
	ChatModelGPT4o2024_11_20                  ChatModel = "gpt-4o-2024-11-20"
	ChatModelGPT4o2024_08_06                  ChatModel = "gpt-4o-2024-08-06"
	ChatModelGPT4o2024_05_13                  ChatModel = "gpt-4o-2024-05-13"
	ChatModelGPT4oAudioPreview                ChatModel = "gpt-4o-audio-preview"
	ChatModelGPT4oAudioPreview2024_10_01      ChatModel = "gpt-4o-audio-preview-2024-10-01"
	ChatModelGPT4oAudioPreview2024_12_17      ChatModel = "gpt-4o-audio-preview-2024-12-17"
	ChatModelGPT4oAudioPreview2025_06_03      ChatModel = "gpt-4o-audio-preview-2025-06-03"
	ChatModelGPT4oMiniAudioPreview            ChatModel = "gpt-4o-mini-audio-preview"
	ChatModelGPT4oMiniAudioPreview2024_12_17  ChatModel = "gpt-4o-mini-audio-preview-2024-12-17"
	ChatModelGPT4oSearchPreview               ChatModel = "gpt-4o-search-preview"
	ChatModelGPT4oMiniSearchPreview           ChatModel = "gpt-4o-mini-search-preview"
	ChatModelGPT4oSearchPreview2025_03_11     ChatModel = "gpt-4o-search-preview-2025-03-11"
	ChatModelGPT4oMiniSearchPreview2025_03_11 ChatModel = "gpt-4o-mini-search-preview-2025-03-11"
	ChatModelChatgpt4oLatest                  ChatModel = "chatgpt-4o-latest"
	ChatModelCodexMiniLatest                  ChatModel = "codex-mini-latest"
	ChatModelGPT4oMini                        ChatModel = "gpt-4o-mini"
	ChatModelGPT4oMini2024_07_18              ChatModel = "gpt-4o-mini-2024-07-18"
	ChatModelGPT4Turbo                        ChatModel = "gpt-4-turbo"
	ChatModelGPT4Turbo2024_04_09              ChatModel = "gpt-4-turbo-2024-04-09"
	ChatModelGPT4_0125Preview                 ChatModel = "gpt-4-0125-preview"
	ChatModelGPT4TurboPreview                 ChatModel = "gpt-4-turbo-preview"
	ChatModelGPT4_1106Preview                 ChatModel = "gpt-4-1106-preview"
	ChatModelGPT4VisionPreview                ChatModel = "gpt-4-vision-preview"
	ChatModelGPT4                             ChatModel = "gpt-4"
	ChatModelGPT4_0314                        ChatModel = "gpt-4-0314"
	ChatModelGPT4_0613                        ChatModel = "gpt-4-0613"
	ChatModelGPT4_32k                         ChatModel = "gpt-4-32k"
	ChatModelGPT4_32k0314                     ChatModel = "gpt-4-32k-0314"
	ChatModelGPT4_32k0613                     ChatModel = "gpt-4-32k-0613"
	ChatModelGPT3_5Turbo                      ChatModel = "gpt-3.5-turbo"
	ChatModelGPT3_5Turbo16k                   ChatModel = "gpt-3.5-turbo-16k"
	ChatModelGPT3_5Turbo0301                  ChatModel = "gpt-3.5-turbo-0301"
	ChatModelGPT3_5Turbo0613                  ChatModel = "gpt-3.5-turbo-0613"
	ChatModelGPT3_5Turbo1106                  ChatModel = "gpt-3.5-turbo-1106"
	ChatModelGPT3_5Turbo0125                  ChatModel = "gpt-3.5-turbo-0125"
	ChatModelGPT3_5Turbo16k0613               ChatModel = "gpt-3.5-turbo-16k-0613"
)

// A filter used to compare a specified attribute key to a given value using a
// defined comparison operation.
type ComparisonFilter struct {
	// The key to compare against the value.
	Key string `json:"key,required"`
	// Specifies the comparison operator: `eq`, `ne`, `gt`, `gte`, `lt`, `lte`.
	//
	// - `eq`: equals
	// - `ne`: not equal
	// - `gt`: greater than
	// - `gte`: greater than or equal
	// - `lt`: less than
	// - `lte`: less than or equal
	//
	// Any of "eq", "ne", "gt", "gte", "lt", "lte".
	Type ComparisonFilterType `json:"type,required"`
	// The value to compare against the attribute key; supports string, number, or
	// boolean types.
	Value ComparisonFilterValueUnion `json:"value,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Key         respjson.Field
		Type        respjson.Field
		Value       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ComparisonFilter) RawJSON() string { return r.JSON.raw }
func (r *ComparisonFilter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ComparisonFilter to a ComparisonFilterParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ComparisonFilterParam.Overrides()
func (r ComparisonFilter) ToParam() ComparisonFilterParam {
	return param.Override[ComparisonFilterParam](json.RawMessage(r.RawJSON()))
}

// Specifies the comparison operator: `eq`, `ne`, `gt`, `gte`, `lt`, `lte`.
//
// - `eq`: equals
// - `ne`: not equal
// - `gt`: greater than
// - `gte`: greater than or equal
// - `lt`: less than
// - `lte`: less than or equal
type ComparisonFilterType string

const (
	ComparisonFilterTypeEq  ComparisonFilterType = "eq"
	ComparisonFilterTypeNe  ComparisonFilterType = "ne"
	ComparisonFilterTypeGt  ComparisonFilterType = "gt"
	ComparisonFilterTypeGte ComparisonFilterType = "gte"
	ComparisonFilterTypeLt  ComparisonFilterType = "lt"
	ComparisonFilterTypeLte ComparisonFilterType = "lte"
)

// ComparisonFilterValueUnion contains all possible properties and values from
// [string], [float64], [bool].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfString OfFloat OfBool]
type ComparisonFilterValueUnion struct {
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

func (u ComparisonFilterValueUnion) AsString() (v string) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ComparisonFilterValueUnion) AsFloat() (v float64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ComparisonFilterValueUnion) AsBool() (v bool) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ComparisonFilterValueUnion) RawJSON() string { return u.JSON.raw }

func (r *ComparisonFilterValueUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A filter used to compare a specified attribute key to a given value using a
// defined comparison operation.
//
// The properties Key, Type, Value are required.
type ComparisonFilterParam struct {
	// The key to compare against the value.
	Key string `json:"key,required"`
	// Specifies the comparison operator: `eq`, `ne`, `gt`, `gte`, `lt`, `lte`.
	//
	// - `eq`: equals
	// - `ne`: not equal
	// - `gt`: greater than
	// - `gte`: greater than or equal
	// - `lt`: less than
	// - `lte`: less than or equal
	//
	// Any of "eq", "ne", "gt", "gte", "lt", "lte".
	Type ComparisonFilterType `json:"type,omitzero,required"`
	// The value to compare against the attribute key; supports string, number, or
	// boolean types.
	Value ComparisonFilterValueUnionParam `json:"value,omitzero,required"`
	paramObj
}

func (r ComparisonFilterParam) MarshalJSON() (data []byte, err error) {
	type shadow ComparisonFilterParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ComparisonFilterParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ComparisonFilterValueUnionParam struct {
	OfString param.Opt[string]  `json:",omitzero,inline"`
	OfFloat  param.Opt[float64] `json:",omitzero,inline"`
	OfBool   param.Opt[bool]    `json:",omitzero,inline"`
	paramUnion
}

func (u ComparisonFilterValueUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfFloat, u.OfBool)
}
func (u *ComparisonFilterValueUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ComparisonFilterValueUnionParam) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfFloat) {
		return &u.OfFloat.Value
	} else if !param.IsOmitted(u.OfBool) {
		return &u.OfBool.Value
	}
	return nil
}

// Combine multiple filters using `and` or `or`.
type CompoundFilter struct {
	// Array of filters to combine. Items can be `ComparisonFilter` or
	// `CompoundFilter`.
	Filters []ComparisonFilter `json:"filters,required"`
	// Type of operation: `and` or `or`.
	//
	// Any of "and", "or".
	Type CompoundFilterType `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Filters     respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r CompoundFilter) RawJSON() string { return r.JSON.raw }
func (r *CompoundFilter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this CompoundFilter to a CompoundFilterParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// CompoundFilterParam.Overrides()
func (r CompoundFilter) ToParam() CompoundFilterParam {
	return param.Override[CompoundFilterParam](json.RawMessage(r.RawJSON()))
}

// Type of operation: `and` or `or`.
type CompoundFilterType string

const (
	CompoundFilterTypeAnd CompoundFilterType = "and"
	CompoundFilterTypeOr  CompoundFilterType = "or"
)

// Combine multiple filters using `and` or `or`.
//
// The properties Filters, Type are required.
type CompoundFilterParam struct {
	// Array of filters to combine. Items can be `ComparisonFilter` or
	// `CompoundFilter`.
	Filters []ComparisonFilterParam `json:"filters,omitzero,required"`
	// Type of operation: `and` or `or`.
	//
	// Any of "and", "or".
	Type CompoundFilterType `json:"type,omitzero,required"`
	paramObj
}

func (r CompoundFilterParam) MarshalJSON() (data []byte, err error) {
	type shadow CompoundFilterParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *CompoundFilterParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ErrorObject struct {
	Code    string `json:"code,required"`
	Message string `json:"message,required"`
	Param   string `json:"param,required"`
	Type    string `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Code        respjson.Field
		Message     respjson.Field
		Param       respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ErrorObject) RawJSON() string { return r.JSON.raw }
func (r *ErrorObject) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type FunctionDefinition struct {
	// The name of the function to be called. Must be a-z, A-Z, 0-9, or contain
	// underscores and dashes, with a maximum length of 64.
	Name string `json:"name,required"`
	// A description of what the function does, used by the model to choose when and
	// how to call the function.
	Description string `json:"description"`
	// The parameters the functions accepts, described as a JSON Schema object. See the
	// [guide](https://platform.openai.com/docs/guides/function-calling) for examples,
	// and the
	// [JSON Schema reference](https://json-schema.org/understanding-json-schema/) for
	// documentation about the format.
	//
	// Omitting `parameters` defines a function with an empty parameter list.
	Parameters FunctionParameters `json:"parameters"`
	// Whether to enable strict schema adherence when generating the function call. If
	// set to true, the model will follow the exact schema defined in the `parameters`
	// field. Only a subset of JSON Schema is supported when `strict` is `true`. Learn
	// more about Structured Outputs in the
	// [function calling guide](docs/guides/function-calling).
	Strict bool `json:"strict,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Name        respjson.Field
		Description respjson.Field
		Parameters  respjson.Field
		Strict      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FunctionDefinition) RawJSON() string { return r.JSON.raw }
func (r *FunctionDefinition) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this FunctionDefinition to a FunctionDefinitionParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// FunctionDefinitionParam.Overrides()
func (r FunctionDefinition) ToParam() FunctionDefinitionParam {
	return param.Override[FunctionDefinitionParam](json.RawMessage(r.RawJSON()))
}

// The property Name is required.
type FunctionDefinitionParam struct {
	// The name of the function to be called. Must be a-z, A-Z, 0-9, or contain
	// underscores and dashes, with a maximum length of 64.
	Name string `json:"name,required"`
	// Whether to enable strict schema adherence when generating the function call. If
	// set to true, the model will follow the exact schema defined in the `parameters`
	// field. Only a subset of JSON Schema is supported when `strict` is `true`. Learn
	// more about Structured Outputs in the
	// [function calling guide](docs/guides/function-calling).
	Strict param.Opt[bool] `json:"strict,omitzero"`
	// A description of what the function does, used by the model to choose when and
	// how to call the function.
	Description param.Opt[string] `json:"description,omitzero"`
	// The parameters the functions accepts, described as a JSON Schema object. See the
	// [guide](https://platform.openai.com/docs/guides/function-calling) for examples,
	// and the
	// [JSON Schema reference](https://json-schema.org/understanding-json-schema/) for
	// documentation about the format.
	//
	// Omitting `parameters` defines a function with an empty parameter list.
	Parameters FunctionParameters `json:"parameters,omitzero"`
	paramObj
}

func (r FunctionDefinitionParam) MarshalJSON() (data []byte, err error) {
	type shadow FunctionDefinitionParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *FunctionDefinitionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type FunctionParameters map[string]any

type Metadata map[string]string

// **o-series models only**
//
// Configuration options for
// [reasoning models](https://platform.openai.com/docs/guides/reasoning).
type Reasoning struct {
	// **o-series models only**
	//
	// Constrains effort on reasoning for
	// [reasoning models](https://platform.openai.com/docs/guides/reasoning). Currently
	// supported values are `low`, `medium`, and `high`. Reducing reasoning effort can
	// result in faster responses and fewer tokens used on reasoning in a response.
	//
	// Any of "low", "medium", "high".
	Effort ReasoningEffort `json:"effort,nullable"`
	// **Deprecated:** use `summary` instead.
	//
	// A summary of the reasoning performed by the model. This can be useful for
	// debugging and understanding the model's reasoning process. One of `auto`,
	// `concise`, or `detailed`.
	//
	// Any of "auto", "concise", "detailed".
	//
	// Deprecated: deprecated
	GenerateSummary ReasoningGenerateSummary `json:"generate_summary,nullable"`
	// A summary of the reasoning performed by the model. This can be useful for
	// debugging and understanding the model's reasoning process. One of `auto`,
	// `concise`, or `detailed`.
	//
	// Any of "auto", "concise", "detailed".
	Summary ReasoningSummary `json:"summary,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Effort          respjson.Field
		GenerateSummary respjson.Field
		Summary         respjson.Field
		ExtraFields     map[string]respjson.Field
		raw             string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r Reasoning) RawJSON() string { return r.JSON.raw }
func (r *Reasoning) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this Reasoning to a ReasoningParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ReasoningParam.Overrides()
func (r Reasoning) ToParam() ReasoningParam {
	return param.Override[ReasoningParam](json.RawMessage(r.RawJSON()))
}

// **Deprecated:** use `summary` instead.
//
// A summary of the reasoning performed by the model. This can be useful for
// debugging and understanding the model's reasoning process. One of `auto`,
// `concise`, or `detailed`.
type ReasoningGenerateSummary string

const (
	ReasoningGenerateSummaryAuto     ReasoningGenerateSummary = "auto"
	ReasoningGenerateSummaryConcise  ReasoningGenerateSummary = "concise"
	ReasoningGenerateSummaryDetailed ReasoningGenerateSummary = "detailed"
)

// A summary of the reasoning performed by the model. This can be useful for
// debugging and understanding the model's reasoning process. One of `auto`,
// `concise`, or `detailed`.
type ReasoningSummary string

const (
	ReasoningSummaryAuto     ReasoningSummary = "auto"
	ReasoningSummaryConcise  ReasoningSummary = "concise"
	ReasoningSummaryDetailed ReasoningSummary = "detailed"
)

// **o-series models only**
//
// Configuration options for
// [reasoning models](https://platform.openai.com/docs/guides/reasoning).
type ReasoningParam struct {
	// **o-series models only**
	//
	// Constrains effort on reasoning for
	// [reasoning models](https://platform.openai.com/docs/guides/reasoning). Currently
	// supported values are `low`, `medium`, and `high`. Reducing reasoning effort can
	// result in faster responses and fewer tokens used on reasoning in a response.
	//
	// Any of "low", "medium", "high".
	Effort ReasoningEffort `json:"effort,omitzero"`
	// **Deprecated:** use `summary` instead.
	//
	// A summary of the reasoning performed by the model. This can be useful for
	// debugging and understanding the model's reasoning process. One of `auto`,
	// `concise`, or `detailed`.
	//
	// Any of "auto", "concise", "detailed".
	//
	// Deprecated: deprecated
	GenerateSummary ReasoningGenerateSummary `json:"generate_summary,omitzero"`
	// A summary of the reasoning performed by the model. This can be useful for
	// debugging and understanding the model's reasoning process. One of `auto`,
	// `concise`, or `detailed`.
	//
	// Any of "auto", "concise", "detailed".
	Summary ReasoningSummary `json:"summary,omitzero"`
	paramObj
}

func (r ReasoningParam) MarshalJSON() (data []byte, err error) {
	type shadow ReasoningParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ReasoningParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// **o-series models only**
//
// Constrains effort on reasoning for
// [reasoning models](https://platform.openai.com/docs/guides/reasoning). Currently
// supported values are `low`, `medium`, and `high`. Reducing reasoning effort can
// result in faster responses and fewer tokens used on reasoning in a response.
type ReasoningEffort string

const (
	ReasoningEffortLow    ReasoningEffort = "low"
	ReasoningEffortMedium ReasoningEffort = "medium"
	ReasoningEffortHigh   ReasoningEffort = "high"
)

// JSON object response format. An older method of generating JSON responses. Using
// `json_schema` is recommended for models that support it. Note that the model
// will not generate JSON without a system or user message instructing it to do so.
type ResponseFormatJSONObject struct {
	// The type of response format being defined. Always `json_object`.
	Type constant.JSONObject `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFormatJSONObject) RawJSON() string { return r.JSON.raw }
func (r *ResponseFormatJSONObject) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func (ResponseFormatJSONObject) ImplResponseFormatTextConfigUnion() {}

// ToParam converts this ResponseFormatJSONObject to a
// ResponseFormatJSONObjectParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseFormatJSONObjectParam.Overrides()
func (r ResponseFormatJSONObject) ToParam() ResponseFormatJSONObjectParam {
	return param.Override[ResponseFormatJSONObjectParam](json.RawMessage(r.RawJSON()))
}

func NewResponseFormatJSONObjectParam() ResponseFormatJSONObjectParam {
	return ResponseFormatJSONObjectParam{
		Type: "json_object",
	}
}

// JSON object response format. An older method of generating JSON responses. Using
// `json_schema` is recommended for models that support it. Note that the model
// will not generate JSON without a system or user message instructing it to do so.
//
// This struct has a constant value, construct it with
// [NewResponseFormatJSONObjectParam].
type ResponseFormatJSONObjectParam struct {
	// The type of response format being defined. Always `json_object`.
	Type constant.JSONObject `json:"type,required"`
	paramObj
}

func (r ResponseFormatJSONObjectParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseFormatJSONObjectParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseFormatJSONObjectParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// JSON Schema response format. Used to generate structured JSON responses. Learn
// more about
// [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs).
type ResponseFormatJSONSchema struct {
	// Structured Outputs configuration options, including a JSON Schema.
	JSONSchema ResponseFormatJSONSchemaJSONSchema `json:"json_schema,required"`
	// The type of response format being defined. Always `json_schema`.
	Type constant.JSONSchema `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		JSONSchema  respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFormatJSONSchema) RawJSON() string { return r.JSON.raw }
func (r *ResponseFormatJSONSchema) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ResponseFormatJSONSchema to a
// ResponseFormatJSONSchemaParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseFormatJSONSchemaParam.Overrides()
func (r ResponseFormatJSONSchema) ToParam() ResponseFormatJSONSchemaParam {
	return param.Override[ResponseFormatJSONSchemaParam](json.RawMessage(r.RawJSON()))
}

// Structured Outputs configuration options, including a JSON Schema.
type ResponseFormatJSONSchemaJSONSchema struct {
	// The name of the response format. Must be a-z, A-Z, 0-9, or contain underscores
	// and dashes, with a maximum length of 64.
	Name string `json:"name,required"`
	// A description of what the response format is for, used by the model to determine
	// how to respond in the format.
	Description string `json:"description"`
	// The schema for the response format, described as a JSON Schema object. Learn how
	// to build JSON schemas [here](https://json-schema.org/).
	Schema map[string]any `json:"schema"`
	// Whether to enable strict schema adherence when generating the output. If set to
	// true, the model will always follow the exact schema defined in the `schema`
	// field. Only a subset of JSON Schema is supported when `strict` is `true`. To
	// learn more, read the
	// [Structured Outputs guide](https://platform.openai.com/docs/guides/structured-outputs).
	Strict bool `json:"strict,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Name        respjson.Field
		Description respjson.Field
		Schema      respjson.Field
		Strict      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFormatJSONSchemaJSONSchema) RawJSON() string { return r.JSON.raw }
func (r *ResponseFormatJSONSchemaJSONSchema) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// JSON Schema response format. Used to generate structured JSON responses. Learn
// more about
// [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs).
//
// The properties JSONSchema, Type are required.
type ResponseFormatJSONSchemaParam struct {
	// Structured Outputs configuration options, including a JSON Schema.
	JSONSchema ResponseFormatJSONSchemaJSONSchemaParam `json:"json_schema,omitzero,required"`
	// The type of response format being defined. Always `json_schema`.
	//
	// This field can be elided, and will marshal its zero value as "json_schema".
	Type constant.JSONSchema `json:"type,required"`
	paramObj
}

func (r ResponseFormatJSONSchemaParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseFormatJSONSchemaParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseFormatJSONSchemaParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Structured Outputs configuration options, including a JSON Schema.
//
// The property Name is required.
type ResponseFormatJSONSchemaJSONSchemaParam struct {
	// The name of the response format. Must be a-z, A-Z, 0-9, or contain underscores
	// and dashes, with a maximum length of 64.
	Name string `json:"name,required"`
	// Whether to enable strict schema adherence when generating the output. If set to
	// true, the model will always follow the exact schema defined in the `schema`
	// field. Only a subset of JSON Schema is supported when `strict` is `true`. To
	// learn more, read the
	// [Structured Outputs guide](https://platform.openai.com/docs/guides/structured-outputs).
	Strict param.Opt[bool] `json:"strict,omitzero"`
	// A description of what the response format is for, used by the model to determine
	// how to respond in the format.
	Description param.Opt[string] `json:"description,omitzero"`
	// The schema for the response format, described as a JSON Schema object. Learn how
	// to build JSON schemas [here](https://json-schema.org/).
	Schema any `json:"schema,omitzero"`
	paramObj
}

func (r ResponseFormatJSONSchemaJSONSchemaParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseFormatJSONSchemaJSONSchemaParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseFormatJSONSchemaJSONSchemaParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Default response format. Used to generate text responses.
type ResponseFormatText struct {
	// The type of response format being defined. Always `text`.
	Type constant.Text `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFormatText) RawJSON() string { return r.JSON.raw }
func (r *ResponseFormatText) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func (ResponseFormatText) ImplResponseFormatTextConfigUnion() {}

// ToParam converts this ResponseFormatText to a ResponseFormatTextParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ResponseFormatTextParam.Overrides()
func (r ResponseFormatText) ToParam() ResponseFormatTextParam {
	return param.Override[ResponseFormatTextParam](json.RawMessage(r.RawJSON()))
}

func NewResponseFormatTextParam() ResponseFormatTextParam {
	return ResponseFormatTextParam{
		Type: "text",
	}
}

// Default response format. Used to generate text responses.
//
// This struct has a constant value, construct it with
// [NewResponseFormatTextParam].
type ResponseFormatTextParam struct {
	// The type of response format being defined. Always `text`.
	Type constant.Text `json:"type,required"`
	paramObj
}

func (r ResponseFormatTextParam) MarshalJSON() (data []byte, err error) {
	type shadow ResponseFormatTextParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ResponseFormatTextParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ResponsesModel also accepts any [string] or [ChatModel]

const (
	ResponsesModelO1Pro                        ResponsesModel = "o1-pro"
	ResponsesModelO1Pro2025_03_19              ResponsesModel = "o1-pro-2025-03-19"
	ResponsesModelO3Pro                        ResponsesModel = "o3-pro"
	ResponsesModelO3Pro2025_06_10              ResponsesModel = "o3-pro-2025-06-10"
	ResponsesModelO3DeepResearch               ResponsesModel = "o3-deep-research"
	ResponsesModelO3DeepResearch2025_06_26     ResponsesModel = "o3-deep-research-2025-06-26"
	ResponsesModelO4MiniDeepResearch           ResponsesModel = "o4-mini-deep-research"
	ResponsesModelO4MiniDeepResearch2025_06_26 ResponsesModel = "o4-mini-deep-research-2025-06-26"
	ResponsesModelComputerUsePreview           ResponsesModel = "computer-use-preview"
	ResponsesModelComputerUsePreview2025_03_11 ResponsesModel = "computer-use-preview-2025-03-11"
	// Or some ...[ChatModel]
)
