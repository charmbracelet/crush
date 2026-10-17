package discover

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

// anthropicCatalog is the shape a Claude plan publishes: a listing whose
// capability tree says which effort levels a model takes, and how much context
// it holds. Nothing here is a field name Crush knows.
const anthropicCatalog = `{
  "data": [
    {"id": "claude-opus-5-5", "display_name": "Claude Opus 5.5",
     "max_input_tokens": 1000000, "max_tokens": 128000,
     "capabilities": {
       "effort": {"supported": true, "low": {"supported": true}, "medium": {"supported": true},
                  "high": {"supported": true}, "xhigh": {"supported": true}, "max": {"supported": true}},
       "thinking": {"supported": true}, "image_input": {"supported": true}}},
    {"id": "claude-opus-4-6", "display_name": "Claude Opus 4.6",
     "max_input_tokens": 1000000, "max_tokens": 128000,
     "capabilities": {
       "effort": {"supported": true, "low": {"supported": true}, "medium": {"supported": true},
                  "high": {"supported": true}, "xhigh": {"supported": false}, "max": {"supported": true}},
       "thinking": {"supported": true}, "image_input": {"supported": true}}},
    {"id": "claude-haiku-4-5", "display_name": "Claude Haiku 4.5",
     "max_input_tokens": 200000, "max_tokens": 64000,
     "capabilities": {
       "effort": {"supported": false, "low": {"supported": false}, "medium": {"supported": false},
                  "high": {"supported": false}, "xhigh": {"supported": false}, "max": {"supported": false}},
       "thinking": {"supported": true}, "image_input": {"supported": false}}}
  ]
}`

// catalogProgram is the mapping the plugin declares: pick the supported rungs
// of the effort ladder, in the ladder's order rather than the response's.
const catalogProgram = `def ladder: ["low", "medium", "high", "xhigh", "max"];
def supported($effort): ladder | map(select(($effort[.] // {})["supported"] == true));
def preferred($levels):
  if ($levels | index("medium")) then "medium"
  elif ($levels | index("high")) then "high"
  elif ($levels | index("low")) then "low"
  else "" end;
.data | map(
  (.capabilities // {}) as $caps
  | supported($caps.effort // {}) as $levels
  | {id, name: (.display_name // .id), context_window: (.max_input_tokens // 0),
     default_max_tokens: (.max_tokens // 0),
     can_reason: (($caps.thinking // {})["supported"] == true),
     supports_attachments: (($caps.image_input // {})["supported"] == true),
     reasoning_levels: $levels, default_reasoning_effort: preferred($levels)})`

func TestCatalogReadsAProvidersOwnListing(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, "Bearer at-123", r.Header.Get("Authorization"))
		// A listing can be gated behind a header the inference endpoint does
		// not want; it rides on the catalog call alone.
		require.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))
		_, _ = w.Write([]byte(anthropicCatalog))
	}))
	defer server.Close()

	models, err := Catalog(context.Background(), CatalogConfig{
		ID:      "plan",
		BaseURL: server.URL,
		Spec: &oauth.CatalogSpec{
			URL:     "/v1/models",
			Program: catalogProgram,
			Headers: map[string]string{"anthropic-version": "2023-06-01"},
		},
		Bearer: "at-123",
	}, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 3)

	opus := models[0]
	require.Equal(t, "claude-opus-5-5", opus.ID)
	require.Equal(t, "Claude Opus 5.5", opus.Name)
	require.Equal(t, int64(1000000), opus.ContextWindow)
	require.Equal(t, int64(128000), opus.DefaultMaxTokens)
	require.True(t, opus.CanReason)
	require.True(t, opus.SupportsImages)
	require.Equal(t, []string{"low", "medium", "high", "xhigh", "max"}, opus.ReasoningLevels)
	require.Equal(t, "medium", opus.DefaultReasoningEffort)

	// A rung the listing marks unsupported is left out, and the levels stay
	// in ladder order rather than the order the response listed them.
	require.Equal(t, []string{"low", "medium", "high", "max"}, models[1].ReasoningLevels)

	// A model that takes no effort at all gets no default rather than a
	// made-up one, and reports the smaller window it actually has.
	haiku := models[2]
	require.Empty(t, haiku.ReasoningLevels)
	require.Empty(t, haiku.DefaultReasoningEffort)
	require.Equal(t, int64(200000), haiku.ContextWindow)
	require.False(t, haiku.SupportsImages)
}

// A catalog fills in what a declared model leaves out and leaves alone what it
// sets, so a plugin can pin one field without restating the rest.
func TestCatalogKeepsDeclaredFields(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(anthropicCatalog))
	}))
	defer server.Close()

	models, err := Catalog(context.Background(), CatalogConfig{
		ID:      "plan",
		BaseURL: server.URL,
		Spec:    &oauth.CatalogSpec{URL: "/v1/models", Program: catalogProgram},
		Existing: []catwalk.Model{{
			ID:                     "claude-opus-5-5",
			Name:                   "Pinned Name",
			DefaultReasoningEffort: "high",
		}},
	}, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 3, "declared models are not duplicated by the catalog")

	first := models[0]
	require.Equal(t, "Pinned Name", first.Name, "a declared name wins")
	require.Equal(t, "high", first.DefaultReasoningEffort, "a declared effort wins")
	require.Equal(t, int64(1000000), first.ContextWindow, "the catalog fills what was left unset")
	require.Equal(t, []string{"low", "medium", "high", "xhigh", "max"}, first.ReasoningLevels)
	require.True(t, first.SupportsImages)

	// A declared model the catalog never lists survives as it was.
	models, err = Catalog(context.Background(), CatalogConfig{
		ID:       "plan",
		BaseURL:  server.URL,
		Spec:     &oauth.CatalogSpec{URL: "/v1/models", Program: catalogProgram},
		Existing: []catwalk.Model{{ID: "claude-from-the-future", Name: "Unreleased"}},
	}, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 4)
	require.Equal(t, "claude-from-the-future", models[0].ID)
	require.Equal(t, "Unreleased", models[0].Name)
}

// A program that maps one entry rather than the listing returns an object, and
// an entry with no id cannot be selected, so it is dropped.
func TestCatalogAcceptsSingleEntryAndDropsNameless(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"model": {"id": "only", "display_name": "Only"}}`))
	}))
	defer server.Close()

	models, err := Catalog(context.Background(), CatalogConfig{
		ID:      "plan",
		BaseURL: server.URL,
		Spec:    &oauth.CatalogSpec{URL: "/v1/models", Program: `.model | {id, name: .display_name}`},
	}, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "only", models[0].ID)

	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data": [{"display_name": "no id"}, {"id": "kept"}]}`))
	})
	models, err = Catalog(context.Background(), CatalogConfig{
		ID:      "plan",
		BaseURL: server.URL,
		Spec:    &oauth.CatalogSpec{URL: "/v1/models", Program: `.data | map({id, name: .display_name})`},
	}, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "kept", models[0].ID)
}

// Some gateways serve their catalog the way they serve their quota: over
// POST, answering any client but only to an empty JSON body.
func TestCatalogPostsWhenTheGatewayDemandsIt(t *testing.T) {
	t.Parallel()

	var sawMethod, sawBody, sawContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawMethod = r.Method
		sawContentType = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		sawBody = string(raw)
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, `{"data": [{"id": "posted", "display_name": "Posted"}]}`)
	}))
	defer server.Close()

	models, err := Catalog(context.Background(), CatalogConfig{
		ID:      "plan",
		BaseURL: server.URL,
		Spec: &oauth.CatalogSpec{
			URL:     "/v1internal:fetchAvailableModels",
			Method:  "POST",
			Program: `.data | map({id, name: .display_name})`,
		},
	}, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "posted", models[0].ID)
	require.Equal(t, http.MethodPost, sawMethod)
	require.Equal(t, "application/json", sawContentType)
	require.JSONEq(t, "{}", sawBody, "a POST catalog defaults to an empty body")
}

func TestCatalogErrors(t *testing.T) {
	t.Parallel()

	_, err := Catalog(context.Background(), CatalogConfig{
		ID: "plan", Spec: &oauth.CatalogSpec{URL: "/v1/models"},
	}, &mockResolver{})
	require.ErrorContains(t, err, "names no url or program")

	_, err = Catalog(context.Background(), CatalogConfig{
		ID:   "plan",
		Spec: &oauth.CatalogSpec{URL: "/v1/models", Program: `if`},
	}, &mockResolver{})
	require.ErrorContains(t, err, "catalog program for provider plan")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "expired"}`))
	}))
	defer server.Close()

	_, err = Catalog(context.Background(), CatalogConfig{
		ID:      "plan",
		BaseURL: server.URL,
		Spec:    &oauth.CatalogSpec{URL: "/v1/models", Program: `.data`},
	}, &mockResolver{})
	require.ErrorContains(t, err, "401")
}
