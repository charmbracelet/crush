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

// A provider that fails mid-stream answers inside a response that already
// went out as a 200. Anthropic opens the stream and then sends an
// overloaded_error event down it, so the status says the request succeeded
// while the body says the provider could not serve it. fantasy derives the
// title from the status, and showing that as the heading of a failed turn
// tells the reader the request went fine.
func TestProviderErrorTitleDropsATitleTakenFromASuccessfulStatus(t *testing.T) {
	t.Parallel()

	// The error as it actually arrives: titled from the status, and marked
	// transient because the payload named a condition worth retrying.
	overloaded := &fantasy.ProviderError{
		Title:          fantasy.ErrorTitleForStatusCode(http.StatusOK),
		Message:        `POST "https://api.anthropic.com/v1/messages?beta=true": 200 OK`,
		StatusCode:     http.StatusOK,
		TransientError: fantasy.TransientStreamErrorTypes["overloaded_error"],
	}
	require.True(t, overloaded.TransientError, "the payload has to count as retryable")
	require.Empty(t, providerErrorTitle(overloaded),
		`a 200 must not contribute a title; %q reads as success`, overloaded.Title)
}

// Every 2xx status is a successful exchange, so none of them can describe a
// failure. The bounds are checked rather than assumed, since the range is
// what makes this more than a special case for one provider's status code.
func TestProviderErrorTitleDropsTheWholeSuccessRange(t *testing.T) {
	t.Parallel()

	for _, status := range []int{
		http.StatusOK,
		http.StatusCreated,
		http.StatusNoContent,
		http.StatusPartialContent,
		http.StatusIMUsed,
	} {
		err := &fantasy.ProviderError{
			Title:      fantasy.ErrorTitleForStatusCode(status),
			StatusCode: status,
		}
		require.Empty(t, providerErrorTitle(err), "status %d is a success", status)
	}
}

// The title is still the right thing to show for a genuine failure, so this
// has to stay narrow. Dropping it everywhere would turn every provider error
// into the same uninformative heading.
func TestProviderErrorTitleKeepsATitleTakenFromAFailure(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		status int
		want   string
	}{
		{http.StatusBadRequest, "bad request"},
		{http.StatusUnauthorized, "unauthorized"},
		{http.StatusTooManyRequests, "too many requests"},
		{http.StatusInternalServerError, "internal server error"},
		{529, ""}, // Anthropic's own overload status has no http.StatusText.
	} {
		err := &fantasy.ProviderError{
			Title:      fantasy.ErrorTitleForStatusCode(tc.status),
			StatusCode: tc.status,
		}
		require.Equal(t, tc.want, providerErrorTitle(err), "status %d", tc.status)
	}
}

// A status of zero means no HTTP exchange produced this error, which carries
// no title either. It must not be swept up as a success.
func TestProviderErrorTitleLeavesAnUnsetStatusAlone(t *testing.T) {
	t.Parallel()

	err := &fantasy.ProviderError{StatusCode: 0}
	require.Empty(t, providerErrorTitle(err))

	named := &fantasy.ProviderError{Title: "stream ended early", StatusCode: 0}
	require.Equal(t, "stream ended early", providerErrorTitle(named),
		"a title that did not come from a status is worth keeping")
}

// The attempt count belongs to every kind of failure, not to the provider
// branch that happened to need it first. Before it moved up here the generic
// fallback silently lost it, so each arm of the chain is checked.
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
			last: &fantasy.Error{Title: "stream error", Message: envelope},
			want: "Failed after 4 attempts. Overloaded.",
		},
		{
			// The arm that used to miss out: a plain error reached the
			// fallback, which never asked how many attempts it took.
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

// A cancelled turn was not a failure of attempts, so it says nothing about
// them even when it unwraps to something retried.
func TestTurnFailureLeavesACancellationAlone(t *testing.T) {
	t.Parallel()

	retried := &fantasy.RetryError{Errors: []error{context.Canceled, context.Canceled}}
	reason, title, detail := turnFailure(retried, Model{}, true)

	require.Equal(t, message.FinishReasonCanceled, reason)
	require.Equal(t, "User canceled request", title)
	require.Empty(t, detail)
}

// A response refused on every retry keeps the refusal banner instead of
// reading as a provider failure.
func TestTurnFailureKeepsARefusalARefusal(t *testing.T) {
	t.Parallel()

	retried := &fantasy.RetryError{Errors: []error{refusedError(false), refusedError(false), refusedError(false)}}
	reason, title, detail := turnFailure(retried, Model{}, false)

	require.Equal(t, message.FinishReasonContentFilter, reason)
	require.Empty(t, title)
	require.Empty(t, detail)
	require.Equal(t, refusedToolCallResult, turnEndReason(&message.Message{}, retried))
}

// A failure that was never retried has no count to report, and inventing
// "after 1 attempt" would be noise on the common case.
func TestTurnFailureLeavesASingleFailureAlone(t *testing.T) {
	t.Parallel()

	plain := &fantasy.ProviderError{Title: "bad request", Message: "bad request", StatusCode: http.StatusBadRequest}
	_, title, detail := turnFailure(plain, Model{}, false)
	require.Equal(t, "Bad Request", title)
	require.Equal(t, "bad request", detail)

	// fantasy leaves a first-try failure unwrapped, but guard the degenerate
	// wrapper too.
	for _, single := range []error{
		&fantasy.RetryError{Errors: []error{plain}},
		&fantasy.RetryError{},
	} {
		_, _, detail := turnFailure(single, Model{}, false)
		require.NotContains(t, detail, "attempts")
	}
}

// The exact message from the report that prompted this, start to finish.
// The envelope is what an SDK writes for a bug report: request line, status,
// ids, and the one readable sentence last. Rebuilding it around that
// sentence is the whole point, so the assertion is the finished string.
func TestProviderErrorDetailRebuildsAnAnthropicOverload(t *testing.T) {
	t.Parallel()

	const raw = `POST "https://api.anthropic.com/v1/messages?beta=true": 200 OK ` +
		`(Request-ID: req_011Cfm4qDWZ8nD1m4qEtuczo) (Workspace-ID: wrkspc_015u9B6rCAGBp3YPvRrt38NF) ` +
		`{"type":"error","error":{"details":null,"type":"overloaded_error","message":"Overloaded"},` +
		`"request_id":"req_011Cfm4qDWZ8nD1m4qEtuczo"}`

	require.Equal(t,
		"Overloaded. Request req_011Cfm4qDWZ8nD1m4qEtuczo.",
		providerErrorDetail(raw))

	// And as the reader actually meets it, once the turn has given up.
	last := &fantasy.ProviderError{
		Message:        raw,
		StatusCode:     http.StatusOK,
		TransientError: fantasy.TransientStreamErrorTypes["overloaded_error"],
	}
	exhausted := &fantasy.RetryError{Errors: []error{last, last, last, last}}
	_, title, detail := turnFailure(exhausted, Model{}, false)

	require.Equal(t, "Provider Error", title,
		"a 200 cannot title a failure, so the default stands in")
	require.Equal(t,
		"Failed after 4 attempts. Overloaded. Request req_011Cfm4qDWZ8nD1m4qEtuczo.",
		detail)
}

// The envelope is shared well beyond Anthropic, so the same rebuild has to
// hold for the other providers Crush talks to rather than being a special
// case for the one that prompted it.
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
			require.Equal(t, tc.want, providerErrorDetail(tc.raw))
		})
	}
}

// A body Crush cannot read is the only account of the failure there is, so
// it has to survive intact. Trading it for a tidier line that says nothing
// would make an unrecognised provider impossible to debug.
func TestProviderErrorDetailKeepsAnUnparseableBody(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		`POST "https://x/y": 502 Bad Gateway <html>upstream down</html>`, // not JSON
		`POST "https://x/y": 500 Internal Server Error {truncated`,       // JSON that stops
		`POST "https://x/y": 200 OK {"error":{"details":null}}`,          // parses, says nothing
		`POST "https://x/y": 429 Too Many Requests`,                      // no body at all
	} {
		require.Equal(t, raw, providerErrorDetail(raw),
			"an unreadable body must reach the reader whole")
	}

	// The attempt count is still added to a body we cannot rewrite, since
	// that is exactly when the reader has least to go on.
	const opaque = `POST "https://x/y": 502 Bad Gateway <html>upstream down</html>`
	exhausted := &fantasy.RetryError{Errors: []error{
		&fantasy.ProviderError{Message: opaque},
		&fantasy.ProviderError{Message: opaque},
	}}
	_, _, detail := turnFailure(exhausted, Model{}, false)
	require.Equal(t, "Failed after 2 attempts. "+opaque, detail)
}

// A message with no envelope at all is already as readable as it gets.
func TestProviderErrorDetailLeavesAPlainMessageAlone(t *testing.T) {
	t.Parallel()

	require.Equal(t, "bad request", providerErrorDetail("bad request"))
	require.Empty(t, providerErrorDetail(""))
}

// fantasy titles an Anthropic stream failure from its error type, so an
// overload sent over a 200 already arrives with a heading worth keeping. Only
// the name of the success status itself is replaced.
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

	for _, statusTitle := range []string{"ok", "OK"} {
		ok := &fantasy.ProviderError{Title: statusTitle, Message: "Overloaded", StatusCode: http.StatusOK}
		_, title, _ := turnFailure(ok, Model{}, false)
		require.Equal(t, "Provider Error", title, "%q names the status, not the failure", statusTitle)
	}
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
