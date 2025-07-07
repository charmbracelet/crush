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

// BetaAssistantService contains methods and other services that help with
// interacting with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewBetaAssistantService] method instead.
type BetaAssistantService struct {
	Options []option.RequestOption
}

// NewBetaAssistantService generates a new service that applies the given options
// to each request. These options are applied after the parent client's options (if
// there is one), and before any request-specific options.
func NewBetaAssistantService(opts ...option.RequestOption) (r BetaAssistantService) {
	r = BetaAssistantService{}
	r.Options = opts
	return
}

// Create an assistant with a model and instructions.
func (r *BetaAssistantService) New(ctx context.Context, body BetaAssistantNewParams, opts ...option.RequestOption) (res *Assistant, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	path := "assistants"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// Retrieves an assistant.
func (r *BetaAssistantService) Get(ctx context.Context, assistantID string, opts ...option.RequestOption) (res *Assistant, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	if assistantID == "" {
		err = errors.New("missing required assistant_id parameter")
		return
	}
	path := fmt.Sprintf("assistants/%s", assistantID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// Modifies an assistant.
func (r *BetaAssistantService) Update(ctx context.Context, assistantID string, body BetaAssistantUpdateParams, opts ...option.RequestOption) (res *Assistant, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	if assistantID == "" {
		err = errors.New("missing required assistant_id parameter")
		return
	}
	path := fmt.Sprintf("assistants/%s", assistantID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// Returns a list of assistants.
func (r *BetaAssistantService) List(ctx context.Context, query BetaAssistantListParams, opts ...option.RequestOption) (res *pagination.CursorPage[Assistant], err error) {
	var raw *http.Response
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2"), option.WithResponseInto(&raw)}, opts...)
	path := "assistants"
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

// Returns a list of assistants.
func (r *BetaAssistantService) ListAutoPaging(ctx context.Context, query BetaAssistantListParams, opts ...option.RequestOption) *pagination.CursorPageAutoPager[Assistant] {
	return pagination.NewCursorPageAutoPager(r.List(ctx, query, opts...))
}

// Delete an assistant.
func (r *BetaAssistantService) Delete(ctx context.Context, assistantID string, opts ...option.RequestOption) (res *AssistantDeleted, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	if assistantID == "" {
		err = errors.New("missing required assistant_id parameter")
		return
	}
	path := fmt.Sprintf("assistants/%s", assistantID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodDelete, path, nil, &res, opts...)
	return
}

// Represents an `assistant` that can call the model and use tools.
type Assistant struct {
	// The identifier, which can be referenced in API endpoints.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) for when the assistant was created.
	CreatedAt int64 `json:"created_at,required"`
	// The description of the assistant. The maximum length is 512 characters.
	Description string `json:"description,required"`
	// The system instructions that the assistant uses. The maximum length is 256,000
	// characters.
	Instructions string `json:"instructions,required"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,required"`
	// ID of the model to use. You can use the
	// [List models](https://platform.openai.com/docs/api-reference/models/list) API to
	// see all of your available models, or see our
	// [Model overview](https://platform.openai.com/docs/models) for descriptions of
	// them.
	Model string `json:"model,required"`
	// The name of the assistant. The maximum length is 256 characters.
	Name string `json:"name,required"`
	// The object type, which is always `assistant`.
	Object constant.Assistant `json:"object,required"`
	// A list of tool enabled on the assistant. There can be a maximum of 128 tools per
	// assistant. Tools can be of types `code_interpreter`, `file_search`, or
	// `function`.
	Tools []AssistantToolUnion `json:"tools,required"`
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
	ResponseFormat AssistantResponseFormatOptionUnion `json:"response_format,nullable"`
	// What sampling temperature to use, between 0 and 2. Higher values like 0.8 will
	// make the output more random, while lower values like 0.2 will make it more
	// focused and deterministic.
	Temperature float64 `json:"temperature,nullable"`
	// A set of resources that are used by the assistant's tools. The resources are
	// specific to the type of tool. For example, the `code_interpreter` tool requires
	// a list of file IDs, while the `file_search` tool requires a list of vector store
	// IDs.
	ToolResources AssistantToolResources `json:"tool_resources,nullable"`
	// An alternative to sampling with temperature, called nucleus sampling, where the
	// model considers the results of the tokens with top_p probability mass. So 0.1
	// means only the tokens comprising the top 10% probability mass are considered.
	//
	// We generally recommend altering this or temperature but not both.
	TopP float64 `json:"top_p,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID             respjson.Field
		CreatedAt      respjson.Field
		Description    respjson.Field
		Instructions   respjson.Field
		Metadata       respjson.Field
		Model          respjson.Field
		Name           respjson.Field
		Object         respjson.Field
		Tools          respjson.Field
		ResponseFormat respjson.Field
		Temperature    respjson.Field
		ToolResources  respjson.Field
		TopP           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r Assistant) RawJSON() string { return r.JSON.raw }
func (r *Assistant) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A set of resources that are used by the assistant's tools. The resources are
// specific to the type of tool. For example, the `code_interpreter` tool requires
// a list of file IDs, while the `file_search` tool requires a list of vector store
// IDs.
type AssistantToolResources struct {
	CodeInterpreter AssistantToolResourcesCodeInterpreter `json:"code_interpreter"`
	FileSearch      AssistantToolResourcesFileSearch      `json:"file_search"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		CodeInterpreter respjson.Field
		FileSearch      respjson.Field
		ExtraFields     map[string]respjson.Field
		raw             string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantToolResources) RawJSON() string { return r.JSON.raw }
func (r *AssistantToolResources) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type AssistantToolResourcesCodeInterpreter struct {
	// A list of [file](https://platform.openai.com/docs/api-reference/files) IDs made
	// available to the `code_interpreter“ tool. There can be a maximum of 20 files
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
func (r AssistantToolResourcesCodeInterpreter) RawJSON() string { return r.JSON.raw }
func (r *AssistantToolResourcesCodeInterpreter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type AssistantToolResourcesFileSearch struct {
	// The ID of the
	// [vector store](https://platform.openai.com/docs/api-reference/vector-stores/object)
	// attached to this assistant. There can be a maximum of 1 vector store attached to
	// the assistant.
	VectorStoreIDs []string `json:"vector_store_ids"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		VectorStoreIDs respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantToolResourcesFileSearch) RawJSON() string { return r.JSON.raw }
func (r *AssistantToolResourcesFileSearch) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type AssistantDeleted struct {
	ID      string                    `json:"id,required"`
	Deleted bool                      `json:"deleted,required"`
	Object  constant.AssistantDeleted `json:"object,required"`
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
func (r AssistantDeleted) RawJSON() string { return r.JSON.raw }
func (r *AssistantDeleted) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// AssistantStreamEventUnion contains all possible properties and values from
// [AssistantStreamEventThreadCreated], [AssistantStreamEventThreadRunCreated],
// [AssistantStreamEventThreadRunQueued],
// [AssistantStreamEventThreadRunInProgress],
// [AssistantStreamEventThreadRunRequiresAction],
// [AssistantStreamEventThreadRunCompleted],
// [AssistantStreamEventThreadRunIncomplete],
// [AssistantStreamEventThreadRunFailed],
// [AssistantStreamEventThreadRunCancelling],
// [AssistantStreamEventThreadRunCancelled],
// [AssistantStreamEventThreadRunExpired],
// [AssistantStreamEventThreadRunStepCreated],
// [AssistantStreamEventThreadRunStepInProgress],
// [AssistantStreamEventThreadRunStepDelta],
// [AssistantStreamEventThreadRunStepCompleted],
// [AssistantStreamEventThreadRunStepFailed],
// [AssistantStreamEventThreadRunStepCancelled],
// [AssistantStreamEventThreadRunStepExpired],
// [AssistantStreamEventThreadMessageCreated],
// [AssistantStreamEventThreadMessageInProgress],
// [AssistantStreamEventThreadMessageDelta],
// [AssistantStreamEventThreadMessageCompleted],
// [AssistantStreamEventThreadMessageIncomplete], [AssistantStreamEventErrorEvent].
//
// Use the [AssistantStreamEventUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type AssistantStreamEventUnion struct {
	// This field is a union of [Thread], [Run], [RunStep], [RunStepDeltaEvent],
	// [Message], [MessageDeltaEvent], [shared.ErrorObject]
	Data AssistantStreamEventUnionData `json:"data"`
	// Any of "thread.created", "thread.run.created", "thread.run.queued",
	// "thread.run.in_progress", "thread.run.requires_action", "thread.run.completed",
	// "thread.run.incomplete", "thread.run.failed", "thread.run.cancelling",
	// "thread.run.cancelled", "thread.run.expired", "thread.run.step.created",
	// "thread.run.step.in_progress", "thread.run.step.delta",
	// "thread.run.step.completed", "thread.run.step.failed",
	// "thread.run.step.cancelled", "thread.run.step.expired",
	// "thread.message.created", "thread.message.in_progress", "thread.message.delta",
	// "thread.message.completed", "thread.message.incomplete", "error".
	Event string `json:"event"`
	// This field is from variant [AssistantStreamEventThreadCreated].
	Enabled bool `json:"enabled"`
	JSON    struct {
		Data    respjson.Field
		Event   respjson.Field
		Enabled respjson.Field
		raw     string
	} `json:"-"`
}

// anyAssistantStreamEvent is implemented by each variant of
// [AssistantStreamEventUnion] to add type safety for the return type of
// [AssistantStreamEventUnion.AsAny]
type anyAssistantStreamEvent interface {
	implAssistantStreamEventUnion()
}

func (AssistantStreamEventThreadCreated) implAssistantStreamEventUnion()           {}
func (AssistantStreamEventThreadRunCreated) implAssistantStreamEventUnion()        {}
func (AssistantStreamEventThreadRunQueued) implAssistantStreamEventUnion()         {}
func (AssistantStreamEventThreadRunInProgress) implAssistantStreamEventUnion()     {}
func (AssistantStreamEventThreadRunRequiresAction) implAssistantStreamEventUnion() {}
func (AssistantStreamEventThreadRunCompleted) implAssistantStreamEventUnion()      {}
func (AssistantStreamEventThreadRunIncomplete) implAssistantStreamEventUnion()     {}
func (AssistantStreamEventThreadRunFailed) implAssistantStreamEventUnion()         {}
func (AssistantStreamEventThreadRunCancelling) implAssistantStreamEventUnion()     {}
func (AssistantStreamEventThreadRunCancelled) implAssistantStreamEventUnion()      {}
func (AssistantStreamEventThreadRunExpired) implAssistantStreamEventUnion()        {}
func (AssistantStreamEventThreadRunStepCreated) implAssistantStreamEventUnion()    {}
func (AssistantStreamEventThreadRunStepInProgress) implAssistantStreamEventUnion() {}
func (AssistantStreamEventThreadRunStepDelta) implAssistantStreamEventUnion()      {}
func (AssistantStreamEventThreadRunStepCompleted) implAssistantStreamEventUnion()  {}
func (AssistantStreamEventThreadRunStepFailed) implAssistantStreamEventUnion()     {}
func (AssistantStreamEventThreadRunStepCancelled) implAssistantStreamEventUnion()  {}
func (AssistantStreamEventThreadRunStepExpired) implAssistantStreamEventUnion()    {}
func (AssistantStreamEventThreadMessageCreated) implAssistantStreamEventUnion()    {}
func (AssistantStreamEventThreadMessageInProgress) implAssistantStreamEventUnion() {}
func (AssistantStreamEventThreadMessageDelta) implAssistantStreamEventUnion()      {}
func (AssistantStreamEventThreadMessageCompleted) implAssistantStreamEventUnion()  {}
func (AssistantStreamEventThreadMessageIncomplete) implAssistantStreamEventUnion() {}
func (AssistantStreamEventErrorEvent) implAssistantStreamEventUnion()              {}

// Use the following switch statement to find the correct variant
//
//	switch variant := AssistantStreamEventUnion.AsAny().(type) {
//	case openai.AssistantStreamEventThreadCreated:
//	case openai.AssistantStreamEventThreadRunCreated:
//	case openai.AssistantStreamEventThreadRunQueued:
//	case openai.AssistantStreamEventThreadRunInProgress:
//	case openai.AssistantStreamEventThreadRunRequiresAction:
//	case openai.AssistantStreamEventThreadRunCompleted:
//	case openai.AssistantStreamEventThreadRunIncomplete:
//	case openai.AssistantStreamEventThreadRunFailed:
//	case openai.AssistantStreamEventThreadRunCancelling:
//	case openai.AssistantStreamEventThreadRunCancelled:
//	case openai.AssistantStreamEventThreadRunExpired:
//	case openai.AssistantStreamEventThreadRunStepCreated:
//	case openai.AssistantStreamEventThreadRunStepInProgress:
//	case openai.AssistantStreamEventThreadRunStepDelta:
//	case openai.AssistantStreamEventThreadRunStepCompleted:
//	case openai.AssistantStreamEventThreadRunStepFailed:
//	case openai.AssistantStreamEventThreadRunStepCancelled:
//	case openai.AssistantStreamEventThreadRunStepExpired:
//	case openai.AssistantStreamEventThreadMessageCreated:
//	case openai.AssistantStreamEventThreadMessageInProgress:
//	case openai.AssistantStreamEventThreadMessageDelta:
//	case openai.AssistantStreamEventThreadMessageCompleted:
//	case openai.AssistantStreamEventThreadMessageIncomplete:
//	case openai.AssistantStreamEventErrorEvent:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u AssistantStreamEventUnion) AsAny() anyAssistantStreamEvent {
	switch u.Event {
	case "thread.created":
		return u.AsThreadCreated()
	case "thread.run.created":
		return u.AsThreadRunCreated()
	case "thread.run.queued":
		return u.AsThreadRunQueued()
	case "thread.run.in_progress":
		return u.AsThreadRunInProgress()
	case "thread.run.requires_action":
		return u.AsThreadRunRequiresAction()
	case "thread.run.completed":
		return u.AsThreadRunCompleted()
	case "thread.run.incomplete":
		return u.AsThreadRunIncomplete()
	case "thread.run.failed":
		return u.AsThreadRunFailed()
	case "thread.run.cancelling":
		return u.AsThreadRunCancelling()
	case "thread.run.cancelled":
		return u.AsThreadRunCancelled()
	case "thread.run.expired":
		return u.AsThreadRunExpired()
	case "thread.run.step.created":
		return u.AsThreadRunStepCreated()
	case "thread.run.step.in_progress":
		return u.AsThreadRunStepInProgress()
	case "thread.run.step.delta":
		return u.AsThreadRunStepDelta()
	case "thread.run.step.completed":
		return u.AsThreadRunStepCompleted()
	case "thread.run.step.failed":
		return u.AsThreadRunStepFailed()
	case "thread.run.step.cancelled":
		return u.AsThreadRunStepCancelled()
	case "thread.run.step.expired":
		return u.AsThreadRunStepExpired()
	case "thread.message.created":
		return u.AsThreadMessageCreated()
	case "thread.message.in_progress":
		return u.AsThreadMessageInProgress()
	case "thread.message.delta":
		return u.AsThreadMessageDelta()
	case "thread.message.completed":
		return u.AsThreadMessageCompleted()
	case "thread.message.incomplete":
		return u.AsThreadMessageIncomplete()
	case "error":
		return u.AsErrorEvent()
	}
	return nil
}

func (u AssistantStreamEventUnion) AsThreadCreated() (v AssistantStreamEventThreadCreated) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadRunCreated() (v AssistantStreamEventThreadRunCreated) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadRunQueued() (v AssistantStreamEventThreadRunQueued) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadRunInProgress() (v AssistantStreamEventThreadRunInProgress) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadRunRequiresAction() (v AssistantStreamEventThreadRunRequiresAction) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadRunCompleted() (v AssistantStreamEventThreadRunCompleted) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadRunIncomplete() (v AssistantStreamEventThreadRunIncomplete) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadRunFailed() (v AssistantStreamEventThreadRunFailed) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadRunCancelling() (v AssistantStreamEventThreadRunCancelling) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadRunCancelled() (v AssistantStreamEventThreadRunCancelled) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadRunExpired() (v AssistantStreamEventThreadRunExpired) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadRunStepCreated() (v AssistantStreamEventThreadRunStepCreated) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadRunStepInProgress() (v AssistantStreamEventThreadRunStepInProgress) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadRunStepDelta() (v AssistantStreamEventThreadRunStepDelta) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadRunStepCompleted() (v AssistantStreamEventThreadRunStepCompleted) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadRunStepFailed() (v AssistantStreamEventThreadRunStepFailed) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadRunStepCancelled() (v AssistantStreamEventThreadRunStepCancelled) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadRunStepExpired() (v AssistantStreamEventThreadRunStepExpired) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadMessageCreated() (v AssistantStreamEventThreadMessageCreated) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadMessageInProgress() (v AssistantStreamEventThreadMessageInProgress) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadMessageDelta() (v AssistantStreamEventThreadMessageDelta) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadMessageCompleted() (v AssistantStreamEventThreadMessageCompleted) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsThreadMessageIncomplete() (v AssistantStreamEventThreadMessageIncomplete) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantStreamEventUnion) AsErrorEvent() (v AssistantStreamEventErrorEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u AssistantStreamEventUnion) RawJSON() string { return u.JSON.raw }

func (r *AssistantStreamEventUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// AssistantStreamEventUnionData is an implicit subunion of
// [AssistantStreamEventUnion]. AssistantStreamEventUnionData provides convenient
// access to the sub-properties of the union.
//
// For type safety it is recommended to directly use a variant of the
// [AssistantStreamEventUnion].
type AssistantStreamEventUnionData struct {
	ID        string `json:"id"`
	CreatedAt int64  `json:"created_at"`
	// This field is from variant [Thread].
	Metadata shared.Metadata `json:"metadata"`
	Object   string          `json:"object"`
	// This field is from variant [Thread].
	ToolResources ThreadToolResources `json:"tool_resources"`
	AssistantID   string              `json:"assistant_id"`
	CancelledAt   int64               `json:"cancelled_at"`
	CompletedAt   int64               `json:"completed_at"`
	// This field is from variant [Run].
	ExpiresAt int64 `json:"expires_at"`
	FailedAt  int64 `json:"failed_at"`
	// This field is a union of [RunIncompleteDetails], [MessageIncompleteDetails]
	IncompleteDetails AssistantStreamEventUnionDataIncompleteDetails `json:"incomplete_details"`
	// This field is from variant [Run].
	Instructions string `json:"instructions"`
	// This field is a union of [RunLastError], [RunStepLastError]
	LastError AssistantStreamEventUnionDataLastError `json:"last_error"`
	// This field is from variant [Run].
	MaxCompletionTokens int64 `json:"max_completion_tokens"`
	// This field is from variant [Run].
	MaxPromptTokens int64 `json:"max_prompt_tokens"`
	// This field is from variant [Run].
	Model string `json:"model"`
	// This field is from variant [Run].
	ParallelToolCalls bool `json:"parallel_tool_calls"`
	// This field is from variant [Run].
	RequiredAction RunRequiredAction `json:"required_action"`
	// This field is from variant [Run].
	ResponseFormat AssistantResponseFormatOptionUnion `json:"response_format"`
	// This field is from variant [Run].
	StartedAt int64  `json:"started_at"`
	Status    string `json:"status"`
	ThreadID  string `json:"thread_id"`
	// This field is from variant [Run].
	ToolChoice AssistantToolChoiceOptionUnion `json:"tool_choice"`
	// This field is from variant [Run].
	Tools []AssistantToolUnion `json:"tools"`
	// This field is from variant [Run].
	TruncationStrategy RunTruncationStrategy `json:"truncation_strategy"`
	// This field is a union of [RunUsage], [RunStepUsage]
	Usage AssistantStreamEventUnionDataUsage `json:"usage"`
	// This field is from variant [Run].
	Temperature float64 `json:"temperature"`
	// This field is from variant [Run].
	TopP float64 `json:"top_p"`
	// This field is from variant [RunStep].
	ExpiredAt int64  `json:"expired_at"`
	RunID     string `json:"run_id"`
	// This field is from variant [RunStep].
	StepDetails RunStepStepDetailsUnion `json:"step_details"`
	Type        string                  `json:"type"`
	// This field is a union of [RunStepDelta], [MessageDelta]
	Delta AssistantStreamEventUnionDataDelta `json:"delta"`
	// This field is from variant [Message].
	Attachments []MessageAttachment `json:"attachments"`
	// This field is from variant [Message].
	Content []MessageContentUnion `json:"content"`
	// This field is from variant [Message].
	IncompleteAt int64 `json:"incomplete_at"`
	// This field is from variant [Message].
	Role MessageRole `json:"role"`
	// This field is from variant [shared.ErrorObject].
	Code string `json:"code"`
	// This field is from variant [shared.ErrorObject].
	Message string `json:"message"`
	// This field is from variant [shared.ErrorObject].
	Param string `json:"param"`
	JSON  struct {
		ID                  respjson.Field
		CreatedAt           respjson.Field
		Metadata            respjson.Field
		Object              respjson.Field
		ToolResources       respjson.Field
		AssistantID         respjson.Field
		CancelledAt         respjson.Field
		CompletedAt         respjson.Field
		ExpiresAt           respjson.Field
		FailedAt            respjson.Field
		IncompleteDetails   respjson.Field
		Instructions        respjson.Field
		LastError           respjson.Field
		MaxCompletionTokens respjson.Field
		MaxPromptTokens     respjson.Field
		Model               respjson.Field
		ParallelToolCalls   respjson.Field
		RequiredAction      respjson.Field
		ResponseFormat      respjson.Field
		StartedAt           respjson.Field
		Status              respjson.Field
		ThreadID            respjson.Field
		ToolChoice          respjson.Field
		Tools               respjson.Field
		TruncationStrategy  respjson.Field
		Usage               respjson.Field
		Temperature         respjson.Field
		TopP                respjson.Field
		ExpiredAt           respjson.Field
		RunID               respjson.Field
		StepDetails         respjson.Field
		Type                respjson.Field
		Delta               respjson.Field
		Attachments         respjson.Field
		Content             respjson.Field
		IncompleteAt        respjson.Field
		Role                respjson.Field
		Code                respjson.Field
		Message             respjson.Field
		Param               respjson.Field
		raw                 string
	} `json:"-"`
}

func (r *AssistantStreamEventUnionData) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// AssistantStreamEventUnionDataIncompleteDetails is an implicit subunion of
// [AssistantStreamEventUnion]. AssistantStreamEventUnionDataIncompleteDetails
// provides convenient access to the sub-properties of the union.
//
// For type safety it is recommended to directly use a variant of the
// [AssistantStreamEventUnion].
type AssistantStreamEventUnionDataIncompleteDetails struct {
	Reason string `json:"reason"`
	JSON   struct {
		Reason respjson.Field
		raw    string
	} `json:"-"`
}

func (r *AssistantStreamEventUnionDataIncompleteDetails) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// AssistantStreamEventUnionDataLastError is an implicit subunion of
// [AssistantStreamEventUnion]. AssistantStreamEventUnionDataLastError provides
// convenient access to the sub-properties of the union.
//
// For type safety it is recommended to directly use a variant of the
// [AssistantStreamEventUnion].
type AssistantStreamEventUnionDataLastError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	JSON    struct {
		Code    respjson.Field
		Message respjson.Field
		raw     string
	} `json:"-"`
}

func (r *AssistantStreamEventUnionDataLastError) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// AssistantStreamEventUnionDataUsage is an implicit subunion of
// [AssistantStreamEventUnion]. AssistantStreamEventUnionDataUsage provides
// convenient access to the sub-properties of the union.
//
// For type safety it is recommended to directly use a variant of the
// [AssistantStreamEventUnion].
type AssistantStreamEventUnionDataUsage struct {
	CompletionTokens int64 `json:"completion_tokens"`
	PromptTokens     int64 `json:"prompt_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
	JSON             struct {
		CompletionTokens respjson.Field
		PromptTokens     respjson.Field
		TotalTokens      respjson.Field
		raw              string
	} `json:"-"`
}

func (r *AssistantStreamEventUnionDataUsage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// AssistantStreamEventUnionDataDelta is an implicit subunion of
// [AssistantStreamEventUnion]. AssistantStreamEventUnionDataDelta provides
// convenient access to the sub-properties of the union.
//
// For type safety it is recommended to directly use a variant of the
// [AssistantStreamEventUnion].
type AssistantStreamEventUnionDataDelta struct {
	// This field is from variant [RunStepDelta].
	StepDetails RunStepDeltaStepDetailsUnion `json:"step_details"`
	// This field is from variant [MessageDelta].
	Content []MessageContentDeltaUnion `json:"content"`
	// This field is from variant [MessageDelta].
	Role MessageDeltaRole `json:"role"`
	JSON struct {
		StepDetails respjson.Field
		Content     respjson.Field
		Role        respjson.Field
		raw         string
	} `json:"-"`
}

func (r *AssistantStreamEventUnionDataDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a new
// [thread](https://platform.openai.com/docs/api-reference/threads/object) is
// created.
type AssistantStreamEventThreadCreated struct {
	// Represents a thread that contains
	// [messages](https://platform.openai.com/docs/api-reference/messages).
	Data  Thread                 `json:"data,required"`
	Event constant.ThreadCreated `json:"event,required"`
	// Whether to enable input audio transcription.
	Enabled bool `json:"enabled"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		Enabled     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadCreated) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadCreated) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a new
// [run](https://platform.openai.com/docs/api-reference/runs/object) is created.
type AssistantStreamEventThreadRunCreated struct {
	// Represents an execution run on a
	// [thread](https://platform.openai.com/docs/api-reference/threads).
	Data  Run                       `json:"data,required"`
	Event constant.ThreadRunCreated `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadRunCreated) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadRunCreated) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a [run](https://platform.openai.com/docs/api-reference/runs/object)
// moves to a `queued` status.
type AssistantStreamEventThreadRunQueued struct {
	// Represents an execution run on a
	// [thread](https://platform.openai.com/docs/api-reference/threads).
	Data  Run                      `json:"data,required"`
	Event constant.ThreadRunQueued `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadRunQueued) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadRunQueued) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a [run](https://platform.openai.com/docs/api-reference/runs/object)
// moves to an `in_progress` status.
type AssistantStreamEventThreadRunInProgress struct {
	// Represents an execution run on a
	// [thread](https://platform.openai.com/docs/api-reference/threads).
	Data  Run                          `json:"data,required"`
	Event constant.ThreadRunInProgress `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadRunInProgress) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadRunInProgress) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a [run](https://platform.openai.com/docs/api-reference/runs/object)
// moves to a `requires_action` status.
type AssistantStreamEventThreadRunRequiresAction struct {
	// Represents an execution run on a
	// [thread](https://platform.openai.com/docs/api-reference/threads).
	Data  Run                              `json:"data,required"`
	Event constant.ThreadRunRequiresAction `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadRunRequiresAction) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadRunRequiresAction) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a [run](https://platform.openai.com/docs/api-reference/runs/object)
// is completed.
type AssistantStreamEventThreadRunCompleted struct {
	// Represents an execution run on a
	// [thread](https://platform.openai.com/docs/api-reference/threads).
	Data  Run                         `json:"data,required"`
	Event constant.ThreadRunCompleted `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadRunCompleted) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadRunCompleted) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a [run](https://platform.openai.com/docs/api-reference/runs/object)
// ends with status `incomplete`.
type AssistantStreamEventThreadRunIncomplete struct {
	// Represents an execution run on a
	// [thread](https://platform.openai.com/docs/api-reference/threads).
	Data  Run                          `json:"data,required"`
	Event constant.ThreadRunIncomplete `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadRunIncomplete) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadRunIncomplete) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a [run](https://platform.openai.com/docs/api-reference/runs/object)
// fails.
type AssistantStreamEventThreadRunFailed struct {
	// Represents an execution run on a
	// [thread](https://platform.openai.com/docs/api-reference/threads).
	Data  Run                      `json:"data,required"`
	Event constant.ThreadRunFailed `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadRunFailed) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadRunFailed) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a [run](https://platform.openai.com/docs/api-reference/runs/object)
// moves to a `cancelling` status.
type AssistantStreamEventThreadRunCancelling struct {
	// Represents an execution run on a
	// [thread](https://platform.openai.com/docs/api-reference/threads).
	Data  Run                          `json:"data,required"`
	Event constant.ThreadRunCancelling `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadRunCancelling) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadRunCancelling) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a [run](https://platform.openai.com/docs/api-reference/runs/object)
// is cancelled.
type AssistantStreamEventThreadRunCancelled struct {
	// Represents an execution run on a
	// [thread](https://platform.openai.com/docs/api-reference/threads).
	Data  Run                         `json:"data,required"`
	Event constant.ThreadRunCancelled `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadRunCancelled) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadRunCancelled) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a [run](https://platform.openai.com/docs/api-reference/runs/object)
// expires.
type AssistantStreamEventThreadRunExpired struct {
	// Represents an execution run on a
	// [thread](https://platform.openai.com/docs/api-reference/threads).
	Data  Run                       `json:"data,required"`
	Event constant.ThreadRunExpired `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadRunExpired) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadRunExpired) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a
// [run step](https://platform.openai.com/docs/api-reference/run-steps/step-object)
// is created.
type AssistantStreamEventThreadRunStepCreated struct {
	// Represents a step in execution of a run.
	Data  RunStep                       `json:"data,required"`
	Event constant.ThreadRunStepCreated `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadRunStepCreated) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadRunStepCreated) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a
// [run step](https://platform.openai.com/docs/api-reference/run-steps/step-object)
// moves to an `in_progress` state.
type AssistantStreamEventThreadRunStepInProgress struct {
	// Represents a step in execution of a run.
	Data  RunStep                          `json:"data,required"`
	Event constant.ThreadRunStepInProgress `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadRunStepInProgress) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadRunStepInProgress) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when parts of a
// [run step](https://platform.openai.com/docs/api-reference/run-steps/step-object)
// are being streamed.
type AssistantStreamEventThreadRunStepDelta struct {
	// Represents a run step delta i.e. any changed fields on a run step during
	// streaming.
	Data  RunStepDeltaEvent           `json:"data,required"`
	Event constant.ThreadRunStepDelta `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadRunStepDelta) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadRunStepDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a
// [run step](https://platform.openai.com/docs/api-reference/run-steps/step-object)
// is completed.
type AssistantStreamEventThreadRunStepCompleted struct {
	// Represents a step in execution of a run.
	Data  RunStep                         `json:"data,required"`
	Event constant.ThreadRunStepCompleted `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadRunStepCompleted) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadRunStepCompleted) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a
// [run step](https://platform.openai.com/docs/api-reference/run-steps/step-object)
// fails.
type AssistantStreamEventThreadRunStepFailed struct {
	// Represents a step in execution of a run.
	Data  RunStep                      `json:"data,required"`
	Event constant.ThreadRunStepFailed `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadRunStepFailed) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadRunStepFailed) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a
// [run step](https://platform.openai.com/docs/api-reference/run-steps/step-object)
// is cancelled.
type AssistantStreamEventThreadRunStepCancelled struct {
	// Represents a step in execution of a run.
	Data  RunStep                         `json:"data,required"`
	Event constant.ThreadRunStepCancelled `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadRunStepCancelled) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadRunStepCancelled) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a
// [run step](https://platform.openai.com/docs/api-reference/run-steps/step-object)
// expires.
type AssistantStreamEventThreadRunStepExpired struct {
	// Represents a step in execution of a run.
	Data  RunStep                       `json:"data,required"`
	Event constant.ThreadRunStepExpired `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadRunStepExpired) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadRunStepExpired) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a
// [message](https://platform.openai.com/docs/api-reference/messages/object) is
// created.
type AssistantStreamEventThreadMessageCreated struct {
	// Represents a message within a
	// [thread](https://platform.openai.com/docs/api-reference/threads).
	Data  Message                       `json:"data,required"`
	Event constant.ThreadMessageCreated `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadMessageCreated) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadMessageCreated) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a
// [message](https://platform.openai.com/docs/api-reference/messages/object) moves
// to an `in_progress` state.
type AssistantStreamEventThreadMessageInProgress struct {
	// Represents a message within a
	// [thread](https://platform.openai.com/docs/api-reference/threads).
	Data  Message                          `json:"data,required"`
	Event constant.ThreadMessageInProgress `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadMessageInProgress) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadMessageInProgress) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when parts of a
// [Message](https://platform.openai.com/docs/api-reference/messages/object) are
// being streamed.
type AssistantStreamEventThreadMessageDelta struct {
	// Represents a message delta i.e. any changed fields on a message during
	// streaming.
	Data  MessageDeltaEvent           `json:"data,required"`
	Event constant.ThreadMessageDelta `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadMessageDelta) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadMessageDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a
// [message](https://platform.openai.com/docs/api-reference/messages/object) is
// completed.
type AssistantStreamEventThreadMessageCompleted struct {
	// Represents a message within a
	// [thread](https://platform.openai.com/docs/api-reference/threads).
	Data  Message                         `json:"data,required"`
	Event constant.ThreadMessageCompleted `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadMessageCompleted) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadMessageCompleted) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when a
// [message](https://platform.openai.com/docs/api-reference/messages/object) ends
// before it is completed.
type AssistantStreamEventThreadMessageIncomplete struct {
	// Represents a message within a
	// [thread](https://platform.openai.com/docs/api-reference/threads).
	Data  Message                          `json:"data,required"`
	Event constant.ThreadMessageIncomplete `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventThreadMessageIncomplete) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventThreadMessageIncomplete) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Occurs when an
// [error](https://platform.openai.com/docs/guides/error-codes#api-errors) occurs.
// This can happen due to an internal server error or a timeout.
type AssistantStreamEventErrorEvent struct {
	Data  shared.ErrorObject `json:"data,required"`
	Event constant.Error     `json:"event,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Event       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r AssistantStreamEventErrorEvent) RawJSON() string { return r.JSON.raw }
func (r *AssistantStreamEventErrorEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// AssistantToolUnion contains all possible properties and values from
// [CodeInterpreterTool], [FileSearchTool], [FunctionTool].
//
// Use the [AssistantToolUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type AssistantToolUnion struct {
	// Any of "code_interpreter", "file_search", "function".
	Type string `json:"type"`
	// This field is from variant [FileSearchTool].
	FileSearch FileSearchToolFileSearch `json:"file_search"`
	// This field is from variant [FunctionTool].
	Function shared.FunctionDefinition `json:"function"`
	JSON     struct {
		Type       respjson.Field
		FileSearch respjson.Field
		Function   respjson.Field
		raw        string
	} `json:"-"`
}

// anyAssistantTool is implemented by each variant of [AssistantToolUnion] to add
// type safety for the return type of [AssistantToolUnion.AsAny]
type anyAssistantTool interface {
	implAssistantToolUnion()
}

func (CodeInterpreterTool) implAssistantToolUnion() {}
func (FileSearchTool) implAssistantToolUnion()      {}
func (FunctionTool) implAssistantToolUnion()        {}

// Use the following switch statement to find the correct variant
//
//	switch variant := AssistantToolUnion.AsAny().(type) {
//	case openai.CodeInterpreterTool:
//	case openai.FileSearchTool:
//	case openai.FunctionTool:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u AssistantToolUnion) AsAny() anyAssistantTool {
	switch u.Type {
	case "code_interpreter":
		return u.AsCodeInterpreter()
	case "file_search":
		return u.AsFileSearch()
	case "function":
		return u.AsFunction()
	}
	return nil
}

func (u AssistantToolUnion) AsCodeInterpreter() (v CodeInterpreterTool) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantToolUnion) AsFileSearch() (v FileSearchTool) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AssistantToolUnion) AsFunction() (v FunctionTool) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u AssistantToolUnion) RawJSON() string { return u.JSON.raw }

func (r *AssistantToolUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this AssistantToolUnion to a AssistantToolUnionParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// AssistantToolUnionParam.Overrides()
func (r AssistantToolUnion) ToParam() AssistantToolUnionParam {
	return param.Override[AssistantToolUnionParam](json.RawMessage(r.RawJSON()))
}

func AssistantToolParamOfFunction(function shared.FunctionDefinitionParam) AssistantToolUnionParam {
	var variant FunctionToolParam
	variant.Function = function
	return AssistantToolUnionParam{OfFunction: &variant}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type AssistantToolUnionParam struct {
	OfCodeInterpreter *CodeInterpreterToolParam `json:",omitzero,inline"`
	OfFileSearch      *FileSearchToolParam      `json:",omitzero,inline"`
	OfFunction        *FunctionToolParam        `json:",omitzero,inline"`
	paramUnion
}

func (u AssistantToolUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfCodeInterpreter, u.OfFileSearch, u.OfFunction)
}
func (u *AssistantToolUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *AssistantToolUnionParam) asAny() any {
	if !param.IsOmitted(u.OfCodeInterpreter) {
		return u.OfCodeInterpreter
	} else if !param.IsOmitted(u.OfFileSearch) {
		return u.OfFileSearch
	} else if !param.IsOmitted(u.OfFunction) {
		return u.OfFunction
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u AssistantToolUnionParam) GetFileSearch() *FileSearchToolFileSearchParam {
	if vt := u.OfFileSearch; vt != nil {
		return &vt.FileSearch
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u AssistantToolUnionParam) GetFunction() *shared.FunctionDefinitionParam {
	if vt := u.OfFunction; vt != nil {
		return &vt.Function
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u AssistantToolUnionParam) GetType() *string {
	if vt := u.OfCodeInterpreter; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfFileSearch; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfFunction; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[AssistantToolUnionParam](
		"type",
		apijson.Discriminator[CodeInterpreterToolParam]("code_interpreter"),
		apijson.Discriminator[FileSearchToolParam]("file_search"),
		apijson.Discriminator[FunctionToolParam]("function"),
	)
}

type CodeInterpreterTool struct {
	// The type of tool being defined: `code_interpreter`
	Type constant.CodeInterpreter `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r CodeInterpreterTool) RawJSON() string { return r.JSON.raw }
func (r *CodeInterpreterTool) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this CodeInterpreterTool to a CodeInterpreterToolParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// CodeInterpreterToolParam.Overrides()
func (r CodeInterpreterTool) ToParam() CodeInterpreterToolParam {
	return param.Override[CodeInterpreterToolParam](json.RawMessage(r.RawJSON()))
}

func NewCodeInterpreterToolParam() CodeInterpreterToolParam {
	return CodeInterpreterToolParam{
		Type: "code_interpreter",
	}
}

// This struct has a constant value, construct it with
// [NewCodeInterpreterToolParam].
type CodeInterpreterToolParam struct {
	// The type of tool being defined: `code_interpreter`
	Type constant.CodeInterpreter `json:"type,required"`
	paramObj
}

func (r CodeInterpreterToolParam) MarshalJSON() (data []byte, err error) {
	type shadow CodeInterpreterToolParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *CodeInterpreterToolParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type FileSearchTool struct {
	// The type of tool being defined: `file_search`
	Type constant.FileSearch `json:"type,required"`
	// Overrides for the file search tool.
	FileSearch FileSearchToolFileSearch `json:"file_search"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		FileSearch  respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
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

// Overrides for the file search tool.
type FileSearchToolFileSearch struct {
	// The maximum number of results the file search tool should output. The default is
	// 20 for `gpt-4*` models and 5 for `gpt-3.5-turbo`. This number should be between
	// 1 and 50 inclusive.
	//
	// Note that the file search tool may output fewer than `max_num_results` results.
	// See the
	// [file search tool documentation](https://platform.openai.com/docs/assistants/tools/file-search#customizing-file-search-settings)
	// for more information.
	MaxNumResults int64 `json:"max_num_results"`
	// The ranking options for the file search. If not specified, the file search tool
	// will use the `auto` ranker and a score_threshold of 0.
	//
	// See the
	// [file search tool documentation](https://platform.openai.com/docs/assistants/tools/file-search#customizing-file-search-settings)
	// for more information.
	RankingOptions FileSearchToolFileSearchRankingOptions `json:"ranking_options"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		MaxNumResults  respjson.Field
		RankingOptions respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FileSearchToolFileSearch) RawJSON() string { return r.JSON.raw }
func (r *FileSearchToolFileSearch) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The ranking options for the file search. If not specified, the file search tool
// will use the `auto` ranker and a score_threshold of 0.
//
// See the
// [file search tool documentation](https://platform.openai.com/docs/assistants/tools/file-search#customizing-file-search-settings)
// for more information.
type FileSearchToolFileSearchRankingOptions struct {
	// The score threshold for the file search. All values must be a floating point
	// number between 0 and 1.
	ScoreThreshold float64 `json:"score_threshold,required"`
	// The ranker to use for the file search. If not specified will use the `auto`
	// ranker.
	//
	// Any of "auto", "default_2024_08_21".
	Ranker string `json:"ranker"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ScoreThreshold respjson.Field
		Ranker         respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FileSearchToolFileSearchRankingOptions) RawJSON() string { return r.JSON.raw }
func (r *FileSearchToolFileSearchRankingOptions) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The property Type is required.
type FileSearchToolParam struct {
	// Overrides for the file search tool.
	FileSearch FileSearchToolFileSearchParam `json:"file_search,omitzero"`
	// The type of tool being defined: `file_search`
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

// Overrides for the file search tool.
type FileSearchToolFileSearchParam struct {
	// The maximum number of results the file search tool should output. The default is
	// 20 for `gpt-4*` models and 5 for `gpt-3.5-turbo`. This number should be between
	// 1 and 50 inclusive.
	//
	// Note that the file search tool may output fewer than `max_num_results` results.
	// See the
	// [file search tool documentation](https://platform.openai.com/docs/assistants/tools/file-search#customizing-file-search-settings)
	// for more information.
	MaxNumResults param.Opt[int64] `json:"max_num_results,omitzero"`
	// The ranking options for the file search. If not specified, the file search tool
	// will use the `auto` ranker and a score_threshold of 0.
	//
	// See the
	// [file search tool documentation](https://platform.openai.com/docs/assistants/tools/file-search#customizing-file-search-settings)
	// for more information.
	RankingOptions FileSearchToolFileSearchRankingOptionsParam `json:"ranking_options,omitzero"`
	paramObj
}

func (r FileSearchToolFileSearchParam) MarshalJSON() (data []byte, err error) {
	type shadow FileSearchToolFileSearchParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *FileSearchToolFileSearchParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The ranking options for the file search. If not specified, the file search tool
// will use the `auto` ranker and a score_threshold of 0.
//
// See the
// [file search tool documentation](https://platform.openai.com/docs/assistants/tools/file-search#customizing-file-search-settings)
// for more information.
//
// The property ScoreThreshold is required.
type FileSearchToolFileSearchRankingOptionsParam struct {
	// The score threshold for the file search. All values must be a floating point
	// number between 0 and 1.
	ScoreThreshold float64 `json:"score_threshold,required"`
	// The ranker to use for the file search. If not specified will use the `auto`
	// ranker.
	//
	// Any of "auto", "default_2024_08_21".
	Ranker string `json:"ranker,omitzero"`
	paramObj
}

func (r FileSearchToolFileSearchRankingOptionsParam) MarshalJSON() (data []byte, err error) {
	type shadow FileSearchToolFileSearchRankingOptionsParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *FileSearchToolFileSearchRankingOptionsParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[FileSearchToolFileSearchRankingOptionsParam](
		"ranker", "auto", "default_2024_08_21",
	)
}

type FunctionTool struct {
	Function shared.FunctionDefinition `json:"function,required"`
	// The type of tool being defined: `function`
	Type constant.Function `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Function    respjson.Field
		Type        respjson.Field
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

// The properties Function, Type are required.
type FunctionToolParam struct {
	Function shared.FunctionDefinitionParam `json:"function,omitzero,required"`
	// The type of tool being defined: `function`
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

type BetaAssistantNewParams struct {
	// ID of the model to use. You can use the
	// [List models](https://platform.openai.com/docs/api-reference/models/list) API to
	// see all of your available models, or see our
	// [Model overview](https://platform.openai.com/docs/models) for descriptions of
	// them.
	Model shared.ChatModel `json:"model,omitzero,required"`
	// The description of the assistant. The maximum length is 512 characters.
	Description param.Opt[string] `json:"description,omitzero"`
	// The system instructions that the assistant uses. The maximum length is 256,000
	// characters.
	Instructions param.Opt[string] `json:"instructions,omitzero"`
	// The name of the assistant. The maximum length is 256 characters.
	Name param.Opt[string] `json:"name,omitzero"`
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
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,omitzero"`
	// **o-series models only**
	//
	// Constrains effort on reasoning for
	// [reasoning models](https://platform.openai.com/docs/guides/reasoning). Currently
	// supported values are `low`, `medium`, and `high`. Reducing reasoning effort can
	// result in faster responses and fewer tokens used on reasoning in a response.
	//
	// Any of "low", "medium", "high".
	ReasoningEffort shared.ReasoningEffort `json:"reasoning_effort,omitzero"`
	// A set of resources that are used by the assistant's tools. The resources are
	// specific to the type of tool. For example, the `code_interpreter` tool requires
	// a list of file IDs, while the `file_search` tool requires a list of vector store
	// IDs.
	ToolResources BetaAssistantNewParamsToolResources `json:"tool_resources,omitzero"`
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
	// A list of tool enabled on the assistant. There can be a maximum of 128 tools per
	// assistant. Tools can be of types `code_interpreter`, `file_search`, or
	// `function`.
	Tools []AssistantToolUnionParam `json:"tools,omitzero"`
	paramObj
}

func (r BetaAssistantNewParams) MarshalJSON() (data []byte, err error) {
	type shadow BetaAssistantNewParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaAssistantNewParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A set of resources that are used by the assistant's tools. The resources are
// specific to the type of tool. For example, the `code_interpreter` tool requires
// a list of file IDs, while the `file_search` tool requires a list of vector store
// IDs.
type BetaAssistantNewParamsToolResources struct {
	CodeInterpreter BetaAssistantNewParamsToolResourcesCodeInterpreter `json:"code_interpreter,omitzero"`
	FileSearch      BetaAssistantNewParamsToolResourcesFileSearch      `json:"file_search,omitzero"`
	paramObj
}

func (r BetaAssistantNewParamsToolResources) MarshalJSON() (data []byte, err error) {
	type shadow BetaAssistantNewParamsToolResources
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaAssistantNewParamsToolResources) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaAssistantNewParamsToolResourcesCodeInterpreter struct {
	// A list of [file](https://platform.openai.com/docs/api-reference/files) IDs made
	// available to the `code_interpreter` tool. There can be a maximum of 20 files
	// associated with the tool.
	FileIDs []string `json:"file_ids,omitzero"`
	paramObj
}

func (r BetaAssistantNewParamsToolResourcesCodeInterpreter) MarshalJSON() (data []byte, err error) {
	type shadow BetaAssistantNewParamsToolResourcesCodeInterpreter
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaAssistantNewParamsToolResourcesCodeInterpreter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaAssistantNewParamsToolResourcesFileSearch struct {
	// The
	// [vector store](https://platform.openai.com/docs/api-reference/vector-stores/object)
	// attached to this assistant. There can be a maximum of 1 vector store attached to
	// the assistant.
	VectorStoreIDs []string `json:"vector_store_ids,omitzero"`
	// A helper to create a
	// [vector store](https://platform.openai.com/docs/api-reference/vector-stores/object)
	// with file_ids and attach it to this assistant. There can be a maximum of 1
	// vector store attached to the assistant.
	VectorStores []BetaAssistantNewParamsToolResourcesFileSearchVectorStore `json:"vector_stores,omitzero"`
	paramObj
}

func (r BetaAssistantNewParamsToolResourcesFileSearch) MarshalJSON() (data []byte, err error) {
	type shadow BetaAssistantNewParamsToolResourcesFileSearch
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaAssistantNewParamsToolResourcesFileSearch) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaAssistantNewParamsToolResourcesFileSearchVectorStore struct {
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,omitzero"`
	// The chunking strategy used to chunk the file(s). If not set, will use the `auto`
	// strategy.
	ChunkingStrategy BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyUnion `json:"chunking_strategy,omitzero"`
	// A list of [file](https://platform.openai.com/docs/api-reference/files) IDs to
	// add to the vector store. There can be a maximum of 10000 files in a vector
	// store.
	FileIDs []string `json:"file_ids,omitzero"`
	paramObj
}

func (r BetaAssistantNewParamsToolResourcesFileSearchVectorStore) MarshalJSON() (data []byte, err error) {
	type shadow BetaAssistantNewParamsToolResourcesFileSearchVectorStore
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaAssistantNewParamsToolResourcesFileSearchVectorStore) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyUnion struct {
	OfAuto   *BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto   `json:",omitzero,inline"`
	OfStatic *BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStatic `json:",omitzero,inline"`
	paramUnion
}

func (u BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfStatic)
}
func (u *BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyUnion) asAny() any {
	if !param.IsOmitted(u.OfAuto) {
		return u.OfAuto
	} else if !param.IsOmitted(u.OfStatic) {
		return u.OfStatic
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyUnion) GetStatic() *BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStaticStatic {
	if vt := u.OfStatic; vt != nil {
		return &vt.Static
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyUnion) GetType() *string {
	if vt := u.OfAuto; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfStatic; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyUnion](
		"type",
		apijson.Discriminator[BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto]("auto"),
		apijson.Discriminator[BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStatic]("static"),
	)
}

func NewBetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto() BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto {
	return BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto{
		Type: "auto",
	}
}

// The default strategy. This strategy currently uses a `max_chunk_size_tokens` of
// `800` and `chunk_overlap_tokens` of `400`.
//
// This struct has a constant value, construct it with
// [NewBetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto].
type BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto struct {
	// Always `auto`.
	Type constant.Auto `json:"type,required"`
	paramObj
}

func (r BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto) MarshalJSON() (data []byte, err error) {
	type shadow BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyAuto) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Static, Type are required.
type BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStatic struct {
	Static BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStaticStatic `json:"static,omitzero,required"`
	// Always `static`.
	//
	// This field can be elided, and will marshal its zero value as "static".
	Type constant.Static `json:"type,required"`
	paramObj
}

func (r BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStatic) MarshalJSON() (data []byte, err error) {
	type shadow BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStatic
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStatic) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties ChunkOverlapTokens, MaxChunkSizeTokens are required.
type BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStaticStatic struct {
	// The number of tokens that overlap between chunks. The default value is `400`.
	//
	// Note that the overlap must not exceed half of `max_chunk_size_tokens`.
	ChunkOverlapTokens int64 `json:"chunk_overlap_tokens,required"`
	// The maximum number of tokens in each chunk. The default value is `800`. The
	// minimum value is `100` and the maximum value is `4096`.
	MaxChunkSizeTokens int64 `json:"max_chunk_size_tokens,required"`
	paramObj
}

func (r BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStaticStatic) MarshalJSON() (data []byte, err error) {
	type shadow BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStaticStatic
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaAssistantNewParamsToolResourcesFileSearchVectorStoreChunkingStrategyStaticStatic) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaAssistantUpdateParams struct {
	// The description of the assistant. The maximum length is 512 characters.
	Description param.Opt[string] `json:"description,omitzero"`
	// The system instructions that the assistant uses. The maximum length is 256,000
	// characters.
	Instructions param.Opt[string] `json:"instructions,omitzero"`
	// The name of the assistant. The maximum length is 256 characters.
	Name param.Opt[string] `json:"name,omitzero"`
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
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,omitzero"`
	// **o-series models only**
	//
	// Constrains effort on reasoning for
	// [reasoning models](https://platform.openai.com/docs/guides/reasoning). Currently
	// supported values are `low`, `medium`, and `high`. Reducing reasoning effort can
	// result in faster responses and fewer tokens used on reasoning in a response.
	//
	// Any of "low", "medium", "high".
	ReasoningEffort shared.ReasoningEffort `json:"reasoning_effort,omitzero"`
	// A set of resources that are used by the assistant's tools. The resources are
	// specific to the type of tool. For example, the `code_interpreter` tool requires
	// a list of file IDs, while the `file_search` tool requires a list of vector store
	// IDs.
	ToolResources BetaAssistantUpdateParamsToolResources `json:"tool_resources,omitzero"`
	// ID of the model to use. You can use the
	// [List models](https://platform.openai.com/docs/api-reference/models/list) API to
	// see all of your available models, or see our
	// [Model overview](https://platform.openai.com/docs/models) for descriptions of
	// them.
	Model BetaAssistantUpdateParamsModel `json:"model,omitzero"`
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
	// A list of tool enabled on the assistant. There can be a maximum of 128 tools per
	// assistant. Tools can be of types `code_interpreter`, `file_search`, or
	// `function`.
	Tools []AssistantToolUnionParam `json:"tools,omitzero"`
	paramObj
}

func (r BetaAssistantUpdateParams) MarshalJSON() (data []byte, err error) {
	type shadow BetaAssistantUpdateParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaAssistantUpdateParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ID of the model to use. You can use the
// [List models](https://platform.openai.com/docs/api-reference/models/list) API to
// see all of your available models, or see our
// [Model overview](https://platform.openai.com/docs/models) for descriptions of
// them.
type BetaAssistantUpdateParamsModel string

const (
	BetaAssistantUpdateParamsModelGPT4_1                  BetaAssistantUpdateParamsModel = "gpt-4.1"
	BetaAssistantUpdateParamsModelGPT4_1Mini              BetaAssistantUpdateParamsModel = "gpt-4.1-mini"
	BetaAssistantUpdateParamsModelGPT4_1Nano              BetaAssistantUpdateParamsModel = "gpt-4.1-nano"
	BetaAssistantUpdateParamsModelGPT4_1_2025_04_14       BetaAssistantUpdateParamsModel = "gpt-4.1-2025-04-14"
	BetaAssistantUpdateParamsModelGPT4_1Mini2025_04_14    BetaAssistantUpdateParamsModel = "gpt-4.1-mini-2025-04-14"
	BetaAssistantUpdateParamsModelGPT4_1Nano2025_04_14    BetaAssistantUpdateParamsModel = "gpt-4.1-nano-2025-04-14"
	BetaAssistantUpdateParamsModelO3Mini                  BetaAssistantUpdateParamsModel = "o3-mini"
	BetaAssistantUpdateParamsModelO3Mini2025_01_31        BetaAssistantUpdateParamsModel = "o3-mini-2025-01-31"
	BetaAssistantUpdateParamsModelO1                      BetaAssistantUpdateParamsModel = "o1"
	BetaAssistantUpdateParamsModelO1_2024_12_17           BetaAssistantUpdateParamsModel = "o1-2024-12-17"
	BetaAssistantUpdateParamsModelGPT4o                   BetaAssistantUpdateParamsModel = "gpt-4o"
	BetaAssistantUpdateParamsModelGPT4o2024_11_20         BetaAssistantUpdateParamsModel = "gpt-4o-2024-11-20"
	BetaAssistantUpdateParamsModelGPT4o2024_08_06         BetaAssistantUpdateParamsModel = "gpt-4o-2024-08-06"
	BetaAssistantUpdateParamsModelGPT4o2024_05_13         BetaAssistantUpdateParamsModel = "gpt-4o-2024-05-13"
	BetaAssistantUpdateParamsModelGPT4oMini               BetaAssistantUpdateParamsModel = "gpt-4o-mini"
	BetaAssistantUpdateParamsModelGPT4oMini2024_07_18     BetaAssistantUpdateParamsModel = "gpt-4o-mini-2024-07-18"
	BetaAssistantUpdateParamsModelGPT4_5Preview           BetaAssistantUpdateParamsModel = "gpt-4.5-preview"
	BetaAssistantUpdateParamsModelGPT4_5Preview2025_02_27 BetaAssistantUpdateParamsModel = "gpt-4.5-preview-2025-02-27"
	BetaAssistantUpdateParamsModelGPT4Turbo               BetaAssistantUpdateParamsModel = "gpt-4-turbo"
	BetaAssistantUpdateParamsModelGPT4Turbo2024_04_09     BetaAssistantUpdateParamsModel = "gpt-4-turbo-2024-04-09"
	BetaAssistantUpdateParamsModelGPT4_0125Preview        BetaAssistantUpdateParamsModel = "gpt-4-0125-preview"
	BetaAssistantUpdateParamsModelGPT4TurboPreview        BetaAssistantUpdateParamsModel = "gpt-4-turbo-preview"
	BetaAssistantUpdateParamsModelGPT4_1106Preview        BetaAssistantUpdateParamsModel = "gpt-4-1106-preview"
	BetaAssistantUpdateParamsModelGPT4VisionPreview       BetaAssistantUpdateParamsModel = "gpt-4-vision-preview"
	BetaAssistantUpdateParamsModelGPT4                    BetaAssistantUpdateParamsModel = "gpt-4"
	BetaAssistantUpdateParamsModelGPT4_0314               BetaAssistantUpdateParamsModel = "gpt-4-0314"
	BetaAssistantUpdateParamsModelGPT4_0613               BetaAssistantUpdateParamsModel = "gpt-4-0613"
	BetaAssistantUpdateParamsModelGPT4_32k                BetaAssistantUpdateParamsModel = "gpt-4-32k"
	BetaAssistantUpdateParamsModelGPT4_32k0314            BetaAssistantUpdateParamsModel = "gpt-4-32k-0314"
	BetaAssistantUpdateParamsModelGPT4_32k0613            BetaAssistantUpdateParamsModel = "gpt-4-32k-0613"
	BetaAssistantUpdateParamsModelGPT3_5Turbo             BetaAssistantUpdateParamsModel = "gpt-3.5-turbo"
	BetaAssistantUpdateParamsModelGPT3_5Turbo16k          BetaAssistantUpdateParamsModel = "gpt-3.5-turbo-16k"
	BetaAssistantUpdateParamsModelGPT3_5Turbo0613         BetaAssistantUpdateParamsModel = "gpt-3.5-turbo-0613"
	BetaAssistantUpdateParamsModelGPT3_5Turbo1106         BetaAssistantUpdateParamsModel = "gpt-3.5-turbo-1106"
	BetaAssistantUpdateParamsModelGPT3_5Turbo0125         BetaAssistantUpdateParamsModel = "gpt-3.5-turbo-0125"
	BetaAssistantUpdateParamsModelGPT3_5Turbo16k0613      BetaAssistantUpdateParamsModel = "gpt-3.5-turbo-16k-0613"
)

// A set of resources that are used by the assistant's tools. The resources are
// specific to the type of tool. For example, the `code_interpreter` tool requires
// a list of file IDs, while the `file_search` tool requires a list of vector store
// IDs.
type BetaAssistantUpdateParamsToolResources struct {
	CodeInterpreter BetaAssistantUpdateParamsToolResourcesCodeInterpreter `json:"code_interpreter,omitzero"`
	FileSearch      BetaAssistantUpdateParamsToolResourcesFileSearch      `json:"file_search,omitzero"`
	paramObj
}

func (r BetaAssistantUpdateParamsToolResources) MarshalJSON() (data []byte, err error) {
	type shadow BetaAssistantUpdateParamsToolResources
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaAssistantUpdateParamsToolResources) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaAssistantUpdateParamsToolResourcesCodeInterpreter struct {
	// Overrides the list of
	// [file](https://platform.openai.com/docs/api-reference/files) IDs made available
	// to the `code_interpreter` tool. There can be a maximum of 20 files associated
	// with the tool.
	FileIDs []string `json:"file_ids,omitzero"`
	paramObj
}

func (r BetaAssistantUpdateParamsToolResourcesCodeInterpreter) MarshalJSON() (data []byte, err error) {
	type shadow BetaAssistantUpdateParamsToolResourcesCodeInterpreter
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaAssistantUpdateParamsToolResourcesCodeInterpreter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaAssistantUpdateParamsToolResourcesFileSearch struct {
	// Overrides the
	// [vector store](https://platform.openai.com/docs/api-reference/vector-stores/object)
	// attached to this assistant. There can be a maximum of 1 vector store attached to
	// the assistant.
	VectorStoreIDs []string `json:"vector_store_ids,omitzero"`
	paramObj
}

func (r BetaAssistantUpdateParamsToolResourcesFileSearch) MarshalJSON() (data []byte, err error) {
	type shadow BetaAssistantUpdateParamsToolResourcesFileSearch
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaAssistantUpdateParamsToolResourcesFileSearch) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaAssistantListParams struct {
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
	Order BetaAssistantListParamsOrder `query:"order,omitzero" json:"-"`
	paramObj
}

// URLQuery serializes [BetaAssistantListParams]'s query parameters as
// `url.Values`.
func (r BetaAssistantListParams) URLQuery() (v url.Values, err error) {
	return apiquery.MarshalWithSettings(r, apiquery.QuerySettings{
		ArrayFormat:  apiquery.ArrayQueryFormatBrackets,
		NestedFormat: apiquery.NestedQueryFormatBrackets,
	})
}

// Sort order by the `created_at` timestamp of the objects. `asc` for ascending
// order and `desc` for descending order.
type BetaAssistantListParamsOrder string

const (
	BetaAssistantListParamsOrderAsc  BetaAssistantListParamsOrder = "asc"
	BetaAssistantListParamsOrderDesc BetaAssistantListParamsOrder = "desc"
)
