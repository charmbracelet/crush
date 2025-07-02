// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package responses

import (
	"github.com/openai/openai-go/internal/apierror"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/resp"
	"github.com/openai/openai-go/shared"
)

// aliased to make [param.APIUnion] private when embedding
type paramUnion = param.APIUnion

// aliased to make [param.APIObject] private when embedding
type paramObj = param.APIObject

type Error = apierror.Error

// This is an alias to an internal type.
type ChatModel = shared.ChatModel

// Equals "o3-mini"
const ChatModelO3Mini = shared.ChatModelO3Mini

// Equals "o3-mini-2025-01-31"
const ChatModelO3Mini2025_01_31 = shared.ChatModelO3Mini2025_01_31

// Equals "o1"
const ChatModelO1 = shared.ChatModelO1

// Equals "o1-2024-12-17"
const ChatModelO1_2024_12_17 = shared.ChatModelO1_2024_12_17

// Equals "o1-preview"
const ChatModelO1Preview = shared.ChatModelO1Preview

// Equals "o1-preview-2024-09-12"
const ChatModelO1Preview2024_09_12 = shared.ChatModelO1Preview2024_09_12

// Equals "o1-mini"
const ChatModelO1Mini = shared.ChatModelO1Mini

// Equals "o1-mini-2024-09-12"
const ChatModelO1Mini2024_09_12 = shared.ChatModelO1Mini2024_09_12

// Equals "gpt-4o"
const ChatModelGPT4o = shared.ChatModelGPT4o

// Equals "gpt-4o-2024-11-20"
const ChatModelGPT4o2024_11_20 = shared.ChatModelGPT4o2024_11_20

// Equals "gpt-4o-2024-08-06"
const ChatModelGPT4o2024_08_06 = shared.ChatModelGPT4o2024_08_06

// Equals "gpt-4o-2024-05-13"
const ChatModelGPT4o2024_05_13 = shared.ChatModelGPT4o2024_05_13

// Equals "gpt-4o-audio-preview"
const ChatModelGPT4oAudioPreview = shared.ChatModelGPT4oAudioPreview

// Equals "gpt-4o-audio-preview-2024-10-01"
const ChatModelGPT4oAudioPreview2024_10_01 = shared.ChatModelGPT4oAudioPreview2024_10_01

// Equals "gpt-4o-audio-preview-2024-12-17"
const ChatModelGPT4oAudioPreview2024_12_17 = shared.ChatModelGPT4oAudioPreview2024_12_17

// Equals "gpt-4o-mini-audio-preview"
const ChatModelGPT4oMiniAudioPreview = shared.ChatModelGPT4oMiniAudioPreview

// Equals "gpt-4o-mini-audio-preview-2024-12-17"
const ChatModelGPT4oMiniAudioPreview2024_12_17 = shared.ChatModelGPT4oMiniAudioPreview2024_12_17

// Equals "gpt-4o-search-preview"
const ChatModelGPT4oSearchPreview = shared.ChatModelGPT4oSearchPreview

// Equals "gpt-4o-mini-search-preview"
const ChatModelGPT4oMiniSearchPreview = shared.ChatModelGPT4oMiniSearchPreview

// Equals "gpt-4o-search-preview-2025-03-11"
const ChatModelGPT4oSearchPreview2025_03_11 = shared.ChatModelGPT4oSearchPreview2025_03_11

// Equals "gpt-4o-mini-search-preview-2025-03-11"
const ChatModelGPT4oMiniSearchPreview2025_03_11 = shared.ChatModelGPT4oMiniSearchPreview2025_03_11

// Equals "chatgpt-4o-latest"
const ChatModelChatgpt4oLatest = shared.ChatModelChatgpt4oLatest

// Equals "gpt-4o-mini"
const ChatModelGPT4oMini = shared.ChatModelGPT4oMini

// Equals "gpt-4o-mini-2024-07-18"
const ChatModelGPT4oMini2024_07_18 = shared.ChatModelGPT4oMini2024_07_18

// Equals "gpt-4-turbo"
const ChatModelGPT4Turbo = shared.ChatModelGPT4Turbo

// Equals "gpt-4-turbo-2024-04-09"
const ChatModelGPT4Turbo2024_04_09 = shared.ChatModelGPT4Turbo2024_04_09

// Equals "gpt-4-0125-preview"
const ChatModelGPT4_0125Preview = shared.ChatModelGPT4_0125Preview

// Equals "gpt-4-turbo-preview"
const ChatModelGPT4TurboPreview = shared.ChatModelGPT4TurboPreview

// Equals "gpt-4-1106-preview"
const ChatModelGPT4_1106Preview = shared.ChatModelGPT4_1106Preview

// Equals "gpt-4-vision-preview"
const ChatModelGPT4VisionPreview = shared.ChatModelGPT4VisionPreview

// Equals "gpt-4"
const ChatModelGPT4 = shared.ChatModelGPT4

// Equals "gpt-4-0314"
const ChatModelGPT4_0314 = shared.ChatModelGPT4_0314

// Equals "gpt-4-0613"
const ChatModelGPT4_0613 = shared.ChatModelGPT4_0613

// Equals "gpt-4-32k"
const ChatModelGPT4_32k = shared.ChatModelGPT4_32k

// Equals "gpt-4-32k-0314"
const ChatModelGPT4_32k0314 = shared.ChatModelGPT4_32k0314

// Equals "gpt-4-32k-0613"
const ChatModelGPT4_32k0613 = shared.ChatModelGPT4_32k0613

// Equals "gpt-3.5-turbo"
const ChatModelGPT3_5Turbo = shared.ChatModelGPT3_5Turbo

// Equals "gpt-3.5-turbo-16k"
const ChatModelGPT3_5Turbo16k = shared.ChatModelGPT3_5Turbo16k

// Equals "gpt-3.5-turbo-0301"
const ChatModelGPT3_5Turbo0301 = shared.ChatModelGPT3_5Turbo0301

// Equals "gpt-3.5-turbo-0613"
const ChatModelGPT3_5Turbo0613 = shared.ChatModelGPT3_5Turbo0613

// Equals "gpt-3.5-turbo-1106"
const ChatModelGPT3_5Turbo1106 = shared.ChatModelGPT3_5Turbo1106

// Equals "gpt-3.5-turbo-0125"
const ChatModelGPT3_5Turbo0125 = shared.ChatModelGPT3_5Turbo0125

// Equals "gpt-3.5-turbo-16k-0613"
const ChatModelGPT3_5Turbo16k0613 = shared.ChatModelGPT3_5Turbo16k0613

// A filter used to compare a specified attribute key to a given value using a
// defined comparison operation.
//
// This is an alias to an internal type.
type ComparisonFilter = shared.ComparisonFilter

// Specifies the comparison operator: `eq`, `ne`, `gt`, `gte`, `lt`, `lte`.
//
// - `eq`: equals
// - `ne`: not equal
// - `gt`: greater than
// - `gte`: greater than or equal
// - `lt`: less than
// - `lte`: less than or equal
//
// This is an alias to an internal type.
type ComparisonFilterType = shared.ComparisonFilterType

// Equals "eq"
const ComparisonFilterTypeEq = shared.ComparisonFilterTypeEq

// Equals "ne"
const ComparisonFilterTypeNe = shared.ComparisonFilterTypeNe

// Equals "gt"
const ComparisonFilterTypeGt = shared.ComparisonFilterTypeGt

// Equals "gte"
const ComparisonFilterTypeGte = shared.ComparisonFilterTypeGte

// Equals "lt"
const ComparisonFilterTypeLt = shared.ComparisonFilterTypeLt

// Equals "lte"
const ComparisonFilterTypeLte = shared.ComparisonFilterTypeLte

// The value to compare against the attribute key; supports string, number, or
// boolean types.
//
// This is an alias to an internal type.
type ComparisonFilterValueUnion = shared.ComparisonFilterValueUnion

// A filter used to compare a specified attribute key to a given value using a
// defined comparison operation.
//
// This is an alias to an internal type.
type ComparisonFilterParam = shared.ComparisonFilterParam

// The value to compare against the attribute key; supports string, number, or
// boolean types.
//
// This is an alias to an internal type.
type ComparisonFilterValueUnionParam = shared.ComparisonFilterValueUnionParam

// Combine multiple filters using `and` or `or`.
//
// This is an alias to an internal type.
type CompoundFilter = shared.CompoundFilter

// Type of operation: `and` or `or`.
//
// This is an alias to an internal type.
type CompoundFilterType = shared.CompoundFilterType

// Equals "and"
const CompoundFilterTypeAnd = shared.CompoundFilterTypeAnd

// Equals "or"
const CompoundFilterTypeOr = shared.CompoundFilterTypeOr

// Combine multiple filters using `and` or `or`.
//
// This is an alias to an internal type.
type CompoundFilterParam = shared.CompoundFilterParam

// This is an alias to an internal type.
type ErrorObject = shared.ErrorObject

// This is an alias to an internal type.
type FunctionDefinition = shared.FunctionDefinition

// This is an alias to an internal type.
type FunctionDefinitionParam = shared.FunctionDefinitionParam

// The parameters the functions accepts, described as a JSON Schema object. See the
// [guide](https://platform.openai.com/docs/guides/function-calling) for examples,
// and the
// [JSON Schema reference](https://json-schema.org/understanding-json-schema/) for
// documentation about the format.
//
// Omitting `parameters` defines a function with an empty parameter list.
//
// This is an alias to an internal type.
type FunctionParameters = shared.FunctionParameters

// Set of 16 key-value pairs that can be attached to an object. This can be useful
// for storing additional information about the object in a structured format, and
// querying for objects via API or the dashboard.
//
// Keys are strings with a maximum length of 64 characters. Values are strings with
// a maximum length of 512 characters.
//
// This is an alias to an internal type.
type Metadata = shared.Metadata

// Set of 16 key-value pairs that can be attached to an object. This can be useful
// for storing additional information about the object in a structured format, and
// querying for objects via API or the dashboard.
//
// Keys are strings with a maximum length of 64 characters. Values are strings with
// a maximum length of 512 characters.
//
// This is an alias to an internal type.
type MetadataParam = shared.MetadataParam

// **o-series models only**
//
// Configuration options for
// [reasoning models](https://platform.openai.com/docs/guides/reasoning).
//
// This is an alias to an internal type.
type Reasoning = shared.Reasoning

// **computer_use_preview only**
//
// A summary of the reasoning performed by the model. This can be useful for
// debugging and understanding the model's reasoning process. One of `concise` or
// `detailed`.
//
// This is an alias to an internal type.
type ReasoningGenerateSummary = shared.ReasoningGenerateSummary

// Equals "concise"
const ReasoningGenerateSummaryConcise = shared.ReasoningGenerateSummaryConcise

// Equals "detailed"
const ReasoningGenerateSummaryDetailed = shared.ReasoningGenerateSummaryDetailed

// **o-series models only**
//
// Configuration options for
// [reasoning models](https://platform.openai.com/docs/guides/reasoning).
//
// This is an alias to an internal type.
type ReasoningParam = shared.ReasoningParam

// **o-series models only**
//
// Constrains effort on reasoning for
// [reasoning models](https://platform.openai.com/docs/guides/reasoning). Currently
// supported values are `low`, `medium`, and `high`. Reducing reasoning effort can
// result in faster responses and fewer tokens used on reasoning in a response.
//
// This is an alias to an internal type.
type ReasoningEffort = shared.ReasoningEffort

// Equals "low"
const ReasoningEffortLow = shared.ReasoningEffortLow

// Equals "medium"
const ReasoningEffortMedium = shared.ReasoningEffortMedium

// Equals "high"
const ReasoningEffortHigh = shared.ReasoningEffortHigh

// JSON object response format. An older method of generating JSON responses. Using
// `json_schema` is recommended for models that support it. Note that the model
// will not generate JSON without a system or user message instructing it to do so.
//
// This is an alias to an internal type.
type ResponseFormatJSONObject = shared.ResponseFormatJSONObject

// JSON object response format. An older method of generating JSON responses. Using
// `json_schema` is recommended for models that support it. Note that the model
// will not generate JSON without a system or user message instructing it to do so.
//
// This is an alias to an internal type.
type ResponseFormatJSONObjectParam = shared.ResponseFormatJSONObjectParam

// JSON Schema response format. Used to generate structured JSON responses. Learn
// more about
// [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs).
//
// This is an alias to an internal type.
type ResponseFormatJSONSchema = shared.ResponseFormatJSONSchema

// Structured Outputs configuration options, including a JSON Schema.
//
// This is an alias to an internal type.
type ResponseFormatJSONSchemaJSONSchema = shared.ResponseFormatJSONSchemaJSONSchema

// JSON Schema response format. Used to generate structured JSON responses. Learn
// more about
// [Structured Outputs](https://platform.openai.com/docs/guides/structured-outputs).
//
// This is an alias to an internal type.
type ResponseFormatJSONSchemaParam = shared.ResponseFormatJSONSchemaParam

// Structured Outputs configuration options, including a JSON Schema.
//
// This is an alias to an internal type.
type ResponseFormatJSONSchemaJSONSchemaParam = shared.ResponseFormatJSONSchemaJSONSchemaParam

// Default response format. Used to generate text responses.
//
// This is an alias to an internal type.
type ResponseFormatText = shared.ResponseFormatText

// Default response format. Used to generate text responses.
//
// This is an alias to an internal type.
type ResponseFormatTextParam = shared.ResponseFormatTextParam

// This is an alias to an internal type.
type ResponsesModel = shared.ResponsesModel

// Equals "o1-pro"
const ResponsesModelO1Pro = shared.ResponsesModelO1Pro

// Equals "o1-pro-2025-03-19"
const ResponsesModelO1Pro2025_03_19 = shared.ResponsesModelO1Pro2025_03_19

// Equals "computer-use-preview"
const ResponsesModelComputerUsePreview = shared.ResponsesModelComputerUsePreview

// Equals "computer-use-preview-2025-03-11"
const ResponsesModelComputerUsePreview2025_03_11 = shared.ResponsesModelComputerUsePreview2025_03_11

func toParam[T comparable](value T, meta resp.Field) param.Opt[T] {
	if meta.IsPresent() {
		return param.NewOpt(value)
	}
	if meta.IsExplicitNull() {
		return param.NullOpt[T]()
	}
	return param.Opt[T]{}
}
