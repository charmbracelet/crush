package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestTypeSafeChoose(t *testing.T) {
	t.Parallel()

	var got typesafeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/systemone", r.URL.Path)
		require.Equal(t, "Bearer k", r.Header.Get("Authorization"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{"model":"jev-1.13.0","answers":{"route":{"type":"choice","choice":"complex","probabilities":{"simple":0.1,"complex":0.9},"confidence":0.8}}}`))
	}))
	t.Cleanup(srv.Close)

	rc := config.RouterConfig{
		Classifier:   config.SelectedModel{Provider: "typesafe", Model: "jev-latest"},
		Routes:       testRoutes(),
		Instructions: "Prefer simple.",
	}
	choice, err := typesafeChoose(t.Context(), srv.Client(), srv.URL+"/v1/", "k", nil, rc, RouteInput{Prompt: "refactor auth", PrevAssistant: "ok"})
	require.NoError(t, err)
	require.Equal(t, "complex", choice)

	require.Equal(t, "jev-latest", got.Model)
	require.Equal(t, "refactor auth", got.State["request"])
	require.Equal(t, "ok", got.State["previous_assistant_reply"])
	q := got.Questions["route"]
	require.Equal(t, "choice", q.Type)
	require.Contains(t, q.Instructions, "Prefer simple.")
	require.Equal(t, map[string]string{"simple": "quick questions", "complex": "hard work"}, q.Criteria)
}

func TestTypeSafeChooseError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"bad key"}`, http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	_, err := typesafeChoose(t.Context(), srv.Client(), srv.URL, "k", nil, config.RouterConfig{Routes: testRoutes()}, RouteInput{Prompt: "x"})
	require.ErrorContains(t, err, "401")
}
