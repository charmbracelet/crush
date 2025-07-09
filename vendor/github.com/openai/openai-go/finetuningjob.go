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

// FineTuningJobService contains methods and other services that help with
// interacting with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewFineTuningJobService] method instead.
type FineTuningJobService struct {
	Options     []option.RequestOption
	Checkpoints FineTuningJobCheckpointService
}

// NewFineTuningJobService generates a new service that applies the given options
// to each request. These options are applied after the parent client's options (if
// there is one), and before any request-specific options.
func NewFineTuningJobService(opts ...option.RequestOption) (r FineTuningJobService) {
	r = FineTuningJobService{}
	r.Options = opts
	r.Checkpoints = NewFineTuningJobCheckpointService(opts...)
	return
}

// Creates a fine-tuning job which begins the process of creating a new model from
// a given dataset.
//
// Response includes details of the enqueued job including job status and the name
// of the fine-tuned models once complete.
//
// [Learn more about fine-tuning](https://platform.openai.com/docs/guides/model-optimization)
func (r *FineTuningJobService) New(ctx context.Context, body FineTuningJobNewParams, opts ...option.RequestOption) (res *FineTuningJob, err error) {
	opts = append(r.Options[:], opts...)
	path := "fine_tuning/jobs"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// Get info about a fine-tuning job.
//
// [Learn more about fine-tuning](https://platform.openai.com/docs/guides/model-optimization)
func (r *FineTuningJobService) Get(ctx context.Context, fineTuningJobID string, opts ...option.RequestOption) (res *FineTuningJob, err error) {
	opts = append(r.Options[:], opts...)
	if fineTuningJobID == "" {
		err = errors.New("missing required fine_tuning_job_id parameter")
		return
	}
	path := fmt.Sprintf("fine_tuning/jobs/%s", fineTuningJobID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// List your organization's fine-tuning jobs
func (r *FineTuningJobService) List(ctx context.Context, query FineTuningJobListParams, opts ...option.RequestOption) (res *pagination.CursorPage[FineTuningJob], err error) {
	var raw *http.Response
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithResponseInto(&raw)}, opts...)
	path := "fine_tuning/jobs"
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

// List your organization's fine-tuning jobs
func (r *FineTuningJobService) ListAutoPaging(ctx context.Context, query FineTuningJobListParams, opts ...option.RequestOption) *pagination.CursorPageAutoPager[FineTuningJob] {
	return pagination.NewCursorPageAutoPager(r.List(ctx, query, opts...))
}

// Immediately cancel a fine-tune job.
func (r *FineTuningJobService) Cancel(ctx context.Context, fineTuningJobID string, opts ...option.RequestOption) (res *FineTuningJob, err error) {
	opts = append(r.Options[:], opts...)
	if fineTuningJobID == "" {
		err = errors.New("missing required fine_tuning_job_id parameter")
		return
	}
	path := fmt.Sprintf("fine_tuning/jobs/%s/cancel", fineTuningJobID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, nil, &res, opts...)
	return
}

// Get status updates for a fine-tuning job.
func (r *FineTuningJobService) ListEvents(ctx context.Context, fineTuningJobID string, query FineTuningJobListEventsParams, opts ...option.RequestOption) (res *pagination.CursorPage[FineTuningJobEvent], err error) {
	var raw *http.Response
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithResponseInto(&raw)}, opts...)
	if fineTuningJobID == "" {
		err = errors.New("missing required fine_tuning_job_id parameter")
		return
	}
	path := fmt.Sprintf("fine_tuning/jobs/%s/events", fineTuningJobID)
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

// Get status updates for a fine-tuning job.
func (r *FineTuningJobService) ListEventsAutoPaging(ctx context.Context, fineTuningJobID string, query FineTuningJobListEventsParams, opts ...option.RequestOption) *pagination.CursorPageAutoPager[FineTuningJobEvent] {
	return pagination.NewCursorPageAutoPager(r.ListEvents(ctx, fineTuningJobID, query, opts...))
}

// Pause a fine-tune job.
func (r *FineTuningJobService) Pause(ctx context.Context, fineTuningJobID string, opts ...option.RequestOption) (res *FineTuningJob, err error) {
	opts = append(r.Options[:], opts...)
	if fineTuningJobID == "" {
		err = errors.New("missing required fine_tuning_job_id parameter")
		return
	}
	path := fmt.Sprintf("fine_tuning/jobs/%s/pause", fineTuningJobID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, nil, &res, opts...)
	return
}

// Resume a fine-tune job.
func (r *FineTuningJobService) Resume(ctx context.Context, fineTuningJobID string, opts ...option.RequestOption) (res *FineTuningJob, err error) {
	opts = append(r.Options[:], opts...)
	if fineTuningJobID == "" {
		err = errors.New("missing required fine_tuning_job_id parameter")
		return
	}
	path := fmt.Sprintf("fine_tuning/jobs/%s/resume", fineTuningJobID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, nil, &res, opts...)
	return
}

// The `fine_tuning.job` object represents a fine-tuning job that has been created
// through the API.
type FineTuningJob struct {
	// The object identifier, which can be referenced in the API endpoints.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) for when the fine-tuning job was created.
	CreatedAt int64 `json:"created_at,required"`
	// For fine-tuning jobs that have `failed`, this will contain more information on
	// the cause of the failure.
	Error FineTuningJobError `json:"error,required"`
	// The name of the fine-tuned model that is being created. The value will be null
	// if the fine-tuning job is still running.
	FineTunedModel string `json:"fine_tuned_model,required"`
	// The Unix timestamp (in seconds) for when the fine-tuning job was finished. The
	// value will be null if the fine-tuning job is still running.
	FinishedAt int64 `json:"finished_at,required"`
	// The hyperparameters used for the fine-tuning job. This value will only be
	// returned when running `supervised` jobs.
	Hyperparameters FineTuningJobHyperparameters `json:"hyperparameters,required"`
	// The base model that is being fine-tuned.
	Model string `json:"model,required"`
	// The object type, which is always "fine_tuning.job".
	Object constant.FineTuningJob `json:"object,required"`
	// The organization that owns the fine-tuning job.
	OrganizationID string `json:"organization_id,required"`
	// The compiled results file ID(s) for the fine-tuning job. You can retrieve the
	// results with the
	// [Files API](https://platform.openai.com/docs/api-reference/files/retrieve-contents).
	ResultFiles []string `json:"result_files,required"`
	// The seed used for the fine-tuning job.
	Seed int64 `json:"seed,required"`
	// The current status of the fine-tuning job, which can be either
	// `validating_files`, `queued`, `running`, `succeeded`, `failed`, or `cancelled`.
	//
	// Any of "validating_files", "queued", "running", "succeeded", "failed",
	// "cancelled".
	Status FineTuningJobStatus `json:"status,required"`
	// The total number of billable tokens processed by this fine-tuning job. The value
	// will be null if the fine-tuning job is still running.
	TrainedTokens int64 `json:"trained_tokens,required"`
	// The file ID used for training. You can retrieve the training data with the
	// [Files API](https://platform.openai.com/docs/api-reference/files/retrieve-contents).
	TrainingFile string `json:"training_file,required"`
	// The file ID used for validation. You can retrieve the validation results with
	// the
	// [Files API](https://platform.openai.com/docs/api-reference/files/retrieve-contents).
	ValidationFile string `json:"validation_file,required"`
	// The Unix timestamp (in seconds) for when the fine-tuning job is estimated to
	// finish. The value will be null if the fine-tuning job is not running.
	EstimatedFinish int64 `json:"estimated_finish,nullable"`
	// A list of integrations to enable for this fine-tuning job.
	Integrations []FineTuningJobWandbIntegrationObject `json:"integrations,nullable"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,nullable"`
	// The method used for fine-tuning.
	Method FineTuningJobMethod `json:"method"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID              respjson.Field
		CreatedAt       respjson.Field
		Error           respjson.Field
		FineTunedModel  respjson.Field
		FinishedAt      respjson.Field
		Hyperparameters respjson.Field
		Model           respjson.Field
		Object          respjson.Field
		OrganizationID  respjson.Field
		ResultFiles     respjson.Field
		Seed            respjson.Field
		Status          respjson.Field
		TrainedTokens   respjson.Field
		TrainingFile    respjson.Field
		ValidationFile  respjson.Field
		EstimatedFinish respjson.Field
		Integrations    respjson.Field
		Metadata        respjson.Field
		Method          respjson.Field
		ExtraFields     map[string]respjson.Field
		raw             string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FineTuningJob) RawJSON() string { return r.JSON.raw }
func (r *FineTuningJob) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// For fine-tuning jobs that have `failed`, this will contain more information on
// the cause of the failure.
type FineTuningJobError struct {
	// A machine-readable error code.
	Code string `json:"code,required"`
	// A human-readable error message.
	Message string `json:"message,required"`
	// The parameter that was invalid, usually `training_file` or `validation_file`.
	// This field will be null if the failure was not parameter-specific.
	Param string `json:"param,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Code        respjson.Field
		Message     respjson.Field
		Param       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FineTuningJobError) RawJSON() string { return r.JSON.raw }
func (r *FineTuningJobError) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The hyperparameters used for the fine-tuning job. This value will only be
// returned when running `supervised` jobs.
type FineTuningJobHyperparameters struct {
	// Number of examples in each batch. A larger batch size means that model
	// parameters are updated less frequently, but with lower variance.
	BatchSize FineTuningJobHyperparametersBatchSizeUnion `json:"batch_size,nullable"`
	// Scaling factor for the learning rate. A smaller learning rate may be useful to
	// avoid overfitting.
	LearningRateMultiplier FineTuningJobHyperparametersLearningRateMultiplierUnion `json:"learning_rate_multiplier"`
	// The number of epochs to train the model for. An epoch refers to one full cycle
	// through the training dataset.
	NEpochs FineTuningJobHyperparametersNEpochsUnion `json:"n_epochs"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		BatchSize              respjson.Field
		LearningRateMultiplier respjson.Field
		NEpochs                respjson.Field
		ExtraFields            map[string]respjson.Field
		raw                    string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FineTuningJobHyperparameters) RawJSON() string { return r.JSON.raw }
func (r *FineTuningJobHyperparameters) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// FineTuningJobHyperparametersBatchSizeUnion contains all possible properties and
// values from [constant.Auto], [int64].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfAuto OfInt]
type FineTuningJobHyperparametersBatchSizeUnion struct {
	// This field will be present if the value is a [constant.Auto] instead of an
	// object.
	OfAuto constant.Auto `json:",inline"`
	// This field will be present if the value is a [int64] instead of an object.
	OfInt int64 `json:",inline"`
	JSON  struct {
		OfAuto respjson.Field
		OfInt  respjson.Field
		raw    string
	} `json:"-"`
}

func (u FineTuningJobHyperparametersBatchSizeUnion) AsAuto() (v constant.Auto) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u FineTuningJobHyperparametersBatchSizeUnion) AsInt() (v int64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u FineTuningJobHyperparametersBatchSizeUnion) RawJSON() string { return u.JSON.raw }

func (r *FineTuningJobHyperparametersBatchSizeUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// FineTuningJobHyperparametersLearningRateMultiplierUnion contains all possible
// properties and values from [constant.Auto], [float64].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfAuto OfFloat]
type FineTuningJobHyperparametersLearningRateMultiplierUnion struct {
	// This field will be present if the value is a [constant.Auto] instead of an
	// object.
	OfAuto constant.Auto `json:",inline"`
	// This field will be present if the value is a [float64] instead of an object.
	OfFloat float64 `json:",inline"`
	JSON    struct {
		OfAuto  respjson.Field
		OfFloat respjson.Field
		raw     string
	} `json:"-"`
}

func (u FineTuningJobHyperparametersLearningRateMultiplierUnion) AsAuto() (v constant.Auto) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u FineTuningJobHyperparametersLearningRateMultiplierUnion) AsFloat() (v float64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u FineTuningJobHyperparametersLearningRateMultiplierUnion) RawJSON() string { return u.JSON.raw }

func (r *FineTuningJobHyperparametersLearningRateMultiplierUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// FineTuningJobHyperparametersNEpochsUnion contains all possible properties and
// values from [constant.Auto], [int64].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfAuto OfInt]
type FineTuningJobHyperparametersNEpochsUnion struct {
	// This field will be present if the value is a [constant.Auto] instead of an
	// object.
	OfAuto constant.Auto `json:",inline"`
	// This field will be present if the value is a [int64] instead of an object.
	OfInt int64 `json:",inline"`
	JSON  struct {
		OfAuto respjson.Field
		OfInt  respjson.Field
		raw    string
	} `json:"-"`
}

func (u FineTuningJobHyperparametersNEpochsUnion) AsAuto() (v constant.Auto) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u FineTuningJobHyperparametersNEpochsUnion) AsInt() (v int64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u FineTuningJobHyperparametersNEpochsUnion) RawJSON() string { return u.JSON.raw }

func (r *FineTuningJobHyperparametersNEpochsUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The current status of the fine-tuning job, which can be either
// `validating_files`, `queued`, `running`, `succeeded`, `failed`, or `cancelled`.
type FineTuningJobStatus string

const (
	FineTuningJobStatusValidatingFiles FineTuningJobStatus = "validating_files"
	FineTuningJobStatusQueued          FineTuningJobStatus = "queued"
	FineTuningJobStatusRunning         FineTuningJobStatus = "running"
	FineTuningJobStatusSucceeded       FineTuningJobStatus = "succeeded"
	FineTuningJobStatusFailed          FineTuningJobStatus = "failed"
	FineTuningJobStatusCancelled       FineTuningJobStatus = "cancelled"
)

// The method used for fine-tuning.
type FineTuningJobMethod struct {
	// The type of method. Is either `supervised`, `dpo`, or `reinforcement`.
	//
	// Any of "supervised", "dpo", "reinforcement".
	Type string `json:"type,required"`
	// Configuration for the DPO fine-tuning method.
	Dpo DpoMethod `json:"dpo"`
	// Configuration for the reinforcement fine-tuning method.
	Reinforcement ReinforcementMethod `json:"reinforcement"`
	// Configuration for the supervised fine-tuning method.
	Supervised SupervisedMethod `json:"supervised"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type          respjson.Field
		Dpo           respjson.Field
		Reinforcement respjson.Field
		Supervised    respjson.Field
		ExtraFields   map[string]respjson.Field
		raw           string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FineTuningJobMethod) RawJSON() string { return r.JSON.raw }
func (r *FineTuningJobMethod) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Fine-tuning job event object
type FineTuningJobEvent struct {
	// The object identifier.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) for when the fine-tuning job was created.
	CreatedAt int64 `json:"created_at,required"`
	// The log level of the event.
	//
	// Any of "info", "warn", "error".
	Level FineTuningJobEventLevel `json:"level,required"`
	// The message of the event.
	Message string `json:"message,required"`
	// The object type, which is always "fine_tuning.job.event".
	Object constant.FineTuningJobEvent `json:"object,required"`
	// The data associated with the event.
	Data any `json:"data"`
	// The type of event.
	//
	// Any of "message", "metrics".
	Type FineTuningJobEventType `json:"type"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		CreatedAt   respjson.Field
		Level       respjson.Field
		Message     respjson.Field
		Object      respjson.Field
		Data        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FineTuningJobEvent) RawJSON() string { return r.JSON.raw }
func (r *FineTuningJobEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The log level of the event.
type FineTuningJobEventLevel string

const (
	FineTuningJobEventLevelInfo  FineTuningJobEventLevel = "info"
	FineTuningJobEventLevelWarn  FineTuningJobEventLevel = "warn"
	FineTuningJobEventLevelError FineTuningJobEventLevel = "error"
)

// The type of event.
type FineTuningJobEventType string

const (
	FineTuningJobEventTypeMessage FineTuningJobEventType = "message"
	FineTuningJobEventTypeMetrics FineTuningJobEventType = "metrics"
)

// The settings for your integration with Weights and Biases. This payload
// specifies the project that metrics will be sent to. Optionally, you can set an
// explicit display name for your run, add tags to your run, and set a default
// entity (team, username, etc) to be associated with your run.
type FineTuningJobWandbIntegration struct {
	// The name of the project that the new run will be created under.
	Project string `json:"project,required"`
	// The entity to use for the run. This allows you to set the team or username of
	// the WandB user that you would like associated with the run. If not set, the
	// default entity for the registered WandB API key is used.
	Entity string `json:"entity,nullable"`
	// A display name to set for the run. If not set, we will use the Job ID as the
	// name.
	Name string `json:"name,nullable"`
	// A list of tags to be attached to the newly created run. These tags are passed
	// through directly to WandB. Some default tags are generated by OpenAI:
	// "openai/finetune", "openai/{base-model}", "openai/{ftjob-abcdef}".
	Tags []string `json:"tags"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Project     respjson.Field
		Entity      respjson.Field
		Name        respjson.Field
		Tags        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FineTuningJobWandbIntegration) RawJSON() string { return r.JSON.raw }
func (r *FineTuningJobWandbIntegration) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type FineTuningJobWandbIntegrationObject struct {
	// The type of the integration being enabled for the fine-tuning job
	Type constant.Wandb `json:"type,required"`
	// The settings for your integration with Weights and Biases. This payload
	// specifies the project that metrics will be sent to. Optionally, you can set an
	// explicit display name for your run, add tags to your run, and set a default
	// entity (team, username, etc) to be associated with your run.
	Wandb FineTuningJobWandbIntegration `json:"wandb,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		Wandb       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FineTuningJobWandbIntegrationObject) RawJSON() string { return r.JSON.raw }
func (r *FineTuningJobWandbIntegrationObject) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type FineTuningJobNewParams struct {
	// The name of the model to fine-tune. You can select one of the
	// [supported models](https://platform.openai.com/docs/guides/fine-tuning#which-models-can-be-fine-tuned).
	Model FineTuningJobNewParamsModel `json:"model,omitzero,required"`
	// The ID of an uploaded file that contains training data.
	//
	// See [upload file](https://platform.openai.com/docs/api-reference/files/create)
	// for how to upload a file.
	//
	// Your dataset must be formatted as a JSONL file. Additionally, you must upload
	// your file with the purpose `fine-tune`.
	//
	// The contents of the file should differ depending on if the model uses the
	// [chat](https://platform.openai.com/docs/api-reference/fine-tuning/chat-input),
	// [completions](https://platform.openai.com/docs/api-reference/fine-tuning/completions-input)
	// format, or if the fine-tuning method uses the
	// [preference](https://platform.openai.com/docs/api-reference/fine-tuning/preference-input)
	// format.
	//
	// See the
	// [fine-tuning guide](https://platform.openai.com/docs/guides/model-optimization)
	// for more details.
	TrainingFile string `json:"training_file,required"`
	// The seed controls the reproducibility of the job. Passing in the same seed and
	// job parameters should produce the same results, but may differ in rare cases. If
	// a seed is not specified, one will be generated for you.
	Seed param.Opt[int64] `json:"seed,omitzero"`
	// A string of up to 64 characters that will be added to your fine-tuned model
	// name.
	//
	// For example, a `suffix` of "custom-model-name" would produce a model name like
	// `ft:gpt-4o-mini:openai:custom-model-name:7p4lURel`.
	Suffix param.Opt[string] `json:"suffix,omitzero"`
	// The ID of an uploaded file that contains validation data.
	//
	// If you provide this file, the data is used to generate validation metrics
	// periodically during fine-tuning. These metrics can be viewed in the fine-tuning
	// results file. The same data should not be present in both train and validation
	// files.
	//
	// Your dataset must be formatted as a JSONL file. You must upload your file with
	// the purpose `fine-tune`.
	//
	// See the
	// [fine-tuning guide](https://platform.openai.com/docs/guides/model-optimization)
	// for more details.
	ValidationFile param.Opt[string] `json:"validation_file,omitzero"`
	// A list of integrations to enable for your fine-tuning job.
	Integrations []FineTuningJobNewParamsIntegration `json:"integrations,omitzero"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,omitzero"`
	// The hyperparameters used for the fine-tuning job. This value is now deprecated
	// in favor of `method`, and should be passed in under the `method` parameter.
	Hyperparameters FineTuningJobNewParamsHyperparameters `json:"hyperparameters,omitzero"`
	// The method used for fine-tuning.
	Method FineTuningJobNewParamsMethod `json:"method,omitzero"`
	paramObj
}

func (r FineTuningJobNewParams) MarshalJSON() (data []byte, err error) {
	type shadow FineTuningJobNewParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *FineTuningJobNewParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The name of the model to fine-tune. You can select one of the
// [supported models](https://platform.openai.com/docs/guides/fine-tuning#which-models-can-be-fine-tuned).
type FineTuningJobNewParamsModel string

const (
	FineTuningJobNewParamsModelBabbage002  FineTuningJobNewParamsModel = "babbage-002"
	FineTuningJobNewParamsModelDavinci002  FineTuningJobNewParamsModel = "davinci-002"
	FineTuningJobNewParamsModelGPT3_5Turbo FineTuningJobNewParamsModel = "gpt-3.5-turbo"
	FineTuningJobNewParamsModelGPT4oMini   FineTuningJobNewParamsModel = "gpt-4o-mini"
)

// The hyperparameters used for the fine-tuning job. This value is now deprecated
// in favor of `method`, and should be passed in under the `method` parameter.
//
// Deprecated: deprecated
type FineTuningJobNewParamsHyperparameters struct {
	// Number of examples in each batch. A larger batch size means that model
	// parameters are updated less frequently, but with lower variance.
	BatchSize FineTuningJobNewParamsHyperparametersBatchSizeUnion `json:"batch_size,omitzero"`
	// Scaling factor for the learning rate. A smaller learning rate may be useful to
	// avoid overfitting.
	LearningRateMultiplier FineTuningJobNewParamsHyperparametersLearningRateMultiplierUnion `json:"learning_rate_multiplier,omitzero"`
	// The number of epochs to train the model for. An epoch refers to one full cycle
	// through the training dataset.
	NEpochs FineTuningJobNewParamsHyperparametersNEpochsUnion `json:"n_epochs,omitzero"`
	paramObj
}

func (r FineTuningJobNewParamsHyperparameters) MarshalJSON() (data []byte, err error) {
	type shadow FineTuningJobNewParamsHyperparameters
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *FineTuningJobNewParamsHyperparameters) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type FineTuningJobNewParamsHyperparametersBatchSizeUnion struct {
	// Construct this variant with constant.ValueOf[constant.Auto]()
	OfAuto constant.Auto    `json:",omitzero,inline"`
	OfInt  param.Opt[int64] `json:",omitzero,inline"`
	paramUnion
}

func (u FineTuningJobNewParamsHyperparametersBatchSizeUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfInt)
}
func (u *FineTuningJobNewParamsHyperparametersBatchSizeUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *FineTuningJobNewParamsHyperparametersBatchSizeUnion) asAny() any {
	if !param.IsOmitted(u.OfAuto) {
		return &u.OfAuto
	} else if !param.IsOmitted(u.OfInt) {
		return &u.OfInt.Value
	}
	return nil
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type FineTuningJobNewParamsHyperparametersLearningRateMultiplierUnion struct {
	// Construct this variant with constant.ValueOf[constant.Auto]()
	OfAuto  constant.Auto      `json:",omitzero,inline"`
	OfFloat param.Opt[float64] `json:",omitzero,inline"`
	paramUnion
}

func (u FineTuningJobNewParamsHyperparametersLearningRateMultiplierUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfFloat)
}
func (u *FineTuningJobNewParamsHyperparametersLearningRateMultiplierUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *FineTuningJobNewParamsHyperparametersLearningRateMultiplierUnion) asAny() any {
	if !param.IsOmitted(u.OfAuto) {
		return &u.OfAuto
	} else if !param.IsOmitted(u.OfFloat) {
		return &u.OfFloat.Value
	}
	return nil
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type FineTuningJobNewParamsHyperparametersNEpochsUnion struct {
	// Construct this variant with constant.ValueOf[constant.Auto]()
	OfAuto constant.Auto    `json:",omitzero,inline"`
	OfInt  param.Opt[int64] `json:",omitzero,inline"`
	paramUnion
}

func (u FineTuningJobNewParamsHyperparametersNEpochsUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfInt)
}
func (u *FineTuningJobNewParamsHyperparametersNEpochsUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *FineTuningJobNewParamsHyperparametersNEpochsUnion) asAny() any {
	if !param.IsOmitted(u.OfAuto) {
		return &u.OfAuto
	} else if !param.IsOmitted(u.OfInt) {
		return &u.OfInt.Value
	}
	return nil
}

// The properties Type, Wandb are required.
type FineTuningJobNewParamsIntegration struct {
	// The settings for your integration with Weights and Biases. This payload
	// specifies the project that metrics will be sent to. Optionally, you can set an
	// explicit display name for your run, add tags to your run, and set a default
	// entity (team, username, etc) to be associated with your run.
	Wandb FineTuningJobNewParamsIntegrationWandb `json:"wandb,omitzero,required"`
	// The type of integration to enable. Currently, only "wandb" (Weights and Biases)
	// is supported.
	//
	// This field can be elided, and will marshal its zero value as "wandb".
	Type constant.Wandb `json:"type,required"`
	paramObj
}

func (r FineTuningJobNewParamsIntegration) MarshalJSON() (data []byte, err error) {
	type shadow FineTuningJobNewParamsIntegration
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *FineTuningJobNewParamsIntegration) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The settings for your integration with Weights and Biases. This payload
// specifies the project that metrics will be sent to. Optionally, you can set an
// explicit display name for your run, add tags to your run, and set a default
// entity (team, username, etc) to be associated with your run.
//
// The property Project is required.
type FineTuningJobNewParamsIntegrationWandb struct {
	// The name of the project that the new run will be created under.
	Project string `json:"project,required"`
	// The entity to use for the run. This allows you to set the team or username of
	// the WandB user that you would like associated with the run. If not set, the
	// default entity for the registered WandB API key is used.
	Entity param.Opt[string] `json:"entity,omitzero"`
	// A display name to set for the run. If not set, we will use the Job ID as the
	// name.
	Name param.Opt[string] `json:"name,omitzero"`
	// A list of tags to be attached to the newly created run. These tags are passed
	// through directly to WandB. Some default tags are generated by OpenAI:
	// "openai/finetune", "openai/{base-model}", "openai/{ftjob-abcdef}".
	Tags []string `json:"tags,omitzero"`
	paramObj
}

func (r FineTuningJobNewParamsIntegrationWandb) MarshalJSON() (data []byte, err error) {
	type shadow FineTuningJobNewParamsIntegrationWandb
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *FineTuningJobNewParamsIntegrationWandb) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The method used for fine-tuning.
//
// The property Type is required.
type FineTuningJobNewParamsMethod struct {
	// The type of method. Is either `supervised`, `dpo`, or `reinforcement`.
	//
	// Any of "supervised", "dpo", "reinforcement".
	Type string `json:"type,omitzero,required"`
	// Configuration for the DPO fine-tuning method.
	Dpo DpoMethodParam `json:"dpo,omitzero"`
	// Configuration for the reinforcement fine-tuning method.
	Reinforcement ReinforcementMethodParam `json:"reinforcement,omitzero"`
	// Configuration for the supervised fine-tuning method.
	Supervised SupervisedMethodParam `json:"supervised,omitzero"`
	paramObj
}

func (r FineTuningJobNewParamsMethod) MarshalJSON() (data []byte, err error) {
	type shadow FineTuningJobNewParamsMethod
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *FineTuningJobNewParamsMethod) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func init() {
	apijson.RegisterFieldValidator[FineTuningJobNewParamsMethod](
		"type", "supervised", "dpo", "reinforcement",
	)
}

type FineTuningJobListParams struct {
	// Identifier for the last job from the previous pagination request.
	After param.Opt[string] `query:"after,omitzero" json:"-"`
	// Number of fine-tuning jobs to retrieve.
	Limit param.Opt[int64] `query:"limit,omitzero" json:"-"`
	// Optional metadata filter. To filter, use the syntax `metadata[k]=v`.
	// Alternatively, set `metadata=null` to indicate no metadata.
	Metadata map[string]string `query:"metadata,omitzero" json:"-"`
	paramObj
}

// URLQuery serializes [FineTuningJobListParams]'s query parameters as
// `url.Values`.
func (r FineTuningJobListParams) URLQuery() (v url.Values, err error) {
	return apiquery.MarshalWithSettings(r, apiquery.QuerySettings{
		ArrayFormat:  apiquery.ArrayQueryFormatBrackets,
		NestedFormat: apiquery.NestedQueryFormatBrackets,
	})
}

type FineTuningJobListEventsParams struct {
	// Identifier for the last event from the previous pagination request.
	After param.Opt[string] `query:"after,omitzero" json:"-"`
	// Number of events to retrieve.
	Limit param.Opt[int64] `query:"limit,omitzero" json:"-"`
	paramObj
}

// URLQuery serializes [FineTuningJobListEventsParams]'s query parameters as
// `url.Values`.
func (r FineTuningJobListEventsParams) URLQuery() (v url.Values, err error) {
	return apiquery.MarshalWithSettings(r, apiquery.QuerySettings{
		ArrayFormat:  apiquery.ArrayQueryFormatBrackets,
		NestedFormat: apiquery.NestedQueryFormatBrackets,
	})
}
