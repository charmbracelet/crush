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
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/shared"
	"github.com/openai/openai-go/shared/constant"
)

// ChatCompletionService contains methods and other services that help with
// interacting with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewChatCompletionService] method instead.
type ChatCompletionService struct {
	Options  []option.RequestOption
	Messages ChatCompletionMessageService
}

// NewChatCompletionService generates a new service that applies the given options
// to each request. These options are applied after the parent client's options (if
// there is one), and before any request-specific options.
func NewChatCompletionService(opts ...option.RequestOption) (r ChatCompletionService) {
	r = ChatCompletionService{}
	r.Options = opts
	r.Messages = NewChatCompletionMessageService(opts...)
	return
}

// **Starting a new project?** We recommend trying
// [Responses](https://platform.openai.com/docs/api-reference/responses) to take
// advantage of the latest OpenAI platform features. Compare
// [Chat Completions with Responses](https://platform.openai.com/docs/guides/responses-vs-chat-completions?api-mode=responses).
//
// ---
//
// Creates a model response for the given chat conversation. Learn more in the
// [text generation](https://platform.openai.com/docs/guides/text-generation),
// [vision](https://platform.openai.com/docs/guides/vision), and
// [audio](https://platform.openai.com/docs/guides/audio) guides.
//
// Parameter support can differ depending on the model used to generate the
// response, particularly for newer reasoning models. Parameters that are only
// supported for reasoning models are noted below. For the current state of
// unsupported parameters in reasoning models,
// [refer to the reasoning guide](https://platform.openai.com/docs/guides/reasoning).
func (r *ChatCompletionService) New(ctx context.Context, body ChatCompletionNewParams, opts ...option.RequestOption) (res *ChatCompletion, err error) {
	opts = append(r.Options[:], opts...)
	path := "chat/completions"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// **Starting a new project?** We recommend trying
// [Responses](https://platform.openai.com/docs/api-reference/responses) to take
// advantage of the latest OpenAI platform features. Compare
// [Chat Completions with Responses](https://platform.openai.com/docs/guides/responses-vs-chat-completions?api-mode=responses).
//
// ---
//
// Creates a model response for the given chat conversation. Learn more in the
// [text generation](https://platform.openai.com/docs/guides/text-generation),
// [vision](https://platform.openai.com/docs/guides/vision), and
// [audio](https://platform.openai.com/docs/guides/audio) guides.
//
// Parameter support can differ depending on the model used to generate the
// response, particularly for newer reasoning models. Parameters that are only
// supported for reasoning models are noted below. For the current state of
// unsupported parameters in reasoning models,
// [refer to the reasoning guide](https://platform.openai.com/docs/guides/reasoning).
func (r *ChatCompletionService) NewStreaming(ctx context.Context, body ChatCompletionNewParams, opts ...option.RequestOption) (stream *ssestream.Stream[ChatCompletionChunk]) {
	var (
		raw *http.Response
		err error
	)
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithJSONSet("stream", true)}, opts...)
	path := "chat/completions"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &raw, opts...)
	return ssestream.NewStream[ChatCompletionChunk](ssestream.NewDecoder(raw), err)
}

// Get a stored chat completion. Only Chat Completions that have been created with
// the `store` parameter set to `true` will be returned.
func (r *ChatCompletionService) Get(ctx context.Context, completionID string, opts ...option.RequestOption) (res *ChatCompletion, err error) {
	opts = append(r.Options[:], opts...)
	if completionID == "" {
		err = errors.New("missing required completion_id parameter")
		return
	}
	path := fmt.Sprintf("chat/completions/%s", completionID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// Modify a stored chat completion. Only Chat Completions that have been created
// with the `store` parameter set to `true` can be modified. Currently, the only
// supported modification is to update the `metadata` field.
func (r *ChatCompletionService) Update(ctx context.Context, completionID string, body ChatCompletionUpdateParams, opts ...option.RequestOption) (res *ChatCompletion, err error) {
	opts = append(r.Options[:], opts...)
	if completionID == "" {
		err = errors.New("missing required completion_id parameter")
		return
	}
	path := fmt.Sprintf("chat/completions/%s", completionID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// List stored Chat Completions. Only Chat Completions that have been stored with
// the `store` parameter set to `true` will be returned.
func (r *ChatCompletionService) List(ctx context.Context, query ChatCompletionListParams, opts ...option.RequestOption) (res *pagination.CursorPage[ChatCompletion], err error) {
	var raw *http.Response
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithResponseInto(&raw)}, opts...)
	path := "chat/completions"
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

// List stored Chat Completions. Only Chat Completions that have been stored with
// the `store` parameter set to `true` will be returned.
func (r *ChatCompletionService) ListAutoPaging(ctx context.Context, query ChatCompletionListParams, opts ...option.RequestOption) *pagination.CursorPageAutoPager[ChatCompletion] {
	return pagination.NewCursorPageAutoPager(r.List(ctx, query, opts...))
}

// Delete a stored chat completion. Only Chat Completions that have been created
// with the `store` parameter set to `true` can be deleted.
func (r *ChatCompletionService) Delete(ctx context.Context, completionID string, opts ...option.RequestOption) (res *ChatCompletionDeleted, err error) {
	opts = append(r.Options[:], opts...)
	if completionID == "" {
		err = errors.New("missing required completion_id parameter")
		return
	}
	path := fmt.Sprintf("chat/completions/%s", completionID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodDelete, path, nil, &res, opts...)
	return
}

// Represents a chat completion response returned by model, based on the provided
// input.
type ChatCompletion struct {
	// A unique identifier for the chat completion.
	ID string `json:"id,required"`
	// A list of chat completion choices. Can be more than one if `n` is greater
	// than 1.
	Choices []ChatCompletionChoice `json:"choices,required"`
	// The Unix timestamp (in seconds) of when the chat completion was created.
	Created int64 `json:"created,required"`
	// The model used for the chat completion.
	Model string `json:"model,required"`
	// The object type, which is always `chat.completion`.
	Object constant.ChatCompletion `json:"object,required"`
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
	ServiceTier ChatCompletionServiceTier `json:"service_tier,nullable"`
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
		ServiceTier       respjson.Field
		SystemFingerprint respjson.Field
		Usage             respjson.Field
		ExtraFields       map[string]respjson.Field
		raw               string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ChatCompletion) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ChatCompletionChoice struct {
	// The reason the model stopped generating tokens. This will be `stop` if the model
	// hit a natural stop point or a provided stop sequence, `length` if the maximum
	// number of tokens specified in the request was reached, `content_filter` if
	// content was omitted due to a flag from our content filters, `tool_calls` if the
	// model called a tool, or `function_call` (deprecated) if the model called a
	// function.
	//
	// Any of "stop", "length", "tool_calls", "content_filter", "function_call".
	FinishReason string `json:"finish_reason,required"`
	// The index of the choice in the list of choices.
	Index int64 `json:"index,required"`
	// Log probability information for the choice.
	Logprobs ChatCompletionChoiceLogprobs `json:"logprobs,required"`
	// A chat completion message generated by the model.
	Message ChatCompletionMessage `json:"message,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		FinishReason respjson.Field
		Index        respjson.Field
		Logprobs     respjson.Field
		Message      respjson.Field
		ExtraFields  map[string]respjson.Field
		raw          string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ChatCompletionChoice) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionChoice) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Log probability information for the choice.
type ChatCompletionChoiceLogprobs struct {
	// A list of message content tokens with log probability information.
	Content []ChatCompletionTokenLogprob `json:"content,required"`
	// A list of message refusal tokens with log probability information.
	Refusal []ChatCompletionTokenLogprob `json:"refusal,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Content     respjson.Field
		Refusal     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ChatCompletionChoiceLogprobs) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionChoiceLogprobs) UnmarshalJSON(data []byte) error {
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
type ChatCompletionServiceTier string

const (
	ChatCompletionServiceTierAuto     ChatCompletionServiceTier = "auto"
	ChatCompletionServiceTierDefault  ChatCompletionServiceTier = "default"
	ChatCompletionServiceTierFlex     ChatCompletionServiceTier = "flex"
	ChatCompletionServiceTierScale    ChatCompletionServiceTier = "scale"
	ChatCompletionServiceTierPriority ChatCompletionServiceTier = "priority"
)

// Messages sent by the model in response to user messages.
//
// The property Role is required.
type ChatCompletionAssistantMessageParam struct {
	// The refusal message by the assistant.
	Refusal param.Opt[string] `json:"refusal,omitzero"`
	// An optional name for the participant. Provides the model information to
	// differentiate between participants of the same role.
	Name param.Opt[string] `json:"name,omitzero"`
	// Data about a previous audio response from the model.
	// [Learn more](https://platform.openai.com/docs/guides/audio).
	Audio ChatCompletionAssistantMessageParamAudio `json:"audio,omitzero"`
	// The contents of the assistant message. Required unless `tool_calls` or
	// `function_call` is specified.
	Content ChatCompletionAssistantMessageParamContentUnion `json:"content,omitzero"`
	// Deprecated and replaced by `tool_calls`. The name and arguments of a function
	// that should be called, as generated by the model.
	//
	// Deprecated: deprecated
	FunctionCall ChatCompletionAssistantMessageParamFunctionCall `json:"function_call,omitzero"`
	// The tool calls generated by the model, such as function calls.
	ToolCalls []ChatCompletionMessageToolCallParam `json:"tool_calls,omitzero"`
	// The role of the messages author, in this case `assistant`.
	//
	// This field can be elided, and will marshal its zero value as "assistant".
	Role constant.Assistant `json:"role,required"`
	paramObj
}

func (r ChatCompletionAssistantMessageParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionAssistantMessageParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionAssistantMessageParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Data about a previous audio response from the model.
// [Learn more](https://platform.openai.com/docs/guides/audio).
//
// The property ID is required.
type ChatCompletionAssistantMessageParamAudio struct {
	// Unique identifier for a previous audio response from the model.
	ID string `json:"id,required"`
	paramObj
}

func (r ChatCompletionAssistantMessageParamAudio) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionAssistantMessageParamAudio
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionAssistantMessageParamAudio) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ChatCompletionAssistantMessageParamContentUnion struct {
	OfString              param.Opt[string]                                                   `json:",omitzero,inline"`
	OfArrayOfContentParts []ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion `json:",omitzero,inline"`
	paramUnion
}

func (u ChatCompletionAssistantMessageParamContentUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfArrayOfContentParts)
}
func (u *ChatCompletionAssistantMessageParamContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ChatCompletionAssistantMessageParamContentUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfArrayOfContentParts) {
		return &u.OfArrayOfContentParts
	}
	return nil
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion struct {
	OfText    *ChatCompletionContentPartTextParam    `json:",omitzero,inline"`
	OfRefusal *ChatCompletionContentPartRefusalParam `json:",omitzero,inline"`
	paramUnion
}

func (u ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfText, u.OfRefusal)
}
func (u *ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion) asAny() any {
	if !param.IsOmitted(u.OfText) {
		return u.OfText
	} else if !param.IsOmitted(u.OfRefusal) {
		return u.OfRefusal
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion) GetText() *string {
	if vt := u.OfText; vt != nil {
		return &vt.Text
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion) GetRefusal() *string {
	if vt := u.OfRefusal; vt != nil {
		return &vt.Refusal
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion) GetType() *string {
	if vt := u.OfText; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfRefusal; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion](
		"type",
		apijson.Discriminator[ChatCompletionContentPartTextParam]("text"),
		apijson.Discriminator[ChatCompletionContentPartRefusalParam]("refusal"),
	)
}

// Deprecated and replaced by `tool_calls`. The name and arguments of a function
// that should be called, as generated by the model.
//
// Deprecated: deprecated
//
// The properties Arguments, Name are required.
type ChatCompletionAssistantMessageParamFunctionCall struct {
	// The arguments to call the function with, as generated by the model in JSON
	// format. Note that the model does not always generate valid JSON, and may
	// hallucinate parameters not defined by your function schema. Validate the
	// arguments in your code before calling your function.
	Arguments string `json:"arguments,required"`
	// The name of the function to call.
	Name string `json:"name,required"`
	paramObj
}

func (r ChatCompletionAssistantMessageParamFunctionCall) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionAssistantMessageParamFunctionCall
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionAssistantMessageParamFunctionCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// If the audio output modality is requested, this object contains data about the
// audio response from the model.
// [Learn more](https://platform.openai.com/docs/guides/audio).
type ChatCompletionAudio struct {
	// Unique identifier for this audio response.
	ID string `json:"id,required"`
	// Base64 encoded audio bytes generated by the model, in the format specified in
	// the request.
	Data string `json:"data,required"`
	// The Unix timestamp (in seconds) for when this audio response will no longer be
	// accessible on the server for use in multi-turn conversations.
	ExpiresAt int64 `json:"expires_at,required"`
	// Transcript of the audio generated by the model.
	Transcript string `json:"transcript,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Data        respjson.Field
		ExpiresAt   respjson.Field
		Transcript  respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ChatCompletionAudio) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionAudio) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Parameters for audio output. Required when audio output is requested with
// `modalities: ["audio"]`.
// [Learn more](https://platform.openai.com/docs/guides/audio).
//
// The properties Format, Voice are required.
type ChatCompletionAudioParam struct {
	// Specifies the output audio format. Must be one of `wav`, `mp3`, `flac`, `opus`,
	// or `pcm16`.
	//
	// Any of "wav", "aac", "mp3", "flac", "opus", "pcm16".
	Format ChatCompletionAudioParamFormat `json:"format,omitzero,required"`
	// The voice the model uses to respond. Supported voices are `alloy`, `ash`,
	// `ballad`, `coral`, `echo`, `fable`, `nova`, `onyx`, `sage`, and `shimmer`.
	Voice ChatCompletionAudioParamVoice `json:"voice,omitzero,required"`
	paramObj
}

func (r ChatCompletionAudioParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionAudioParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionAudioParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Specifies the output audio format. Must be one of `wav`, `mp3`, `flac`, `opus`,
// or `pcm16`.
type ChatCompletionAudioParamFormat string

const (
	ChatCompletionAudioParamFormatWAV   ChatCompletionAudioParamFormat = "wav"
	ChatCompletionAudioParamFormatAAC   ChatCompletionAudioParamFormat = "aac"
	ChatCompletionAudioParamFormatMP3   ChatCompletionAudioParamFormat = "mp3"
	ChatCompletionAudioParamFormatFLAC  ChatCompletionAudioParamFormat = "flac"
	ChatCompletionAudioParamFormatOpus  ChatCompletionAudioParamFormat = "opus"
	ChatCompletionAudioParamFormatPcm16 ChatCompletionAudioParamFormat = "pcm16"
)

// The voice the model uses to respond. Supported voices are `alloy`, `ash`,
// `ballad`, `coral`, `echo`, `fable`, `nova`, `onyx`, `sage`, and `shimmer`.
type ChatCompletionAudioParamVoice string

const (
	ChatCompletionAudioParamVoiceAlloy   ChatCompletionAudioParamVoice = "alloy"
	ChatCompletionAudioParamVoiceAsh     ChatCompletionAudioParamVoice = "ash"
	ChatCompletionAudioParamVoiceBallad  ChatCompletionAudioParamVoice = "ballad"
	ChatCompletionAudioParamVoiceCoral   ChatCompletionAudioParamVoice = "coral"
	ChatCompletionAudioParamVoiceEcho    ChatCompletionAudioParamVoice = "echo"
	ChatCompletionAudioParamVoiceFable   ChatCompletionAudioParamVoice = "fable"
	ChatCompletionAudioParamVoiceOnyx    ChatCompletionAudioParamVoice = "onyx"
	ChatCompletionAudioParamVoiceNova    ChatCompletionAudioParamVoice = "nova"
	ChatCompletionAudioParamVoiceSage    ChatCompletionAudioParamVoice = "sage"
	ChatCompletionAudioParamVoiceShimmer ChatCompletionAudioParamVoice = "shimmer"
	ChatCompletionAudioParamVoiceVerse   ChatCompletionAudioParamVoice = "verse"
)

// Represents a streamed chunk of a chat completion response returned by the model,
// based on the provided input.
// [Learn more](https://platform.openai.com/docs/guides/streaming-responses).
type ChatCompletionChunk struct {
	// A unique identifier for the chat completion. Each chunk has the same ID.
	ID string `json:"id,required"`
	// A list of chat completion choices. Can contain more than one elements if `n` is
	// greater than 1. Can also be empty for the last chunk if you set
	// `stream_options: {"include_usage": true}`.
	Choices []ChatCompletionChunkChoice `json:"choices,required"`
	// The Unix timestamp (in seconds) of when the chat completion was created. Each
	// chunk has the same timestamp.
	Created int64 `json:"created,required"`
	// The model to generate the completion.
	Model string `json:"model,required"`
	// The object type, which is always `chat.completion.chunk`.
	Object constant.ChatCompletionChunk `json:"object,required"`
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
	ServiceTier ChatCompletionChunkServiceTier `json:"service_tier,nullable"`
	// This fingerprint represents the backend configuration that the model runs with.
	// Can be used in conjunction with the `seed` request parameter to understand when
	// backend changes have been made that might impact determinism.
	SystemFingerprint string `json:"system_fingerprint"`
	// An optional field that will only be present when you set
	// `stream_options: {"include_usage": true}` in your request. When present, it
	// contains a null value **except for the last chunk** which contains the token
	// usage statistics for the entire request.
	//
	// **NOTE:** If the stream is interrupted or cancelled, you may not receive the
	// final usage chunk which contains the total token usage for the request.
	Usage CompletionUsage `json:"usage,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID                respjson.Field
		Choices           respjson.Field
		Created           respjson.Field
		Model             respjson.Field
		Object            respjson.Field
		ServiceTier       respjson.Field
		SystemFingerprint respjson.Field
		Usage             respjson.Field
		ExtraFields       map[string]respjson.Field
		raw               string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ChatCompletionChunk) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionChunk) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ChatCompletionChunkChoice struct {
	// A chat completion delta generated by streamed model responses.
	Delta ChatCompletionChunkChoiceDelta `json:"delta,required"`
	// The reason the model stopped generating tokens. This will be `stop` if the model
	// hit a natural stop point or a provided stop sequence, `length` if the maximum
	// number of tokens specified in the request was reached, `content_filter` if
	// content was omitted due to a flag from our content filters, `tool_calls` if the
	// model called a tool, or `function_call` (deprecated) if the model called a
	// function.
	//
	// Any of "stop", "length", "tool_calls", "content_filter", "function_call".
	FinishReason string `json:"finish_reason,required"`
	// The index of the choice in the list of choices.
	Index int64 `json:"index,required"`
	// Log probability information for the choice.
	Logprobs ChatCompletionChunkChoiceLogprobs `json:"logprobs,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Delta        respjson.Field
		FinishReason respjson.Field
		Index        respjson.Field
		Logprobs     respjson.Field
		ExtraFields  map[string]respjson.Field
		raw          string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ChatCompletionChunkChoice) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionChunkChoice) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A chat completion delta generated by streamed model responses.
type ChatCompletionChunkChoiceDelta struct {
	// The contents of the chunk message.
	Content string `json:"content,nullable"`
	// Deprecated and replaced by `tool_calls`. The name and arguments of a function
	// that should be called, as generated by the model.
	//
	// Deprecated: deprecated
	FunctionCall ChatCompletionChunkChoiceDeltaFunctionCall `json:"function_call"`
	// The refusal message generated by the model.
	Refusal string `json:"refusal,nullable"`
	// The role of the author of this message.
	//
	// Any of "developer", "system", "user", "assistant", "tool".
	Role      string                                   `json:"role"`
	ToolCalls []ChatCompletionChunkChoiceDeltaToolCall `json:"tool_calls"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Content      respjson.Field
		FunctionCall respjson.Field
		Refusal      respjson.Field
		Role         respjson.Field
		ToolCalls    respjson.Field
		ExtraFields  map[string]respjson.Field
		raw          string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ChatCompletionChunkChoiceDelta) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionChunkChoiceDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Deprecated and replaced by `tool_calls`. The name and arguments of a function
// that should be called, as generated by the model.
//
// Deprecated: deprecated
type ChatCompletionChunkChoiceDeltaFunctionCall struct {
	// The arguments to call the function with, as generated by the model in JSON
	// format. Note that the model does not always generate valid JSON, and may
	// hallucinate parameters not defined by your function schema. Validate the
	// arguments in your code before calling your function.
	Arguments string `json:"arguments"`
	// The name of the function to call.
	Name string `json:"name"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Arguments   respjson.Field
		Name        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ChatCompletionChunkChoiceDeltaFunctionCall) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionChunkChoiceDeltaFunctionCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ChatCompletionChunkChoiceDeltaToolCall struct {
	Index int64 `json:"index,required"`
	// The ID of the tool call.
	ID       string                                         `json:"id"`
	Function ChatCompletionChunkChoiceDeltaToolCallFunction `json:"function"`
	// The type of the tool. Currently, only `function` is supported.
	//
	// Any of "function".
	Type string `json:"type"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Index       respjson.Field
		ID          respjson.Field
		Function    respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ChatCompletionChunkChoiceDeltaToolCall) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionChunkChoiceDeltaToolCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ChatCompletionChunkChoiceDeltaToolCallFunction struct {
	// The arguments to call the function with, as generated by the model in JSON
	// format. Note that the model does not always generate valid JSON, and may
	// hallucinate parameters not defined by your function schema. Validate the
	// arguments in your code before calling your function.
	Arguments string `json:"arguments"`
	// The name of the function to call.
	Name string `json:"name"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Arguments   respjson.Field
		Name        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ChatCompletionChunkChoiceDeltaToolCallFunction) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionChunkChoiceDeltaToolCallFunction) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Log probability information for the choice.
type ChatCompletionChunkChoiceLogprobs struct {
	// A list of message content tokens with log probability information.
	Content []ChatCompletionTokenLogprob `json:"content,required"`
	// A list of message refusal tokens with log probability information.
	Refusal []ChatCompletionTokenLogprob `json:"refusal,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Content     respjson.Field
		Refusal     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ChatCompletionChunkChoiceLogprobs) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionChunkChoiceLogprobs) UnmarshalJSON(data []byte) error {
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
type ChatCompletionChunkServiceTier string

const (
	ChatCompletionChunkServiceTierAuto     ChatCompletionChunkServiceTier = "auto"
	ChatCompletionChunkServiceTierDefault  ChatCompletionChunkServiceTier = "default"
	ChatCompletionChunkServiceTierFlex     ChatCompletionChunkServiceTier = "flex"
	ChatCompletionChunkServiceTierScale    ChatCompletionChunkServiceTier = "scale"
	ChatCompletionChunkServiceTierPriority ChatCompletionChunkServiceTier = "priority"
)

func TextContentPart(text string) ChatCompletionContentPartUnionParam {
	var variant ChatCompletionContentPartTextParam
	variant.Text = text
	return ChatCompletionContentPartUnionParam{OfText: &variant}
}

func ImageContentPart(imageURL ChatCompletionContentPartImageImageURLParam) ChatCompletionContentPartUnionParam {
	var variant ChatCompletionContentPartImageParam
	variant.ImageURL = imageURL
	return ChatCompletionContentPartUnionParam{OfImageURL: &variant}
}

func InputAudioContentPart(inputAudio ChatCompletionContentPartInputAudioInputAudioParam) ChatCompletionContentPartUnionParam {
	var variant ChatCompletionContentPartInputAudioParam
	variant.InputAudio = inputAudio
	return ChatCompletionContentPartUnionParam{OfInputAudio: &variant}
}

func FileContentPart(file ChatCompletionContentPartFileFileParam) ChatCompletionContentPartUnionParam {
	var variant ChatCompletionContentPartFileParam
	variant.File = file
	return ChatCompletionContentPartUnionParam{OfFile: &variant}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ChatCompletionContentPartUnionParam struct {
	OfText       *ChatCompletionContentPartTextParam       `json:",omitzero,inline"`
	OfImageURL   *ChatCompletionContentPartImageParam      `json:",omitzero,inline"`
	OfInputAudio *ChatCompletionContentPartInputAudioParam `json:",omitzero,inline"`
	OfFile       *ChatCompletionContentPartFileParam       `json:",omitzero,inline"`
	paramUnion
}

func (u ChatCompletionContentPartUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfText, u.OfImageURL, u.OfInputAudio, u.OfFile)
}
func (u *ChatCompletionContentPartUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ChatCompletionContentPartUnionParam) asAny() any {
	if !param.IsOmitted(u.OfText) {
		return u.OfText
	} else if !param.IsOmitted(u.OfImageURL) {
		return u.OfImageURL
	} else if !param.IsOmitted(u.OfInputAudio) {
		return u.OfInputAudio
	} else if !param.IsOmitted(u.OfFile) {
		return u.OfFile
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ChatCompletionContentPartUnionParam) GetText() *string {
	if vt := u.OfText; vt != nil {
		return &vt.Text
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ChatCompletionContentPartUnionParam) GetImageURL() *ChatCompletionContentPartImageImageURLParam {
	if vt := u.OfImageURL; vt != nil {
		return &vt.ImageURL
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ChatCompletionContentPartUnionParam) GetInputAudio() *ChatCompletionContentPartInputAudioInputAudioParam {
	if vt := u.OfInputAudio; vt != nil {
		return &vt.InputAudio
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ChatCompletionContentPartUnionParam) GetFile() *ChatCompletionContentPartFileFileParam {
	if vt := u.OfFile; vt != nil {
		return &vt.File
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ChatCompletionContentPartUnionParam) GetType() *string {
	if vt := u.OfText; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfImageURL; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfInputAudio; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfFile; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[ChatCompletionContentPartUnionParam](
		"type",
		apijson.Discriminator[ChatCompletionContentPartTextParam]("text"),
		apijson.Discriminator[ChatCompletionContentPartImageParam]("image_url"),
		apijson.Discriminator[ChatCompletionContentPartInputAudioParam]("input_audio"),
		apijson.Discriminator[ChatCompletionContentPartFileParam]("file"),
	)
}

// Learn about [file inputs](https://platform.openai.com/docs/guides/text) for text
// generation.
//
// The properties File, Type are required.
type ChatCompletionContentPartFileParam struct {
	File ChatCompletionContentPartFileFileParam `json:"file,omitzero,required"`
	// The type of the content part. Always `file`.
	//
	// This field can be elided, and will marshal its zero value as "file".
	Type constant.File `json:"type,required"`
	paramObj
}

func (r ChatCompletionContentPartFileParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionContentPartFileParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionContentPartFileParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ChatCompletionContentPartFileFileParam struct {
	// The base64 encoded file data, used when passing the file to the model as a
	// string.
	FileData param.Opt[string] `json:"file_data,omitzero"`
	// The ID of an uploaded file to use as input.
	FileID param.Opt[string] `json:"file_id,omitzero"`
	// The name of the file, used when passing the file to the model as a string.
	Filename param.Opt[string] `json:"filename,omitzero"`
	paramObj
}

func (r ChatCompletionContentPartFileFileParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionContentPartFileFileParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionContentPartFileFileParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Learn about [image inputs](https://platform.openai.com/docs/guides/vision).
//
// The properties ImageURL, Type are required.
type ChatCompletionContentPartImageParam struct {
	ImageURL ChatCompletionContentPartImageImageURLParam `json:"image_url,omitzero,required"`
	// The type of the content part.
	//
	// This field can be elided, and will marshal its zero value as "image_url".
	Type constant.ImageURL `json:"type,required"`
	paramObj
}

func (r ChatCompletionContentPartImageParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionContentPartImageParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionContentPartImageParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The property URL is required.
type ChatCompletionContentPartImageImageURLParam struct {
	// Either a URL of the image or the base64 encoded image data.
	URL string `json:"url,required" format:"uri"`
	// Specifies the detail level of the image. Learn more in the
	// [Vision guide](https://platform.openai.com/docs/guides/vision#low-or-high-fidelity-image-understanding).
	//
	// Any of "auto", "low", "high".
	Detail string `json:"detail,omitzero"`
	paramObj
}

func (r ChatCompletionContentPartImageImageURLParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionContentPartImageImageURLParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionContentPartImageImageURLParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[ChatCompletionContentPartImageImageURLParam](
		"detail", "auto", "low", "high",
	)
}

// Learn about [audio inputs](https://platform.openai.com/docs/guides/audio).
//
// The properties InputAudio, Type are required.
type ChatCompletionContentPartInputAudioParam struct {
	InputAudio ChatCompletionContentPartInputAudioInputAudioParam `json:"input_audio,omitzero,required"`
	// The type of the content part. Always `input_audio`.
	//
	// This field can be elided, and will marshal its zero value as "input_audio".
	Type constant.InputAudio `json:"type,required"`
	paramObj
}

func (r ChatCompletionContentPartInputAudioParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionContentPartInputAudioParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionContentPartInputAudioParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Data, Format are required.
type ChatCompletionContentPartInputAudioInputAudioParam struct {
	// Base64 encoded audio data.
	Data string `json:"data,required"`
	// The format of the encoded audio data. Currently supports "wav" and "mp3".
	//
	// Any of "wav", "mp3".
	Format string `json:"format,omitzero,required"`
	paramObj
}

func (r ChatCompletionContentPartInputAudioInputAudioParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionContentPartInputAudioInputAudioParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionContentPartInputAudioInputAudioParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[ChatCompletionContentPartInputAudioInputAudioParam](
		"format", "wav", "mp3",
	)
}

// The properties Refusal, Type are required.
type ChatCompletionContentPartRefusalParam struct {
	// The refusal message generated by the model.
	Refusal string `json:"refusal,required"`
	// The type of the content part.
	//
	// This field can be elided, and will marshal its zero value as "refusal".
	Type constant.Refusal `json:"type,required"`
	paramObj
}

func (r ChatCompletionContentPartRefusalParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionContentPartRefusalParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionContentPartRefusalParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Learn about
// [text inputs](https://platform.openai.com/docs/guides/text-generation).
//
// The properties Text, Type are required.
type ChatCompletionContentPartTextParam struct {
	// The text content.
	Text string `json:"text,required"`
	// The type of the content part.
	//
	// This field can be elided, and will marshal its zero value as "text".
	Type constant.Text `json:"type,required"`
	paramObj
}

func (r ChatCompletionContentPartTextParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionContentPartTextParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionContentPartTextParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ChatCompletionDeleted struct {
	// The ID of the chat completion that was deleted.
	ID string `json:"id,required"`
	// Whether the chat completion was deleted.
	Deleted bool `json:"deleted,required"`
	// The type of object being deleted.
	Object constant.ChatCompletionDeleted `json:"object,required"`
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
func (r ChatCompletionDeleted) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionDeleted) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Developer-provided instructions that the model should follow, regardless of
// messages sent by the user. With o1 models and newer, `developer` messages
// replace the previous `system` messages.
//
// The properties Content, Role are required.
type ChatCompletionDeveloperMessageParam struct {
	// The contents of the developer message.
	Content ChatCompletionDeveloperMessageParamContentUnion `json:"content,omitzero,required"`
	// An optional name for the participant. Provides the model information to
	// differentiate between participants of the same role.
	Name param.Opt[string] `json:"name,omitzero"`
	// The role of the messages author, in this case `developer`.
	//
	// This field can be elided, and will marshal its zero value as "developer".
	Role constant.Developer `json:"role,required"`
	paramObj
}

func (r ChatCompletionDeveloperMessageParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionDeveloperMessageParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionDeveloperMessageParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ChatCompletionDeveloperMessageParamContentUnion struct {
	OfString              param.Opt[string]                    `json:",omitzero,inline"`
	OfArrayOfContentParts []ChatCompletionContentPartTextParam `json:",omitzero,inline"`
	paramUnion
}

func (u ChatCompletionDeveloperMessageParamContentUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfArrayOfContentParts)
}
func (u *ChatCompletionDeveloperMessageParamContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ChatCompletionDeveloperMessageParamContentUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfArrayOfContentParts) {
		return &u.OfArrayOfContentParts
	}
	return nil
}

// Specifying a particular function via `{"name": "my_function"}` forces the model
// to call that function.
//
// The property Name is required.
type ChatCompletionFunctionCallOptionParam struct {
	// The name of the function to call.
	Name string `json:"name,required"`
	paramObj
}

func (r ChatCompletionFunctionCallOptionParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionFunctionCallOptionParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionFunctionCallOptionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Deprecated: deprecated
//
// The properties Content, Name, Role are required.
type ChatCompletionFunctionMessageParam struct {
	// The contents of the function message.
	Content param.Opt[string] `json:"content,omitzero,required"`
	// The name of the function to call.
	Name string `json:"name,required"`
	// The role of the messages author, in this case `function`.
	//
	// This field can be elided, and will marshal its zero value as "function".
	Role constant.Function `json:"role,required"`
	paramObj
}

func (r ChatCompletionFunctionMessageParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionFunctionMessageParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionFunctionMessageParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A chat completion message generated by the model.
type ChatCompletionMessage struct {
	// The contents of the message.
	Content string `json:"content,required"`
	// The refusal message generated by the model.
	Refusal string `json:"refusal,required"`
	// The role of the author of this message.
	Role constant.Assistant `json:"role,required"`
	// Annotations for the message, when applicable, as when using the
	// [web search tool](https://platform.openai.com/docs/guides/tools-web-search?api-mode=chat).
	Annotations []ChatCompletionMessageAnnotation `json:"annotations"`
	// If the audio output modality is requested, this object contains data about the
	// audio response from the model.
	// [Learn more](https://platform.openai.com/docs/guides/audio).
	Audio ChatCompletionAudio `json:"audio,nullable"`
	// Deprecated and replaced by `tool_calls`. The name and arguments of a function
	// that should be called, as generated by the model.
	//
	// Deprecated: deprecated
	FunctionCall ChatCompletionMessageFunctionCall `json:"function_call"`
	// The tool calls generated by the model, such as function calls.
	ToolCalls []ChatCompletionMessageToolCall `json:"tool_calls"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Content      respjson.Field
		Refusal      respjson.Field
		Role         respjson.Field
		Annotations  respjson.Field
		Audio        respjson.Field
		FunctionCall respjson.Field
		ToolCalls    respjson.Field
		ExtraFields  map[string]respjson.Field
		raw          string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ChatCompletionMessage) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionMessage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func (r ChatCompletionMessage) ToParam() ChatCompletionMessageParamUnion {
	asst := r.ToAssistantMessageParam()
	return ChatCompletionMessageParamUnion{OfAssistant: &asst}
}

func (r ChatCompletionMessage) ToAssistantMessageParam() ChatCompletionAssistantMessageParam {
	var p ChatCompletionAssistantMessageParam

	// It is important to not rely on the JSON metadata property
	// here, it may be unset if the receiver was generated via a
	// [ChatCompletionAccumulator].
	//
	// Explicit null is intentionally elided from the response.
	if r.Content != "" {
		p.Content.OfString = String(r.Content)
	}
	if r.Refusal != "" {
		p.Refusal = String(r.Refusal)
	}

	p.Audio.ID = r.Audio.ID
	p.Role = r.Role
	p.FunctionCall.Arguments = r.FunctionCall.Arguments
	p.FunctionCall.Name = r.FunctionCall.Name

	if len(r.ToolCalls) > 0 {
		p.ToolCalls = make([]ChatCompletionMessageToolCallParam, len(r.ToolCalls))
		for i, v := range r.ToolCalls {
			p.ToolCalls[i].ID = v.ID
			p.ToolCalls[i].Function.Arguments = v.Function.Arguments
			p.ToolCalls[i].Function.Name = v.Function.Name
		}
	}
	return p
}

// A URL citation when using web search.
type ChatCompletionMessageAnnotation struct {
	// The type of the URL citation. Always `url_citation`.
	Type constant.URLCitation `json:"type,required"`
	// A URL citation when using web search.
	URLCitation ChatCompletionMessageAnnotationURLCitation `json:"url_citation,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		URLCitation respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ChatCompletionMessageAnnotation) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionMessageAnnotation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A URL citation when using web search.
type ChatCompletionMessageAnnotationURLCitation struct {
	// The index of the last character of the URL citation in the message.
	EndIndex int64 `json:"end_index,required"`
	// The index of the first character of the URL citation in the message.
	StartIndex int64 `json:"start_index,required"`
	// The title of the web resource.
	Title string `json:"title,required"`
	// The URL of the web resource.
	URL string `json:"url,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		EndIndex    respjson.Field
		StartIndex  respjson.Field
		Title       respjson.Field
		URL         respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ChatCompletionMessageAnnotationURLCitation) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionMessageAnnotationURLCitation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Deprecated and replaced by `tool_calls`. The name and arguments of a function
// that should be called, as generated by the model.
//
// Deprecated: deprecated
type ChatCompletionMessageFunctionCall struct {
	// The arguments to call the function with, as generated by the model in JSON
	// format. Note that the model does not always generate valid JSON, and may
	// hallucinate parameters not defined by your function schema. Validate the
	// arguments in your code before calling your function.
	Arguments string `json:"arguments,required"`
	// The name of the function to call.
	Name string `json:"name,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Arguments   respjson.Field
		Name        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ChatCompletionMessageFunctionCall) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionMessageFunctionCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func AssistantMessage[T string | []ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion](content T) ChatCompletionMessageParamUnion {
	var assistant ChatCompletionAssistantMessageParam
	switch v := any(content).(type) {
	case string:
		assistant.Content.OfString = param.NewOpt(v)
	case []ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion:
		assistant.Content.OfArrayOfContentParts = v
	}
	return ChatCompletionMessageParamUnion{OfAssistant: &assistant}
}

func DeveloperMessage[T string | []ChatCompletionContentPartTextParam](content T) ChatCompletionMessageParamUnion {
	var developer ChatCompletionDeveloperMessageParam
	switch v := any(content).(type) {
	case string:
		developer.Content.OfString = param.NewOpt(v)
	case []ChatCompletionContentPartTextParam:
		developer.Content.OfArrayOfContentParts = v
	}
	return ChatCompletionMessageParamUnion{OfDeveloper: &developer}
}

func SystemMessage[T string | []ChatCompletionContentPartTextParam](content T) ChatCompletionMessageParamUnion {
	var system ChatCompletionSystemMessageParam
	switch v := any(content).(type) {
	case string:
		system.Content.OfString = param.NewOpt(v)
	case []ChatCompletionContentPartTextParam:
		system.Content.OfArrayOfContentParts = v
	}
	return ChatCompletionMessageParamUnion{OfSystem: &system}
}

func UserMessage[T string | []ChatCompletionContentPartUnionParam](content T) ChatCompletionMessageParamUnion {
	var user ChatCompletionUserMessageParam
	switch v := any(content).(type) {
	case string:
		user.Content.OfString = param.NewOpt(v)
	case []ChatCompletionContentPartUnionParam:
		user.Content.OfArrayOfContentParts = v
	}
	return ChatCompletionMessageParamUnion{OfUser: &user}
}

func ToolMessage[T string | []ChatCompletionContentPartTextParam](content T, toolCallID string) ChatCompletionMessageParamUnion {
	var tool ChatCompletionToolMessageParam
	switch v := any(content).(type) {
	case string:
		tool.Content.OfString = param.NewOpt(v)
	case []ChatCompletionContentPartTextParam:
		tool.Content.OfArrayOfContentParts = v
	}
	tool.ToolCallID = toolCallID
	return ChatCompletionMessageParamUnion{OfTool: &tool}
}

func ChatCompletionMessageParamOfFunction(content string, name string) ChatCompletionMessageParamUnion {
	var function ChatCompletionFunctionMessageParam
	function.Content = param.NewOpt(content)
	function.Name = name
	return ChatCompletionMessageParamUnion{OfFunction: &function}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ChatCompletionMessageParamUnion struct {
	OfDeveloper *ChatCompletionDeveloperMessageParam `json:",omitzero,inline"`
	OfSystem    *ChatCompletionSystemMessageParam    `json:",omitzero,inline"`
	OfUser      *ChatCompletionUserMessageParam      `json:",omitzero,inline"`
	OfAssistant *ChatCompletionAssistantMessageParam `json:",omitzero,inline"`
	OfTool      *ChatCompletionToolMessageParam      `json:",omitzero,inline"`
	OfFunction  *ChatCompletionFunctionMessageParam  `json:",omitzero,inline"`
	paramUnion
}

func (u ChatCompletionMessageParamUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfDeveloper,
		u.OfSystem,
		u.OfUser,
		u.OfAssistant,
		u.OfTool,
		u.OfFunction)
}
func (u *ChatCompletionMessageParamUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ChatCompletionMessageParamUnion) asAny() any {
	if !param.IsOmitted(u.OfDeveloper) {
		return u.OfDeveloper
	} else if !param.IsOmitted(u.OfSystem) {
		return u.OfSystem
	} else if !param.IsOmitted(u.OfUser) {
		return u.OfUser
	} else if !param.IsOmitted(u.OfAssistant) {
		return u.OfAssistant
	} else if !param.IsOmitted(u.OfTool) {
		return u.OfTool
	} else if !param.IsOmitted(u.OfFunction) {
		return u.OfFunction
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ChatCompletionMessageParamUnion) GetAudio() *ChatCompletionAssistantMessageParamAudio {
	if vt := u.OfAssistant; vt != nil {
		return &vt.Audio
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ChatCompletionMessageParamUnion) GetFunctionCall() *ChatCompletionAssistantMessageParamFunctionCall {
	if vt := u.OfAssistant; vt != nil {
		return &vt.FunctionCall
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ChatCompletionMessageParamUnion) GetRefusal() *string {
	if vt := u.OfAssistant; vt != nil && vt.Refusal.Valid() {
		return &vt.Refusal.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ChatCompletionMessageParamUnion) GetToolCalls() []ChatCompletionMessageToolCallParam {
	if vt := u.OfAssistant; vt != nil {
		return vt.ToolCalls
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ChatCompletionMessageParamUnion) GetToolCallID() *string {
	if vt := u.OfTool; vt != nil {
		return &vt.ToolCallID
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ChatCompletionMessageParamUnion) GetRole() *string {
	if vt := u.OfDeveloper; vt != nil {
		return (*string)(&vt.Role)
	} else if vt := u.OfSystem; vt != nil {
		return (*string)(&vt.Role)
	} else if vt := u.OfUser; vt != nil {
		return (*string)(&vt.Role)
	} else if vt := u.OfAssistant; vt != nil {
		return (*string)(&vt.Role)
	} else if vt := u.OfTool; vt != nil {
		return (*string)(&vt.Role)
	} else if vt := u.OfFunction; vt != nil {
		return (*string)(&vt.Role)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ChatCompletionMessageParamUnion) GetName() *string {
	if vt := u.OfDeveloper; vt != nil && vt.Name.Valid() {
		return &vt.Name.Value
	} else if vt := u.OfSystem; vt != nil && vt.Name.Valid() {
		return &vt.Name.Value
	} else if vt := u.OfUser; vt != nil && vt.Name.Valid() {
		return &vt.Name.Value
	} else if vt := u.OfAssistant; vt != nil && vt.Name.Valid() {
		return &vt.Name.Value
	} else if vt := u.OfFunction; vt != nil {
		return (*string)(&vt.Name)
	}
	return nil
}

// Returns a subunion which exports methods to access subproperties
//
// Or use AsAny() to get the underlying value
func (u ChatCompletionMessageParamUnion) GetContent() (res chatCompletionMessageParamUnionContent) {
	if vt := u.OfDeveloper; vt != nil {
		res.any = vt.Content.asAny()
	} else if vt := u.OfSystem; vt != nil {
		res.any = vt.Content.asAny()
	} else if vt := u.OfUser; vt != nil {
		res.any = vt.Content.asAny()
	} else if vt := u.OfAssistant; vt != nil {
		res.any = vt.Content.asAny()
	} else if vt := u.OfTool; vt != nil {
		res.any = vt.Content.asAny()
	} else if vt := u.OfFunction; vt != nil && vt.Content.Valid() {
		res.any = &vt.Content.Value
	}
	return
}

// Can have the runtime types [*string], [_[]ChatCompletionContentPartTextParam],
// [_[]ChatCompletionContentPartUnionParam],
// [\*[]ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion]
type chatCompletionMessageParamUnionContent struct{ any }

// Use the following switch statement to get the type of the union:
//
//	switch u.AsAny().(type) {
//	case *string:
//	case *[]openai.ChatCompletionContentPartTextParam:
//	case *[]openai.ChatCompletionContentPartUnionParam:
//	case *[]openai.ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion:
//	default:
//	    fmt.Errorf("not present")
//	}
func (u chatCompletionMessageParamUnionContent) AsAny() any { return u.any }

func init() {
	apijson.RegisterUnion[ChatCompletionMessageParamUnion](
		"role",
		apijson.Discriminator[ChatCompletionDeveloperMessageParam]("developer"),
		apijson.Discriminator[ChatCompletionSystemMessageParam]("system"),
		apijson.Discriminator[ChatCompletionUserMessageParam]("user"),
		apijson.Discriminator[ChatCompletionAssistantMessageParam]("assistant"),
		apijson.Discriminator[ChatCompletionToolMessageParam]("tool"),
		apijson.Discriminator[ChatCompletionFunctionMessageParam]("function"),
	)
}

type ChatCompletionMessageToolCall struct {
	// The ID of the tool call.
	ID string `json:"id,required"`
	// The function that the model called.
	Function ChatCompletionMessageToolCallFunction `json:"function,required"`
	// The type of the tool. Currently, only `function` is supported.
	Type constant.Function `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Function    respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ChatCompletionMessageToolCall) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionMessageToolCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ChatCompletionMessageToolCall to a
// ChatCompletionMessageToolCallParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ChatCompletionMessageToolCallParam.Overrides()
func (r ChatCompletionMessageToolCall) ToParam() ChatCompletionMessageToolCallParam {
	return param.Override[ChatCompletionMessageToolCallParam](json.RawMessage(r.RawJSON()))
}

// The function that the model called.
type ChatCompletionMessageToolCallFunction struct {
	// The arguments to call the function with, as generated by the model in JSON
	// format. Note that the model does not always generate valid JSON, and may
	// hallucinate parameters not defined by your function schema. Validate the
	// arguments in your code before calling your function.
	Arguments string `json:"arguments,required"`
	// The name of the function to call.
	Name string `json:"name,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Arguments   respjson.Field
		Name        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ChatCompletionMessageToolCallFunction) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionMessageToolCallFunction) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties ID, Function, Type are required.
type ChatCompletionMessageToolCallParam struct {
	// The ID of the tool call.
	ID string `json:"id,required"`
	// The function that the model called.
	Function ChatCompletionMessageToolCallFunctionParam `json:"function,omitzero,required"`
	// The type of the tool. Currently, only `function` is supported.
	//
	// This field can be elided, and will marshal its zero value as "function".
	Type constant.Function `json:"type,required"`
	paramObj
}

func (r ChatCompletionMessageToolCallParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionMessageToolCallParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionMessageToolCallParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The function that the model called.
//
// The properties Arguments, Name are required.
type ChatCompletionMessageToolCallFunctionParam struct {
	// The arguments to call the function with, as generated by the model in JSON
	// format. Note that the model does not always generate valid JSON, and may
	// hallucinate parameters not defined by your function schema. Validate the
	// arguments in your code before calling your function.
	Arguments string `json:"arguments,required"`
	// The name of the function to call.
	Name string `json:"name,required"`
	paramObj
}

func (r ChatCompletionMessageToolCallFunctionParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionMessageToolCallFunctionParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionMessageToolCallFunctionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Specifies a tool the model should use. Use to force the model to call a specific
// function.
//
// The properties Function, Type are required.
type ChatCompletionNamedToolChoiceParam struct {
	Function ChatCompletionNamedToolChoiceFunctionParam `json:"function,omitzero,required"`
	// The type of the tool. Currently, only `function` is supported.
	//
	// This field can be elided, and will marshal its zero value as "function".
	Type constant.Function `json:"type,required"`
	paramObj
}

func (r ChatCompletionNamedToolChoiceParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionNamedToolChoiceParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionNamedToolChoiceParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The property Name is required.
type ChatCompletionNamedToolChoiceFunctionParam struct {
	// The name of the function to call.
	Name string `json:"name,required"`
	paramObj
}

func (r ChatCompletionNamedToolChoiceFunctionParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionNamedToolChoiceFunctionParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionNamedToolChoiceFunctionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Static predicted output content, such as the content of a text file that is
// being regenerated.
//
// The properties Content, Type are required.
type ChatCompletionPredictionContentParam struct {
	// The content that should be matched when generating a model response. If
	// generated tokens would match this content, the entire model response can be
	// returned much more quickly.
	Content ChatCompletionPredictionContentContentUnionParam `json:"content,omitzero,required"`
	// The type of the predicted content you want to provide. This type is currently
	// always `content`.
	//
	// This field can be elided, and will marshal its zero value as "content".
	Type constant.Content `json:"type,required"`
	paramObj
}

func (r ChatCompletionPredictionContentParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionPredictionContentParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionPredictionContentParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ChatCompletionPredictionContentContentUnionParam struct {
	OfString              param.Opt[string]                    `json:",omitzero,inline"`
	OfArrayOfContentParts []ChatCompletionContentPartTextParam `json:",omitzero,inline"`
	paramUnion
}

func (u ChatCompletionPredictionContentContentUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfArrayOfContentParts)
}
func (u *ChatCompletionPredictionContentContentUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ChatCompletionPredictionContentContentUnionParam) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfArrayOfContentParts) {
		return &u.OfArrayOfContentParts
	}
	return nil
}

// A chat completion message generated by the model.
type ChatCompletionStoreMessage struct {
	// The identifier of the chat message.
	ID string `json:"id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
	ChatCompletionMessage
}

// Returns the unmodified JSON received from the API
func (r ChatCompletionStoreMessage) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionStoreMessage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Options for streaming response. Only set this when you set `stream: true`.
type ChatCompletionStreamOptionsParam struct {
	// If set, an additional chunk will be streamed before the `data: [DONE]` message.
	// The `usage` field on this chunk shows the token usage statistics for the entire
	// request, and the `choices` field will always be an empty array.
	//
	// All other chunks will also include a `usage` field, but with a null value.
	// **NOTE:** If the stream is interrupted, you may not receive the final usage
	// chunk which contains the total token usage for the request.
	IncludeUsage param.Opt[bool] `json:"include_usage,omitzero"`
	paramObj
}

func (r ChatCompletionStreamOptionsParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionStreamOptionsParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionStreamOptionsParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Developer-provided instructions that the model should follow, regardless of
// messages sent by the user. With o1 models and newer, use `developer` messages
// for this purpose instead.
//
// The properties Content, Role are required.
type ChatCompletionSystemMessageParam struct {
	// The contents of the system message.
	Content ChatCompletionSystemMessageParamContentUnion `json:"content,omitzero,required"`
	// An optional name for the participant. Provides the model information to
	// differentiate between participants of the same role.
	Name param.Opt[string] `json:"name,omitzero"`
	// The role of the messages author, in this case `system`.
	//
	// This field can be elided, and will marshal its zero value as "system".
	Role constant.System `json:"role,required"`
	paramObj
}

func (r ChatCompletionSystemMessageParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionSystemMessageParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionSystemMessageParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ChatCompletionSystemMessageParamContentUnion struct {
	OfString              param.Opt[string]                    `json:",omitzero,inline"`
	OfArrayOfContentParts []ChatCompletionContentPartTextParam `json:",omitzero,inline"`
	paramUnion
}

func (u ChatCompletionSystemMessageParamContentUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfArrayOfContentParts)
}
func (u *ChatCompletionSystemMessageParamContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ChatCompletionSystemMessageParamContentUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfArrayOfContentParts) {
		return &u.OfArrayOfContentParts
	}
	return nil
}

type ChatCompletionTokenLogprob struct {
	// The token.
	Token string `json:"token,required"`
	// A list of integers representing the UTF-8 bytes representation of the token.
	// Useful in instances where characters are represented by multiple tokens and
	// their byte representations must be combined to generate the correct text
	// representation. Can be `null` if there is no bytes representation for the token.
	Bytes []int64 `json:"bytes,required"`
	// The log probability of this token, if it is within the top 20 most likely
	// tokens. Otherwise, the value `-9999.0` is used to signify that the token is very
	// unlikely.
	Logprob float64 `json:"logprob,required"`
	// List of the most likely tokens and their log probability, at this token
	// position. In rare cases, there may be fewer than the number of requested
	// `top_logprobs` returned.
	TopLogprobs []ChatCompletionTokenLogprobTopLogprob `json:"top_logprobs,required"`
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
func (r ChatCompletionTokenLogprob) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionTokenLogprob) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ChatCompletionTokenLogprobTopLogprob struct {
	// The token.
	Token string `json:"token,required"`
	// A list of integers representing the UTF-8 bytes representation of the token.
	// Useful in instances where characters are represented by multiple tokens and
	// their byte representations must be combined to generate the correct text
	// representation. Can be `null` if there is no bytes representation for the token.
	Bytes []int64 `json:"bytes,required"`
	// The log probability of this token, if it is within the top 20 most likely
	// tokens. Otherwise, the value `-9999.0` is used to signify that the token is very
	// unlikely.
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
func (r ChatCompletionTokenLogprobTopLogprob) RawJSON() string { return r.JSON.raw }
func (r *ChatCompletionTokenLogprobTopLogprob) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Function, Type are required.
type ChatCompletionToolParam struct {
	Function shared.FunctionDefinitionParam `json:"function,omitzero,required"`
	// The type of the tool. Currently, only `function` is supported.
	//
	// This field can be elided, and will marshal its zero value as "function".
	Type constant.Function `json:"type,required"`
	paramObj
}

func (r ChatCompletionToolParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionToolParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionToolParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func ChatCompletionToolChoiceOptionParamOfChatCompletionNamedToolChoice(function ChatCompletionNamedToolChoiceFunctionParam) ChatCompletionToolChoiceOptionUnionParam {
	var variant ChatCompletionNamedToolChoiceParam
	variant.Function = function
	return ChatCompletionToolChoiceOptionUnionParam{OfChatCompletionNamedToolChoice: &variant}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ChatCompletionToolChoiceOptionUnionParam struct {
	// Check if union is this variant with !param.IsOmitted(union.OfAuto)
	OfAuto                          param.Opt[string]                   `json:",omitzero,inline"`
	OfChatCompletionNamedToolChoice *ChatCompletionNamedToolChoiceParam `json:",omitzero,inline"`
	paramUnion
}

func (u ChatCompletionToolChoiceOptionUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfChatCompletionNamedToolChoice)
}
func (u *ChatCompletionToolChoiceOptionUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ChatCompletionToolChoiceOptionUnionParam) asAny() any {
	if !param.IsOmitted(u.OfAuto) {
		return &u.OfAuto
	} else if !param.IsOmitted(u.OfChatCompletionNamedToolChoice) {
		return u.OfChatCompletionNamedToolChoice
	}
	return nil
}

// `none` means the model will not call any tool and instead generates a message.
// `auto` means the model can pick between generating a message or calling one or
// more tools. `required` means the model must call one or more tools.
type ChatCompletionToolChoiceOptionAuto string

const (
	ChatCompletionToolChoiceOptionAutoNone     ChatCompletionToolChoiceOptionAuto = "none"
	ChatCompletionToolChoiceOptionAutoAuto     ChatCompletionToolChoiceOptionAuto = "auto"
	ChatCompletionToolChoiceOptionAutoRequired ChatCompletionToolChoiceOptionAuto = "required"
)

// The properties Content, Role, ToolCallID are required.
type ChatCompletionToolMessageParam struct {
	// The contents of the tool message.
	Content ChatCompletionToolMessageParamContentUnion `json:"content,omitzero,required"`
	// Tool call that this message is responding to.
	ToolCallID string `json:"tool_call_id,required"`
	// The role of the messages author, in this case `tool`.
	//
	// This field can be elided, and will marshal its zero value as "tool".
	Role constant.Tool `json:"role,required"`
	paramObj
}

func (r ChatCompletionToolMessageParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionToolMessageParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionToolMessageParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ChatCompletionToolMessageParamContentUnion struct {
	OfString              param.Opt[string]                    `json:",omitzero,inline"`
	OfArrayOfContentParts []ChatCompletionContentPartTextParam `json:",omitzero,inline"`
	paramUnion
}

func (u ChatCompletionToolMessageParamContentUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfArrayOfContentParts)
}
func (u *ChatCompletionToolMessageParamContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ChatCompletionToolMessageParamContentUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfArrayOfContentParts) {
		return &u.OfArrayOfContentParts
	}
	return nil
}

// Messages sent by an end user, containing prompts or additional context
// information.
//
// The properties Content, Role are required.
type ChatCompletionUserMessageParam struct {
	// The contents of the user message.
	Content ChatCompletionUserMessageParamContentUnion `json:"content,omitzero,required"`
	// An optional name for the participant. Provides the model information to
	// differentiate between participants of the same role.
	Name param.Opt[string] `json:"name,omitzero"`
	// The role of the messages author, in this case `user`.
	//
	// This field can be elided, and will marshal its zero value as "user".
	Role constant.User `json:"role,required"`
	paramObj
}

func (r ChatCompletionUserMessageParam) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionUserMessageParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionUserMessageParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ChatCompletionUserMessageParamContentUnion struct {
	OfString              param.Opt[string]                     `json:",omitzero,inline"`
	OfArrayOfContentParts []ChatCompletionContentPartUnionParam `json:",omitzero,inline"`
	paramUnion
}

func (u ChatCompletionUserMessageParamContentUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfArrayOfContentParts)
}
func (u *ChatCompletionUserMessageParamContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ChatCompletionUserMessageParamContentUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfArrayOfContentParts) {
		return &u.OfArrayOfContentParts
	}
	return nil
}

type ChatCompletionNewParams struct {
	// A list of messages comprising the conversation so far. Depending on the
	// [model](https://platform.openai.com/docs/models) you use, different message
	// types (modalities) are supported, like
	// [text](https://platform.openai.com/docs/guides/text-generation),
	// [images](https://platform.openai.com/docs/guides/vision), and
	// [audio](https://platform.openai.com/docs/guides/audio).
	Messages []ChatCompletionMessageParamUnion `json:"messages,omitzero,required"`
	// Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI offers a
	// wide range of models with different capabilities, performance characteristics,
	// and price points. Refer to the
	// [model guide](https://platform.openai.com/docs/models) to browse and compare
	// available models.
	Model shared.ChatModel `json:"model,omitzero,required"`
	// Number between -2.0 and 2.0. Positive values penalize new tokens based on their
	// existing frequency in the text so far, decreasing the model's likelihood to
	// repeat the same line verbatim.
	FrequencyPenalty param.Opt[float64] `json:"frequency_penalty,omitzero"`
	// Whether to return log probabilities of the output tokens or not. If true,
	// returns the log probabilities of each output token returned in the `content` of
	// `message`.
	Logprobs param.Opt[bool] `json:"logprobs,omitzero"`
	// An upper bound for the number of tokens that can be generated for a completion,
	// including visible output tokens and
	// [reasoning tokens](https://platform.openai.com/docs/guides/reasoning).
	MaxCompletionTokens param.Opt[int64] `json:"max_completion_tokens,omitzero"`
	// The maximum number of [tokens](/tokenizer) that can be generated in the chat
	// completion. This value can be used to control
	// [costs](https://openai.com/api/pricing/) for text generated via API.
	//
	// This value is now deprecated in favor of `max_completion_tokens`, and is not
	// compatible with
	// [o-series models](https://platform.openai.com/docs/guides/reasoning).
	MaxTokens param.Opt[int64] `json:"max_tokens,omitzero"`
	// How many chat completion choices to generate for each input message. Note that
	// you will be charged based on the number of generated tokens across all of the
	// choices. Keep `n` as `1` to minimize costs.
	N param.Opt[int64] `json:"n,omitzero"`
	// Number between -2.0 and 2.0. Positive values penalize new tokens based on
	// whether they appear in the text so far, increasing the model's likelihood to
	// talk about new topics.
	PresencePenalty param.Opt[float64] `json:"presence_penalty,omitzero"`
	// This feature is in Beta. If specified, our system will make a best effort to
	// sample deterministically, such that repeated requests with the same `seed` and
	// parameters should return the same result. Determinism is not guaranteed, and you
	// should refer to the `system_fingerprint` response parameter to monitor changes
	// in the backend.
	Seed param.Opt[int64] `json:"seed,omitzero"`
	// Whether or not to store the output of this chat completion request for use in
	// our [model distillation](https://platform.openai.com/docs/guides/distillation)
	// or [evals](https://platform.openai.com/docs/guides/evals) products.
	//
	// Supports text and image inputs. Note: image inputs over 10MB will be dropped.
	Store param.Opt[bool] `json:"store,omitzero"`
	// What sampling temperature to use, between 0 and 2. Higher values like 0.8 will
	// make the output more random, while lower values like 0.2 will make it more
	// focused and deterministic. We generally recommend altering this or `top_p` but
	// not both.
	Temperature param.Opt[float64] `json:"temperature,omitzero"`
	// An integer between 0 and 20 specifying the number of most likely tokens to
	// return at each token position, each with an associated log probability.
	// `logprobs` must be set to `true` if this parameter is used.
	TopLogprobs param.Opt[int64] `json:"top_logprobs,omitzero"`
	// An alternative to sampling with temperature, called nucleus sampling, where the
	// model considers the results of the tokens with top_p probability mass. So 0.1
	// means only the tokens comprising the top 10% probability mass are considered.
	//
	// We generally recommend altering this or `temperature` but not both.
	TopP param.Opt[float64] `json:"top_p,omitzero"`
	// Whether to enable
	// [parallel function calling](https://platform.openai.com/docs/guides/function-calling#configuring-parallel-function-calling)
	// during tool use.
	ParallelToolCalls param.Opt[bool] `json:"parallel_tool_calls,omitzero"`
	// A stable identifier for your end-users. Used to boost cache hit rates by better
	// bucketing similar requests and to help OpenAI detect and prevent abuse.
	// [Learn more](https://platform.openai.com/docs/guides/safety-best-practices#end-user-ids).
	User param.Opt[string] `json:"user,omitzero"`
	// Parameters for audio output. Required when audio output is requested with
	// `modalities: ["audio"]`.
	// [Learn more](https://platform.openai.com/docs/guides/audio).
	Audio ChatCompletionAudioParam `json:"audio,omitzero"`
	// Modify the likelihood of specified tokens appearing in the completion.
	//
	// Accepts a JSON object that maps tokens (specified by their token ID in the
	// tokenizer) to an associated bias value from -100 to 100. Mathematically, the
	// bias is added to the logits generated by the model prior to sampling. The exact
	// effect will vary per model, but values between -1 and 1 should decrease or
	// increase likelihood of selection; values like -100 or 100 should result in a ban
	// or exclusive selection of the relevant token.
	LogitBias map[string]int64 `json:"logit_bias,omitzero"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,omitzero"`
	// Output types that you would like the model to generate. Most models are capable
	// of generating text, which is the default:
	//
	// `["text"]`
	//
	// The `gpt-4o-audio-preview` model can also be used to
	// [generate audio](https://platform.openai.com/docs/guides/audio). To request that
	// this model generate both text and audio responses, you can use:
	//
	// `["text", "audio"]`
	//
	// Any of "text", "audio".
	Modalities []string `json:"modalities,omitzero"`
	// **o-series models only**
	//
	// Constrains effort on reasoning for
	// [reasoning models](https://platform.openai.com/docs/guides/reasoning). Currently
	// supported values are `low`, `medium`, and `high`. Reducing reasoning effort can
	// result in faster responses and fewer tokens used on reasoning in a response.
	//
	// Any of "low", "medium", "high".
	ReasoningEffort shared.ReasoningEffort `json:"reasoning_effort,omitzero"`
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
	ServiceTier ChatCompletionNewParamsServiceTier `json:"service_tier,omitzero"`
	// Not supported with latest reasoning models `o3` and `o4-mini`.
	//
	// Up to 4 sequences where the API will stop generating further tokens. The
	// returned text will not contain the stop sequence.
	Stop ChatCompletionNewParamsStopUnion `json:"stop,omitzero"`
	// Options for streaming response. Only set this when you set `stream: true`.
	StreamOptions ChatCompletionStreamOptionsParam `json:"stream_options,omitzero"`
	// Deprecated in favor of `tool_choice`.
	//
	// Controls which (if any) function is called by the model.
	//
	// `none` means the model will not call a function and instead generates a message.
	//
	// `auto` means the model can pick between generating a message or calling a
	// function.
	//
	// Specifying a particular function via `{"name": "my_function"}` forces the model
	// to call that function.
	//
	// `none` is the default when no functions are present. `auto` is the default if
	// functions are present.
	FunctionCall ChatCompletionNewParamsFunctionCallUnion `json:"function_call,omitzero"`
	// Deprecated in favor of `tools`.
	//
	// A list of functions the model may generate JSON inputs for.
	Functions []ChatCompletionNewParamsFunction `json:"functions,omitzero"`
	// Static predicted output content, such as the content of a text file that is
	// being regenerated.
	Prediction ChatCompletionPredictionContentParam `json:"prediction,omitzero"`
	// An object specifying the format that the model must output.
	//
	// Setting to `{ "type": "json_schema", "json_schema": {...} }` enables Structured
	// Outputs which ensures the model will match your supplied JSON schema. Learn more
	// in the
	// [Structured Outputs guide](https://platform.openai.com/docs/guides/structured-outputs).
	//
	// Setting to `{ "type": "json_object" }` enables the older JSON mode, which
	// ensures the message the model generates is valid JSON. Using `json_schema` is
	// preferred for models that support it.
	ResponseFormat ChatCompletionNewParamsResponseFormatUnion `json:"response_format,omitzero"`
	// Controls which (if any) tool is called by the model. `none` means the model will
	// not call any tool and instead generates a message. `auto` means the model can
	// pick between generating a message or calling one or more tools. `required` means
	// the model must call one or more tools. Specifying a particular tool via
	// `{"type": "function", "function": {"name": "my_function"}}` forces the model to
	// call that tool.
	//
	// `none` is the default when no tools are present. `auto` is the default if tools
	// are present.
	ToolChoice ChatCompletionToolChoiceOptionUnionParam `json:"tool_choice,omitzero"`
	// A list of tools the model may call. Currently, only functions are supported as a
	// tool. Use this to provide a list of functions the model may generate JSON inputs
	// for. A max of 128 functions are supported.
	Tools []ChatCompletionToolParam `json:"tools,omitzero"`
	// This tool searches the web for relevant results to use in a response. Learn more
	// about the
	// [web search tool](https://platform.openai.com/docs/guides/tools-web-search?api-mode=chat).
	WebSearchOptions ChatCompletionNewParamsWebSearchOptions `json:"web_search_options,omitzero"`
	paramObj
}

func (r ChatCompletionNewParams) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionNewParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionNewParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ChatCompletionNewParamsFunctionCallUnion struct {
	// Check if union is this variant with !param.IsOmitted(union.OfFunctionCallMode)
	OfFunctionCallMode   param.Opt[string]                      `json:",omitzero,inline"`
	OfFunctionCallOption *ChatCompletionFunctionCallOptionParam `json:",omitzero,inline"`
	paramUnion
}

func (u ChatCompletionNewParamsFunctionCallUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfFunctionCallMode, u.OfFunctionCallOption)
}
func (u *ChatCompletionNewParamsFunctionCallUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ChatCompletionNewParamsFunctionCallUnion) asAny() any {
	if !param.IsOmitted(u.OfFunctionCallMode) {
		return &u.OfFunctionCallMode
	} else if !param.IsOmitted(u.OfFunctionCallOption) {
		return u.OfFunctionCallOption
	}
	return nil
}

// `none` means the model will not call a function and instead generates a message.
// `auto` means the model can pick between generating a message or calling a
// function.
type ChatCompletionNewParamsFunctionCallFunctionCallMode string

const (
	ChatCompletionNewParamsFunctionCallFunctionCallModeNone ChatCompletionNewParamsFunctionCallFunctionCallMode = "none"
	ChatCompletionNewParamsFunctionCallFunctionCallModeAuto ChatCompletionNewParamsFunctionCallFunctionCallMode = "auto"
)

// Deprecated: deprecated
//
// The property Name is required.
type ChatCompletionNewParamsFunction struct {
	// The name of the function to be called. Must be a-z, A-Z, 0-9, or contain
	// underscores and dashes, with a maximum length of 64.
	Name string `json:"name,required"`
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
	Parameters shared.FunctionParameters `json:"parameters,omitzero"`
	paramObj
}

func (r ChatCompletionNewParamsFunction) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionNewParamsFunction
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionNewParamsFunction) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ChatCompletionNewParamsResponseFormatUnion struct {
	OfText       *shared.ResponseFormatTextParam       `json:",omitzero,inline"`
	OfJSONSchema *shared.ResponseFormatJSONSchemaParam `json:",omitzero,inline"`
	OfJSONObject *shared.ResponseFormatJSONObjectParam `json:",omitzero,inline"`
	paramUnion
}

func (u ChatCompletionNewParamsResponseFormatUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfText, u.OfJSONSchema, u.OfJSONObject)
}
func (u *ChatCompletionNewParamsResponseFormatUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ChatCompletionNewParamsResponseFormatUnion) asAny() any {
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
func (u ChatCompletionNewParamsResponseFormatUnion) GetJSONSchema() *shared.ResponseFormatJSONSchemaJSONSchemaParam {
	if vt := u.OfJSONSchema; vt != nil {
		return &vt.JSONSchema
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ChatCompletionNewParamsResponseFormatUnion) GetType() *string {
	if vt := u.OfText; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfJSONSchema; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfJSONObject; vt != nil {
		return (*string)(&vt.Type)
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
type ChatCompletionNewParamsServiceTier string

const (
	ChatCompletionNewParamsServiceTierAuto     ChatCompletionNewParamsServiceTier = "auto"
	ChatCompletionNewParamsServiceTierDefault  ChatCompletionNewParamsServiceTier = "default"
	ChatCompletionNewParamsServiceTierFlex     ChatCompletionNewParamsServiceTier = "flex"
	ChatCompletionNewParamsServiceTierScale    ChatCompletionNewParamsServiceTier = "scale"
	ChatCompletionNewParamsServiceTierPriority ChatCompletionNewParamsServiceTier = "priority"
)

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ChatCompletionNewParamsStopUnion struct {
	OfString      param.Opt[string] `json:",omitzero,inline"`
	OfStringArray []string          `json:",omitzero,inline"`
	paramUnion
}

func (u ChatCompletionNewParamsStopUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfStringArray)
}
func (u *ChatCompletionNewParamsStopUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ChatCompletionNewParamsStopUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfStringArray) {
		return &u.OfStringArray
	}
	return nil
}

// This tool searches the web for relevant results to use in a response. Learn more
// about the
// [web search tool](https://platform.openai.com/docs/guides/tools-web-search?api-mode=chat).
type ChatCompletionNewParamsWebSearchOptions struct {
	// Approximate location parameters for the search.
	UserLocation ChatCompletionNewParamsWebSearchOptionsUserLocation `json:"user_location,omitzero"`
	// High level guidance for the amount of context window space to use for the
	// search. One of `low`, `medium`, or `high`. `medium` is the default.
	//
	// Any of "low", "medium", "high".
	SearchContextSize string `json:"search_context_size,omitzero"`
	paramObj
}

func (r ChatCompletionNewParamsWebSearchOptions) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionNewParamsWebSearchOptions
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionNewParamsWebSearchOptions) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[ChatCompletionNewParamsWebSearchOptions](
		"search_context_size", "low", "medium", "high",
	)
}

// Approximate location parameters for the search.
//
// The properties Approximate, Type are required.
type ChatCompletionNewParamsWebSearchOptionsUserLocation struct {
	// Approximate location parameters for the search.
	Approximate ChatCompletionNewParamsWebSearchOptionsUserLocationApproximate `json:"approximate,omitzero,required"`
	// The type of location approximation. Always `approximate`.
	//
	// This field can be elided, and will marshal its zero value as "approximate".
	Type constant.Approximate `json:"type,required"`
	paramObj
}

func (r ChatCompletionNewParamsWebSearchOptionsUserLocation) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionNewParamsWebSearchOptionsUserLocation
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionNewParamsWebSearchOptionsUserLocation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Approximate location parameters for the search.
type ChatCompletionNewParamsWebSearchOptionsUserLocationApproximate struct {
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
	paramObj
}

func (r ChatCompletionNewParamsWebSearchOptionsUserLocationApproximate) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionNewParamsWebSearchOptionsUserLocationApproximate
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionNewParamsWebSearchOptionsUserLocationApproximate) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ChatCompletionUpdateParams struct {
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,omitzero,required"`
	paramObj
}

func (r ChatCompletionUpdateParams) MarshalJSON() (data []byte, err error) {
	type shadow ChatCompletionUpdateParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ChatCompletionUpdateParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ChatCompletionListParams struct {
	// Identifier for the last chat completion from the previous pagination request.
	After param.Opt[string] `query:"after,omitzero" json:"-"`
	// Number of Chat Completions to retrieve.
	Limit param.Opt[int64] `query:"limit,omitzero" json:"-"`
	// The model used to generate the Chat Completions.
	Model param.Opt[string] `query:"model,omitzero" json:"-"`
	// A list of metadata keys to filter the Chat Completions by. Example:
	//
	// `metadata[key1]=value1&metadata[key2]=value2`
	Metadata shared.Metadata `query:"metadata,omitzero" json:"-"`
	// Sort order for Chat Completions by timestamp. Use `asc` for ascending order or
	// `desc` for descending order. Defaults to `asc`.
	//
	// Any of "asc", "desc".
	Order ChatCompletionListParamsOrder `query:"order,omitzero" json:"-"`
	paramObj
}

// URLQuery serializes [ChatCompletionListParams]'s query parameters as
// `url.Values`.
func (r ChatCompletionListParams) URLQuery() (v url.Values, err error) {
	return apiquery.MarshalWithSettings(r, apiquery.QuerySettings{
		ArrayFormat:  apiquery.ArrayQueryFormatBrackets,
		NestedFormat: apiquery.NestedQueryFormatBrackets,
	})
}

// Sort order for Chat Completions by timestamp. Use `asc` for ascending order or
// `desc` for descending order. Defaults to `asc`.
type ChatCompletionListParamsOrder string

const (
	ChatCompletionListParamsOrderAsc  ChatCompletionListParamsOrder = "asc"
	ChatCompletionListParamsOrderDesc ChatCompletionListParamsOrder = "desc"
)
