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

// BetaThreadMessageService contains methods and other services that help with
// interacting with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewBetaThreadMessageService] method instead.
//
// Deprecated: The Assistants API is deprecated in favor of the Responses API
type BetaThreadMessageService struct {
	Options []option.RequestOption
}

// NewBetaThreadMessageService generates a new service that applies the given
// options to each request. These options are applied after the parent client's
// options (if there is one), and before any request-specific options.
func NewBetaThreadMessageService(opts ...option.RequestOption) (r BetaThreadMessageService) {
	r = BetaThreadMessageService{}
	r.Options = opts
	return
}

// Create a message.
//
// Deprecated: The Assistants API is deprecated in favor of the Responses API
func (r *BetaThreadMessageService) New(ctx context.Context, threadID string, body BetaThreadMessageNewParams, opts ...option.RequestOption) (res *Message, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	if threadID == "" {
		err = errors.New("missing required thread_id parameter")
		return
	}
	path := fmt.Sprintf("threads/%s/messages", threadID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// Retrieve a message.
//
// Deprecated: The Assistants API is deprecated in favor of the Responses API
func (r *BetaThreadMessageService) Get(ctx context.Context, threadID string, messageID string, opts ...option.RequestOption) (res *Message, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	if threadID == "" {
		err = errors.New("missing required thread_id parameter")
		return
	}
	if messageID == "" {
		err = errors.New("missing required message_id parameter")
		return
	}
	path := fmt.Sprintf("threads/%s/messages/%s", threadID, messageID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// Modifies a message.
//
// Deprecated: The Assistants API is deprecated in favor of the Responses API
func (r *BetaThreadMessageService) Update(ctx context.Context, threadID string, messageID string, body BetaThreadMessageUpdateParams, opts ...option.RequestOption) (res *Message, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	if threadID == "" {
		err = errors.New("missing required thread_id parameter")
		return
	}
	if messageID == "" {
		err = errors.New("missing required message_id parameter")
		return
	}
	path := fmt.Sprintf("threads/%s/messages/%s", threadID, messageID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// Returns a list of messages for a given thread.
//
// Deprecated: The Assistants API is deprecated in favor of the Responses API
func (r *BetaThreadMessageService) List(ctx context.Context, threadID string, query BetaThreadMessageListParams, opts ...option.RequestOption) (res *pagination.CursorPage[Message], err error) {
	var raw *http.Response
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2"), option.WithResponseInto(&raw)}, opts...)
	if threadID == "" {
		err = errors.New("missing required thread_id parameter")
		return
	}
	path := fmt.Sprintf("threads/%s/messages", threadID)
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

// Returns a list of messages for a given thread.
//
// Deprecated: The Assistants API is deprecated in favor of the Responses API
func (r *BetaThreadMessageService) ListAutoPaging(ctx context.Context, threadID string, query BetaThreadMessageListParams, opts ...option.RequestOption) *pagination.CursorPageAutoPager[Message] {
	return pagination.NewCursorPageAutoPager(r.List(ctx, threadID, query, opts...))
}

// Deletes a message.
//
// Deprecated: The Assistants API is deprecated in favor of the Responses API
func (r *BetaThreadMessageService) Delete(ctx context.Context, threadID string, messageID string, opts ...option.RequestOption) (res *MessageDeleted, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	if threadID == "" {
		err = errors.New("missing required thread_id parameter")
		return
	}
	if messageID == "" {
		err = errors.New("missing required message_id parameter")
		return
	}
	path := fmt.Sprintf("threads/%s/messages/%s", threadID, messageID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodDelete, path, nil, &res, opts...)
	return
}

// AnnotationUnion contains all possible properties and values from
// [FileCitationAnnotation], [FilePathAnnotation].
//
// Use the [AnnotationUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type AnnotationUnion struct {
	EndIndex int64 `json:"end_index"`
	// This field is from variant [FileCitationAnnotation].
	FileCitation FileCitationAnnotationFileCitation `json:"file_citation"`
	StartIndex   int64                              `json:"start_index"`
	Text         string                             `json:"text"`
	// Any of "file_citation", "file_path".
	Type string `json:"type"`
	// This field is from variant [FilePathAnnotation].
	FilePath FilePathAnnotationFilePath `json:"file_path"`
	JSON     struct {
		EndIndex     respjson.Field
		FileCitation respjson.Field
		StartIndex   respjson.Field
		Text         respjson.Field
		Type         respjson.Field
		FilePath     respjson.Field
		raw          string
	} `json:"-"`
}

// anyAnnotation is implemented by each variant of [AnnotationUnion] to add type
// safety for the return type of [AnnotationUnion.AsAny]
type anyAnnotation interface {
	implAnnotationUnion()
}

func (FileCitationAnnotation) implAnnotationUnion() {}
func (FilePathAnnotation) implAnnotationUnion()     {}

// Use the following switch statement to find the correct variant
//
//	switch variant := AnnotationUnion.AsAny().(type) {
//	case openai.FileCitationAnnotation:
//	case openai.FilePathAnnotation:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u AnnotationUnion) AsAny() anyAnnotation {
	switch u.Type {
	case "file_citation":
		return u.AsFileCitation()
	case "file_path":
		return u.AsFilePath()
	}
	return nil
}

func (u AnnotationUnion) AsFileCitation() (v FileCitationAnnotation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AnnotationUnion) AsFilePath() (v FilePathAnnotation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u AnnotationUnion) RawJSON() string { return u.JSON.raw }

func (r *AnnotationUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// AnnotationDeltaUnion contains all possible properties and values from
// [FileCitationDeltaAnnotation], [FilePathDeltaAnnotation].
//
// Use the [AnnotationDeltaUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type AnnotationDeltaUnion struct {
	Index int64 `json:"index"`
	// Any of "file_citation", "file_path".
	Type     string `json:"type"`
	EndIndex int64  `json:"end_index"`
	// This field is from variant [FileCitationDeltaAnnotation].
	FileCitation FileCitationDeltaAnnotationFileCitation `json:"file_citation"`
	StartIndex   int64                                   `json:"start_index"`
	Text         string                                  `json:"text"`
	// This field is from variant [FilePathDeltaAnnotation].
	FilePath FilePathDeltaAnnotationFilePath `json:"file_path"`
	JSON     struct {
		Index        respjson.Field
		Type         respjson.Field
		EndIndex     respjson.Field
		FileCitation respjson.Field
		StartIndex   respjson.Field
		Text         respjson.Field
		FilePath     respjson.Field
		raw          string
	} `json:"-"`
}

// anyAnnotationDelta is implemented by each variant of [AnnotationDeltaUnion] to
// add type safety for the return type of [AnnotationDeltaUnion.AsAny]
type anyAnnotationDelta interface {
	implAnnotationDeltaUnion()
}

func (FileCitationDeltaAnnotation) implAnnotationDeltaUnion() {}
func (FilePathDeltaAnnotation) implAnnotationDeltaUnion()     {}

// Use the following switch statement to find the correct variant
//
//	switch variant := AnnotationDeltaUnion.AsAny().(type) {
//	case openai.FileCitationDeltaAnnotation:
//	case openai.FilePathDeltaAnnotation:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u AnnotationDeltaUnion) AsAny() anyAnnotationDelta {
	switch u.Type {
	case "file_citation":
		return u.AsFileCitation()
	case "file_path":
		return u.AsFilePath()
	}
	return nil
}

func (u AnnotationDeltaUnion) AsFileCitation() (v FileCitationDeltaAnnotation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u AnnotationDeltaUnion) AsFilePath() (v FilePathDeltaAnnotation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u AnnotationDeltaUnion) RawJSON() string { return u.JSON.raw }

func (r *AnnotationDeltaUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A citation within the message that points to a specific quote from a specific
// File associated with the assistant or the message. Generated when the assistant
// uses the "file_search" tool to search files.
type FileCitationAnnotation struct {
	EndIndex     int64                              `json:"end_index,required"`
	FileCitation FileCitationAnnotationFileCitation `json:"file_citation,required"`
	StartIndex   int64                              `json:"start_index,required"`
	// The text in the message content that needs to be replaced.
	Text string `json:"text,required"`
	// Always `file_citation`.
	Type constant.FileCitation `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		EndIndex     respjson.Field
		FileCitation respjson.Field
		StartIndex   respjson.Field
		Text         respjson.Field
		Type         respjson.Field
		ExtraFields  map[string]respjson.Field
		raw          string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FileCitationAnnotation) RawJSON() string { return r.JSON.raw }
func (r *FileCitationAnnotation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type FileCitationAnnotationFileCitation struct {
	// The ID of the specific File the citation is from.
	FileID string `json:"file_id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		FileID      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FileCitationAnnotationFileCitation) RawJSON() string { return r.JSON.raw }
func (r *FileCitationAnnotationFileCitation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A citation within the message that points to a specific quote from a specific
// File associated with the assistant or the message. Generated when the assistant
// uses the "file_search" tool to search files.
type FileCitationDeltaAnnotation struct {
	// The index of the annotation in the text content part.
	Index int64 `json:"index,required"`
	// Always `file_citation`.
	Type         constant.FileCitation                   `json:"type,required"`
	EndIndex     int64                                   `json:"end_index"`
	FileCitation FileCitationDeltaAnnotationFileCitation `json:"file_citation"`
	StartIndex   int64                                   `json:"start_index"`
	// The text in the message content that needs to be replaced.
	Text string `json:"text"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Index        respjson.Field
		Type         respjson.Field
		EndIndex     respjson.Field
		FileCitation respjson.Field
		StartIndex   respjson.Field
		Text         respjson.Field
		ExtraFields  map[string]respjson.Field
		raw          string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FileCitationDeltaAnnotation) RawJSON() string { return r.JSON.raw }
func (r *FileCitationDeltaAnnotation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type FileCitationDeltaAnnotationFileCitation struct {
	// The ID of the specific File the citation is from.
	FileID string `json:"file_id"`
	// The specific quote in the file.
	Quote string `json:"quote"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		FileID      respjson.Field
		Quote       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FileCitationDeltaAnnotationFileCitation) RawJSON() string { return r.JSON.raw }
func (r *FileCitationDeltaAnnotationFileCitation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A URL for the file that's generated when the assistant used the
// `code_interpreter` tool to generate a file.
type FilePathAnnotation struct {
	EndIndex   int64                      `json:"end_index,required"`
	FilePath   FilePathAnnotationFilePath `json:"file_path,required"`
	StartIndex int64                      `json:"start_index,required"`
	// The text in the message content that needs to be replaced.
	Text string `json:"text,required"`
	// Always `file_path`.
	Type constant.FilePath `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		EndIndex    respjson.Field
		FilePath    respjson.Field
		StartIndex  respjson.Field
		Text        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FilePathAnnotation) RawJSON() string { return r.JSON.raw }
func (r *FilePathAnnotation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type FilePathAnnotationFilePath struct {
	// The ID of the file that was generated.
	FileID string `json:"file_id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		FileID      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FilePathAnnotationFilePath) RawJSON() string { return r.JSON.raw }
func (r *FilePathAnnotationFilePath) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A URL for the file that's generated when the assistant used the
// `code_interpreter` tool to generate a file.
type FilePathDeltaAnnotation struct {
	// The index of the annotation in the text content part.
	Index int64 `json:"index,required"`
	// Always `file_path`.
	Type       constant.FilePath               `json:"type,required"`
	EndIndex   int64                           `json:"end_index"`
	FilePath   FilePathDeltaAnnotationFilePath `json:"file_path"`
	StartIndex int64                           `json:"start_index"`
	// The text in the message content that needs to be replaced.
	Text string `json:"text"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Index       respjson.Field
		Type        respjson.Field
		EndIndex    respjson.Field
		FilePath    respjson.Field
		StartIndex  respjson.Field
		Text        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FilePathDeltaAnnotation) RawJSON() string { return r.JSON.raw }
func (r *FilePathDeltaAnnotation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type FilePathDeltaAnnotationFilePath struct {
	// The ID of the file that was generated.
	FileID string `json:"file_id"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		FileID      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FilePathDeltaAnnotationFilePath) RawJSON() string { return r.JSON.raw }
func (r *FilePathDeltaAnnotationFilePath) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ImageFile struct {
	// The [File](https://platform.openai.com/docs/api-reference/files) ID of the image
	// in the message content. Set `purpose="vision"` when uploading the File if you
	// need to later display the file content.
	FileID string `json:"file_id,required"`
	// Specifies the detail level of the image if specified by the user. `low` uses
	// fewer tokens, you can opt in to high resolution using `high`.
	//
	// Any of "auto", "low", "high".
	Detail ImageFileDetail `json:"detail"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		FileID      respjson.Field
		Detail      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ImageFile) RawJSON() string { return r.JSON.raw }
func (r *ImageFile) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ImageFile to a ImageFileParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ImageFileParam.Overrides()
func (r ImageFile) ToParam() ImageFileParam {
	return param.Override[ImageFileParam](json.RawMessage(r.RawJSON()))
}

// Specifies the detail level of the image if specified by the user. `low` uses
// fewer tokens, you can opt in to high resolution using `high`.
type ImageFileDetail string

const (
	ImageFileDetailAuto ImageFileDetail = "auto"
	ImageFileDetailLow  ImageFileDetail = "low"
	ImageFileDetailHigh ImageFileDetail = "high"
)

// The property FileID is required.
type ImageFileParam struct {
	// The [File](https://platform.openai.com/docs/api-reference/files) ID of the image
	// in the message content. Set `purpose="vision"` when uploading the File if you
	// need to later display the file content.
	FileID string `json:"file_id,required"`
	// Specifies the detail level of the image if specified by the user. `low` uses
	// fewer tokens, you can opt in to high resolution using `high`.
	//
	// Any of "auto", "low", "high".
	Detail ImageFileDetail `json:"detail,omitzero"`
	paramObj
}

func (r ImageFileParam) MarshalJSON() (data []byte, err error) {
	type shadow ImageFileParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ImageFileParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// References an image [File](https://platform.openai.com/docs/api-reference/files)
// in the content of a message.
type ImageFileContentBlock struct {
	ImageFile ImageFile `json:"image_file,required"`
	// Always `image_file`.
	Type constant.ImageFile `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ImageFile   respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ImageFileContentBlock) RawJSON() string { return r.JSON.raw }
func (r *ImageFileContentBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ImageFileContentBlock to a ImageFileContentBlockParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ImageFileContentBlockParam.Overrides()
func (r ImageFileContentBlock) ToParam() ImageFileContentBlockParam {
	return param.Override[ImageFileContentBlockParam](json.RawMessage(r.RawJSON()))
}

// References an image [File](https://platform.openai.com/docs/api-reference/files)
// in the content of a message.
//
// The properties ImageFile, Type are required.
type ImageFileContentBlockParam struct {
	ImageFile ImageFileParam `json:"image_file,omitzero,required"`
	// Always `image_file`.
	//
	// This field can be elided, and will marshal its zero value as "image_file".
	Type constant.ImageFile `json:"type,required"`
	paramObj
}

func (r ImageFileContentBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow ImageFileContentBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ImageFileContentBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ImageFileDelta struct {
	// Specifies the detail level of the image if specified by the user. `low` uses
	// fewer tokens, you can opt in to high resolution using `high`.
	//
	// Any of "auto", "low", "high".
	Detail ImageFileDeltaDetail `json:"detail"`
	// The [File](https://platform.openai.com/docs/api-reference/files) ID of the image
	// in the message content. Set `purpose="vision"` when uploading the File if you
	// need to later display the file content.
	FileID string `json:"file_id"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Detail      respjson.Field
		FileID      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ImageFileDelta) RawJSON() string { return r.JSON.raw }
func (r *ImageFileDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Specifies the detail level of the image if specified by the user. `low` uses
// fewer tokens, you can opt in to high resolution using `high`.
type ImageFileDeltaDetail string

const (
	ImageFileDeltaDetailAuto ImageFileDeltaDetail = "auto"
	ImageFileDeltaDetailLow  ImageFileDeltaDetail = "low"
	ImageFileDeltaDetailHigh ImageFileDeltaDetail = "high"
)

// References an image [File](https://platform.openai.com/docs/api-reference/files)
// in the content of a message.
type ImageFileDeltaBlock struct {
	// The index of the content part in the message.
	Index int64 `json:"index,required"`
	// Always `image_file`.
	Type      constant.ImageFile `json:"type,required"`
	ImageFile ImageFileDelta     `json:"image_file"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Index       respjson.Field
		Type        respjson.Field
		ImageFile   respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ImageFileDeltaBlock) RawJSON() string { return r.JSON.raw }
func (r *ImageFileDeltaBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ImageURL struct {
	// The external URL of the image, must be a supported image types: jpeg, jpg, png,
	// gif, webp.
	URL string `json:"url,required" format:"uri"`
	// Specifies the detail level of the image. `low` uses fewer tokens, you can opt in
	// to high resolution using `high`. Default value is `auto`
	//
	// Any of "auto", "low", "high".
	Detail ImageURLDetail `json:"detail"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		URL         respjson.Field
		Detail      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ImageURL) RawJSON() string { return r.JSON.raw }
func (r *ImageURL) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ImageURL to a ImageURLParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ImageURLParam.Overrides()
func (r ImageURL) ToParam() ImageURLParam {
	return param.Override[ImageURLParam](json.RawMessage(r.RawJSON()))
}

// Specifies the detail level of the image. `low` uses fewer tokens, you can opt in
// to high resolution using `high`. Default value is `auto`
type ImageURLDetail string

const (
	ImageURLDetailAuto ImageURLDetail = "auto"
	ImageURLDetailLow  ImageURLDetail = "low"
	ImageURLDetailHigh ImageURLDetail = "high"
)

// The property URL is required.
type ImageURLParam struct {
	// The external URL of the image, must be a supported image types: jpeg, jpg, png,
	// gif, webp.
	URL string `json:"url,required" format:"uri"`
	// Specifies the detail level of the image. `low` uses fewer tokens, you can opt in
	// to high resolution using `high`. Default value is `auto`
	//
	// Any of "auto", "low", "high".
	Detail ImageURLDetail `json:"detail,omitzero"`
	paramObj
}

func (r ImageURLParam) MarshalJSON() (data []byte, err error) {
	type shadow ImageURLParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ImageURLParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// References an image URL in the content of a message.
type ImageURLContentBlock struct {
	ImageURL ImageURL `json:"image_url,required"`
	// The type of the content part.
	Type constant.ImageURL `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ImageURL    respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ImageURLContentBlock) RawJSON() string { return r.JSON.raw }
func (r *ImageURLContentBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToParam converts this ImageURLContentBlock to a ImageURLContentBlockParam.
//
// Warning: the fields of the param type will not be present. ToParam should only
// be used at the last possible moment before sending a request. Test for this with
// ImageURLContentBlockParam.Overrides()
func (r ImageURLContentBlock) ToParam() ImageURLContentBlockParam {
	return param.Override[ImageURLContentBlockParam](json.RawMessage(r.RawJSON()))
}

// References an image URL in the content of a message.
//
// The properties ImageURL, Type are required.
type ImageURLContentBlockParam struct {
	ImageURL ImageURLParam `json:"image_url,omitzero,required"`
	// The type of the content part.
	//
	// This field can be elided, and will marshal its zero value as "image_url".
	Type constant.ImageURL `json:"type,required"`
	paramObj
}

func (r ImageURLContentBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow ImageURLContentBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ImageURLContentBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ImageURLDelta struct {
	// Specifies the detail level of the image. `low` uses fewer tokens, you can opt in
	// to high resolution using `high`.
	//
	// Any of "auto", "low", "high".
	Detail ImageURLDeltaDetail `json:"detail"`
	// The URL of the image, must be a supported image types: jpeg, jpg, png, gif,
	// webp.
	URL string `json:"url"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Detail      respjson.Field
		URL         respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ImageURLDelta) RawJSON() string { return r.JSON.raw }
func (r *ImageURLDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Specifies the detail level of the image. `low` uses fewer tokens, you can opt in
// to high resolution using `high`.
type ImageURLDeltaDetail string

const (
	ImageURLDeltaDetailAuto ImageURLDeltaDetail = "auto"
	ImageURLDeltaDetailLow  ImageURLDeltaDetail = "low"
	ImageURLDeltaDetailHigh ImageURLDeltaDetail = "high"
)

// References an image URL in the content of a message.
type ImageURLDeltaBlock struct {
	// The index of the content part in the message.
	Index int64 `json:"index,required"`
	// Always `image_url`.
	Type     constant.ImageURL `json:"type,required"`
	ImageURL ImageURLDelta     `json:"image_url"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Index       respjson.Field
		Type        respjson.Field
		ImageURL    respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ImageURLDeltaBlock) RawJSON() string { return r.JSON.raw }
func (r *ImageURLDeltaBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Represents a message within a
// [thread](https://platform.openai.com/docs/api-reference/threads).
type Message struct {
	// The identifier, which can be referenced in API endpoints.
	ID string `json:"id,required"`
	// If applicable, the ID of the
	// [assistant](https://platform.openai.com/docs/api-reference/assistants) that
	// authored this message.
	AssistantID string `json:"assistant_id,required"`
	// A list of files attached to the message, and the tools they were added to.
	Attachments []MessageAttachment `json:"attachments,required"`
	// The Unix timestamp (in seconds) for when the message was completed.
	CompletedAt int64 `json:"completed_at,required"`
	// The content of the message in array of text and/or images.
	Content []MessageContentUnion `json:"content,required"`
	// The Unix timestamp (in seconds) for when the message was created.
	CreatedAt int64 `json:"created_at,required"`
	// The Unix timestamp (in seconds) for when the message was marked as incomplete.
	IncompleteAt int64 `json:"incomplete_at,required"`
	// On an incomplete message, details about why the message is incomplete.
	IncompleteDetails MessageIncompleteDetails `json:"incomplete_details,required"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,required"`
	// The object type, which is always `thread.message`.
	Object constant.ThreadMessage `json:"object,required"`
	// The entity that produced the message. One of `user` or `assistant`.
	//
	// Any of "user", "assistant".
	Role MessageRole `json:"role,required"`
	// The ID of the [run](https://platform.openai.com/docs/api-reference/runs)
	// associated with the creation of this message. Value is `null` when messages are
	// created manually using the create message or create thread endpoints.
	RunID string `json:"run_id,required"`
	// The status of the message, which can be either `in_progress`, `incomplete`, or
	// `completed`.
	//
	// Any of "in_progress", "incomplete", "completed".
	Status MessageStatus `json:"status,required"`
	// The [thread](https://platform.openai.com/docs/api-reference/threads) ID that
	// this message belongs to.
	ThreadID string `json:"thread_id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID                respjson.Field
		AssistantID       respjson.Field
		Attachments       respjson.Field
		CompletedAt       respjson.Field
		Content           respjson.Field
		CreatedAt         respjson.Field
		IncompleteAt      respjson.Field
		IncompleteDetails respjson.Field
		Metadata          respjson.Field
		Object            respjson.Field
		Role              respjson.Field
		RunID             respjson.Field
		Status            respjson.Field
		ThreadID          respjson.Field
		ExtraFields       map[string]respjson.Field
		raw               string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r Message) RawJSON() string { return r.JSON.raw }
func (r *Message) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type MessageAttachment struct {
	// The ID of the file to attach to the message.
	FileID string `json:"file_id"`
	// The tools to add this file to.
	Tools []MessageAttachmentToolUnion `json:"tools"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		FileID      respjson.Field
		Tools       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r MessageAttachment) RawJSON() string { return r.JSON.raw }
func (r *MessageAttachment) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// MessageAttachmentToolUnion contains all possible properties and values from
// [CodeInterpreterTool], [MessageAttachmentToolFileSearchTool].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type MessageAttachmentToolUnion struct {
	Type string `json:"type"`
	JSON struct {
		Type respjson.Field
		raw  string
	} `json:"-"`
}

func (u MessageAttachmentToolUnion) AsCodeInterpreterTool() (v CodeInterpreterTool) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u MessageAttachmentToolUnion) AsFileSearchTool() (v MessageAttachmentToolFileSearchTool) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u MessageAttachmentToolUnion) RawJSON() string { return u.JSON.raw }

func (r *MessageAttachmentToolUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type MessageAttachmentToolFileSearchTool struct {
	// The type of tool being defined: `file_search`
	Type constant.FileSearch `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r MessageAttachmentToolFileSearchTool) RawJSON() string { return r.JSON.raw }
func (r *MessageAttachmentToolFileSearchTool) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// On an incomplete message, details about why the message is incomplete.
type MessageIncompleteDetails struct {
	// The reason the message is incomplete.
	//
	// Any of "content_filter", "max_tokens", "run_cancelled", "run_expired",
	// "run_failed".
	Reason string `json:"reason,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Reason      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r MessageIncompleteDetails) RawJSON() string { return r.JSON.raw }
func (r *MessageIncompleteDetails) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The entity that produced the message. One of `user` or `assistant`.
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
)

// The status of the message, which can be either `in_progress`, `incomplete`, or
// `completed`.
type MessageStatus string

const (
	MessageStatusInProgress MessageStatus = "in_progress"
	MessageStatusIncomplete MessageStatus = "incomplete"
	MessageStatusCompleted  MessageStatus = "completed"
)

// MessageContentUnion contains all possible properties and values from
// [ImageFileContentBlock], [ImageURLContentBlock], [TextContentBlock],
// [RefusalContentBlock].
//
// Use the [MessageContentUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type MessageContentUnion struct {
	// This field is from variant [ImageFileContentBlock].
	ImageFile ImageFile `json:"image_file"`
	// Any of "image_file", "image_url", "text", "refusal".
	Type string `json:"type"`
	// This field is from variant [ImageURLContentBlock].
	ImageURL ImageURL `json:"image_url"`
	// This field is from variant [TextContentBlock].
	Text Text `json:"text"`
	// This field is from variant [RefusalContentBlock].
	Refusal string `json:"refusal"`
	JSON    struct {
		ImageFile respjson.Field
		Type      respjson.Field
		ImageURL  respjson.Field
		Text      respjson.Field
		Refusal   respjson.Field
		raw       string
	} `json:"-"`
}

// anyMessageContent is implemented by each variant of [MessageContentUnion] to add
// type safety for the return type of [MessageContentUnion.AsAny]
type anyMessageContent interface {
	implMessageContentUnion()
}

func (ImageFileContentBlock) implMessageContentUnion() {}
func (ImageURLContentBlock) implMessageContentUnion()  {}
func (TextContentBlock) implMessageContentUnion()      {}
func (RefusalContentBlock) implMessageContentUnion()   {}

// Use the following switch statement to find the correct variant
//
//	switch variant := MessageContentUnion.AsAny().(type) {
//	case openai.ImageFileContentBlock:
//	case openai.ImageURLContentBlock:
//	case openai.TextContentBlock:
//	case openai.RefusalContentBlock:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u MessageContentUnion) AsAny() anyMessageContent {
	switch u.Type {
	case "image_file":
		return u.AsImageFile()
	case "image_url":
		return u.AsImageURL()
	case "text":
		return u.AsText()
	case "refusal":
		return u.AsRefusal()
	}
	return nil
}

func (u MessageContentUnion) AsImageFile() (v ImageFileContentBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u MessageContentUnion) AsImageURL() (v ImageURLContentBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u MessageContentUnion) AsText() (v TextContentBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u MessageContentUnion) AsRefusal() (v RefusalContentBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u MessageContentUnion) RawJSON() string { return u.JSON.raw }

func (r *MessageContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// MessageContentDeltaUnion contains all possible properties and values from
// [ImageFileDeltaBlock], [TextDeltaBlock], [RefusalDeltaBlock],
// [ImageURLDeltaBlock].
//
// Use the [MessageContentDeltaUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type MessageContentDeltaUnion struct {
	Index int64 `json:"index"`
	// Any of "image_file", "text", "refusal", "image_url".
	Type string `json:"type"`
	// This field is from variant [ImageFileDeltaBlock].
	ImageFile ImageFileDelta `json:"image_file"`
	// This field is from variant [TextDeltaBlock].
	Text TextDelta `json:"text"`
	// This field is from variant [RefusalDeltaBlock].
	Refusal string `json:"refusal"`
	// This field is from variant [ImageURLDeltaBlock].
	ImageURL ImageURLDelta `json:"image_url"`
	JSON     struct {
		Index     respjson.Field
		Type      respjson.Field
		ImageFile respjson.Field
		Text      respjson.Field
		Refusal   respjson.Field
		ImageURL  respjson.Field
		raw       string
	} `json:"-"`
}

// anyMessageContentDelta is implemented by each variant of
// [MessageContentDeltaUnion] to add type safety for the return type of
// [MessageContentDeltaUnion.AsAny]
type anyMessageContentDelta interface {
	implMessageContentDeltaUnion()
}

func (ImageFileDeltaBlock) implMessageContentDeltaUnion() {}
func (TextDeltaBlock) implMessageContentDeltaUnion()      {}
func (RefusalDeltaBlock) implMessageContentDeltaUnion()   {}
func (ImageURLDeltaBlock) implMessageContentDeltaUnion()  {}

// Use the following switch statement to find the correct variant
//
//	switch variant := MessageContentDeltaUnion.AsAny().(type) {
//	case openai.ImageFileDeltaBlock:
//	case openai.TextDeltaBlock:
//	case openai.RefusalDeltaBlock:
//	case openai.ImageURLDeltaBlock:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u MessageContentDeltaUnion) AsAny() anyMessageContentDelta {
	switch u.Type {
	case "image_file":
		return u.AsImageFile()
	case "text":
		return u.AsText()
	case "refusal":
		return u.AsRefusal()
	case "image_url":
		return u.AsImageURL()
	}
	return nil
}

func (u MessageContentDeltaUnion) AsImageFile() (v ImageFileDeltaBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u MessageContentDeltaUnion) AsText() (v TextDeltaBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u MessageContentDeltaUnion) AsRefusal() (v RefusalDeltaBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u MessageContentDeltaUnion) AsImageURL() (v ImageURLDeltaBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u MessageContentDeltaUnion) RawJSON() string { return u.JSON.raw }

func (r *MessageContentDeltaUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func MessageContentPartParamOfImageFile(imageFile ImageFileParam) MessageContentPartParamUnion {
	var variant ImageFileContentBlockParam
	variant.ImageFile = imageFile
	return MessageContentPartParamUnion{OfImageFile: &variant}
}

func MessageContentPartParamOfImageURL(imageURL ImageURLParam) MessageContentPartParamUnion {
	var variant ImageURLContentBlockParam
	variant.ImageURL = imageURL
	return MessageContentPartParamUnion{OfImageURL: &variant}
}

func MessageContentPartParamOfText(text string) MessageContentPartParamUnion {
	var variant TextContentBlockParam
	variant.Text = text
	return MessageContentPartParamUnion{OfText: &variant}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type MessageContentPartParamUnion struct {
	OfImageFile *ImageFileContentBlockParam `json:",omitzero,inline"`
	OfImageURL  *ImageURLContentBlockParam  `json:",omitzero,inline"`
	OfText      *TextContentBlockParam      `json:",omitzero,inline"`
	paramUnion
}

func (u MessageContentPartParamUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfImageFile, u.OfImageURL, u.OfText)
}
func (u *MessageContentPartParamUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *MessageContentPartParamUnion) asAny() any {
	if !param.IsOmitted(u.OfImageFile) {
		return u.OfImageFile
	} else if !param.IsOmitted(u.OfImageURL) {
		return u.OfImageURL
	} else if !param.IsOmitted(u.OfText) {
		return u.OfText
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u MessageContentPartParamUnion) GetImageFile() *ImageFileParam {
	if vt := u.OfImageFile; vt != nil {
		return &vt.ImageFile
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u MessageContentPartParamUnion) GetImageURL() *ImageURLParam {
	if vt := u.OfImageURL; vt != nil {
		return &vt.ImageURL
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u MessageContentPartParamUnion) GetText() *string {
	if vt := u.OfText; vt != nil {
		return &vt.Text
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u MessageContentPartParamUnion) GetType() *string {
	if vt := u.OfImageFile; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfImageURL; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfText; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[MessageContentPartParamUnion](
		"type",
		apijson.Discriminator[ImageFileContentBlockParam]("image_file"),
		apijson.Discriminator[ImageURLContentBlockParam]("image_url"),
		apijson.Discriminator[TextContentBlockParam]("text"),
	)
}

type MessageDeleted struct {
	ID      string                        `json:"id,required"`
	Deleted bool                          `json:"deleted,required"`
	Object  constant.ThreadMessageDeleted `json:"object,required"`
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
func (r MessageDeleted) RawJSON() string { return r.JSON.raw }
func (r *MessageDeleted) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The delta containing the fields that have changed on the Message.
type MessageDelta struct {
	// The content of the message in array of text and/or images.
	Content []MessageContentDeltaUnion `json:"content"`
	// The entity that produced the message. One of `user` or `assistant`.
	//
	// Any of "user", "assistant".
	Role MessageDeltaRole `json:"role"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Content     respjson.Field
		Role        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r MessageDelta) RawJSON() string { return r.JSON.raw }
func (r *MessageDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The entity that produced the message. One of `user` or `assistant`.
type MessageDeltaRole string

const (
	MessageDeltaRoleUser      MessageDeltaRole = "user"
	MessageDeltaRoleAssistant MessageDeltaRole = "assistant"
)

// Represents a message delta i.e. any changed fields on a message during
// streaming.
type MessageDeltaEvent struct {
	// The identifier of the message, which can be referenced in API endpoints.
	ID string `json:"id,required"`
	// The delta containing the fields that have changed on the Message.
	Delta MessageDelta `json:"delta,required"`
	// The object type, which is always `thread.message.delta`.
	Object constant.ThreadMessageDelta `json:"object,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Delta       respjson.Field
		Object      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r MessageDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *MessageDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The refusal content generated by the assistant.
type RefusalContentBlock struct {
	Refusal string `json:"refusal,required"`
	// Always `refusal`.
	Type constant.Refusal `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Refusal     respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r RefusalContentBlock) RawJSON() string { return r.JSON.raw }
func (r *RefusalContentBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The refusal content that is part of a message.
type RefusalDeltaBlock struct {
	// The index of the refusal part in the message.
	Index int64 `json:"index,required"`
	// Always `refusal`.
	Type    constant.Refusal `json:"type,required"`
	Refusal string           `json:"refusal"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Index       respjson.Field
		Type        respjson.Field
		Refusal     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r RefusalDeltaBlock) RawJSON() string { return r.JSON.raw }
func (r *RefusalDeltaBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type Text struct {
	Annotations []AnnotationUnion `json:"annotations,required"`
	// The data that makes up the text.
	Value string `json:"value,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Annotations respjson.Field
		Value       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r Text) RawJSON() string { return r.JSON.raw }
func (r *Text) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The text content that is part of a message.
type TextContentBlock struct {
	Text Text `json:"text,required"`
	// Always `text`.
	Type constant.Text `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Text        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r TextContentBlock) RawJSON() string { return r.JSON.raw }
func (r *TextContentBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The text content that is part of a message.
//
// The properties Text, Type are required.
type TextContentBlockParam struct {
	// Text content to be sent to the model
	Text string `json:"text,required"`
	// Always `text`.
	//
	// This field can be elided, and will marshal its zero value as "text".
	Type constant.Text `json:"type,required"`
	paramObj
}

func (r TextContentBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow TextContentBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *TextContentBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type TextDelta struct {
	Annotations []AnnotationDeltaUnion `json:"annotations"`
	// The data that makes up the text.
	Value string `json:"value"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Annotations respjson.Field
		Value       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r TextDelta) RawJSON() string { return r.JSON.raw }
func (r *TextDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The text content that is part of a message.
type TextDeltaBlock struct {
	// The index of the content part in the message.
	Index int64 `json:"index,required"`
	// Always `text`.
	Type constant.Text `json:"type,required"`
	Text TextDelta     `json:"text"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Index       respjson.Field
		Type        respjson.Field
		Text        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r TextDeltaBlock) RawJSON() string { return r.JSON.raw }
func (r *TextDeltaBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaThreadMessageNewParams struct {
	// The text contents of the message.
	Content BetaThreadMessageNewParamsContentUnion `json:"content,omitzero,required"`
	// The role of the entity that is creating the message. Allowed values include:
	//
	//   - `user`: Indicates the message is sent by an actual user and should be used in
	//     most cases to represent user-generated messages.
	//   - `assistant`: Indicates the message is generated by the assistant. Use this
	//     value to insert messages from the assistant into the conversation.
	//
	// Any of "user", "assistant".
	Role BetaThreadMessageNewParamsRole `json:"role,omitzero,required"`
	// A list of files attached to the message, and the tools they should be added to.
	Attachments []BetaThreadMessageNewParamsAttachment `json:"attachments,omitzero"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,omitzero"`
	paramObj
}

func (r BetaThreadMessageNewParams) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadMessageNewParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadMessageNewParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaThreadMessageNewParamsContentUnion struct {
	OfString              param.Opt[string]              `json:",omitzero,inline"`
	OfArrayOfContentParts []MessageContentPartParamUnion `json:",omitzero,inline"`
	paramUnion
}

func (u BetaThreadMessageNewParamsContentUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfArrayOfContentParts)
}
func (u *BetaThreadMessageNewParamsContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaThreadMessageNewParamsContentUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfArrayOfContentParts) {
		return &u.OfArrayOfContentParts
	}
	return nil
}

// The role of the entity that is creating the message. Allowed values include:
//
//   - `user`: Indicates the message is sent by an actual user and should be used in
//     most cases to represent user-generated messages.
//   - `assistant`: Indicates the message is generated by the assistant. Use this
//     value to insert messages from the assistant into the conversation.
type BetaThreadMessageNewParamsRole string

const (
	BetaThreadMessageNewParamsRoleUser      BetaThreadMessageNewParamsRole = "user"
	BetaThreadMessageNewParamsRoleAssistant BetaThreadMessageNewParamsRole = "assistant"
)

type BetaThreadMessageNewParamsAttachment struct {
	// The ID of the file to attach to the message.
	FileID param.Opt[string] `json:"file_id,omitzero"`
	// The tools to add this file to.
	Tools []BetaThreadMessageNewParamsAttachmentToolUnion `json:"tools,omitzero"`
	paramObj
}

func (r BetaThreadMessageNewParamsAttachment) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadMessageNewParamsAttachment
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadMessageNewParamsAttachment) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaThreadMessageNewParamsAttachmentToolUnion struct {
	OfCodeInterpreter *CodeInterpreterToolParam                           `json:",omitzero,inline"`
	OfFileSearch      *BetaThreadMessageNewParamsAttachmentToolFileSearch `json:",omitzero,inline"`
	paramUnion
}

func (u BetaThreadMessageNewParamsAttachmentToolUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfCodeInterpreter, u.OfFileSearch)
}
func (u *BetaThreadMessageNewParamsAttachmentToolUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaThreadMessageNewParamsAttachmentToolUnion) asAny() any {
	if !param.IsOmitted(u.OfCodeInterpreter) {
		return u.OfCodeInterpreter
	} else if !param.IsOmitted(u.OfFileSearch) {
		return u.OfFileSearch
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaThreadMessageNewParamsAttachmentToolUnion) GetType() *string {
	if vt := u.OfCodeInterpreter; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfFileSearch; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[BetaThreadMessageNewParamsAttachmentToolUnion](
		"type",
		apijson.Discriminator[CodeInterpreterToolParam]("code_interpreter"),
		apijson.Discriminator[BetaThreadMessageNewParamsAttachmentToolFileSearch]("file_search"),
	)
}

func NewBetaThreadMessageNewParamsAttachmentToolFileSearch() BetaThreadMessageNewParamsAttachmentToolFileSearch {
	return BetaThreadMessageNewParamsAttachmentToolFileSearch{
		Type: "file_search",
	}
}

// This struct has a constant value, construct it with
// [NewBetaThreadMessageNewParamsAttachmentToolFileSearch].
type BetaThreadMessageNewParamsAttachmentToolFileSearch struct {
	// The type of tool being defined: `file_search`
	Type constant.FileSearch `json:"type,required"`
	paramObj
}

func (r BetaThreadMessageNewParamsAttachmentToolFileSearch) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadMessageNewParamsAttachmentToolFileSearch
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadMessageNewParamsAttachmentToolFileSearch) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaThreadMessageUpdateParams struct {
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,omitzero"`
	paramObj
}

func (r BetaThreadMessageUpdateParams) MarshalJSON() (data []byte, err error) {
	type shadow BetaThreadMessageUpdateParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThreadMessageUpdateParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaThreadMessageListParams struct {
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
	// Filter messages by the run ID that generated them.
	RunID param.Opt[string] `query:"run_id,omitzero" json:"-"`
	// Sort order by the `created_at` timestamp of the objects. `asc` for ascending
	// order and `desc` for descending order.
	//
	// Any of "asc", "desc".
	Order BetaThreadMessageListParamsOrder `query:"order,omitzero" json:"-"`
	paramObj
}

// URLQuery serializes [BetaThreadMessageListParams]'s query parameters as
// `url.Values`.
func (r BetaThreadMessageListParams) URLQuery() (v url.Values, err error) {
	return apiquery.MarshalWithSettings(r, apiquery.QuerySettings{
		ArrayFormat:  apiquery.ArrayQueryFormatBrackets,
		NestedFormat: apiquery.NestedQueryFormatBrackets,
	})
}

// Sort order by the `created_at` timestamp of the objects. `asc` for ascending
// order and `desc` for descending order.
type BetaThreadMessageListParamsOrder string

const (
	BetaThreadMessageListParamsOrderAsc  BetaThreadMessageListParamsOrder = "asc"
	BetaThreadMessageListParamsOrderDesc BetaThreadMessageListParamsOrder = "desc"
)
