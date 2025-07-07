// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/anthropics/anthropic-sdk-go/internal/apijson"
	"github.com/anthropics/anthropic-sdk-go/internal/apiquery"
	"github.com/anthropics/anthropic-sdk-go/internal/requestconfig"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/jsonl"
	"github.com/anthropics/anthropic-sdk-go/packages/pagination"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/anthropics/anthropic-sdk-go/packages/respjson"
	"github.com/anthropics/anthropic-sdk-go/shared"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
)

// MessageBatchService contains methods and other services that help with
// interacting with the anthropic API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewMessageBatchService] method instead.
type MessageBatchService struct {
	Options []option.RequestOption
}

// NewMessageBatchService generates a new service that applies the given options to
// each request. These options are applied after the parent client's options (if
// there is one), and before any request-specific options.
func NewMessageBatchService(opts ...option.RequestOption) (r MessageBatchService) {
	r = MessageBatchService{}
	r.Options = opts
	return
}

// Send a batch of Message creation requests.
//
// The Message Batches API can be used to process multiple Messages API requests at
// once. Once a Message Batch is created, it begins processing immediately. Batches
// can take up to 24 hours to complete.
//
// Learn more about the Message Batches API in our
// [user guide](/en/docs/build-with-claude/batch-processing)
func (r *MessageBatchService) New(ctx context.Context, body MessageBatchNewParams, opts ...option.RequestOption) (res *MessageBatch, err error) {
	opts = append(r.Options[:], opts...)
	path := "v1/messages/batches"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// This endpoint is idempotent and can be used to poll for Message Batch
// completion. To access the results of a Message Batch, make a request to the
// `results_url` field in the response.
//
// Learn more about the Message Batches API in our
// [user guide](/en/docs/build-with-claude/batch-processing)
func (r *MessageBatchService) Get(ctx context.Context, messageBatchID string, opts ...option.RequestOption) (res *MessageBatch, err error) {
	opts = append(r.Options[:], opts...)
	if messageBatchID == "" {
		err = errors.New("missing required message_batch_id parameter")
		return
	}
	path := fmt.Sprintf("v1/messages/batches/%s", messageBatchID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// List all Message Batches within a Workspace. Most recently created batches are
// returned first.
//
// Learn more about the Message Batches API in our
// [user guide](/en/docs/build-with-claude/batch-processing)
func (r *MessageBatchService) List(ctx context.Context, query MessageBatchListParams, opts ...option.RequestOption) (res *pagination.Page[MessageBatch], err error) {
	var raw *http.Response
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithResponseInto(&raw)}, opts...)
	path := "v1/messages/batches"
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

// List all Message Batches within a Workspace. Most recently created batches are
// returned first.
//
// Learn more about the Message Batches API in our
// [user guide](/en/docs/build-with-claude/batch-processing)
func (r *MessageBatchService) ListAutoPaging(ctx context.Context, query MessageBatchListParams, opts ...option.RequestOption) *pagination.PageAutoPager[MessageBatch] {
	return pagination.NewPageAutoPager(r.List(ctx, query, opts...))
}

// Delete a Message Batch.
//
// Message Batches can only be deleted once they've finished processing. If you'd
// like to delete an in-progress batch, you must first cancel it.
//
// Learn more about the Message Batches API in our
// [user guide](/en/docs/build-with-claude/batch-processing)
func (r *MessageBatchService) Delete(ctx context.Context, messageBatchID string, opts ...option.RequestOption) (res *DeletedMessageBatch, err error) {
	opts = append(r.Options[:], opts...)
	if messageBatchID == "" {
		err = errors.New("missing required message_batch_id parameter")
		return
	}
	path := fmt.Sprintf("v1/messages/batches/%s", messageBatchID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodDelete, path, nil, &res, opts...)
	return
}

// Batches may be canceled any time before processing ends. Once cancellation is
// initiated, the batch enters a `canceling` state, at which time the system may
// complete any in-progress, non-interruptible requests before finalizing
// cancellation.
//
// The number of canceled requests is specified in `request_counts`. To determine
// which requests were canceled, check the individual results within the batch.
// Note that cancellation may not result in any canceled requests if they were
// non-interruptible.
//
// Learn more about the Message Batches API in our
// [user guide](/en/docs/build-with-claude/batch-processing)
func (r *MessageBatchService) Cancel(ctx context.Context, messageBatchID string, opts ...option.RequestOption) (res *MessageBatch, err error) {
	opts = append(r.Options[:], opts...)
	if messageBatchID == "" {
		err = errors.New("missing required message_batch_id parameter")
		return
	}
	path := fmt.Sprintf("v1/messages/batches/%s/cancel", messageBatchID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, nil, &res, opts...)
	return
}

// Streams the results of a Message Batch as a `.jsonl` file.
//
// Each line in the file is a JSON object containing the result of a single request
// in the Message Batch. Results are not guaranteed to be in the same order as
// requests. Use the `custom_id` field to match results to requests.
//
// Learn more about the Message Batches API in our
// [user guide](/en/docs/build-with-claude/batch-processing)
func (r *MessageBatchService) ResultsStreaming(ctx context.Context, messageBatchID string, opts ...option.RequestOption) (stream *jsonl.Stream[MessageBatchIndividualResponse]) {
	var (
		raw *http.Response
		err error
	)
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("Accept", "application/x-jsonl")}, opts...)
	if messageBatchID == "" {
		err = errors.New("missing required message_batch_id parameter")
		return
	}
	path := fmt.Sprintf("v1/messages/batches/%s/results", messageBatchID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &raw, opts...)
	return jsonl.NewStream[MessageBatchIndividualResponse](raw, err)
}

type DeletedMessageBatch struct {
	// ID of the Message Batch.
	ID string `json:"id,required"`
	// Deleted object type.
	//
	// For Message Batches, this is always `"message_batch_deleted"`.
	Type constant.MessageBatchDeleted `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r DeletedMessageBatch) RawJSON() string { return r.JSON.raw }
func (r *DeletedMessageBatch) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type MessageBatch struct {
	// Unique object identifier.
	//
	// The format and length of IDs may change over time.
	ID string `json:"id,required"`
	// RFC 3339 datetime string representing the time at which the Message Batch was
	// archived and its results became unavailable.
	ArchivedAt time.Time `json:"archived_at,required" format:"date-time"`
	// RFC 3339 datetime string representing the time at which cancellation was
	// initiated for the Message Batch. Specified only if cancellation was initiated.
	CancelInitiatedAt time.Time `json:"cancel_initiated_at,required" format:"date-time"`
	// RFC 3339 datetime string representing the time at which the Message Batch was
	// created.
	CreatedAt time.Time `json:"created_at,required" format:"date-time"`
	// RFC 3339 datetime string representing the time at which processing for the
	// Message Batch ended. Specified only once processing ends.
	//
	// Processing ends when every request in a Message Batch has either succeeded,
	// errored, canceled, or expired.
	EndedAt time.Time `json:"ended_at,required" format:"date-time"`
	// RFC 3339 datetime string representing the time at which the Message Batch will
	// expire and end processing, which is 24 hours after creation.
	ExpiresAt time.Time `json:"expires_at,required" format:"date-time"`
	// Processing status of the Message Batch.
	//
	// Any of "in_progress", "canceling", "ended".
	ProcessingStatus MessageBatchProcessingStatus `json:"processing_status,required"`
	// Tallies requests within the Message Batch, categorized by their status.
	//
	// Requests start as `processing` and move to one of the other statuses only once
	// processing of the entire batch ends. The sum of all values always matches the
	// total number of requests in the batch.
	RequestCounts MessageBatchRequestCounts `json:"request_counts,required"`
	// URL to a `.jsonl` file containing the results of the Message Batch requests.
	// Specified only once processing ends.
	//
	// Results in the file are not guaranteed to be in the same order as requests. Use
	// the `custom_id` field to match results to requests.
	ResultsURL string `json:"results_url,required"`
	// Object type.
	//
	// For Message Batches, this is always `"message_batch"`.
	Type constant.MessageBatch `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID                respjson.Field
		ArchivedAt        respjson.Field
		CancelInitiatedAt respjson.Field
		CreatedAt         respjson.Field
		EndedAt           respjson.Field
		ExpiresAt         respjson.Field
		ProcessingStatus  respjson.Field
		RequestCounts     respjson.Field
		ResultsURL        respjson.Field
		Type              respjson.Field
		ExtraFields       map[string]respjson.Field
		raw               string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r MessageBatch) RawJSON() string { return r.JSON.raw }
func (r *MessageBatch) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Processing status of the Message Batch.
type MessageBatchProcessingStatus string

const (
	MessageBatchProcessingStatusInProgress MessageBatchProcessingStatus = "in_progress"
	MessageBatchProcessingStatusCanceling  MessageBatchProcessingStatus = "canceling"
	MessageBatchProcessingStatusEnded      MessageBatchProcessingStatus = "ended"
)

type MessageBatchCanceledResult struct {
	Type constant.Canceled `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r MessageBatchCanceledResult) RawJSON() string { return r.JSON.raw }
func (r *MessageBatchCanceledResult) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type MessageBatchErroredResult struct {
	Error shared.ErrorResponse `json:"error,required"`
	Type  constant.Errored     `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Error       respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r MessageBatchErroredResult) RawJSON() string { return r.JSON.raw }
func (r *MessageBatchErroredResult) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type MessageBatchExpiredResult struct {
	Type constant.Expired `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r MessageBatchExpiredResult) RawJSON() string { return r.JSON.raw }
func (r *MessageBatchExpiredResult) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// This is a single line in the response `.jsonl` file and does not represent the
// response as a whole.
type MessageBatchIndividualResponse struct {
	// Developer-provided ID created for each request in a Message Batch. Useful for
	// matching results to requests, as results may be given out of request order.
	//
	// Must be unique for each request within the Message Batch.
	CustomID string `json:"custom_id,required"`
	// Processing result for this request.
	//
	// Contains a Message output if processing was successful, an error response if
	// processing failed, or the reason why processing was not attempted, such as
	// cancellation or expiration.
	Result MessageBatchResultUnion `json:"result,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		CustomID    respjson.Field
		Result      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r MessageBatchIndividualResponse) RawJSON() string { return r.JSON.raw }
func (r *MessageBatchIndividualResponse) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type MessageBatchRequestCounts struct {
	// Number of requests in the Message Batch that have been canceled.
	//
	// This is zero until processing of the entire Message Batch has ended.
	Canceled int64 `json:"canceled,required"`
	// Number of requests in the Message Batch that encountered an error.
	//
	// This is zero until processing of the entire Message Batch has ended.
	Errored int64 `json:"errored,required"`
	// Number of requests in the Message Batch that have expired.
	//
	// This is zero until processing of the entire Message Batch has ended.
	Expired int64 `json:"expired,required"`
	// Number of requests in the Message Batch that are processing.
	Processing int64 `json:"processing,required"`
	// Number of requests in the Message Batch that have completed successfully.
	//
	// This is zero until processing of the entire Message Batch has ended.
	Succeeded int64 `json:"succeeded,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Canceled    respjson.Field
		Errored     respjson.Field
		Expired     respjson.Field
		Processing  respjson.Field
		Succeeded   respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r MessageBatchRequestCounts) RawJSON() string { return r.JSON.raw }
func (r *MessageBatchRequestCounts) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// MessageBatchResultUnion contains all possible properties and values from
// [MessageBatchSucceededResult], [MessageBatchErroredResult],
// [MessageBatchCanceledResult], [MessageBatchExpiredResult].
//
// Use the [MessageBatchResultUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type MessageBatchResultUnion struct {
	// This field is from variant [MessageBatchSucceededResult].
	Message Message `json:"message"`
	// Any of "succeeded", "errored", "canceled", "expired".
	Type string `json:"type"`
	// This field is from variant [MessageBatchErroredResult].
	Error shared.ErrorResponse `json:"error"`
	JSON  struct {
		Message respjson.Field
		Type    respjson.Field
		Error   respjson.Field
		raw     string
	} `json:"-"`
}

// anyMessageBatchResult is implemented by each variant of
// [MessageBatchResultUnion] to add type safety for the return type of
// [MessageBatchResultUnion.AsAny]
type anyMessageBatchResult interface {
	implMessageBatchResultUnion()
}

func (MessageBatchSucceededResult) implMessageBatchResultUnion() {}
func (MessageBatchErroredResult) implMessageBatchResultUnion()   {}
func (MessageBatchCanceledResult) implMessageBatchResultUnion()  {}
func (MessageBatchExpiredResult) implMessageBatchResultUnion()   {}

// Use the following switch statement to find the correct variant
//
//	switch variant := MessageBatchResultUnion.AsAny().(type) {
//	case anthropic.MessageBatchSucceededResult:
//	case anthropic.MessageBatchErroredResult:
//	case anthropic.MessageBatchCanceledResult:
//	case anthropic.MessageBatchExpiredResult:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u MessageBatchResultUnion) AsAny() anyMessageBatchResult {
	switch u.Type {
	case "succeeded":
		return u.AsSucceeded()
	case "errored":
		return u.AsErrored()
	case "canceled":
		return u.AsCanceled()
	case "expired":
		return u.AsExpired()
	}
	return nil
}

func (u MessageBatchResultUnion) AsSucceeded() (v MessageBatchSucceededResult) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u MessageBatchResultUnion) AsErrored() (v MessageBatchErroredResult) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u MessageBatchResultUnion) AsCanceled() (v MessageBatchCanceledResult) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u MessageBatchResultUnion) AsExpired() (v MessageBatchExpiredResult) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u MessageBatchResultUnion) RawJSON() string { return u.JSON.raw }

func (r *MessageBatchResultUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type MessageBatchSucceededResult struct {
	Message Message            `json:"message,required"`
	Type    constant.Succeeded `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Message     respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r MessageBatchSucceededResult) RawJSON() string { return r.JSON.raw }
func (r *MessageBatchSucceededResult) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type MessageBatchNewParams struct {
	// List of requests for prompt completion. Each is an individual request to create
	// a Message.
	Requests []MessageBatchNewParamsRequest `json:"requests,omitzero,required"`
	paramObj
}

func (r MessageBatchNewParams) MarshalJSON() (data []byte, err error) {
	type shadow MessageBatchNewParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *MessageBatchNewParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties CustomID, Params are required.
type MessageBatchNewParamsRequest struct {
	// Developer-provided ID created for each request in a Message Batch. Useful for
	// matching results to requests, as results may be given out of request order.
	//
	// Must be unique for each request within the Message Batch.
	CustomID string `json:"custom_id,required"`
	// Messages API creation parameters for the individual request.
	//
	// See the [Messages API reference](/en/api/messages) for full documentation on
	// available parameters.
	Params MessageBatchNewParamsRequestParams `json:"params,omitzero,required"`
	paramObj
}

func (r MessageBatchNewParamsRequest) MarshalJSON() (data []byte, err error) {
	type shadow MessageBatchNewParamsRequest
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *MessageBatchNewParamsRequest) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Messages API creation parameters for the individual request.
//
// See the [Messages API reference](/en/api/messages) for full documentation on
// available parameters.
//
// The properties MaxTokens, Messages, Model are required.
type MessageBatchNewParamsRequestParams struct {
	// The maximum number of tokens to generate before stopping.
	//
	// Note that our models may stop _before_ reaching this maximum. This parameter
	// only specifies the absolute maximum number of tokens to generate.
	//
	// Different models have different maximum values for this parameter. See
	// [models](https://docs.anthropic.com/en/docs/models-overview) for details.
	MaxTokens int64 `json:"max_tokens,required"`
	// Input messages.
	//
	// Our models are trained to operate on alternating `user` and `assistant`
	// conversational turns. When creating a new `Message`, you specify the prior
	// conversational turns with the `messages` parameter, and the model then generates
	// the next `Message` in the conversation. Consecutive `user` or `assistant` turns
	// in your request will be combined into a single turn.
	//
	// Each input message must be an object with a `role` and `content`. You can
	// specify a single `user`-role message, or you can include multiple `user` and
	// `assistant` messages.
	//
	// If the final message uses the `assistant` role, the response content will
	// continue immediately from the content in that message. This can be used to
	// constrain part of the model's response.
	//
	// Example with a single `user` message:
	//
	// ```json
	// [{ "role": "user", "content": "Hello, Claude" }]
	// ```
	//
	// Example with multiple conversational turns:
	//
	// ```json
	// [
	//
	//	{ "role": "user", "content": "Hello there." },
	//	{ "role": "assistant", "content": "Hi, I'm Claude. How can I help you?" },
	//	{ "role": "user", "content": "Can you explain LLMs in plain English?" }
	//
	// ]
	// ```
	//
	// Example with a partially-filled response from Claude:
	//
	// ```json
	// [
	//
	//	{
	//	  "role": "user",
	//	  "content": "What's the Greek name for Sun? (A) Sol (B) Helios (C) Sun"
	//	},
	//	{ "role": "assistant", "content": "The best answer is (" }
	//
	// ]
	// ```
	//
	// Each input message `content` may be either a single `string` or an array of
	// content blocks, where each block has a specific `type`. Using a `string` for
	// `content` is shorthand for an array of one content block of type `"text"`. The
	// following input messages are equivalent:
	//
	// ```json
	// { "role": "user", "content": "Hello, Claude" }
	// ```
	//
	// ```json
	// { "role": "user", "content": [{ "type": "text", "text": "Hello, Claude" }] }
	// ```
	//
	// Starting with Claude 3 models, you can also send image content blocks:
	//
	// ```json
	//
	//	{
	//	  "role": "user",
	//	  "content": [
	//	    {
	//	      "type": "image",
	//	      "source": {
	//	        "type": "base64",
	//	        "media_type": "image/jpeg",
	//	        "data": "/9j/4AAQSkZJRg..."
	//	      }
	//	    },
	//	    { "type": "text", "text": "What is in this image?" }
	//	  ]
	//	}
	//
	// ```
	//
	// We currently support the `base64` source type for images, and the `image/jpeg`,
	// `image/png`, `image/gif`, and `image/webp` media types.
	//
	// See [examples](https://docs.anthropic.com/en/api/messages-examples#vision) for
	// more input examples.
	//
	// Note that if you want to include a
	// [system prompt](https://docs.anthropic.com/en/docs/system-prompts), you can use
	// the top-level `system` parameter — there is no `"system"` role for input
	// messages in the Messages API.
	//
	// There is a limit of 100000 messages in a single request.
	Messages []MessageParam `json:"messages,omitzero,required"`
	// The model that will complete your prompt.\n\nSee
	// [models](https://docs.anthropic.com/en/docs/models-overview) for additional
	// details and options.
	Model Model `json:"model,omitzero,required"`
	// Whether to incrementally stream the response using server-sent events.
	//
	// See [streaming](https://docs.anthropic.com/en/api/messages-streaming) for
	// details.
	Stream param.Opt[bool] `json:"stream,omitzero"`
	// Amount of randomness injected into the response.
	//
	// Defaults to `1.0`. Ranges from `0.0` to `1.0`. Use `temperature` closer to `0.0`
	// for analytical / multiple choice, and closer to `1.0` for creative and
	// generative tasks.
	//
	// Note that even with `temperature` of `0.0`, the results will not be fully
	// deterministic.
	Temperature param.Opt[float64] `json:"temperature,omitzero"`
	// Only sample from the top K options for each subsequent token.
	//
	// Used to remove "long tail" low probability responses.
	// [Learn more technical details here](https://towardsdatascience.com/how-to-sample-from-language-models-682bceb97277).
	//
	// Recommended for advanced use cases only. You usually only need to use
	// `temperature`.
	TopK param.Opt[int64] `json:"top_k,omitzero"`
	// Use nucleus sampling.
	//
	// In nucleus sampling, we compute the cumulative distribution over all the options
	// for each subsequent token in decreasing probability order and cut it off once it
	// reaches a particular probability specified by `top_p`. You should either alter
	// `temperature` or `top_p`, but not both.
	//
	// Recommended for advanced use cases only. You usually only need to use
	// `temperature`.
	TopP param.Opt[float64] `json:"top_p,omitzero"`
	// An object describing metadata about the request.
	Metadata MetadataParam `json:"metadata,omitzero"`
	// Determines whether to use priority capacity (if available) or standard capacity
	// for this request.
	//
	// Anthropic offers different levels of service for your API requests. See
	// [service-tiers](https://docs.anthropic.com/en/api/service-tiers) for details.
	//
	// Any of "auto", "standard_only".
	ServiceTier string `json:"service_tier,omitzero"`
	// Custom text sequences that will cause the model to stop generating.
	//
	// Our models will normally stop when they have naturally completed their turn,
	// which will result in a response `stop_reason` of `"end_turn"`.
	//
	// If you want the model to stop generating when it encounters custom strings of
	// text, you can use the `stop_sequences` parameter. If the model encounters one of
	// the custom sequences, the response `stop_reason` value will be `"stop_sequence"`
	// and the response `stop_sequence` value will contain the matched stop sequence.
	StopSequences []string `json:"stop_sequences,omitzero"`
	// System prompt.
	//
	// A system prompt is a way of providing context and instructions to Claude, such
	// as specifying a particular goal or role. See our
	// [guide to system prompts](https://docs.anthropic.com/en/docs/system-prompts).
	System []TextBlockParam `json:"system,omitzero"`
	// Configuration for enabling Claude's extended thinking.
	//
	// When enabled, responses include `thinking` content blocks showing Claude's
	// thinking process before the final answer. Requires a minimum budget of 1,024
	// tokens and counts towards your `max_tokens` limit.
	//
	// See
	// [extended thinking](https://docs.anthropic.com/en/docs/build-with-claude/extended-thinking)
	// for details.
	Thinking ThinkingConfigParamUnion `json:"thinking,omitzero"`
	// How the model should use the provided tools. The model can use a specific tool,
	// any available tool, decide by itself, or not use tools at all.
	ToolChoice ToolChoiceUnionParam `json:"tool_choice,omitzero"`
	// Definitions of tools that the model may use.
	//
	// If you include `tools` in your API request, the model may return `tool_use`
	// content blocks that represent the model's use of those tools. You can then run
	// those tools using the tool input generated by the model and then optionally
	// return results back to the model using `tool_result` content blocks.
	//
	// Each tool definition includes:
	//
	//   - `name`: Name of the tool.
	//   - `description`: Optional, but strongly-recommended description of the tool.
	//   - `input_schema`: [JSON schema](https://json-schema.org/draft/2020-12) for the
	//     tool `input` shape that the model will produce in `tool_use` output content
	//     blocks.
	//
	// For example, if you defined `tools` as:
	//
	// ```json
	// [
	//
	//	{
	//	  "name": "get_stock_price",
	//	  "description": "Get the current stock price for a given ticker symbol.",
	//	  "input_schema": {
	//	    "type": "object",
	//	    "properties": {
	//	      "ticker": {
	//	        "type": "string",
	//	        "description": "The stock ticker symbol, e.g. AAPL for Apple Inc."
	//	      }
	//	    },
	//	    "required": ["ticker"]
	//	  }
	//	}
	//
	// ]
	// ```
	//
	// And then asked the model "What's the S&P 500 at today?", the model might produce
	// `tool_use` content blocks in the response like this:
	//
	// ```json
	// [
	//
	//	{
	//	  "type": "tool_use",
	//	  "id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV",
	//	  "name": "get_stock_price",
	//	  "input": { "ticker": "^GSPC" }
	//	}
	//
	// ]
	// ```
	//
	// You might then run your `get_stock_price` tool with `{"ticker": "^GSPC"}` as an
	// input, and return the following back to the model in a subsequent `user`
	// message:
	//
	// ```json
	// [
	//
	//	{
	//	  "type": "tool_result",
	//	  "tool_use_id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV",
	//	  "content": "259.75 USD"
	//	}
	//
	// ]
	// ```
	//
	// Tools can be used for workflows that include running client-side tools and
	// functions, or more generally whenever you want the model to produce a particular
	// JSON structure of output.
	//
	// See our [guide](https://docs.anthropic.com/en/docs/tool-use) for more details.
	Tools []ToolUnionParam `json:"tools,omitzero"`
	paramObj
}

func (r MessageBatchNewParamsRequestParams) MarshalJSON() (data []byte, err error) {
	type shadow MessageBatchNewParamsRequestParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *MessageBatchNewParamsRequestParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[MessageBatchNewParamsRequestParams](
		"service_tier", "auto", "standard_only",
	)
}

type MessageBatchListParams struct {
	// ID of the object to use as a cursor for pagination. When provided, returns the
	// page of results immediately after this object.
	AfterID param.Opt[string] `query:"after_id,omitzero" json:"-"`
	// ID of the object to use as a cursor for pagination. When provided, returns the
	// page of results immediately before this object.
	BeforeID param.Opt[string] `query:"before_id,omitzero" json:"-"`
	// Number of items to return per page.
	//
	// Defaults to `20`. Ranges from `1` to `1000`.
	Limit param.Opt[int64] `query:"limit,omitzero" json:"-"`
	paramObj
}

// URLQuery serializes [MessageBatchListParams]'s query parameters as `url.Values`.
func (r MessageBatchListParams) URLQuery() (v url.Values, err error) {
	return apiquery.MarshalWithSettings(r, apiquery.QuerySettings{
		ArrayFormat:  apiquery.ArrayQueryFormatComma,
		NestedFormat: apiquery.NestedQueryFormatBrackets,
	})
}
