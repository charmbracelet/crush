package config_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// A plugin that declares a catalog gets the plan's own model list, with the
// context windows and capability lists the plan publishes. What the plan does
// not publish, the config still can: the declared entry below pins one model's
// default effort and keeps everything else from the listing.
func TestPluginCatalogDiscoversModels(t *testing.T) {
	// Isolated from the developer's real config, and not parallel: the load
	// path reads env vars.
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(isolated, ".config", "crush"))
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(isolated, ".local", "share", "crush"))

	var sawPath, sawVersion, sawAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		sawVersion = r.Header.Get("anthropic-version")
		sawAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"data":[
			{"id":"big","display_name":"Big Model","max_input_tokens":1000000,"max_tokens":128000,
			 "capabilities":{"effort":{"low":{"supported":true},"high":{"supported":true}}}},
			{"id":"small","display_name":"Small Model","max_input_tokens":200000,"max_tokens":64000,
			 "capabilities":{}}]}`)
	}))
	defer server.Close()

	workDir := t.TempDir()
	writePluginScript(t, workDir, "catalog.sh", fmt.Sprintf(`provider add catalog-sub   --name "Catalog Subscription"   --type anthropic   --base-url %[1]q   --catalog-url "%[1]s/v1/models"   --catalog-header anthropic-version "2023-06-01"   --catalog-program '.data | map({id, name: .display_name, context_window: (.max_input_tokens // 0), reasoning_levels: ([.capabilities.effort // {} | to_entries[] | select(.value.supported == true) | .key])})'

model add catalog-sub/big --reasoning-effort high`, server.URL))

	store, err := config.Load(workDir, t.TempDir(), false)
	require.NoError(t, err)

	pc, ok := store.Config().Providers.Get("catalog-sub")
	require.True(t, ok)
	require.Equal(t, "/v1/models", sawPath, "a catalog url naming a full endpoint is fetched as given")
	require.Equal(t, "2023-06-01", sawVersion, "the catalog carries the header the listing demands")
	require.Empty(t, sawAuth, "an unsigned-in provider asks for its catalog with no credential")

	require.Len(t, pc.Models, 2, "the declared model is folded into the listing, not duplicated")
	var big, small bool
	for _, model := range pc.Models {
		switch model.ID {
		case "big":
			big = true
			require.Equal(t, "Big Model", model.Name, "the listing supplies what the declaration left out")
			require.Equal(t, int64(1000000), model.ContextWindow)
			require.Equal(t, "high", model.DefaultReasoningEffort, "the declared default wins")
		case "small":
			small = true
			require.Equal(t, int64(200000), model.ContextWindow)
			require.Empty(t, model.ReasoningLevels)
		}
	}
	require.True(t, big, "the pinned model keeps its id")
	require.True(t, small, "a model config never mentioned still arrives from the catalog")
}

// A catalog is usually behind the credential, so a provider that has not been
// signed in yet cannot list its models. It has to survive the load: dropping it
// would drop the very sign-in that would fix it.
func TestPluginCatalogStaysReachableBeforeSignIn(t *testing.T) {
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(isolated, ".config", "crush"))
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(isolated, ".local", "share", "crush"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	workDir := t.TempDir()
	writePluginScript(t, workDir, "signedout.sh", fmt.Sprintf(`provider add signedout-sub   --name "Signed Out"   --type google   --base-url %q   --catalog-url "%s/v1internal:fetchAvailableModels"   --catalog-method POST   --catalog-program '.data | map({id})'   --oauth-flow browser   --oauth-client-id "public-id"   --oauth-auth-url "https://auth.example.com/oauth2/authorize"   --oauth-token-url "https://oauth.example.com/token"`, server.URL, server.URL))

	store, err := config.Load(workDir, t.TempDir(), false)
	require.NoError(t, err)

	_, ok := store.Config().Providers.Get("signedout-sub")
	require.True(t, ok, "a failed catalog read must not drop a provider waiting on a sign-in")
}

// A catalog whose program does not compile is a mistake in the plugin, so it
// fails the load naming the provider instead of leaving one without models.
func TestPluginCatalogWithBadProgramFailsLoad(t *testing.T) {
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(isolated, ".config", "crush"))
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(isolated, ".local", "share", "crush"))

	workDir := t.TempDir()
	writePluginScript(t, workDir, "broken.sh", `provider add broken-sub   --name "Broken"   --type anthropic   --base-url "https://api.example.com"   --catalog-url "https://api.example.com/v1/models"   --catalog-program 'if'`)

	_, err := config.Load(workDir, t.TempDir(), false)
	require.ErrorContains(t, err, "invalid catalog for provider broken-sub")
}
