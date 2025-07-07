// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package openai

import (
	"encoding/json"

	"github.com/openai/openai-go/internal/apijson"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/respjson"
	"github.com/openai/openai-go/shared/constant"
)

// FineTuningMethodService contains methods and other services that help with
// interacting with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewFineTuningMethodService] method instead.
type FineTuningMethodService struct {
	Options []option.RequestOption
}

// NewFineTuningMethodService generates a new service that applies the given
// options to each request. These options are applied after the parent client's
// options (if there is one), and before any request-specific options.
func NewFineTuningMethodService(opts ...option.RequestOption) (r FineTuningMethodService) {
	r = FineTuningMethodService{}
	r.Options = opts
	return
}

// The hyperparameters used for the DPO fine-tuning job.
type DpoHyperparametersResp struct {
	// Number of examples in each batch. A larger batch size means that model
	// parameters are updated less frequently, but with lower variance.
	BatchSize DpoHyperparametersBatchSizeUnionResp `json:"batch_size"`
	// The beta value for the DPO method. A higher beta value will increase the weight
	// of the penalty between the policy and reference model.
	Beta DpoHyperparametersBetaUnionResp `json:"beta"`
	// Scaling factor for the learning rate. A smaller learning rate may be useful to
	// avoid overfitting.
	LearningRateMultiplier DpoHyperparametersLearningRateMultiplierUnionResp `json:"learning_rate_multiplier"`
	// The number of epochs to train the model for. An epoch refers to one full cycle
	// through the training dataset.
	NEpochs DpoHyperparametersNEpochsUnionResp `json:"n_epochs"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		BatchSize              respjson.Field
		Beta                   respjson.Field
		LearningRateMultiplier respjson.Field
		NEpochs                respjson.Field
		ExtraFields            map[string]respjson.Field
		raw                    string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r DpoHyperparametersResp) RawJSON() string { return r.JSON.raw }
func (r *DpoHyperparametersResp) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this DpoHyperparametersResp to a DpoHyperparameters.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// DpoHyperparameters.Overrides()
func (r DpoHyperparametersResp) ToParam() DpoHyperparameters {
	return param.Override[DpoHyperparameters](json.RawMessage(r.RawJSON()))
}

// DpoHyperparametersBatchSizeUnionResp contains all possible properties and values
// from [constant.Auto], [int64].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfAuto OfInt]
type DpoHyperparametersBatchSizeUnionResp struct {
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

func (u DpoHyperparametersBatchSizeUnionResp) AsAuto() (v constant.Auto) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u DpoHyperparametersBatchSizeUnionResp) AsInt() (v int64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u DpoHyperparametersBatchSizeUnionResp) RawJSON() string { return u.JSON.raw }

func (r *DpoHyperparametersBatchSizeUnionResp) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// DpoHyperparametersBetaUnionResp contains all possible properties and values from
// [constant.Auto], [float64].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfAuto OfFloat]
type DpoHyperparametersBetaUnionResp struct {
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

func (u DpoHyperparametersBetaUnionResp) AsAuto() (v constant.Auto) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u DpoHyperparametersBetaUnionResp) AsFloat() (v float64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u DpoHyperparametersBetaUnionResp) RawJSON() string { return u.JSON.raw }

func (r *DpoHyperparametersBetaUnionResp) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// DpoHyperparametersLearningRateMultiplierUnionResp contains all possible
// properties and values from [constant.Auto], [float64].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfAuto OfFloat]
type DpoHyperparametersLearningRateMultiplierUnionResp struct {
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

func (u DpoHyperparametersLearningRateMultiplierUnionResp) AsAuto() (v constant.Auto) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u DpoHyperparametersLearningRateMultiplierUnionResp) AsFloat() (v float64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u DpoHyperparametersLearningRateMultiplierUnionResp) RawJSON() string { return u.JSON.raw }

func (r *DpoHyperparametersLearningRateMultiplierUnionResp) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// DpoHyperparametersNEpochsUnionResp contains all possible properties and values
// from [constant.Auto], [int64].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfAuto OfInt]
type DpoHyperparametersNEpochsUnionResp struct {
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

func (u DpoHyperparametersNEpochsUnionResp) AsAuto() (v constant.Auto) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u DpoHyperparametersNEpochsUnionResp) AsInt() (v int64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u DpoHyperparametersNEpochsUnionResp) RawJSON() string { return u.JSON.raw }

func (r *DpoHyperparametersNEpochsUnionResp) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The hyperparameters used for the DPO fine-tuning job.
type DpoHyperparameters struct {
	// Number of examples in each batch. A larger batch size means that model
	// parameters are updated less frequently, but with lower variance.
	BatchSize DpoHyperparametersBatchSizeUnion `json:"batch_size,omitzero"`
	// The beta value for the DPO method. A higher beta value will increase the weight
	// of the penalty between the policy and reference model.
	Beta DpoHyperparametersBetaUnion `json:"beta,omitzero"`
	// Scaling factor for the learning rate. A smaller learning rate may be useful to
	// avoid overfitting.
	LearningRateMultiplier DpoHyperparametersLearningRateMultiplierUnion `json:"learning_rate_multiplier,omitzero"`
	// The number of epochs to train the model for. An epoch refers to one full cycle
	// through the training dataset.
	NEpochs DpoHyperparametersNEpochsUnion `json:"n_epochs,omitzero"`
	paramObj
}

func (r DpoHyperparameters) MarshalJSON() (data []byte, err error) {
	type shadow DpoHyperparameters
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *DpoHyperparameters) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type DpoHyperparametersBatchSizeUnion struct {
	// Construct this variant with constant.ValueOf[constant.Auto]()
	OfAuto constant.Auto    `json:",omitzero,inline"`
	OfInt  param.Opt[int64] `json:",omitzero,inline"`
	paramUnion
}

func (u DpoHyperparametersBatchSizeUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfInt)
}
func (u *DpoHyperparametersBatchSizeUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *DpoHyperparametersBatchSizeUnion) asAny() any {
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
type DpoHyperparametersBetaUnion struct {
	// Construct this variant with constant.ValueOf[constant.Auto]()
	OfAuto  constant.Auto      `json:",omitzero,inline"`
	OfFloat param.Opt[float64] `json:",omitzero,inline"`
	paramUnion
}

func (u DpoHyperparametersBetaUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfFloat)
}
func (u *DpoHyperparametersBetaUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *DpoHyperparametersBetaUnion) asAny() any {
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
type DpoHyperparametersLearningRateMultiplierUnion struct {
	// Construct this variant with constant.ValueOf[constant.Auto]()
	OfAuto  constant.Auto      `json:",omitzero,inline"`
	OfFloat param.Opt[float64] `json:",omitzero,inline"`
	paramUnion
}

func (u DpoHyperparametersLearningRateMultiplierUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfFloat)
}
func (u *DpoHyperparametersLearningRateMultiplierUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *DpoHyperparametersLearningRateMultiplierUnion) asAny() any {
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
type DpoHyperparametersNEpochsUnion struct {
	// Construct this variant with constant.ValueOf[constant.Auto]()
	OfAuto constant.Auto    `json:",omitzero,inline"`
	OfInt  param.Opt[int64] `json:",omitzero,inline"`
	paramUnion
}

func (u DpoHyperparametersNEpochsUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfInt)
}
func (u *DpoHyperparametersNEpochsUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *DpoHyperparametersNEpochsUnion) asAny() any {
	if !param.IsOmitted(u.OfAuto) {
		return &u.OfAuto
	} else if !param.IsOmitted(u.OfInt) {
		return &u.OfInt.Value
	}
	return nil
}

// Configuration for the DPO fine-tuning method.
type DpoMethod struct {
	// The hyperparameters used for the DPO fine-tuning job.
	Hyperparameters DpoHyperparametersResp `json:"hyperparameters"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Hyperparameters respjson.Field
		ExtraFields     map[string]respjson.Field
		raw             string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r DpoMethod) RawJSON() string { return r.JSON.raw }
func (r *DpoMethod) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this DpoMethod to a DpoMethodParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// DpoMethodParam.Overrides()
func (r DpoMethod) ToParam() DpoMethodParam {
	return param.Override[DpoMethodParam](json.RawMessage(r.RawJSON()))
}

// Configuration for the DPO fine-tuning method.
type DpoMethodParam struct {
	// The hyperparameters used for the DPO fine-tuning job.
	Hyperparameters DpoHyperparameters `json:"hyperparameters,omitzero"`
	paramObj
}

func (r DpoMethodParam) MarshalJSON() (data []byte, err error) {
	type shadow DpoMethodParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *DpoMethodParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The hyperparameters used for the reinforcement fine-tuning job.
type ReinforcementHyperparametersResp struct {
	// Number of examples in each batch. A larger batch size means that model
	// parameters are updated less frequently, but with lower variance.
	BatchSize ReinforcementHyperparametersBatchSizeUnionResp `json:"batch_size"`
	// Multiplier on amount of compute used for exploring search space during training.
	ComputeMultiplier ReinforcementHyperparametersComputeMultiplierUnionResp `json:"compute_multiplier"`
	// The number of training steps between evaluation runs.
	EvalInterval ReinforcementHyperparametersEvalIntervalUnionResp `json:"eval_interval"`
	// Number of evaluation samples to generate per training step.
	EvalSamples ReinforcementHyperparametersEvalSamplesUnionResp `json:"eval_samples"`
	// Scaling factor for the learning rate. A smaller learning rate may be useful to
	// avoid overfitting.
	LearningRateMultiplier ReinforcementHyperparametersLearningRateMultiplierUnionResp `json:"learning_rate_multiplier"`
	// The number of epochs to train the model for. An epoch refers to one full cycle
	// through the training dataset.
	NEpochs ReinforcementHyperparametersNEpochsUnionResp `json:"n_epochs"`
	// Level of reasoning effort.
	//
	// Any of "default", "low", "medium", "high".
	ReasoningEffort ReinforcementHyperparametersReasoningEffort `json:"reasoning_effort"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		BatchSize              respjson.Field
		ComputeMultiplier      respjson.Field
		EvalInterval           respjson.Field
		EvalSamples            respjson.Field
		LearningRateMultiplier respjson.Field
		NEpochs                respjson.Field
		ReasoningEffort        respjson.Field
		ExtraFields            map[string]respjson.Field
		raw                    string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ReinforcementHyperparametersResp) RawJSON() string { return r.JSON.raw }
func (r *ReinforcementHyperparametersResp) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ReinforcementHyperparametersResp to a
// ReinforcementHyperparameters.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ReinforcementHyperparameters.Overrides()
func (r ReinforcementHyperparametersResp) ToParam() ReinforcementHyperparameters {
	return param.Override[ReinforcementHyperparameters](json.RawMessage(r.RawJSON()))
}

// ReinforcementHyperparametersBatchSizeUnionResp contains all possible properties
// and values from [constant.Auto], [int64].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfAuto OfInt]
type ReinforcementHyperparametersBatchSizeUnionResp struct {
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

func (u ReinforcementHyperparametersBatchSizeUnionResp) AsAuto() (v constant.Auto) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ReinforcementHyperparametersBatchSizeUnionResp) AsInt() (v int64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ReinforcementHyperparametersBatchSizeUnionResp) RawJSON() string { return u.JSON.raw }

func (r *ReinforcementHyperparametersBatchSizeUnionResp) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ReinforcementHyperparametersComputeMultiplierUnionResp contains all possible
// properties and values from [constant.Auto], [float64].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfAuto OfFloat]
type ReinforcementHyperparametersComputeMultiplierUnionResp struct {
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

func (u ReinforcementHyperparametersComputeMultiplierUnionResp) AsAuto() (v constant.Auto) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ReinforcementHyperparametersComputeMultiplierUnionResp) AsFloat() (v float64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ReinforcementHyperparametersComputeMultiplierUnionResp) RawJSON() string { return u.JSON.raw }

func (r *ReinforcementHyperparametersComputeMultiplierUnionResp) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ReinforcementHyperparametersEvalIntervalUnionResp contains all possible
// properties and values from [constant.Auto], [int64].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfAuto OfInt]
type ReinforcementHyperparametersEvalIntervalUnionResp struct {
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

func (u ReinforcementHyperparametersEvalIntervalUnionResp) AsAuto() (v constant.Auto) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ReinforcementHyperparametersEvalIntervalUnionResp) AsInt() (v int64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ReinforcementHyperparametersEvalIntervalUnionResp) RawJSON() string { return u.JSON.raw }

func (r *ReinforcementHyperparametersEvalIntervalUnionResp) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ReinforcementHyperparametersEvalSamplesUnionResp contains all possible
// properties and values from [constant.Auto], [int64].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfAuto OfInt]
type ReinforcementHyperparametersEvalSamplesUnionResp struct {
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

func (u ReinforcementHyperparametersEvalSamplesUnionResp) AsAuto() (v constant.Auto) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ReinforcementHyperparametersEvalSamplesUnionResp) AsInt() (v int64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ReinforcementHyperparametersEvalSamplesUnionResp) RawJSON() string { return u.JSON.raw }

func (r *ReinforcementHyperparametersEvalSamplesUnionResp) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ReinforcementHyperparametersLearningRateMultiplierUnionResp contains all
// possible properties and values from [constant.Auto], [float64].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfAuto OfFloat]
type ReinforcementHyperparametersLearningRateMultiplierUnionResp struct {
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

func (u ReinforcementHyperparametersLearningRateMultiplierUnionResp) AsAuto() (v constant.Auto) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ReinforcementHyperparametersLearningRateMultiplierUnionResp) AsFloat() (v float64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ReinforcementHyperparametersLearningRateMultiplierUnionResp) RawJSON() string {
	return u.JSON.raw
}

func (r *ReinforcementHyperparametersLearningRateMultiplierUnionResp) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ReinforcementHyperparametersNEpochsUnionResp contains all possible properties
// and values from [constant.Auto], [int64].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfAuto OfInt]
type ReinforcementHyperparametersNEpochsUnionResp struct {
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

func (u ReinforcementHyperparametersNEpochsUnionResp) AsAuto() (v constant.Auto) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ReinforcementHyperparametersNEpochsUnionResp) AsInt() (v int64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ReinforcementHyperparametersNEpochsUnionResp) RawJSON() string { return u.JSON.raw }

func (r *ReinforcementHyperparametersNEpochsUnionResp) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Level of reasoning effort.
type ReinforcementHyperparametersReasoningEffort string

const (
	ReinforcementHyperparametersReasoningEffortDefault ReinforcementHyperparametersReasoningEffort = "default"
	ReinforcementHyperparametersReasoningEffortLow     ReinforcementHyperparametersReasoningEffort = "low"
	ReinforcementHyperparametersReasoningEffortMedium  ReinforcementHyperparametersReasoningEffort = "medium"
	ReinforcementHyperparametersReasoningEffortHigh    ReinforcementHyperparametersReasoningEffort = "high"
)

// The hyperparameters used for the reinforcement fine-tuning job.
type ReinforcementHyperparameters struct {
	// Number of examples in each batch. A larger batch size means that model
	// parameters are updated less frequently, but with lower variance.
	BatchSize ReinforcementHyperparametersBatchSizeUnion `json:"batch_size,omitzero"`
	// Multiplier on amount of compute used for exploring search space during training.
	ComputeMultiplier ReinforcementHyperparametersComputeMultiplierUnion `json:"compute_multiplier,omitzero"`
	// The number of training steps between evaluation runs.
	EvalInterval ReinforcementHyperparametersEvalIntervalUnion `json:"eval_interval,omitzero"`
	// Number of evaluation samples to generate per training step.
	EvalSamples ReinforcementHyperparametersEvalSamplesUnion `json:"eval_samples,omitzero"`
	// Scaling factor for the learning rate. A smaller learning rate may be useful to
	// avoid overfitting.
	LearningRateMultiplier ReinforcementHyperparametersLearningRateMultiplierUnion `json:"learning_rate_multiplier,omitzero"`
	// The number of epochs to train the model for. An epoch refers to one full cycle
	// through the training dataset.
	NEpochs ReinforcementHyperparametersNEpochsUnion `json:"n_epochs,omitzero"`
	// Level of reasoning effort.
	//
	// Any of "default", "low", "medium", "high".
	ReasoningEffort ReinforcementHyperparametersReasoningEffort `json:"reasoning_effort,omitzero"`
	paramObj
}

func (r ReinforcementHyperparameters) MarshalJSON() (data []byte, err error) {
	type shadow ReinforcementHyperparameters
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ReinforcementHyperparameters) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ReinforcementHyperparametersBatchSizeUnion struct {
	// Construct this variant with constant.ValueOf[constant.Auto]()
	OfAuto constant.Auto    `json:",omitzero,inline"`
	OfInt  param.Opt[int64] `json:",omitzero,inline"`
	paramUnion
}

func (u ReinforcementHyperparametersBatchSizeUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfInt)
}
func (u *ReinforcementHyperparametersBatchSizeUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ReinforcementHyperparametersBatchSizeUnion) asAny() any {
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
type ReinforcementHyperparametersComputeMultiplierUnion struct {
	// Construct this variant with constant.ValueOf[constant.Auto]()
	OfAuto  constant.Auto      `json:",omitzero,inline"`
	OfFloat param.Opt[float64] `json:",omitzero,inline"`
	paramUnion
}

func (u ReinforcementHyperparametersComputeMultiplierUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfFloat)
}
func (u *ReinforcementHyperparametersComputeMultiplierUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ReinforcementHyperparametersComputeMultiplierUnion) asAny() any {
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
type ReinforcementHyperparametersEvalIntervalUnion struct {
	// Construct this variant with constant.ValueOf[constant.Auto]()
	OfAuto constant.Auto    `json:",omitzero,inline"`
	OfInt  param.Opt[int64] `json:",omitzero,inline"`
	paramUnion
}

func (u ReinforcementHyperparametersEvalIntervalUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfInt)
}
func (u *ReinforcementHyperparametersEvalIntervalUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ReinforcementHyperparametersEvalIntervalUnion) asAny() any {
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
type ReinforcementHyperparametersEvalSamplesUnion struct {
	// Construct this variant with constant.ValueOf[constant.Auto]()
	OfAuto constant.Auto    `json:",omitzero,inline"`
	OfInt  param.Opt[int64] `json:",omitzero,inline"`
	paramUnion
}

func (u ReinforcementHyperparametersEvalSamplesUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfInt)
}
func (u *ReinforcementHyperparametersEvalSamplesUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ReinforcementHyperparametersEvalSamplesUnion) asAny() any {
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
type ReinforcementHyperparametersLearningRateMultiplierUnion struct {
	// Construct this variant with constant.ValueOf[constant.Auto]()
	OfAuto  constant.Auto      `json:",omitzero,inline"`
	OfFloat param.Opt[float64] `json:",omitzero,inline"`
	paramUnion
}

func (u ReinforcementHyperparametersLearningRateMultiplierUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfFloat)
}
func (u *ReinforcementHyperparametersLearningRateMultiplierUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ReinforcementHyperparametersLearningRateMultiplierUnion) asAny() any {
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
type ReinforcementHyperparametersNEpochsUnion struct {
	// Construct this variant with constant.ValueOf[constant.Auto]()
	OfAuto constant.Auto    `json:",omitzero,inline"`
	OfInt  param.Opt[int64] `json:",omitzero,inline"`
	paramUnion
}

func (u ReinforcementHyperparametersNEpochsUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfInt)
}
func (u *ReinforcementHyperparametersNEpochsUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ReinforcementHyperparametersNEpochsUnion) asAny() any {
	if !param.IsOmitted(u.OfAuto) {
		return &u.OfAuto
	} else if !param.IsOmitted(u.OfInt) {
		return &u.OfInt.Value
	}
	return nil
}

// Configuration for the reinforcement fine-tuning method.
type ReinforcementMethod struct {
	// The grader used for the fine-tuning job.
	Grader ReinforcementMethodGraderUnion `json:"grader,required"`
	// The hyperparameters used for the reinforcement fine-tuning job.
	Hyperparameters ReinforcementHyperparametersResp `json:"hyperparameters"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Grader          respjson.Field
		Hyperparameters respjson.Field
		ExtraFields     map[string]respjson.Field
		raw             string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ReinforcementMethod) RawJSON() string { return r.JSON.raw }
func (r *ReinforcementMethod) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ReinforcementMethod to a ReinforcementMethodParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ReinforcementMethodParam.Overrides()
func (r ReinforcementMethod) ToParam() ReinforcementMethodParam {
	return param.Override[ReinforcementMethodParam](json.RawMessage(r.RawJSON()))
}

// ReinforcementMethodGraderUnion contains all possible properties and values from
// [StringCheckGrader], [TextSimilarityGrader], [PythonGrader], [ScoreModelGrader],
// [MultiGrader].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ReinforcementMethodGraderUnion struct {
	// This field is a union of [string], [string], [[]ScoreModelGraderInput]
	Input ReinforcementMethodGraderUnionInput `json:"input"`
	Name  string                              `json:"name"`
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

func (u ReinforcementMethodGraderUnion) AsStringCheckGrader() (v StringCheckGrader) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ReinforcementMethodGraderUnion) AsTextSimilarityGrader() (v TextSimilarityGrader) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ReinforcementMethodGraderUnion) AsPythonGrader() (v PythonGrader) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ReinforcementMethodGraderUnion) AsScoreModelGrader() (v ScoreModelGrader) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ReinforcementMethodGraderUnion) AsMultiGrader() (v MultiGrader) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ReinforcementMethodGraderUnion) RawJSON() string { return u.JSON.raw }

func (r *ReinforcementMethodGraderUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ReinforcementMethodGraderUnionInput is an implicit subunion of
// [ReinforcementMethodGraderUnion]. ReinforcementMethodGraderUnionInput provides
// convenient access to the sub-properties of the union.
//
// For type safety it is recommended to directly use a variant of the
// [ReinforcementMethodGraderUnion].
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfString OfScoreModelGraderInputArray]
type ReinforcementMethodGraderUnionInput struct {
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

func (r *ReinforcementMethodGraderUnionInput) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Configuration for the reinforcement fine-tuning method.
//
// The property Grader is required.
type ReinforcementMethodParam struct {
	// The grader used for the fine-tuning job.
	Grader ReinforcementMethodGraderUnionParam `json:"grader,omitzero,required"`
	// The hyperparameters used for the reinforcement fine-tuning job.
	Hyperparameters ReinforcementHyperparameters `json:"hyperparameters,omitzero"`
	paramObj
}

func (r ReinforcementMethodParam) MarshalJSON() (data []byte, err error) {
	type shadow ReinforcementMethodParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ReinforcementMethodParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ReinforcementMethodGraderUnionParam struct {
	OfStringCheckGrader    *StringCheckGraderParam    `json:",omitzero,inline"`
	OfTextSimilarityGrader *TextSimilarityGraderParam `json:",omitzero,inline"`
	OfPythonGrader         *PythonGraderParam         `json:",omitzero,inline"`
	OfScoreModelGrader     *ScoreModelGraderParam     `json:",omitzero,inline"`
	OfMultiGrader          *MultiGraderParam          `json:",omitzero,inline"`
	paramUnion
}

func (u ReinforcementMethodGraderUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfStringCheckGrader,
		u.OfTextSimilarityGrader,
		u.OfPythonGrader,
		u.OfScoreModelGrader,
		u.OfMultiGrader)
}
func (u *ReinforcementMethodGraderUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ReinforcementMethodGraderUnionParam) asAny() any {
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
func (u ReinforcementMethodGraderUnionParam) GetOperation() *string {
	if vt := u.OfStringCheckGrader; vt != nil {
		return (*string)(&vt.Operation)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ReinforcementMethodGraderUnionParam) GetEvaluationMetric() *string {
	if vt := u.OfTextSimilarityGrader; vt != nil {
		return (*string)(&vt.EvaluationMetric)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ReinforcementMethodGraderUnionParam) GetSource() *string {
	if vt := u.OfPythonGrader; vt != nil {
		return &vt.Source
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ReinforcementMethodGraderUnionParam) GetImageTag() *string {
	if vt := u.OfPythonGrader; vt != nil && vt.ImageTag.Valid() {
		return &vt.ImageTag.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ReinforcementMethodGraderUnionParam) GetModel() *string {
	if vt := u.OfScoreModelGrader; vt != nil {
		return &vt.Model
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ReinforcementMethodGraderUnionParam) GetRange() []float64 {
	if vt := u.OfScoreModelGrader; vt != nil {
		return vt.Range
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ReinforcementMethodGraderUnionParam) GetSamplingParams() *any {
	if vt := u.OfScoreModelGrader; vt != nil {
		return &vt.SamplingParams
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ReinforcementMethodGraderUnionParam) GetCalculateOutput() *string {
	if vt := u.OfMultiGrader; vt != nil {
		return &vt.CalculateOutput
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ReinforcementMethodGraderUnionParam) GetGraders() *MultiGraderGradersUnionParam {
	if vt := u.OfMultiGrader; vt != nil {
		return &vt.Graders
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ReinforcementMethodGraderUnionParam) GetName() *string {
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
func (u ReinforcementMethodGraderUnionParam) GetReference() *string {
	if vt := u.OfStringCheckGrader; vt != nil {
		return (*string)(&vt.Reference)
	} else if vt := u.OfTextSimilarityGrader; vt != nil {
		return (*string)(&vt.Reference)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ReinforcementMethodGraderUnionParam) GetType() *string {
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
func (u ReinforcementMethodGraderUnionParam) GetInput() (res reinforcementMethodGraderUnionParamInput) {
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
type reinforcementMethodGraderUnionParamInput struct{ any }

// Use the following switch statement to get the type of the union:
//
//	switch u.AsAny().(type) {
//	case *string:
//	case *[]openai.ScoreModelGraderInputParam:
//	default:
//	    fmt.Errorf("not present")
//	}
func (u reinforcementMethodGraderUnionParamInput) AsAny() any { return u.any }

// The hyperparameters used for the fine-tuning job.
type SupervisedHyperparametersResp struct {
	// Number of examples in each batch. A larger batch size means that model
	// parameters are updated less frequently, but with lower variance.
	BatchSize SupervisedHyperparametersBatchSizeUnionResp `json:"batch_size"`
	// Scaling factor for the learning rate. A smaller learning rate may be useful to
	// avoid overfitting.
	LearningRateMultiplier SupervisedHyperparametersLearningRateMultiplierUnionResp `json:"learning_rate_multiplier"`
	// The number of epochs to train the model for. An epoch refers to one full cycle
	// through the training dataset.
	NEpochs SupervisedHyperparametersNEpochsUnionResp `json:"n_epochs"`
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
func (r SupervisedHyperparametersResp) RawJSON() string { return r.JSON.raw }
func (r *SupervisedHyperparametersResp) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this SupervisedHyperparametersResp to a
// SupervisedHyperparameters.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// SupervisedHyperparameters.Overrides()
func (r SupervisedHyperparametersResp) ToParam() SupervisedHyperparameters {
	return param.Override[SupervisedHyperparameters](json.RawMessage(r.RawJSON()))
}

// SupervisedHyperparametersBatchSizeUnionResp contains all possible properties and
// values from [constant.Auto], [int64].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfAuto OfInt]
type SupervisedHyperparametersBatchSizeUnionResp struct {
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

func (u SupervisedHyperparametersBatchSizeUnionResp) AsAuto() (v constant.Auto) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u SupervisedHyperparametersBatchSizeUnionResp) AsInt() (v int64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u SupervisedHyperparametersBatchSizeUnionResp) RawJSON() string { return u.JSON.raw }

func (r *SupervisedHyperparametersBatchSizeUnionResp) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// SupervisedHyperparametersLearningRateMultiplierUnionResp contains all possible
// properties and values from [constant.Auto], [float64].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfAuto OfFloat]
type SupervisedHyperparametersLearningRateMultiplierUnionResp struct {
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

func (u SupervisedHyperparametersLearningRateMultiplierUnionResp) AsAuto() (v constant.Auto) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u SupervisedHyperparametersLearningRateMultiplierUnionResp) AsFloat() (v float64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u SupervisedHyperparametersLearningRateMultiplierUnionResp) RawJSON() string { return u.JSON.raw }

func (r *SupervisedHyperparametersLearningRateMultiplierUnionResp) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// SupervisedHyperparametersNEpochsUnionResp contains all possible properties and
// values from [constant.Auto], [int64].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfAuto OfInt]
type SupervisedHyperparametersNEpochsUnionResp struct {
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

func (u SupervisedHyperparametersNEpochsUnionResp) AsAuto() (v constant.Auto) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u SupervisedHyperparametersNEpochsUnionResp) AsInt() (v int64) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u SupervisedHyperparametersNEpochsUnionResp) RawJSON() string { return u.JSON.raw }

func (r *SupervisedHyperparametersNEpochsUnionResp) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The hyperparameters used for the fine-tuning job.
type SupervisedHyperparameters struct {
	// Number of examples in each batch. A larger batch size means that model
	// parameters are updated less frequently, but with lower variance.
	BatchSize SupervisedHyperparametersBatchSizeUnion `json:"batch_size,omitzero"`
	// Scaling factor for the learning rate. A smaller learning rate may be useful to
	// avoid overfitting.
	LearningRateMultiplier SupervisedHyperparametersLearningRateMultiplierUnion `json:"learning_rate_multiplier,omitzero"`
	// The number of epochs to train the model for. An epoch refers to one full cycle
	// through the training dataset.
	NEpochs SupervisedHyperparametersNEpochsUnion `json:"n_epochs,omitzero"`
	paramObj
}

func (r SupervisedHyperparameters) MarshalJSON() (data []byte, err error) {
	type shadow SupervisedHyperparameters
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *SupervisedHyperparameters) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type SupervisedHyperparametersBatchSizeUnion struct {
	// Construct this variant with constant.ValueOf[constant.Auto]()
	OfAuto constant.Auto    `json:",omitzero,inline"`
	OfInt  param.Opt[int64] `json:",omitzero,inline"`
	paramUnion
}

func (u SupervisedHyperparametersBatchSizeUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfInt)
}
func (u *SupervisedHyperparametersBatchSizeUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *SupervisedHyperparametersBatchSizeUnion) asAny() any {
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
type SupervisedHyperparametersLearningRateMultiplierUnion struct {
	// Construct this variant with constant.ValueOf[constant.Auto]()
	OfAuto  constant.Auto      `json:",omitzero,inline"`
	OfFloat param.Opt[float64] `json:",omitzero,inline"`
	paramUnion
}

func (u SupervisedHyperparametersLearningRateMultiplierUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfFloat)
}
func (u *SupervisedHyperparametersLearningRateMultiplierUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *SupervisedHyperparametersLearningRateMultiplierUnion) asAny() any {
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
type SupervisedHyperparametersNEpochsUnion struct {
	// Construct this variant with constant.ValueOf[constant.Auto]()
	OfAuto constant.Auto    `json:",omitzero,inline"`
	OfInt  param.Opt[int64] `json:",omitzero,inline"`
	paramUnion
}

func (u SupervisedHyperparametersNEpochsUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfInt)
}
func (u *SupervisedHyperparametersNEpochsUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *SupervisedHyperparametersNEpochsUnion) asAny() any {
	if !param.IsOmitted(u.OfAuto) {
		return &u.OfAuto
	} else if !param.IsOmitted(u.OfInt) {
		return &u.OfInt.Value
	}
	return nil
}

// Configuration for the supervised fine-tuning method.
type SupervisedMethod struct {
	// The hyperparameters used for the fine-tuning job.
	Hyperparameters SupervisedHyperparametersResp `json:"hyperparameters"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Hyperparameters respjson.Field
		ExtraFields     map[string]respjson.Field
		raw             string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r SupervisedMethod) RawJSON() string { return r.JSON.raw }
func (r *SupervisedMethod) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this SupervisedMethod to a SupervisedMethodParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// SupervisedMethodParam.Overrides()
func (r SupervisedMethod) ToParam() SupervisedMethodParam {
	return param.Override[SupervisedMethodParam](json.RawMessage(r.RawJSON()))
}

// Configuration for the supervised fine-tuning method.
type SupervisedMethodParam struct {
	// The hyperparameters used for the fine-tuning job.
	Hyperparameters SupervisedHyperparameters `json:"hyperparameters,omitzero"`
	paramObj
}

func (r SupervisedMethodParam) MarshalJSON() (data []byte, err error) {
	type shadow SupervisedMethodParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *SupervisedMethodParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}
