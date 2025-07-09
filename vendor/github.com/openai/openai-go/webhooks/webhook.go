// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/openai/openai-go/internal/apijson"
	"github.com/openai/openai-go/internal/requestconfig"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/respjson"
	"github.com/openai/openai-go/shared/constant"
)

// WebhookService contains methods and other services that help with interacting
// with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewWebhookService] method instead.
type WebhookService struct {
	Options []option.RequestOption
}

// NewWebhookService generates a new service that applies the given options to each
// request. These options are applied after the parent client's options (if there
// is one), and before any request-specific options.
func NewWebhookService(opts ...option.RequestOption) (r WebhookService) {
	r = WebhookService{}
	r.Options = opts
	return
}

// Validates that the given payload was sent by OpenAI and parses the payload.
func (r *WebhookService) Unwrap(body []byte, headers http.Header, opts ...option.RequestOption) (*UnwrapWebhookEventUnion, error) {
	// Always perform signature verification
	err := r.VerifySignature(body, headers, opts...)
	if err != nil {
		return nil, err
	}

	res := &UnwrapWebhookEventUnion{}
	err = res.UnmarshalJSON(body)
	if err != nil {
		return res, err
	}
	return res, nil
}

// UnwrapWithTolerance validates that the given payload was sent by OpenAI using custom tolerance, then parses the payload.
// tolerance specifies the maximum age of the webhook.
func (r *WebhookService) UnwrapWithTolerance(body []byte, headers http.Header, tolerance time.Duration, opts ...option.RequestOption) (*UnwrapWebhookEventUnion, error) {
	err := r.VerifySignatureWithTolerance(body, headers, tolerance, opts...)
	if err != nil {
		return nil, err
	}

	res := &UnwrapWebhookEventUnion{}
	err = res.UnmarshalJSON(body)
	if err != nil {
		return res, err
	}
	return res, nil
}

// UnwrapWithToleranceAndTime validates that the given payload was sent by OpenAI using custom tolerance and time, then parses the payload.
// tolerance specifies the maximum age of the webhook.
// now allows specifying the current time for testing purposes.
func (r *WebhookService) UnwrapWithToleranceAndTime(body []byte, headers http.Header, tolerance time.Duration, now time.Time, opts ...option.RequestOption) (*UnwrapWebhookEventUnion, error) {
	err := r.VerifySignatureWithToleranceAndTime(body, headers, tolerance, now, opts...)
	if err != nil {
		return nil, err
	}

	res := &UnwrapWebhookEventUnion{}
	err = res.UnmarshalJSON(body)
	if err != nil {
		return res, err
	}
	return res, nil
}

// VerifySignature validates whether or not the webhook payload was sent by OpenAI.
// An error will be raised if the webhook signature is invalid.
// tolerance specifies the maximum age of the webhook (default: 5 minutes).
func (r *WebhookService) VerifySignature(body []byte, headers http.Header, opts ...option.RequestOption) error {
	return r.VerifySignatureWithTolerance(body, headers, 5*time.Minute, opts...)
}

// VerifySignatureWithTolerance validates whether or not the webhook payload was sent by OpenAI.
// An error will be raised if the webhook signature is invalid.
// tolerance specifies the maximum age of the webhook.
func (r *WebhookService) VerifySignatureWithTolerance(body []byte, headers http.Header, tolerance time.Duration, opts ...option.RequestOption) error {
	return r.VerifySignatureWithToleranceAndTime(body, headers, tolerance, time.Now(), opts...)
}

// VerifySignatureWithToleranceAndTime validates whether or not the webhook payload was sent by OpenAI.
// An error will be raised if the webhook signature is invalid.
// tolerance specifies the maximum age of the webhook.
// now allows specifying the current time for testing purposes.
func (r *WebhookService) VerifySignatureWithToleranceAndTime(body []byte, headers http.Header, tolerance time.Duration, now time.Time, opts ...option.RequestOption) error {
	cfg, err := requestconfig.PreRequestOptions(r.Options...)
	if err != nil {
		return err
	}
	webhookSecret := cfg.WebhookSecret

	if webhookSecret == "" {
		return errors.New("webhook secret must be provided either in the method call or configured on the client")
	}

	if headers == nil {
		return errors.New("headers are required for webhook verification")
	}

	// Extract required headers
	signatureHeader := headers.Get("webhook-signature")
	if signatureHeader == "" {
		return errors.New("missing required webhook-signature header")
	}

	timestampHeader := headers.Get("webhook-timestamp")
	if timestampHeader == "" {
		return errors.New("missing required webhook-timestamp header")
	}

	webhookID := headers.Get("webhook-id")
	if webhookID == "" {
		return errors.New("missing required webhook-id header")
	}

	// Validate timestamp to prevent replay attacks
	timestampSeconds, err := strconv.ParseInt(timestampHeader, 10, 64)
	if err != nil {
		return errors.New("invalid webhook timestamp format")
	}

	nowUnix := now.Unix()
	toleranceSeconds := int64(tolerance.Seconds())

	if nowUnix-timestampSeconds > toleranceSeconds {
		return errors.New("webhook timestamp is too old")
	}

	if timestampSeconds > nowUnix+toleranceSeconds {
		return errors.New("webhook timestamp is too new")
	}

	// Extract signatures from v1,<base64> format
	// The signature header can have multiple values, separated by spaces.
	// Each value is in the format v1,<base64>. We should accept if any match.
	var signatures []string
	for _, part := range strings.Fields(signatureHeader) {
		if strings.HasPrefix(part, "v1,") {
			signatures = append(signatures, part[3:])
		} else {
			signatures = append(signatures, part)
		}
	}

	// Decode the secret if it starts with whsec_
	var decodedSecret []byte
	if strings.HasPrefix(webhookSecret, "whsec_") {
		decodedSecret, err = base64.StdEncoding.DecodeString(webhookSecret[6:])
		if err != nil {
			return fmt.Errorf("invalid webhook secret format: %v", err)
		}
	} else {
		decodedSecret = []byte(webhookSecret)
	}

	// Create the signed payload: {webhook_id}.{timestamp}.{payload}
	signedPayload := fmt.Sprintf("%s.%s.%s", webhookID, timestampHeader, string(body))

	// Compute HMAC-SHA256 signature
	h := hmac.New(sha256.New, decodedSecret)
	h.Write([]byte(signedPayload))
	expectedSignature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Accept if any signature matches using timing-safe comparison
	for _, signature := range signatures {
		if subtle.ConstantTimeCompare([]byte(expectedSignature), []byte(signature)) == 1 {
			return nil
		}
	}

	return errors.New("webhook signature verification failed")
}

// Sent when a batch API request has been cancelled.
type BatchCancelledWebhookEvent struct {
	// The unique ID of the event.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) of when the batch API request was cancelled.
	CreatedAt int64 `json:"created_at,required"`
	// Event data payload.
	Data BatchCancelledWebhookEventData `json:"data,required"`
	// The type of the event. Always `batch.cancelled`.
	Type constant.BatchCancelled `json:"type,required"`
	// The object of the event. Always `event`.
	//
	// Any of "event".
	Object BatchCancelledWebhookEventObject `json:"object"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		CreatedAt   respjson.Field
		Data        respjson.Field
		Type        respjson.Field
		Object      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BatchCancelledWebhookEvent) RawJSON() string { return r.JSON.raw }
func (r *BatchCancelledWebhookEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Event data payload.
type BatchCancelledWebhookEventData struct {
	// The unique ID of the batch API request.
	ID string `json:"id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BatchCancelledWebhookEventData) RawJSON() string { return r.JSON.raw }
func (r *BatchCancelledWebhookEventData) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The object of the event. Always `event`.
type BatchCancelledWebhookEventObject string

const (
	BatchCancelledWebhookEventObjectEvent BatchCancelledWebhookEventObject = "event"
)

// Sent when a batch API request has been completed.
type BatchCompletedWebhookEvent struct {
	// The unique ID of the event.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) of when the batch API request was completed.
	CreatedAt int64 `json:"created_at,required"`
	// Event data payload.
	Data BatchCompletedWebhookEventData `json:"data,required"`
	// The type of the event. Always `batch.completed`.
	Type constant.BatchCompleted `json:"type,required"`
	// The object of the event. Always `event`.
	//
	// Any of "event".
	Object BatchCompletedWebhookEventObject `json:"object"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		CreatedAt   respjson.Field
		Data        respjson.Field
		Type        respjson.Field
		Object      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BatchCompletedWebhookEvent) RawJSON() string { return r.JSON.raw }
func (r *BatchCompletedWebhookEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Event data payload.
type BatchCompletedWebhookEventData struct {
	// The unique ID of the batch API request.
	ID string `json:"id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BatchCompletedWebhookEventData) RawJSON() string { return r.JSON.raw }
func (r *BatchCompletedWebhookEventData) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The object of the event. Always `event`.
type BatchCompletedWebhookEventObject string

const (
	BatchCompletedWebhookEventObjectEvent BatchCompletedWebhookEventObject = "event"
)

// Sent when a batch API request has expired.
type BatchExpiredWebhookEvent struct {
	// The unique ID of the event.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) of when the batch API request expired.
	CreatedAt int64 `json:"created_at,required"`
	// Event data payload.
	Data BatchExpiredWebhookEventData `json:"data,required"`
	// The type of the event. Always `batch.expired`.
	Type constant.BatchExpired `json:"type,required"`
	// The object of the event. Always `event`.
	//
	// Any of "event".
	Object BatchExpiredWebhookEventObject `json:"object"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		CreatedAt   respjson.Field
		Data        respjson.Field
		Type        respjson.Field
		Object      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BatchExpiredWebhookEvent) RawJSON() string { return r.JSON.raw }
func (r *BatchExpiredWebhookEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Event data payload.
type BatchExpiredWebhookEventData struct {
	// The unique ID of the batch API request.
	ID string `json:"id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BatchExpiredWebhookEventData) RawJSON() string { return r.JSON.raw }
func (r *BatchExpiredWebhookEventData) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The object of the event. Always `event`.
type BatchExpiredWebhookEventObject string

const (
	BatchExpiredWebhookEventObjectEvent BatchExpiredWebhookEventObject = "event"
)

// Sent when a batch API request has failed.
type BatchFailedWebhookEvent struct {
	// The unique ID of the event.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) of when the batch API request failed.
	CreatedAt int64 `json:"created_at,required"`
	// Event data payload.
	Data BatchFailedWebhookEventData `json:"data,required"`
	// The type of the event. Always `batch.failed`.
	Type constant.BatchFailed `json:"type,required"`
	// The object of the event. Always `event`.
	//
	// Any of "event".
	Object BatchFailedWebhookEventObject `json:"object"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		CreatedAt   respjson.Field
		Data        respjson.Field
		Type        respjson.Field
		Object      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BatchFailedWebhookEvent) RawJSON() string { return r.JSON.raw }
func (r *BatchFailedWebhookEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Event data payload.
type BatchFailedWebhookEventData struct {
	// The unique ID of the batch API request.
	ID string `json:"id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r BatchFailedWebhookEventData) RawJSON() string { return r.JSON.raw }
func (r *BatchFailedWebhookEventData) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The object of the event. Always `event`.
type BatchFailedWebhookEventObject string

const (
	BatchFailedWebhookEventObjectEvent BatchFailedWebhookEventObject = "event"
)

// Sent when an eval run has been canceled.
type EvalRunCanceledWebhookEvent struct {
	// The unique ID of the event.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) of when the eval run was canceled.
	CreatedAt int64 `json:"created_at,required"`
	// Event data payload.
	Data EvalRunCanceledWebhookEventData `json:"data,required"`
	// The type of the event. Always `eval.run.canceled`.
	Type constant.EvalRunCanceled `json:"type,required"`
	// The object of the event. Always `event`.
	//
	// Any of "event".
	Object EvalRunCanceledWebhookEventObject `json:"object"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		CreatedAt   respjson.Field
		Data        respjson.Field
		Type        respjson.Field
		Object      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r EvalRunCanceledWebhookEvent) RawJSON() string { return r.JSON.raw }
func (r *EvalRunCanceledWebhookEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Event data payload.
type EvalRunCanceledWebhookEventData struct {
	// The unique ID of the eval run.
	ID string `json:"id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r EvalRunCanceledWebhookEventData) RawJSON() string { return r.JSON.raw }
func (r *EvalRunCanceledWebhookEventData) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The object of the event. Always `event`.
type EvalRunCanceledWebhookEventObject string

const (
	EvalRunCanceledWebhookEventObjectEvent EvalRunCanceledWebhookEventObject = "event"
)

// Sent when an eval run has failed.
type EvalRunFailedWebhookEvent struct {
	// The unique ID of the event.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) of when the eval run failed.
	CreatedAt int64 `json:"created_at,required"`
	// Event data payload.
	Data EvalRunFailedWebhookEventData `json:"data,required"`
	// The type of the event. Always `eval.run.failed`.
	Type constant.EvalRunFailed `json:"type,required"`
	// The object of the event. Always `event`.
	//
	// Any of "event".
	Object EvalRunFailedWebhookEventObject `json:"object"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		CreatedAt   respjson.Field
		Data        respjson.Field
		Type        respjson.Field
		Object      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r EvalRunFailedWebhookEvent) RawJSON() string { return r.JSON.raw }
func (r *EvalRunFailedWebhookEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Event data payload.
type EvalRunFailedWebhookEventData struct {
	// The unique ID of the eval run.
	ID string `json:"id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r EvalRunFailedWebhookEventData) RawJSON() string { return r.JSON.raw }
func (r *EvalRunFailedWebhookEventData) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The object of the event. Always `event`.
type EvalRunFailedWebhookEventObject string

const (
	EvalRunFailedWebhookEventObjectEvent EvalRunFailedWebhookEventObject = "event"
)

// Sent when an eval run has succeeded.
type EvalRunSucceededWebhookEvent struct {
	// The unique ID of the event.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) of when the eval run succeeded.
	CreatedAt int64 `json:"created_at,required"`
	// Event data payload.
	Data EvalRunSucceededWebhookEventData `json:"data,required"`
	// The type of the event. Always `eval.run.succeeded`.
	Type constant.EvalRunSucceeded `json:"type,required"`
	// The object of the event. Always `event`.
	//
	// Any of "event".
	Object EvalRunSucceededWebhookEventObject `json:"object"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		CreatedAt   respjson.Field
		Data        respjson.Field
		Type        respjson.Field
		Object      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r EvalRunSucceededWebhookEvent) RawJSON() string { return r.JSON.raw }
func (r *EvalRunSucceededWebhookEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Event data payload.
type EvalRunSucceededWebhookEventData struct {
	// The unique ID of the eval run.
	ID string `json:"id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r EvalRunSucceededWebhookEventData) RawJSON() string { return r.JSON.raw }
func (r *EvalRunSucceededWebhookEventData) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The object of the event. Always `event`.
type EvalRunSucceededWebhookEventObject string

const (
	EvalRunSucceededWebhookEventObjectEvent EvalRunSucceededWebhookEventObject = "event"
)

// Sent when a fine-tuning job has been cancelled.
type FineTuningJobCancelledWebhookEvent struct {
	// The unique ID of the event.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) of when the fine-tuning job was cancelled.
	CreatedAt int64 `json:"created_at,required"`
	// Event data payload.
	Data FineTuningJobCancelledWebhookEventData `json:"data,required"`
	// The type of the event. Always `fine_tuning.job.cancelled`.
	Type constant.FineTuningJobCancelled `json:"type,required"`
	// The object of the event. Always `event`.
	//
	// Any of "event".
	Object FineTuningJobCancelledWebhookEventObject `json:"object"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		CreatedAt   respjson.Field
		Data        respjson.Field
		Type        respjson.Field
		Object      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FineTuningJobCancelledWebhookEvent) RawJSON() string { return r.JSON.raw }
func (r *FineTuningJobCancelledWebhookEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Event data payload.
type FineTuningJobCancelledWebhookEventData struct {
	// The unique ID of the fine-tuning job.
	ID string `json:"id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FineTuningJobCancelledWebhookEventData) RawJSON() string { return r.JSON.raw }
func (r *FineTuningJobCancelledWebhookEventData) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The object of the event. Always `event`.
type FineTuningJobCancelledWebhookEventObject string

const (
	FineTuningJobCancelledWebhookEventObjectEvent FineTuningJobCancelledWebhookEventObject = "event"
)

// Sent when a fine-tuning job has failed.
type FineTuningJobFailedWebhookEvent struct {
	// The unique ID of the event.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) of when the fine-tuning job failed.
	CreatedAt int64 `json:"created_at,required"`
	// Event data payload.
	Data FineTuningJobFailedWebhookEventData `json:"data,required"`
	// The type of the event. Always `fine_tuning.job.failed`.
	Type constant.FineTuningJobFailed `json:"type,required"`
	// The object of the event. Always `event`.
	//
	// Any of "event".
	Object FineTuningJobFailedWebhookEventObject `json:"object"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		CreatedAt   respjson.Field
		Data        respjson.Field
		Type        respjson.Field
		Object      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FineTuningJobFailedWebhookEvent) RawJSON() string { return r.JSON.raw }
func (r *FineTuningJobFailedWebhookEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Event data payload.
type FineTuningJobFailedWebhookEventData struct {
	// The unique ID of the fine-tuning job.
	ID string `json:"id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FineTuningJobFailedWebhookEventData) RawJSON() string { return r.JSON.raw }
func (r *FineTuningJobFailedWebhookEventData) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The object of the event. Always `event`.
type FineTuningJobFailedWebhookEventObject string

const (
	FineTuningJobFailedWebhookEventObjectEvent FineTuningJobFailedWebhookEventObject = "event"
)

// Sent when a fine-tuning job has succeeded.
type FineTuningJobSucceededWebhookEvent struct {
	// The unique ID of the event.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) of when the fine-tuning job succeeded.
	CreatedAt int64 `json:"created_at,required"`
	// Event data payload.
	Data FineTuningJobSucceededWebhookEventData `json:"data,required"`
	// The type of the event. Always `fine_tuning.job.succeeded`.
	Type constant.FineTuningJobSucceeded `json:"type,required"`
	// The object of the event. Always `event`.
	//
	// Any of "event".
	Object FineTuningJobSucceededWebhookEventObject `json:"object"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		CreatedAt   respjson.Field
		Data        respjson.Field
		Type        respjson.Field
		Object      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FineTuningJobSucceededWebhookEvent) RawJSON() string { return r.JSON.raw }
func (r *FineTuningJobSucceededWebhookEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Event data payload.
type FineTuningJobSucceededWebhookEventData struct {
	// The unique ID of the fine-tuning job.
	ID string `json:"id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FineTuningJobSucceededWebhookEventData) RawJSON() string { return r.JSON.raw }
func (r *FineTuningJobSucceededWebhookEventData) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The object of the event. Always `event`.
type FineTuningJobSucceededWebhookEventObject string

const (
	FineTuningJobSucceededWebhookEventObjectEvent FineTuningJobSucceededWebhookEventObject = "event"
)

// Sent when a background response has been cancelled.
type ResponseCancelledWebhookEvent struct {
	// The unique ID of the event.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) of when the model response was cancelled.
	CreatedAt int64 `json:"created_at,required"`
	// Event data payload.
	Data ResponseCancelledWebhookEventData `json:"data,required"`
	// The type of the event. Always `response.cancelled`.
	Type constant.ResponseCancelled `json:"type,required"`
	// The object of the event. Always `event`.
	//
	// Any of "event".
	Object ResponseCancelledWebhookEventObject `json:"object"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		CreatedAt   respjson.Field
		Data        respjson.Field
		Type        respjson.Field
		Object      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCancelledWebhookEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseCancelledWebhookEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Event data payload.
type ResponseCancelledWebhookEventData struct {
	// The unique ID of the model response.
	ID string `json:"id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCancelledWebhookEventData) RawJSON() string { return r.JSON.raw }
func (r *ResponseCancelledWebhookEventData) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The object of the event. Always `event`.
type ResponseCancelledWebhookEventObject string

const (
	ResponseCancelledWebhookEventObjectEvent ResponseCancelledWebhookEventObject = "event"
)

// Sent when a background response has been completed.
type ResponseCompletedWebhookEvent struct {
	// The unique ID of the event.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) of when the model response was completed.
	CreatedAt int64 `json:"created_at,required"`
	// Event data payload.
	Data ResponseCompletedWebhookEventData `json:"data,required"`
	// The type of the event. Always `response.completed`.
	Type constant.ResponseCompleted `json:"type,required"`
	// The object of the event. Always `event`.
	//
	// Any of "event".
	Object ResponseCompletedWebhookEventObject `json:"object"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		CreatedAt   respjson.Field
		Data        respjson.Field
		Type        respjson.Field
		Object      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCompletedWebhookEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseCompletedWebhookEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Event data payload.
type ResponseCompletedWebhookEventData struct {
	// The unique ID of the model response.
	ID string `json:"id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseCompletedWebhookEventData) RawJSON() string { return r.JSON.raw }
func (r *ResponseCompletedWebhookEventData) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The object of the event. Always `event`.
type ResponseCompletedWebhookEventObject string

const (
	ResponseCompletedWebhookEventObjectEvent ResponseCompletedWebhookEventObject = "event"
)

// Sent when a background response has failed.
type ResponseFailedWebhookEvent struct {
	// The unique ID of the event.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) of when the model response failed.
	CreatedAt int64 `json:"created_at,required"`
	// Event data payload.
	Data ResponseFailedWebhookEventData `json:"data,required"`
	// The type of the event. Always `response.failed`.
	Type constant.ResponseFailed `json:"type,required"`
	// The object of the event. Always `event`.
	//
	// Any of "event".
	Object ResponseFailedWebhookEventObject `json:"object"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		CreatedAt   respjson.Field
		Data        respjson.Field
		Type        respjson.Field
		Object      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFailedWebhookEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseFailedWebhookEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Event data payload.
type ResponseFailedWebhookEventData struct {
	// The unique ID of the model response.
	ID string `json:"id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseFailedWebhookEventData) RawJSON() string { return r.JSON.raw }
func (r *ResponseFailedWebhookEventData) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The object of the event. Always `event`.
type ResponseFailedWebhookEventObject string

const (
	ResponseFailedWebhookEventObjectEvent ResponseFailedWebhookEventObject = "event"
)

// Sent when a background response has been interrupted.
type ResponseIncompleteWebhookEvent struct {
	// The unique ID of the event.
	ID string `json:"id,required"`
	// The Unix timestamp (in seconds) of when the model response was interrupted.
	CreatedAt int64 `json:"created_at,required"`
	// Event data payload.
	Data ResponseIncompleteWebhookEventData `json:"data,required"`
	// The type of the event. Always `response.incomplete`.
	Type constant.ResponseIncomplete `json:"type,required"`
	// The object of the event. Always `event`.
	//
	// Any of "event".
	Object ResponseIncompleteWebhookEventObject `json:"object"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		CreatedAt   respjson.Field
		Data        respjson.Field
		Type        respjson.Field
		Object      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseIncompleteWebhookEvent) RawJSON() string { return r.JSON.raw }
func (r *ResponseIncompleteWebhookEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Event data payload.
type ResponseIncompleteWebhookEventData struct {
	// The unique ID of the model response.
	ID string `json:"id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ResponseIncompleteWebhookEventData) RawJSON() string { return r.JSON.raw }
func (r *ResponseIncompleteWebhookEventData) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The object of the event. Always `event`.
type ResponseIncompleteWebhookEventObject string

const (
	ResponseIncompleteWebhookEventObjectEvent ResponseIncompleteWebhookEventObject = "event"
)

// UnwrapWebhookEventUnion contains all possible properties and values from
// [BatchCancelledWebhookEvent], [BatchCompletedWebhookEvent],
// [BatchExpiredWebhookEvent], [BatchFailedWebhookEvent],
// [EvalRunCanceledWebhookEvent], [EvalRunFailedWebhookEvent],
// [EvalRunSucceededWebhookEvent], [FineTuningJobCancelledWebhookEvent],
// [FineTuningJobFailedWebhookEvent], [FineTuningJobSucceededWebhookEvent],
// [ResponseCancelledWebhookEvent], [ResponseCompletedWebhookEvent],
// [ResponseFailedWebhookEvent], [ResponseIncompleteWebhookEvent].
//
// Use the [UnwrapWebhookEventUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type UnwrapWebhookEventUnion struct {
	ID        string `json:"id"`
	CreatedAt int64  `json:"created_at"`
	// This field is a union of [BatchCancelledWebhookEventData],
	// [BatchCompletedWebhookEventData], [BatchExpiredWebhookEventData],
	// [BatchFailedWebhookEventData], [EvalRunCanceledWebhookEventData],
	// [EvalRunFailedWebhookEventData], [EvalRunSucceededWebhookEventData],
	// [FineTuningJobCancelledWebhookEventData], [FineTuningJobFailedWebhookEventData],
	// [FineTuningJobSucceededWebhookEventData], [ResponseCancelledWebhookEventData],
	// [ResponseCompletedWebhookEventData], [ResponseFailedWebhookEventData],
	// [ResponseIncompleteWebhookEventData]
	Data UnwrapWebhookEventUnionData `json:"data"`
	// Any of "batch.cancelled", "batch.completed", "batch.expired", "batch.failed",
	// "eval.run.canceled", "eval.run.failed", "eval.run.succeeded",
	// "fine_tuning.job.cancelled", "fine_tuning.job.failed",
	// "fine_tuning.job.succeeded", "response.cancelled", "response.completed",
	// "response.failed", "response.incomplete".
	Type   string `json:"type"`
	Object string `json:"object"`
	JSON   struct {
		ID        respjson.Field
		CreatedAt respjson.Field
		Data      respjson.Field
		Type      respjson.Field
		Object    respjson.Field
		raw       string
	} `json:"-"`
}

// anyUnwrapWebhookEvent is implemented by each variant of
// [UnwrapWebhookEventUnion] to add type safety for the return type of
// [UnwrapWebhookEventUnion.AsAny]
type anyUnwrapWebhookEvent interface {
	implUnwrapWebhookEventUnion()
}

func (BatchCancelledWebhookEvent) implUnwrapWebhookEventUnion()         {}
func (BatchCompletedWebhookEvent) implUnwrapWebhookEventUnion()         {}
func (BatchExpiredWebhookEvent) implUnwrapWebhookEventUnion()           {}
func (BatchFailedWebhookEvent) implUnwrapWebhookEventUnion()            {}
func (EvalRunCanceledWebhookEvent) implUnwrapWebhookEventUnion()        {}
func (EvalRunFailedWebhookEvent) implUnwrapWebhookEventUnion()          {}
func (EvalRunSucceededWebhookEvent) implUnwrapWebhookEventUnion()       {}
func (FineTuningJobCancelledWebhookEvent) implUnwrapWebhookEventUnion() {}
func (FineTuningJobFailedWebhookEvent) implUnwrapWebhookEventUnion()    {}
func (FineTuningJobSucceededWebhookEvent) implUnwrapWebhookEventUnion() {}
func (ResponseCancelledWebhookEvent) implUnwrapWebhookEventUnion()      {}
func (ResponseCompletedWebhookEvent) implUnwrapWebhookEventUnion()      {}
func (ResponseFailedWebhookEvent) implUnwrapWebhookEventUnion()         {}
func (ResponseIncompleteWebhookEvent) implUnwrapWebhookEventUnion()     {}

// Use the following switch statement to find the correct variant
//
//	switch variant := UnwrapWebhookEventUnion.AsAny().(type) {
//	case webhooks.BatchCancelledWebhookEvent:
//	case webhooks.BatchCompletedWebhookEvent:
//	case webhooks.BatchExpiredWebhookEvent:
//	case webhooks.BatchFailedWebhookEvent:
//	case webhooks.EvalRunCanceledWebhookEvent:
//	case webhooks.EvalRunFailedWebhookEvent:
//	case webhooks.EvalRunSucceededWebhookEvent:
//	case webhooks.FineTuningJobCancelledWebhookEvent:
//	case webhooks.FineTuningJobFailedWebhookEvent:
//	case webhooks.FineTuningJobSucceededWebhookEvent:
//	case webhooks.ResponseCancelledWebhookEvent:
//	case webhooks.ResponseCompletedWebhookEvent:
//	case webhooks.ResponseFailedWebhookEvent:
//	case webhooks.ResponseIncompleteWebhookEvent:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u UnwrapWebhookEventUnion) AsAny() anyUnwrapWebhookEvent {
	switch u.Type {
	case "batch.cancelled":
		return u.AsBatchCancelled()
	case "batch.completed":
		return u.AsBatchCompleted()
	case "batch.expired":
		return u.AsBatchExpired()
	case "batch.failed":
		return u.AsBatchFailed()
	case "eval.run.canceled":
		return u.AsEvalRunCanceled()
	case "eval.run.failed":
		return u.AsEvalRunFailed()
	case "eval.run.succeeded":
		return u.AsEvalRunSucceeded()
	case "fine_tuning.job.cancelled":
		return u.AsFineTuningJobCancelled()
	case "fine_tuning.job.failed":
		return u.AsFineTuningJobFailed()
	case "fine_tuning.job.succeeded":
		return u.AsFineTuningJobSucceeded()
	case "response.cancelled":
		return u.AsResponseCancelled()
	case "response.completed":
		return u.AsResponseCompleted()
	case "response.failed":
		return u.AsResponseFailed()
	case "response.incomplete":
		return u.AsResponseIncomplete()
	}
	return nil
}

func (u UnwrapWebhookEventUnion) AsBatchCancelled() (v BatchCancelledWebhookEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u UnwrapWebhookEventUnion) AsBatchCompleted() (v BatchCompletedWebhookEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u UnwrapWebhookEventUnion) AsBatchExpired() (v BatchExpiredWebhookEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u UnwrapWebhookEventUnion) AsBatchFailed() (v BatchFailedWebhookEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u UnwrapWebhookEventUnion) AsEvalRunCanceled() (v EvalRunCanceledWebhookEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u UnwrapWebhookEventUnion) AsEvalRunFailed() (v EvalRunFailedWebhookEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u UnwrapWebhookEventUnion) AsEvalRunSucceeded() (v EvalRunSucceededWebhookEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u UnwrapWebhookEventUnion) AsFineTuningJobCancelled() (v FineTuningJobCancelledWebhookEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u UnwrapWebhookEventUnion) AsFineTuningJobFailed() (v FineTuningJobFailedWebhookEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u UnwrapWebhookEventUnion) AsFineTuningJobSucceeded() (v FineTuningJobSucceededWebhookEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u UnwrapWebhookEventUnion) AsResponseCancelled() (v ResponseCancelledWebhookEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u UnwrapWebhookEventUnion) AsResponseCompleted() (v ResponseCompletedWebhookEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u UnwrapWebhookEventUnion) AsResponseFailed() (v ResponseFailedWebhookEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u UnwrapWebhookEventUnion) AsResponseIncomplete() (v ResponseIncompleteWebhookEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u UnwrapWebhookEventUnion) RawJSON() string { return u.JSON.raw }

func (r *UnwrapWebhookEventUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// UnwrapWebhookEventUnionData is an implicit subunion of
// [UnwrapWebhookEventUnion]. UnwrapWebhookEventUnionData provides convenient
// access to the sub-properties of the union.
//
// For type safety it is recommended to directly use a variant of the
// [UnwrapWebhookEventUnion].
type UnwrapWebhookEventUnionData struct {
	ID   string `json:"id"`
	JSON struct {
		ID  respjson.Field
		raw string
	} `json:"-"`
}

func (r *UnwrapWebhookEventUnionData) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}
