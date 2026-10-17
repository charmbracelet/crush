package copilot

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/openaicompat"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func TestAutoProviderRoutesChatCompletionsModel(t *testing.T) {
	testAutoProviderRoute(t, "gpt-4.1", chatCompletionsEndpoint)
}

func TestAutoProviderRoutesResponsesModel(t *testing.T) {
	testAutoProviderRoute(t, "gpt-5.4-mini", responsesEndpoint)
}

func testAutoProviderRoute(t *testing.T, modelID, endpoint string) {
	t.Helper()

	const responseBody = `{"id":"resp_1","object":"response","created_at":1,"status":"completed","model":"gpt-5","output":[{"id":"msg_1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"Hi","annotations":[]}]}],"usage":{"input_tokens":1,"output_tokens":1,"output_tokens_details":{},"total_tokens":2}}`
	const chatBody = `{"id":"chat_1","object":"chat.completion","created":1,"model":"gpt-4.1","choices":[{"index":0,"message":{"role":"assistant","content":"Hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`

	var inferenceCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models/session":
			fmt.Fprintf(w, `{"available_models":[%q],"session_token":"sess-1","expires_at":4102444800}`, modelID)
		case "/models":
			fmt.Fprintf(w, `{"data":[{"id":%q,"supported_endpoints":[%q]}]}`, modelID, endpoint)
		case endpoint:
			inferenceCalls++
			require.Equal(t, "sess-1", r.Header.Get(sessionTokenHeader))
			require.Equal(t, "keep", r.Header.Get("X-Test"))
			var payload struct {
				Model string `json:"model"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			require.Equal(t, modelID, payload.Model)
			w.Header().Set("Content-Type", "application/json")
			if endpoint == responsesEndpoint {
				_, _ = w.Write([]byte(responseBody))
			} else {
				_, _ = w.Write([]byte(chatBody))
			}
		default:
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	origSessionURL := autoSessionURL
	origModelsEndpoint := modelsEndpoint
	autoSessionURL = server.URL + "/models/session"
	modelsEndpoint = server.URL + "/models"
	t.Cleanup(func() {
		autoSessionURL = origSessionURL
		modelsEndpoint = origModelsEndpoint
	})

	resolver := NewAutoResolver(
		func() *oauth.Token { return &oauth.Token{AccessToken: "at"} },
		func(id string) bool { return id == "gpt-5.4-mini" },
	)
	base, err := openaicompat.New(
		openaicompat.WithBaseURL(server.URL),
		openaicompat.WithAPIKey("test"),
		openaicompat.WithHTTPClient(NewClient(false, false)),
		openaicompat.WithUseResponsesAPI(),
		openaicompat.WithResponsesAPIFunc(resolver.UsesResponsesAPI),
	)
	require.NoError(t, err)

	provider := NewAutoProvider(base, resolver)
	model, err := provider.LanguageModel(t.Context(), AutoModelID)
	require.NoError(t, err)
	require.Equal(t, AutoModelID, model.Model())

	headers := map[string]string{"X-Test": "keep"}
	response, err := model.Generate(t.Context(), fantasy.Call{
		Prompt:  fantasy.Prompt{fantasy.NewUserMessage("Hello")},
		Headers: headers,
	})
	require.NoError(t, err)
	require.Equal(t, "Hi", response.Content.Text())
	require.Equal(t, 1, inferenceCalls)
	require.NotContains(t, headers, sessionTokenHeader, "the caller's header map must not be mutated")
}
