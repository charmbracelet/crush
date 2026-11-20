package agent

import (
	"context"
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

// TestProviderSDKKeepsStatusOnAnUndecodableBody guards the dependency every
// status-keyed recovery path (OAuth renewal on a 401 included) relies on.
//
// The SDK decodes an error body into its typed error to read the status code
// off it, and a body that is not the documented envelope fails that decode.
// The upstream SDK returns the decode failure and the status goes with it;
// the fork Crush builds against keeps the status and carries the raw body
// through instead, which is the difference between "rate limited, wait" and
// a line about JSON that nobody can act on.
//
// If a dependency change ever puts the unpatched SDK back, this fails here
// rather than in front of a user.
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

// TestUnauthorizedStillOnlyMatchesA401 keeps the 401 check from widening:
// a body the SDK could not decode carries no status, so it must not be
// treated as a refused credential.
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
