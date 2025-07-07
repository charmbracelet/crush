// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package openai

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/openai/openai-go/internal/apijson"
	"github.com/openai/openai-go/internal/requestconfig"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/respjson"
)

// FineTuningAlphaGraderService contains methods and other services that help with
// interacting with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewFineTuningAlphaGraderService] method instead.
type FineTuningAlphaGraderService struct {
	Options []option.RequestOption
}

// NewFineTuningAlphaGraderService generates a new service that applies the given
// options to each request. These options are applied after the parent client's
// options (if there is one), and before any request-specific options.
func NewFineTuningAlphaGraderService(opts ...option.RequestOption) (r FineTuningAlphaGraderService) {
	r = FineTuningAlphaGraderService{}
	r.Options = opts
	return
}

// Run a grader.
func (r *FineTuningAlphaGraderService) Run(ctx context.Context, body FineTuningAlphaGraderRunParams, opts ...option.RequestOption) (res *FineTuningAlphaGraderRunResponse, err error) {
	opts = append(r.Options[:], opts...)
	path := "fine_tuning/alpha/graders/run"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// Validate a grader.
func (r *FineTuningAlphaGraderService) Validate(ctx context.Context, body FineTuningAlphaGraderValidateParams, opts ...option.RequestOption) (res *FineTuningAlphaGraderValidateResponse, err error) {
	opts = append(r.Options[:], opts...)
	path := "fine_tuning/alpha/graders/validate"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

type FineTuningAlphaGraderRunResponse struct {
	Metadata                      FineTuningAlphaGraderRunResponseMetadata `json:"metadata,required"`
	ModelGraderTokenUsagePerModel map[string]any                           `json:"model_grader_token_usage_per_model,required"`
	Reward                        float64                                  `json:"reward,required"`
	SubRewards                    map[string]any                           `json:"sub_rewards,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Metadata                      respjson.Field
		ModelGraderTokenUsagePerModel respjson.Field
		Reward                        respjson.Field
		SubRewards                    respjson.Field
		ExtraFields                   map[string]respjson.Field
		raw                           string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FineTuningAlphaGraderRunResponse) RawJSON() string { return r.JSON.raw }
func (r *FineTuningAlphaGraderRunResponse) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type FineTuningAlphaGraderRunResponseMetadata struct {
	Errors           FineTuningAlphaGraderRunResponseMetadataErrors `json:"errors,required"`
	ExecutionTime    float64                                        `json:"execution_time,required"`
	Name             string                                         `json:"name,required"`
	SampledModelName string                                         `json:"sampled_model_name,required"`
	Scores           map[string]any                                 `json:"scores,required"`
	TokenUsage       int64                                          `json:"token_usage,required"`
	Type             string                                         `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Errors           respjson.Field
		ExecutionTime    respjson.Field
		Name             respjson.Field
		SampledModelName respjson.Field
		Scores           respjson.Field
		TokenUsage       respjson.Field
		Type             respjson.Field
		ExtraFields      map[string]respjson.Field
		raw              string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FineTuningAlphaGraderRunResponseMetadata) RawJSON() string { return r.JSON.raw }
func (r *FineTuningAlphaGraderRunResponseMetadata) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type FineTuningAlphaGraderRunResponseMetadataErrors struct {
	FormulaParseError               bool   `json:"formula_parse_error,required"`
	InvalidVariableError            bool   `json:"invalid_variable_error,required"`
	ModelGraderParseError           bool   `json:"model_grader_parse_error,required"`
	ModelGraderRefusalError         bool   `json:"model_grader_refusal_error,required"`
	ModelGraderServerError          bool   `json:"model_grader_server_error,required"`
	ModelGraderServerErrorDetails   string `json:"model_grader_server_error_details,required"`
	OtherError                      bool   `json:"other_error,required"`
	PythonGraderRuntimeError        bool   `json:"python_grader_runtime_error,required"`
	PythonGraderRuntimeErrorDetails string `json:"python_grader_runtime_error_details,required"`
	PythonGraderServerError         bool   `json:"python_grader_server_error,required"`
	PythonGraderServerErrorType     string `json:"python_grader_server_error_type,required"`
	SampleParseError                bool   `json:"sample_parse_error,required"`
	TruncatedObservationError       bool   `json:"truncated_observation_error,required"`
	UnresponsiveRewardError         bool   `json:"unresponsive_reward_error,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		FormulaParseError               respjson.Field
		InvalidVariableError            respjson.Field
		ModelGraderParseError           respjson.Field
		ModelGraderRefusalError         respjson.Field
		ModelGraderServerError          respjson.Field
		ModelGraderServerErrorDetails   respjson.Field
		OtherError                      respjson.Field
		PythonGraderRuntimeError        respjson.Field
		PythonGraderRuntimeErrorDetails respjson.Field
		PythonGraderServerError         respjson.Field
		PythonGraderServerErrorType     respjson.Field
		SampleParseError                respjson.Field
		TruncatedObservationError       respjson.Field
		UnresponsiveRewardError         respjson.Field
		ExtraFields                     map[string]respjson.Field
		raw                             string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FineTuningAlphaGraderRunResponseMetadataErrors) RawJSON() string { return r.JSON.raw }
func (r *FineTuningAlphaGraderRunResponseMetadataErrors) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type FineTuningAlphaGraderValidateResponse struct {
	// The grader used for the fine-tuning job.
	Grader FineTuningAlphaGraderValidateResponseGraderUnion `json:"grader"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Grader      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FineTuningAlphaGraderValidateResponse) RawJSON() string { return r.JSON.raw }
func (r *FineTuningAlphaGraderValidateResponse) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// FineTuningAlphaGraderValidateResponseGraderUnion contains all possible
// properties and values from [StringCheckGrader], [TextSimilarityGrader],
// [PythonGrader], [ScoreModelGrader], [MultiGrader].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type FineTuningAlphaGraderValidateResponseGraderUnion struct {
	// This field is a union of [string], [string], [[]ScoreModelGraderInput]
	Input FineTuningAlphaGraderValidateResponseGraderUnionInput `json:"input"`
	Name  string                                                `json:"name"`
	// This field is from variant [StringCheckGrader].
	Operation StringCheckGraderOperation `json:"operation"`
	Reference string                     `json:"reference"`
	Type      string                     `json:"type"`
	// This field is from variant [TextSimilarityGrader].
	EvaluationMetric TextSimilarityGraderEvaluationMetric `json:"evaluation_metric"`
	// This field is from variant [PythonGrader].
	Source string `json:"source"`
	// This field is from variant [PythonGrader].
	ImageTag string `json:"image_tag"`
	// This field is from variant [ScoreModelGrader].
	Model string `json:"model"`
	// This field is from variant [ScoreModelGrader].
	Range []float64 `json:"range"`
	// This field is from variant [ScoreModelGrader].
	SamplingParams any `json:"sampling_params"`
	// This field is from variant [MultiGrader].
	CalculateOutput string `json:"calculate_output"`
	// This field is from variant [MultiGrader].
	Graders MultiGraderGradersUnion `json:"graders"`
	JSON    struct {
		Input            respjson.Field
		Name             respjson.Field
		Operation        respjson.Field
		Reference        respjson.Field
		Type             respjson.Field
		EvaluationMetric respjson.Field
		Source           respjson.Field
		ImageTag         respjson.Field
		Model            respjson.Field
		Range            respjson.Field
		SamplingParams   respjson.Field
		CalculateOutput  respjson.Field
		Graders          respjson.Field
		raw              string
	} `json:"-"`
}

func (u FineTuningAlphaGraderValidateResponseGraderUnion) AsStringCheckGrader() (v StringCheckGrader) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u FineTuningAlphaGraderValidateResponseGraderUnion) AsTextSimilarityGrader() (v TextSimilarityGrader) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u FineTuningAlphaGraderValidateResponseGraderUnion) AsPythonGrader() (v PythonGrader) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u FineTuningAlphaGraderValidateResponseGraderUnion) AsScoreModelGrader() (v ScoreModelGrader) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u FineTuningAlphaGraderValidateResponseGraderUnion) AsMultiGrader() (v MultiGrader) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u FineTuningAlphaGraderValidateResponseGraderUnion) RawJSON() string { return u.JSON.raw }

func (r *FineTuningAlphaGraderValidateResponseGraderUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// FineTuningAlphaGraderValidateResponseGraderUnionInput is an implicit subunion of
// [FineTuningAlphaGraderValidateResponseGraderUnion].
// FineTuningAlphaGraderValidateResponseGraderUnionInput provides convenient access
// to the sub-properties of the union.
//
// For type safety it is recommended to directly use a variant of the
// [FineTuningAlphaGraderValidateResponseGraderUnion].
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfString OfScoreModelGraderInputArray]
type FineTuningAlphaGraderValidateResponseGraderUnionInput struct {
	// This field will be present if the value is a [string] instead of an object.
	OfString string `json:",inline"`
	// This field will be present if the value is a [[]ScoreModelGraderInput] instead
	// of an object.
	OfScoreModelGraderInputArray []ScoreModelGraderInput `json:",inline"`
	JSON                         struct {
		OfString                     respjson.Field
		OfScoreModelGraderInputArray respjson.Field
		raw                          string
	} `json:"-"`
}

func (r *FineTuningAlphaGraderValidateResponseGraderUnionInput) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type FineTuningAlphaGraderRunParams struct {
	// The grader used for the fine-tuning job.
	Grader FineTuningAlphaGraderRunParamsGraderUnion `json:"grader,omitzero,required"`
	// The model sample to be evaluated. This value will be used to populate the
	// `sample` namespace. See
	// [the guide](https://platform.openai.com/docs/guides/graders) for more details.
	// The `output_json` variable will be populated if the model sample is a valid JSON
	// string.
	ModelSample string `json:"model_sample,required"`
	// The dataset item provided to the grader. This will be used to populate the
	// `item` namespace. See
	// [the guide](https://platform.openai.com/docs/guides/graders) for more details.
	Item any `json:"item,omitzero"`
	paramObj
}

func (r FineTuningAlphaGraderRunParams) MarshalJSON() (data []byte, err error) {
	type shadow FineTuningAlphaGraderRunParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *FineTuningAlphaGraderRunParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type FineTuningAlphaGraderRunParamsGraderUnion struct {
	OfStringCheck    *StringCheckGraderParam    `json:",omitzero,inline"`
	OfTextSimilarity *TextSimilarityGraderParam `json:",omitzero,inline"`
	OfPython         *PythonGraderParam         `json:",omitzero,inline"`
	OfScoreModel     *ScoreModelGraderParam     `json:",omitzero,inline"`
	OfMulti          *MultiGraderParam          `json:",omitzero,inline"`
	paramUnion
}

func (u FineTuningAlphaGraderRunParamsGraderUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfStringCheck,
		u.OfTextSimilarity,
		u.OfPython,
		u.OfScoreModel,
		u.OfMulti)
}
func (u *FineTuningAlphaGraderRunParamsGraderUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *FineTuningAlphaGraderRunParamsGraderUnion) asAny() any {
	if !param.IsOmitted(u.OfStringCheck) {
		return u.OfStringCheck
	} else if !param.IsOmitted(u.OfTextSimilarity) {
		return u.OfTextSimilarity
	} else if !param.IsOmitted(u.OfPython) {
		return u.OfPython
	} else if !param.IsOmitted(u.OfScoreModel) {
		return u.OfScoreModel
	} else if !param.IsOmitted(u.OfMulti) {
		return u.OfMulti
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderRunParamsGraderUnion) GetOperation() *string {
	if vt := u.OfStringCheck; vt != nil {
		return (*string)(&vt.Operation)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderRunParamsGraderUnion) GetEvaluationMetric() *string {
	if vt := u.OfTextSimilarity; vt != nil {
		return (*string)(&vt.EvaluationMetric)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderRunParamsGraderUnion) GetSource() *string {
	if vt := u.OfPython; vt != nil {
		return &vt.Source
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderRunParamsGraderUnion) GetImageTag() *string {
	if vt := u.OfPython; vt != nil && vt.ImageTag.Valid() {
		return &vt.ImageTag.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderRunParamsGraderUnion) GetModel() *string {
	if vt := u.OfScoreModel; vt != nil {
		return &vt.Model
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderRunParamsGraderUnion) GetRange() []float64 {
	if vt := u.OfScoreModel; vt != nil {
		return vt.Range
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderRunParamsGraderUnion) GetSamplingParams() *any {
	if vt := u.OfScoreModel; vt != nil {
		return &vt.SamplingParams
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderRunParamsGraderUnion) GetCalculateOutput() *string {
	if vt := u.OfMulti; vt != nil {
		return &vt.CalculateOutput
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderRunParamsGraderUnion) GetGraders() *MultiGraderGradersUnionParam {
	if vt := u.OfMulti; vt != nil {
		return &vt.Graders
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderRunParamsGraderUnion) GetName() *string {
	if vt := u.OfStringCheck; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfTextSimilarity; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfPython; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfScoreModel; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfMulti; vt != nil {
		return (*string)(&vt.Name)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderRunParamsGraderUnion) GetReference() *string {
	if vt := u.OfStringCheck; vt != nil {
		return (*string)(&vt.Reference)
	} else if vt := u.OfTextSimilarity; vt != nil {
		return (*string)(&vt.Reference)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderRunParamsGraderUnion) GetType() *string {
	if vt := u.OfStringCheck; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfTextSimilarity; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfPython; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfScoreModel; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfMulti; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Returns a subunion which exports methods to access subproperties
//
// Or use AsAny() to get the underlying value
func (u FineTuningAlphaGraderRunParamsGraderUnion) GetInput() (res fineTuningAlphaGraderRunParamsGraderUnionInput) {
	if vt := u.OfStringCheck; vt != nil {
		res.any = &vt.Input
	} else if vt := u.OfTextSimilarity; vt != nil {
		res.any = &vt.Input
	} else if vt := u.OfScoreModel; vt != nil {
		res.any = &vt.Input
	}
	return
}

// Can have the runtime types [*string], [\*[]ScoreModelGraderInputParam]
type fineTuningAlphaGraderRunParamsGraderUnionInput struct{ any }

// Use the following switch statement to get the type of the union:
//
//	switch u.AsAny().(type) {
//	case *string:
//	case *[]openai.ScoreModelGraderInputParam:
//	default:
//	    fmt.Errorf("not present")
//	}
func (u fineTuningAlphaGraderRunParamsGraderUnionInput) AsAny() any { return u.any }

func init() {
	apijson.RegisterUnion[FineTuningAlphaGraderRunParamsGraderUnion](
		"type",
		apijson.Discriminator[StringCheckGraderParam]("string_check"),
		apijson.Discriminator[TextSimilarityGraderParam]("text_similarity"),
		apijson.Discriminator[PythonGraderParam]("python"),
		apijson.Discriminator[ScoreModelGraderParam]("score_model"),
		apijson.Discriminator[MultiGraderParam]("multi"),
	)
}

type FineTuningAlphaGraderValidateParams struct {
	// The grader used for the fine-tuning job.
	Grader FineTuningAlphaGraderValidateParamsGraderUnion `json:"grader,omitzero,required"`
	paramObj
}

func (r FineTuningAlphaGraderValidateParams) MarshalJSON() (data []byte, err error) {
	type shadow FineTuningAlphaGraderValidateParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *FineTuningAlphaGraderValidateParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type FineTuningAlphaGraderValidateParamsGraderUnion struct {
	OfStringCheckGrader    *StringCheckGraderParam    `json:",omitzero,inline"`
	OfTextSimilarityGrader *TextSimilarityGraderParam `json:",omitzero,inline"`
	OfPythonGrader         *PythonGraderParam         `json:",omitzero,inline"`
	OfScoreModelGrader     *ScoreModelGraderParam     `json:",omitzero,inline"`
	OfMultiGrader          *MultiGraderParam          `json:",omitzero,inline"`
	paramUnion
}

func (u FineTuningAlphaGraderValidateParamsGraderUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfStringCheckGrader,
		u.OfTextSimilarityGrader,
		u.OfPythonGrader,
		u.OfScoreModelGrader,
		u.OfMultiGrader)
}
func (u *FineTuningAlphaGraderValidateParamsGraderUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *FineTuningAlphaGraderValidateParamsGraderUnion) asAny() any {
	if !param.IsOmitted(u.OfStringCheckGrader) {
		return u.OfStringCheckGrader
	} else if !param.IsOmitted(u.OfTextSimilarityGrader) {
		return u.OfTextSimilarityGrader
	} else if !param.IsOmitted(u.OfPythonGrader) {
		return u.OfPythonGrader
	} else if !param.IsOmitted(u.OfScoreModelGrader) {
		return u.OfScoreModelGrader
	} else if !param.IsOmitted(u.OfMultiGrader) {
		return u.OfMultiGrader
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderValidateParamsGraderUnion) GetOperation() *string {
	if vt := u.OfStringCheckGrader; vt != nil {
		return (*string)(&vt.Operation)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderValidateParamsGraderUnion) GetEvaluationMetric() *string {
	if vt := u.OfTextSimilarityGrader; vt != nil {
		return (*string)(&vt.EvaluationMetric)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderValidateParamsGraderUnion) GetSource() *string {
	if vt := u.OfPythonGrader; vt != nil {
		return &vt.Source
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderValidateParamsGraderUnion) GetImageTag() *string {
	if vt := u.OfPythonGrader; vt != nil && vt.ImageTag.Valid() {
		return &vt.ImageTag.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderValidateParamsGraderUnion) GetModel() *string {
	if vt := u.OfScoreModelGrader; vt != nil {
		return &vt.Model
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderValidateParamsGraderUnion) GetRange() []float64 {
	if vt := u.OfScoreModelGrader; vt != nil {
		return vt.Range
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderValidateParamsGraderUnion) GetSamplingParams() *any {
	if vt := u.OfScoreModelGrader; vt != nil {
		return &vt.SamplingParams
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderValidateParamsGraderUnion) GetCalculateOutput() *string {
	if vt := u.OfMultiGrader; vt != nil {
		return &vt.CalculateOutput
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderValidateParamsGraderUnion) GetGraders() *MultiGraderGradersUnionParam {
	if vt := u.OfMultiGrader; vt != nil {
		return &vt.Graders
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderValidateParamsGraderUnion) GetName() *string {
	if vt := u.OfStringCheckGrader; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfTextSimilarityGrader; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfPythonGrader; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfScoreModelGrader; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfMultiGrader; vt != nil {
		return (*string)(&vt.Name)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderValidateParamsGraderUnion) GetReference() *string {
	if vt := u.OfStringCheckGrader; vt != nil {
		return (*string)(&vt.Reference)
	} else if vt := u.OfTextSimilarityGrader; vt != nil {
		return (*string)(&vt.Reference)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u FineTuningAlphaGraderValidateParamsGraderUnion) GetType() *string {
	if vt := u.OfStringCheckGrader; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfTextSimilarityGrader; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfPythonGrader; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfScoreModelGrader; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfMultiGrader; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Returns a subunion which exports methods to access subproperties
//
// Or use AsAny() to get the underlying value
func (u FineTuningAlphaGraderValidateParamsGraderUnion) GetInput() (res fineTuningAlphaGraderValidateParamsGraderUnionInput) {
	if vt := u.OfStringCheckGrader; vt != nil {
		res.any = &vt.Input
	} else if vt := u.OfTextSimilarityGrader; vt != nil {
		res.any = &vt.Input
	} else if vt := u.OfScoreModelGrader; vt != nil {
		res.any = &vt.Input
	}
	return
}

// Can have the runtime types [*string], [\*[]ScoreModelGraderInputParam]
type fineTuningAlphaGraderValidateParamsGraderUnionInput struct{ any }

// Use the following switch statement to get the type of the union:
//
//	switch u.AsAny().(type) {
//	case *string:
//	case *[]openai.ScoreModelGraderInputParam:
//	default:
//	    fmt.Errorf("not present")
//	}
func (u fineTuningAlphaGraderValidateParamsGraderUnionInput) AsAny() any { return u.any }
