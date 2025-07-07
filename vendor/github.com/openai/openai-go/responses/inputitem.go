// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package responses

import (
	"context"
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
	"github.com/openai/openai-go/shared/constant"
)

// InputItemService contains methods and other services that help with interacting
// with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewInputItemService] method instead.
type InputItemService struct {
	Options []option.RequestOption
}

// NewInputItemService generates a new service that applies the given options to
// each request. These options are applied after the parent client's options (if
// there is one), and before any request-specific options.
func NewInputItemService(opts ...option.RequestOption) (r InputItemService) {
	r = InputItemService{}
	r.Options = opts
	return
}

// Returns a list of input items for a given response.
func (r *InputItemService) List(ctx context.Context, responseID string, query InputItemListParams, opts ...option.RequestOption) (res *pagination.CursorPage[ResponseItemUnion], err error) {
	var raw *http.Response
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithResponseInto(&raw)}, opts...)
	if responseID == "" {
		err = errors.New("missing required response_id parameter")
		return
	}
	path := fmt.Sprintf("responses/%s/input_items", responseID)
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

// Returns a list of input items for a given response.
func (r *InputItemService) ListAutoPaging(ctx context.Context, responseID string, query InputItemListParams, opts ...option.RequestOption) *pagination.CursorPageAutoPager[ResponseItemUnion] {
	return pagination.NewCursorPageAutoPager(r.List(ctx, responseID, query, opts...))
}

// A list of Response items.
type ResponseItemList struct {
	// A list of items used to generate this response.
	Data []ResponseItemUnion `json:"data,required"`
	// The ID of the first item in the list.
	FirstID string `json:"first_id,required"`
	// Whether there are more items available.
	HasMore bool `json:"has_more,required"`
	// The ID of the last item in the list.
	LastID string `json:"last_id,required"`
	// The type of object returned, must be `list`.
	Object constant.List `json:"object,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Data        respjson.Field
		FirstID     respjson.Field
		HasMore     respjson.Field
		LastID      respjson.Field
		Object      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseItemList) RawJSON() string { return r.JSON.raw }
func (r *ResponseItemList) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type InputItemListParams struct {
	// An item ID to list items after, used in pagination.
	After param.Opt[string] `query:"after,omitzero" json:"-"`
	// An item ID to list items before, used in pagination.
	Before param.Opt[string] `query:"before,omitzero" json:"-"`
	// A limit on the number of objects to be returned. Limit can range between 1 and
	// 100, and the default is 20.
	Limit param.Opt[int64] `query:"limit,omitzero" json:"-"`
	// Additional fields to include in the response. See the `include` parameter for
	// Response creation above for more information.
	Include []ResponseIncludable `query:"include,omitzero" json:"-"`
	// The order to return the input items in. Default is `desc`.
	//
	// - `asc`: Return the input items in ascending order.
	// - `desc`: Return the input items in descending order.
	//
	// Any of "asc", "desc".
	Order InputItemListParamsOrder `query:"order,omitzero" json:"-"`
	paramObj
}

// URLQuery serializes [InputItemListParams]'s query parameters as `url.Values`.
func (r InputItemListParams) URLQuery() (v url.Values, err error) {
	return apiquery.MarshalWithSettings(r, apiquery.QuerySettings{
		ArrayFormat:  apiquery.ArrayQueryFormatBrackets,
		NestedFormat: apiquery.NestedQueryFormatBrackets,
	})
}

// The order to return the input items in. Default is `desc`.
//
// - `asc`: Return the input items in ascending order.
// - `desc`: Return the input items in descending order.
type InputItemListParamsOrder string

const (
	InputItemListParamsOrderAsc  InputItemListParamsOrder = "asc"
	InputItemListParamsOrderDesc InputItemListParamsOrder = "desc"
)
