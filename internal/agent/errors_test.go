package agent

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// Every arm of failureText reports the attempt count.
func TestTurnFailureReportsTheAttemptCountForEveryKindOfFailure(t *testing.T) {
	t.Parallel()

	const envelope = `POST "https://api.anthropic.com/v1/messages": 200 OK ` +
		`{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`

	for _, tc := range []struct {
		name string
		last error
		want string
	}{
		{
			name: "provider error",
			last: &fantasy.ProviderError{Message: envelope, StatusCode: http.StatusOK},
			want: "Failed after 4 attempts. Overloaded.",
		},
		{
			name: "fantasy error",
			last: &fantasy.Error{Title: "response failed", Message: "Overloaded (code: server_error)"},
			want: "Failed after 4 attempts. Overloaded (code: server_error)",
		},
		{
			name: "plain error",
			last: errors.New("something gave way"),
			want: "Failed after 4 attempts. something gave way",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			exhausted := &fantasy.RetryError{Errors: []error{tc.last, tc.last, tc.last, tc.last}}
			reason, _, detail := turnFailure(exhausted, Model{}, false)

			require.Equal(t, message.FinishReasonError, reason)
			require.Equal(t, tc.want, detail)
		})
	}
}

// A cancelled turn says nothing about attempts.
func TestTurnFailureLeavesACancellationAlone(t *testing.T) {
	t.Parallel()

	retried := &fantasy.RetryError{Errors: []error{context.Canceled, context.Canceled}}
	reason, title, detail := turnFailure(retried, Model{}, true)

	require.Equal(t, message.FinishReasonCanceled, reason)
	require.Equal(t, "User canceled request", title)
	require.Empty(t, detail)
}

// A turn refused on every retry keeps the refusal banner.
func TestTurnFailureKeepsARefusalARefusal(t *testing.T) {
	t.Parallel()

	retried := &fantasy.RetryError{Errors: []error{refusedError(false), refusedError(false), refusedError(false)}}
	reason, title, detail := turnFailure(retried, Model{}, false)

	require.Equal(t, message.FinishReasonContentFilter, reason)
	require.Empty(t, title)
	require.Empty(t, detail)
	require.Equal(t, refusedToolCallResult, turnEndReason(&message.Message{}, retried))
}

// A failure that was never retried reports no count.
func TestTurnFailureLeavesASingleFailureAlone(t *testing.T) {
	t.Parallel()

	plain := &fantasy.ProviderError{Title: "bad request", Message: "bad request", StatusCode: http.StatusBadRequest}
	_, title, detail := turnFailure(plain, Model{}, false)
	require.Equal(t, "Bad Request", title)
	require.Equal(t, "bad request", detail)

	// fantasy never wraps a single attempt, but guard the degenerate case.
	for _, single := range []error{
		&fantasy.RetryError{Errors: []error{plain}},
		&fantasy.RetryError{},
	} {
		_, _, detail := turnFailure(single, Model{}, false)
		require.NotContains(t, detail, "attempts")
	}
}

// The message from the original report, end to end.
func TestProviderErrorDetailRebuildsAnAnthropicOverload(t *testing.T) {
	t.Parallel()

	const raw = `POST "https://api.anthropic.com/v1/messages?beta=true": 200 OK ` +
		`(Request-ID: req_011Cfm4qDWZ8nD1m4qEtuczo) (Workspace-ID: wrkspc_015u9B6rCAGBp3YPvRrt38NF) ` +
		`{"type":"error","error":{"details":null,"type":"overloaded_error","message":"Overloaded"},` +
		`"request_id":"req_011Cfm4qDWZ8nD1m4qEtuczo"}`

	require.Equal(t,
		"Overloaded. Request req_011Cfm4qDWZ8nD1m4qEtuczo.",
		providerErrorDetail(&fantasy.ProviderError{Message: raw}))

	last := &fantasy.ProviderError{
		Message:        raw,
		StatusCode:     http.StatusOK,
		TransientError: fantasy.TransientStreamErrorTypes["overloaded_error"],
	}
	exhausted := &fantasy.RetryError{Errors: []error{last, last, last, last}}
	_, title, detail := turnFailure(exhausted, Model{}, false)

	require.Equal(t, "Provider Error", title)
	require.Equal(t,
		"Failed after 4 attempts. Overloaded. Request req_011Cfm4qDWZ8nD1m4qEtuczo.",
		detail)
}

// Other providers share the envelope.
func TestProviderErrorDetailRebuildsOtherProviders(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "gemini carries no error type",
			raw:  `POST "https://generativelanguage.googleapis.com/v1beta/models": 429 Too Many Requests {"error":{"code":429,"message":"Resource has been exhausted","status":"RESOURCE_EXHAUSTED"}}`,
			want: "Resource has been exhausted.",
		},
		{
			name: "a type with no message names the condition",
			raw:  `POST "https://api.anthropic.com/v1/messages": 200 OK {"type":"error","error":{"type":"overloaded_error"}}`,
			want: "Overloaded.",
		},
		{
			name: "a message already a sentence is not punctuated twice",
			raw:  `POST "https://x/y": 500 Internal Server Error {"error":{"message":"The server had a problem."}}`,
			want: "The server had a problem.",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, providerErrorDetail(&fantasy.ProviderError{Message: tc.raw}))
		})
	}
}

// A body Crush cannot read is the only account of the failure, so it is kept.
func TestProviderErrorDetailKeepsAnUnparseableBody(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		`POST "https://x/y": 502 Bad Gateway <html>upstream down</html>`, // not JSON
		`POST "https://x/y": 500 Internal Server Error {truncated`,       // JSON that stops
		`POST "https://x/y": 200 OK {"error":{"details":null}}`,          // parses, says nothing
		`POST "https://x/y": 429 Too Many Requests`,                      // no body at all
	} {
		require.Equal(t, raw, providerErrorDetail(&fantasy.ProviderError{Message: raw}))
	}

	const opaque = `POST "https://x/y": 502 Bad Gateway <html>upstream down</html>`
	exhausted := &fantasy.RetryError{Errors: []error{
		&fantasy.ProviderError{Message: opaque},
		&fantasy.ProviderError{Message: opaque},
	}}
	_, _, detail := turnFailure(exhausted, Model{}, false)
	require.Equal(t, "Failed after 2 attempts. "+opaque, detail)
}

// A message with no envelope is left alone.
func TestProviderErrorDetailLeavesAPlainMessageAlone(t *testing.T) {
	t.Parallel()

	require.Equal(t, "bad request", providerErrorDetail(&fantasy.ProviderError{Message: "bad request"}))
	require.Empty(t, providerErrorDetail(&fantasy.ProviderError{}))
}

// fantasy titles a mid-stream failure from its type, and that title stands.
func TestTurnFailureKeepsAMeaningfulTitleOnASuccessfulStatus(t *testing.T) {
	t.Parallel()

	overloaded := &fantasy.ProviderError{
		Title:      "provider overloaded",
		Message:    "Overloaded",
		StatusCode: http.StatusOK,
		ErrorType:  "overloaded_error",
	}
	_, title, _ := turnFailure(overloaded, Model{}, false)
	require.Equal(t, "Provider Overloaded", title)
}

// The request ID is read from the shapes fantasy's adapters actually build:
// an API error carries the response headers, while a mid-stream failure has
// only its event payload.
func TestTurnFailureFindsTheRequestIDWhereTheAdapterPutIt(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		err  *fantasy.ProviderError
		want string
	}{
		{
			// providers/anthropic: Message is the SDK's Error(), which
			// already names the request, and the headers repeat it.
			name: "anthropic api error",
			err: &fantasy.ProviderError{
				Title: "provider overloaded",
				Message: `POST "https://api.anthropic.com/v1/messages": 529 status code 529 ` +
					`(Request-ID: req_011Anthropic) ` +
					`{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"},"request_id":"req_011Anthropic"}`,
				URL:        "https://api.anthropic.com/v1/messages",
				StatusCode: 529,
				ResponseHeaders: map[string]string{
					"Request-Id": "req_011Anthropic",
					"request-id": "req_011Anthropic",
				},
				ResponseBody: []byte("HTTP/1.1 529 status code 529\r\nRequest-Id: req_011Anthropic\r\n\r\n" +
					`{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"},"request_id":"req_011Anthropic"}`),
				ErrorType:      "overloaded_error",
				TransientError: true,
			},
			want: "Overloaded. Request req_011Anthropic.",
		},
		{
			// providers/anthropic, mid-stream: no headers, and Message is
			// only the payload's sentence, so the ID is in ResponseBody.
			name: "anthropic stream error",
			err: &fantasy.ProviderError{
				Title:          "provider overloaded",
				Message:        "Overloaded",
				ResponseBody:   []byte(`{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"},"request_id":"req_011Stream"}`),
				ErrorType:      "overloaded_error",
				TransientError: true,
			},
			want: "Overloaded. Request req_011Stream.",
		},
		{
			// providers/openai: Message is the bare sentence the SDK parsed,
			// so the headers are the only place the ID appears.
			name: "openai api error, header only",
			err: &fantasy.ProviderError{
				Title:      "too many requests",
				Message:    "Rate limit reached for gpt-4",
				URL:        "https://api.openai.com/v1/chat/completions",
				StatusCode: http.StatusTooManyRequests,
				ResponseHeaders: map[string]string{
					"X-Request-Id": "req_openai123",
					"x-request-id": "req_openai123",
				},
				ResponseBody: []byte("HTTP/1.1 429 Too Many Requests\r\nX-Request-Id: req_openai123\r\n\r\n" +
					`{"error":{"message":"Rate limit reached for gpt-4","type":"tokens","code":"rate_limit_exceeded"}}`),
				ErrorType: "tokens",
			},
			want: "Rate limit reached for gpt-4. Request req_openai123.",
		},
		{
			name: "openai api error, no request id",
			err: &fantasy.ProviderError{
				Title:      "too many requests",
				Message:    "Rate limit reached for gpt-4",
				StatusCode: http.StatusTooManyRequests,
			},
			want: "Rate limit reached for gpt-4",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, detail := turnFailure(tc.err, Model{}, false)
			require.Equal(t, tc.want, detail)
		})
	}
}

// The header names the request the response answered, so it wins over an ID
// the body happens to carry.
func TestProviderErrorRequestIDPrefersTheHeader(t *testing.T) {
	t.Parallel()

	err := &fantasy.ProviderError{
		Message:         `{"error":{"message":"Overloaded"},"request_id":"req_body"}`,
		ResponseHeaders: map[string]string{"request-id": "req_header"},
		ResponseBody:    []byte(`{"error":{"message":"Overloaded"},"request_id":"req_body"}`),
	}
	_, _, detail := turnFailure(err, Model{}, false)
	require.Equal(t, "Overloaded. Request req_header.", detail)
}

// Copilot's wording for a model the account has not enabled is a plain
// sentence another provider could send too, and only Copilot has GitHub
// settings to point at.
func TestTurnFailureSendsOnlyCopilotToGitHubSettings(t *testing.T) {
	t.Parallel()

	notSupported := &fantasy.ProviderError{
		Title:      "bad request",
		Message:    "The requested model is not supported.",
		StatusCode: http.StatusBadRequest,
	}

	copilot := Model{
		CatwalkCfg: catwalk.Model{Name: "GPT-5"},
		ModelCfg:   config.SelectedModel{Provider: string(catwalk.InferenceProviderCopilot)},
	}
	title, detail := failureText(notSupported, copilot)
	require.Equal(t, "Copilot model not enabled", title)
	require.Contains(t, detail, "github.com/settings/copilot")

	other := Model{
		CatwalkCfg: catwalk.Model{Name: "GPT-5"},
		ModelCfg:   config.SelectedModel{Provider: string(catwalk.InferenceProviderOpenAI)},
	}
	title, detail = failureText(notSupported, other)
	require.Equal(t, "Bad Request", title)
	require.Equal(t, "The requested model is not supported.", detail)
	require.NotContains(t, detail, "github.com")
}
