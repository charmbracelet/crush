package shellconfig

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// loadScriptErr runs a crushrc script expecting it to fail.
func loadScriptErr(t *testing.T, script string) error {
	t.Helper()
	path := filepath.Join(t.TempDir(), "crushrc")
	_, err := LoadShellConfig(t.Context(), path, []byte(script))
	return err
}

// honchoSection returns the honcho block from a loaded script.
func honchoSection(t *testing.T, script string) map[string]any {
	t.Helper()
	result := loadScript(t, script)
	section, ok := result["honcho"].(map[string]any)
	require.True(t, ok, "expected a honcho section, got %v", result)
	return section
}

func TestHonchoEnabled(t *testing.T) {
	t.Parallel()

	h := honchoSection(t, `honcho enabled`)
	require.Equal(t, true, h["enabled"])
}

func TestHonchoAnyDirectiveImpliesEnabled(t *testing.T) {
	t.Parallel()

	// Setting a workspace is an unambiguous statement of intent, so
	// the user should not have to also write "honcho enabled".
	h := honchoSection(t, `honcho workspace my-project`)
	require.Equal(t, "my-project", h["workspace"])
	require.Equal(t, true, h["enabled"], "a honcho directive should turn memory on")
}

func TestHonchoExplicitDisableWins(t *testing.T) {
	t.Parallel()

	// Directives are applied in execution order, so an explicit
	// disable written after other settings must survive.
	h := honchoSection(t, `honcho workspace my-project
honcho enabled false`)
	require.Equal(t, false, h["enabled"])
}

func TestHonchoStringKeys(t *testing.T) {
	t.Parallel()

	h := honchoSection(t, `honcho api-key secret-value
honcho base-url http://127.0.0.1:8000
honcho peer-name kieran
honcho agent-peer crush-dev`)

	require.Equal(t, "secret-value", h["api_key"])
	require.Equal(t, "http://127.0.0.1:8000", h["base_url"])
	require.Equal(t, "kieran", h["peer_name"])
	require.Equal(t, "crush-dev", h["agent_peer"])
}

func TestHonchoBooleanKeys(t *testing.T) {
	t.Parallel()

	h := honchoSection(t, `honcho agent-observe-me
honcho capture-tools false`)

	require.Equal(t, true, h["agent_observe_me"], "a bare boolean key means true")
	require.Equal(t, false, h["capture_tools"])
}

func TestHonchoIntKeys(t *testing.T) {
	t.Parallel()

	h := honchoSection(t, `honcho max-conclusions 12
honcho context-tokens 4000`)

	require.Equal(t, float64(12), h["max_conclusions"])
	require.Equal(t, float64(4000), h["context_tokens"])
}

func TestHonchoEnumKeys(t *testing.T) {
	t.Parallel()

	h := honchoSection(t, `honcho recall-mode tools
honcho observation-mode directional
honcho session-strategy per-repo`)

	require.Equal(t, "tools", h["recall_mode"])
	require.Equal(t, "directional", h["observation_mode"])
	require.Equal(t, "per-repo", h["session_strategy"])
}

func TestHonchoRejectsBadValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		script string
	}{
		{"unknown key", `honcho nonsense value`},
		{"no key", `honcho`},
		{"bad recall mode", `honcho recall-mode telepathy`},
		{"bad observation mode", `honcho observation-mode sideways`},
		{"bad session strategy", `honcho session-strategy per-vibe`},
		{"bad boolean", `honcho capture-tools maybe`},
		{"non-numeric int", `honcho max-conclusions lots`},
		{"negative int", `honcho context-tokens -5`},
		{"string key without value", `honcho workspace`},
		{"int key without value", `honcho max-conclusions`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Error(t, loadScriptErr(t, tc.script))
		})
	}
}

func TestHonchoLastWriteWins(t *testing.T) {
	t.Parallel()

	h := honchoSection(t, `honcho workspace first
honcho workspace second`)
	require.Equal(t, "second", h["workspace"])
}

func TestHonchoAbsentWithoutDirectives(t *testing.T) {
	t.Parallel()

	// No honcho directives means no honcho section, which is what
	// keeps the integration off by default.
	result := loadScript(t, `option debug true`)
	require.NotContains(t, result, "honcho")
}
