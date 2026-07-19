package shellconfig

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func loadScript(t *testing.T, script string) map[string]any {
	t.Helper()
	path := filepath.Join(t.TempDir(), "crushrc")
	jsonBytes, err := LoadShellConfig(path, []byte(script))
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))
	return result
}

func TestModelAdd(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider add openai --api-key k
model add openai/gpt-5.6-sol --name "GPT 5.6 Sol" --context-window 200000 --can-reason true`)

	providers := result["providers"].(map[string]any)
	openai := providers["openai"].(map[string]any)
	models := openai["models"].([]any)
	require.Len(t, models, 1)
	m := models[0].(map[string]any)
	require.Equal(t, "gpt-5.6-sol", m["id"])
	require.Equal(t, "GPT 5.6 Sol", m["name"])
	require.Equal(t, float64(200000), m["context_window"])
	require.Equal(t, true, m["can_reason"])
}

func TestModelAddPricingFlags(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider add anthropic --api-key k
model add anthropic/claude-x --price-input 3 --price-output 15 --price-cache-create 3.75 --price-cache-hit 0.3`)

	model := result["providers"].(map[string]any)["anthropic"].(map[string]any)["models"].([]any)[0].(map[string]any)
	require.Equal(t, 3.0, model["cost_per_1m_in"])
	require.Equal(t, 15.0, model["cost_per_1m_out"])
	require.Equal(t, 3.75, model["cost_per_1m_out_cached"])
	require.Equal(t, 0.3, model["cost_per_1m_in_cached"])
}

func TestModelAddRejectsLegacyPricingFlags(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "crushrc")
	_, err := LoadShellConfig(path, []byte(`provider add openai --api-key k
model add openai/gpt-x --cost-per-1m-in 1`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown flag")
}

func TestModelAddUnknownProvider(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "crushrc")
	_, err := LoadShellConfig(path, []byte(`model add openai/gpt-5.6-sol --name "x"`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not exist")
}

func TestModelAddNoSlash(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "crushrc")
	_, err := LoadShellConfig(path, []byte(`provider add openai --api-key k
model add gpt-5.6-sol --name "x"`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "<provider>/<id>")
}

func TestModelAddSlashInID(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider add openrouter --api-key k
model add openrouter/anthropic/claude --name "Claude via OR"`)

	providers := result["providers"].(map[string]any)
	models := providers["openrouter"].(map[string]any)["models"].([]any)
	require.Equal(t, "anthropic/claude", models[0].(map[string]any)["id"])
}

func TestModelUnset(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider add openai --api-key k
model add openai/a --name "A"
model add openai/b --name "B"
model remove openai/a`)

	models := result["providers"].(map[string]any)["openai"].(map[string]any)["models"].([]any)
	require.Len(t, models, 1)
	require.Equal(t, "b", models[0].(map[string]any)["id"])
}

func TestModelLargeSmall(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `model large openai/gpt-4o --think
model small anthropic/claude-3-5-haiku`)

	models := result["models"].(map[string]any)
	large := models["large"].(map[string]any)
	require.Equal(t, "openai", large["provider"])
	require.Equal(t, "gpt-4o", large["model"])
	require.Equal(t, true, large["think"])

	small := models["small"].(map[string]any)
	require.Equal(t, "anthropic", small["provider"])
	require.Equal(t, "claude-3-5-haiku", small["model"])
}

// TestModelLargePrint verifies that `model large` with no argument prints the
// current selection, capturable via command substitution.
func TestModelLargePrint(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `model large openai/gpt-4o
option data-directory "$(model large)"`)

	require.Equal(t, "openai/gpt-4o", result["options"].(map[string]any)["data_directory"])
}

func TestProviderUnset(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider add openai --api-key k
provider add anthropic --api-key k
provider remove openai`)

	providers := result["providers"].(map[string]any)
	require.NotContains(t, providers, "openai")
	require.Contains(t, providers, "anthropic")
}

// TestRemoveRmAlias verifies that "rm" works as an alias for "remove" on both
// provider and model.
func TestRemoveRmAlias(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider add openai --api-key k
provider add anthropic --api-key k
model add openai/a --name "A"
model add openai/b --name "B"
model rm openai/a
provider rm anthropic`)

	providers := result["providers"].(map[string]any)
	require.NotContains(t, providers, "anthropic")
	models := providers["openai"].(map[string]any)["models"].([]any)
	require.Len(t, models, 1)
	require.Equal(t, "b", models[0].(map[string]any)["id"])
}
