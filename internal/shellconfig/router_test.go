package shellconfig

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRouterBuiltin(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `router add auto --classifier anthropic/claude-haiku --default simple --timeout 5
router route auto simple anthropic/claude-haiku --description "Quick questions"
router route auto complex anthropic/claude-opus --description "Hard refactors"
router route auto simple openai/gpt-mini --description "Replaced"
router use auto`)

	require.Equal(t, "auto", result["active_router"])
	r := result["routers"].(map[string]any)["auto"].(map[string]any)
	require.Equal(t, map[string]any{"provider": "anthropic", "model": "claude-haiku"}, r["classifier"])
	require.Equal(t, "simple", r["default"])
	require.Equal(t, float64(5), r["timeout"])
	routes := r["routes"].([]any)
	require.Len(t, routes, 2)
	simple := routes[0].(map[string]any)
	require.Equal(t, "simple", simple["name"])
	require.Equal(t, "Replaced", simple["description"])
	require.Equal(t, map[string]any{"provider": "openai", "model": "gpt-mini"}, simple["model"])
}

func TestRouterRemove(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `router add auto --classifier a/b
router route auto x a/b --description d
router route auto y a/c --description d
router remove auto x
router add other --classifier a/b
router use other
router rm other`)

	routers := result["routers"].(map[string]any)
	require.NotContains(t, routers, "other")
	require.Equal(t, "", result["active_router"])
	routes := routers["auto"].(map[string]any)["routes"].([]any)
	require.Len(t, routes, 1)
	require.Equal(t, "y", routes[0].(map[string]any)["name"])
}

func TestRouterErrors(t *testing.T) {
	t.Parallel()

	for name, script := range map[string]string{
		"unknown router":      `router route nope x a/b --description d`,
		"missing classifier":  `router add auto`,
		"bad classifier":      `router add auto --classifier nope`,
		"missing description": "router add auto --classifier a/b\nrouter route auto x a/b",
		"unknown subcommand":  `router frob`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "crushrc")
			_, err := LoadShellConfig(t.Context(), path, []byte(script))
			require.Error(t, err)
		})
	}
}
