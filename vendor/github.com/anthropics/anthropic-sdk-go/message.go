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

// MessageService contains methods and other services that help with interacting
// with the anthropic API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewMessageService] method instead.
type MessageService struct {
	Options []option.RequestOption
	Batches MessageBatchService
}

// NewMessageService generates a new service that applies the given options to each
// request. These options are applied after the parent client's options (if there
// is one), and before any request-specific options.
func NewMessageService(opts ...option.RequestOption) (r MessageService) {
	r = MessageService{}
	r.Options = opts
	r.Batches = NewMessageBatchService(opts...)
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
func (r *MessageService) New(ctx context.Context, body MessageNewParams, opts ...option.RequestOption) (res *Message, err error) {
	opts = append(r.Options[:], opts...)

	// For non-streaming requests, calculate the appropriate timeout based on maxTokens
	// and check against model-specific limits
	timeout, timeoutErr := CalculateNonStreamingTimeout(int(body.MaxTokens), body.Model, opts)
	if timeoutErr != nil {
		return nil, timeoutErr
	}
	opts = append(opts, option.WithRequestTimeout(timeout))

	path := "v1/messages"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
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
func (r *MessageService) NewStreaming(ctx context.Context, body MessageNewParams, opts ...option.RequestOption) (stream *ssestream.Stream[MessageStreamEventUnion]) {
	var (
		raw *http.Response
		err error
	)
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithJSONSet("stream", true)}, opts...)
	path := "v1/messages"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &raw, opts...)
	return ssestream.NewStream[MessageStreamEventUnion](ssestream.NewDecoder(raw), err)
}

// Count the number of tokens in a Message.
//
// The Token Count API can be used to count the number of tokens in a Message,
// including tools, images, and documents, without creating it.
//
// Learn more about token counting in our
// [user guide](/en/docs/build-with-claude/token-counting)
func (r *MessageService) CountTokens(ctx context.Context, body MessageCountTokensParams, opts ...option.RequestOption) (res *MessageTokensCount, err error) {
	opts = append(r.Options[:], opts...)
	path := "v1/messages/count_tokens"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// The properties Data, MediaType, Type are required.
type Base64ImageSourceParam struct {
	Data string `json:"data,required" format:"byte"`
	// Any of "image/jpeg", "image/png", "image/gif", "image/webp".
	MediaType Base64ImageSourceMediaType `json:"media_type,omitzero,required"`
	// This field can be elided, and will marshal its zero value as "base64".
	Type constant.Base64 `json:"type,required"`
	paramObj
}

func (r Base64ImageSourceParam) MarshalJSON() (data []byte, err error) {
	type shadow Base64ImageSourceParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *Base64ImageSourceParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type Base64ImageSourceMediaType string

const (
	Base64ImageSourceMediaTypeImageJPEG Base64ImageSourceMediaType = "image/jpeg"
	Base64ImageSourceMediaTypeImagePNG  Base64ImageSourceMediaType = "image/png"
	Base64ImageSourceMediaTypeImageGIF  Base64ImageSourceMediaType = "image/gif"
	Base64ImageSourceMediaTypeImageWebP Base64ImageSourceMediaType = "image/webp"
)

// The properties Data, MediaType, Type are required.
type Base64PDFSourceParam struct {
	Data string `json:"data,required" format:"byte"`
	// This field can be elided, and will marshal its zero value as "application/pdf".
	MediaType constant.ApplicationPDF `json:"media_type,required"`
	// This field can be elided, and will marshal its zero value as "base64".
	Type constant.Base64 `json:"type,required"`
	paramObj
}

func (r Base64PDFSourceParam) MarshalJSON() (data []byte, err error) {
	type shadow Base64PDFSourceParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *Base64PDFSourceParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func NewCacheControlEphemeralParam() CacheControlEphemeralParam {
	return CacheControlEphemeralParam{
		Type: "ephemeral",
	}
}

// This struct has a constant value, construct it with
// [NewCacheControlEphemeralParam].
type CacheControlEphemeralParam struct {
	Type constant.Ephemeral `json:"type,required"`
	paramObj
}

func (r CacheControlEphemeralParam) MarshalJSON() (data []byte, err error) {
	type shadow CacheControlEphemeralParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *CacheControlEphemeralParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type CitationCharLocation struct {
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
func (r CitationCharLocation) RawJSON() string { return r.JSON.raw }
func (r *CitationCharLocation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties CitedText, DocumentIndex, DocumentTitle, EndCharIndex,
// StartCharIndex, Type are required.
type CitationCharLocationParam struct {
	DocumentTitle  param.Opt[string] `json:"document_title,omitzero,required"`
	CitedText      string            `json:"cited_text,required"`
	DocumentIndex  int64             `json:"document_index,required"`
	EndCharIndex   int64             `json:"end_char_index,required"`
	StartCharIndex int64             `json:"start_char_index,required"`
	// This field can be elided, and will marshal its zero value as "char_location".
	Type constant.CharLocation `json:"type,required"`
	paramObj
}

func (r CitationCharLocationParam) MarshalJSON() (data []byte, err error) {
	type shadow CitationCharLocationParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *CitationCharLocationParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type CitationContentBlockLocation struct {
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
func (r CitationContentBlockLocation) RawJSON() string { return r.JSON.raw }
func (r *CitationContentBlockLocation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties CitedText, DocumentIndex, DocumentTitle, EndBlockIndex,
// StartBlockIndex, Type are required.
type CitationContentBlockLocationParam struct {
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

func (r CitationContentBlockLocationParam) MarshalJSON() (data []byte, err error) {
	type shadow CitationContentBlockLocationParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *CitationContentBlockLocationParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type CitationPageLocation struct {
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
func (r CitationPageLocation) RawJSON() string { return r.JSON.raw }
func (r *CitationPageLocation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties CitedText, DocumentIndex, DocumentTitle, EndPageNumber,
// StartPageNumber, Type are required.
type CitationPageLocationParam struct {
	DocumentTitle   param.Opt[string] `json:"document_title,omitzero,required"`
	CitedText       string            `json:"cited_text,required"`
	DocumentIndex   int64             `json:"document_index,required"`
	EndPageNumber   int64             `json:"end_page_number,required"`
	StartPageNumber int64             `json:"start_page_number,required"`
	// This field can be elided, and will marshal its zero value as "page_location".
	Type constant.PageLocation `json:"type,required"`
	paramObj
}

func (r CitationPageLocationParam) MarshalJSON() (data []byte, err error) {
	type shadow CitationPageLocationParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *CitationPageLocationParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties CitedText, EncryptedIndex, Title, Type, URL are required.
type CitationWebSearchResultLocationParam struct {
	Title          param.Opt[string] `json:"title,omitzero,required"`
	CitedText      string            `json:"cited_text,required"`
	EncryptedIndex string            `json:"encrypted_index,required"`
	URL            string            `json:"url,required"`
	// This field can be elided, and will marshal its zero value as
	// "web_search_result_location".
	Type constant.WebSearchResultLocation `json:"type,required"`
	paramObj
}

func (r CitationWebSearchResultLocationParam) MarshalJSON() (data []byte, err error) {
	type shadow CitationWebSearchResultLocationParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *CitationWebSearchResultLocationParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type CitationsConfigParam struct {
	Enabled param.Opt[bool] `json:"enabled,omitzero"`
	paramObj
}

func (r CitationsConfigParam) MarshalJSON() (data []byte, err error) {
	type shadow CitationsConfigParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *CitationsConfigParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type CitationsDelta struct {
	Citation CitationsDeltaCitationUnion `json:"citation,required"`
	Type     constant.CitationsDelta     `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Citation    respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r CitationsDelta) RawJSON() string { return r.JSON.raw }
func (r *CitationsDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// CitationsDeltaCitationUnion contains all possible properties and values from
// [CitationCharLocation], [CitationPageLocation], [CitationContentBlockLocation],
// [CitationsWebSearchResultLocation].
//
// Use the [CitationsDeltaCitationUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type CitationsDeltaCitationUnion struct {
	CitedText     string `json:"cited_text"`
	DocumentIndex int64  `json:"document_index"`
	DocumentTitle string `json:"document_title"`
	// This field is from variant [CitationCharLocation].
	EndCharIndex int64 `json:"end_char_index"`
	// This field is from variant [CitationCharLocation].
	StartCharIndex int64 `json:"start_char_index"`
	// Any of "char_location", "page_location", "content_block_location",
	// "web_search_result_location".
	Type string `json:"type"`
	// This field is from variant [CitationPageLocation].
	EndPageNumber int64 `json:"end_page_number"`
	// This field is from variant [CitationPageLocation].
	StartPageNumber int64 `json:"start_page_number"`
	// This field is from variant [CitationContentBlockLocation].
	EndBlockIndex int64 `json:"end_block_index"`
	// This field is from variant [CitationContentBlockLocation].
	StartBlockIndex int64 `json:"start_block_index"`
	// This field is from variant [CitationsWebSearchResultLocation].
	EncryptedIndex string `json:"encrypted_index"`
	// This field is from variant [CitationsWebSearchResultLocation].
	Title string `json:"title"`
	// This field is from variant [CitationsWebSearchResultLocation].
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

// anyCitationsDeltaCitation is implemented by each variant of
// [CitationsDeltaCitationUnion] to add type safety for the return type of
// [CitationsDeltaCitationUnion.AsAny]
type anyCitationsDeltaCitation interface {
	implCitationsDeltaCitationUnion()
}

func (CitationCharLocation) implCitationsDeltaCitationUnion()             {}
func (CitationPageLocation) implCitationsDeltaCitationUnion()             {}
func (CitationContentBlockLocation) implCitationsDeltaCitationUnion()     {}
func (CitationsWebSearchResultLocation) implCitationsDeltaCitationUnion() {}

// Use the following switch statement to find the correct variant
//
//	switch variant := CitationsDeltaCitationUnion.AsAny().(type) {
//	case anthropic.CitationCharLocation:
//	case anthropic.CitationPageLocation:
//	case anthropic.CitationContentBlockLocation:
//	case anthropic.CitationsWebSearchResultLocation:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u CitationsDeltaCitationUnion) AsAny() anyCitationsDeltaCitation {
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

func (u CitationsDeltaCitationUnion) AsCharLocation() (v CitationCharLocation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u CitationsDeltaCitationUnion) AsPageLocation() (v CitationPageLocation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u CitationsDeltaCitationUnion) AsContentBlockLocation() (v CitationContentBlockLocation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u CitationsDeltaCitationUnion) AsWebSearchResultLocation() (v CitationsWebSearchResultLocation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u CitationsDeltaCitationUnion) RawJSON() string { return u.JSON.raw }

func (r *CitationsDeltaCitationUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type CitationsWebSearchResultLocation struct {
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
func (r CitationsWebSearchResultLocation) RawJSON() string { return r.JSON.raw }
func (r *CitationsWebSearchResultLocation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ContentBlockUnion contains all possible properties and values from [TextBlock],
// [ToolUseBlock], [ServerToolUseBlock], [WebSearchToolResultBlock],
// [ThinkingBlock], [RedactedThinkingBlock].
//
// Use the [ContentBlockUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ContentBlockUnion struct {
	// This field is from variant [TextBlock].
	Citations []TextCitationUnion `json:"citations"`
	// This field is from variant [TextBlock].
	Text string `json:"text"`
	// Any of "text", "tool_use", "server_tool_use", "web_search_tool_result",
	// "thinking", "redacted_thinking".
	Type  string          `json:"type"`
	ID    string          `json:"id"`
	Input json.RawMessage `json:"input"`
	Name  string          `json:"name"`
	// This field is from variant [WebSearchToolResultBlock].
	Content WebSearchToolResultBlockContentUnion `json:"content"`
	// This field is from variant [WebSearchToolResultBlock].
	ToolUseID string `json:"tool_use_id"`
	// This field is from variant [ThinkingBlock].
	Signature string `json:"signature"`
	// This field is from variant [ThinkingBlock].
	Thinking string `json:"thinking"`
	// This field is from variant [RedactedThinkingBlock].
	Data string `json:"data"`
	JSON struct {
		Citations respjson.Field
		Text      respjson.Field
		Type      respjson.Field
		ID        respjson.Field
		Input     respjson.Field
		Name      respjson.Field
		Content   respjson.Field
		ToolUseID respjson.Field
		Signature respjson.Field
		Thinking  respjson.Field
		Data      respjson.Field
		raw       string
	} `json:"-"`
}

func (r ContentBlockUnion) ToParam() ContentBlockParamUnion {
	switch variant := r.AsAny().(type) {
	case TextBlock:
		p := variant.ToParam()
		return ContentBlockParamUnion{OfText: &p}
	case ToolUseBlock:
		p := variant.ToParam()
		return ContentBlockParamUnion{OfToolUse: &p}
	case ThinkingBlock:
		p := variant.ToParam()
		return ContentBlockParamUnion{OfThinking: &p}
	case RedactedThinkingBlock:
		p := variant.ToParam()
		return ContentBlockParamUnion{OfRedactedThinking: &p}
	}
	return ContentBlockParamUnion{}
}

// anyContentBlock is implemented by each variant of [ContentBlockUnion] to add
// type safety for the return type of [ContentBlockUnion.AsAny]
type anyContentBlock interface {
	implContentBlockUnion()
}

func (TextBlock) implContentBlockUnion()                {}
func (ToolUseBlock) implContentBlockUnion()             {}
func (ServerToolUseBlock) implContentBlockUnion()       {}
func (WebSearchToolResultBlock) implContentBlockUnion() {}
func (ThinkingBlock) implContentBlockUnion()            {}
func (RedactedThinkingBlock) implContentBlockUnion()    {}

// Use the following switch statement to find the correct variant
//
//	switch variant := ContentBlockUnion.AsAny().(type) {
//	case anthropic.TextBlock:
//	case anthropic.ToolUseBlock:
//	case anthropic.ServerToolUseBlock:
//	case anthropic.WebSearchToolResultBlock:
//	case anthropic.ThinkingBlock:
//	case anthropic.RedactedThinkingBlock:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ContentBlockUnion) AsAny() anyContentBlock {
	switch u.Type {
	case "text":
		return u.AsText()
	case "tool_use":
		return u.AsToolUse()
	case "server_tool_use":
		return u.AsServerToolUse()
	case "web_search_tool_result":
		return u.AsWebSearchToolResult()
	case "thinking":
		return u.AsThinking()
	case "redacted_thinking":
		return u.AsRedactedThinking()
	}
	return nil
}

func (u ContentBlockUnion) AsText() (v TextBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ContentBlockUnion) AsToolUse() (v ToolUseBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ContentBlockUnion) AsServerToolUse() (v ServerToolUseBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ContentBlockUnion) AsWebSearchToolResult() (v WebSearchToolResultBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ContentBlockUnion) AsThinking() (v ThinkingBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ContentBlockUnion) AsRedactedThinking() (v RedactedThinkingBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ContentBlockUnion) RawJSON() string { return u.JSON.raw }

func (r *ContentBlockUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func NewServerToolUseBlock(id string, input any) ContentBlockParamUnion {
	var serverToolUse ServerToolUseBlockParam
	serverToolUse.ID = id
	serverToolUse.Input = input
	return ContentBlockParamUnion{OfServerToolUse: &serverToolUse}
}

func NewWebSearchToolResultBlock[
	T []WebSearchResultBlockParam | WebSearchToolRequestErrorParam,
](content T, toolUseID string) ContentBlockParamUnion {
	var webSearchToolResult WebSearchToolResultBlockParam
	switch v := any(content).(type) {
	case []WebSearchResultBlockParam:
		webSearchToolResult.Content.OfWebSearchToolResultBlockItem = v
	case WebSearchToolRequestErrorParam:
		webSearchToolResult.Content.OfRequestWebSearchToolResultError = &v
	}
	webSearchToolResult.ToolUseID = toolUseID
	return ContentBlockParamUnion{OfWebSearchToolResult: &webSearchToolResult}
}

func NewTextBlock(text string) ContentBlockParamUnion {
	var variant TextBlockParam
	variant.Text = text
	return ContentBlockParamUnion{OfText: &variant}
}

func NewImageBlock[T Base64ImageSourceParam | URLImageSourceParam](source T) ContentBlockParamUnion {
	var image ImageBlockParam
	switch v := any(source).(type) {
	case Base64ImageSourceParam:
		image.Source.OfBase64 = &v
	case URLImageSourceParam:
		image.Source.OfURL = &v
	}
	return ContentBlockParamUnion{OfImage: &image}
}

func NewImageBlockBase64(mediaType string, encodedData string) ContentBlockParamUnion {
	return ContentBlockParamUnion{
		OfImage: &ImageBlockParam{
			Source: ImageBlockParamSourceUnion{
				OfBase64: &Base64ImageSourceParam{
					Data:      encodedData,
					MediaType: Base64ImageSourceMediaType(mediaType),
				},
			},
		},
	}
}

func NewToolUseBlock(id string, input any, name string) ContentBlockParamUnion {
	var toolUse ToolUseBlockParam
	toolUse.ID = id
	toolUse.Input = input
	toolUse.Name = name
	return ContentBlockParamUnion{OfToolUse: &toolUse}
}

func NewToolResultBlock(toolUseID string, content string, isError bool) ContentBlockParamUnion {
	toolBlock := ToolResultBlockParam{
		ToolUseID: toolUseID,
		Content: []ToolResultBlockParamContentUnion{
			{OfText: &TextBlockParam{Text: content}},
		},
		IsError: Bool(isError),
	}
	return ContentBlockParamUnion{OfToolResult: &toolBlock}
}

func NewDocumentBlock[
	T Base64PDFSourceParam | PlainTextSourceParam | ContentBlockSourceParam | URLPDFSourceParam,
](source T) ContentBlockParamUnion {
	var document DocumentBlockParam
	switch v := any(source).(type) {
	case Base64PDFSourceParam:
		document.Source.OfBase64 = &v
	case PlainTextSourceParam:
		document.Source.OfText = &v
	case ContentBlockSourceParam:
		document.Source.OfContent = &v
	case URLPDFSourceParam:
		document.Source.OfURL = &v
	}
	return ContentBlockParamUnion{OfDocument: &document}
}

func NewThinkingBlock(signature string, thinking string) ContentBlockParamUnion {
	var variant ThinkingBlockParam
	variant.Signature = signature
	variant.Thinking = thinking
	return ContentBlockParamUnion{OfThinking: &variant}
}

func NewRedactedThinkingBlock(data string) ContentBlockParamUnion {
	var redactedThinking RedactedThinkingBlockParam
	redactedThinking.Data = data
	return ContentBlockParamUnion{OfRedactedThinking: &redactedThinking}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ContentBlockParamUnion struct {
	OfServerToolUse       *ServerToolUseBlockParam       `json:",omitzero,inline"`
	OfWebSearchToolResult *WebSearchToolResultBlockParam `json:",omitzero,inline"`
	OfText                *TextBlockParam                `json:",omitzero,inline"`
	OfImage               *ImageBlockParam               `json:",omitzero,inline"`
	OfToolUse             *ToolUseBlockParam             `json:",omitzero,inline"`
	OfToolResult          *ToolResultBlockParam          `json:",omitzero,inline"`
	OfDocument            *DocumentBlockParam            `json:",omitzero,inline"`
	OfThinking            *ThinkingBlockParam            `json:",omitzero,inline"`
	OfRedactedThinking    *RedactedThinkingBlockParam    `json:",omitzero,inline"`
	paramUnion
}

func (u ContentBlockParamUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfServerToolUse,
		u.OfWebSearchToolResult,
		u.OfText,
		u.OfImage,
		u.OfToolUse,
		u.OfToolResult,
		u.OfDocument,
		u.OfThinking,
		u.OfRedactedThinking)
}
func (u *ContentBlockParamUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ContentBlockParamUnion) asAny() any {
	if !param.IsOmitted(u.OfServerToolUse) {
		return u.OfServerToolUse
	} else if !param.IsOmitted(u.OfWebSearchToolResult) {
		return u.OfWebSearchToolResult
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
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ContentBlockParamUnion) GetText() *string {
	if vt := u.OfText; vt != nil {
		return &vt.Text
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ContentBlockParamUnion) GetIsError() *bool {
	if vt := u.OfToolResult; vt != nil && vt.IsError.Valid() {
		return &vt.IsError.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ContentBlockParamUnion) GetContext() *string {
	if vt := u.OfDocument; vt != nil && vt.Context.Valid() {
		return &vt.Context.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ContentBlockParamUnion) GetTitle() *string {
	if vt := u.OfDocument; vt != nil && vt.Title.Valid() {
		return &vt.Title.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ContentBlockParamUnion) GetSignature() *string {
	if vt := u.OfThinking; vt != nil {
		return &vt.Signature
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ContentBlockParamUnion) GetThinking() *string {
	if vt := u.OfThinking; vt != nil {
		return &vt.Thinking
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ContentBlockParamUnion) GetData() *string {
	if vt := u.OfRedactedThinking; vt != nil {
		return &vt.Data
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ContentBlockParamUnion) GetID() *string {
	if vt := u.OfServerToolUse; vt != nil {
		return (*string)(&vt.ID)
	} else if vt := u.OfToolUse; vt != nil {
		return (*string)(&vt.ID)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ContentBlockParamUnion) GetName() *string {
	if vt := u.OfServerToolUse; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfToolUse; vt != nil {
		return (*string)(&vt.Name)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ContentBlockParamUnion) GetType() *string {
	if vt := u.OfServerToolUse; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfWebSearchToolResult; vt != nil {
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
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ContentBlockParamUnion) GetToolUseID() *string {
	if vt := u.OfWebSearchToolResult; vt != nil {
		return (*string)(&vt.ToolUseID)
	} else if vt := u.OfToolResult; vt != nil {
		return (*string)(&vt.ToolUseID)
	}
	return nil
}

// Returns a pointer to the underlying variant's Input property, if present.
func (u ContentBlockParamUnion) GetInput() *any {
	if vt := u.OfServerToolUse; vt != nil {
		return &vt.Input
	} else if vt := u.OfToolUse; vt != nil {
		return &vt.Input
	}
	return nil
}

// Returns a pointer to the underlying variant's CacheControl property, if present.
func (u ContentBlockParamUnion) GetCacheControl() *CacheControlEphemeralParam {
	if vt := u.OfServerToolUse; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfWebSearchToolResult; vt != nil {
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
	}
	return nil
}

// Returns a subunion which exports methods to access subproperties
//
// Or use AsAny() to get the underlying value
func (u ContentBlockParamUnion) GetContent() (res contentBlockParamUnionContent) {
	if vt := u.OfWebSearchToolResult; vt != nil {
		res.any = vt.Content.asAny()
	} else if vt := u.OfToolResult; vt != nil {
		res.any = &vt.Content
	}
	return
}

// Can have the runtime types [_[]WebSearchResultBlockParam],
// [_[]ToolResultBlockParamContentUnion]
type contentBlockParamUnionContent struct{ any }

// Use the following switch statement to get the type of the union:
//
//	switch u.AsAny().(type) {
//	case *[]anthropic.WebSearchResultBlockParam:
//	case *[]anthropic.ToolResultBlockParamContentUnion:
//	default:
//	    fmt.Errorf("not present")
//	}
func (u contentBlockParamUnionContent) AsAny() any { return u.any }

// Returns a subunion which exports methods to access subproperties
//
// Or use AsAny() to get the underlying value
func (u ContentBlockParamUnion) GetCitations() (res contentBlockParamUnionCitations) {
	if vt := u.OfText; vt != nil {
		res.any = &vt.Citations
	} else if vt := u.OfDocument; vt != nil {
		res.any = &vt.Citations
	}
	return
}

// Can have the runtime types [*[]TextCitationParamUnion], [*CitationsConfigParam]
type contentBlockParamUnionCitations struct{ any }

// Use the following switch statement to get the type of the union:
//
//	switch u.AsAny().(type) {
//	case *[]anthropic.TextCitationParamUnion:
//	case *anthropic.CitationsConfigParam:
//	default:
//	    fmt.Errorf("not present")
//	}
func (u contentBlockParamUnionCitations) AsAny() any { return u.any }

// Returns a subunion which exports methods to access subproperties
//
// Or use AsAny() to get the underlying value
func (u ContentBlockParamUnion) GetSource() (res contentBlockParamUnionSource) {
	if vt := u.OfImage; vt != nil {
		res.any = vt.Source.asAny()
	} else if vt := u.OfDocument; vt != nil {
		res.any = vt.Source.asAny()
	}
	return
}

// Can have the runtime types [*Base64ImageSourceParam], [*URLImageSourceParam],
// [*Base64PDFSourceParam], [*PlainTextSourceParam], [*ContentBlockSourceParam],
// [*URLPDFSourceParam]
type contentBlockParamUnionSource struct{ any }

// Use the following switch statement to get the type of the union:
//
//	switch u.AsAny().(type) {
//	case *anthropic.Base64ImageSourceParam:
//	case *anthropic.URLImageSourceParam:
//	case *anthropic.Base64PDFSourceParam:
//	case *anthropic.PlainTextSourceParam:
//	case *anthropic.ContentBlockSourceParam:
//	case *anthropic.URLPDFSourceParam:
//	default:
//	    fmt.Errorf("not present")
//	}
func (u contentBlockParamUnionSource) AsAny() any { return u.any }

// Returns a pointer to the underlying variant's property, if present.
func (u contentBlockParamUnionSource) GetContent() *ContentBlockSourceContentUnionParam {
	switch vt := u.any.(type) {
	case *DocumentBlockParamSourceUnion:
		return vt.GetContent()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u contentBlockParamUnionSource) GetData() *string {
	switch vt := u.any.(type) {
	case *ImageBlockParamSourceUnion:
		return vt.GetData()
	case *DocumentBlockParamSourceUnion:
		return vt.GetData()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u contentBlockParamUnionSource) GetMediaType() *string {
	switch vt := u.any.(type) {
	case *ImageBlockParamSourceUnion:
		return vt.GetMediaType()
	case *DocumentBlockParamSourceUnion:
		return vt.GetMediaType()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u contentBlockParamUnionSource) GetType() *string {
	switch vt := u.any.(type) {
	case *ImageBlockParamSourceUnion:
		return vt.GetType()
	case *DocumentBlockParamSourceUnion:
		return vt.GetType()
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u contentBlockParamUnionSource) GetURL() *string {
	switch vt := u.any.(type) {
	case *ImageBlockParamSourceUnion:
		return vt.GetURL()
	case *DocumentBlockParamSourceUnion:
		return vt.GetURL()
	}
	return nil
}

func init() {
	apijson.RegisterUnion[ContentBlockParamUnion](
		"type",
		apijson.Discriminator[ServerToolUseBlockParam]("server_tool_use"),
		apijson.Discriminator[WebSearchToolResultBlockParam]("web_search_tool_result"),
		apijson.Discriminator[TextBlockParam]("text"),
		apijson.Discriminator[ImageBlockParam]("image"),
		apijson.Discriminator[ToolUseBlockParam]("tool_use"),
		apijson.Discriminator[ToolResultBlockParam]("tool_result"),
		apijson.Discriminator[DocumentBlockParam]("document"),
		apijson.Discriminator[ThinkingBlockParam]("thinking"),
		apijson.Discriminator[RedactedThinkingBlockParam]("redacted_thinking"),
	)
}

func init() {
	apijson.RegisterUnion[DocumentBlockParamSourceUnion](
		"type",
		apijson.Discriminator[Base64PDFSourceParam]("base64"),
		apijson.Discriminator[PlainTextSourceParam]("text"),
		apijson.Discriminator[ContentBlockSourceParam]("content"),
		apijson.Discriminator[URLPDFSourceParam]("url"),
	)
}

func init() {
	apijson.RegisterUnion[ImageBlockParamSourceUnion](
		"type",
		apijson.Discriminator[Base64ImageSourceParam]("base64"),
		apijson.Discriminator[URLImageSourceParam]("url"),
	)
}

func init() {
	apijson.RegisterUnion[TextCitationParamUnion](
		"type",
		apijson.Discriminator[CitationCharLocationParam]("char_location"),
		apijson.Discriminator[CitationPageLocationParam]("page_location"),
		apijson.Discriminator[CitationContentBlockLocationParam]("content_block_location"),
		apijson.Discriminator[CitationWebSearchResultLocationParam]("web_search_result_location"),
	)
}

func init() {
	apijson.RegisterUnion[ThinkingConfigParamUnion](
		"type",
		apijson.Discriminator[ThinkingConfigEnabledParam]("enabled"),
		apijson.Discriminator[ThinkingConfigDisabledParam]("disabled"),
	)
}

func init() {
	apijson.RegisterUnion[ToolChoiceUnionParam](
		"type",
		apijson.Discriminator[ToolChoiceAutoParam]("auto"),
		apijson.Discriminator[ToolChoiceAnyParam]("any"),
		apijson.Discriminator[ToolChoiceToolParam]("tool"),
		apijson.Discriminator[ToolChoiceNoneParam]("none"),
	)
}

func init() {
	apijson.RegisterUnion[ToolResultBlockParamContentUnion](
		"type",
		apijson.Discriminator[TextBlockParam]("text"),
		apijson.Discriminator[ImageBlockParam]("image"),
	)
}

// The properties Content, Type are required.
type ContentBlockSourceParam struct {
	Content ContentBlockSourceContentUnionParam `json:"content,omitzero,required"`
	// This field can be elided, and will marshal its zero value as "content".
	Type constant.Content `json:"type,required"`
	paramObj
}

func (r ContentBlockSourceParam) MarshalJSON() (data []byte, err error) {
	type shadow ContentBlockSourceParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ContentBlockSourceParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ContentBlockSourceContentUnionParam struct {
	OfString                    param.Opt[string]                     `json:",omitzero,inline"`
	OfContentBlockSourceContent []ContentBlockSourceContentUnionParam `json:",omitzero,inline"`
	paramUnion
}

func (u ContentBlockSourceContentUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfContentBlockSourceContent)
}
func (u *ContentBlockSourceContentUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ContentBlockSourceContentUnionParam) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfContentBlockSourceContent) {
		return &u.OfContentBlockSourceContent
	}
	return nil
}

// The properties Source, Type are required.
type DocumentBlockParam struct {
	Source  DocumentBlockParamSourceUnion `json:"source,omitzero,required"`
	Context param.Opt[string]             `json:"context,omitzero"`
	Title   param.Opt[string]             `json:"title,omitzero"`
	// Create a cache control breakpoint at this content block.
	CacheControl CacheControlEphemeralParam `json:"cache_control,omitzero"`
	Citations    CitationsConfigParam       `json:"citations,omitzero"`
	// This field can be elided, and will marshal its zero value as "document".
	Type constant.Document `json:"type,required"`
	paramObj
}

func (r DocumentBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow DocumentBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *DocumentBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type DocumentBlockParamSourceUnion struct {
	OfBase64  *Base64PDFSourceParam    `json:",omitzero,inline"`
	OfText    *PlainTextSourceParam    `json:",omitzero,inline"`
	OfContent *ContentBlockSourceParam `json:",omitzero,inline"`
	OfURL     *URLPDFSourceParam       `json:",omitzero,inline"`
	paramUnion
}

func (u DocumentBlockParamSourceUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfBase64, u.OfText, u.OfContent, u.OfURL)
}
func (u *DocumentBlockParamSourceUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *DocumentBlockParamSourceUnion) asAny() any {
	if !param.IsOmitted(u.OfBase64) {
		return u.OfBase64
	} else if !param.IsOmitted(u.OfText) {
		return u.OfText
	} else if !param.IsOmitted(u.OfContent) {
		return u.OfContent
	} else if !param.IsOmitted(u.OfURL) {
		return u.OfURL
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u DocumentBlockParamSourceUnion) GetContent() *ContentBlockSourceContentUnionParam {
	if vt := u.OfContent; vt != nil {
		return &vt.Content
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u DocumentBlockParamSourceUnion) GetURL() *string {
	if vt := u.OfURL; vt != nil {
		return &vt.URL
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u DocumentBlockParamSourceUnion) GetData() *string {
	if vt := u.OfBase64; vt != nil {
		return (*string)(&vt.Data)
	} else if vt := u.OfText; vt != nil {
		return (*string)(&vt.Data)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u DocumentBlockParamSourceUnion) GetMediaType() *string {
	if vt := u.OfBase64; vt != nil {
		return (*string)(&vt.MediaType)
	} else if vt := u.OfText; vt != nil {
		return (*string)(&vt.MediaType)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u DocumentBlockParamSourceUnion) GetType() *string {
	if vt := u.OfBase64; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfText; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfContent; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfURL; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// The properties Source, Type are required.
type ImageBlockParam struct {
	Source ImageBlockParamSourceUnion `json:"source,omitzero,required"`
	// Create a cache control breakpoint at this content block.
	CacheControl CacheControlEphemeralParam `json:"cache_control,omitzero"`
	// This field can be elided, and will marshal its zero value as "image".
	Type constant.Image `json:"type,required"`
	paramObj
}

func (r ImageBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow ImageBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ImageBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ImageBlockParamSourceUnion struct {
	OfBase64 *Base64ImageSourceParam `json:",omitzero,inline"`
	OfURL    *URLImageSourceParam    `json:",omitzero,inline"`
	paramUnion
}

func (u ImageBlockParamSourceUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfBase64, u.OfURL)
}
func (u *ImageBlockParamSourceUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ImageBlockParamSourceUnion) asAny() any {
	if !param.IsOmitted(u.OfBase64) {
		return u.OfBase64
	} else if !param.IsOmitted(u.OfURL) {
		return u.OfURL
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ImageBlockParamSourceUnion) GetData() *string {
	if vt := u.OfBase64; vt != nil {
		return &vt.Data
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ImageBlockParamSourceUnion) GetMediaType() *string {
	if vt := u.OfBase64; vt != nil {
		return (*string)(&vt.MediaType)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ImageBlockParamSourceUnion) GetURL() *string {
	if vt := u.OfURL; vt != nil {
		return &vt.URL
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ImageBlockParamSourceUnion) GetType() *string {
	if vt := u.OfBase64; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfURL; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

type InputJSONDelta struct {
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
func (r InputJSONDelta) RawJSON() string { return r.JSON.raw }
func (r *InputJSONDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type Message struct {
	// Unique object identifier.
	//
	// The format and length of IDs may change over time.
	ID string `json:"id,required"`
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
	Content []ContentBlockUnion `json:"content,required"`
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
	StopReason StopReason `json:"stop_reason,required"`
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
	Usage Usage `json:"usage,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID           respjson.Field
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
func (r Message) RawJSON() string { return r.JSON.raw }
func (r *Message) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func (r Message) ToParam() MessageParam {
	var p MessageParam
	p.Role = MessageParamRole(r.Role)
	p.Content = make([]ContentBlockParamUnion, len(r.Content))
	for i, c := range r.Content {
		p.Content[i] = c.ToParam()
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
type MessageStopReason string

const (
	MessageStopReasonEndTurn      MessageStopReason = "end_turn"
	MessageStopReasonMaxTokens    MessageStopReason = "max_tokens"
	MessageStopReasonStopSequence MessageStopReason = "stop_sequence"
	MessageStopReasonToolUse      MessageStopReason = "tool_use"
)

func MessageCountTokensToolParamOfTool(inputSchema ToolInputSchemaParam, name string) MessageCountTokensToolUnionParam {
	var variant ToolParam
	variant.InputSchema = inputSchema
	variant.Name = name
	return MessageCountTokensToolUnionParam{OfTool: &variant}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type MessageCountTokensToolUnionParam struct {
	OfTool                  *ToolParam                                     `json:",omitzero,inline"`
	OfBashTool20250124      *ToolBash20250124Param                         `json:",omitzero,inline"`
	OfTextEditor20250124    *ToolTextEditor20250124Param                   `json:",omitzero,inline"`
	OfTextEditor20250429    *MessageCountTokensToolTextEditor20250429Param `json:",omitzero,inline"`
	OfWebSearchTool20250305 *WebSearchTool20250305Param                    `json:",omitzero,inline"`
	paramUnion
}

func (u MessageCountTokensToolUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfTool,
		u.OfBashTool20250124,
		u.OfTextEditor20250124,
		u.OfTextEditor20250429,
		u.OfWebSearchTool20250305)
}
func (u *MessageCountTokensToolUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *MessageCountTokensToolUnionParam) asAny() any {
	if !param.IsOmitted(u.OfTool) {
		return u.OfTool
	} else if !param.IsOmitted(u.OfBashTool20250124) {
		return u.OfBashTool20250124
	} else if !param.IsOmitted(u.OfTextEditor20250124) {
		return u.OfTextEditor20250124
	} else if !param.IsOmitted(u.OfTextEditor20250429) {
		return u.OfTextEditor20250429
	} else if !param.IsOmitted(u.OfWebSearchTool20250305) {
		return u.OfWebSearchTool20250305
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u MessageCountTokensToolUnionParam) GetInputSchema() *ToolInputSchemaParam {
	if vt := u.OfTool; vt != nil {
		return &vt.InputSchema
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u MessageCountTokensToolUnionParam) GetDescription() *string {
	if vt := u.OfTool; vt != nil && vt.Description.Valid() {
		return &vt.Description.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u MessageCountTokensToolUnionParam) GetAllowedDomains() []string {
	if vt := u.OfWebSearchTool20250305; vt != nil {
		return vt.AllowedDomains
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u MessageCountTokensToolUnionParam) GetBlockedDomains() []string {
	if vt := u.OfWebSearchTool20250305; vt != nil {
		return vt.BlockedDomains
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u MessageCountTokensToolUnionParam) GetMaxUses() *int64 {
	if vt := u.OfWebSearchTool20250305; vt != nil && vt.MaxUses.Valid() {
		return &vt.MaxUses.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u MessageCountTokensToolUnionParam) GetUserLocation() *WebSearchTool20250305UserLocationParam {
	if vt := u.OfWebSearchTool20250305; vt != nil {
		return &vt.UserLocation
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u MessageCountTokensToolUnionParam) GetName() *string {
	if vt := u.OfTool; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfBashTool20250124; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfTextEditor20250124; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfTextEditor20250429; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfWebSearchTool20250305; vt != nil {
		return (*string)(&vt.Name)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u MessageCountTokensToolUnionParam) GetType() *string {
	if vt := u.OfTool; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfBashTool20250124; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfTextEditor20250124; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfTextEditor20250429; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfWebSearchTool20250305; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Returns a pointer to the underlying variant's CacheControl property, if present.
func (u MessageCountTokensToolUnionParam) GetCacheControl() *CacheControlEphemeralParam {
	if vt := u.OfTool; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfBashTool20250124; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfTextEditor20250124; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfTextEditor20250429; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfWebSearchTool20250305; vt != nil {
		return &vt.CacheControl
	}
	return nil
}

// The properties Name, Type are required.
type MessageCountTokensToolTextEditor20250429Param struct {
	// Create a cache control breakpoint at this content block.
	CacheControl CacheControlEphemeralParam `json:"cache_control,omitzero"`
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

func (r MessageCountTokensToolTextEditor20250429Param) MarshalJSON() (data []byte, err error) {
	type shadow MessageCountTokensToolTextEditor20250429Param
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *MessageCountTokensToolTextEditor20250429Param) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type MessageDeltaUsage struct {
	// The cumulative number of input tokens used to create the cache entry.
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens,required"`
	// The cumulative number of input tokens read from the cache.
	CacheReadInputTokens int64 `json:"cache_read_input_tokens,required"`
	// The cumulative number of input tokens which were used.
	InputTokens int64 `json:"input_tokens,required"`
	// The cumulative number of output tokens which were used.
	OutputTokens int64 `json:"output_tokens,required"`
	// The number of server tool requests.
	ServerToolUse ServerToolUsage `json:"server_tool_use,required"`
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
func (r MessageDeltaUsage) RawJSON() string { return r.JSON.raw }
func (r *MessageDeltaUsage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Content, Role are required.
type MessageParam struct {
	Content []ContentBlockParamUnion `json:"content,omitzero,required"`
	// Any of "user", "assistant".
	Role MessageParamRole `json:"role,omitzero,required"`
	paramObj
}

func NewUserMessage(blocks ...ContentBlockParamUnion) MessageParam {
	return MessageParam{
		Role:    MessageParamRoleUser,
		Content: blocks,
	}
}

func NewAssistantMessage(blocks ...ContentBlockParamUnion) MessageParam {
	return MessageParam{
		Role:    MessageParamRoleAssistant,
		Content: blocks,
	}
}

func (r MessageParam) MarshalJSON() (data []byte, err error) {
	type shadow MessageParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *MessageParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type MessageParamRole string

const (
	MessageParamRoleUser      MessageParamRole = "user"
	MessageParamRoleAssistant MessageParamRole = "assistant"
)

type MessageTokensCount struct {
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
func (r MessageTokensCount) RawJSON() string { return r.JSON.raw }
func (r *MessageTokensCount) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type MetadataParam struct {
	// An external identifier for the user who is associated with the request.
	//
	// This should be a uuid, hash value, or other opaque identifier. Anthropic may use
	// this id to help detect abuse. Do not include any identifying information such as
	// name, email address, or phone number.
	UserID param.Opt[string] `json:"user_id,omitzero"`
	paramObj
}

func (r MetadataParam) MarshalJSON() (data []byte, err error) {
	type shadow MetadataParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *MetadataParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The model that will complete your prompt.\n\nSee
// [models](https://docs.anthropic.com/en/docs/models-overview) for additional
// details and options.
type Model string

const (
	ModelClaude3_7SonnetLatest      Model = "claude-3-7-sonnet-latest"
	ModelClaude3_7Sonnet20250219    Model = "claude-3-7-sonnet-20250219"
	ModelClaude3_5HaikuLatest       Model = "claude-3-5-haiku-latest"
	ModelClaude3_5Haiku20241022     Model = "claude-3-5-haiku-20241022"
	ModelClaudeSonnet4_20250514     Model = "claude-sonnet-4-20250514"
	ModelClaudeSonnet4_0            Model = "claude-sonnet-4-0"
	ModelClaude4Sonnet20250514      Model = "claude-4-sonnet-20250514"
	ModelClaude3_5SonnetLatest      Model = "claude-3-5-sonnet-latest"
	ModelClaude3_5Sonnet20241022    Model = "claude-3-5-sonnet-20241022"
	ModelClaude_3_5_Sonnet_20240620 Model = "claude-3-5-sonnet-20240620"
	ModelClaudeOpus4_0              Model = "claude-opus-4-0"
	ModelClaudeOpus4_20250514       Model = "claude-opus-4-20250514"
	ModelClaude4Opus20250514        Model = "claude-4-opus-20250514"
	ModelClaude3OpusLatest          Model = "claude-3-opus-latest"
	ModelClaude_3_Opus_20240229     Model = "claude-3-opus-20240229"
	// Deprecated: Will reach end-of-life on July 21st, 2025. Please migrate to a newer
	// model. Visit https://docs.anthropic.com/en/docs/resources/model-deprecations for
	// more information.
	ModelClaude_3_Sonnet_20240229 Model = "claude-3-sonnet-20240229"
	ModelClaude_3_Haiku_20240307  Model = "claude-3-haiku-20240307"
	// Deprecated: Will reach end-of-life on July 21st, 2025. Please migrate to a newer
	// model. Visit https://docs.anthropic.com/en/docs/resources/model-deprecations for
	// more information.
	ModelClaude_2_1 Model = "claude-2.1"
	// Deprecated: Will reach end-of-life on July 21st, 2025. Please migrate to a newer
	// model. Visit https://docs.anthropic.com/en/docs/resources/model-deprecations for
	// more information.
	ModelClaude_2_0 Model = "claude-2.0"
)

// The properties Data, MediaType, Type are required.
type PlainTextSourceParam struct {
	Data string `json:"data,required"`
	// This field can be elided, and will marshal its zero value as "text/plain".
	MediaType constant.TextPlain `json:"media_type,required"`
	// This field can be elided, and will marshal its zero value as "text".
	Type constant.Text `json:"type,required"`
	paramObj
}

func (r PlainTextSourceParam) MarshalJSON() (data []byte, err error) {
	type shadow PlainTextSourceParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *PlainTextSourceParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// RawContentBlockDeltaUnion contains all possible properties and values from
// [TextDelta], [InputJSONDelta], [CitationsDelta], [ThinkingDelta],
// [SignatureDelta].
//
// Use the [RawContentBlockDeltaUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type RawContentBlockDeltaUnion struct {
	// This field is from variant [TextDelta].
	Text string `json:"text"`
	// Any of "text_delta", "input_json_delta", "citations_delta", "thinking_delta",
	// "signature_delta".
	Type string `json:"type"`
	// This field is from variant [InputJSONDelta].
	PartialJSON string `json:"partial_json"`
	// This field is from variant [CitationsDelta].
	Citation CitationsDeltaCitationUnion `json:"citation"`
	// This field is from variant [ThinkingDelta].
	Thinking string `json:"thinking"`
	// This field is from variant [SignatureDelta].
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

// anyRawContentBlockDelta is implemented by each variant of
// [RawContentBlockDeltaUnion] to add type safety for the return type of
// [RawContentBlockDeltaUnion.AsAny]
type anyRawContentBlockDelta interface {
	implRawContentBlockDeltaUnion()
}

func (TextDelta) implRawContentBlockDeltaUnion()      {}
func (InputJSONDelta) implRawContentBlockDeltaUnion() {}
func (CitationsDelta) implRawContentBlockDeltaUnion() {}
func (ThinkingDelta) implRawContentBlockDeltaUnion()  {}
func (SignatureDelta) implRawContentBlockDeltaUnion() {}

// Use the following switch statement to find the correct variant
//
//	switch variant := RawContentBlockDeltaUnion.AsAny().(type) {
//	case anthropic.TextDelta:
//	case anthropic.InputJSONDelta:
//	case anthropic.CitationsDelta:
//	case anthropic.ThinkingDelta:
//	case anthropic.SignatureDelta:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u RawContentBlockDeltaUnion) AsAny() anyRawContentBlockDelta {
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

func (u RawContentBlockDeltaUnion) AsTextDelta() (v TextDelta) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u RawContentBlockDeltaUnion) AsInputJSONDelta() (v InputJSONDelta) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u RawContentBlockDeltaUnion) AsCitationsDelta() (v CitationsDelta) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u RawContentBlockDeltaUnion) AsThinkingDelta() (v ThinkingDelta) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u RawContentBlockDeltaUnion) AsSignatureDelta() (v SignatureDelta) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u RawContentBlockDeltaUnion) RawJSON() string { return u.JSON.raw }

func (r *RawContentBlockDeltaUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ContentBlockDeltaEvent struct {
	Delta RawContentBlockDeltaUnion  `json:"delta,required"`
	Index int64                      `json:"index,required"`
	Type  constant.ContentBlockDelta `json:"type,required"`
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
func (r ContentBlockDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *ContentBlockDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ContentBlockStartEvent struct {
	ContentBlock ContentBlockStartEventContentBlockUnion `json:"content_block,required"`
	Index        int64                                   `json:"index,required"`
	Type         constant.ContentBlockStart              `json:"type,required"`
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
func (r ContentBlockStartEvent) RawJSON() string { return r.JSON.raw }
func (r *ContentBlockStartEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ContentBlockStartEventContentBlockUnion contains all possible properties and
// values from [TextBlock], [ToolUseBlock], [ServerToolUseBlock],
// [WebSearchToolResultBlock], [ThinkingBlock], [RedactedThinkingBlock].
//
// Use the [ContentBlockStartEventContentBlockUnion.AsAny] method to switch on the
// variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ContentBlockStartEventContentBlockUnion struct {
	// This field is from variant [TextBlock].
	Citations []TextCitationUnion `json:"citations"`
	// This field is from variant [TextBlock].
	Text string `json:"text"`
	// Any of "text", "tool_use", "server_tool_use", "web_search_tool_result",
	// "thinking", "redacted_thinking".
	Type  string `json:"type"`
	ID    string `json:"id"`
	Input any    `json:"input"`
	Name  string `json:"name"`
	// This field is from variant [WebSearchToolResultBlock].
	Content WebSearchToolResultBlockContentUnion `json:"content"`
	// This field is from variant [WebSearchToolResultBlock].
	ToolUseID string `json:"tool_use_id"`
	// This field is from variant [ThinkingBlock].
	Signature string `json:"signature"`
	// This field is from variant [ThinkingBlock].
	Thinking string `json:"thinking"`
	// This field is from variant [RedactedThinkingBlock].
	Data string `json:"data"`
	JSON struct {
		Citations respjson.Field
		Text      respjson.Field
		Type      respjson.Field
		ID        respjson.Field
		Input     respjson.Field
		Name      respjson.Field
		Content   respjson.Field
		ToolUseID respjson.Field
		Signature respjson.Field
		Thinking  respjson.Field
		Data      respjson.Field
		raw       string
	} `json:"-"`
}

// anyContentBlockStartEventContentBlock is implemented by each variant of
// [ContentBlockStartEventContentBlockUnion] to add type safety for the return type
// of [ContentBlockStartEventContentBlockUnion.AsAny]
type anyContentBlockStartEventContentBlock interface {
	implContentBlockStartEventContentBlockUnion()
}

func (TextBlock) implContentBlockStartEventContentBlockUnion()                {}
func (ToolUseBlock) implContentBlockStartEventContentBlockUnion()             {}
func (ServerToolUseBlock) implContentBlockStartEventContentBlockUnion()       {}
func (WebSearchToolResultBlock) implContentBlockStartEventContentBlockUnion() {}
func (ThinkingBlock) implContentBlockStartEventContentBlockUnion()            {}
func (RedactedThinkingBlock) implContentBlockStartEventContentBlockUnion()    {}

// Use the following switch statement to find the correct variant
//
//	switch variant := ContentBlockStartEventContentBlockUnion.AsAny().(type) {
//	case anthropic.TextBlock:
//	case anthropic.ToolUseBlock:
//	case anthropic.ServerToolUseBlock:
//	case anthropic.WebSearchToolResultBlock:
//	case anthropic.ThinkingBlock:
//	case anthropic.RedactedThinkingBlock:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ContentBlockStartEventContentBlockUnion) AsAny() anyContentBlockStartEventContentBlock {
	switch u.Type {
	case "text":
		return u.AsText()
	case "tool_use":
		return u.AsToolUse()
	case "server_tool_use":
		return u.AsServerToolUse()
	case "web_search_tool_result":
		return u.AsWebSearchToolResult()
	case "thinking":
		return u.AsThinking()
	case "redacted_thinking":
		return u.AsRedactedThinking()
	}
	return nil
}

func (u ContentBlockStartEventContentBlockUnion) AsText() (v TextBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ContentBlockStartEventContentBlockUnion) AsToolUse() (v ToolUseBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ContentBlockStartEventContentBlockUnion) AsServerToolUse() (v ServerToolUseBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ContentBlockStartEventContentBlockUnion) AsWebSearchToolResult() (v WebSearchToolResultBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ContentBlockStartEventContentBlockUnion) AsThinking() (v ThinkingBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ContentBlockStartEventContentBlockUnion) AsRedactedThinking() (v RedactedThinkingBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ContentBlockStartEventContentBlockUnion) RawJSON() string { return u.JSON.raw }

func (r *ContentBlockStartEventContentBlockUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ContentBlockStopEvent struct {
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
func (r ContentBlockStopEvent) RawJSON() string { return r.JSON.raw }
func (r *ContentBlockStopEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type MessageDeltaEvent struct {
	Delta MessageDeltaEventDelta `json:"delta,required"`
	Type  constant.MessageDelta  `json:"type,required"`
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
	Usage MessageDeltaUsage `json:"usage,required"`
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
func (r MessageDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *MessageDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type MessageDeltaEventDelta struct {
	// Any of "end_turn", "max_tokens", "stop_sequence", "tool_use", "pause_turn",
	// "refusal".
	StopReason   StopReason `json:"stop_reason,required"`
	StopSequence string     `json:"stop_sequence,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		StopReason   respjson.Field
		StopSequence respjson.Field
		ExtraFields  map[string]respjson.Field
		raw          string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r MessageDeltaEventDelta) RawJSON() string { return r.JSON.raw }
func (r *MessageDeltaEventDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type MessageStartEvent struct {
	Message Message               `json:"message,required"`
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
func (r MessageStartEvent) RawJSON() string { return r.JSON.raw }
func (r *MessageStartEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type MessageStopEvent struct {
	Type constant.MessageStop `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r MessageStopEvent) RawJSON() string { return r.JSON.raw }
func (r *MessageStopEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// MessageStreamEventUnion contains all possible properties and values from
// [MessageStartEvent], [MessageDeltaEvent], [MessageStopEvent],
// [ContentBlockStartEvent], [ContentBlockDeltaEvent], [ContentBlockStopEvent].
//
// Use the [MessageStreamEventUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type MessageStreamEventUnion struct {
	// This field is from variant [MessageStartEvent].
	Message Message `json:"message"`
	// Any of "message_start", "message_delta", "message_stop", "content_block_start",
	// "content_block_delta", "content_block_stop".
	Type string `json:"type"`
	// This field is a union of [MessageDeltaEventDelta], [RawContentBlockDeltaUnion]
	Delta MessageStreamEventUnionDelta `json:"delta"`
	// This field is from variant [MessageDeltaEvent].
	Usage MessageDeltaUsage `json:"usage"`
	// This field is from variant [ContentBlockStartEvent].
	ContentBlock ContentBlockStartEventContentBlockUnion `json:"content_block"`
	Index        int64                                   `json:"index"`
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

// anyMessageStreamEvent is implemented by each variant of
// [MessageStreamEventUnion] to add type safety for the return type of
// [MessageStreamEventUnion.AsAny]
type anyMessageStreamEvent interface {
	implMessageStreamEventUnion()
}

func (MessageStartEvent) implMessageStreamEventUnion()      {}
func (MessageDeltaEvent) implMessageStreamEventUnion()      {}
func (MessageStopEvent) implMessageStreamEventUnion()       {}
func (ContentBlockStartEvent) implMessageStreamEventUnion() {}
func (ContentBlockDeltaEvent) implMessageStreamEventUnion() {}
func (ContentBlockStopEvent) implMessageStreamEventUnion()  {}

// Use the following switch statement to find the correct variant
//
//	switch variant := MessageStreamEventUnion.AsAny().(type) {
//	case anthropic.MessageStartEvent:
//	case anthropic.MessageDeltaEvent:
//	case anthropic.MessageStopEvent:
//	case anthropic.ContentBlockStartEvent:
//	case anthropic.ContentBlockDeltaEvent:
//	case anthropic.ContentBlockStopEvent:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u MessageStreamEventUnion) AsAny() anyMessageStreamEvent {
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

func (u MessageStreamEventUnion) AsMessageStart() (v MessageStartEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u MessageStreamEventUnion) AsMessageDelta() (v MessageDeltaEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u MessageStreamEventUnion) AsMessageStop() (v MessageStopEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u MessageStreamEventUnion) AsContentBlockStart() (v ContentBlockStartEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u MessageStreamEventUnion) AsContentBlockDelta() (v ContentBlockDeltaEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u MessageStreamEventUnion) AsContentBlockStop() (v ContentBlockStopEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u MessageStreamEventUnion) RawJSON() string { return u.JSON.raw }

func (r *MessageStreamEventUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// MessageStreamEventUnionDelta is an implicit subunion of
// [MessageStreamEventUnion]. MessageStreamEventUnionDelta provides convenient
// access to the sub-properties of the union.
//
// For type safety it is recommended to directly use a variant of the
// [MessageStreamEventUnion].
type MessageStreamEventUnionDelta struct {
	// This field is from variant [MessageDeltaEventDelta].
	StopReason StopReason `json:"stop_reason"`
	// This field is from variant [MessageDeltaEventDelta].
	StopSequence string `json:"stop_sequence"`
	// This field is from variant [RawContentBlockDeltaUnion].
	Text string `json:"text"`
	Type string `json:"type"`
	// This field is from variant [RawContentBlockDeltaUnion].
	PartialJSON string `json:"partial_json"`
	// This field is from variant [RawContentBlockDeltaUnion].
	Citation CitationsDeltaCitationUnion `json:"citation"`
	// This field is from variant [RawContentBlockDeltaUnion].
	Thinking string `json:"thinking"`
	// This field is from variant [RawContentBlockDeltaUnion].
	Signature string `json:"signature"`
	JSON      struct {
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

func (r *MessageStreamEventUnionDelta) UnmarshalJSON(data []byte) error {
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
func (acc *Message) Accumulate(event MessageStreamEventUnion) error {
	if acc == nil {
		return fmt.Errorf("accumulate: cannot accumlate into nil Message")
	}

	switch event := event.AsAny().(type) {
	case MessageStartEvent:
		*acc = event.Message
	case MessageDeltaEvent:
		acc.StopReason = event.Delta.StopReason
		acc.StopSequence = event.Delta.StopSequence
		acc.Usage.OutputTokens = event.Usage.OutputTokens
	case MessageStopEvent:
		accJson, err := json.Marshal(acc)
		if err != nil {
			return fmt.Errorf("error converting content block to JSON: %w", err)
		}
		acc.JSON.raw = string(accJson)
	case ContentBlockStartEvent:
		acc.Content = append(acc.Content, ContentBlockUnion{})
		err := acc.Content[len(acc.Content)-1].UnmarshalJSON([]byte(event.ContentBlock.RawJSON()))
		if err != nil {
			return err
		}
	case ContentBlockDeltaEvent:
		if len(acc.Content) == 0 {
			return fmt.Errorf("received event of type %s but there was no content block", event.Type)
		}
		cb := &acc.Content[len(acc.Content)-1]
		switch delta := event.Delta.AsAny().(type) {
		case TextDelta:
			cb.Text += delta.Text
		case InputJSONDelta:
			if len(delta.PartialJSON) != 0 {
				if string(cb.Input) == "{}" {
					cb.Input = []byte(delta.PartialJSON)
				} else {
					cb.Input = append(cb.Input, []byte(delta.PartialJSON)...)
				}
			}
		case ThinkingDelta:
			cb.Thinking += delta.Thinking
		case SignatureDelta:
			cb.Signature += delta.Signature
		case CitationsDelta:
			citation := TextCitationUnion{}
			err := citation.UnmarshalJSON([]byte(delta.Citation.RawJSON()))
			if err != nil {
				return fmt.Errorf("could not unmarshal citation delta into citation type: %w", err)
			}
			cb.Citations = append(cb.Citations, citation)
		}
	case ContentBlockStopEvent:
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

type RedactedThinkingBlock struct {
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
func (r RedactedThinkingBlock) RawJSON() string { return r.JSON.raw }
func (r *RedactedThinkingBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func (r RedactedThinkingBlock) ToParam() RedactedThinkingBlockParam {
	var p RedactedThinkingBlockParam
	p.Type = r.Type
	p.Data = r.Data
	return p
}

// The properties Data, Type are required.
type RedactedThinkingBlockParam struct {
	Data string `json:"data,required"`
	// This field can be elided, and will marshal its zero value as
	// "redacted_thinking".
	Type constant.RedactedThinking `json:"type,required"`
	paramObj
}

func (r RedactedThinkingBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow RedactedThinkingBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *RedactedThinkingBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ServerToolUsage struct {
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
func (r ServerToolUsage) RawJSON() string { return r.JSON.raw }
func (r *ServerToolUsage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ServerToolUseBlock struct {
	ID    string                 `json:"id,required"`
	Input any                    `json:"input,required"`
	Name  constant.WebSearch     `json:"name,required"`
	Type  constant.ServerToolUse `json:"type,required"`
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
func (r ServerToolUseBlock) RawJSON() string { return r.JSON.raw }
func (r *ServerToolUseBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties ID, Input, Name, Type are required.
type ServerToolUseBlockParam struct {
	ID    string `json:"id,required"`
	Input any    `json:"input,omitzero,required"`
	// Create a cache control breakpoint at this content block.
	CacheControl CacheControlEphemeralParam `json:"cache_control,omitzero"`
	// This field can be elided, and will marshal its zero value as "web_search".
	Name constant.WebSearch `json:"name,required"`
	// This field can be elided, and will marshal its zero value as "server_tool_use".
	Type constant.ServerToolUse `json:"type,required"`
	paramObj
}

func (r ServerToolUseBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow ServerToolUseBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ServerToolUseBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type SignatureDelta struct {
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
func (r SignatureDelta) RawJSON() string { return r.JSON.raw }
func (r *SignatureDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type StopReason string

const (
	StopReasonEndTurn      StopReason = "end_turn"
	StopReasonMaxTokens    StopReason = "max_tokens"
	StopReasonStopSequence StopReason = "stop_sequence"
	StopReasonToolUse      StopReason = "tool_use"
	StopReasonPauseTurn    StopReason = "pause_turn"
	StopReasonRefusal      StopReason = "refusal"
)

type TextBlock struct {
	// Citations supporting the text block.
	//
	// The type of citation returned will depend on the type of document being cited.
	// Citing a PDF results in `page_location`, plain text results in `char_location`,
	// and content document results in `content_block_location`.
	Citations []TextCitationUnion `json:"citations,required"`
	Text      string              `json:"text,required"`
	Type      constant.Text       `json:"type,required"`
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
func (r TextBlock) RawJSON() string { return r.JSON.raw }
func (r *TextBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func (r TextBlock) ToParam() TextBlockParam {
	var p TextBlockParam
	p.Type = r.Type
	p.Text = r.Text

	// Distinguish between a nil and zero length slice, since some compatible
	// APIs may not require citations.
	if r.Citations != nil {
		p.Citations = make([]TextCitationParamUnion, len(r.Citations))
	}

	for i, citation := range r.Citations {
		switch citationVariant := citation.AsAny().(type) {
		case CitationCharLocation:
			var citationParam CitationCharLocationParam
			citationParam.Type = citationVariant.Type
			citationParam.DocumentTitle = paramutil.ToOpt(citationVariant.DocumentTitle, citationVariant.JSON.DocumentTitle)
			citationParam.CitedText = citationVariant.CitedText
			citationParam.DocumentIndex = citationVariant.DocumentIndex
			citationParam.EndCharIndex = citationVariant.EndCharIndex
			citationParam.StartCharIndex = citationVariant.StartCharIndex
			p.Citations[i] = TextCitationParamUnion{OfCharLocation: &citationParam}
		case CitationPageLocation:
			var citationParam CitationPageLocationParam
			citationParam.Type = citationVariant.Type
			citationParam.DocumentTitle = paramutil.ToOpt(citationVariant.DocumentTitle, citationVariant.JSON.DocumentTitle)
			citationParam.DocumentIndex = citationVariant.DocumentIndex
			citationParam.EndPageNumber = citationVariant.EndPageNumber
			citationParam.StartPageNumber = citationVariant.StartPageNumber
			p.Citations[i] = TextCitationParamUnion{OfPageLocation: &citationParam}
		case CitationContentBlockLocation:
			var citationParam CitationContentBlockLocationParam
			citationParam.Type = citationVariant.Type
			citationParam.DocumentTitle = paramutil.ToOpt(citationVariant.DocumentTitle, citationVariant.JSON.DocumentTitle)
			citationParam.CitedText = citationVariant.CitedText
			citationParam.DocumentIndex = citationVariant.DocumentIndex
			citationParam.EndBlockIndex = citationVariant.EndBlockIndex
			citationParam.StartBlockIndex = citationVariant.StartBlockIndex
			p.Citations[i] = TextCitationParamUnion{OfContentBlockLocation: &citationParam}
		}
	}
	return p
}

// The properties Text, Type are required.
type TextBlockParam struct {
	Text      string                   `json:"text,required"`
	Citations []TextCitationParamUnion `json:"citations,omitzero"`
	// Create a cache control breakpoint at this content block.
	CacheControl CacheControlEphemeralParam `json:"cache_control,omitzero"`
	// This field can be elided, and will marshal its zero value as "text".
	Type constant.Text `json:"type,required"`
	paramObj
}

func (r TextBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow TextBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *TextBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// TextCitationUnion contains all possible properties and values from
// [CitationCharLocation], [CitationPageLocation], [CitationContentBlockLocation],
// [CitationsWebSearchResultLocation].
//
// Use the [TextCitationUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type TextCitationUnion struct {
	CitedText     string `json:"cited_text"`
	DocumentIndex int64  `json:"document_index"`
	DocumentTitle string `json:"document_title"`
	// This field is from variant [CitationCharLocation].
	EndCharIndex int64 `json:"end_char_index"`
	// This field is from variant [CitationCharLocation].
	StartCharIndex int64 `json:"start_char_index"`
	// Any of "char_location", "page_location", "content_block_location",
	// "web_search_result_location".
	Type string `json:"type"`
	// This field is from variant [CitationPageLocation].
	EndPageNumber int64 `json:"end_page_number"`
	// This field is from variant [CitationPageLocation].
	StartPageNumber int64 `json:"start_page_number"`
	// This field is from variant [CitationContentBlockLocation].
	EndBlockIndex int64 `json:"end_block_index"`
	// This field is from variant [CitationContentBlockLocation].
	StartBlockIndex int64 `json:"start_block_index"`
	// This field is from variant [CitationsWebSearchResultLocation].
	EncryptedIndex string `json:"encrypted_index"`
	// This field is from variant [CitationsWebSearchResultLocation].
	Title string `json:"title"`
	// This field is from variant [CitationsWebSearchResultLocation].
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

// anyTextCitation is implemented by each variant of [TextCitationUnion] to add
// type safety for the return type of [TextCitationUnion.AsAny]
type anyTextCitation interface {
	implTextCitationUnion()
}

func (CitationCharLocation) implTextCitationUnion()             {}
func (CitationPageLocation) implTextCitationUnion()             {}
func (CitationContentBlockLocation) implTextCitationUnion()     {}
func (CitationsWebSearchResultLocation) implTextCitationUnion() {}

// Use the following switch statement to find the correct variant
//
//	switch variant := TextCitationUnion.AsAny().(type) {
//	case anthropic.CitationCharLocation:
//	case anthropic.CitationPageLocation:
//	case anthropic.CitationContentBlockLocation:
//	case anthropic.CitationsWebSearchResultLocation:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u TextCitationUnion) AsAny() anyTextCitation {
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

func (u TextCitationUnion) AsCharLocation() (v CitationCharLocation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u TextCitationUnion) AsPageLocation() (v CitationPageLocation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u TextCitationUnion) AsContentBlockLocation() (v CitationContentBlockLocation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u TextCitationUnion) AsWebSearchResultLocation() (v CitationsWebSearchResultLocation) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u TextCitationUnion) RawJSON() string { return u.JSON.raw }

func (r *TextCitationUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type TextCitationParamUnion struct {
	OfCharLocation            *CitationCharLocationParam            `json:",omitzero,inline"`
	OfPageLocation            *CitationPageLocationParam            `json:",omitzero,inline"`
	OfContentBlockLocation    *CitationContentBlockLocationParam    `json:",omitzero,inline"`
	OfWebSearchResultLocation *CitationWebSearchResultLocationParam `json:",omitzero,inline"`
	paramUnion
}

func (u TextCitationParamUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfCharLocation, u.OfPageLocation, u.OfContentBlockLocation, u.OfWebSearchResultLocation)
}
func (u *TextCitationParamUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *TextCitationParamUnion) asAny() any {
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
func (u TextCitationParamUnion) GetEndCharIndex() *int64 {
	if vt := u.OfCharLocation; vt != nil {
		return &vt.EndCharIndex
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u TextCitationParamUnion) GetStartCharIndex() *int64 {
	if vt := u.OfCharLocation; vt != nil {
		return &vt.StartCharIndex
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u TextCitationParamUnion) GetEndPageNumber() *int64 {
	if vt := u.OfPageLocation; vt != nil {
		return &vt.EndPageNumber
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u TextCitationParamUnion) GetStartPageNumber() *int64 {
	if vt := u.OfPageLocation; vt != nil {
		return &vt.StartPageNumber
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u TextCitationParamUnion) GetEndBlockIndex() *int64 {
	if vt := u.OfContentBlockLocation; vt != nil {
		return &vt.EndBlockIndex
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u TextCitationParamUnion) GetStartBlockIndex() *int64 {
	if vt := u.OfContentBlockLocation; vt != nil {
		return &vt.StartBlockIndex
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u TextCitationParamUnion) GetEncryptedIndex() *string {
	if vt := u.OfWebSearchResultLocation; vt != nil {
		return &vt.EncryptedIndex
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u TextCitationParamUnion) GetTitle() *string {
	if vt := u.OfWebSearchResultLocation; vt != nil && vt.Title.Valid() {
		return &vt.Title.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u TextCitationParamUnion) GetURL() *string {
	if vt := u.OfWebSearchResultLocation; vt != nil {
		return &vt.URL
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u TextCitationParamUnion) GetCitedText() *string {
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
func (u TextCitationParamUnion) GetDocumentIndex() *int64 {
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
func (u TextCitationParamUnion) GetDocumentTitle() *string {
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
func (u TextCitationParamUnion) GetType() *string {
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

type TextDelta struct {
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
func (r TextDelta) RawJSON() string { return r.JSON.raw }
func (r *TextDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ThinkingBlock struct {
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
func (r ThinkingBlock) RawJSON() string { return r.JSON.raw }
func (r *ThinkingBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func (r ThinkingBlock) ToParam() ThinkingBlockParam {
	var p ThinkingBlockParam
	p.Type = r.Type
	p.Signature = r.Signature
	p.Thinking = r.Thinking
	return p
}

// The properties Signature, Thinking, Type are required.
type ThinkingBlockParam struct {
	Signature string `json:"signature,required"`
	Thinking  string `json:"thinking,required"`
	// This field can be elided, and will marshal its zero value as "thinking".
	Type constant.Thinking `json:"type,required"`
	paramObj
}

func (r ThinkingBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow ThinkingBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ThinkingBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func NewThinkingConfigDisabledParam() ThinkingConfigDisabledParam {
	return ThinkingConfigDisabledParam{
		Type: "disabled",
	}
}

// This struct has a constant value, construct it with
// [NewThinkingConfigDisabledParam].
type ThinkingConfigDisabledParam struct {
	Type constant.Disabled `json:"type,required"`
	paramObj
}

func (r ThinkingConfigDisabledParam) MarshalJSON() (data []byte, err error) {
	type shadow ThinkingConfigDisabledParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ThinkingConfigDisabledParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties BudgetTokens, Type are required.
type ThinkingConfigEnabledParam struct {
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

func (r ThinkingConfigEnabledParam) MarshalJSON() (data []byte, err error) {
	type shadow ThinkingConfigEnabledParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ThinkingConfigEnabledParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func ThinkingConfigParamOfEnabled(budgetTokens int64) ThinkingConfigParamUnion {
	var enabled ThinkingConfigEnabledParam
	enabled.BudgetTokens = budgetTokens
	return ThinkingConfigParamUnion{OfEnabled: &enabled}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ThinkingConfigParamUnion struct {
	OfEnabled  *ThinkingConfigEnabledParam  `json:",omitzero,inline"`
	OfDisabled *ThinkingConfigDisabledParam `json:",omitzero,inline"`
	paramUnion
}

func (u ThinkingConfigParamUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfEnabled, u.OfDisabled)
}
func (u *ThinkingConfigParamUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ThinkingConfigParamUnion) asAny() any {
	if !param.IsOmitted(u.OfEnabled) {
		return u.OfEnabled
	} else if !param.IsOmitted(u.OfDisabled) {
		return u.OfDisabled
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ThinkingConfigParamUnion) GetBudgetTokens() *int64 {
	if vt := u.OfEnabled; vt != nil {
		return &vt.BudgetTokens
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ThinkingConfigParamUnion) GetType() *string {
	if vt := u.OfEnabled; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfDisabled; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

type ThinkingDelta struct {
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
func (r ThinkingDelta) RawJSON() string { return r.JSON.raw }
func (r *ThinkingDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties InputSchema, Name are required.
type ToolParam struct {
	// [JSON schema](https://json-schema.org/draft/2020-12) for this tool's input.
	//
	// This defines the shape of the `input` that your tool accepts and that the model
	// will produce.
	InputSchema ToolInputSchemaParam `json:"input_schema,omitzero,required"`
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
	Type ToolType `json:"type,omitzero"`
	// Create a cache control breakpoint at this content block.
	CacheControl CacheControlEphemeralParam `json:"cache_control,omitzero"`
	paramObj
}

func (r ToolParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// [JSON schema](https://json-schema.org/draft/2020-12) for this tool's input.
//
// This defines the shape of the `input` that your tool accepts and that the model
// will produce.
//
// The property Type is required.
type ToolInputSchemaParam struct {
	Properties any      `json:"properties,omitzero"`
	Required   []string `json:"required,omitzero"`
	// This field can be elided, and will marshal its zero value as "object".
	Type        constant.Object `json:"type,required"`
	ExtraFields map[string]any  `json:"-"`
	paramObj
}

func (r ToolInputSchemaParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolInputSchemaParam
	return param.MarshalWithExtras(r, (*shadow)(&r), r.ExtraFields)
}
func (r *ToolInputSchemaParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ToolType string

const (
	ToolTypeCustom ToolType = "custom"
)

// The properties Name, Type are required.
type ToolBash20250124Param struct {
	// Create a cache control breakpoint at this content block.
	CacheControl CacheControlEphemeralParam `json:"cache_control,omitzero"`
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

func (r ToolBash20250124Param) MarshalJSON() (data []byte, err error) {
	type shadow ToolBash20250124Param
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolBash20250124Param) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func ToolChoiceParamOfTool(name string) ToolChoiceUnionParam {
	var tool ToolChoiceToolParam
	tool.Name = name
	return ToolChoiceUnionParam{OfTool: &tool}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ToolChoiceUnionParam struct {
	OfAuto *ToolChoiceAutoParam `json:",omitzero,inline"`
	OfAny  *ToolChoiceAnyParam  `json:",omitzero,inline"`
	OfTool *ToolChoiceToolParam `json:",omitzero,inline"`
	OfNone *ToolChoiceNoneParam `json:",omitzero,inline"`
	paramUnion
}

func (u ToolChoiceUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfAuto, u.OfAny, u.OfTool, u.OfNone)
}
func (u *ToolChoiceUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ToolChoiceUnionParam) asAny() any {
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
func (u ToolChoiceUnionParam) GetName() *string {
	if vt := u.OfTool; vt != nil {
		return &vt.Name
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolChoiceUnionParam) GetType() *string {
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
func (u ToolChoiceUnionParam) GetDisableParallelToolUse() *bool {
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
type ToolChoiceAnyParam struct {
	// Whether to disable parallel tool use.
	//
	// Defaults to `false`. If set to `true`, the model will output exactly one tool
	// use.
	DisableParallelToolUse param.Opt[bool] `json:"disable_parallel_tool_use,omitzero"`
	// This field can be elided, and will marshal its zero value as "any".
	Type constant.Any `json:"type,required"`
	paramObj
}

func (r ToolChoiceAnyParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolChoiceAnyParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolChoiceAnyParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The model will automatically decide whether to use tools.
//
// The property Type is required.
type ToolChoiceAutoParam struct {
	// Whether to disable parallel tool use.
	//
	// Defaults to `false`. If set to `true`, the model will output at most one tool
	// use.
	DisableParallelToolUse param.Opt[bool] `json:"disable_parallel_tool_use,omitzero"`
	// This field can be elided, and will marshal its zero value as "auto".
	Type constant.Auto `json:"type,required"`
	paramObj
}

func (r ToolChoiceAutoParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolChoiceAutoParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolChoiceAutoParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func NewToolChoiceNoneParam() ToolChoiceNoneParam {
	return ToolChoiceNoneParam{
		Type: "none",
	}
}

// The model will not be allowed to use tools.
//
// This struct has a constant value, construct it with [NewToolChoiceNoneParam].
type ToolChoiceNoneParam struct {
	Type constant.None `json:"type,required"`
	paramObj
}

func (r ToolChoiceNoneParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolChoiceNoneParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolChoiceNoneParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The model will use the specified tool with `tool_choice.name`.
//
// The properties Name, Type are required.
type ToolChoiceToolParam struct {
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

func (r ToolChoiceToolParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolChoiceToolParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolChoiceToolParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties ToolUseID, Type are required.
type ToolResultBlockParam struct {
	ToolUseID string          `json:"tool_use_id,required"`
	IsError   param.Opt[bool] `json:"is_error,omitzero"`
	// Create a cache control breakpoint at this content block.
	CacheControl CacheControlEphemeralParam         `json:"cache_control,omitzero"`
	Content      []ToolResultBlockParamContentUnion `json:"content,omitzero"`
	// This field can be elided, and will marshal its zero value as "tool_result".
	Type constant.ToolResult `json:"type,required"`
	paramObj
}

func (r ToolResultBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolResultBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolResultBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ToolResultBlockParamContentUnion struct {
	OfText  *TextBlockParam  `json:",omitzero,inline"`
	OfImage *ImageBlockParam `json:",omitzero,inline"`
	paramUnion
}

func (u ToolResultBlockParamContentUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfText, u.OfImage)
}
func (u *ToolResultBlockParamContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ToolResultBlockParamContentUnion) asAny() any {
	if !param.IsOmitted(u.OfText) {
		return u.OfText
	} else if !param.IsOmitted(u.OfImage) {
		return u.OfImage
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolResultBlockParamContentUnion) GetText() *string {
	if vt := u.OfText; vt != nil {
		return &vt.Text
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolResultBlockParamContentUnion) GetCitations() []TextCitationParamUnion {
	if vt := u.OfText; vt != nil {
		return vt.Citations
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolResultBlockParamContentUnion) GetSource() *ImageBlockParamSourceUnion {
	if vt := u.OfImage; vt != nil {
		return &vt.Source
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolResultBlockParamContentUnion) GetType() *string {
	if vt := u.OfText; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfImage; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Returns a pointer to the underlying variant's CacheControl property, if present.
func (u ToolResultBlockParamContentUnion) GetCacheControl() *CacheControlEphemeralParam {
	if vt := u.OfText; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfImage; vt != nil {
		return &vt.CacheControl
	}
	return nil
}

// The properties Name, Type are required.
type ToolTextEditor20250124Param struct {
	// Create a cache control breakpoint at this content block.
	CacheControl CacheControlEphemeralParam `json:"cache_control,omitzero"`
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

func (r ToolTextEditor20250124Param) MarshalJSON() (data []byte, err error) {
	type shadow ToolTextEditor20250124Param
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolTextEditor20250124Param) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func ToolUnionParamOfTool(inputSchema ToolInputSchemaParam, name string) ToolUnionParam {
	var variant ToolParam
	variant.InputSchema = inputSchema
	variant.Name = name
	return ToolUnionParam{OfTool: &variant}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type ToolUnionParam struct {
	OfTool                  *ToolParam                        `json:",omitzero,inline"`
	OfBashTool20250124      *ToolBash20250124Param            `json:",omitzero,inline"`
	OfTextEditor20250124    *ToolTextEditor20250124Param      `json:",omitzero,inline"`
	OfTextEditor20250429    *ToolUnionTextEditor20250429Param `json:",omitzero,inline"`
	OfWebSearchTool20250305 *WebSearchTool20250305Param       `json:",omitzero,inline"`
	paramUnion
}

func (u ToolUnionParam) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfTool,
		u.OfBashTool20250124,
		u.OfTextEditor20250124,
		u.OfTextEditor20250429,
		u.OfWebSearchTool20250305)
}
func (u *ToolUnionParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *ToolUnionParam) asAny() any {
	if !param.IsOmitted(u.OfTool) {
		return u.OfTool
	} else if !param.IsOmitted(u.OfBashTool20250124) {
		return u.OfBashTool20250124
	} else if !param.IsOmitted(u.OfTextEditor20250124) {
		return u.OfTextEditor20250124
	} else if !param.IsOmitted(u.OfTextEditor20250429) {
		return u.OfTextEditor20250429
	} else if !param.IsOmitted(u.OfWebSearchTool20250305) {
		return u.OfWebSearchTool20250305
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetInputSchema() *ToolInputSchemaParam {
	if vt := u.OfTool; vt != nil {
		return &vt.InputSchema
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetDescription() *string {
	if vt := u.OfTool; vt != nil && vt.Description.Valid() {
		return &vt.Description.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetAllowedDomains() []string {
	if vt := u.OfWebSearchTool20250305; vt != nil {
		return vt.AllowedDomains
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetBlockedDomains() []string {
	if vt := u.OfWebSearchTool20250305; vt != nil {
		return vt.BlockedDomains
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetMaxUses() *int64 {
	if vt := u.OfWebSearchTool20250305; vt != nil && vt.MaxUses.Valid() {
		return &vt.MaxUses.Value
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetUserLocation() *WebSearchTool20250305UserLocationParam {
	if vt := u.OfWebSearchTool20250305; vt != nil {
		return &vt.UserLocation
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetName() *string {
	if vt := u.OfTool; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfBashTool20250124; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfTextEditor20250124; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfTextEditor20250429; vt != nil {
		return (*string)(&vt.Name)
	} else if vt := u.OfWebSearchTool20250305; vt != nil {
		return (*string)(&vt.Name)
	}
	return nil
}

// Returns a pointer to the underlying variant's property, if present.
func (u ToolUnionParam) GetType() *string {
	if vt := u.OfTool; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfBashTool20250124; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfTextEditor20250124; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfTextEditor20250429; vt != nil {
		return (*string)(&vt.Type)
	} else if vt := u.OfWebSearchTool20250305; vt != nil {
		return (*string)(&vt.Type)
	}
	return nil
}

// Returns a pointer to the underlying variant's CacheControl property, if present.
func (u ToolUnionParam) GetCacheControl() *CacheControlEphemeralParam {
	if vt := u.OfTool; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfBashTool20250124; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfTextEditor20250124; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfTextEditor20250429; vt != nil {
		return &vt.CacheControl
	} else if vt := u.OfWebSearchTool20250305; vt != nil {
		return &vt.CacheControl
	}
	return nil
}

// The properties Name, Type are required.
type ToolUnionTextEditor20250429Param struct {
	// Create a cache control breakpoint at this content block.
	CacheControl CacheControlEphemeralParam `json:"cache_control,omitzero"`
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

func (r ToolUnionTextEditor20250429Param) MarshalJSON() (data []byte, err error) {
	type shadow ToolUnionTextEditor20250429Param
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolUnionTextEditor20250429Param) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type ToolUseBlock struct {
	ID    string           `json:"id,required"`
	Input json.RawMessage  `json:"input,required"`
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
func (r ToolUseBlock) RawJSON() string { return r.JSON.raw }
func (r *ToolUseBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func (r ToolUseBlock) ToParam() ToolUseBlockParam {
	var toolUse ToolUseBlockParam
	toolUse.Type = r.Type
	toolUse.ID = r.ID
	toolUse.Input = r.Input
	toolUse.Name = r.Name
	return toolUse
}

// The properties ID, Input, Name, Type are required.
type ToolUseBlockParam struct {
	ID    string `json:"id,required"`
	Input any    `json:"input,omitzero,required"`
	Name  string `json:"name,required"`
	// Create a cache control breakpoint at this content block.
	CacheControl CacheControlEphemeralParam `json:"cache_control,omitzero"`
	// This field can be elided, and will marshal its zero value as "tool_use".
	Type constant.ToolUse `json:"type,required"`
	paramObj
}

func (r ToolUseBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow ToolUseBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *ToolUseBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Type, URL are required.
type URLImageSourceParam struct {
	URL string `json:"url,required"`
	// This field can be elided, and will marshal its zero value as "url".
	Type constant.URL `json:"type,required"`
	paramObj
}

func (r URLImageSourceParam) MarshalJSON() (data []byte, err error) {
	type shadow URLImageSourceParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *URLImageSourceParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Type, URL are required.
type URLPDFSourceParam struct {
	URL string `json:"url,required"`
	// This field can be elided, and will marshal its zero value as "url".
	Type constant.URL `json:"type,required"`
	paramObj
}

func (r URLPDFSourceParam) MarshalJSON() (data []byte, err error) {
	type shadow URLPDFSourceParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *URLPDFSourceParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type Usage struct {
	// The number of input tokens used to create the cache entry.
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens,required"`
	// The number of input tokens read from the cache.
	CacheReadInputTokens int64 `json:"cache_read_input_tokens,required"`
	// The number of input tokens which were used.
	InputTokens int64 `json:"input_tokens,required"`
	// The number of output tokens which were used.
	OutputTokens int64 `json:"output_tokens,required"`
	// The number of server tool requests.
	ServerToolUse ServerToolUsage `json:"server_tool_use,required"`
	// If the request used the priority, standard, or batch tier.
	//
	// Any of "standard", "priority", "batch".
	ServiceTier UsageServiceTier `json:"service_tier,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
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
func (r Usage) RawJSON() string { return r.JSON.raw }
func (r *Usage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// If the request used the priority, standard, or batch tier.
type UsageServiceTier string

const (
	UsageServiceTierStandard UsageServiceTier = "standard"
	UsageServiceTierPriority UsageServiceTier = "priority"
	UsageServiceTierBatch    UsageServiceTier = "batch"
)

type WebSearchResultBlock struct {
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
func (r WebSearchResultBlock) RawJSON() string { return r.JSON.raw }
func (r *WebSearchResultBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties EncryptedContent, Title, Type, URL are required.
type WebSearchResultBlockParam struct {
	EncryptedContent string            `json:"encrypted_content,required"`
	Title            string            `json:"title,required"`
	URL              string            `json:"url,required"`
	PageAge          param.Opt[string] `json:"page_age,omitzero"`
	// This field can be elided, and will marshal its zero value as
	// "web_search_result".
	Type constant.WebSearchResult `json:"type,required"`
	paramObj
}

func (r WebSearchResultBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow WebSearchResultBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *WebSearchResultBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Name, Type are required.
type WebSearchTool20250305Param struct {
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
	UserLocation WebSearchTool20250305UserLocationParam `json:"user_location,omitzero"`
	// Create a cache control breakpoint at this content block.
	CacheControl CacheControlEphemeralParam `json:"cache_control,omitzero"`
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

func (r WebSearchTool20250305Param) MarshalJSON() (data []byte, err error) {
	type shadow WebSearchTool20250305Param
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *WebSearchTool20250305Param) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Parameters for the user's location. Used to provide more relevant search
// results.
//
// The property Type is required.
type WebSearchTool20250305UserLocationParam struct {
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

func (r WebSearchTool20250305UserLocationParam) MarshalJSON() (data []byte, err error) {
	type shadow WebSearchTool20250305UserLocationParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *WebSearchTool20250305UserLocationParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties ErrorCode, Type are required.
type WebSearchToolRequestErrorParam struct {
	// Any of "invalid_tool_input", "unavailable", "max_uses_exceeded",
	// "too_many_requests", "query_too_long".
	ErrorCode WebSearchToolRequestErrorErrorCode `json:"error_code,omitzero,required"`
	// This field can be elided, and will marshal its zero value as
	// "web_search_tool_result_error".
	Type constant.WebSearchToolResultError `json:"type,required"`
	paramObj
}

func (r WebSearchToolRequestErrorParam) MarshalJSON() (data []byte, err error) {
	type shadow WebSearchToolRequestErrorParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *WebSearchToolRequestErrorParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type WebSearchToolRequestErrorErrorCode string

const (
	WebSearchToolRequestErrorErrorCodeInvalidToolInput WebSearchToolRequestErrorErrorCode = "invalid_tool_input"
	WebSearchToolRequestErrorErrorCodeUnavailable      WebSearchToolRequestErrorErrorCode = "unavailable"
	WebSearchToolRequestErrorErrorCodeMaxUsesExceeded  WebSearchToolRequestErrorErrorCode = "max_uses_exceeded"
	WebSearchToolRequestErrorErrorCodeTooManyRequests  WebSearchToolRequestErrorErrorCode = "too_many_requests"
	WebSearchToolRequestErrorErrorCodeQueryTooLong     WebSearchToolRequestErrorErrorCode = "query_too_long"
)

type WebSearchToolResultBlock struct {
	Content   WebSearchToolResultBlockContentUnion `json:"content,required"`
	ToolUseID string                               `json:"tool_use_id,required"`
	Type      constant.WebSearchToolResult         `json:"type,required"`
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
func (r WebSearchToolResultBlock) RawJSON() string { return r.JSON.raw }
func (r *WebSearchToolResultBlock) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// WebSearchToolResultBlockContentUnion contains all possible properties and values
// from [WebSearchToolResultError], [[]WebSearchResultBlock].
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfWebSearchResultBlockArray]
type WebSearchToolResultBlockContentUnion struct {
	// This field will be present if the value is a [[]WebSearchResultBlock] instead of
	// an object.
	OfWebSearchResultBlockArray []WebSearchResultBlock `json:",inline"`
	// This field is from variant [WebSearchToolResultError].
	ErrorCode WebSearchToolResultErrorErrorCode `json:"error_code"`
	// This field is from variant [WebSearchToolResultError].
	Type constant.WebSearchToolResultError `json:"type"`
	JSON struct {
		OfWebSearchResultBlockArray respjson.Field
		ErrorCode                   respjson.Field
		Type                        respjson.Field
		raw                         string
	} `json:"-"`
}

func (u WebSearchToolResultBlockContentUnion) AsResponseWebSearchToolResultError() (v WebSearchToolResultError) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u WebSearchToolResultBlockContentUnion) AsWebSearchResultBlockArray() (v []WebSearchResultBlock) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u WebSearchToolResultBlockContentUnion) RawJSON() string { return u.JSON.raw }

func (r *WebSearchToolResultBlockContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The properties Content, ToolUseID, Type are required.
type WebSearchToolResultBlockParam struct {
	Content   WebSearchToolResultBlockParamContentUnion `json:"content,omitzero,required"`
	ToolUseID string                                    `json:"tool_use_id,required"`
	// Create a cache control breakpoint at this content block.
	CacheControl CacheControlEphemeralParam `json:"cache_control,omitzero"`
	// This field can be elided, and will marshal its zero value as
	// "web_search_tool_result".
	Type constant.WebSearchToolResult `json:"type,required"`
	paramObj
}

func (r WebSearchToolResultBlockParam) MarshalJSON() (data []byte, err error) {
	type shadow WebSearchToolResultBlockParam
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *WebSearchToolResultBlockParam) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

func NewWebSearchToolRequestError(errorCode WebSearchToolRequestErrorErrorCode) WebSearchToolResultBlockParamContentUnion {
	var variant WebSearchToolRequestErrorParam
	variant.ErrorCode = errorCode
	return WebSearchToolResultBlockParamContentUnion{OfRequestWebSearchToolResultError: &variant}
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type WebSearchToolResultBlockParamContentUnion struct {
	OfWebSearchToolResultBlockItem    []WebSearchResultBlockParam     `json:",omitzero,inline"`
	OfRequestWebSearchToolResultError *WebSearchToolRequestErrorParam `json:",omitzero,inline"`
	paramUnion
}

func (u WebSearchToolResultBlockParamContentUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfWebSearchToolResultBlockItem, u.OfRequestWebSearchToolResultError)
}
func (u *WebSearchToolResultBlockParamContentUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *WebSearchToolResultBlockParamContentUnion) asAny() any {
	if !param.IsOmitted(u.OfWebSearchToolResultBlockItem) {
		return &u.OfWebSearchToolResultBlockItem
	} else if !param.IsOmitted(u.OfRequestWebSearchToolResultError) {
		return u.OfRequestWebSearchToolResultError
	}
	return nil
}

type WebSearchToolResultError struct {
	// Any of "invalid_tool_input", "unavailable", "max_uses_exceeded",
	// "too_many_requests", "query_too_long".
	ErrorCode WebSearchToolResultErrorErrorCode `json:"error_code,required"`
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
func (r WebSearchToolResultError) RawJSON() string { return r.JSON.raw }
func (r *WebSearchToolResultError) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type WebSearchToolResultErrorErrorCode string

const (
	WebSearchToolResultErrorErrorCodeInvalidToolInput WebSearchToolResultErrorErrorCode = "invalid_tool_input"
	WebSearchToolResultErrorErrorCodeUnavailable      WebSearchToolResultErrorErrorCode = "unavailable"
	WebSearchToolResultErrorErrorCodeMaxUsesExceeded  WebSearchToolResultErrorErrorCode = "max_uses_exceeded"
	WebSearchToolResultErrorErrorCodeTooManyRequests  WebSearchToolResultErrorErrorCode = "too_many_requests"
	WebSearchToolResultErrorErrorCodeQueryTooLong     WebSearchToolResultErrorErrorCode = "query_too_long"
)

type MessageNewParams struct {
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
	ServiceTier MessageNewParamsServiceTier `json:"service_tier,omitzero"`
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

func (r MessageNewParams) MarshalJSON() (data []byte, err error) {
	type shadow MessageNewParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *MessageNewParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Determines whether to use priority capacity (if available) or standard capacity
// for this request.
//
// Anthropic offers different levels of service for your API requests. See
// [service-tiers](https://docs.anthropic.com/en/api/service-tiers) for details.
type MessageNewParamsServiceTier string

const (
	MessageNewParamsServiceTierAuto         MessageNewParamsServiceTier = "auto"
	MessageNewParamsServiceTierStandardOnly MessageNewParamsServiceTier = "standard_only"
)

type MessageCountTokensParams struct {
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
	// System prompt.
	//
	// A system prompt is a way of providing context and instructions to Claude, such
	// as specifying a particular goal or role. See our
	// [guide to system prompts](https://docs.anthropic.com/en/docs/system-prompts).
	System MessageCountTokensParamsSystemUnion `json:"system,omitzero"`
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
	Tools []MessageCountTokensToolUnionParam `json:"tools,omitzero"`
	paramObj
}

func (r MessageCountTokensParams) MarshalJSON() (data []byte, err error) {
	type shadow MessageCountTokensParams
	return param.MarshalObject(r, (*shadow)(&r))
}
func (r *MessageCountTokensParams) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Only one field can be non-zero.
//
// Use [param.IsOmitted] to confirm if a field is set.
type MessageCountTokensParamsSystemUnion struct {
	OfString         param.Opt[string] `json:",omitzero,inline"`
	OfTextBlockArray []TextBlockParam  `json:",omitzero,inline"`
	paramUnion
}

func (u MessageCountTokensParamsSystemUnion) MarshalJSON() ([]byte, error) {
	return param.MarshalUnion(u, u.OfString, u.OfTextBlockArray)
}
func (u *MessageCountTokensParamsSystemUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, u)
}

func (u *MessageCountTokensParamsSystemUnion) asAny() any {
	if !param.IsOmitted(u.OfString) {
		return &u.OfString.Value
	} else if !param.IsOmitted(u.OfTextBlockArray) {
		return &u.OfTextBlockArray
	}
	return nil
}

// CalculateNonStreamingTimeout calculates the appropriate timeout for a non-streaming request
// based on the maximum number of tokens and the model's non-streaming token limit
func CalculateNonStreamingTimeout(maxTokens int, model Model, opts []option.RequestOption) (time.Duration, error) {
	preCfg, err := requestconfig.PreRequestOptions(opts...)
	if err != nil {
		return 0, fmt.Errorf("error applying request options: %w", err)
	}
	// if the user has set a specific request timeout, use that
	if preCfg.RequestTimeout != 0 {
		return preCfg.RequestTimeout, nil
	}

	maximumTime := 60 * 60 * time.Second
	defaultTime := 60 * 10 * time.Second

	expectedTime := time.Duration(float64(maximumTime) * float64(maxTokens) / 128000.0)

	// If the model has a non-streaming token limit and max_tokens exceeds it,
	// or if the expected time exceeds default time, recommend streaming
	maxNonStreamingTokens, hasLimit := constant.ModelNonStreamingTokens[string(model)]
	if expectedTime > defaultTime || (hasLimit && maxTokens > maxNonStreamingTokens) {
		return 0, fmt.Errorf("streaming is strongly recommended for operations that may take longer than 10 minutes")
	}

	return expectedTime, nil
}
