// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package openai

import (
	"context"
	"net/http"

	"github.com/openai/openai-go/internal/apijson"
	"github.com/openai/openai-go/internal/requestconfig"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/respjson"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/shared/constant"
)

// CompletionService contains methods and other services that help with interacting
// with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewCompletionService] method instead.
type CompletionService struct {
	Options []option.RequestOption
}

// NewCompletionService generates a new service that applies the given options to
// each request. These options are applied after the parent client's options (if
// there is one), and before any request-specific options.
func NewCompletionService(opts ...option.RequestOption) (r CompletionService) {
	r = CompletionService{}
	r.Options = opts
	return
}

// Creates a completion for the provided prompt and parameters.
func (r *CompletionService) New(ctx context.Context, body CompletionNewParams, opts ...option.RequestOption) (res *Completion, err error) {
	opts = append(r.Options[:], opts...)
	path := "completions"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// Creates a completion for the provided prompt and parameters.
func (r *CompletionService) NewStreaming(ctx context.Context, body CompletionNewParams, opts ...option.RequestOption) (stream *ssestream.Stream[Completion]) {
	var (
		raw *http.Response
		err error
	)
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithJSONSet("stream", true)}, opts...)
	path := "completions"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &raw, opts...)
	return ssestream.NewStream[Completion](ssestream.NewDecoder(raw), err)
}

// Represents a completion response from the API. Note: both the streamed and
// non-streamed response objects share the same shape (unlike the chat endpoint).
type Completion struct {
	// A unique identifier for the completion.
	ID string `json:"id,required"`
	// The list of completion choices the model generated for the input prompt.
	Choices []CompletionChoice `json:"choices,required"`
	// The Unix timestamp (in seconds) of when the completion was created.
	Created int64 `json:"created,required"`
	// The model used for completion.
	Model string `json:"model,required"`
	// The object type, which is always "text_completion"
	Object constant.TextCompletion `json:"object,required"`
	// This fingerprint represents the backend configuration that the model runs with.
	//
	// Can be used in conjunction with the `seed` request parameter to understand when
	// backend changes have been made that might impact determinism.
	SystemFingerprint string `json:"system_fingerprint"`
	// Usage statistics for the completion request.
	Usage CompletionUsage `json:"usage"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID                respjson.Field
		Choices           respjson.Field
		Created           respjson.Field
		Model             respjson.Field
		Object            respjson.Field
		SystemFingerprint respjson.Field
		Usage             respjson.Field
		ExtraFields       map[string]respjson.Field
		raw               string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r Completion) RawJSON() string { return r.JSON.raw }
func (r *Completion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type CompletionChoice struct {
	// The reason the model stopped generating tokens. This will be `stop` if the model
	// hit a natural stop point or a provided stop sequence, `length` if the maximum
	// number of tokens specified in the request was reached, or `content_filter` if
	// content was omitted due to a flag from our content filters.
	//
	// Any of "stop", "length", "content_filter".
	FinishReason CompletionChoiceFinishReason `json:"finish_reason,required"`
	Index        int64                        `json:"index,required"`
	Logprobs     CompletionChoiceLogprobs     `json:"logprobs,required"`
	Text         string                       `json:"text,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		FinishReason respjson.Field
		Index        respjson.Field
		Logprobs     respjson.Field
		Text         respjson.Field
		ExtraFields  map[string]respjson.Field
		raw          string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r CompletionChoice) RawJSON() string { return r.JSON.raw }
func (r *CompletionChoice) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The reason the model stopped generating tokens. This will be `stop` if the model
// hit a natural stop point or a provided stop sequence, `length` if the maximum
// number of tokens specified in the request was reached, or `content_filter` if
// content was omitted due to a flag from our content filters.
type CompletionChoiceFinishReason string

const (
	CompletionChoiceFinishReasonStop          CompletionChoiceFinishReason = "stop"
	CompletionChoiceFinishReasonLength        CompletionChoiceFinishReason = "length"
	CompletionChoiceFinishReasonContentFilter CompletionChoiceFinishReason = "content_filter"
)

type CompletionChoiceLogprobs struct {
	TextOffset    []int64              `json:"text_offset"`
	TokenLogprobs []float64            `json:"token_logprobs"`
	Tokens        []string             `json:"tokens"`
	TopLogprobs   []map[string]float64 `json:"top_logprobs"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		TextOffset    respjson.Field
		TokenLogprobs respjson.Field
		Tokens        respjson.Field
		TopLogprobs   respjson.Field
		ExtraFields   map[string]respjson.Field
		raw           string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r CompletionChoiceLogprobs) RawJSON() string { return r.JSON.raw }
func (r *CompletionChoiceLogprobs) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Usage statistics for the completion request.
type CompletionUsage struct {
	// Number of tokens in the generated completion.
	CompletionTokens int64 `json:"completion_tokens,required"`
	// Number of tokens in the prompt.
	PromptTokens int64 `json:"prompt_tokens,required"`
	// Total number of tokens used in the request (prompt + completion).
	TotalTokens int64 `json:"total_tokens,required"`
	// Breakdown of tokens used in a completion.
	CompletionTokensDetails CompletionUsageCompletionTokensDetails `json:"completion_tokens_details"`
	// Breakdown of tokens used in the prompt.
	PromptTokensDetails CompletionUsagePromptTokensDetails `json:"prompt_tokens_details"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		CompletionTokens        respjson.Field
		PromptTokens            respjson.Field
		TotalTokens             respjson.Field
		CompletionTokensDetails respjson.Field
		PromptTokensDetails     respjson.Field
		ExtraFields             map[string]respjson.Field
		raw                     string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r CompletionUsage) RawJSON() string { return r.JSON.raw }
func (r *CompletionUsage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Breakdown of tokens used in a completion.
type CompletionUsageCompletionTokensDetails struct {
	// When using Predicted Outputs, the number of tokens in the prediction that
	// appeared in the completion.
	AcceptedPredictionTokens int64 `json:"accepted_prediction_tokens"`
	// Audio input tokens generated by the model.
	AudioTokens int64 `json:"audio_tokens"`
	// Tokens generated by the model for reasoning.
	ReasoningTokens int64 `json:"reasoning_tokens"`
	// When using Predicted Outputs, the number of tokens in the prediction that did
	// not appear in the completion. However, like reasoning tokens, these tokens are
	// still counted in the total completion tokens for purposes of billing, output,
	// and context window limits.
	RejectedPredictionTokens int64 `json:"rejected_prediction_tokens"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		AcceptedPredictionTokens respjson.Field
		AudioTokens              respjson.Field
		ReasoningTokens          respjson.Field
		RejectedPredictionTokens respjson.Field
		ExtraFields              map[string]respjson.Field
		raw                      string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r CompletionUsageCompletionTokensDetails) RawJSON() string { return r.JSON.raw }
func (r *CompletionUsageCompletionTokensDetails) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Breakdown of tokens used in the prompt.
type CompletionUsagePromptTokensDetails struct {
	// Audio input tokens present in the prompt.
	AudioTokens int64 `json:"audio_tokens"`
	// Cached tokens present in the prompt.
	CachedTokens int64 `json:"cached_tokens"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		AudioTokens  respjson.Field
		CachedTokens respjson.Field
		ExtraFields  map[string]respjson.Field
		raw          string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r CompletionUsagePromptTokensDetails) RawJSON() string { return r.JSON.raw }
func (r *CompletionUsagePromptTokensDetails) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type CompletionNewParams struct {
	// The prompt(s) to generate completions for, encoded as a string, array of
	// strings, array of tokens, or array of token arrays.
	//
	// Note that <|endoftext|> is the document separator that the model sees during
	// training, so if a prompt is not specified the model will generate as if from the
	// beginning of a new document.
	Prompt CompletionNewParamsPromptUnion `json:"prompt,omitzero,required"`
	// ID of the model to use. You can use the
	// [List models](https://platform.openai.com/docs/api-reference/models/list) API to
	// see all of your available models, or see our
	// [Model overview](https://platform.openai.com/docs/models) for descriptions of
	// them.
	Model CompletionNewParamsModel `json:"model,omitzero,required"`
	// Generates `best_of` completions server-side and returns the "best" (the one with
	// the highest log probability per token). Results cannot be streamed.
	//
	// When used with `n`, `best_of` controls the number of candidate completions and
	// `n` specifies how many to return – `best_of` must be greater than `n`.
	//
	// **Note:** Because this parameter generates many completions, it can quickly
	// consume your token quota. Use carefully and ensure that you have reasonable
	// settings for `max_tokens` and `stop`.
	BestOf param.Opt[int64] `json:"best_of,omitzero"`
	// Echo back the prompt in addition to the completion
	Echo param.Opt[bool] `json:"echo,omitzero"`
	// Number between -2.0 and 2.0. Positive values penalize new tokens based on their
	// existing frequency in the text so far, decreasing the model's likelihood to
	// repeat the same line verbatim.
	//
	// [See more information about frequency and presence penalties.](https://platform.openai.com/docs/guides/text-generation)
	FrequencyPenalty param.Opt[float64] `json:"frequency_penalty,omitzero"`
	// Include the log probabilities on the `logprobs` most likely output tokens, as
	// well the chosen tokens. For example, if `logprobs` is 5, the API will return a
	// list of the 5 most likely tokens. The API will always return the `logprob` of
	// the sampled token, so there may be up to `logprobs+1` elements in the response.
	//
	// The maximum value for `logprobs` is 5.
	Logprobs param.Opt[int64] `json:"logprobs,omitzero"`
	// The maximum number of [tokens](/tokenizer) that can be generated in the
	// completion.
	//
	// The token count of your prompt plus `max_tokens` cannot exceed the model's
	// context length.
	// [Example Python code](https://cookbook.openai.com/examples/how_to_count_tokens_with_tiktoken)
	// for counting tokens.
	MaxTokens param.Opt[int64] `json:"max_tokens,omitzero"`
	// How many completions to generate for each prompt.
	//
	// **Note:** Because this parameter generates many completions, it can quickly
	// consume your token quota. Use carefully and ensure that you have reasonable
	// settings for `max_tokens` and `stop`.
	N param.Opt[int64] `json:"n,omitzero"`
	// Number between -2.0 and 2.0. Positive values penalize new tokens based on
	// whether they appear in the text so far, increasing the model's likelihood to
	// talk about new topics.
	//
	// [See more information about frequency and presence penalties.](https://platform.openai.com/docs/guides/text-generation)
	PresencePenalty param.Opt[float64] `json:"presence_penalty,omitzero"`
	// If specified, our system will make a best effort to sample deterministically,
	// such that repeated requests with the same `seed` and parameters should return
	// the same result.
	//
	// Determinism is not guaranteed, and you should refer to the `system_fingerprint`
	// response parameter to monitor changes in the backend.
	Seed param.Opt[int64] `json:"seed,omitzero"`
	// The suffix that comes after a completion of inserted text.
	//
	// This parameter is only supported for `gpt-3.5-turbo-instruct`.
	Suffix param.Opt[string] `json:"suffix,omitzero"`
	// What sampling temperature to use, between 0 and 2. Higher values like 0.8 will
	// make the output more random, while lower values like 0.2 will make it more
	// focused and deterministic.
	//
	// We generally recommend altering this or `top_p` but not both.
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
	// Modify the likelihood of specified tokens appearing in the completion.
	//
	// Accepts a JSON object that maps tokens (specified by their token ID in the GPT
	// tokenizer) to an associated bias value from -100 to 100. You can use this
	// [tokenizer tool](/tokenizer?view=bpe) to convert text to token IDs.
	// Mathematically, the bias is added to the logits generated by the model prior to
	// sampling. The exact effect will vary per model, but values between -1 and 1
	// should decrease or increase likelihood of selection; values like -100 or 100
	// should result in a ban or exclusive selection of the relevant token.
	//
	// As an example, you can pass `{"50256": -100}` to prevent the <|endoftext|> token
	// from being generated.
	LogitBias map[string]int64 `json:"logit_bias,omitzero"`
	// Not supported with latest reasoning models `o3` and `o4-mini`.
	//
	// Up to 4 sequences where the API will stop generating further tokens. The
	// returned text will not contain the stop sequence.
	Stop CompletionNewParamsStopUnion `json:"stop,omitzero"`
	// Options for streaming response. Only set this when you set `stream: true`.
	StreamOptions ChatCompletionStreamOptionsParam `json:"stream_options,omitzero"`
	paramObj
}

func (r CompletionNewParams) MarshalJSON() (data []byte, err error) {
	type shadow CompletionNewParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *CompletionNewParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ID of the model to use. You can use the
// [List models](https://platform.openai.com/docs/api-reference/models/list) API to
// see all of your available models, or see our
// [Model overview](https://platform.openai.com/docs/models) for descriptions of
// them.
type CompletionNewParamsModel string

const (
	CompletionNewParamsModelGPT3_5TurboInstruct CompletionNewParamsModel = "gpt-3.5-turbo-instruct"
	CompletionNewParamsModelDavinci002          CompletionNewParamsModel = "davinci-002"
	CompletionNewParamsModelBabbage002          CompletionNewParamsModel = "babbage-002"
)

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type CompletionNewParamsPromptUnion struct {
	OfString             param.Opt[string] `json:",omitzero,inline"`
	OfArrayOfStrings     []string          `json:",omitzero,inline"`
	OfArrayOfTokens      []int64           `json:",omitzero,inline"`
	OfArrayOfTokenArrays [][]int64         `json:",omitzero,inline"`
	paramUnion
}

func (u CompletionNewParamsPromptUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfArrayOfStrings, u.OfArrayOfTokens, u.OfArrayOfTokenArrays)
}
func (u *CompletionNewParamsPromptUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *CompletionNewParamsPromptUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfArrayOfStrings) {
		return &u.OfArrayOfStrings
	} else if !param.IsOmitted(u.OfArrayOfTokens) {
		return &u.OfArrayOfTokens
	} else if !param.IsOmitted(u.OfArrayOfTokenArrays) {
		return &u.OfArrayOfTokenArrays
	}
	return nil
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type CompletionNewParamsStopUnion struct {
	OfString      param.Opt[string] `json:",omitzero,inline"`
	OfStringArray []string          `json:",omitzero,inline"`
	paramUnion
}

func (u CompletionNewParamsStopUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfStringArray)
}
func (u *CompletionNewParamsStopUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *CompletionNewParamsStopUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfStringArray) {
		return &u.OfStringArray
	}
	return nil
}
