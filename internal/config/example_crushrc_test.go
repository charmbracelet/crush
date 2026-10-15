package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/catwalk/pkg/embedded"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

// Các test dưới đây chạy chính agent-cli.crushrc.example qua config.Load, với
// cấu trúc thư mục như bản cài thật: crushrc nằm ở thư mục cấu hình toàn cục,
// còn token đăng nhập (agent-cli login) nằm ở crush.json trong thư mục dữ liệu
// toàn cục, được gộp SAU crushrc.

const loginToken = "login-access-token"

// loadExampleCrushrc dựng một "máy" sạch (HOME, XDG, CRUSH_GLOBAL_* trỏ vào thư
// mục tạm; mọi biến AGENT_CLI_* và khoá provider quen thuộc bị xoá) rồi nạp
// cấu hình với crushrc mẫu. storedData là nội dung crush.json trong thư mục dữ
// liệu toàn cục (rỗng = chưa đăng nhập gì).
func loadExampleCrushrc(t *testing.T, env map[string]string, storedData string) (*ConfigStore, error) {
	t.Helper()
	script, err := os.ReadFile(filepath.Join("..", "..", "agent-cli.crushrc.example"))
	require.NoError(t, err)

	isolated := t.TempDir()
	cfgDir := filepath.Join(isolated, "config")
	dataGlobal := filepath.Join(isolated, "data")
	require.NoError(t, os.MkdirAll(cfgDir, 0o755))
	require.NoError(t, os.MkdirAll(dataGlobal, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cfgDir, "crushrc"), script, 0o644))
	if storedData != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dataGlobal, "crush.json"), []byte(storedData), 0o600))
	}

	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", cfgDir)
	t.Setenv("CRUSH_GLOBAL_DATA", dataGlobal)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
	t.Setenv("AGENT_CLI_ENV_FILE", filepath.Join(isolated, "no-env-file"))
	for _, k := range []string{
		"AGENT_CLI_PROVIDER_ID", "AGENT_CLI_PROVIDER_NAME", "AGENT_CLI_PROVIDER_TYPE",
		"AGENT_CLI_BASE_URL", "AGENT_CLI_MODEL", "AGENT_CLI_SMALL_MODEL", "AGENT_CLI_API_KEY",
		"AGENT_CLI_CONTEXT_WINDOW", "AGENT_CLI_MAX_TOKENS", "AGENT_CLI_CAN_REASON",
		// Khoá provider của người chạy test không được làm provider "tự cấu hình".
		"OPENAI_API_KEY", "OPENAI_API_ENDPOINT", "ANTHROPIC_API_KEY", "XAI_API_KEY", "ZAI_API_KEY",
	} {
		t.Setenv(k, env[k])
	}

	// Danh mục provider là trạng thái toàn tiến trình: dựng lại từ danh mục nhúng
	// (không chạm mạng) và trả về như cũ cho test khác.
	resetProviderState()
	t.Cleanup(resetProviderState)

	// Không test nào ở đây được phép gọi mạng lấy danh sách model của tài khoản.
	origOpenAI, origGrok := fetchOpenAIModels, fetchGrokModels
	fetchOpenAIModels = func(context.Context, *oauth.Token) ([]catwalk.Model, error) {
		return nil, errors.New("network is off in tests")
	}
	fetchGrokModels = func(context.Context, *oauth.Token) ([]catwalk.Model, error) {
		return nil, errors.New("network is off in tests")
	}
	t.Cleanup(func() { fetchOpenAIModels, fetchGrokModels = origOpenAI, origGrok })

	return Load(t.TempDir(), t.TempDir(), false)
}

// firstEmbeddedModel trả về model đầu tiên của provider dựng sẵn id, để test
// không phụ thuộc một tên model cụ thể trong danh mục.
func firstEmbeddedModel(t *testing.T, id string) string {
	t.Helper()
	for _, p := range embedded.GetAll() {
		if string(p.ID) == id && len(p.Models) > 0 {
			return p.Models[0].ID
		}
	}
	t.Fatalf("embedded catalog has no models for provider %q", id)
	return ""
}

func storedLogin(providerID string) string {
	return fmt.Sprintf(`{"providers":{%q:{"api_key":%q,"oauth":{"access_token":%q,"refresh_token":"login-refresh","expires_in":3600,"expires_at":%d}}}}`,
		providerID, loginToken, loginToken, time.Now().Add(time.Hour).Unix())
}

// ID provider gõ sai (không có trong danh mục, không có base_url) làm khởi động
// dừng và nêu đúng tên, thay vì lặng lẽ chọn model của provider khác.
func TestExampleCrushrc_TypoedProviderIDStopsStartup(t *testing.T) {
	_, err := loadExampleCrushrc(t, map[string]string{
		"AGENT_CLI_PROVIDER_ID": "foo",
		"AGENT_CLI_MODEL":       "some-model",
		"AGENT_CLI_API_KEY":     "test-key",
	}, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), `provider "foo"`)
	require.Contains(t, err.Error(), "large model")
	require.Contains(t, err.Error(), "typo")
}

// Đường provider "main" tự khai bằng AGENT_CLI_BASE_URL phải nạp như cũ.
func TestExampleCrushrc_MainProviderStillLoads(t *testing.T) {
	store, err := loadExampleCrushrc(t, map[string]string{
		"AGENT_CLI_BASE_URL": "http://127.0.0.1:1/v1",
		"AGENT_CLI_MODEL":    "local-model",
	}, "")
	require.NoError(t, err)

	main, ok := store.Config().Providers.Get("main")
	require.True(t, ok, "the self-declared main provider must be configured")
	require.Equal(t, "http://127.0.0.1:1/v1", main.BaseURL)
	large := store.Config().Models[SelectedModelTypeLarge]
	require.Equal(t, "main", large.Provider)
	require.Equal(t, "local-model", large.Model)
	// Ngân sách output mặc định: min(32000, context/4) — 8192 cũ làm model có
	// suy luận bị cắt giữa chừng khi viết tệp lớn.
	require.EqualValues(t, 32000, large.MaxTokens)

	small, err := loadExampleCrushrc(t, map[string]string{
		"AGENT_CLI_BASE_URL":       "http://127.0.0.1:1/v1",
		"AGENT_CLI_MODEL":          "local-model",
		"AGENT_CLI_CONTEXT_WINDOW": "32768",
	}, "")
	require.NoError(t, err)
	require.EqualValues(t, 8192, small.Config().Models[SelectedModelTypeLarge].MaxTokens, "a small context keeps room for the input")
}

// Đã `agent-cli login` thì để trống AGENT_CLI_API_KEY: provider vẫn lên, token
// đăng nhập còn nguyên (crushrc không phát key rỗng đè lên nó).
func TestExampleCrushrc_LoginProviderKeepsStoredToken(t *testing.T) {
	model := firstEmbeddedModel(t, "xai")
	store, err := loadExampleCrushrc(t, map[string]string{
		"AGENT_CLI_PROVIDER_ID": "xai",
		"AGENT_CLI_MODEL":       model,
	}, storedLogin("xai"))
	require.NoError(t, err)

	xai, ok := store.Config().Providers.Get("xai")
	require.True(t, ok, "a logged-in provider must be configured without an API key")
	require.NotNil(t, xai.OAuthToken)
	require.Equal(t, loginToken, xai.OAuthToken.AccessToken)
	require.Equal(t, loginToken, xai.APIKey)
	large := store.Config().Models[SelectedModelTypeLarge]
	require.Equal(t, "xai", large.Provider)
	require.Equal(t, model, large.Model)
}

// Thứ tự nạp: crush.json trong thư mục dữ liệu (nơi login ghi token) nằm SAU
// crushrc, nên khi đã đăng nhập thì token thắng AGENT_CLI_API_KEY. Muốn dùng
// khoá thì `agent-cli logout` trước; docs ghi đúng điều này.
func TestExampleCrushrc_StoredLoginBeatsEnvKey(t *testing.T) {
	store, err := loadExampleCrushrc(t, map[string]string{
		"AGENT_CLI_PROVIDER_ID": "xai",
		"AGENT_CLI_MODEL":       firstEmbeddedModel(t, "xai"),
		"AGENT_CLI_API_KEY":     "env-key",
	}, storedLogin("xai"))
	require.NoError(t, err)

	xai, ok := store.Config().Providers.Get("xai")
	require.True(t, ok)
	require.Equal(t, loginToken, xai.APIKey)
}

// Chưa đăng nhập, chưa có key: khởi động vẫn phải thành công (provider chỉ
// chưa được cấu hình). Nếu không, `agent-cli login openai` cũng không chạy nổi.
func TestExampleCrushrc_LoginProviderNotLoggedInStillStarts(t *testing.T) {
	store, err := loadExampleCrushrc(t, map[string]string{
		"AGENT_CLI_PROVIDER_ID": "openai",
		"AGENT_CLI_MODEL":       firstEmbeddedModel(t, "openai"),
	}, "")
	require.NoError(t, err)

	_, ok := store.Config().Providers.Get("openai")
	require.False(t, ok, "without a login or a key the provider stays unconfigured")
}
