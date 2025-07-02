// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package anthropic

import (
	"encoding/json"

	"github.com/anthropics/anthropic-sdk-go/internal/apijson"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/respjson"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
)

// BetaService contains methods and other services that help with interacting with
// the anthropic API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewBetaService] method instead.
type BetaService struct {
	Options  []option.RequestOption
	Models   BetaModelService
	Messages BetaMessageService
	Files    BetaFileService
}

// NewBetaService generates a new service that applies the given options to each
// request. These options are applied after the parent client's options (if there
// is one), and before any request-specific options.
func NewBetaService(opts ...option.RequestOption) (r BetaService) {
	r = BetaService{}
	r.Options = opts
	r.Models = NewBetaModelService(opts...)
	r.Messages = NewBetaMessageService(opts...)
	r.Files = NewBetaFileService(opts...)
	return
}

type AnthropicBeta = string

const (
	AnthropicBetaMessageBatches2024_09_24      AnthropicBeta = "message-batches-2024-09-24"
	AnthropicBetaPromptCaching2024_07_31       AnthropicBeta = "prompt-caching-2024-07-31"
	AnthropicBetaComputerUse2024_10_22         AnthropicBeta = "computer-use-2024-10-22"
	AnthropicBetaComputerUse2025_01_24         AnthropicBeta = "computer-use-2025-01-24"
	AnthropicBetaPDFs2024_09_25                AnthropicBeta = "pdfs-2024-09-25"
	AnthropicBetaTokenCounting2024_11_01       AnthropicBeta = "token-counting-2024-11-01"
	AnthropicBetaTokenEfficientTools2025_02_19 AnthropicBeta = "token-efficient-tools-2025-02-19"
	AnthropicBetaOutput128k2025_02_19          AnthropicBeta = "output-128k-2025-02-19"
	AnthropicBetaFilesAPI2025_04_14            AnthropicBeta = "files-api-2025-04-14"
	AnthropicBetaMCPClient2025_04_04           AnthropicBeta = "mcp-client-2025-04-04"
	AnthropicBetaDevFullThinking2025_05_14     AnthropicBeta = "dev-full-thinking-2025-05-14"
	AnthropicBetaInterleavedThinking2025_05_14 AnthropicBeta = "interleaved-thinking-2025-05-14"
	AnthropicBetaCodeExecution2025_05_22       AnthropicBeta = "code-execution-2025-05-22"
	AnthropicBetaExtendedCacheTTL2025_04_11    AnthropicBeta = "extended-cache-ttl-2025-04-11"
)

type BetaAPIError struct {
	Message string            `json:"message,required"`
	Type    constant.APIError `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Message     respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaAPIError) RawJSON() string { return r.JSON.raw }
func (r *BetaAPIError) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaAuthenticationError struct {
	Message string                       `json:"message,required"`
	Type    constant.AuthenticationError `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Message     respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaAuthenticationError) RawJSON() string { return r.JSON.raw }
func (r *BetaAuthenticationError) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaBillingError struct {
	Message string                `json:"message,required"`
	Type    constant.BillingError `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Message     respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaBillingError) RawJSON() string { return r.JSON.raw }
func (r *BetaBillingError) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// BetaErrorUnion contains all possible properties and values from
// [BetaInvalidRequestError], [BetaAuthenticationError], [BetaBillingError],
// [BetaPermissionError], [BetaNotFoundError], [BetaRateLimitError],
// [BetaGatewayTimeoutError], [BetaAPIError], [BetaOverloadedError].
//
// Use the [BetaErrorUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type BetaErrorUnion struct {
	Message string `json:"message"`
	// Any of "invalid_request_error", "authentication_error", "billing_error",
	// "permission_error", "not_found_error", "rate_limit_error", "timeout_error",
	// "api_error", "overloaded_error".
	Type string `json:"type"`
	JSON struct {
		Message respjson.Field
		Type    respjson.Field
		raw     string
	} `json:"-"`
}

// anyBetaError is implemented by each variant of [BetaErrorUnion] to add type
// safety for the return type of [BetaErrorUnion.AsAny]
type anyBetaError interface {
	implBetaErrorUnion()
}

func (BetaInvalidRequestError) implBetaErrorUnion() {}
func (BetaAuthenticationError) implBetaErrorUnion() {}
func (BetaBillingError) implBetaErrorUnion()        {}
func (BetaPermissionError) implBetaErrorUnion()     {}
func (BetaNotFoundError) implBetaErrorUnion()       {}
func (BetaRateLimitError) implBetaErrorUnion()      {}
func (BetaGatewayTimeoutError) implBetaErrorUnion() {}
func (BetaAPIError) implBetaErrorUnion()            {}
func (BetaOverloadedError) implBetaErrorUnion()     {}

// Use the following switch statement to find the correct variant
//
//	switch variant := BetaErrorUnion.AsAny().(type) {
//	case anthropic.BetaInvalidRequestError:
//	case anthropic.BetaAuthenticationError:
//	case anthropic.BetaBillingError:
//	case anthropic.BetaPermissionError:
//	case anthropic.BetaNotFoundError:
//	case anthropic.BetaRateLimitError:
//	case anthropic.BetaGatewayTimeoutError:
//	case anthropic.BetaAPIError:
//	case anthropic.BetaOverloadedError:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u BetaErrorUnion) AsAny() anyBetaError {
	switch u.Type {
	case "invalid_request_error":
		return u.AsInvalidRequestError()
	case "authentication_error":
		return u.AsAuthenticationError()
	case "billing_error":
		return u.AsBillingError()
	case "permission_error":
		return u.AsPermissionError()
	case "not_found_error":
		return u.AsNotFoundError()
	case "rate_limit_error":
		return u.AsRateLimitError()
	case "timeout_error":
		return u.AsTimeoutError()
	case "api_error":
		return u.AsAPIError()
	case "overloaded_error":
		return u.AsOverloadedError()
	}
	return nil
}

func (u BetaErrorUnion) AsInvalidRequestError() (v BetaInvalidRequestError) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaErrorUnion) AsAuthenticationError() (v BetaAuthenticationError) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaErrorUnion) AsBillingError() (v BetaBillingError) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaErrorUnion) AsPermissionError() (v BetaPermissionError) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaErrorUnion) AsNotFoundError() (v BetaNotFoundError) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaErrorUnion) AsRateLimitError() (v BetaRateLimitError) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaErrorUnion) AsTimeoutError() (v BetaGatewayTimeoutError) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaErrorUnion) AsAPIError() (v BetaAPIError) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u BetaErrorUnion) AsOverloadedError() (v BetaOverloadedError) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u BetaErrorUnion) RawJSON() string { return u.JSON.raw }

func (r *BetaErrorUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaErrorResponse struct {
	Error BetaErrorUnion `json:"error,required"`
	Type  constant.Error `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Error       respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaErrorResponse) RawJSON() string { return r.JSON.raw }
func (r *BetaErrorResponse) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaGatewayTimeoutError struct {
	Message string                `json:"message,required"`
	Type    constant.TimeoutError `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Message     respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaGatewayTimeoutError) RawJSON() string { return r.JSON.raw }
func (r *BetaGatewayTimeoutError) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaInvalidRequestError struct {
	Message string                       `json:"message,required"`
	Type    constant.InvalidRequestError `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Message     respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaInvalidRequestError) RawJSON() string { return r.JSON.raw }
func (r *BetaInvalidRequestError) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaNotFoundError struct {
	Message string                 `json:"message,required"`
	Type    constant.NotFoundError `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Message     respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaNotFoundError) RawJSON() string { return r.JSON.raw }
func (r *BetaNotFoundError) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaOverloadedError struct {
	Message string                   `json:"message,required"`
	Type    constant.OverloadedError `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Message     respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaOverloadedError) RawJSON() string { return r.JSON.raw }
func (r *BetaOverloadedError) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaPermissionError struct {
	Message string                   `json:"message,required"`
	Type    constant.PermissionError `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Message     respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaPermissionError) RawJSON() string { return r.JSON.raw }
func (r *BetaPermissionError) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaRateLimitError struct {
	Message string                  `json:"message,required"`
	Type    constant.RateLimitError `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Message     respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BetaRateLimitError) RawJSON() string { return r.JSON.raw }
func (r *BetaRateLimitError) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}
