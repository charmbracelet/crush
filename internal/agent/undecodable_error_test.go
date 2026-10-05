package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/openai-go"
	"github.com/charmbracelet/openai-go/option"
	"github.com/stretchr/testify/require"
)

// TestUndecodableProviderErrorIsRecognised reproduces the failure with the
// SDK that actually produces it. An OpenAI-compatible gateway rejects a
// request with a bare JSON string instead of an error object, the SDK cannot
// decode that into the typed error it reports status codes through, and the
// decode failure is what reaches us with the HTTP status gone. Recognising
// it is the only thing standing between an expired credential and a message
// nobody can act on.
func TestUndecodableProviderErrorIsRecognised(t *testing.T) {
	t.Parallel()

	for _, body := range []string{`"Unauthorized"`, `"Overloaded"`} {
		var apiErr openai.Error
		err := apiErr.UnmarshalJSON([]byte(body))
		require.Error(t, err, "a bare string must not decode into the SDK error type")
		require.True(t, isUndecodableProviderError(err), "got %T: %v", err, err)
		require.True(t, isUndecodableProviderError(fmt.Errorf("stream failed: %w", err)),
			"the check has to survive wrapping on the way up")
	}
}

// TestProviderSDKKeepsStatusOnAnUndecodableBody guards the dependency this
// whole recovery path is a backstop for.
//
// The SDK decodes an error body into its typed error to read the status code
// off it, and a body that is not the documented envelope fails that decode.
// If the SDK ever returns the decode failure and drops the status with it,
// an expired credential arrives looking like a JSON problem and every
// recovery path keyed on the status sits out. If a dependency change puts
// that back, this fails here rather than in front of a user.
func TestProviderSDKKeepsStatusOnAnUndecodableBody(t *testing.T) {
	t.Parallel()

	for _, body := range []string{
		`{"error":"Unauthorized"}`, // error field is a string, not an object
		`"Overloaded"`,             // the whole body is a bare string
		`502 Bad Gateway`,          // not JSON at all, as a CDN would answer
	} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("content-type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprint(w, body)
			}))
			defer srv.Close()

			client := openai.NewClient(
				option.WithBaseURL(srv.URL),
				option.WithAPIKey("test"),
				option.WithMaxRetries(0),
			)
			_, err := client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
				Model:    "gpt-4",
				Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("hi")},
			})

			var apiErr *openai.Error
			require.True(t, errors.As(err, &apiErr), "got %T: %v", err, err)
			require.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)
			require.Contains(t, apiErr.RawJSON(), body,
				"the body has to survive, or there is nothing left to report")
		})
	}
}

// TestUndecodableProviderErrorIgnoresOtherFailures keeps the check narrow.
// Widening it to any decode failure would route unrelated bugs into the
// credential-refresh path, where they would be quietly retried instead of
// reported.
func TestUndecodableProviderErrorIgnoresOtherFailures(t *testing.T) {
	t.Parallel()

	var payload struct {
		Count int `json:"count"`
	}
	decodeErr := json.Unmarshal([]byte(`{"count":"lots"}`), &payload)
	require.Error(t, decodeErr)

	for _, err := range []error{
		decodeErr,
		fmt.Errorf("plain failure"),
		&fantasy.ProviderError{StatusCode: 401, Message: "unauthorized"},
		&fantasy.ProviderError{StatusCode: 500, Message: "boom"},
	} {
		require.False(t, isUndecodableProviderError(err), "%T should not match", err)
	}
}

// TestUnauthorizedStillOnlyMatchesA401 guards the boundary between the two
// checks: the new one must not quietly widen what counts as a 401.
func TestUnauthorizedStillOnlyMatchesA401(t *testing.T) {
	t.Parallel()

	require.True(t, isUnauthorized(&fantasy.ProviderError{StatusCode: 401}))
	require.False(t, isUnauthorized(&fantasy.ProviderError{StatusCode: 500}))

	var apiErr openai.Error
	undecodable := apiErr.UnmarshalJSON([]byte(`"Unauthorized"`))
	require.Error(t, undecodable)
	require.False(t, isUnauthorized(undecodable),
		"an undecodable body carries no status, so it is not a 401")
}
