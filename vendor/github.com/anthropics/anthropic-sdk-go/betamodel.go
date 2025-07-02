// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package anthropic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/anthropics/anthropic-sdk-go/internal/apijson"
	"github.com/anthropics/anthropic-sdk-go/internal/apiquery"
	"github.com/anthropics/anthropic-sdk-go/internal/requestconfig"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/pagination"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/anthropics/anthropic-sdk-go/packages/respjson"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
)

// BetaModelService contains methods and other services that help with interacting
// with the anthropic API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewBetaModelService] method instead.
type BetaModelService struct {
	Options []option.RequestOption
}

// NewBetaModelService generates a new service that applies the given options to
// each request. These options are applied after the parent client's options (if
// there is one), and before any request-specific options.
func NewBetaModelService(opts ...option.RequestOption) (r BetaModelService) {
	r = BetaModelService{}
	r.Options = opts
	return
}

// Get a specific model.
//
// The Models API response can be used to determine information about a specific
// model or resolve a model alias to a model ID.
func (r *BetaModelService) Get(ctx context.Context, modelID string, query BetaModelGetParams, opts ...option.RequestOption) (res *BetaModelInfo, err error) {
	for _, v := range query.Betas {
		opts = append(opts, option.WithHeaderAdd("anthropic-beta", fmt.Sprintf("%s", v)))
	}
	opts = append(r.Options[:], opts...)
	if modelID == "" {
		err = errors.New("missing required model_id parameter")
		return
	}
	path := fmt.Sprintf("v1/models/%s?beta=true", modelID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// List available models.
//
// The Models API response can be used to determine which models are available for
// use in the API. More recently released models are listed first.
func (r *BetaModelService) List(ctx context.Context, params BetaModelListParams, opts ...option.RequestOption) (res *pagination.Page[BetaModelInfo], err error) {
	var raw *http.Response
	for _, v := range params.Betas {
		opts = append(opts, option.WithHeaderAdd("anthropic-beta", fmt.Sprintf("%s", v)))
	}
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithResponseInto(&raw)}, opts...)
	path := "v1/models?beta=true"
	cfg, err := requestconfig.NewRequestConfig(ctx, http.MethodGet, path, params, &res, opts...)
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

// List available models.
//
// The Models API response can be used to determine which models are available for
// use in the API. More recently released models are listed first.
func (r *BetaModelService) ListAutoPaging(ctx context.Context, params BetaModelListParams, opts ...option.RequestOption) *pagination.PageAutoPager[BetaModelInfo] {
	return pagination.NewPageAutoPager(r.List(ctx, params, opts...))
}

type BetaModelInfo struct {
	// Unique model identifier.
	ID string `json:"id,required"`
	// RFC 3339 datetime string representing the time at which the model was released.
	// May be set to an epoch value if the release date is unknown.
	CreatedAt time.Time `json:"created_at,required" format:"date-time"`
	// A human-readable name for the model.
	DisplayName string `json:"display_name,required"`
	// Object type.
	//
	// For Models, this is always `"model"`.
	Type constant.Model `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		CreatedAt   respjson.Field
		DisplayName respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaModelInfo) RawJSON() string { return r.JSON.raw }
func (r *BetaModelInfo) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaModelGetParams struct {
	// Optional header to specify the beta version(s) you want to use.
	Betas []AnthropicBeta `header:"anthropic-beta,omitzero" json:"-"`
	paramObj
}

type BetaModelListParams struct {
	// ID of the object to use as a cursor for pagination. When provided, returns the
	// page of results immediately after this object.
	AfterID param.Opt[string] `query:"after_id,omitzero" json:"-"`
	// ID of the object to use as a cursor for pagination. When provided, returns the
	// page of results immediately before this object.
	BeforeID param.Opt[string] `query:"before_id,omitzero" json:"-"`
	// Number of items to return per page.
	//
	// Defaults to `20`. Ranges from `1` to `1000`.
	Limit param.Opt[int64] `query:"limit,omitzero" json:"-"`
	// Optional header to specify the beta version(s) you want to use.
	Betas []AnthropicBeta `header:"anthropic-beta,omitzero" json:"-"`
	paramObj
}

// URLQuery serializes [BetaModelListParams]'s query parameters as `url.Values`.
func (r BetaModelListParams) URLQuery() (v url.Values, err error) {
	return apiquery.MarshalWithSettings(r, apiquery.QuerySettings{
		ArrayFormat:  apiquery.ArrayQueryFormatComma,
		NestedFormat: apiquery.NestedQueryFormatBrackets,
	})
}
