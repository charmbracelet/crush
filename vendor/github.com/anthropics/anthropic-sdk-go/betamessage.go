// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/anthropics/anthropic-sdk-go/internal/apijson"
	"github.com/anthropics/anthropic-sdk-go/internal/paramutil"
	"github.com/anthropics/anthropic-sdk-go/internal/requestconfig"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/anthropics/anthropic-sdk-go/packages/respjson"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
)

// BetaMessageService contains methods and other services that help with
// interacting with the anthropic API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewBetaMessageService] method instead.
type BetaMessageService struct {
	Options []option.RequestOption
	Batches BetaMessageBatchService
}

// NewBetaMessageService generates a new service that applies the given options to
// each request. These options are applied after the parent client's options (if
// there is one), and before any request-specific options.
func NewBetaMessageService(opts ...option.RequestOption) (r BetaMessageService) {
	r = BetaMessageService{}
	r.Options = opts
	r.Batches = NewBetaMessageBatchService(opts...)
	return
}

// Send a structured list of input messages with text and/or image content, and the
// model will generate the next message in the conversation.
//
// The Messages API can be used for either single queries or stateless multi-turn
// conversations.
//
// Learn more about the Messages API in our [user guide](/en/docs/initial-setup)
//
// Note: If you choose to set a timeout for this request, we recommend 10 minutes.
func (r *BetaMessageService) New(ctx context.Context, params BetaMessageNewParams, opts ...option.RequestOption) (res *BetaMessage, err error) {
	for _, v := range params.Betas {
		opts = append(opts, option.WithHeaderAdd("anthropic-beta", fmt.Sprintf("%s", v)))
	}
	opts = append(r.Options[:], opts...)

	// For non-streaming requests, calculate the appropriate timeout based on maxTokens
	// and check against model-specific limits
	timeout, timeoutErr := CalculateNonStreamingTimeout(int(params.MaxTokens), params.Model, opts)
	if timeoutErr != nil {
		return nil, timeoutErr
	}
	opts = append(opts, option.WithRequestTimeout(timeout))

	path := "v1/messages?beta=true"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, params, &res, opts...)
	return
}

// Send a structured list of input messages with text and/or image content, and the
// model will generate the next message in the conversation.
//
// The Messages API can be used for either single queries or stateless multi-turn
// conversations.
//
// Learn more about the Messages API in our [user guide](/en/docs/initial-setup)
//
// Note: If you choose to set a timeout for this request, we recommend 10 minutes.
func (r *BetaMessageService) NewStreaming(ctx context.Context, params BetaMessageNewParams, opts ...option.RequestOption) (stream *ssestream.Stream[BetaRawMessageStreamEventUnion]) {
	var (
		raw *http.Response
		err error
	)
	for _, v := range params.Betas {
		opts = append(opts, option.WithHeaderAdd("anthropic-beta", fmt.Sprintf("%s", v)))
	}
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithJSONSet("stream", true)}, opts...)
	path := "v1/messages?beta=true"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, params, &raw, opts...)
	return ssestream.NewStream[BetaRawMessageStreamEventUnion](ssestream.NewDecoder(raw), err)
}

// Count the number of tokens in a Message.
//
// The Token Count API can be used to count the number of tokens in a Message,
// including tools, images, and documents, without creating it.
//
// Learn more about token counting in our
// [user guide](/en/docs/build-with-claude/token-counting)
func (r *BetaMessageService) CountTokens(ctx context.Context, params BetaMessageCountTokensParams, opts ...option.RequestOption) (res *BetaMessageTokensCount, err error) {
	for _, v := range params.Betas {
		opts = append(opts, option.WithHeaderAdd("anthropic-beta", fmt.Sprintf("%s", v)))
	}
	opts = append(r.Options[:], opts...)
	path := "v1/messages/count_tokens?beta=true"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, params, &res, opts...)
	return
}

// The properties Data, MediaType, Type are required.
type BetaBase64ImageSourceParam struct {
	Data string `json:"data,required" format:"byte"`
	// Any of "image/jpeg", "image/png", "image/gif", "image/webp".
	MediaType BetaBase64ImageSourceMediaType `json:"media_type,omitzero,required"`
	// This field can be elided, and will marshal its zero value as "base64".
	Type constant.Base64 `json:"type,required"`
	paramObj
}

func (r BetaBase64ImageSourceParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaBase64ImageSourceParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaBase64ImageSourceParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaBase64ImageSourceMediaType string

const (
	BetaBase64ImageSourceMediaTypeImageJPEG BetaBase64ImageSourceMediaType = "image/jpeg"
	BetaBase64ImageSourceMediaTypeImagePNG  BetaBase64ImageSourceMediaType = "image/png"
	BetaBase64ImageSourceMediaTypeImageGIF  BetaBase64ImageSourceMediaType = "image/gif"
	BetaBase64ImageSourceMediaTypeImageWebP BetaBase64ImageSourceMediaType = "image/webp"
)

// The properties Source, Type are required.
type BetaBase64PDFBlockParam struct {
	Source  BetaBase64PDFBlockSourceUnionParam `json:"source,omitzero,required"`
	Context param.Opt[string]                  `json:"context,omitzero"`
	Title   param.Opt[string]                  `json:"title,omitzero"`
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam `json:"cache_control,omitzero"`
	Citations    BetaCitationsConfigParam       `json:"citations,omitzero"`
	// This field can be elided, and will marshal its zero value as "document".
	Type constant.Document `json:"type,required"`
	paramObj
}

func (r BetaBase64PDFBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaBase64PDFBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaBase64PDFBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaBase64PDFBlockSourceUnionParam struct {
	OfBase64  *BetaBase64PDFSourceParam    `json:",omitzero,inline"`
	OfText    *BetaPlainTextSourceParam    `json:",omitzero,inline"`
	OfContent *BetaContentBlockSourceParam `json:",omitzero,inline"`
	OfURL     *BetaURLPDFSourceParam       `json:",omitzero,inline"`
	OfFile    *BetaFileDocumentSourceParam `json:",omitzero,inline"`
	paramUnion
}

func (u BetaBase64PDFBlockSourceUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfBase64,
		u.OfText,
		u.OfContent,
		u.OfURL,
		u.OfFile)
}
func (u *BetaBase64PDFBlockSourceUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaBase64PDFBlockSourceUnionParam) asAny() any {
	if !param.IsOmitted(u.OfBase64) {
		return u.OfBase64
	} else if !param.IsOmitted(u.OfText) {
		return u.OfText
	} else if !param.IsOmitted(u.OfContent) {
		return u.OfContent
	} else if !param.IsOmitted(u.OfURL) {
		return u.OfURL
	} else if !param.IsOmitted(u.OfFile) {
		return u.OfFile
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaBase64PDFBlockSourceUnionParam) GetContent() *BetaContentBlockSourceContentUnionParam {
	if vt := u.OfContent; vt != nil {
		return &vt.Content
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaBase64PDFBlockSourceUnionParam) GetURL() *string {
	if vt := u.OfURL; vt != nil {
		return &vt.URL
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaBase64PDFBlockSourceUnionParam) GetFileID() *string {
	if vt := u.OfFile; vt != nil {
		return &vt.FileID
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaBase64PDFBlockSourceUnionParam) GetData() *string {
	if vt := u.OfBase64; vt != nil {
		return (*string)(&vt.Data)
	} else if vt := u.OfText; vt != nil {
		return (*string)(&vt.Data)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaBase64PDFBlockSourceUnionParam) GetMediaType() *string {
	if vt := u.OfBase64; vt != nil {
		return (*string)(&vt.MediaType)
	} else if vt := u.OfText; vt != nil {
		return (*string)(&vt.MediaType)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaBase64PDFBlockSourceUnionParam) GetType() *string {
	if vt := u.OfBase64; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfText; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfContent; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfURL; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfFile; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

func init() {
	apijson.RegisterUnion[BetaBase64PDFBlockSourceUnionParam](
		"type",
		apijson.Discriminator[BetaBase64PDFSourceParam]("base64"),
		apijson.Discriminator[BetaPlainTextSourceParam]("text"),
		apijson.Discriminator[BetaContentBlockSourceParam]("content"),
		apijson.Discriminator[BetaURLPDFSourceParam]("url"),
		apijson.Discriminator[BetaFileDocumentSourceParam]("file"),
	)
}

func init() {
	apijson.RegisterUnion[BetaContentBlockParamUnion](
		"type",
		apijson.Discriminator[BetaServerToolUseBlockParam]("server_tool_use"),
		apijson.Discriminator[BetaWebSearchToolResultBlockParam]("web_search_tool_result"),
		apijson.Discriminator[BetaCodeExecutionToolResultBlockParam]("code_execution_tool_result"),
		apijson.Discriminator[BetaMCPToolUseBlockParam]("mcp_tool_use"),
		apijson.Discriminator[BetaRequestMCPToolResultBlockParam]("mcp_tool_result"),
		apijson.Discriminator[BetaTextBlockParam]("text"),
		apijson.Discriminator[BetaImageBlockParam]("image"),
		apijson.Discriminator[BetaToolUseBlockParam]("tool_use"),
		apijson.Discriminator[BetaToolResultBlockParam]("tool_result"),
		apijson.Discriminator[BetaBase64PDFBlockParam]("document"),
		apijson.Discriminator[BetaThinkingBlockParam]("thinking"),
		apijson.Discriminator[BetaRedactedThinkingBlockParam]("redacted_thinking"),
		apijson.Discriminator[BetaContainerUploadBlockParam]("container_upload"),
	)
}

func init() {
	apijson.RegisterUnion[BetaImageBlockParamSourceUnion](
		"type",
		apijson.Discriminator[BetaBase64ImageSourceParam]("base64"),
		apijson.Discriminator[BetaURLImageSourceParam]("url"),
		apijson.Discriminator[BetaFileImageSourceParam]("file"),
	)
}

func init() {
	apijson.RegisterUnion[BetaTextCitationParamUnion](
		"type",
		apijson.Discriminator[BetaCitationCharLocationParam]("char_location"),
		apijson.Discriminator[BetaCitationPageLocationParam]("page_location"),
		apijson.Discriminator[BetaCitationContentBlockLocationParam]("content_block_location"),
		apijson.Discriminator[BetaCitationWebSearchResultLocationParam]("web_search_result_location"),
	)
}

func init() {
	apijson.RegisterUnion[BetaThinkingConfigParamUnion](
		"type",
		apijson.Discriminator[BetaThinkingConfigEnabledParam]("enabled"),
		apijson.Discriminator[BetaThinkingConfigDisabledParam]("disabled"),
	)
}

func init() {
	apijson.RegisterUnion[BetaToolChoiceUnionParam](
		"type",
		apijson.Discriminator[BetaToolChoiceAutoParam]("auto"),
		apijson.Discriminator[BetaToolChoiceAnyParam]("any"),
		apijson.Discriminator[BetaToolChoiceToolParam]("tool"),
		apijson.Discriminator[BetaToolChoiceNoneParam]("none"),
	)
}

func init() {
	apijson.RegisterUnion[BetaToolResultBlockParamContentUnion](
		"type",
		apijson.Discriminator[BetaTextBlockParam]("text"),
		apijson.Discriminator[BetaImageBlockParam]("image"),
	)
}

// The properties Data, MediaType, Type are required.
type BetaBase64PDFSourceParam struct {
	Data string `json:"data,required" format:"byte"`
	// This field can be elided, and will marshal its zero value as "application/pdf".
	MediaType constant.ApplicationPDF `json:"media_type,required"`
	// This field can be elided, and will marshal its zero value as "base64".
	Type constant.Base64 `json:"type,required"`
	paramObj
}

func (r BetaBase64PDFSourceParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaBase64PDFSourceParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaBase64PDFSourceParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func NewBetaCacheControlEphemeralParam() BetaCacheControlEphemeralParam {
	return BetaCacheControlEphemeralParam{
		Type: "ephemeral",
	}
}

// This struct has a constant value, construct it with
// [NewBetaCacheControlEphemeralParam].
type BetaCacheControlEphemeralParam struct {
	// The time-to-live for the cache control breakpoint.
	//
	// This may be one the following values:
	//
	// - `5m`: 5 minutes
	// - `1h`: 1 hour
	//
	// Defaults to `5m`.
	//
	// Any of "5m", "1h".
	TTL  BetaCacheControlEphemeralTTL `json:"ttl,omitzero"`
	Type constant.Ephemeral           `json:"type,required"`
	paramObj
}

func (r BetaCacheControlEphemeralParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaCacheControlEphemeralParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaCacheControlEphemeralParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The time-to-live for the cache control breakpoint.
//
// This may be one the following values:
//
// - `5m`: 5 minutes
// - `1h`: 1 hour
//
// Defaults to `5m`.
type BetaCacheControlEphemeralTTL string

const (
	BetaCacheControlEphemeralTTLTTL5m BetaCacheControlEphemeralTTL = "5m"
	BetaCacheControlEphemeralTTLTTL1h BetaCacheControlEphemeralTTL = "1h"
)

type BetaCacheCreation struct {
	// The number of input tokens used to create the 1 hour cache entry.
	Ephemeral1hInputTokens int64 `json:"ephemeral_1h_input_tokens,required"`
	// The number of input tokens used to create the 5 minute cache entry.
	Ephemeral5mInputTokens int64 `json:"ephemeral_5m_input_tokens,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Ephemeral1hInputTokens respjson.Field
		Ephemeral5mInputTokens respjson.Field
		ExtraFields            map[string]respjson.Field
		raw                    string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaCacheCreation) RawJSON() string { return r.JSON.raw }
func (r *BetaCacheCreation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaCitationCharLocation struct {
	CitedText      string                `json:"cited_text,required"`
	DocumentIndex  int64                 `json:"document_index,required"`
	DocumentTitle  string                `json:"document_title,required"`
	EndCharIndex   int64                 `json:"end_char_index,required"`
	StartCharIndex int64                 `json:"start_char_index,required"`
	Type           constant.CharLocation `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		CitedText      respjson.Field
		DocumentIndex  respjson.Field
		DocumentTitle  respjson.Field
		EndCharIndex   respjson.Field
		StartCharIndex respjson.Field
		Type           respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaCitationCharLocation) RawJSON() string { return r.JSON.raw }
func (r *BetaCitationCharLocation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties CitedText, DocumentIndex, DocumentTitle, EndCharIndex,
// StartCharIndex, Type are required.
type BetaCitationCharLocationParam struct {
	DocumentTitle  param.Opt[string] `json:"document_title,omitzero,required"`
	CitedText      string            `json:"cited_text,required"`
	DocumentIndex  int64             `json:"document_index,required"`
	EndCharIndex   int64             `json:"end_char_index,required"`
	StartCharIndex int64             `json:"start_char_index,required"`
	// This field can be elided, and will marshal its zero value as "char_location".
	Type constant.CharLocation `json:"type,required"`
	paramObj
}

func (r BetaCitationCharLocationParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaCitationCharLocationParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaCitationCharLocationParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaCitationContentBlockLocation struct {
	CitedText       string                        `json:"cited_text,required"`
	DocumentIndex   int64                         `json:"document_index,required"`
	DocumentTitle   string                        `json:"document_title,required"`
	EndBlockIndex   int64                         `json:"end_block_index,required"`
	StartBlockIndex int64                         `json:"start_block_index,required"`
	Type            constant.ContentBlockLocation `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		CitedText       respjson.Field
		DocumentIndex   respjson.Field
		DocumentTitle   respjson.Field
		EndBlockIndex   respjson.Field
		StartBlockIndex respjson.Field
		Type            respjson.Field
		ExtraFields     map[string]respjson.Field
		raw             string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaCitationContentBlockLocation) RawJSON() string { return r.JSON.raw }
func (r *BetaCitationContentBlockLocation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties CitedText, DocumentIndex, DocumentTitle, EndBlockIndex,
// StartBlockIndex, Type are required.
type BetaCitationContentBlockLocationParam struct {
	DocumentTitle   param.Opt[string] `json:"document_title,omitzero,required"`
	CitedText       string            `json:"cited_text,required"`
	DocumentIndex   int64             `json:"document_index,required"`
	EndBlockIndex   int64             `json:"end_block_index,required"`
	StartBlockIndex int64             `json:"start_block_index,required"`
	// This field can be elided, and will marshal its zero value as
	// "content_block_location".
	Type constant.ContentBlockLocation `json:"type,required"`
	paramObj
}

func (r BetaCitationContentBlockLocationParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaCitationContentBlockLocationParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaCitationContentBlockLocationParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaCitationPageLocation struct {
	CitedText       string                `json:"cited_text,required"`
	DocumentIndex   int64                 `json:"document_index,required"`
	DocumentTitle   string                `json:"document_title,required"`
	EndPageNumber   int64                 `json:"end_page_number,required"`
	StartPageNumber int64                 `json:"start_page_number,required"`
	Type            constant.PageLocation `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		CitedText       respjson.Field
		DocumentIndex   respjson.Field
		DocumentTitle   respjson.Field
		EndPageNumber   respjson.Field
		StartPageNumber respjson.Field
		Type            respjson.Field
		ExtraFields     map[string]respjson.Field
		raw             string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaCitationPageLocation) RawJSON() string { return r.JSON.raw }
func (r *BetaCitationPageLocation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties CitedText, DocumentIndex, DocumentTitle, EndPageNumber,
// StartPageNumber, Type are required.
type BetaCitationPageLocationParam struct {
	DocumentTitle   param.Opt[string] `json:"document_title,omitzero,required"`
	CitedText       string            `json:"cited_text,required"`
	DocumentIndex   int64             `json:"document_index,required"`
	EndPageNumber   int64             `json:"end_page_number,required"`
	StartPageNumber int64             `json:"start_page_number,required"`
	// This field can be elided, and will marshal its zero value as "page_location".
	Type constant.PageLocation `json:"type,required"`
	paramObj
}

func (r BetaCitationPageLocationParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaCitationPageLocationParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaCitationPageLocationParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties CitedText, EncryptedIndex, Title, Type, URL are required.
type BetaCitationWebSearchResultLocationParam struct {
	Title          param.Opt[string] `json:"title,omitzero,required"`
	CitedText      string            `json:"cited_text,required"`
	EncryptedIndex string            `json:"encrypted_index,required"`
	URL            string            `json:"url,required"`
	// This field can be elided, and will marshal its zero value as
	// "web_search_result_location".
	Type constant.WebSearchResultLocation `json:"type,required"`
	paramObj
}

func (r BetaCitationWebSearchResultLocationParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaCitationWebSearchResultLocationParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaCitationWebSearchResultLocationParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaCitationsConfigParam struct {
	Enabled param.Opt[bool] `json:"enabled,omitzero"`
	paramObj
}

func (r BetaCitationsConfigParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaCitationsConfigParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaCitationsConfigParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaCitationsDelta struct {
	Citation BetaCitationsDeltaCitationUnion `json:"citation,required"`
	Type     constant.CitationsDelta         `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Citation    respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaCitationsDelta) RawJSON() string { return r.JSON.raw }
func (r *BetaCitationsDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// BetaCitationsDeltaCitationUnion contains all possible properties and values from
// [BetaCitationCharLocation], [BetaCitationPageLocation],
// [BetaCitationContentBlockLocation], [BetaCitationsWebSearchResultLocation].
//
// Use the [BetaCitationsDeltaCitationUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type BetaCitationsDeltaCitationUnion struct {
	CitedText     string `json:"cited_text"`
	DocumentIndex int64  `json:"document_index"`
	DocumentTitle string `json:"document_title"`
	// This field is from variant [BetaCitationCharLocation].
	EndCharIndex int64 `json:"end_char_index"`
	// This field is from variant [BetaCitationCharLocation].
	StartCharIndex int64 `json:"start_char_index"`
	// Any of "char_location", "page_location", "content_block_location",
	// "web_search_result_location".
	Type string `json:"type"`
	// This field is from variant [BetaCitationPageLocation].
	EndPageNumber int64 `json:"end_page_number"`
	// This field is from variant [BetaCitationPageLocation].
	StartPageNumber int64 `json:"start_page_number"`
	// This field is from variant [BetaCitationContentBlockLocation].
	EndBlockIndex int64 `json:"end_block_index"`
	// This field is from variant [BetaCitationContentBlockLocation].
	StartBlockIndex int64 `json:"start_block_index"`
	// This field is from variant [BetaCitationsWebSearchResultLocation].
	EncryptedIndex string `json:"encrypted_index"`
	// This field is from variant [BetaCitationsWebSearchResultLocation].
	Title string `json:"title"`
	// This field is from variant [BetaCitationsWebSearchResultLocation].
	URL  string `json:"url"`
	JSON struct {
		CitedText       respjson.Field
		DocumentIndex   respjson.Field
		DocumentTitle   respjson.Field
		EndCharIndex    respjson.Field
		StartCharIndex  respjson.Field
		Type            respjson.Field
		EndPageNumber   respjson.Field
		StartPageNumber respjson.Field
		EndBlockIndex   respjson.Field
		StartBlockIndex respjson.Field
		EncryptedIndex  respjson.Field
		Title           respjson.Field
		URL             respjson.Field
		raw             string
	} `json:"-"`
}

// anyBetaCitationsDeltaCitation is implemented by each variant of
// [BetaCitationsDeltaCitationUnion] to add type safety for the return type of
// [BetaCitationsDeltaCitationUnion.AsAny]
type anyBetaCitationsDeltaCitation interface {
	implBetaCitationsDeltaCitationUnion()
}

func (BetaCitationCharLocation) implBetaCitationsDeltaCitationUnion()             {}
func (BetaCitationPageLocation) implBetaCitationsDeltaCitationUnion()             {}
func (BetaCitationContentBlockLocation) implBetaCitationsDeltaCitationUnion()     {}
func (BetaCitationsWebSearchResultLocation) implBetaCitationsDeltaCitationUnion() {}

// Use the following switch statement to find the correct variant
//
//	switch variant := BetaCitationsDeltaCitationUnion.AsAny().(type) {
//	case anthropic.BetaCitationCharLocation:
//	case anthropic.BetaCitationPageLocation:
//	case anthropic.BetaCitationContentBlockLocation:
//	case anthropic.BetaCitationsWebSearchResultLocation:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u BetaCitationsDeltaCitationUnion) AsAny() anyBetaCitationsDeltaCitation {
	switch u.Type {
	case "char_location":
		return u.AsCharLocation()
	case "page_location":
		return u.AsPageLocation()
	case "content_block_location":
		return u.AsContentBlockLocation()
	case "web_search_result_location":
		return u.AsWebSearchResultLocation()
	}
	return nil
}

func (u BetaCitationsDeltaCitationUnion) AsCharLocation() (v BetaCitationCharLocation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaCitationsDeltaCitationUnion) AsPageLocation() (v BetaCitationPageLocation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaCitationsDeltaCitationUnion) AsContentBlockLocation() (v BetaCitationContentBlockLocation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaCitationsDeltaCitationUnion) AsWebSearchResultLocation() (v BetaCitationsWebSearchResultLocation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u BetaCitationsDeltaCitationUnion) RawJSON() string { return u.JSON.raw }

func (r *BetaCitationsDeltaCitationUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaCitationsWebSearchResultLocation struct {
	CitedText      string                           `json:"cited_text,required"`
	EncryptedIndex string                           `json:"encrypted_index,required"`
	Title          string                           `json:"title,required"`
	Type           constant.WebSearchResultLocation `json:"type,required"`
	URL            string                           `json:"url,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		CitedText      respjson.Field
		EncryptedIndex respjson.Field
		Title          respjson.Field
		Type           respjson.Field
		URL            respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaCitationsWebSearchResultLocation) RawJSON() string { return r.JSON.raw }
func (r *BetaCitationsWebSearchResultLocation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaCodeExecutionOutputBlock struct {
	FileID string                       `json:"file_id,required"`
	Type   constant.CodeExecutionOutput `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		FileID      respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaCodeExecutionOutputBlock) RawJSON() string { return r.JSON.raw }
func (r *BetaCodeExecutionOutputBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties FileID, Type are required.
type BetaCodeExecutionOutputBlockParam struct {
	FileID string `json:"file_id,required"`
	// This field can be elided, and will marshal its zero value as
	// "code_execution_output".
	Type constant.CodeExecutionOutput `json:"type,required"`
	paramObj
}

func (r BetaCodeExecutionOutputBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaCodeExecutionOutputBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaCodeExecutionOutputBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaCodeExecutionResultBlock struct {
	Content    []BetaCodeExecutionOutputBlock `json:"content,required"`
	ReturnCode int64                          `json:"return_code,required"`
	Stderr     string                         `json:"stderr,required"`
	Stdout     string                         `json:"stdout,required"`
	Type       constant.CodeExecutionResult   `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Content     respjson.Field
		ReturnCode  respjson.Field
		Stderr      respjson.Field
		Stdout      respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaCodeExecutionResultBlock) RawJSON() string { return r.JSON.raw }
func (r *BetaCodeExecutionResultBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Content, ReturnCode, Stderr, Stdout, Type are required.
type BetaCodeExecutionResultBlockParam struct {
	Content    []BetaCodeExecutionOutputBlockParam `json:"content,omitzero,required"`
	ReturnCode int64                               `json:"return_code,required"`
	Stderr     string                              `json:"stderr,required"`
	Stdout     string                              `json:"stdout,required"`
	// This field can be elided, and will marshal its zero value as
	// "code_execution_result".
	Type constant.CodeExecutionResult `json:"type,required"`
	paramObj
}

func (r BetaCodeExecutionResultBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaCodeExecutionResultBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaCodeExecutionResultBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Name, Type are required.
type BetaCodeExecutionTool20250522Param struct {
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam `json:"cache_control,omitzero"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	//
	// This field can be elided, and will marshal its zero value as "code_execution".
	Name constant.CodeExecution `json:"name,required"`
	// This field can be elided, and will marshal its zero value as
	// "code_execution_20250522".
	Type constant.CodeExecution20250522 `json:"type,required"`
	paramObj
}

func (r BetaCodeExecutionTool20250522Param) MarshalJSON() (data []byte, err error) {
	type shadow BetaCodeExecutionTool20250522Param
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaCodeExecutionTool20250522Param) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaCodeExecutionToolResultBlock struct {
	Content   BetaCodeExecutionToolResultBlockContentUnion `json:"content,required"`
	ToolUseID string                                       `json:"tool_use_id,required"`
	Type      constant.CodeExecutionToolResult             `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Content     respjson.Field
		ToolUseID   respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaCodeExecutionToolResultBlock) RawJSON() string { return r.JSON.raw }
func (r *BetaCodeExecutionToolResultBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// BetaCodeExecutionToolResultBlockContentUnion contains all possible properties
// and values from [BetaCodeExecutionToolResultError],
// [BetaCodeExecutionResultBlock].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type BetaCodeExecutionToolResultBlockContentUnion struct {
	// This field is from variant [BetaCodeExecutionToolResultError].
	ErrorCode BetaCodeExecutionToolResultErrorCode `json:"error_code"`
	Type      string                               `json:"type"`
	// This field is from variant [BetaCodeExecutionResultBlock].
	Content []BetaCodeExecutionOutputBlock `json:"content"`
	// This field is from variant [BetaCodeExecutionResultBlock].
	ReturnCode int64 `json:"return_code"`
	// This field is from variant [BetaCodeExecutionResultBlock].
	Stderr string `json:"stderr"`
	// This field is from variant [BetaCodeExecutionResultBlock].
	Stdout string `json:"stdout"`
	JSON   struct {
		ErrorCode  respjson.Field
		Type       respjson.Field
		Content    respjson.Field
		ReturnCode respjson.Field
		Stderr     respjson.Field
		Stdout     respjson.Field
		raw        string
	} `json:"-"`
}

func (u BetaCodeExecutionToolResultBlockContentUnion) AsResponseCodeExecutionToolResultError() (v BetaCodeExecutionToolResultError) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaCodeExecutionToolResultBlockContentUnion) AsResponseCodeExecutionResultBlock() (v BetaCodeExecutionResultBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u BetaCodeExecutionToolResultBlockContentUnion) RawJSON() string { return u.JSON.raw }

func (r *BetaCodeExecutionToolResultBlockContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Content, ToolUseID, Type are required.
type BetaCodeExecutionToolResultBlockParam struct {
	Content   BetaCodeExecutionToolResultBlockParamContentUnion `json:"content,omitzero,required"`
	ToolUseID string                                            `json:"tool_use_id,required"`
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam `json:"cache_control,omitzero"`
	// This field can be elided, and will marshal its zero value as
	// "code_execution_tool_result".
	Type constant.CodeExecutionToolResult `json:"type,required"`
	paramObj
}

func (r BetaCodeExecutionToolResultBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaCodeExecutionToolResultBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaCodeExecutionToolResultBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func BetaNewCodeExecutionToolRequestError(errorCode BetaCodeExecutionToolResultErrorCode) BetaCodeExecutionToolResultBlockParamContentUnion {
	var variant BetaCodeExecutionToolResultErrorParam
	variant.ErrorCode = errorCode
	return BetaCodeExecutionToolResultBlockParamContentUnion{OfError: &variant}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaCodeExecutionToolResultBlockParamContentUnion struct {
	OfError       *BetaCodeExecutionToolResultErrorParam `json:",omitzero,inline"`
	OfResultBlock *BetaCodeExecutionResultBlockParam     `json:",omitzero,inline"`
	paramUnion
}

func (u BetaCodeExecutionToolResultBlockParamContentUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfError, u.OfResultBlock)
}
func (u *BetaCodeExecutionToolResultBlockParamContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaCodeExecutionToolResultBlockParamContentUnion) asAny() any {
	if !param.IsOmitted(u.OfError) {
		return u.OfError
	} else if !param.IsOmitted(u.OfResultBlock) {
		return u.OfResultBlock
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaCodeExecutionToolResultBlockParamContentUnion) GetErrorCode() *string {
	if vt := u.OfError; vt != nil {
		return (*string)(&vt.ErrorCode)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaCodeExecutionToolResultBlockParamContentUnion) GetContent() []BetaCodeExecutionOutputBlockParam {
	if vt := u.OfResultBlock; vt != nil {
		return vt.Content
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaCodeExecutionToolResultBlockParamContentUnion) GetReturnCode() *int64 {
	if vt := u.OfResultBlock; vt != nil {
		return &vt.ReturnCode
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaCodeExecutionToolResultBlockParamContentUnion) GetStderr() *string {
	if vt := u.OfResultBlock; vt != nil {
		return &vt.Stderr
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaCodeExecutionToolResultBlockParamContentUnion) GetStdout() *string {
	if vt := u.OfResultBlock; vt != nil {
		return &vt.Stdout
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaCodeExecutionToolResultBlockParamContentUnion) GetType() *string {
	if vt := u.OfError; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfResultBlock; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

type BetaCodeExecutionToolResultError struct {
	// Any of "invalid_tool_input", "unavailable", "too_many_requests",
	// "execution_time_exceeded".
	ErrorCode BetaCodeExecutionToolResultErrorCode  `json:"error_code,required"`
	Type      constant.CodeExecutionToolResultError `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ErrorCode   respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaCodeExecutionToolResultError) RawJSON() string { return r.JSON.raw }
func (r *BetaCodeExecutionToolResultError) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaCodeExecutionToolResultErrorCode string

const (
	BetaCodeExecutionToolResultErrorCodeInvalidToolInput      BetaCodeExecutionToolResultErrorCode = "invalid_tool_input"
	BetaCodeExecutionToolResultErrorCodeUnavailable           BetaCodeExecutionToolResultErrorCode = "unavailable"
	BetaCodeExecutionToolResultErrorCodeTooManyRequests       BetaCodeExecutionToolResultErrorCode = "too_many_requests"
	BetaCodeExecutionToolResultErrorCodeExecutionTimeExceeded BetaCodeExecutionToolResultErrorCode = "execution_time_exceeded"
)

// The properties ErrorCode, Type are required.
type BetaCodeExecutionToolResultErrorParam struct {
	// Any of "invalid_tool_input", "unavailable", "too_many_requests",
	// "execution_time_exceeded".
	ErrorCode BetaCodeExecutionToolResultErrorCode `json:"error_code,omitzero,required"`
	// This field can be elided, and will marshal its zero value as
	// "code_execution_tool_result_error".
	Type constant.CodeExecutionToolResultError `json:"type,required"`
	paramObj
}

func (r BetaCodeExecutionToolResultErrorParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaCodeExecutionToolResultErrorParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaCodeExecutionToolResultErrorParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Information about the container used in the request (for the code execution
// tool)
type BetaContainer struct {
	// Identifier for the container used in this request
	ID string `json:"id,required"`
	// The time at which the container will expire.
	ExpiresAt time.Time `json:"expires_at,required" format:"date-time"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ExpiresAt   respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaContainer) RawJSON() string { return r.JSON.raw }
func (r *BetaContainer) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Response model for a file uploaded to the container.
type BetaContainerUploadBlock struct {
	FileID string                   `json:"file_id,required"`
	Type   constant.ContainerUpload `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		FileID      respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaContainerUploadBlock) RawJSON() string { return r.JSON.raw }
func (r *BetaContainerUploadBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A content block that represents a file to be uploaded to the container Files
// uploaded via this block will be available in the container's input directory.
//
// The properties FileID, Type are required.
type BetaContainerUploadBlockParam struct {
	FileID string `json:"file_id,required"`
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam `json:"cache_control,omitzero"`
	// This field can be elided, and will marshal its zero value as "container_upload".
	Type constant.ContainerUpload `json:"type,required"`
	paramObj
}

func (r BetaContainerUploadBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaContainerUploadBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaContainerUploadBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// BetaContentBlockUnion contains all possible properties and values from
// [BetaTextBlock], [BetaToolUseBlock], [BetaServerToolUseBlock],
// [BetaWebSearchToolResultBlock], [BetaCodeExecutionToolResultBlock],
// [BetaMCPToolUseBlock], [BetaMCPToolResultBlock], [BetaContainerUploadBlock],
// [BetaThinkingBlock], [BetaRedactedThinkingBlock].
//
// Use the [BetaContentBlockUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type BetaContentBlockUnion struct {
	// This field is from variant [BetaTextBlock].
	Citations []BetaTextCitationUnion `json:"citations"`
	// This field is from variant [BetaTextBlock].
	Text string `json:"text"`
	// Any of "text", "tool_use", "server_tool_use", "web_search_tool_result",
	// "code_execution_tool_result", "mcp_tool_use", "mcp_tool_result",
	// "container_upload", "thinking", "redacted_thinking".
	Type  string          `json:"type"`
	ID    string          `json:"id"`
	Input json.RawMessage `json:"input"`
	Name  string          `json:"name"`
	// This field is a union of [BetaWebSearchToolResultBlockContentUnion],
	// [BetaCodeExecutionToolResultBlockContentUnion],
	// [BetaMCPToolResultBlockContentUnion]
	Content   BetaContentBlockUnionContent `json:"content"`
	ToolUseID string                       `json:"tool_use_id"`
	// This field is from variant [BetaMCPToolUseBlock].
	ServerName string `json:"server_name"`
	// This field is from variant [BetaMCPToolResultBlock].
	IsError bool `json:"is_error"`
	// This field is from variant [BetaContainerUploadBlock].
	FileID string `json:"file_id"`
	// This field is from variant [BetaThinkingBlock].
	Signature string `json:"signature"`
	// This field is from variant [BetaThinkingBlock].
	Thinking string `json:"thinking"`
	// This field is from variant [BetaRedactedThinkingBlock].
	Data string `json:"data"`
	JSON struct {
		Citations  respjson.Field
		Text       respjson.Field
		Type       respjson.Field
		ID         respjson.Field
		Input      respjson.Field
		Name       respjson.Field
		Content    respjson.Field
		ToolUseID  respjson.Field
		ServerName respjson.Field
		IsError    respjson.Field
		FileID     respjson.Field
		Signature  respjson.Field
		Thinking   respjson.Field
		Data       respjson.Field
		raw        string
	} `json:"-"`
}

func (r BetaContentBlockUnion) ToParam() BetaContentBlockParamUnion {
	switch variant := r.AsAny().(type) {
	case BetaTextBlock:
		p := variant.ToParam()
		return BetaContentBlockParamUnion{OfText: &p}
	case BetaToolUseBlock:
		p := variant.ToParam()
		return BetaContentBlockParamUnion{OfToolUse: &p}
	case BetaThinkingBlock:
		p := variant.ToParam()
		return BetaContentBlockParamUnion{OfThinking: &p}
	case BetaRedactedThinkingBlock:
		p := variant.ToParam()
		return BetaContentBlockParamUnion{OfRedactedThinking: &p}
	}
	return BetaContentBlockParamUnion{}
}

// anyBetaContentBlock is implemented by each variant of [BetaContentBlockUnion] to
// add type safety for the return type of [BetaContentBlockUnion.AsAny]
type anyBetaContentBlock interface {
	implBetaContentBlockUnion()
}

func (BetaTextBlock) implBetaContentBlockUnion()                    {}
func (BetaToolUseBlock) implBetaContentBlockUnion()                 {}
func (BetaServerToolUseBlock) implBetaContentBlockUnion()           {}
func (BetaWebSearchToolResultBlock) implBetaContentBlockUnion()     {}
func (BetaCodeExecutionToolResultBlock) implBetaContentBlockUnion() {}
func (BetaMCPToolUseBlock) implBetaContentBlockUnion()              {}
func (BetaMCPToolResultBlock) implBetaContentBlockUnion()           {}
func (BetaContainerUploadBlock) implBetaContentBlockUnion()         {}
func (BetaThinkingBlock) implBetaContentBlockUnion()                {}
func (BetaRedactedThinkingBlock) implBetaContentBlockUnion()        {}

// Use the following switch statement to find the correct variant
//
//	switch variant := BetaContentBlockUnion.AsAny().(type) {
//	case anthropic.BetaTextBlock:
//	case anthropic.BetaToolUseBlock:
//	case anthropic.BetaServerToolUseBlock:
//	case anthropic.BetaWebSearchToolResultBlock:
//	case anthropic.BetaCodeExecutionToolResultBlock:
//	case anthropic.BetaMCPToolUseBlock:
//	case anthropic.BetaMCPToolResultBlock:
//	case anthropic.BetaContainerUploadBlock:
//	case anthropic.BetaThinkingBlock:
//	case anthropic.BetaRedactedThinkingBlock:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u BetaContentBlockUnion) AsAny() anyBetaContentBlock {
	switch u.Type {
	case "text":
		return u.AsText()
	case "tool_use":
		return u.AsToolUse()
	case "server_tool_use":
		return u.AsServerToolUse()
	case "web_search_tool_result":
		return u.AsWebSearchToolResult()
	case "code_execution_tool_result":
		return u.AsCodeExecutionToolResult()
	case "mcp_tool_use":
		return u.AsMCPToolUse()
	case "mcp_tool_result":
		return u.AsMCPToolResult()
	case "container_upload":
		return u.AsContainerUpload()
	case "thinking":
		return u.AsThinking()
	case "redacted_thinking":
		return u.AsRedactedThinking()
	}
	return nil
}

func (u BetaContentBlockUnion) AsText() (v BetaTextBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaContentBlockUnion) AsToolUse() (v BetaToolUseBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaContentBlockUnion) AsServerToolUse() (v BetaServerToolUseBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaContentBlockUnion) AsWebSearchToolResult() (v BetaWebSearchToolResultBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaContentBlockUnion) AsCodeExecutionToolResult() (v BetaCodeExecutionToolResultBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaContentBlockUnion) AsMCPToolUse() (v BetaMCPToolUseBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaContentBlockUnion) AsMCPToolResult() (v BetaMCPToolResultBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaContentBlockUnion) AsContainerUpload() (v BetaContainerUploadBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaContentBlockUnion) AsThinking() (v BetaThinkingBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaContentBlockUnion) AsRedactedThinking() (v BetaRedactedThinkingBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u BetaContentBlockUnion) RawJSON() string { return u.JSON.raw }

func (r *BetaContentBlockUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// BetaContentBlockUnionContent is an implicit subunion of [BetaContentBlockUnion].
// BetaContentBlockUnionContent provides convenient access to the sub-properties of
// the union.
//
// For type safety it is recommended to directly use a variant of the
// [BetaContentBlockUnion].
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfBetaWebSearchResultBlockArray OfString
// OfBetaMCPToolResultBlockContent]
type BetaContentBlockUnionContent struct {
	// This field will be present if the value is a [[]BetaWebSearchResultBlock]
	// instead of an object.
	OfBetaWebSearchResultBlockArray []BetaWebSearchResultBlock `json:",inline"`
	// This field will be present if the value is a [string] instead of an object.
	OfString string `json:",inline"`
	// This field will be present if the value is a [[]BetaTextBlock] instead of an
	// object.
	OfBetaMCPToolResultBlockContent []BetaTextBlock `json:",inline"`
	ErrorCode                       string          `json:"error_code"`
	Type                            string          `json:"type"`
	// This field is from variant [BetaCodeExecutionToolResultBlockContentUnion].
	Content []BetaCodeExecutionOutputBlock `json:"content"`
	// This field is from variant [BetaCodeExecutionToolResultBlockContentUnion].
	ReturnCode int64 `json:"return_code"`
	// This field is from variant [BetaCodeExecutionToolResultBlockContentUnion].
	Stderr string `json:"stderr"`
	// This field is from variant [BetaCodeExecutionToolResultBlockContentUnion].
	Stdout string `json:"stdout"`
	JSON   struct {
		OfBetaWebSearchResultBlockArray respjson.Field
		OfString                        respjson.Field
		OfBetaMCPToolResultBlockContent respjson.Field
		ErrorCode                       respjson.Field
		Type                            respjson.Field
		Content                         respjson.Field
		ReturnCode                      respjson.Field
		Stderr                          respjson.Field
		Stdout                          respjson.Field
		raw                             string
	} `json:"-"`
}

func (r *BetaContentBlockUnionContent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func NewBetaServerToolUseBlock(id string, input any, name BetaServerToolUseBlockParamName) BetaContentBlockParamUnion {
	var serverToolUse BetaServerToolUseBlockParam
	serverToolUse.ID = id
	serverToolUse.Input = input
	serverToolUse.Name = name
	return BetaContentBlockParamUnion{OfServerToolUse: &serverToolUse}
}

func NewBetaWebSearchToolResultBlock[
	T []BetaWebSearchResultBlockParam | BetaWebSearchToolRequestErrorParam,
](content T, toolUseID string) BetaContentBlockParamUnion {
	var webSearchToolResult BetaWebSearchToolResultBlockParam
	switch v := any(content).(type) {
	case []BetaWebSearchResultBlockParam:
		webSearchToolResult.Content.OfResultBlock = v
	case BetaWebSearchToolRequestErrorParam:
		webSearchToolResult.Content.OfError = &v
	}
	webSearchToolResult.ToolUseID = toolUseID
	return BetaContentBlockParamUnion{OfWebSearchToolResult: &webSearchToolResult}
}

func NewBetaCodeExecutionToolResultBlock[
	T BetaCodeExecutionToolResultErrorParam | BetaCodeExecutionResultBlockParam,
](content T, toolUseID string) BetaContentBlockParamUnion {
	var codeExecutionToolResult BetaCodeExecutionToolResultBlockParam
	switch v := any(content).(type) {
	case BetaCodeExecutionToolResultErrorParam:
		codeExecutionToolResult.Content.OfError = &v
	case BetaCodeExecutionResultBlockParam:
		codeExecutionToolResult.Content.OfResultBlock = &v
	}
	codeExecutionToolResult.ToolUseID = toolUseID
	return BetaContentBlockParamUnion{OfCodeExecutionToolResult: &codeExecutionToolResult}
}

func NewBetaMCPToolResultBlock(toolUseID string) BetaContentBlockParamUnion {
	var mcpToolResult BetaRequestMCPToolResultBlockParam
	mcpToolResult.ToolUseID = toolUseID
	return BetaContentBlockParamUnion{OfMCPToolResult: &mcpToolResult}
}

func NewBetaTextBlock(text string) BetaContentBlockParamUnion {
	var variant BetaTextBlockParam
	variant.Text = text
	return BetaContentBlockParamUnion{OfText: &variant}
}

func NewBetaImageBlock[
	T BetaBase64ImageSourceParam | BetaURLImageSourceParam | BetaFileImageSourceParam,
](source T) BetaContentBlockParamUnion {
	var image BetaImageBlockParam
	switch v := any(source).(type) {
	case BetaBase64ImageSourceParam:
		image.Source.OfBase64 = &v
	case BetaURLImageSourceParam:
		image.Source.OfURL = &v
	case BetaFileImageSourceParam:
		image.Source.OfFile = &v
	}
	return BetaContentBlockParamUnion{OfImage: &image}
}

func NewBetaToolUseBlock(id string, input any, name string) BetaContentBlockParamUnion {
	var toolUse BetaToolUseBlockParam
	toolUse.ID = id
	toolUse.Input = input
	toolUse.Name = name
	return BetaContentBlockParamUnion{OfToolUse: &toolUse}
}

func NewBetaToolResultBlock(toolUseID string, content string, isError bool) BetaContentBlockParamUnion {
	toolResult := BetaToolResultBlockParam{
		Content: []BetaToolResultBlockParamContentUnion{
			{OfText: &BetaTextBlockParam{Text: content}},
		},
		ToolUseID: toolUseID,
		IsError:   Bool(isError),
	}
	return BetaContentBlockParamUnion{OfToolResult: &toolResult}
}

func NewBetaDocumentBlock[
	T BetaBase64PDFSourceParam | BetaPlainTextSourceParam | BetaContentBlockSourceParam | BetaURLPDFSourceParam | BetaFileDocumentSourceParam,
](source T) BetaContentBlockParamUnion {
	var document BetaBase64PDFBlockParam
	switch v := any(source).(type) {
	case BetaBase64PDFSourceParam:
		document.Source.OfBase64 = &v
	case BetaPlainTextSourceParam:
		document.Source.OfText = &v
	case BetaContentBlockSourceParam:
		document.Source.OfContent = &v
	case BetaURLPDFSourceParam:
		document.Source.OfURL = &v
	case BetaFileDocumentSourceParam:
		document.Source.OfFile = &v
	}
	return BetaContentBlockParamUnion{OfDocument: &document}
}

func NewBetaThinkingBlock(signature string, thinking string) BetaContentBlockParamUnion {
	var variant BetaThinkingBlockParam
	variant.Signature = signature
	variant.Thinking = thinking
	return BetaContentBlockParamUnion{OfThinking: &variant}
}

func NewBetaRedactedThinkingBlock(data string) BetaContentBlockParamUnion {
	var redactedThinking BetaRedactedThinkingBlockParam
	redactedThinking.Data = data
	return BetaContentBlockParamUnion{OfRedactedThinking: &redactedThinking}
}

func NewBetaContainerUploadBlock(fileID string) BetaContentBlockParamUnion {
	var containerUpload BetaContainerUploadBlockParam
	containerUpload.FileID = fileID
	return BetaContentBlockParamUnion{OfContainerUpload: &containerUpload}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaContentBlockParamUnion struct {
	OfServerToolUse           *BetaServerToolUseBlockParam           `json:",omitzero,inline"`
	OfWebSearchToolResult     *BetaWebSearchToolResultBlockParam     `json:",omitzero,inline"`
	OfCodeExecutionToolResult *BetaCodeExecutionToolResultBlockParam `json:",omitzero,inline"`
	OfMCPToolUse              *BetaMCPToolUseBlockParam              `json:",omitzero,inline"`
	OfMCPToolResult           *BetaRequestMCPToolResultBlockParam    `json:",omitzero,inline"`
	OfText                    *BetaTextBlockParam                    `json:",omitzero,inline"`
	OfImage                   *BetaImageBlockParam                   `json:",omitzero,inline"`
	OfToolUse                 *BetaToolUseBlockParam                 `json:",omitzero,inline"`
	OfToolResult              *BetaToolResultBlockParam              `json:",omitzero,inline"`
	OfDocument                *BetaBase64PDFBlockParam               `json:",omitzero,inline"`
	OfThinking                *BetaThinkingBlockParam                `json:",omitzero,inline"`
	OfRedactedThinking        *BetaRedactedThinkingBlockParam        `json:",omitzero,inline"`
	OfContainerUpload         *BetaContainerUploadBlockParam         `json:",omitzero,inline"`
	paramUnion
}

func (u BetaContentBlockParamUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfServerToolUse,
		u.OfWebSearchToolResult,
		u.OfCodeExecutionToolResult,
		u.OfMCPToolUse,
		u.OfMCPToolResult,
		u.OfText,
		u.OfImage,
		u.OfToolUse,
		u.OfToolResult,
		u.OfDocument,
		u.OfThinking,
		u.OfRedactedThinking,
		u.OfContainerUpload)
}
func (u *BetaContentBlockParamUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaContentBlockParamUnion) asAny() any {
	if !param.IsOmitted(u.OfServerToolUse) {
		return u.OfServerToolUse
	} else if !param.IsOmitted(u.OfWebSearchToolResult) {
		return u.OfWebSearchToolResult
	} else if !param.IsOmitted(u.OfCodeExecutionToolResult) {
		return u.OfCodeExecutionToolResult
	} else if !param.IsOmitted(u.OfMCPToolUse) {
		return u.OfMCPToolUse
	} else if !param.IsOmitted(u.OfMCPToolResult) {
		return u.OfMCPToolResult
	} else if !param.IsOmitted(u.OfText) {
		return u.OfText
	} else if !param.IsOmitted(u.OfImage) {
		return u.OfImage
	} else if !param.IsOmitted(u.OfToolUse) {
		return u.OfToolUse
	} else if !param.IsOmitted(u.OfToolResult) {
		return u.OfToolResult
	} else if !param.IsOmitted(u.OfDocument) {
		return u.OfDocument
	} else if !param.IsOmitted(u.OfThinking) {
		return u.OfThinking
	} else if !param.IsOmitted(u.OfRedactedThinking) {
		return u.OfRedactedThinking
	} else if !param.IsOmitted(u.OfContainerUpload) {
		return u.OfContainerUpload
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaContentBlockParamUnion) GetServerName() *string {
	if vt := u.OfMCPToolUse; vt != nil {
		return &vt.ServerName
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaContentBlockParamUnion) GetText() *string {
	if vt := u.OfText; vt != nil {
		return &vt.Text
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaContentBlockParamUnion) GetContext() *string {
	if vt := u.OfDocument; vt != nil && vt.Context.Valid() {
		return &vt.Context.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaContentBlockParamUnion) GetTitle() *string {
	if vt := u.OfDocument; vt != nil && vt.Title.Valid() {
		return &vt.Title.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaContentBlockParamUnion) GetSignature() *string {
	if vt := u.OfThinking; vt != nil {
		return &vt.Signature
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaContentBlockParamUnion) GetThinking() *string {
	if vt := u.OfThinking; vt != nil {
		return &vt.Thinking
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaContentBlockParamUnion) GetData() *string {
	if vt := u.OfRedactedThinking; vt != nil {
		return &vt.Data
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaContentBlockParamUnion) GetFileID() *string {
	if vt := u.OfContainerUpload; vt != nil {
		return &vt.FileID
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaContentBlockParamUnion) GetID() *string {
	if vt := u.OfServerToolUse; vt != nil {
		return (*string)(&vt.ID)
	} else if vt := u.OfMCPToolUse; vt != nil {
		return (*string)(&vt.ID)
	} else if vt := u.OfToolUse; vt != nil {
		return (*string)(&vt.ID)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaContentBlockParamUnion) GetName() *string {
	if vt := u.OfServerToolUse; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfMCPToolUse; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfToolUse; vt != nil {
		return (*string)(&vt.Name)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaContentBlockParamUnion) GetType() *string {
	if vt := u.OfServerToolUse; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfWebSearchToolResult; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfCodeExecutionToolResult; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfMCPToolUse; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfMCPToolResult; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfText; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfImage; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfToolUse; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfToolResult; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfDocument; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfThinking; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfRedactedThinking; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfContainerUpload; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaContentBlockParamUnion) GetToolUseID() *string {
	if vt := u.OfWebSearchToolResult; vt != nil {
		return (*string)(&vt.ToolUseID)
	} else if vt := u.OfCodeExecutionToolResult; vt != nil {
		return (*string)(&vt.ToolUseID)
	} else if vt := u.OfMCPToolResult; vt != nil {
		return (*string)(&vt.ToolUseID)
	} else if vt := u.OfToolResult; vt != nil {
		return (*string)(&vt.ToolUseID)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaContentBlockParamUnion) GetIsError() *bool {
	if vt := u.OfMCPToolResult; vt != nil && vt.IsError.Valid() {
		return &vt.IsError.Value
	} else if vt := u.OfToolResult; vt != nil && vt.IsError.Valid() {
		return &vt.IsError.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's Input property, if present.
func (u BetaContentBlockParamUnion) GetInput() *any {
	if vt := u.OfServerToolUse; vt != nil {
		return &vt.Input
	} else if vt := u.OfMCPToolUse; vt != nil {
		return &vt.Input
	} else if vt := u.OfToolUse; vt != nil {
		return &vt.Input
	}
	return nil
}

// Returns a pointer to the underlying variant's CacheControl property, if present.
func (u BetaContentBlockParamUnion) GetCacheControl() *BetaCacheControlEphemeralParam {
	if vt := u.OfServerToolUse; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfWebSearchToolResult; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfCodeExecutionToolResult; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfMCPToolUse; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfMCPToolResult; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfText; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfImage; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfToolUse; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfToolResult; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfDocument; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfContainerUpload; vt != nil {
		return &vt.CacheControl
	}
	return nil
}

// Returns a subunion which exports methods to access subproperties
//
// Or use AsAny() to get the underlying value
func (u BetaContentBlockParamUnion) GetContent() (res betaContentBlockParamUnionContent) {
	if vt := u.OfWebSearchToolResult; vt != nil {
		res.any = vt.Content.asAny()
	} else if vt := u.OfCodeExecutionToolResult; vt != nil {
		res.any = vt.Content.asAny()
	} else if vt := u.OfMCPToolResult; vt != nil {
		res.any = vt.Content.asAny()
	} else if vt := u.OfToolResult; vt != nil {
		res.any = &vt.Content
	}
	return
}

// Can have the runtime types [*[]BetaWebSearchResultBlockParam],
// [*BetaCodeExecutionToolResultErrorParam], [*BetaCodeExecutionResultBlockParam],
// [*string], [_[]BetaTextBlockParam], [_[]BetaToolResultBlockParamContentUnion]
type betaContentBlockParamUnionContent struct{ any }

// Use the following switch statement to get the type of the union:
//
//	switch u.AsAny().(type) {
//	case *[]anthropic.BetaWebSearchResultBlockParam:
//	case *anthropic.BetaCodeExecutionToolResultErrorParam:
//	case *anthropic.BetaCodeExecutionResultBlockParam:
//	case *string:
//	case *[]anthropic.BetaTextBlockParam:
//	case *[]anthropic.BetaToolResultBlockParamContentUnion:
//	default:
//	    fmt.Errorf("not present")
//	}
func (u betaContentBlockParamUnionContent) AsAny() any { return u.any }

// Returns a pointer to the underlying variant's property, if present.
func (u betaContentBlockParamUnionContent) GetContent() []BetaCodeExecutionOutputBlockParam {
	switch vt := u.any.(type) {
	case *BetaCodeExecutionToolResultBlockParamContentUnion:
		return vt.GetContent()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u betaContentBlockParamUnionContent) GetReturnCode() *int64 {
	switch vt := u.any.(type) {
	case *BetaCodeExecutionToolResultBlockParamContentUnion:
		return vt.GetReturnCode()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u betaContentBlockParamUnionContent) GetStderr() *string {
	switch vt := u.any.(type) {
	case *BetaCodeExecutionToolResultBlockParamContentUnion:
		return vt.GetStderr()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u betaContentBlockParamUnionContent) GetStdout() *string {
	switch vt := u.any.(type) {
	case *BetaCodeExecutionToolResultBlockParamContentUnion:
		return vt.GetStdout()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u betaContentBlockParamUnionContent) GetErrorCode() *string {
	switch vt := u.any.(type) {
	case *BetaWebSearchToolResultBlockParamContentUnion:
		if vt := vt.OfError; vt != nil {
			return (*string)(&vt.ErrorCode)
		}
	case *BetaCodeExecutionToolResultBlockParamContentUnion:
		return vt.GetErrorCode()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u betaContentBlockParamUnionContent) GetType() *string {
	switch vt := u.any.(type) {
	case *BetaWebSearchToolResultBlockParamContentUnion:
		if vt := vt.OfError; vt != nil {
			return (*string)(&vt.Type)
		}
	case *BetaCodeExecutionToolResultBlockParamContentUnion:
		return vt.GetType()
	}
	return nil
}

// Returns a subunion which exports methods to access subproperties
//
// Or use AsAny() to get the underlying value
func (u BetaContentBlockParamUnion) GetCitations() (res betaContentBlockParamUnionCitations) {
	if vt := u.OfText; vt != nil {
		res.any = &vt.Citations
	} else if vt := u.OfDocument; vt != nil {
		res.any = &vt.Citations
	}
	return
}

// Can have the runtime types [*[]BetaTextCitationParamUnion],
// [*BetaCitationsConfigParam]
type betaContentBlockParamUnionCitations struct{ any }

// Use the following switch statement to get the type of the union:
//
//	switch u.AsAny().(type) {
//	case *[]anthropic.BetaTextCitationParamUnion:
//	case *anthropic.BetaCitationsConfigParam:
//	default:
//	    fmt.Errorf("not present")
//	}
func (u betaContentBlockParamUnionCitations) AsAny() any { return u.any }

// Returns a subunion which exports methods to access subproperties
//
// Or use AsAny() to get the underlying value
func (u BetaContentBlockParamUnion) GetSource() (res betaContentBlockParamUnionSource) {
	if vt := u.OfImage; vt != nil {
		res.any = vt.Source.asAny()
	} else if vt := u.OfDocument; vt != nil {
		res.any = vt.Source.asAny()
	}
	return
}

// Can have the runtime types [*BetaBase64ImageSourceParam],
// [*BetaURLImageSourceParam], [*BetaFileImageSourceParam],
// [*BetaBase64PDFSourceParam], [*BetaPlainTextSourceParam],
// [*BetaContentBlockSourceParam], [*BetaURLPDFSourceParam],
// [*BetaFileDocumentSourceParam]
type betaContentBlockParamUnionSource struct{ any }

// Use the following switch statement to get the type of the union:
//
//	switch u.AsAny().(type) {
//	case *anthropic.BetaBase64ImageSourceParam:
//	case *anthropic.BetaURLImageSourceParam:
//	case *anthropic.BetaFileImageSourceParam:
//	case *anthropic.BetaBase64PDFSourceParam:
//	case *anthropic.BetaPlainTextSourceParam:
//	case *anthropic.BetaContentBlockSourceParam:
//	case *anthropic.BetaURLPDFSourceParam:
//	case *anthropic.BetaFileDocumentSourceParam:
//	default:
//	    fmt.Errorf("not present")
//	}
func (u betaContentBlockParamUnionSource) AsAny() any { return u.any }

// Returns a pointer to the underlying variant's property, if present.
func (u betaContentBlockParamUnionSource) GetContent() *BetaContentBlockSourceContentUnionParam {
	switch vt := u.any.(type) {
	case *BetaBase64PDFBlockSourceUnionParam:
		return vt.GetContent()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u betaContentBlockParamUnionSource) GetData() *string {
	switch vt := u.any.(type) {
	case *BetaImageBlockParamSourceUnion:
		return vt.GetData()
	case *BetaBase64PDFBlockSourceUnionParam:
		return vt.GetData()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u betaContentBlockParamUnionSource) GetMediaType() *string {
	switch vt := u.any.(type) {
	case *BetaImageBlockParamSourceUnion:
		return vt.GetMediaType()
	case *BetaBase64PDFBlockSourceUnionParam:
		return vt.GetMediaType()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u betaContentBlockParamUnionSource) GetType() *string {
	switch vt := u.any.(type) {
	case *BetaImageBlockParamSourceUnion:
		return vt.GetType()
	case *BetaBase64PDFBlockSourceUnionParam:
		return vt.GetType()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u betaContentBlockParamUnionSource) GetURL() *string {
	switch vt := u.any.(type) {
	case *BetaImageBlockParamSourceUnion:
		return vt.GetURL()
	case *BetaBase64PDFBlockSourceUnionParam:
		return vt.GetURL()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u betaContentBlockParamUnionSource) GetFileID() *string {
	switch vt := u.any.(type) {
	case *BetaImageBlockParamSourceUnion:
		return vt.GetFileID()
	case *BetaBase64PDFBlockSourceUnionParam:
		return vt.GetFileID()
	}
	return nil
}

// The properties Content, Type are required.
type BetaContentBlockSourceParam struct {
	Content BetaContentBlockSourceContentUnionParam `json:"content,omitzero,required"`
	// This field can be elided, and will marshal its zero value as "content".
	Type constant.Content `json:"type,required"`
	paramObj
}

func (r BetaContentBlockSourceParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaContentBlockSourceParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaContentBlockSourceParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaContentBlockSourceContentUnionParam struct {
	OfString                        param.Opt[string]                         `json:",omitzero,inline"`
	OfBetaContentBlockSourceContent []BetaContentBlockSourceContentUnionParam `json:",omitzero,inline"`
	paramUnion
}

func (u BetaContentBlockSourceContentUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfBetaContentBlockSourceContent)
}
func (u *BetaContentBlockSourceContentUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaContentBlockSourceContentUnionParam) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfBetaContentBlockSourceContent) {
		return &u.OfBetaContentBlockSourceContent
	}
	return nil
}

// The properties FileID, Type are required.
type BetaFileDocumentSourceParam struct {
	FileID string `json:"file_id,required"`
	// This field can be elided, and will marshal its zero value as "file".
	Type constant.File `json:"type,required"`
	paramObj
}

func (r BetaFileDocumentSourceParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaFileDocumentSourceParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaFileDocumentSourceParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties FileID, Type are required.
type BetaFileImageSourceParam struct {
	FileID string `json:"file_id,required"`
	// This field can be elided, and will marshal its zero value as "file".
	Type constant.File `json:"type,required"`
	paramObj
}

func (r BetaFileImageSourceParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaFileImageSourceParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaFileImageSourceParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Source, Type are required.
type BetaImageBlockParam struct {
	Source BetaImageBlockParamSourceUnion `json:"source,omitzero,required"`
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam `json:"cache_control,omitzero"`
	// This field can be elided, and will marshal its zero value as "image".
	Type constant.Image `json:"type,required"`
	paramObj
}

func (r BetaImageBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaImageBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaImageBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaImageBlockParamSourceUnion struct {
	OfBase64 *BetaBase64ImageSourceParam `json:",omitzero,inline"`
	OfURL    *BetaURLImageSourceParam    `json:",omitzero,inline"`
	OfFile   *BetaFileImageSourceParam   `json:",omitzero,inline"`
	paramUnion
}

func (u BetaImageBlockParamSourceUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfBase64, u.OfURL, u.OfFile)
}
func (u *BetaImageBlockParamSourceUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaImageBlockParamSourceUnion) asAny() any {
	if !param.IsOmitted(u.OfBase64) {
		return u.OfBase64
	} else if !param.IsOmitted(u.OfURL) {
		return u.OfURL
	} else if !param.IsOmitted(u.OfFile) {
		return u.OfFile
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaImageBlockParamSourceUnion) GetData() *string {
	if vt := u.OfBase64; vt != nil {
		return &vt.Data
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaImageBlockParamSourceUnion) GetMediaType() *string {
	if vt := u.OfBase64; vt != nil {
		return (*string)(&vt.MediaType)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaImageBlockParamSourceUnion) GetURL() *string {
	if vt := u.OfURL; vt != nil {
		return &vt.URL
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaImageBlockParamSourceUnion) GetFileID() *string {
	if vt := u.OfFile; vt != nil {
		return &vt.FileID
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaImageBlockParamSourceUnion) GetType() *string {
	if vt := u.OfBase64; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfURL; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfFile; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

type BetaInputJSONDelta struct {
	PartialJSON string                  `json:"partial_json,required"`
	Type        constant.InputJSONDelta `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		PartialJSON respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaInputJSONDelta) RawJSON() string { return r.JSON.raw }
func (r *BetaInputJSONDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaMCPToolResultBlock struct {
	Content   BetaMCPToolResultBlockContentUnion `json:"content,required"`
	IsError   bool                               `json:"is_error,required"`
	ToolUseID string                             `json:"tool_use_id,required"`
	Type      constant.MCPToolResult             `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Content     respjson.Field
		IsError     respjson.Field
		ToolUseID   respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaMCPToolResultBlock) RawJSON() string { return r.JSON.raw }
func (r *BetaMCPToolResultBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// BetaMCPToolResultBlockContentUnion contains all possible properties and values
// from [string], [[]BetaTextBlock].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfString OfBetaMCPToolResultBlockContent]
type BetaMCPToolResultBlockContentUnion struct {
	// This field will be present if the value is a [string] instead of an object.
	OfString string `json:",inline"`
	// This field will be present if the value is a [[]BetaTextBlock] instead of an
	// object.
	OfBetaMCPToolResultBlockContent []BetaTextBlock `json:",inline"`
	JSON                            struct {
		OfString                        respjson.Field
		OfBetaMCPToolResultBlockContent respjson.Field
		raw                             string
	} `json:"-"`
}

func (u BetaMCPToolResultBlockContentUnion) AsString() (v string) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaMCPToolResultBlockContentUnion) AsBetaMCPToolResultBlockContent() (v []BetaTextBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u BetaMCPToolResultBlockContentUnion) RawJSON() string { return u.JSON.raw }

func (r *BetaMCPToolResultBlockContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaMCPToolUseBlock struct {
	ID    string `json:"id,required"`
	Input any    `json:"input,required"`
	// The name of the MCP tool
	Name string `json:"name,required"`
	// The name of the MCP server
	ServerName string              `json:"server_name,required"`
	Type       constant.MCPToolUse `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Input       respjson.Field
		Name        respjson.Field
		ServerName  respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaMCPToolUseBlock) RawJSON() string { return r.JSON.raw }
func (r *BetaMCPToolUseBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties ID, Input, Name, ServerName, Type are required.
type BetaMCPToolUseBlockParam struct {
	ID    string `json:"id,required"`
	Input any    `json:"input,omitzero,required"`
	Name  string `json:"name,required"`
	// The name of the MCP server
	ServerName string `json:"server_name,required"`
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam `json:"cache_control,omitzero"`
	// This field can be elided, and will marshal its zero value as "mcp_tool_use".
	Type constant.MCPToolUse `json:"type,required"`
	paramObj
}

func (r BetaMCPToolUseBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaMCPToolUseBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaMCPToolUseBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaMessage struct {
	// Unique object identifier.
	//
	// The format and length of IDs may change over time.
	ID string `json:"id,required"`
	// Information about the container used in the request (for the code execution
	// tool)
	Container BetaContainer `json:"container,required"`
	// Content generated by the model.
	//
	// This is an array of content blocks, each of which has a `type` that determines
	// its shape.
	//
	// Example:
	//
	// ```json
	// [{ "type": "text", "text": "Hi, I'm Claude." }]
	// ```
	//
	// If the request input `messages` ended with an `assistant` turn, then the
	// response `content` will continue directly from that last turn. You can use this
	// to constrain the model's output.
	//
	// For example, if the input `messages` were:
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
	// Then the response `content` might be:
	//
	// ```json
	// [{ "type": "text", "text": "B)" }]
	// ```
	Content []BetaContentBlockUnion `json:"content,required"`
	// The model that will complete your prompt.\n\nSee
	// [models](https://docs.anthropic.com/en/docs/models-overview) for additional
	// details and options.
	Model Model `json:"model,required"`
	// Conversational role of the generated message.
	//
	// This will always be `"assistant"`.
	Role constant.Assistant `json:"role,required"`
	// The reason that we stopped.
	//
	// This may be one the following values:
	//
	// - `"end_turn"`: the model reached a natural stopping point
	// - `"max_tokens"`: we exceeded the requested `max_tokens` or the model's maximum
	// - `"stop_sequence"`: one of your provided custom `stop_sequences` was generated
	// - `"tool_use"`: the model invoked one or more tools
	//
	// In non-streaming mode this value is always non-null. In streaming mode, it is
	// null in the `message_start` event and non-null otherwise.
	//
	// Any of "end_turn", "max_tokens", "stop_sequence", "tool_use", "pause_turn",
	// "refusal".
	StopReason BetaStopReason `json:"stop_reason,required"`
	// Which custom stop sequence was generated, if any.
	//
	// This value will be a non-null string if one of your custom stop sequences was
	// generated.
	StopSequence string `json:"stop_sequence,required"`
	// Object type.
	//
	// For Messages, this is always `"message"`.
	Type constant.Message `json:"type,required"`
	// Billing and rate-limit usage.
	//
	// Anthropic's API bills and rate-limits by token counts, as tokens represent the
	// underlying cost to our systems.
	//
	// Under the hood, the API transforms requests into a format suitable for the
	// model. The model's output then goes through a parsing stage before becoming an
	// API response. As a result, the token counts in `usage` will not match one-to-one
	// with the exact visible content of an API request or response.
	//
	// For example, `output_tokens` will be non-zero, even for an empty string response
	// from Claude.
	//
	// Total input tokens in a request is the summation of `input_tokens`,
	// `cache_creation_input_tokens`, and `cache_read_input_tokens`.
	Usage BetaUsage `json:"usage,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID           respjson.Field
		Container    respjson.Field
		Content      respjson.Field
		Model        respjson.Field
		Role         respjson.Field
		StopReason   respjson.Field
		StopSequence respjson.Field
		Type         respjson.Field
		Usage        respjson.Field
		ExtraFields  map[string]respjson.Field
		raw          string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaMessage) RawJSON() string { return r.JSON.raw }
func (r *BetaMessage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func (r BetaMessage) ToParam() BetaMessageParam {
	var p BetaMessageParam
	p.Role = BetaMessageParamRole(r.Role)
	p.Content = make([]BetaContentBlockParamUnion, len(r.Content))
	for i, c := range r.Content {
		contentParams := c.ToParam()
		p.Content[i] = contentParams
	}
	return p
}

// The reason that we stopped.
//
// This may be one the following values:
//
// - `"end_turn"`: the model reached a natural stopping point
// - `"max_tokens"`: we exceeded the requested `max_tokens` or the model's maximum
// - `"stop_sequence"`: one of your provided custom `stop_sequences` was generated
// - `"tool_use"`: the model invoked one or more tools
//
// In non-streaming mode this value is always non-null. In streaming mode, it is
// null in the `message_start` event and non-null otherwise.
type BetaMessageStopReason string

const (
	BetaMessageStopReasonEndTurn      BetaMessageStopReason = "end_turn"
	BetaMessageStopReasonMaxTokens    BetaMessageStopReason = "max_tokens"
	BetaMessageStopReasonStopSequence BetaMessageStopReason = "stop_sequence"
	BetaMessageStopReasonToolUse      BetaMessageStopReason = "tool_use"
)

type BetaMessageDeltaUsage struct {
	// The cumulative number of input tokens used to create the cache entry.
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens,required"`
	// The cumulative number of input tokens read from the cache.
	CacheReadInputTokens int64 `json:"cache_read_input_tokens,required"`
	// The cumulative number of input tokens which were used.
	InputTokens int64 `json:"input_tokens,required"`
	// The cumulative number of output tokens which were used.
	OutputTokens int64 `json:"output_tokens,required"`
	// The number of server tool requests.
	ServerToolUse BetaServerToolUsage `json:"server_tool_use,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		CacheCreationInputTokens respjson.Field
		CacheReadInputTokens     respjson.Field
		InputTokens              respjson.Field
		OutputTokens             respjson.Field
		ServerToolUse            respjson.Field
		ExtraFields              map[string]respjson.Field
		raw                      string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaMessageDeltaUsage) RawJSON() string { return r.JSON.raw }
func (r *BetaMessageDeltaUsage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Content, Role are required.
type BetaMessageParam struct {
	Content []BetaContentBlockParamUnion `json:"content,omitzero,required"`
	// Any of "user", "assistant".
	Role BetaMessageParamRole `json:"role,omitzero,required"`
	paramObj
}

func NewBetaUserMessage(blocks ...BetaContentBlockParamUnion) BetaMessageParam {
	return BetaMessageParam{
		Role:    BetaMessageParamRoleUser,
		Content: blocks,
	}
}

func (r BetaMessageParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaMessageParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaMessageParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaMessageParamRole string

const (
	BetaMessageParamRoleUser      BetaMessageParamRole = "user"
	BetaMessageParamRoleAssistant BetaMessageParamRole = "assistant"
)

type BetaMessageTokensCount struct {
	// The total number of tokens across the provided list of messages, system prompt,
	// and tools.
	InputTokens int64 `json:"input_tokens,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		InputTokens respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaMessageTokensCount) RawJSON() string { return r.JSON.raw }
func (r *BetaMessageTokensCount) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaMetadataParam struct {
	// An external identifier for the user who is associated with the request.
	//
	// This should be a uuid, hash value, or other opaque identifier. Anthropic may use
	// this id to help detect abuse. Do not include any identifying information such as
	// name, email address, or phone number.
	UserID param.Opt[string] `json:"user_id,omitzero"`
	paramObj
}

func (r BetaMetadataParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaMetadataParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaMetadataParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Data, MediaType, Type are required.
type BetaPlainTextSourceParam struct {
	Data string `json:"data,required"`
	// This field can be elided, and will marshal its zero value as "text/plain".
	MediaType constant.TextPlain `json:"media_type,required"`
	// This field can be elided, and will marshal its zero value as "text".
	Type constant.Text `json:"type,required"`
	paramObj
}

func (r BetaPlainTextSourceParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaPlainTextSourceParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaPlainTextSourceParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// BetaRawContentBlockDeltaUnion contains all possible properties and values from
// [BetaTextDelta], [BetaInputJSONDelta], [BetaCitationsDelta],
// [BetaThinkingDelta], [BetaSignatureDelta].
//
// Use the [BetaRawContentBlockDeltaUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type BetaRawContentBlockDeltaUnion struct {
	// This field is from variant [BetaTextDelta].
	Text string `json:"text"`
	// Any of "text_delta", "input_json_delta", "citations_delta", "thinking_delta",
	// "signature_delta".
	Type string `json:"type"`
	// This field is from variant [BetaInputJSONDelta].
	PartialJSON string `json:"partial_json"`
	// This field is from variant [BetaCitationsDelta].
	Citation BetaCitationsDeltaCitationUnion `json:"citation"`
	// This field is from variant [BetaThinkingDelta].
	Thinking string `json:"thinking"`
	// This field is from variant [BetaSignatureDelta].
	Signature string `json:"signature"`
	JSON      struct {
		Text        respjson.Field
		Type        respjson.Field
		PartialJSON respjson.Field
		Citation    respjson.Field
		Thinking    respjson.Field
		Signature   respjson.Field
		raw         string
	} `json:"-"`
}

// anyBetaRawContentBlockDelta is implemented by each variant of
// [BetaRawContentBlockDeltaUnion] to add type safety for the return type of
// [BetaRawContentBlockDeltaUnion.AsAny]
type anyBetaRawContentBlockDelta interface {
	implBetaRawContentBlockDeltaUnion()
}

func (BetaTextDelta) implBetaRawContentBlockDeltaUnion()      {}
func (BetaInputJSONDelta) implBetaRawContentBlockDeltaUnion() {}
func (BetaCitationsDelta) implBetaRawContentBlockDeltaUnion() {}
func (BetaThinkingDelta) implBetaRawContentBlockDeltaUnion()  {}
func (BetaSignatureDelta) implBetaRawContentBlockDeltaUnion() {}

// Use the following switch statement to find the correct variant
//
//	switch variant := BetaRawContentBlockDeltaUnion.AsAny().(type) {
//	case anthropic.BetaTextDelta:
//	case anthropic.BetaInputJSONDelta:
//	case anthropic.BetaCitationsDelta:
//	case anthropic.BetaThinkingDelta:
//	case anthropic.BetaSignatureDelta:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u BetaRawContentBlockDeltaUnion) AsAny() anyBetaRawContentBlockDelta {
	switch u.Type {
	case "text_delta":
		return u.AsTextDelta()
	case "input_json_delta":
		return u.AsInputJSONDelta()
	case "citations_delta":
		return u.AsCitationsDelta()
	case "thinking_delta":
		return u.AsThinkingDelta()
	case "signature_delta":
		return u.AsSignatureDelta()
	}
	return nil
}

func (u BetaRawContentBlockDeltaUnion) AsTextDelta() (v BetaTextDelta) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaRawContentBlockDeltaUnion) AsInputJSONDelta() (v BetaInputJSONDelta) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaRawContentBlockDeltaUnion) AsCitationsDelta() (v BetaCitationsDelta) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaRawContentBlockDeltaUnion) AsThinkingDelta() (v BetaThinkingDelta) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaRawContentBlockDeltaUnion) AsSignatureDelta() (v BetaSignatureDelta) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u BetaRawContentBlockDeltaUnion) RawJSON() string { return u.JSON.raw }

func (r *BetaRawContentBlockDeltaUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaRawContentBlockDeltaEvent struct {
	Delta BetaRawContentBlockDeltaUnion `json:"delta,required"`
	Index int64                         `json:"index,required"`
	Type  constant.ContentBlockDelta    `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Delta       respjson.Field
		Index       respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaRawContentBlockDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *BetaRawContentBlockDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaRawContentBlockStartEvent struct {
	// Response model for a file uploaded to the container.
	ContentBlock BetaRawContentBlockStartEventContentBlockUnion `json:"content_block,required"`
	Index        int64                                          `json:"index,required"`
	Type         constant.ContentBlockStart                     `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ContentBlock respjson.Field
		Index        respjson.Field
		Type         respjson.Field
		ExtraFields  map[string]respjson.Field
		raw          string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaRawContentBlockStartEvent) RawJSON() string { return r.JSON.raw }
func (r *BetaRawContentBlockStartEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// BetaRawContentBlockStartEventContentBlockUnion contains all possible properties
// and values from [BetaTextBlock], [BetaToolUseBlock], [BetaServerToolUseBlock],
// [BetaWebSearchToolResultBlock], [BetaCodeExecutionToolResultBlock],
// [BetaMCPToolUseBlock], [BetaMCPToolResultBlock], [BetaContainerUploadBlock],
// [BetaThinkingBlock], [BetaRedactedThinkingBlock].
//
// Use the [BetaRawContentBlockStartEventContentBlockUnion.AsAny] method to switch
// on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type BetaRawContentBlockStartEventContentBlockUnion struct {
	// This field is from variant [BetaTextBlock].
	Citations []BetaTextCitationUnion `json:"citations"`
	// This field is from variant [BetaTextBlock].
	Text string `json:"text"`
	// Any of "text", "tool_use", "server_tool_use", "web_search_tool_result",
	// "code_execution_tool_result", "mcp_tool_use", "mcp_tool_result",
	// "container_upload", "thinking", "redacted_thinking".
	Type  string `json:"type"`
	ID    string `json:"id"`
	Input any    `json:"input"`
	Name  string `json:"name"`
	// This field is a union of [BetaWebSearchToolResultBlockContentUnion],
	// [BetaCodeExecutionToolResultBlockContentUnion],
	// [BetaMCPToolResultBlockContentUnion]
	Content   BetaRawContentBlockStartEventContentBlockUnionContent `json:"content"`
	ToolUseID string                                                `json:"tool_use_id"`
	// This field is from variant [BetaMCPToolUseBlock].
	ServerName string `json:"server_name"`
	// This field is from variant [BetaMCPToolResultBlock].
	IsError bool `json:"is_error"`
	// This field is from variant [BetaContainerUploadBlock].
	FileID string `json:"file_id"`
	// This field is from variant [BetaThinkingBlock].
	Signature string `json:"signature"`
	// This field is from variant [BetaThinkingBlock].
	Thinking string `json:"thinking"`
	// This field is from variant [BetaRedactedThinkingBlock].
	Data string `json:"data"`
	JSON struct {
		Citations  respjson.Field
		Text       respjson.Field
		Type       respjson.Field
		ID         respjson.Field
		Input      respjson.Field
		Name       respjson.Field
		Content    respjson.Field
		ToolUseID  respjson.Field
		ServerName respjson.Field
		IsError    respjson.Field
		FileID     respjson.Field
		Signature  respjson.Field
		Thinking   respjson.Field
		Data       respjson.Field
		raw        string
	} `json:"-"`
}

// anyBetaRawContentBlockStartEventContentBlock is implemented by each variant of
// [BetaRawContentBlockStartEventContentBlockUnion] to add type safety for the
// return type of [BetaRawContentBlockStartEventContentBlockUnion.AsAny]
type anyBetaRawContentBlockStartEventContentBlock interface {
	implBetaRawContentBlockStartEventContentBlockUnion()
}

func (BetaTextBlock) implBetaRawContentBlockStartEventContentBlockUnion()                    {}
func (BetaToolUseBlock) implBetaRawContentBlockStartEventContentBlockUnion()                 {}
func (BetaServerToolUseBlock) implBetaRawContentBlockStartEventContentBlockUnion()           {}
func (BetaWebSearchToolResultBlock) implBetaRawContentBlockStartEventContentBlockUnion()     {}
func (BetaCodeExecutionToolResultBlock) implBetaRawContentBlockStartEventContentBlockUnion() {}
func (BetaMCPToolUseBlock) implBetaRawContentBlockStartEventContentBlockUnion()              {}
func (BetaMCPToolResultBlock) implBetaRawContentBlockStartEventContentBlockUnion()           {}
func (BetaContainerUploadBlock) implBetaRawContentBlockStartEventContentBlockUnion()         {}
func (BetaThinkingBlock) implBetaRawContentBlockStartEventContentBlockUnion()                {}
func (BetaRedactedThinkingBlock) implBetaRawContentBlockStartEventContentBlockUnion()        {}

// Use the following switch statement to find the correct variant
//
//	switch variant := BetaRawContentBlockStartEventContentBlockUnion.AsAny().(type) {
//	case anthropic.BetaTextBlock:
//	case anthropic.BetaToolUseBlock:
//	case anthropic.BetaServerToolUseBlock:
//	case anthropic.BetaWebSearchToolResultBlock:
//	case anthropic.BetaCodeExecutionToolResultBlock:
//	case anthropic.BetaMCPToolUseBlock:
//	case anthropic.BetaMCPToolResultBlock:
//	case anthropic.BetaContainerUploadBlock:
//	case anthropic.BetaThinkingBlock:
//	case anthropic.BetaRedactedThinkingBlock:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u BetaRawContentBlockStartEventContentBlockUnion) AsAny() anyBetaRawContentBlockStartEventContentBlock {
	switch u.Type {
	case "text":
		return u.AsText()
	case "tool_use":
		return u.AsToolUse()
	case "server_tool_use":
		return u.AsServerToolUse()
	case "web_search_tool_result":
		return u.AsWebSearchToolResult()
	case "code_execution_tool_result":
		return u.AsCodeExecutionToolResult()
	case "mcp_tool_use":
		return u.AsMCPToolUse()
	case "mcp_tool_result":
		return u.AsMCPToolResult()
	case "container_upload":
		return u.AsContainerUpload()
	case "thinking":
		return u.AsThinking()
	case "redacted_thinking":
		return u.AsRedactedThinking()
	}
	return nil
}

func (u BetaRawContentBlockStartEventContentBlockUnion) AsText() (v BetaTextBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaRawContentBlockStartEventContentBlockUnion) AsToolUse() (v BetaToolUseBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaRawContentBlockStartEventContentBlockUnion) AsServerToolUse() (v BetaServerToolUseBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaRawContentBlockStartEventContentBlockUnion) AsWebSearchToolResult() (v BetaWebSearchToolResultBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaRawContentBlockStartEventContentBlockUnion) AsCodeExecutionToolResult() (v BetaCodeExecutionToolResultBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaRawContentBlockStartEventContentBlockUnion) AsMCPToolUse() (v BetaMCPToolUseBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaRawContentBlockStartEventContentBlockUnion) AsMCPToolResult() (v BetaMCPToolResultBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaRawContentBlockStartEventContentBlockUnion) AsContainerUpload() (v BetaContainerUploadBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaRawContentBlockStartEventContentBlockUnion) AsThinking() (v BetaThinkingBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaRawContentBlockStartEventContentBlockUnion) AsRedactedThinking() (v BetaRedactedThinkingBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u BetaRawContentBlockStartEventContentBlockUnion) RawJSON() string { return u.JSON.raw }

func (r *BetaRawContentBlockStartEventContentBlockUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// BetaRawContentBlockStartEventContentBlockUnionContent is an implicit subunion of
// [BetaRawContentBlockStartEventContentBlockUnion].
// BetaRawContentBlockStartEventContentBlockUnionContent provides convenient access
// to the sub-properties of the union.
//
// For type safety it is recommended to directly use a variant of the
// [BetaRawContentBlockStartEventContentBlockUnion].
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfBetaWebSearchResultBlockArray OfString
// OfBetaMCPToolResultBlockContent]
type BetaRawContentBlockStartEventContentBlockUnionContent struct {
	// This field will be present if the value is a [[]BetaWebSearchResultBlock]
	// instead of an object.
	OfBetaWebSearchResultBlockArray []BetaWebSearchResultBlock `json:",inline"`
	// This field will be present if the value is a [string] instead of an object.
	OfString string `json:",inline"`
	// This field will be present if the value is a [[]BetaTextBlock] instead of an
	// object.
	OfBetaMCPToolResultBlockContent []BetaTextBlock `json:",inline"`
	ErrorCode                       string          `json:"error_code"`
	Type                            string          `json:"type"`
	// This field is from variant [BetaCodeExecutionToolResultBlockContentUnion].
	Content []BetaCodeExecutionOutputBlock `json:"content"`
	// This field is from variant [BetaCodeExecutionToolResultBlockContentUnion].
	ReturnCode int64 `json:"return_code"`
	// This field is from variant [BetaCodeExecutionToolResultBlockContentUnion].
	Stderr string `json:"stderr"`
	// This field is from variant [BetaCodeExecutionToolResultBlockContentUnion].
	Stdout string `json:"stdout"`
	JSON   struct {
		OfBetaWebSearchResultBlockArray respjson.Field
		OfString                        respjson.Field
		OfBetaMCPToolResultBlockContent respjson.Field
		ErrorCode                       respjson.Field
		Type                            respjson.Field
		Content                         respjson.Field
		ReturnCode                      respjson.Field
		Stderr                          respjson.Field
		Stdout                          respjson.Field
		raw                             string
	} `json:"-"`
}

func (r *BetaRawContentBlockStartEventContentBlockUnionContent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaRawContentBlockStopEvent struct {
	Index int64                     `json:"index,required"`
	Type  constant.ContentBlockStop `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Index       respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaRawContentBlockStopEvent) RawJSON() string { return r.JSON.raw }
func (r *BetaRawContentBlockStopEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaRawMessageDeltaEvent struct {
	Delta BetaRawMessageDeltaEventDelta `json:"delta,required"`
	Type  constant.MessageDelta         `json:"type,required"`
	// Billing and rate-limit usage.
	//
	// Anthropic's API bills and rate-limits by token counts, as tokens represent the
	// underlying cost to our systems.
	//
	// Under the hood, the API transforms requests into a format suitable for the
	// model. The model's output then goes through a parsing stage before becoming an
	// API response. As a result, the token counts in `usage` will not match one-to-one
	// with the exact visible content of an API request or response.
	//
	// For example, `output_tokens` will be non-zero, even for an empty string response
	// from Claude.
	//
	// Total input tokens in a request is the summation of `input_tokens`,
	// `cache_creation_input_tokens`, and `cache_read_input_tokens`.
	Usage BetaMessageDeltaUsage `json:"usage,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Delta       respjson.Field
		Type        respjson.Field
		Usage       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaRawMessageDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *BetaRawMessageDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaRawMessageDeltaEventDelta struct {
	// Information about the container used in the request (for the code execution
	// tool)
	Container BetaContainer `json:"container,required"`
	// Any of "end_turn", "max_tokens", "stop_sequence", "tool_use", "pause_turn",
	// "refusal".
	StopReason   BetaStopReason `json:"stop_reason,required"`
	StopSequence string         `json:"stop_sequence,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Container    respjson.Field
		StopReason   respjson.Field
		StopSequence respjson.Field
		ExtraFields  map[string]respjson.Field
		raw          string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaRawMessageDeltaEventDelta) RawJSON() string { return r.JSON.raw }
func (r *BetaRawMessageDeltaEventDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaRawMessageStartEvent struct {
	Message BetaMessage           `json:"message,required"`
	Type    constant.MessageStart `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Message     respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaRawMessageStartEvent) RawJSON() string { return r.JSON.raw }
func (r *BetaRawMessageStartEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaRawMessageStopEvent struct {
	Type constant.MessageStop `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaRawMessageStopEvent) RawJSON() string { return r.JSON.raw }
func (r *BetaRawMessageStopEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// BetaRawMessageStreamEventUnion contains all possible properties and values from
// [BetaRawMessageStartEvent], [BetaRawMessageDeltaEvent],
// [BetaRawMessageStopEvent], [BetaRawContentBlockStartEvent],
// [BetaRawContentBlockDeltaEvent], [BetaRawContentBlockStopEvent].
//
// Use the [BetaRawMessageStreamEventUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type BetaRawMessageStreamEventUnion struct {
	// This field is from variant [BetaRawMessageStartEvent].
	Message BetaMessage `json:"message"`
	// Any of "message_start", "message_delta", "message_stop", "content_block_start",
	// "content_block_delta", "content_block_stop".
	Type string `json:"type"`
	// This field is a union of [BetaRawMessageDeltaEventDelta],
	// [BetaRawContentBlockDeltaUnion]
	Delta BetaRawMessageStreamEventUnionDelta `json:"delta"`
	// This field is from variant [BetaRawMessageDeltaEvent].
	Usage BetaMessageDeltaUsage `json:"usage"`
	// This field is from variant [BetaRawContentBlockStartEvent].
	ContentBlock BetaRawContentBlockStartEventContentBlockUnion `json:"content_block"`
	Index        int64                                          `json:"index"`
	JSON         struct {
		Message      respjson.Field
		Type         respjson.Field
		Delta        respjson.Field
		Usage        respjson.Field
		ContentBlock respjson.Field
		Index        respjson.Field
		raw          string
	} `json:"-"`
}

// anyBetaRawMessageStreamEvent is implemented by each variant of
// [BetaRawMessageStreamEventUnion] to add type safety for the return type of
// [BetaRawMessageStreamEventUnion.AsAny]
type anyBetaRawMessageStreamEvent interface {
	implBetaRawMessageStreamEventUnion()
}

func (BetaRawMessageStartEvent) implBetaRawMessageStreamEventUnion()      {}
func (BetaRawMessageDeltaEvent) implBetaRawMessageStreamEventUnion()      {}
func (BetaRawMessageStopEvent) implBetaRawMessageStreamEventUnion()       {}
func (BetaRawContentBlockStartEvent) implBetaRawMessageStreamEventUnion() {}
func (BetaRawContentBlockDeltaEvent) implBetaRawMessageStreamEventUnion() {}
func (BetaRawContentBlockStopEvent) implBetaRawMessageStreamEventUnion()  {}

// Use the following switch statement to find the correct variant
//
//	switch variant := BetaRawMessageStreamEventUnion.AsAny().(type) {
//	case anthropic.BetaRawMessageStartEvent:
//	case anthropic.BetaRawMessageDeltaEvent:
//	case anthropic.BetaRawMessageStopEvent:
//	case anthropic.BetaRawContentBlockStartEvent:
//	case anthropic.BetaRawContentBlockDeltaEvent:
//	case anthropic.BetaRawContentBlockStopEvent:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u BetaRawMessageStreamEventUnion) AsAny() anyBetaRawMessageStreamEvent {
	switch u.Type {
	case "message_start":
		return u.AsMessageStart()
	case "message_delta":
		return u.AsMessageDelta()
	case "message_stop":
		return u.AsMessageStop()
	case "content_block_start":
		return u.AsContentBlockStart()
	case "content_block_delta":
		return u.AsContentBlockDelta()
	case "content_block_stop":
		return u.AsContentBlockStop()
	}
	return nil
}

func (u BetaRawMessageStreamEventUnion) AsMessageStart() (v BetaRawMessageStartEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaRawMessageStreamEventUnion) AsMessageDelta() (v BetaRawMessageDeltaEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaRawMessageStreamEventUnion) AsMessageStop() (v BetaRawMessageStopEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaRawMessageStreamEventUnion) AsContentBlockStart() (v BetaRawContentBlockStartEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaRawMessageStreamEventUnion) AsContentBlockDelta() (v BetaRawContentBlockDeltaEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaRawMessageStreamEventUnion) AsContentBlockStop() (v BetaRawContentBlockStopEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u BetaRawMessageStreamEventUnion) RawJSON() string { return u.JSON.raw }

func (r *BetaRawMessageStreamEventUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// BetaRawMessageStreamEventUnionDelta is an implicit subunion of
// [BetaRawMessageStreamEventUnion]. BetaRawMessageStreamEventUnionDelta provides
// convenient access to the sub-properties of the union.
//
// For type safety it is recommended to directly use a variant of the
// [BetaRawMessageStreamEventUnion].
type BetaRawMessageStreamEventUnionDelta struct {
	// This field is from variant [BetaRawMessageDeltaEventDelta].
	Container BetaContainer `json:"container"`
	// This field is from variant [BetaRawMessageDeltaEventDelta].
	StopReason BetaStopReason `json:"stop_reason"`
	// This field is from variant [BetaRawMessageDeltaEventDelta].
	StopSequence string `json:"stop_sequence"`
	// This field is from variant [BetaRawContentBlockDeltaUnion].
	Text string `json:"text"`
	Type string `json:"type"`
	// This field is from variant [BetaRawContentBlockDeltaUnion].
	PartialJSON string `json:"partial_json"`
	// This field is from variant [BetaRawContentBlockDeltaUnion].
	Citation BetaCitationsDeltaCitationUnion `json:"citation"`
	// This field is from variant [BetaRawContentBlockDeltaUnion].
	Thinking string `json:"thinking"`
	// This field is from variant [BetaRawContentBlockDeltaUnion].
	Signature string `json:"signature"`
	JSON      struct {
		Container    respjson.Field
		StopReason   respjson.Field
		StopSequence respjson.Field
		Text         respjson.Field
		Type         respjson.Field
		PartialJSON  respjson.Field
		Citation     respjson.Field
		Thinking     respjson.Field
		Signature    respjson.Field
		raw          string
	} `json:"-"`
}

func (r *BetaRawMessageStreamEventUnionDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Accumulate builds up the Message incrementally from a MessageStreamEvent. The Message then can be used as
// any other Message, except with the caveat that the Message.JSON field which normally can be used to inspect
// the JSON sent over the network may not be populated fully.
//
//	message := anthropic.Message{}
//	for stream.Next() {
//		event := stream.Current()
//		message.Accumulate(event)
//	}
func (acc *BetaMessage) Accumulate(event BetaRawMessageStreamEventUnion) error {
	if acc == nil {
		return fmt.Errorf("accumulate: cannot accumlate into nil Message")
	}

	switch event := event.AsAny().(type) {
	case BetaRawMessageStartEvent:
		*acc = event.Message
	case BetaRawMessageDeltaEvent:
		acc.StopReason = event.Delta.StopReason
		acc.StopSequence = event.Delta.StopSequence
		acc.Usage.OutputTokens = event.Usage.OutputTokens
	case BetaRawMessageStopEvent:
		accJson, err := json.Marshal(acc)
		if err != nil {
			return fmt.Errorf("error converting content block to JSON: %w", err)
		}
		acc.JSON.raw = string(accJson)
	case BetaRawContentBlockStartEvent:
		acc.Content = append(acc.Content, BetaContentBlockUnion{})
		err := acc.Content[len(acc.Content)-1].UnmarshalJSON([]byte(event.ContentBlock.RawJSON()))
		if err != nil {
			return err
		}
	case BetaRawContentBlockDeltaEvent:
		if len(acc.Content) == 0 {
			return fmt.Errorf("received event of type %s but there was no content block", event.Type)
		}
		cb := &acc.Content[len(acc.Content)-1]
		switch delta := event.Delta.AsAny().(type) {
		case BetaTextDelta:
			cb.Text += delta.Text
		case BetaInputJSONDelta:
			if len(delta.PartialJSON) != 0 {
				if string(cb.Input) == "{}" {
					cb.Input = []byte(delta.PartialJSON)
				} else {
					cb.Input = append(cb.Input, []byte(delta.PartialJSON)...)
				}
			}
		case BetaThinkingDelta:
			cb.Thinking += delta.Thinking
		case BetaSignatureDelta:
			cb.Signature += delta.Signature
		case BetaCitationsDelta:
			citation := BetaTextCitationUnion{}
			err := citation.UnmarshalJSON([]byte(delta.Citation.RawJSON()))
			if err != nil {
				return fmt.Errorf("could not unmarshal citation delta into citation type: %w", err)
			}
			cb.Citations = append(cb.Citations, citation)
		}
	case BetaRawContentBlockStopEvent:
		if len(acc.Content) == 0 {
			return fmt.Errorf("received event of type %s but there was no content block", event.Type)
		}
		contentBlock := &acc.Content[len(acc.Content)-1]
		cbJson, err := json.Marshal(contentBlock)
		if err != nil {
			return fmt.Errorf("error converting content block to JSON: %w", err)
		}
		contentBlock.JSON.raw = string(cbJson)
	}

	return nil
}

type BetaRedactedThinkingBlock struct {
	Data string                    `json:"data,required"`
	Type constant.RedactedThinking `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaRedactedThinkingBlock) RawJSON() string { return r.JSON.raw }
func (r *BetaRedactedThinkingBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func (r BetaRedactedThinkingBlock) ToParam() BetaRedactedThinkingBlockParam {
	var p BetaRedactedThinkingBlockParam
	p.Type = r.Type
	p.Data = r.Data
	return p
}

// The properties Data, Type are required.
type BetaRedactedThinkingBlockParam struct {
	Data string `json:"data,required"`
	// This field can be elided, and will marshal its zero value as
	// "redacted_thinking".
	Type constant.RedactedThinking `json:"type,required"`
	paramObj
}

func (r BetaRedactedThinkingBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaRedactedThinkingBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaRedactedThinkingBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaRequestMCPServerToolConfigurationParam struct {
	Enabled      param.Opt[bool] `json:"enabled,omitzero"`
	AllowedTools []string        `json:"allowed_tools,omitzero"`
	paramObj
}

func (r BetaRequestMCPServerToolConfigurationParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaRequestMCPServerToolConfigurationParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaRequestMCPServerToolConfigurationParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Name, Type, URL are required.
type BetaRequestMCPServerURLDefinitionParam struct {
	Name               string                                     `json:"name,required"`
	URL                string                                     `json:"url,required"`
	AuthorizationToken param.Opt[string]                          `json:"authorization_token,omitzero"`
	ToolConfiguration  BetaRequestMCPServerToolConfigurationParam `json:"tool_configuration,omitzero"`
	// This field can be elided, and will marshal its zero value as "url".
	Type constant.URL `json:"type,required"`
	paramObj
}

func (r BetaRequestMCPServerURLDefinitionParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaRequestMCPServerURLDefinitionParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaRequestMCPServerURLDefinitionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties ToolUseID, Type are required.
type BetaRequestMCPToolResultBlockParam struct {
	ToolUseID string          `json:"tool_use_id,required"`
	IsError   param.Opt[bool] `json:"is_error,omitzero"`
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam                 `json:"cache_control,omitzero"`
	Content      BetaRequestMCPToolResultBlockParamContentUnion `json:"content,omitzero"`
	// This field can be elided, and will marshal its zero value as "mcp_tool_result".
	Type constant.MCPToolResult `json:"type,required"`
	paramObj
}

func (r BetaRequestMCPToolResultBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaRequestMCPToolResultBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaRequestMCPToolResultBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaRequestMCPToolResultBlockParamContentUnion struct {
	OfString                        param.Opt[string]    `json:",omitzero,inline"`
	OfBetaMCPToolResultBlockContent []BetaTextBlockParam `json:",omitzero,inline"`
	paramUnion
}

func (u BetaRequestMCPToolResultBlockParamContentUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfBetaMCPToolResultBlockContent)
}
func (u *BetaRequestMCPToolResultBlockParamContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaRequestMCPToolResultBlockParamContentUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfBetaMCPToolResultBlockContent) {
		return &u.OfBetaMCPToolResultBlockContent
	}
	return nil
}

type BetaServerToolUsage struct {
	// The number of web search tool requests.
	WebSearchRequests int64 `json:"web_search_requests,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		WebSearchRequests respjson.Field
		ExtraFields       map[string]respjson.Field
		raw               string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaServerToolUsage) RawJSON() string { return r.JSON.raw }
func (r *BetaServerToolUsage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaServerToolUseBlock struct {
	ID    string `json:"id,required"`
	Input any    `json:"input,required"`
	// Any of "web_search", "code_execution".
	Name BetaServerToolUseBlockName `json:"name,required"`
	Type constant.ServerToolUse     `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Input       respjson.Field
		Name        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaServerToolUseBlock) RawJSON() string { return r.JSON.raw }
func (r *BetaServerToolUseBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaServerToolUseBlockName string

const (
	BetaServerToolUseBlockNameWebSearch     BetaServerToolUseBlockName = "web_search"
	BetaServerToolUseBlockNameCodeExecution BetaServerToolUseBlockName = "code_execution"
)

// The properties ID, Input, Name, Type are required.
type BetaServerToolUseBlockParam struct {
	ID    string `json:"id,required"`
	Input any    `json:"input,omitzero,required"`
	// Any of "web_search", "code_execution".
	Name BetaServerToolUseBlockParamName `json:"name,omitzero,required"`
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam `json:"cache_control,omitzero"`
	// This field can be elided, and will marshal its zero value as "server_tool_use".
	Type constant.ServerToolUse `json:"type,required"`
	paramObj
}

func (r BetaServerToolUseBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaServerToolUseBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaServerToolUseBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaServerToolUseBlockParamName string

const (
	BetaServerToolUseBlockParamNameWebSearch     BetaServerToolUseBlockParamName = "web_search"
	BetaServerToolUseBlockParamNameCodeExecution BetaServerToolUseBlockParamName = "code_execution"
)

type BetaSignatureDelta struct {
	Signature string                  `json:"signature,required"`
	Type      constant.SignatureDelta `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Signature   respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaSignatureDelta) RawJSON() string { return r.JSON.raw }
func (r *BetaSignatureDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaStopReason string

const (
	BetaStopReasonEndTurn      BetaStopReason = "end_turn"
	BetaStopReasonMaxTokens    BetaStopReason = "max_tokens"
	BetaStopReasonStopSequence BetaStopReason = "stop_sequence"
	BetaStopReasonToolUse      BetaStopReason = "tool_use"
	BetaStopReasonPauseTurn    BetaStopReason = "pause_turn"
	BetaStopReasonRefusal      BetaStopReason = "refusal"
)

type BetaTextBlock struct {
	// Citations supporting the text block.
	//
	// The type of citation returned will depend on the type of document being cited.
	// Citing a PDF results in `page_location`, plain text results in `char_location`,
	// and content document results in `content_block_location`.
	Citations []BetaTextCitationUnion `json:"citations,required"`
	Text      string                  `json:"text,required"`
	Type      constant.Text           `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Citations   respjson.Field
		Text        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaTextBlock) RawJSON() string { return r.JSON.raw }
func (r *BetaTextBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func (r BetaTextBlock) ToParam() BetaTextBlockParam {
	var p BetaTextBlockParam
	p.Type = r.Type
	p.Text = r.Text

	// Distinguish between a nil and zero length slice, since some compatible
	// APIs may not require citations.
	if r.Citations != nil {
		p.Citations = make([]BetaTextCitationParamUnion, len(r.Citations))
	}

	for i, citation := range r.Citations {
		switch citationVariant := citation.AsAny().(type) {
		case BetaCitationCharLocation:
			var citationParam BetaCitationCharLocationParam
			citationParam.Type = citationVariant.Type
			citationParam.DocumentTitle = paramutil.ToOpt(citationVariant.DocumentTitle, citationVariant.JSON.DocumentTitle)
			citationParam.CitedText = citationVariant.CitedText
			citationParam.DocumentIndex = citationVariant.DocumentIndex
			citationParam.EndCharIndex = citationVariant.EndCharIndex
			citationParam.StartCharIndex = citationVariant.StartCharIndex
			p.Citations[i] = BetaTextCitationParamUnion{OfCharLocation: &citationParam}
		case BetaCitationPageLocation:
			var citationParam BetaCitationPageLocationParam
			citationParam.Type = citationVariant.Type
			citationParam.DocumentTitle = paramutil.ToOpt(citationVariant.DocumentTitle, citationVariant.JSON.DocumentTitle)
			citationParam.DocumentIndex = citationVariant.DocumentIndex
			citationParam.EndPageNumber = citationVariant.EndPageNumber
			citationParam.StartPageNumber = citationVariant.StartPageNumber
			p.Citations[i] = BetaTextCitationParamUnion{OfPageLocation: &citationParam}
		case BetaCitationContentBlockLocation:
			var citationParam BetaCitationContentBlockLocationParam
			citationParam.Type = citationVariant.Type
			citationParam.DocumentTitle = paramutil.ToOpt(citationVariant.DocumentTitle, citationVariant.JSON.DocumentTitle)
			citationParam.CitedText = citationVariant.CitedText
			citationParam.DocumentIndex = citationVariant.DocumentIndex
			citationParam.EndBlockIndex = citationVariant.EndBlockIndex
			citationParam.StartBlockIndex = citationVariant.StartBlockIndex
			p.Citations[i] = BetaTextCitationParamUnion{OfContentBlockLocation: &citationParam}
		}
	}
	return p
}

// The properties Text, Type are required.
type BetaTextBlockParam struct {
	Text      string                       `json:"text,required"`
	Citations []BetaTextCitationParamUnion `json:"citations,omitzero"`
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam `json:"cache_control,omitzero"`
	// This field can be elided, and will marshal its zero value as "text".
	Type constant.Text `json:"type,required"`
	paramObj
}

func (r BetaTextBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaTextBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaTextBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// BetaTextCitationUnion contains all possible properties and values from
// [BetaCitationCharLocation], [BetaCitationPageLocation],
// [BetaCitationContentBlockLocation], [BetaCitationsWebSearchResultLocation].
//
// Use the [BetaTextCitationUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type BetaTextCitationUnion struct {
	CitedText     string `json:"cited_text"`
	DocumentIndex int64  `json:"document_index"`
	DocumentTitle string `json:"document_title"`
	// This field is from variant [BetaCitationCharLocation].
	EndCharIndex int64 `json:"end_char_index"`
	// This field is from variant [BetaCitationCharLocation].
	StartCharIndex int64 `json:"start_char_index"`
	// Any of "char_location", "page_location", "content_block_location",
	// "web_search_result_location".
	Type string `json:"type"`
	// This field is from variant [BetaCitationPageLocation].
	EndPageNumber int64 `json:"end_page_number"`
	// This field is from variant [BetaCitationPageLocation].
	StartPageNumber int64 `json:"start_page_number"`
	// This field is from variant [BetaCitationContentBlockLocation].
	EndBlockIndex int64 `json:"end_block_index"`
	// This field is from variant [BetaCitationContentBlockLocation].
	StartBlockIndex int64 `json:"start_block_index"`
	// This field is from variant [BetaCitationsWebSearchResultLocation].
	EncryptedIndex string `json:"encrypted_index"`
	// This field is from variant [BetaCitationsWebSearchResultLocation].
	Title string `json:"title"`
	// This field is from variant [BetaCitationsWebSearchResultLocation].
	URL  string `json:"url"`
	JSON struct {
		CitedText       respjson.Field
		DocumentIndex   respjson.Field
		DocumentTitle   respjson.Field
		EndCharIndex    respjson.Field
		StartCharIndex  respjson.Field
		Type            respjson.Field
		EndPageNumber   respjson.Field
		StartPageNumber respjson.Field
		EndBlockIndex   respjson.Field
		StartBlockIndex respjson.Field
		EncryptedIndex  respjson.Field
		Title           respjson.Field
		URL             respjson.Field
		raw             string
	} `json:"-"`
}

// anyBetaTextCitation is implemented by each variant of [BetaTextCitationUnion] to
// add type safety for the return type of [BetaTextCitationUnion.AsAny]
type anyBetaTextCitation interface {
	implBetaTextCitationUnion()
}

func (BetaCitationCharLocation) implBetaTextCitationUnion()             {}
func (BetaCitationPageLocation) implBetaTextCitationUnion()             {}
func (BetaCitationContentBlockLocation) implBetaTextCitationUnion()     {}
func (BetaCitationsWebSearchResultLocation) implBetaTextCitationUnion() {}

// Use the following switch statement to find the correct variant
//
//	switch variant := BetaTextCitationUnion.AsAny().(type) {
//	case anthropic.BetaCitationCharLocation:
//	case anthropic.BetaCitationPageLocation:
//	case anthropic.BetaCitationContentBlockLocation:
//	case anthropic.BetaCitationsWebSearchResultLocation:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u BetaTextCitationUnion) AsAny() anyBetaTextCitation {
	switch u.Type {
	case "char_location":
		return u.AsCharLocation()
	case "page_location":
		return u.AsPageLocation()
	case "content_block_location":
		return u.AsContentBlockLocation()
	case "web_search_result_location":
		return u.AsWebSearchResultLocation()
	}
	return nil
}

func (u BetaTextCitationUnion) AsCharLocation() (v BetaCitationCharLocation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaTextCitationUnion) AsPageLocation() (v BetaCitationPageLocation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaTextCitationUnion) AsContentBlockLocation() (v BetaCitationContentBlockLocation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaTextCitationUnion) AsWebSearchResultLocation() (v BetaCitationsWebSearchResultLocation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u BetaTextCitationUnion) RawJSON() string { return u.JSON.raw }

func (r *BetaTextCitationUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaTextCitationParamUnion struct {
	OfCharLocation            *BetaCitationCharLocationParam            `json:",omitzero,inline"`
	OfPageLocation            *BetaCitationPageLocationParam            `json:",omitzero,inline"`
	OfContentBlockLocation    *BetaCitationContentBlockLocationParam    `json:",omitzero,inline"`
	OfWebSearchResultLocation *BetaCitationWebSearchResultLocationParam `json:",omitzero,inline"`
	paramUnion
}

func (u BetaTextCitationParamUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfCharLocation, u.OfPageLocation, u.OfContentBlockLocation, u.OfWebSearchResultLocation)
}
func (u *BetaTextCitationParamUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaTextCitationParamUnion) asAny() any {
	if !param.IsOmitted(u.OfCharLocation) {
		return u.OfCharLocation
	} else if !param.IsOmitted(u.OfPageLocation) {
		return u.OfPageLocation
	} else if !param.IsOmitted(u.OfContentBlockLocation) {
		return u.OfContentBlockLocation
	} else if !param.IsOmitted(u.OfWebSearchResultLocation) {
		return u.OfWebSearchResultLocation
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaTextCitationParamUnion) GetEndCharIndex() *int64 {
	if vt := u.OfCharLocation; vt != nil {
		return &vt.EndCharIndex
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaTextCitationParamUnion) GetStartCharIndex() *int64 {
	if vt := u.OfCharLocation; vt != nil {
		return &vt.StartCharIndex
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaTextCitationParamUnion) GetEndPageNumber() *int64 {
	if vt := u.OfPageLocation; vt != nil {
		return &vt.EndPageNumber
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaTextCitationParamUnion) GetStartPageNumber() *int64 {
	if vt := u.OfPageLocation; vt != nil {
		return &vt.StartPageNumber
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaTextCitationParamUnion) GetEndBlockIndex() *int64 {
	if vt := u.OfContentBlockLocation; vt != nil {
		return &vt.EndBlockIndex
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaTextCitationParamUnion) GetStartBlockIndex() *int64 {
	if vt := u.OfContentBlockLocation; vt != nil {
		return &vt.StartBlockIndex
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaTextCitationParamUnion) GetEncryptedIndex() *string {
	if vt := u.OfWebSearchResultLocation; vt != nil {
		return &vt.EncryptedIndex
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaTextCitationParamUnion) GetTitle() *string {
	if vt := u.OfWebSearchResultLocation; vt != nil && vt.Title.Valid() {
		return &vt.Title.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaTextCitationParamUnion) GetURL() *string {
	if vt := u.OfWebSearchResultLocation; vt != nil {
		return &vt.URL
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaTextCitationParamUnion) GetCitedText() *string {
	if vt := u.OfCharLocation; vt != nil {
		return (*string)(&vt.CitedText)
	} else if vt := u.OfPageLocation; vt != nil {
		return (*string)(&vt.CitedText)
	} else if vt := u.OfContentBlockLocation; vt != nil {
		return (*string)(&vt.CitedText)
	} else if vt := u.OfWebSearchResultLocation; vt != nil {
		return (*string)(&vt.CitedText)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaTextCitationParamUnion) GetDocumentIndex() *int64 {
	if vt := u.OfCharLocation; vt != nil {
		return (*int64)(&vt.DocumentIndex)
	} else if vt := u.OfPageLocation; vt != nil {
		return (*int64)(&vt.DocumentIndex)
	} else if vt := u.OfContentBlockLocation; vt != nil {
		return (*int64)(&vt.DocumentIndex)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaTextCitationParamUnion) GetDocumentTitle() *string {
	if vt := u.OfCharLocation; vt != nil && vt.DocumentTitle.Valid() {
		return &vt.DocumentTitle.Value
	} else if vt := u.OfPageLocation; vt != nil && vt.DocumentTitle.Valid() {
		return &vt.DocumentTitle.Value
	} else if vt := u.OfContentBlockLocation; vt != nil && vt.DocumentTitle.Valid() {
		return &vt.DocumentTitle.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaTextCitationParamUnion) GetType() *string {
	if vt := u.OfCharLocation; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfPageLocation; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfContentBlockLocation; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfWebSearchResultLocation; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

type BetaTextDelta struct {
	Text string             `json:"text,required"`
	Type constant.TextDelta `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Text        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaTextDelta) RawJSON() string { return r.JSON.raw }
func (r *BetaTextDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaThinkingBlock struct {
	Signature string            `json:"signature,required"`
	Thinking  string            `json:"thinking,required"`
	Type      constant.Thinking `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Signature   respjson.Field
		Thinking    respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaThinkingBlock) RawJSON() string { return r.JSON.raw }
func (r *BetaThinkingBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func (r BetaThinkingBlock) ToParam() BetaThinkingBlockParam {
	var p BetaThinkingBlockParam
	p.Type = r.Type
	p.Signature = r.Signature
	p.Thinking = r.Thinking
	return p
}

// The properties Signature, Thinking, Type are required.
type BetaThinkingBlockParam struct {
	Signature string `json:"signature,required"`
	Thinking  string `json:"thinking,required"`
	// This field can be elided, and will marshal its zero value as "thinking".
	Type constant.Thinking `json:"type,required"`
	paramObj
}

func (r BetaThinkingBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaThinkingBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThinkingBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func NewBetaThinkingConfigDisabledParam() BetaThinkingConfigDisabledParam {
	return BetaThinkingConfigDisabledParam{
		Type: "disabled",
	}
}

// This struct has a constant value, construct it with
// [NewBetaThinkingConfigDisabledParam].
type BetaThinkingConfigDisabledParam struct {
	Type constant.Disabled `json:"type,required"`
	paramObj
}

func (r BetaThinkingConfigDisabledParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaThinkingConfigDisabledParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThinkingConfigDisabledParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties BudgetTokens, Type are required.
type BetaThinkingConfigEnabledParam struct {
	// Determines how many tokens Claude can use for its internal reasoning process.
	// Larger budgets can enable more thorough analysis for complex problems, improving
	// response quality.
	//
	// Must be ≥1024 and less than `max_tokens`.
	//
	// See
	// [extended thinking](https://docs.anthropic.com/en/docs/build-with-claude/extended-thinking)
	// for details.
	BudgetTokens int64 `json:"budget_tokens,required"`
	// This field can be elided, and will marshal its zero value as "enabled".
	Type constant.Enabled `json:"type,required"`
	paramObj
}

func (r BetaThinkingConfigEnabledParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaThinkingConfigEnabledParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaThinkingConfigEnabledParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func BetaThinkingConfigParamOfEnabled(budgetTokens int64) BetaThinkingConfigParamUnion {
	var enabled BetaThinkingConfigEnabledParam
	enabled.BudgetTokens = budgetTokens
	return BetaThinkingConfigParamUnion{OfEnabled: &enabled}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaThinkingConfigParamUnion struct {
	OfEnabled  *BetaThinkingConfigEnabledParam  `json:",omitzero,inline"`
	OfDisabled *BetaThinkingConfigDisabledParam `json:",omitzero,inline"`
	paramUnion
}

func (u BetaThinkingConfigParamUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfEnabled, u.OfDisabled)
}
func (u *BetaThinkingConfigParamUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaThinkingConfigParamUnion) asAny() any {
	if !param.IsOmitted(u.OfEnabled) {
		return u.OfEnabled
	} else if !param.IsOmitted(u.OfDisabled) {
		return u.OfDisabled
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaThinkingConfigParamUnion) GetBudgetTokens() *int64 {
	if vt := u.OfEnabled; vt != nil {
		return &vt.BudgetTokens
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaThinkingConfigParamUnion) GetType() *string {
	if vt := u.OfEnabled; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfDisabled; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

type BetaThinkingDelta struct {
	Thinking string                 `json:"thinking,required"`
	Type     constant.ThinkingDelta `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Thinking    respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaThinkingDelta) RawJSON() string { return r.JSON.raw }
func (r *BetaThinkingDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties InputSchema, Name are required.
type BetaToolParam struct {
	// [JSON schema](https://json-schema.org/draft/2020-12) for this tool's input.
	//
	// This defines the shape of the `input` that your tool accepts and that the model
	// will produce.
	InputSchema BetaToolInputSchemaParam `json:"input_schema,omitzero,required"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	Name string `json:"name,required"`
	// Description of what this tool does.
	//
	// Tool descriptions should be as detailed as possible. The more information that
	// the model has about what the tool is and how to use it, the better it will
	// perform. You can use natural language descriptions to reinforce important
	// aspects of the tool input JSON schema.
	Description param.Opt[string] `json:"description,omitzero"`
	// Any of "custom".
	Type BetaToolType `json:"type,omitzero"`
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam `json:"cache_control,omitzero"`
	paramObj
}

func (r BetaToolParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaToolParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaToolParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// [JSON schema](https://json-schema.org/draft/2020-12) for this tool's input.
//
// This defines the shape of the `input` that your tool accepts and that the model
// will produce.
//
// The property Type is required.
type BetaToolInputSchemaParam struct {
	Properties any      `json:"properties,omitzero"`
	Required   []string `json:"required,omitzero"`
	// This field can be elided, and will marshal its zero value as "object".
	Type        constant.Object `json:"type,required"`
	ExtraFields map[string]any  `json:"-"`
	paramObj
}

func (r BetaToolInputSchemaParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaToolInputSchemaParam
	return param.MarshalWithExtras(r, (*shadow)(&r), r.ExtraFields)
}
func (r *BetaToolInputSchemaParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaToolType string

const (
	BetaToolTypeCustom BetaToolType = "custom"
)

// The properties Name, Type are required.
type BetaToolBash20241022Param struct {
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam `json:"cache_control,omitzero"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	//
	// This field can be elided, and will marshal its zero value as "bash".
	Name constant.Bash `json:"name,required"`
	// This field can be elided, and will marshal its zero value as "bash_20241022".
	Type constant.Bash20241022 `json:"type,required"`
	paramObj
}

func (r BetaToolBash20241022Param) MarshalJSON() (data []byte, err error) {
	type shadow BetaToolBash20241022Param
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaToolBash20241022Param) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Name, Type are required.
type BetaToolBash20250124Param struct {
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam `json:"cache_control,omitzero"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	//
	// This field can be elided, and will marshal its zero value as "bash".
	Name constant.Bash `json:"name,required"`
	// This field can be elided, and will marshal its zero value as "bash_20250124".
	Type constant.Bash20250124 `json:"type,required"`
	paramObj
}

func (r BetaToolBash20250124Param) MarshalJSON() (data []byte, err error) {
	type shadow BetaToolBash20250124Param
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaToolBash20250124Param) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func BetaToolChoiceParamOfTool(name string) BetaToolChoiceUnionParam {
	var tool BetaToolChoiceToolParam
	tool.Name = name
	return BetaToolChoiceUnionParam{OfTool: &tool}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaToolChoiceUnionParam struct {
	OfAuto *BetaToolChoiceAutoParam `json:",omitzero,inline"`
	OfAny  *BetaToolChoiceAnyParam  `json:",omitzero,inline"`
	OfTool *BetaToolChoiceToolParam `json:",omitzero,inline"`
	OfNone *BetaToolChoiceNoneParam `json:",omitzero,inline"`
	paramUnion
}

func (u BetaToolChoiceUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfAny, u.OfTool, u.OfNone)
}
func (u *BetaToolChoiceUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaToolChoiceUnionParam) asAny() any {
	if !param.IsOmitted(u.OfAuto) {
		return u.OfAuto
	} else if !param.IsOmitted(u.OfAny) {
		return u.OfAny
	} else if !param.IsOmitted(u.OfTool) {
		return u.OfTool
	} else if !param.IsOmitted(u.OfNone) {
		return u.OfNone
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaToolChoiceUnionParam) GetName() *string {
	if vt := u.OfTool; vt != nil {
		return &vt.Name
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaToolChoiceUnionParam) GetType() *string {
	if vt := u.OfAuto; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfAny; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfTool; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfNone; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaToolChoiceUnionParam) GetDisableParallelToolUse() *bool {
	if vt := u.OfAuto; vt != nil && vt.DisableParallelToolUse.Valid() {
		return &vt.DisableParallelToolUse.Value
	} else if vt := u.OfAny; vt != nil && vt.DisableParallelToolUse.Valid() {
		return &vt.DisableParallelToolUse.Value
	} else if vt := u.OfTool; vt != nil && vt.DisableParallelToolUse.Valid() {
		return &vt.DisableParallelToolUse.Value
	}
	return nil
}

// The model will use any available tools.
//
// The property Type is required.
type BetaToolChoiceAnyParam struct {
	// Whether to disable parallel tool use.
	//
	// Defaults to `false`. If set to `true`, the model will output exactly one tool
	// use.
	DisableParallelToolUse param.Opt[bool] `json:"disable_parallel_tool_use,omitzero"`
	// This field can be elided, and will marshal its zero value as "any".
	Type constant.Any `json:"type,required"`
	paramObj
}

func (r BetaToolChoiceAnyParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaToolChoiceAnyParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaToolChoiceAnyParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The model will automatically decide whether to use tools.
//
// The property Type is required.
type BetaToolChoiceAutoParam struct {
	// Whether to disable parallel tool use.
	//
	// Defaults to `false`. If set to `true`, the model will output at most one tool
	// use.
	DisableParallelToolUse param.Opt[bool] `json:"disable_parallel_tool_use,omitzero"`
	// This field can be elided, and will marshal its zero value as "auto".
	Type constant.Auto `json:"type,required"`
	paramObj
}

func (r BetaToolChoiceAutoParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaToolChoiceAutoParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaToolChoiceAutoParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func NewBetaToolChoiceNoneParam() BetaToolChoiceNoneParam {
	return BetaToolChoiceNoneParam{
		Type: "none",
	}
}

// The model will not be allowed to use tools.
//
// This struct has a constant value, construct it with
// [NewBetaToolChoiceNoneParam].
type BetaToolChoiceNoneParam struct {
	Type constant.None `json:"type,required"`
	paramObj
}

func (r BetaToolChoiceNoneParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaToolChoiceNoneParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaToolChoiceNoneParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The model will use the specified tool with `tool_choice.name`.
//
// The properties Name, Type are required.
type BetaToolChoiceToolParam struct {
	// The name of the tool to use.
	Name string `json:"name,required"`
	// Whether to disable parallel tool use.
	//
	// Defaults to `false`. If set to `true`, the model will output exactly one tool
	// use.
	DisableParallelToolUse param.Opt[bool] `json:"disable_parallel_tool_use,omitzero"`
	// This field can be elided, and will marshal its zero value as "tool".
	Type constant.Tool `json:"type,required"`
	paramObj
}

func (r BetaToolChoiceToolParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaToolChoiceToolParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaToolChoiceToolParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties DisplayHeightPx, DisplayWidthPx, Name, Type are required.
type BetaToolComputerUse20241022Param struct {
	// The height of the display in pixels.
	DisplayHeightPx int64 `json:"display_height_px,required"`
	// The width of the display in pixels.
	DisplayWidthPx int64 `json:"display_width_px,required"`
	// The X11 display number (e.g. 0, 1) for the display.
	DisplayNumber param.Opt[int64] `json:"display_number,omitzero"`
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam `json:"cache_control,omitzero"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	//
	// This field can be elided, and will marshal its zero value as "computer".
	Name constant.Computer `json:"name,required"`
	// This field can be elided, and will marshal its zero value as
	// "computer_20241022".
	Type constant.Computer20241022 `json:"type,required"`
	paramObj
}

func (r BetaToolComputerUse20241022Param) MarshalJSON() (data []byte, err error) {
	type shadow BetaToolComputerUse20241022Param
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaToolComputerUse20241022Param) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties DisplayHeightPx, DisplayWidthPx, Name, Type are required.
type BetaToolComputerUse20250124Param struct {
	// The height of the display in pixels.
	DisplayHeightPx int64 `json:"display_height_px,required"`
	// The width of the display in pixels.
	DisplayWidthPx int64 `json:"display_width_px,required"`
	// The X11 display number (e.g. 0, 1) for the display.
	DisplayNumber param.Opt[int64] `json:"display_number,omitzero"`
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam `json:"cache_control,omitzero"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	//
	// This field can be elided, and will marshal its zero value as "computer".
	Name constant.Computer `json:"name,required"`
	// This field can be elided, and will marshal its zero value as
	// "computer_20250124".
	Type constant.Computer20250124 `json:"type,required"`
	paramObj
}

func (r BetaToolComputerUse20250124Param) MarshalJSON() (data []byte, err error) {
	type shadow BetaToolComputerUse20250124Param
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaToolComputerUse20250124Param) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties ToolUseID, Type are required.
type BetaToolResultBlockParam struct {
	ToolUseID string          `json:"tool_use_id,required"`
	IsError   param.Opt[bool] `json:"is_error,omitzero"`
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam         `json:"cache_control,omitzero"`
	Content      []BetaToolResultBlockParamContentUnion `json:"content,omitzero"`
	// This field can be elided, and will marshal its zero value as "tool_result".
	Type constant.ToolResult `json:"type,required"`
	paramObj
}

func (r BetaToolResultBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaToolResultBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaToolResultBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaToolResultBlockParamContentUnion struct {
	OfText  *BetaTextBlockParam  `json:",omitzero,inline"`
	OfImage *BetaImageBlockParam `json:",omitzero,inline"`
	paramUnion
}

func (u BetaToolResultBlockParamContentUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfText, u.OfImage)
}
func (u *BetaToolResultBlockParamContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaToolResultBlockParamContentUnion) asAny() any {
	if !param.IsOmitted(u.OfText) {
		return u.OfText
	} else if !param.IsOmitted(u.OfImage) {
		return u.OfImage
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaToolResultBlockParamContentUnion) GetText() *string {
	if vt := u.OfText; vt != nil {
		return &vt.Text
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaToolResultBlockParamContentUnion) GetCitations() []BetaTextCitationParamUnion {
	if vt := u.OfText; vt != nil {
		return vt.Citations
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaToolResultBlockParamContentUnion) GetSource() *BetaImageBlockParamSourceUnion {
	if vt := u.OfImage; vt != nil {
		return &vt.Source
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaToolResultBlockParamContentUnion) GetType() *string {
	if vt := u.OfText; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfImage; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Returns a pointer to the underlying variant's CacheControl property, if present.
func (u BetaToolResultBlockParamContentUnion) GetCacheControl() *BetaCacheControlEphemeralParam {
	if vt := u.OfText; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfImage; vt != nil {
		return &vt.CacheControl
	}
	return nil
}

// The properties Name, Type are required.
type BetaToolTextEditor20241022Param struct {
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam `json:"cache_control,omitzero"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	//
	// This field can be elided, and will marshal its zero value as
	// "str_replace_editor".
	Name constant.StrReplaceEditor `json:"name,required"`
	// This field can be elided, and will marshal its zero value as
	// "text_editor_20241022".
	Type constant.TextEditor20241022 `json:"type,required"`
	paramObj
}

func (r BetaToolTextEditor20241022Param) MarshalJSON() (data []byte, err error) {
	type shadow BetaToolTextEditor20241022Param
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaToolTextEditor20241022Param) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Name, Type are required.
type BetaToolTextEditor20250124Param struct {
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam `json:"cache_control,omitzero"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	//
	// This field can be elided, and will marshal its zero value as
	// "str_replace_editor".
	Name constant.StrReplaceEditor `json:"name,required"`
	// This field can be elided, and will marshal its zero value as
	// "text_editor_20250124".
	Type constant.TextEditor20250124 `json:"type,required"`
	paramObj
}

func (r BetaToolTextEditor20250124Param) MarshalJSON() (data []byte, err error) {
	type shadow BetaToolTextEditor20250124Param
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaToolTextEditor20250124Param) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Name, Type are required.
type BetaToolTextEditor20250429Param struct {
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam `json:"cache_control,omitzero"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	//
	// This field can be elided, and will marshal its zero value as
	// "str_replace_based_edit_tool".
	Name constant.StrReplaceBasedEditTool `json:"name,required"`
	// This field can be elided, and will marshal its zero value as
	// "text_editor_20250429".
	Type constant.TextEditor20250429 `json:"type,required"`
	paramObj
}

func (r BetaToolTextEditor20250429Param) MarshalJSON() (data []byte, err error) {
	type shadow BetaToolTextEditor20250429Param
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaToolTextEditor20250429Param) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func BetaToolUnionParamOfTool(inputSchema BetaToolInputSchemaParam, name string) BetaToolUnionParam {
	var variant BetaToolParam
	variant.InputSchema = inputSchema
	variant.Name = name
	return BetaToolUnionParam{OfTool: &variant}
}

func BetaToolUnionParamOfComputerUseTool20241022(displayHeightPx int64, displayWidthPx int64) BetaToolUnionParam {
	var variant BetaToolComputerUse20241022Param
	variant.DisplayHeightPx = displayHeightPx
	variant.DisplayWidthPx = displayWidthPx
	return BetaToolUnionParam{OfComputerUseTool20241022: &variant}
}

func BetaToolUnionParamOfComputerUseTool20250124(displayHeightPx int64, displayWidthPx int64) BetaToolUnionParam {
	var variant BetaToolComputerUse20250124Param
	variant.DisplayHeightPx = displayHeightPx
	variant.DisplayWidthPx = displayWidthPx
	return BetaToolUnionParam{OfComputerUseTool20250124: &variant}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaToolUnionParam struct {
	OfTool                      *BetaToolParam                      `json:",omitzero,inline"`
	OfComputerUseTool20241022   *BetaToolComputerUse20241022Param   `json:",omitzero,inline"`
	OfBashTool20241022          *BetaToolBash20241022Param          `json:",omitzero,inline"`
	OfTextEditor20241022        *BetaToolTextEditor20241022Param    `json:",omitzero,inline"`
	OfComputerUseTool20250124   *BetaToolComputerUse20250124Param   `json:",omitzero,inline"`
	OfBashTool20250124          *BetaToolBash20250124Param          `json:",omitzero,inline"`
	OfTextEditor20250124        *BetaToolTextEditor20250124Param    `json:",omitzero,inline"`
	OfTextEditor20250429        *BetaToolTextEditor20250429Param    `json:",omitzero,inline"`
	OfWebSearchTool20250305     *BetaWebSearchTool20250305Param     `json:",omitzero,inline"`
	OfCodeExecutionTool20250522 *BetaCodeExecutionTool20250522Param `json:",omitzero,inline"`
	paramUnion
}

func (u BetaToolUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfTool,
		u.OfComputerUseTool20241022,
		u.OfBashTool20241022,
		u.OfTextEditor20241022,
		u.OfComputerUseTool20250124,
		u.OfBashTool20250124,
		u.OfTextEditor20250124,
		u.OfTextEditor20250429,
		u.OfWebSearchTool20250305,
		u.OfCodeExecutionTool20250522)
}
func (u *BetaToolUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaToolUnionParam) asAny() any {
	if !param.IsOmitted(u.OfTool) {
		return u.OfTool
	} else if !param.IsOmitted(u.OfComputerUseTool20241022) {
		return u.OfComputerUseTool20241022
	} else if !param.IsOmitted(u.OfBashTool20241022) {
		return u.OfBashTool20241022
	} else if !param.IsOmitted(u.OfTextEditor20241022) {
		return u.OfTextEditor20241022
	} else if !param.IsOmitted(u.OfComputerUseTool20250124) {
		return u.OfComputerUseTool20250124
	} else if !param.IsOmitted(u.OfBashTool20250124) {
		return u.OfBashTool20250124
	} else if !param.IsOmitted(u.OfTextEditor20250124) {
		return u.OfTextEditor20250124
	} else if !param.IsOmitted(u.OfTextEditor20250429) {
		return u.OfTextEditor20250429
	} else if !param.IsOmitted(u.OfWebSearchTool20250305) {
		return u.OfWebSearchTool20250305
	} else if !param.IsOmitted(u.OfCodeExecutionTool20250522) {
		return u.OfCodeExecutionTool20250522
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaToolUnionParam) GetInputSchema() *BetaToolInputSchemaParam {
	if vt := u.OfTool; vt != nil {
		return &vt.InputSchema
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaToolUnionParam) GetDescription() *string {
	if vt := u.OfTool; vt != nil && vt.Description.Valid() {
		return &vt.Description.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaToolUnionParam) GetAllowedDomains() []string {
	if vt := u.OfWebSearchTool20250305; vt != nil {
		return vt.AllowedDomains
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaToolUnionParam) GetBlockedDomains() []string {
	if vt := u.OfWebSearchTool20250305; vt != nil {
		return vt.BlockedDomains
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaToolUnionParam) GetMaxUses() *int64 {
	if vt := u.OfWebSearchTool20250305; vt != nil && vt.MaxUses.Valid() {
		return &vt.MaxUses.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaToolUnionParam) GetUserLocation() *BetaWebSearchTool20250305UserLocationParam {
	if vt := u.OfWebSearchTool20250305; vt != nil {
		return &vt.UserLocation
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaToolUnionParam) GetName() *string {
	if vt := u.OfTool; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfComputerUseTool20241022; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfBashTool20241022; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfTextEditor20241022; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfComputerUseTool20250124; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfBashTool20250124; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfTextEditor20250124; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfTextEditor20250429; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfWebSearchTool20250305; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfCodeExecutionTool20250522; vt != nil {
		return (*string)(&vt.Name)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaToolUnionParam) GetType() *string {
	if vt := u.OfTool; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfComputerUseTool20241022; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfBashTool20241022; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfTextEditor20241022; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfComputerUseTool20250124; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfBashTool20250124; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfTextEditor20250124; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfTextEditor20250429; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfWebSearchTool20250305; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfCodeExecutionTool20250522; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaToolUnionParam) GetDisplayHeightPx() *int64 {
	if vt := u.OfComputerUseTool20241022; vt != nil {
		return (*int64)(&vt.DisplayHeightPx)
	} else if vt := u.OfComputerUseTool20250124; vt != nil {
		return (*int64)(&vt.DisplayHeightPx)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaToolUnionParam) GetDisplayWidthPx() *int64 {
	if vt := u.OfComputerUseTool20241022; vt != nil {
		return (*int64)(&vt.DisplayWidthPx)
	} else if vt := u.OfComputerUseTool20250124; vt != nil {
		return (*int64)(&vt.DisplayWidthPx)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaToolUnionParam) GetDisplayNumber() *int64 {
	if vt := u.OfComputerUseTool20241022; vt != nil && vt.DisplayNumber.Valid() {
		return &vt.DisplayNumber.Value
	} else if vt := u.OfComputerUseTool20250124; vt != nil && vt.DisplayNumber.Valid() {
		return &vt.DisplayNumber.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's CacheControl property, if present.
func (u BetaToolUnionParam) GetCacheControl() *BetaCacheControlEphemeralParam {
	if vt := u.OfTool; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfComputerUseTool20241022; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfBashTool20241022; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfTextEditor20241022; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfComputerUseTool20250124; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfBashTool20250124; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfTextEditor20250124; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfTextEditor20250429; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfWebSearchTool20250305; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfCodeExecutionTool20250522; vt != nil {
		return &vt.CacheControl
	}
	return nil
}

type BetaToolUseBlock struct {
	ID    string           `json:"id,required"`
	Input any              `json:"input,required"`
	Name  string           `json:"name,required"`
	Type  constant.ToolUse `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Input       respjson.Field
		Name        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaToolUseBlock) RawJSON() string { return r.JSON.raw }
func (r *BetaToolUseBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func (r BetaToolUseBlock) ToParam() BetaToolUseBlockParam {
	var p BetaToolUseBlockParam
	p.Type = r.Type
	p.ID = r.ID
	p.Input = r.Input
	p.Name = r.Name
	return p
}

// The properties ID, Input, Name, Type are required.
type BetaToolUseBlockParam struct {
	ID    string `json:"id,required"`
	Input any    `json:"input,omitzero,required"`
	Name  string `json:"name,required"`
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam `json:"cache_control,omitzero"`
	// This field can be elided, and will marshal its zero value as "tool_use".
	Type constant.ToolUse `json:"type,required"`
	paramObj
}

func (r BetaToolUseBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaToolUseBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaToolUseBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Type, URL are required.
type BetaURLImageSourceParam struct {
	URL string `json:"url,required"`
	// This field can be elided, and will marshal its zero value as "url".
	Type constant.URL `json:"type,required"`
	paramObj
}

func (r BetaURLImageSourceParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaURLImageSourceParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaURLImageSourceParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Type, URL are required.
type BetaURLPDFSourceParam struct {
	URL string `json:"url,required"`
	// This field can be elided, and will marshal its zero value as "url".
	Type constant.URL `json:"type,required"`
	paramObj
}

func (r BetaURLPDFSourceParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaURLPDFSourceParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaURLPDFSourceParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaUsage struct {
	// Breakdown of cached tokens by TTL
	CacheCreation BetaCacheCreation `json:"cache_creation,required"`
	// The number of input tokens used to create the cache entry.
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens,required"`
	// The number of input tokens read from the cache.
	CacheReadInputTokens int64 `json:"cache_read_input_tokens,required"`
	// The number of input tokens which were used.
	InputTokens int64 `json:"input_tokens,required"`
	// The number of output tokens which were used.
	OutputTokens int64 `json:"output_tokens,required"`
	// The number of server tool requests.
	ServerToolUse BetaServerToolUsage `json:"server_tool_use,required"`
	// If the request used the priority, standard, or batch tier.
	//
	// Any of "standard", "priority", "batch".
	ServiceTier BetaUsageServiceTier `json:"service_tier,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		CacheCreation            respjson.Field
		CacheCreationInputTokens respjson.Field
		CacheReadInputTokens     respjson.Field
		InputTokens              respjson.Field
		OutputTokens             respjson.Field
		ServerToolUse            respjson.Field
		ServiceTier              respjson.Field
		ExtraFields              map[string]respjson.Field
		raw                      string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaUsage) RawJSON() string { return r.JSON.raw }
func (r *BetaUsage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// If the request used the priority, standard, or batch tier.
type BetaUsageServiceTier string

const (
	BetaUsageServiceTierStandard BetaUsageServiceTier = "standard"
	BetaUsageServiceTierPriority BetaUsageServiceTier = "priority"
	BetaUsageServiceTierBatch    BetaUsageServiceTier = "batch"
)

type BetaWebSearchResultBlock struct {
	EncryptedContent string                   `json:"encrypted_content,required"`
	PageAge          string                   `json:"page_age,required"`
	Title            string                   `json:"title,required"`
	Type             constant.WebSearchResult `json:"type,required"`
	URL              string                   `json:"url,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		EncryptedContent respjson.Field
		PageAge          respjson.Field
		Title            respjson.Field
		Type             respjson.Field
		URL              respjson.Field
		ExtraFields      map[string]respjson.Field
		raw              string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaWebSearchResultBlock) RawJSON() string { return r.JSON.raw }
func (r *BetaWebSearchResultBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties EncryptedContent, Title, Type, URL are required.
type BetaWebSearchResultBlockParam struct {
	EncryptedContent string            `json:"encrypted_content,required"`
	Title            string            `json:"title,required"`
	URL              string            `json:"url,required"`
	PageAge          param.Opt[string] `json:"page_age,omitzero"`
	// This field can be elided, and will marshal its zero value as
	// "web_search_result".
	Type constant.WebSearchResult `json:"type,required"`
	paramObj
}

func (r BetaWebSearchResultBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaWebSearchResultBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaWebSearchResultBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Name, Type are required.
type BetaWebSearchTool20250305Param struct {
	// Maximum number of times the tool can be used in the API request.
	MaxUses param.Opt[int64] `json:"max_uses,omitzero"`
	// If provided, only these domains will be included in results. Cannot be used
	// alongside `blocked_domains`.
	AllowedDomains []string `json:"allowed_domains,omitzero"`
	// If provided, these domains will never appear in results. Cannot be used
	// alongside `allowed_domains`.
	BlockedDomains []string `json:"blocked_domains,omitzero"`
	// Parameters for the user's location. Used to provide more relevant search
	// results.
	UserLocation BetaWebSearchTool20250305UserLocationParam `json:"user_location,omitzero"`
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam `json:"cache_control,omitzero"`
	// Name of the tool.
	//
	// This is how the tool will be called by the model and in `tool_use` blocks.
	//
	// This field can be elided, and will marshal its zero value as "web_search".
	Name constant.WebSearch `json:"name,required"`
	// This field can be elided, and will marshal its zero value as
	// "web_search_20250305".
	Type constant.WebSearch20250305 `json:"type,required"`
	paramObj
}

func (r BetaWebSearchTool20250305Param) MarshalJSON() (data []byte, err error) {
	type shadow BetaWebSearchTool20250305Param
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaWebSearchTool20250305Param) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Parameters for the user's location. Used to provide more relevant search
// results.
//
// The property Type is required.
type BetaWebSearchTool20250305UserLocationParam struct {
	// The city of the user.
	City param.Opt[string] `json:"city,omitzero"`
	// The two letter
	// [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2) of the
	// user.
	Country param.Opt[string] `json:"country,omitzero"`
	// The region of the user.
	Region param.Opt[string] `json:"region,omitzero"`
	// The [IANA timezone](https://nodatime.org/TimeZones) of the user.
	Timezone param.Opt[string] `json:"timezone,omitzero"`
	// This field can be elided, and will marshal its zero value as "approximate".
	Type constant.Approximate `json:"type,required"`
	paramObj
}

func (r BetaWebSearchTool20250305UserLocationParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaWebSearchTool20250305UserLocationParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaWebSearchTool20250305UserLocationParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties ErrorCode, Type are required.
type BetaWebSearchToolRequestErrorParam struct {
	// Any of "invalid_tool_input", "unavailable", "max_uses_exceeded",
	// "too_many_requests", "query_too_long".
	ErrorCode BetaWebSearchToolResultErrorCode `json:"error_code,omitzero,required"`
	// This field can be elided, and will marshal its zero value as
	// "web_search_tool_result_error".
	Type constant.WebSearchToolResultError `json:"type,required"`
	paramObj
}

func (r BetaWebSearchToolRequestErrorParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaWebSearchToolRequestErrorParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaWebSearchToolRequestErrorParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaWebSearchToolResultBlock struct {
	Content   BetaWebSearchToolResultBlockContentUnion `json:"content,required"`
	ToolUseID string                                   `json:"tool_use_id,required"`
	Type      constant.WebSearchToolResult             `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Content     respjson.Field
		ToolUseID   respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaWebSearchToolResultBlock) RawJSON() string { return r.JSON.raw }
func (r *BetaWebSearchToolResultBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// BetaWebSearchToolResultBlockContentUnion contains all possible properties and
// values from [BetaWebSearchToolResultError], [[]BetaWebSearchResultBlock].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfBetaWebSearchResultBlockArray]
type BetaWebSearchToolResultBlockContentUnion struct {
	// This field will be present if the value is a [[]BetaWebSearchResultBlock]
	// instead of an object.
	OfBetaWebSearchResultBlockArray []BetaWebSearchResultBlock `json:",inline"`
	// This field is from variant [BetaWebSearchToolResultError].
	ErrorCode BetaWebSearchToolResultErrorCode `json:"error_code"`
	// This field is from variant [BetaWebSearchToolResultError].
	Type constant.WebSearchToolResultError `json:"type"`
	JSON struct {
		OfBetaWebSearchResultBlockArray respjson.Field
		ErrorCode                       respjson.Field
		Type                            respjson.Field
		raw                             string
	} `json:"-"`
}

func (u BetaWebSearchToolResultBlockContentUnion) AsResponseWebSearchToolResultError() (v BetaWebSearchToolResultError) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaWebSearchToolResultBlockContentUnion) AsBetaWebSearchResultBlockArray() (v []BetaWebSearchResultBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u BetaWebSearchToolResultBlockContentUnion) RawJSON() string { return u.JSON.raw }

func (r *BetaWebSearchToolResultBlockContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Content, ToolUseID, Type are required.
type BetaWebSearchToolResultBlockParam struct {
	Content   BetaWebSearchToolResultBlockParamContentUnion `json:"content,omitzero,required"`
	ToolUseID string                                        `json:"tool_use_id,required"`
	// Create a cache control breakpoint at this content block.
	CacheControl BetaCacheControlEphemeralParam `json:"cache_control,omitzero"`
	// This field can be elided, and will marshal its zero value as
	// "web_search_tool_result".
	Type constant.WebSearchToolResult `json:"type,required"`
	paramObj
}

func (r BetaWebSearchToolResultBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow BetaWebSearchToolResultBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaWebSearchToolResultBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func BetaNewWebSearchToolRequestError(errorCode BetaWebSearchToolResultErrorCode) BetaWebSearchToolResultBlockParamContentUnion {
	var variant BetaWebSearchToolRequestErrorParam
	variant.ErrorCode = errorCode
	return BetaWebSearchToolResultBlockParamContentUnion{OfError: &variant}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaWebSearchToolResultBlockParamContentUnion struct {
	OfResultBlock []BetaWebSearchResultBlockParam     `json:",omitzero,inline"`
	OfError       *BetaWebSearchToolRequestErrorParam `json:",omitzero,inline"`
	paramUnion
}

func (u BetaWebSearchToolResultBlockParamContentUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfResultBlock, u.OfError)
}
func (u *BetaWebSearchToolResultBlockParamContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaWebSearchToolResultBlockParamContentUnion) asAny() any {
	if !param.IsOmitted(u.OfResultBlock) {
		return &u.OfResultBlock
	} else if !param.IsOmitted(u.OfError) {
		return u.OfError
	}
	return nil
}

type BetaWebSearchToolResultError struct {
	// Any of "invalid_tool_input", "unavailable", "max_uses_exceeded",
	// "too_many_requests", "query_too_long".
	ErrorCode BetaWebSearchToolResultErrorCode  `json:"error_code,required"`
	Type      constant.WebSearchToolResultError `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ErrorCode   respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaWebSearchToolResultError) RawJSON() string { return r.JSON.raw }
func (r *BetaWebSearchToolResultError) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaWebSearchToolResultErrorCode string

const (
	BetaWebSearchToolResultErrorCodeInvalidToolInput BetaWebSearchToolResultErrorCode = "invalid_tool_input"
	BetaWebSearchToolResultErrorCodeUnavailable      BetaWebSearchToolResultErrorCode = "unavailable"
	BetaWebSearchToolResultErrorCodeMaxUsesExceeded  BetaWebSearchToolResultErrorCode = "max_uses_exceeded"
	BetaWebSearchToolResultErrorCodeTooManyRequests  BetaWebSearchToolResultErrorCode = "too_many_requests"
	BetaWebSearchToolResultErrorCodeQueryTooLong     BetaWebSearchToolResultErrorCode = "query_too_long"
)

type BetaMessageNewParams struct {
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
	Messages []BetaMessageParam `json:"messages,omitzero,required"`
	// The model that will complete your prompt.\n\nSee
	// [models](https://docs.anthropic.com/en/docs/models-overview) for additional
	// details and options.
	Model Model `json:"model,omitzero,required"`
	// Container identifier for reuse across requests.
	Container param.Opt[string] `json:"container,omitzero"`
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
	// MCP servers to be utilized in this request
	MCPServers []BetaRequestMCPServerURLDefinitionParam `json:"mcp_servers,omitzero"`
	// An object describing metadata about the request.
	Metadata BetaMetadataParam `json:"metadata,omitzero"`
	// Determines whether to use priority capacity (if available) or standard capacity
	// for this request.
	//
	// Anthropic offers different levels of service for your API requests. See
	// [service-tiers](https://docs.anthropic.com/en/api/service-tiers) for details.
	//
	// Any of "auto", "standard_only".
	ServiceTier BetaMessageNewParamsServiceTier `json:"service_tier,omitzero"`
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
	System []BetaTextBlockParam `json:"system,omitzero"`
	// Configuration for enabling Claude's extended thinking.
	//
	// When enabled, responses include `thinking` content blocks showing Claude's
	// thinking process before the final answer. Requires a minimum budget of 1,024
	// tokens and counts towards your `max_tokens` limit.
	//
	// See
	// [extended thinking](https://docs.anthropic.com/en/docs/build-with-claude/extended-thinking)
	// for details.
	Thinking BetaThinkingConfigParamUnion `json:"thinking,omitzero"`
	// How the model should use the provided tools. The model can use a specific tool,
	// any available tool, decide by itself, or not use tools at all.
	ToolChoice BetaToolChoiceUnionParam `json:"tool_choice,omitzero"`
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
	Tools []BetaToolUnionParam `json:"tools,omitzero"`
	// Optional header to specify the beta version(s) you want to use.
	Betas []AnthropicBeta `header:"anthropic-beta,omitzero" json:"-"`
	paramObj
}

func (r BetaMessageNewParams) MarshalJSON() (data []byte, err error) {
	type shadow BetaMessageNewParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaMessageNewParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Determines whether to use priority capacity (if available) or standard capacity
// for this request.
//
// Anthropic offers different levels of service for your API requests. See
// [service-tiers](https://docs.anthropic.com/en/api/service-tiers) for details.
type BetaMessageNewParamsServiceTier string

const (
	BetaMessageNewParamsServiceTierAuto         BetaMessageNewParamsServiceTier = "auto"
	BetaMessageNewParamsServiceTierStandardOnly BetaMessageNewParamsServiceTier = "standard_only"
)

type BetaMessageCountTokensParams struct {
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
	Messages []BetaMessageParam `json:"messages,omitzero,required"`
	// The model that will complete your prompt.\n\nSee
	// [models](https://docs.anthropic.com/en/docs/models-overview) for additional
	// details and options.
	Model Model `json:"model,omitzero,required"`
	// MCP servers to be utilized in this request
	MCPServers []BetaRequestMCPServerURLDefinitionParam `json:"mcp_servers,omitzero"`
	// System prompt.
	//
	// A system prompt is a way of providing context and instructions to Claude, such
	// as specifying a particular goal or role. See our
	// [guide to system prompts](https://docs.anthropic.com/en/docs/system-prompts).
	System BetaMessageCountTokensParamsSystemUnion `json:"system,omitzero"`
	// Configuration for enabling Claude's extended thinking.
	//
	// When enabled, responses include `thinking` content blocks showing Claude's
	// thinking process before the final answer. Requires a minimum budget of 1,024
	// tokens and counts towards your `max_tokens` limit.
	//
	// See
	// [extended thinking](https://docs.anthropic.com/en/docs/build-with-claude/extended-thinking)
	// for details.
	Thinking BetaThinkingConfigParamUnion `json:"thinking,omitzero"`
	// How the model should use the provided tools. The model can use a specific tool,
	// any available tool, decide by itself, or not use tools at all.
	ToolChoice BetaToolChoiceUnionParam `json:"tool_choice,omitzero"`
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
	Tools []BetaMessageCountTokensParamsToolUnion `json:"tools,omitzero"`
	// Optional header to specify the beta version(s) you want to use.
	Betas []AnthropicBeta `header:"anthropic-beta,omitzero" json:"-"`
	paramObj
}

func (r BetaMessageCountTokensParams) MarshalJSON() (data []byte, err error) {
	type shadow BetaMessageCountTokensParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *BetaMessageCountTokensParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaMessageCountTokensParamsSystemUnion struct {
	OfString             param.Opt[string]    `json:",omitzero,inline"`
	OfBetaTextBlockArray []BetaTextBlockParam `json:",omitzero,inline"`
	paramUnion
}

func (u BetaMessageCountTokensParamsSystemUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfBetaTextBlockArray)
}
func (u *BetaMessageCountTokensParamsSystemUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaMessageCountTokensParamsSystemUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfBetaTextBlockArray) {
		return &u.OfBetaTextBlockArray
	}
	return nil
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type BetaMessageCountTokensParamsToolUnion struct {
	OfTool                      *BetaToolParam                      `json:",omitzero,inline"`
	OfComputerUseTool20241022   *BetaToolComputerUse20241022Param   `json:",omitzero,inline"`
	OfBashTool20241022          *BetaToolBash20241022Param          `json:",omitzero,inline"`
	OfTextEditor20241022        *BetaToolTextEditor20241022Param    `json:",omitzero,inline"`
	OfComputerUseTool20250124   *BetaToolComputerUse20250124Param   `json:",omitzero,inline"`
	OfBashTool20250124          *BetaToolBash20250124Param          `json:",omitzero,inline"`
	OfTextEditor20250124        *BetaToolTextEditor20250124Param    `json:",omitzero,inline"`
	OfTextEditor20250429        *BetaToolTextEditor20250429Param    `json:",omitzero,inline"`
	OfWebSearchTool20250305     *BetaWebSearchTool20250305Param     `json:",omitzero,inline"`
	OfCodeExecutionTool20250522 *BetaCodeExecutionTool20250522Param `json:",omitzero,inline"`
	paramUnion
}

func (u BetaMessageCountTokensParamsToolUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfTool,
		u.OfComputerUseTool20241022,
		u.OfBashTool20241022,
		u.OfTextEditor20241022,
		u.OfComputerUseTool20250124,
		u.OfBashTool20250124,
		u.OfTextEditor20250124,
		u.OfTextEditor20250429,
		u.OfWebSearchTool20250305,
		u.OfCodeExecutionTool20250522)
}
func (u *BetaMessageCountTokensParamsToolUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *BetaMessageCountTokensParamsToolUnion) asAny() any {
	if !param.IsOmitted(u.OfTool) {
		return u.OfTool
	} else if !param.IsOmitted(u.OfComputerUseTool20241022) {
		return u.OfComputerUseTool20241022
	} else if !param.IsOmitted(u.OfBashTool20241022) {
		return u.OfBashTool20241022
	} else if !param.IsOmitted(u.OfTextEditor20241022) {
		return u.OfTextEditor20241022
	} else if !param.IsOmitted(u.OfComputerUseTool20250124) {
		return u.OfComputerUseTool20250124
	} else if !param.IsOmitted(u.OfBashTool20250124) {
		return u.OfBashTool20250124
	} else if !param.IsOmitted(u.OfTextEditor20250124) {
		return u.OfTextEditor20250124
	} else if !param.IsOmitted(u.OfTextEditor20250429) {
		return u.OfTextEditor20250429
	} else if !param.IsOmitted(u.OfWebSearchTool20250305) {
		return u.OfWebSearchTool20250305
	} else if !param.IsOmitted(u.OfCodeExecutionTool20250522) {
		return u.OfCodeExecutionTool20250522
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaMessageCountTokensParamsToolUnion) GetInputSchema() *BetaToolInputSchemaParam {
	if vt := u.OfTool; vt != nil {
		return &vt.InputSchema
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaMessageCountTokensParamsToolUnion) GetDescription() *string {
	if vt := u.OfTool; vt != nil && vt.Description.Valid() {
		return &vt.Description.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaMessageCountTokensParamsToolUnion) GetAllowedDomains() []string {
	if vt := u.OfWebSearchTool20250305; vt != nil {
		return vt.AllowedDomains
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaMessageCountTokensParamsToolUnion) GetBlockedDomains() []string {
	if vt := u.OfWebSearchTool20250305; vt != nil {
		return vt.BlockedDomains
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaMessageCountTokensParamsToolUnion) GetMaxUses() *int64 {
	if vt := u.OfWebSearchTool20250305; vt != nil && vt.MaxUses.Valid() {
		return &vt.MaxUses.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaMessageCountTokensParamsToolUnion) GetUserLocation() *BetaWebSearchTool20250305UserLocationParam {
	if vt := u.OfWebSearchTool20250305; vt != nil {
		return &vt.UserLocation
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaMessageCountTokensParamsToolUnion) GetName() *string {
	if vt := u.OfTool; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfComputerUseTool20241022; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfBashTool20241022; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfTextEditor20241022; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfComputerUseTool20250124; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfBashTool20250124; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfTextEditor20250124; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfTextEditor20250429; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfWebSearchTool20250305; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfCodeExecutionTool20250522; vt != nil {
		return (*string)(&vt.Name)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaMessageCountTokensParamsToolUnion) GetType() *string {
	if vt := u.OfTool; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfComputerUseTool20241022; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfBashTool20241022; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfTextEditor20241022; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfComputerUseTool20250124; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfBashTool20250124; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfTextEditor20250124; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfTextEditor20250429; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfWebSearchTool20250305; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfCodeExecutionTool20250522; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaMessageCountTokensParamsToolUnion) GetDisplayHeightPx() *int64 {
	if vt := u.OfComputerUseTool20241022; vt != nil {
		return (*int64)(&vt.DisplayHeightPx)
	} else if vt := u.OfComputerUseTool20250124; vt != nil {
		return (*int64)(&vt.DisplayHeightPx)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaMessageCountTokensParamsToolUnion) GetDisplayWidthPx() *int64 {
	if vt := u.OfComputerUseTool20241022; vt != nil {
		return (*int64)(&vt.DisplayWidthPx)
	} else if vt := u.OfComputerUseTool20250124; vt != nil {
		return (*int64)(&vt.DisplayWidthPx)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u BetaMessageCountTokensParamsToolUnion) GetDisplayNumber() *int64 {
	if vt := u.OfComputerUseTool20241022; vt != nil && vt.DisplayNumber.Valid() {
		return &vt.DisplayNumber.Value
	} else if vt := u.OfComputerUseTool20250124; vt != nil && vt.DisplayNumber.Valid() {
		return &vt.DisplayNumber.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's CacheControl property, if present.
func (u BetaMessageCountTokensParamsToolUnion) GetCacheControl() *BetaCacheControlEphemeralParam {
	if vt := u.OfTool; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfComputerUseTool20241022; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfBashTool20241022; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfTextEditor20241022; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfComputerUseTool20250124; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfBashTool20250124; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfTextEditor20250124; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfTextEditor20250429; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfWebSearchTool20250305; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfCodeExecutionTool20250522; vt != nil {
		return &vt.CacheControl
	}
	return nil
}
