package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func validRouter() RouterConfig {
	return RouterConfig{
		Classifier: SelectedModel{Provider: "openai", Model: "gpt-4"},
		Routes: []RouterRoute{
			{Name: "simple", Description: "easy", Model: SelectedModel{Provider: "openai", Model: "gpt-4"}},
			{Name: "complex", Description: "hard", Model: SelectedModel{Provider: "anthropic", Model: "claude-3"}},
		},
	}
}

func TestValidateRouter(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		name   string
		mutate func(*RouterConfig)
		ok     bool
	}{
		"valid":               {name: "auto", mutate: func(*RouterConfig) {}, ok: true},
		"bad name":            {name: "Auto Router", mutate: func(*RouterConfig) {}},
		"no classifier":       {name: "auto", mutate: func(r *RouterConfig) { r.Classifier = SelectedModel{} }},
		"no routes":           {name: "auto", mutate: func(r *RouterConfig) { r.Routes = nil }},
		"duplicate route":     {name: "auto", mutate: func(r *RouterConfig) { r.Routes[1].Name = "simple" }},
		"route without model": {name: "auto", mutate: func(r *RouterConfig) { r.Routes[0].Model = SelectedModel{} }},
		"unknown default":     {name: "auto", mutate: func(r *RouterConfig) { r.Default = "nope" }},
		"known default":       {name: "auto", mutate: func(r *RouterConfig) { r.Default = "complex" }, ok: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := validRouter()
			tc.mutate(&r)
			err := ValidateRouter(tc.name, r)
			if tc.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestRouterDefaultRoute(t *testing.T) {
	t.Parallel()
	r := validRouter()
	require.Equal(t, "simple", r.DefaultRoute().Name)
	r.Default = "complex"
	require.Equal(t, "complex", r.DefaultRoute().Name)
}

func TestCloneForWriteDoesNotShareRouters(t *testing.T) {
	t.Parallel()
	orig := &Config{Routers: map[string]RouterConfig{"auto": validRouter()}}
	clone := orig.cloneForWrite()
	clone.Routers["other"] = validRouter()
	require.NotContains(t, orig.Routers, "other")
}

func routerConfigFile() string {
	return `{
		"models": {"large": {"provider": "openai", "model": "gpt-4"}},
		"routers": {
			"auto": {
				"classifier": {"provider": "openai", "model": "gpt-4"},
				"routes": [
					{"name": "simple", "description": "easy", "model": {"provider": "openai", "model": "gpt-4"}},
					{"name": "complex", "description": "hard", "model": {"provider": "anthropic", "model": "claude-3"}}
				]
			}
		},
		"providers": {
			"openai": {"api_key": "test-key", "models": [{"id": "gpt-4", "name": "GPT-4"}]},
			"anthropic": {"api_key": "test-key-2", "models": [{"id": "claude-3", "name": "Claude 3"}]}
		}
	}`
}

func loadRouterStore(t *testing.T) (*ConfigStore, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")
	t.Setenv("CRUSH_GLOBAL_CONFIG", dir)
	t.Setenv("CRUSH_GLOBAL_DATA", dir)
	resetProviderState()
	t.Cleanup(resetProviderState)
	require.NoError(t, os.WriteFile(configPath, []byte(routerConfigFile()), 0o600))

	store, err := Load(t.Context(), dir, dir, false)
	require.NoError(t, err)
	store.globalDataPath = configPath
	store.CaptureStalenessSnapshot([]string{configPath})
	return store, configPath
}

func TestSetActiveRouterPersistsAndSurvivesReload(t *testing.T) {
	store, configPath := loadRouterStore(t)

	require.Error(t, store.SetActiveRouter(ScopeGlobal, "missing"))
	require.NoError(t, store.SetActiveRouter(ScopeGlobal, "auto"))
	require.Equal(t, "auto", store.Config().ActiveRouter)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(data), `"active_router":"auto"`)

	// A sibling instance rewrites the shared file without a router.
	require.NoError(t, os.WriteFile(configPath, []byte(routerConfigFile()), 0o600))
	require.NoError(t, store.ReloadFromDisk(context.Background()))
	require.Equal(t, "auto", store.Config().ActiveRouter)

	name, r, ok := store.Config().ActiveRouterConfig()
	require.True(t, ok)
	require.Equal(t, "auto", name)
	require.Len(t, r.Routes, 2)
}

func TestSelectingLargeModelClearsRouter(t *testing.T) {
	store, _ := loadRouterStore(t)
	require.NoError(t, store.SetActiveRouter(ScopeGlobal, "auto"))

	// Re-saving the same large model (e.g. a reasoning effort change)
	// keeps routing on.
	same := store.Config().Models[SelectedModelTypeLarge]
	require.NoError(t, store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeLarge, same))
	require.Equal(t, "auto", store.Config().ActiveRouter)

	require.NoError(t, store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeLarge, SelectedModel{
		Provider: "anthropic",
		Model:    "claude-3",
	}))
	require.Empty(t, store.Config().ActiveRouter)

	require.NoError(t, store.ReloadFromDisk(context.Background()))
	require.Empty(t, store.Config().ActiveRouter)
}

func TestOverridePreferredLargeModelClearsRouter(t *testing.T) {
	store, _ := loadRouterStore(t)
	require.NoError(t, store.SetActiveRouter(ScopeGlobal, "auto"))
	store.OverridePreferredModel(SelectedModelTypeLarge, SelectedModel{Provider: "openai", Model: "gpt-4"})
	require.Empty(t, store.Config().ActiveRouter)
}
