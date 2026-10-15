package shellconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// runExampleCrushrc chạy chính tệp agent-cli.crushrc.example ở gốc repo với env
// cho sẵn. Mọi biến AGENT_CLI_* khác bị xoá và tệp env trỏ tới chỗ không tồn
// tại, để test không bao giờ đọc cấu hình (và khoá) thật của người chạy.
func runExampleCrushrc(t *testing.T, env map[string]string) (map[string]any, error) {
	t.Helper()
	script, err := os.ReadFile(filepath.Join("..", "..", "agent-cli.crushrc.example"))
	require.NoError(t, err)

	t.Setenv("AGENT_CLI_ENV_FILE", filepath.Join(t.TempDir(), "no-env-file"))
	for _, k := range []string{
		"AGENT_CLI_PROVIDER_ID", "AGENT_CLI_PROVIDER_NAME", "AGENT_CLI_PROVIDER_TYPE",
		"AGENT_CLI_BASE_URL", "AGENT_CLI_MODEL", "AGENT_CLI_SMALL_MODEL", "AGENT_CLI_API_KEY",
		"AGENT_CLI_CONTEXT_WINDOW", "AGENT_CLI_MAX_TOKENS", "AGENT_CLI_CAN_REASON",
	} {
		t.Setenv(k, env[k])
	}

	out, err := LoadShellConfig(t.Context(), filepath.Join(t.TempDir(), "crushrc"), script)
	if err != nil {
		return nil, err
	}
	var cfg map[string]any
	require.NoError(t, json.Unmarshal(out, &cfg))
	return cfg, nil
}

// Provider dựng sẵn (vd zai cho GLM): chỉ ghi khoá, model chọn theo ID
// đó, alt+p liệt kê provider này đầu tiên.
func TestExampleCrushrc_CatalogProvider(t *testing.T) {
	cfg, err := runExampleCrushrc(t, map[string]string{
		"AGENT_CLI_PROVIDER_ID": "zai",
		"AGENT_CLI_MODEL":       "glm-5.3",
		"AGENT_CLI_SMALL_MODEL": "glm-5.3-flash",
		"AGENT_CLI_API_KEY":     "test-key",
	})
	require.NoError(t, err)

	providers := cfg["providers"].(map[string]any)
	require.NotContains(t, providers, "main")
	zai := providers["zai"].(map[string]any)
	require.Equal(t, "test-key", zai["api_key"])
	require.NotContains(t, zai, "name", "the catalog supplies the name")
	require.NotContains(t, zai, "base_url", "the catalog supplies the endpoint")

	models := cfg["models"].(map[string]any)
	require.Equal(t, "zai", models["large"].(map[string]any)["provider"])
	require.Equal(t, "glm-5.3", models["large"].(map[string]any)["model"])
	require.Equal(t, "glm-5.3-flash", models["small"].(map[string]any)["model"])

	opts := cfg["options"].(map[string]any)
	require.Equal(t, []any{"zai", "openai", "anthropic"}, opts["visible_providers"])
}

// Endpoint tự khai (không đặt AGENT_CLI_PROVIDER_ID): provider "main" như cũ.
func TestExampleCrushrc_CustomEndpoint(t *testing.T) {
	cfg, err := runExampleCrushrc(t, map[string]string{
		"AGENT_CLI_BASE_URL":   "http://127.0.0.1:8080/v1",
		"AGENT_CLI_MODEL":      "local-model",
		"AGENT_CLI_CAN_REASON": "true",
	})
	require.NoError(t, err)

	main := cfg["providers"].(map[string]any)["main"].(map[string]any)
	require.Equal(t, "http://127.0.0.1:8080/v1", main["base_url"])
	require.Equal(t, "main", cfg["models"].(map[string]any)["large"].(map[string]any)["provider"])
	require.Equal(t, []any{"main", "openai", "anthropic"}, cfg["options"].(map[string]any)["visible_providers"])
}

// Thiếu biến bắt buộc thì crushrc dừng (cả hai cách khai).
func TestExampleCrushrc_MissingRequired(t *testing.T) {
	_, err := runExampleCrushrc(t, map[string]string{})
	require.Error(t, err)
	_, err = runExampleCrushrc(t, map[string]string{"AGENT_CLI_PROVIDER_ID": "zai", "AGENT_CLI_MODEL": "glm-5.3"})
	require.Error(t, err, "a catalog provider still needs an API key")
}
