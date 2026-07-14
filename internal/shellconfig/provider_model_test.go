package shellconfig

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProviderModel_Basic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := `provider-model \
  --provider local \
  --id auto \
  --name "Auto" \
  --context-window 128000 \
  --default-max-tokens 8000`
	path := filepath.Join(dir, "crush.sh")

	jsonBytes, err := LoadShellConfig(path, []byte(script))
	require.NoError(t, err)
	require.NotNil(t, jsonBytes)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))

	providers := result["providers"].(map[string]any)
	local := providers["local"].(map[string]any)
	models := local["models"].([]any)
	require.Len(t, models, 1)

	model := models[0].(map[string]any)
	require.Equal(t, "auto", model["id"])
	require.Equal(t, "Auto", model["name"])
	require.EqualValues(t, 128000, model["context_window"])
	require.EqualValues(t, 8000, model["default_max_tokens"])
}

func TestProviderModel_MultipleModels(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := `provider-model --provider local --id auto --name "Auto" --context-window 128000
provider-model --provider local --id fast --name "Fast" --context-window 64000`
	path := filepath.Join(dir, "crush.sh")

	jsonBytes, err := LoadShellConfig(path, []byte(script))
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))

	providers := result["providers"].(map[string]any)
	local := providers["local"].(map[string]any)
	models := local["models"].([]any)
	require.Len(t, models, 2)

	m1 := models[0].(map[string]any)
	require.Equal(t, "auto", m1["id"])
	require.EqualValues(t, 128000, m1["context_window"])

	m2 := models[1].(map[string]any)
	require.Equal(t, "fast", m2["id"])
	require.EqualValues(t, 64000, m2["context_window"])
}

func TestProviderModel_FullProviderWithModels(t *testing.T) {
	dir := t.TempDir()
	script := `provider local \
  --type openai-compat \
  --base-url "https://api.charm.withemissary.com/v1" \
  --api-key "$HYPER_EMISSARY_API_KEY"

provider-model \
  --provider local \
  --id auto \
  --name "Auto" \
  --context-window 128000 \
  --default-max-tokens 8000 \
  --can-reason true \
  --supports-images true \
  --cost-per-1m-in 0.15 \
  --cost-per-1m-out 0.60 \
  --reasoning-effort medium`
	path := filepath.Join(dir, "crush.sh")

	t.Setenv("HYPER_EMISSARY_API_KEY", "test-key")
	jsonBytes, err := LoadShellConfig(path, []byte(script))
	require.NoError(t, err)
	require.NotNil(t, jsonBytes)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))

	providers := result["providers"].(map[string]any)
	local := providers["local"].(map[string]any)
	require.Equal(t, "openai-compat", local["type"])
	require.Equal(t, "https://api.charm.withemissary.com/v1", local["base_url"])
	require.Equal(t, "test-key", local["api_key"])

	models := local["models"].([]any)
	require.Len(t, models, 1)

	model := models[0].(map[string]any)
	require.Equal(t, "auto", model["id"])
	require.Equal(t, "Auto", model["name"])
	require.EqualValues(t, 128000, model["context_window"])
	require.EqualValues(t, 8000, model["default_max_tokens"])
	require.Equal(t, true, model["can_reason"])
	require.Equal(t, true, model["supports_attachments"])
	require.EqualValues(t, 0.15, model["cost_per_1m_in"])
	require.EqualValues(t, 0.60, model["cost_per_1m_out"])
	require.Equal(t, "medium", model["default_reasoning_effort"])
}

func TestProviderModel_MissingProvider(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := `provider-model --id auto --name "Auto"`
	path := filepath.Join(dir, "crush.sh")

	_, err := LoadShellConfig(path, []byte(script))
	require.Error(t, err)
	require.Contains(t, err.Error(), "--provider is required")
}

func TestProviderModel_MissingID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := `provider-model --provider local --name "Auto"`
	path := filepath.Join(dir, "crush.sh")

	_, err := LoadShellConfig(path, []byte(script))
	require.Error(t, err)
	require.Contains(t, err.Error(), "--id is required")
}

func TestProviderModel_UnknownFlag(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := `provider-model --provider local --id auto --bogus "value"`
	path := filepath.Join(dir, "crush.sh")

	_, err := LoadShellConfig(path, []byte(script))
	require.Error(t, err)
}
