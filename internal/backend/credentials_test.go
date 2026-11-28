package backend

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var envRef = regexp.MustCompile(`^\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?$`)

// isolateProviderEnv clears every credential the catalog providers read
// from the environment, so whether a workspace opens with a provider does
// not depend on the developer's shell.
func isolateProviderEnv(t *testing.T, catalog []catwalk.Provider) {
	t.Helper()
	for _, name := range []string{
		"HYPER_API_KEY", "CRUSH_HYPER_API_KEY",
		"AWS_BEARER_TOKEN_BEDROCK", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
		"AWS_PROFILE", "AWS_DEFAULT_PROFILE", "AWS_REGION", "AWS_DEFAULT_REGION",
		"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI", "AWS_CONTAINER_CREDENTIALS_FULL_URI",
		"VERTEXAI_PROJECT", "VERTEXAI_LOCATION", "AZURE_OPENAI_API_ENDPOINT",
	} {
		t.Setenv(name, "")
	}
	for _, p := range catalog {
		if m := envRef.FindStringSubmatch(p.APIKey); m != nil {
			t.Setenv(m[1], "")
		}
	}
}

// plainCompletionsProvider picks a catalog provider that is served by the
// generic chat-completions path, and one of its models.
func plainCompletionsProvider(t *testing.T, catalog []catwalk.Provider) (catwalk.Provider, catwalk.Model) {
	t.Helper()
	special := []catwalk.InferenceProvider{
		catwalk.InferenceProviderOpenRouter, catwalk.InferenceProviderVercel,
		catwalk.InferenceProviderCopilot, catwalk.InferenceProviderOpenCodeGo,
		catwalk.InferenceProviderOpenCodeZen, catwalk.InferenceProviderZAI,
		catwalk.InferenceProviderAzure, catwalk.InferenceProviderOpenAI,
		catwalk.InferenceProviderGemini, catwalk.InferenceProviderVertexAI,
		catwalk.InferenceProviderBedrock, catwalk.InferenceProviderBedrockEurope,
		"hyper",
	}
	for _, p := range catalog {
		if p.Type != catwalk.TypeCompletions || slices.Contains(special, p.ID) || !envRef.MatchString(p.APIKey) {
			continue
		}
		for _, m := range p.Models {
			if m.EffectiveType(p.Type) == catwalk.TypeCompletions {
				return p, m
			}
		}
	}
	t.Skip("no plain chat-completions provider in the catalog")
	return catwalk.Provider{}, catwalk.Model{}
}

// A workspace that opened with no usable provider gets one, and a working
// coordinator, from a pushed credential; deleting it stops the token going
// out.
func TestManagedCredential_WorkspaceOpenedWithoutProvider(t *testing.T) {
	xdgIsolated(t)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
	catalog, err := config.Providers(&config.Config{Options: &config.Options{DisableProviderAutoUpdate: true}})
	require.NoError(t, err)
	isolateProviderEnv(t, catalog)
	provider, model := plainCompletionsProvider(t, catalog)

	const chunk = `{"id":"c","object":"chat.completion.chunk","created":0,"model":"m","choices":[{"index":0,"delta":%s,"finish_reason":%s}]}`
	var (
		mu   sync.Mutex
		seen []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "Generate a concise title") {
			mu.Lock()
			seen = append(seen, r.Header.Get("Authorization"))
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: "+chunk+"\n\n", `{"role":"assistant","content":"done"}`, "null")
		fmt.Fprintf(w, "data: "+chunk+"\n\n", `{}`, `"stop"`)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	tokenSeen := func() int {
		mu.Lock()
		defer mu.Unlock()
		n := 0
		for _, a := range seen {
			if a == "Bearer pushed-token" {
				n++
			}
		}
		return n
	}

	cwd := t.TempDir()
	id := string(provider.ID)
	cfg := fmt.Sprintf(`{"providers": {%q: {"base_url": %q}},
  "models": {"large": {"provider": %q, "model": %q}, "small": {"provider": %q, "model": %q}}}`,
		id, srv.URL, id, model.ID, id, model.ID)
	require.NoError(t, os.WriteFile(filepath.Join(cwd, "crush.json"), []byte(cfg), 0o644))

	b := New(t.Context(), nil, func() {})
	t.Cleanup(func() { drainBackend(t, b) })
	ws, _, err := b.CreateWorkspace(protoWS(cwd, filepath.Join(cwd, ".crush"), uuid.NewString()))
	require.NoError(t, err)
	require.False(t, ws.Cfg.Config().IsConfigured(), "the workspace must open without a provider")
	require.Nil(t, ws.Coordinator())

	require.NoError(t, b.SetManagedCredential(t.Context(), id, config.ManagedCredential{AccessToken: "pushed-token"}))
	coord := ws.Coordinator()
	require.NotNil(t, coord, "the push initializes the agent")

	sess, err := ws.Sessions.Create(t.Context(), "managed")
	require.NoError(t, err)
	_, err = coord.Run(t.Context(), sess.ID, "go")
	require.NoError(t, err)
	require.Equal(t, 1, tokenSeen())

	require.NoError(t, b.DeleteManagedCredential(context.Background(), id))
	_ = coord.Summarize(t.Context(), sess.ID) // the provider is gone; it must not send the token
	require.Equal(t, 1, tokenSeen(), "the deleted token was sent again")
}
