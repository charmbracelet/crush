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
	"github.com/openai/openai-go/shared/constant"
)

// ModerationService contains methods and other services that help with interacting
// with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewModerationService] method instead.
type ModerationService struct {
	Options []option.RequestOption
}

// NewModerationService generates a new service that applies the given options to
// each request. These options are applied after the parent client's options (if
// there is one), and before any request-specific options.
func NewModerationService(opts ...option.RequestOption) (r ModerationService) {
	r = ModerationService{}
	r.Options = opts
	return
}

// Classifies if text and/or image inputs are potentially harmful. Learn more in
// the [moderation guide](https://platform.openai.com/docs/guides/moderation).
func (r *ModerationService) New(ctx context.Context, body ModerationNewParams, opts ...option.RequestOption) (res *ModerationNewResponse, err error) {
	opts = append(r.Options[:], opts...)
	path := "moderations"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

type Moderation struct {
	// A list of the categories, and whether they are flagged or not.
	Categories ModerationCategories `json:"categories,required"`
	// A list of the categories along with the input type(s) that the score applies to.
	CategoryAppliedInputTypes ModerationCategoryAppliedInputTypes `json:"category_applied_input_types,required"`
	// A list of the categories along with their scores as predicted by model.
	CategoryScores ModerationCategoryScores `json:"category_scores,required"`
	// Whether any of the below categories are flagged.
	Flagged bool `json:"flagged,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Categories                respjson.Field
		CategoryAppliedInputTypes respjson.Field
		CategoryScores            respjson.Field
		Flagged                   respjson.Field
		ExtraFields               map[string]respjson.Field
		raw                       string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r Moderation) RawJSON() string { return r.JSON.raw }
func (r *Moderation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A list of the categories, and whether they are flagged or not.
type ModerationCategories struct {
	// Content that expresses, incites, or promotes harassing language towards any
	// target.
	Harassment bool `json:"harassment,required"`
	// Harassment content that also includes violence or serious harm towards any
	// target.
	HarassmentThreatening bool `json:"harassment/threatening,required"`
	// Content that expresses, incites, or promotes hate based on race, gender,
	// ethnicity, religion, nationality, sexual orientation, disability status, or
	// caste. Hateful content aimed at non-protected groups (e.g., chess players) is
	// harassment.
	Hate bool `json:"hate,required"`
	// Hateful content that also includes violence or serious harm towards the targeted
	// group based on race, gender, ethnicity, religion, nationality, sexual
	// orientation, disability status, or caste.
	HateThreatening bool `json:"hate/threatening,required"`
	// Content that includes instructions or advice that facilitate the planning or
	// execution of wrongdoing, or that gives advice or instruction on how to commit
	// illicit acts. For example, "how to shoplift" would fit this category.
	Illicit bool `json:"illicit,required"`
	// Content that includes instructions or advice that facilitate the planning or
	// execution of wrongdoing that also includes violence, or that gives advice or
	// instruction on the procurement of any weapon.
	IllicitViolent bool `json:"illicit/violent,required"`
	// Content that promotes, encourages, or depicts acts of self-harm, such as
	// suicide, cutting, and eating disorders.
	SelfHarm bool `json:"self-harm,required"`
	// Content that encourages performing acts of self-harm, such as suicide, cutting,
	// and eating disorders, or that gives instructions or advice on how to commit such
	// acts.
	SelfHarmInstructions bool `json:"self-harm/instructions,required"`
	// Content where the speaker expresses that they are engaging or intend to engage
	// in acts of self-harm, such as suicide, cutting, and eating disorders.
	SelfHarmIntent bool `json:"self-harm/intent,required"`
	// Content meant to arouse sexual excitement, such as the description of sexual
	// activity, or that promotes sexual services (excluding sex education and
	// wellness).
	Sexual bool `json:"sexual,required"`
	// Sexual content that includes an individual who is under 18 years old.
	SexualMinors bool `json:"sexual/minors,required"`
	// Content that depicts death, violence, or physical injury.
	Violence bool `json:"violence,required"`
	// Content that depicts death, violence, or physical injury in graphic detail.
	ViolenceGraphic bool `json:"violence/graphic,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Harassment            respjson.Field
		HarassmentThreatening respjson.Field
		Hate                  respjson.Field
		HateThreatening       respjson.Field
		Illicit               respjson.Field
		IllicitViolent        respjson.Field
		SelfHarm              respjson.Field
		SelfHarmInstructions  respjson.Field
		SelfHarmIntent        respjson.Field
		Sexual                respjson.Field
		SexualMinors          respjson.Field
		Violence              respjson.Field
		ViolenceGraphic       respjson.Field
		ExtraFields           map[string]respjson.Field
		raw                   string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ModerationCategories) RawJSON() string { return r.JSON.raw }
func (r *ModerationCategories) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A list of the categories along with the input type(s) that the score applies to.
type ModerationCategoryAppliedInputTypes struct {
	// The applied input type(s) for the category 'harassment'.
	//
	// Any of "text".
	Harassment []string `json:"harassment,required"`
	// The applied input type(s) for the category 'harassment/threatening'.
	//
	// Any of "text".
	HarassmentThreatening []string `json:"harassment/threatening,required"`
	// The applied input type(s) for the category 'hate'.
	//
	// Any of "text".
	Hate []string `json:"hate,required"`
	// The applied input type(s) for the category 'hate/threatening'.
	//
	// Any of "text".
	HateThreatening []string `json:"hate/threatening,required"`
	// The applied input type(s) for the category 'illicit'.
	//
	// Any of "text".
	Illicit []string `json:"illicit,required"`
	// The applied input type(s) for the category 'illicit/violent'.
	//
	// Any of "text".
	IllicitViolent []string `json:"illicit/violent,required"`
	// The applied input type(s) for the category 'self-harm'.
	//
	// Any of "text", "image".
	SelfHarm []string `json:"self-harm,required"`
	// The applied input type(s) for the category 'self-harm/instructions'.
	//
	// Any of "text", "image".
	SelfHarmInstructions []string `json:"self-harm/instructions,required"`
	// The applied input type(s) for the category 'self-harm/intent'.
	//
	// Any of "text", "image".
	SelfHarmIntent []string `json:"self-harm/intent,required"`
	// The applied input type(s) for the category 'sexual'.
	//
	// Any of "text", "image".
	Sexual []string `json:"sexual,required"`
	// The applied input type(s) for the category 'sexual/minors'.
	//
	// Any of "text".
	SexualMinors []string `json:"sexual/minors,required"`
	// The applied input type(s) for the category 'violence'.
	//
	// Any of "text", "image".
	Violence []string `json:"violence,required"`
	// The applied input type(s) for the category 'violence/graphic'.
	//
	// Any of "text", "image".
	ViolenceGraphic []string `json:"violence/graphic,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Harassment            respjson.Field
		HarassmentThreatening respjson.Field
		Hate                  respjson.Field
		HateThreatening       respjson.Field
		Illicit               respjson.Field
		IllicitViolent        respjson.Field
		SelfHarm              respjson.Field
		SelfHarmInstructions  respjson.Field
		SelfHarmIntent        respjson.Field
		Sexual                respjson.Field
		SexualMinors          respjson.Field
		Violence              respjson.Field
		ViolenceGraphic       respjson.Field
		ExtraFields           map[string]respjson.Field
		raw                   string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ModerationCategoryAppliedInputTypes) RawJSON() string { return r.JSON.raw }
func (r *ModerationCategoryAppliedInputTypes) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A list of the categories along with their scores as predicted by model.
type ModerationCategoryScores struct {
	// The score for the category 'harassment'.
	Harassment float64 `json:"harassment,required"`
	// The score for the category 'harassment/threatening'.
	HarassmentThreatening float64 `json:"harassment/threatening,required"`
	// The score for the category 'hate'.
	Hate float64 `json:"hate,required"`
	// The score for the category 'hate/threatening'.
	HateThreatening float64 `json:"hate/threatening,required"`
	// The score for the category 'illicit'.
	Illicit float64 `json:"illicit,required"`
	// The score for the category 'illicit/violent'.
	IllicitViolent float64 `json:"illicit/violent,required"`
	// The score for the category 'self-harm'.
	SelfHarm float64 `json:"self-harm,required"`
	// The score for the category 'self-harm/instructions'.
	SelfHarmInstructions float64 `json:"self-harm/instructions,required"`
	// The score for the category 'self-harm/intent'.
	SelfHarmIntent float64 `json:"self-harm/intent,required"`
	// The score for the category 'sexual'.
	Sexual float64 `json:"sexual,required"`
	// The score for the category 'sexual/minors'.
	SexualMinors float64 `json:"sexual/minors,required"`
	// The score for the category 'violence'.
	Violence float64 `json:"violence,required"`
	// The score for the category 'violence/graphic'.
	ViolenceGraphic float64 `json:"violence/graphic,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Harassment            respjson.Field
		HarassmentThreatening respjson.Field
		Hate                  respjson.Field
		HateThreatening       respjson.Field
		Illicit               respjson.Field
		IllicitViolent        respjson.Field
		SelfHarm              respjson.Field
		SelfHarmInstructions  respjson.Field
		SelfHarmIntent        respjson.Field
		Sexual                respjson.Field
		SexualMinors          respjson.Field
		Violence              respjson.Field
		ViolenceGraphic       respjson.Field
		ExtraFields           map[string]respjson.Field
		raw                   string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ModerationCategoryScores) RawJSON() string { return r.JSON.raw }
func (r *ModerationCategoryScores) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// An object describing an image to classify.
//
// The properties ImageURL, Type are required.
type ModerationImageURLInputParam struct {
	// Contains either an image URL or a data URL for a base64 encoded image.
	ImageURL ModerationImageURLInputImageURLParam `json:"image_url,omitzero,required"`
	// Always `image_url`.
	//
	// This field can be elided, and will marshal its zero value as "image_url".
	Type constant.ImageURL `json:"type,required"`
	paramObj
}

func (r ModerationImageURLInputParam) MarshalJSON() (data []byte, err error) {
	type shadow ModerationImageURLInputParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ModerationImageURLInputParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Contains either an image URL or a data URL for a base64 encoded image.
//
// The property URL is required.
type ModerationImageURLInputImageURLParam struct {
	// Either a URL of the image or the base64 encoded image data.
	URL string `json:"url,required" format:"uri"`
	paramObj
}

func (r ModerationImageURLInputImageURLParam) MarshalJSON() (data []byte, err error) {
	type shadow ModerationImageURLInputImageURLParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ModerationImageURLInputImageURLParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ModerationModel = string

const (
	ModerationModelOmniModerationLatest     ModerationModel = "omni-moderation-latest"
	ModerationModelOmniModeration2024_09_26 ModerationModel = "omni-moderation-2024-09-26"
	ModerationModelTextModerationLatest     ModerationModel = "text-moderation-latest"
	ModerationModelTextModerationStable     ModerationModel = "text-moderation-stable"
)

func ModerationMultiModalInputParamOfImageURL(imageURL ModerationImageURLInputImageURLParam) ModerationMultiModalInputUnionParam {
	var variant ModerationImageURLInputParam
	variant.ImageURL = imageURL
	return ModerationMultiModalInputUnionParam{OfImageURL: &variant}
}

func ModerationMultiModalInputParamOfText(text string) ModerationMultiModalInputUnionParam {
	var variant ModerationTextInputParam
	variant.Text = text
	return ModerationMultiModalInputUnionParam{OfText: &variant}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ModerationMultiModalInputUnionParam struct {
	OfImageURL *ModerationImageURLInputParam `json:",omitzero,inline"`
	OfText     *ModerationTextInputParam     `json:",omitzero,inline"`
	paramUnion
}

func (u ModerationMultiModalInputUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfImageURL, u.OfText)
}
func (u *ModerationMultiModalInputUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ModerationMultiModalInputUnionParam) asAny() any {
	if !param.IsOmitted(u.OfImageURL) {
		return u.OfImageURL
	} else if !param.IsOmitted(u.OfText) {
		return u.OfText
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ModerationMultiModalInputUnionParam) GetImageURL() *ModerationImageURLInputImageURLParam {
	if vt := u.OfImageURL; vt != nil {
		return &vt.ImageURL
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ModerationMultiModalInputUnionParam) GetText() *string {
	if vt := u.OfText; vt != nil {
		return &vt.Text
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ModerationMultiModalInputUnionParam) GetType() *string {
	if vt := u.OfImageURL; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfText; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[ModerationMultiModalInputUnionParam](
		"type",
		apijson.Discriminator[ModerationImageURLInputParam]("image_url"),
		apijson.Discriminator[ModerationTextInputParam]("text"),
	)
}

// An object describing text to classify.
//
// The properties Text, Type are required.
type ModerationTextInputParam struct {
	// A string of text to classify.
	Text string `json:"text,required"`
	// Always `text`.
	//
	// This field can be elided, and will marshal its zero value as "text".
	Type constant.Text `json:"type,required"`
	paramObj
}

func (r ModerationTextInputParam) MarshalJSON() (data []byte, err error) {
	type shadow ModerationTextInputParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ModerationTextInputParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Represents if a given text input is potentially harmful.
type ModerationNewResponse struct {
	// The unique identifier for the moderation request.
	ID string `json:"id,required"`
	// The model used to generate the moderation results.
	Model string `json:"model,required"`
	// A list of moderation objects.
	Results []Moderation `json:"results,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Model       respjson.Field
		Results     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ModerationNewResponse) RawJSON() string { return r.JSON.raw }
func (r *ModerationNewResponse) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ModerationNewParams struct {
	// Input (or inputs) to classify. Can be a single string, an array of strings, or
	// an array of multi-modal input objects similar to other models.
	Input ModerationNewParamsInputUnion `json:"input,omitzero,required"`
	// The content moderation model you would like to use. Learn more in
	// [the moderation guide](https://platform.openai.com/docs/guides/moderation), and
	// learn about available models
	// [here](https://platform.openai.com/docs/models#moderation).
	Model ModerationModel `json:"model,omitzero"`
	paramObj
}

func (r ModerationNewParams) MarshalJSON() (data []byte, err error) {
	type shadow ModerationNewParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ModerationNewParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ModerationNewParamsInputUnion struct {
	OfString                    param.Opt[string]                     `json:",omitzero,inline"`
	OfStringArray               []string                              `json:",omitzero,inline"`
	OfModerationMultiModalArray []ModerationMultiModalInputUnionParam `json:",omitzero,inline"`
	paramUnion
}

func (u ModerationNewParamsInputUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfStringArray, u.OfModerationMultiModalArray)
}
func (u *ModerationNewParamsInputUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ModerationNewParamsInputUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfStringArray) {
		return &u.OfStringArray
	} else if !param.IsOmitted(u.OfModerationMultiModalArray) {
		return &u.OfModerationMultiModalArray
	}
	return nil
}
