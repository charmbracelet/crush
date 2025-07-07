// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package openai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/openai/openai-go/internal/apiquery"
	"github.com/openai/openai-go/internal/requestconfig"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/pagination"
	"github.com/openai/openai-go/packages/param"
)

// ChatCompletionMessageService contains methods and other services that help with
// interacting with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewChatCompletionMessageService] method instead.
type ChatCompletionMessageService struct {
	Options []option.RequestOption
}

// NewChatCompletionMessageService generates a new service that applies the given
// options to each request. These options are applied after the parent client's
// options (if there is one), and before any request-specific options.
func NewChatCompletionMessageService(opts ...option.RequestOption) (r ChatCompletionMessageService) {
	r = ChatCompletionMessageService{}
	r.Options = opts
	return
}

// Get the messages in a stored chat completion. Only Chat Completions that have
// been created with the `store` parameter set to `true` will be returned.
func (r *ChatCompletionMessageService) List(ctx context.Context, completionID string, query ChatCompletionMessageListParams, opts ...option.RequestOption) (res *pagination.CursorPage[ChatCompletionStoreMessage], err error) {
	var raw *http.Response
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithResponseInto(&raw)}, opts...)
	if completionID == "" {
		err = errors.New("missing required completion_id parameter")
		return
	}
	path := fmt.Sprintf("chat/completions/%s/messages", completionID)
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

// Get the messages in a stored chat completion. Only Chat Completions that have
// been created with the `store` parameter set to `true` will be returned.
func (r *ChatCompletionMessageService) ListAutoPaging(ctx context.Context, completionID string, query ChatCompletionMessageListParams, opts ...option.RequestOption) *pagination.CursorPageAutoPager[ChatCompletionStoreMessage] {
	return pagination.NewCursorPageAutoPager(r.List(ctx, completionID, query, opts...))
}

type ChatCompletionMessageListParams struct {
	// Identifier for the last message from the previous pagination request.
	After param.Opt[string] `query:"after,omitzero" json:"-"`
	// Number of messages to retrieve.
	Limit param.Opt[int64] `query:"limit,omitzero" json:"-"`
	// Sort order for messages by timestamp. Use `asc` for ascending order or `desc`
	// for descending order. Defaults to `asc`.
	//
	// Any of "asc", "desc".
	Order ChatCompletionMessageListParamsOrder `query:"order,omitzero" json:"-"`
	paramObj
}

// URLQuery serializes [ChatCompletionMessageListParams]'s query parameters as
// `url.Values`.
func (r ChatCompletionMessageListParams) URLQuery() (v url.Values, err error) {
	return apiquery.MarshalWithSettings(r, apiquery.QuerySettings{
		ArrayFormat:  apiquery.ArrayQueryFormatBrackets,
		NestedFormat: apiquery.NestedQueryFormatBrackets,
	})
}

// Sort order for messages by timestamp. Use `asc` for ascending order or `desc`
// for descending order. Defaults to `asc`.
type ChatCompletionMessageListParamsOrder string

const (
	ChatCompletionMessageListParamsOrderAsc  ChatCompletionMessageListParamsOrder = "asc"
	ChatCompletionMessageListParamsOrderDesc ChatCompletionMessageListParamsOrder = "desc"
)
