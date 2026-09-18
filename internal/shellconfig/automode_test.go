package shellconfig

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAutoMode_FullSurface(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := `auto_mode on
auto_mode model local-llama/gemma4-e4b
auto_mode max-tokens 2048
auto_mode timeout 90
auto_mode fail-open false
auto_mode environment "Trusted repo: the working repository and its git remotes"
auto_mode prompt stage1 ./prompts/stage1.tmpl
auto_mode transcript-max-chars 12000
auto_mode max-consecutive-denials 3
auto_mode max-total-denials 20`
	path := filepath.Join(dir, "crushrc")

	jsonBytes, err := LoadShellConfig(t.Context(), path, []byte(script))
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))

	am, ok := result["auto_mode"].(map[string]any)
	require.True(t, ok, "auto_mode section should be an object")
	require.Equal(t, true, am["enabled"])
	require.Equal(t, float64(2048), am["max_tokens"])
	require.Equal(t, float64(90), am["timeout_seconds"])
	require.Equal(t, false, am["fail_open"])
	require.Equal(t, "./prompts/stage1.tmpl", am["prompt_stage1_file"])
	require.Equal(t, float64(12000), am["transcript_max_chars"])
	require.Equal(t, float64(3), am["max_consecutive_denials"])
	require.Equal(t, float64(20), am["max_total_denials"])

	env, ok := am["environment"].([]any)
	require.True(t, ok, "environment should be an array")
	require.Len(t, env, 1)
	require.Contains(t, env[0], "Trusted repo")

	classifier, ok := am["classifier"].(map[string]any)
	require.True(t, ok, "classifier should be an object")
	require.Equal(t, "local-llama", classifier["provider"])
	require.Equal(t, "gemma4-e4b", classifier["model"])
}

func TestAutoMode_OffAndErrors(t *testing.T) {
	t.Parallel()

	t.Run("off disables", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "crushrc")
		jsonBytes, err := LoadShellConfig(t.Context(), path, []byte("auto_mode off"))
		require.NoError(t, err)

		var result map[string]any
		require.NoError(t, json.Unmarshal(jsonBytes, &result))
		am := result["auto_mode"].(map[string]any)
		require.Equal(t, false, am["enabled"])
	})

	t.Run("bad model spec errors", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "crushrc")
		_, err := LoadShellConfig(t.Context(), path, []byte("auto_mode model gemma4-e4b"))
		require.Error(t, err, "model without provider/ should be rejected")
	})

	t.Run("bad max-tokens errors", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "crushrc")
		_, err := LoadShellConfig(t.Context(), path, []byte("auto_mode max-tokens notanumber"))
		require.Error(t, err)
	})

	t.Run("bad prompt stage errors", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "crushrc")
		_, err := LoadShellConfig(t.Context(), path, []byte("auto_mode prompt stage3 ./x"))
		require.Error(t, err)
	})
}
