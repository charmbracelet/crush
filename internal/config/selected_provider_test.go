package config

import (
	"context"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/stretchr/testify/require"
)

// configureWith dựng Config từ providers và models đã chọn rồi chạy
// configureProviders với catalog đã cho (nil = không có provider dựng sẵn).
func configureWith(t *testing.T, providers map[string]ProviderConfig, models map[SelectedModelType]SelectedModel, known []catwalk.Provider) (*Config, error) {
	t.Helper()
	cfg := &Config{
		Providers: csync.NewMapFrom(providers),
		Models:    models,
	}
	cfg.setDefaults("/tmp", "")

	e := env.NewFromMap(map[string]string{})
	err := cfg.configureProviders(context.Background(), testStore(cfg), e, NewShellVariableResolver(e), known)
	return cfg, err
}

// Gõ sai ID provider dựng sẵn (vd AGENT_CLI_PROVIDER_ID=foo) cho ra một
// provider không có base_url. Nếu nó đang được chọn làm model large/small thì
// khởi động phải dừng và nêu đúng tên provider, thay vì lặng lẽ chọn model khác.
func TestConfigureProviders_SelectedProviderWithoutEndpointFails(t *testing.T) {
	typo := map[string]ProviderConfig{
		"foo": {APIKey: "test-key"},
	}

	t.Run("large", func(t *testing.T) {
		_, err := configureWith(t, typo, map[SelectedModelType]SelectedModel{
			SelectedModelTypeLarge: {Provider: "foo", Model: "m"},
		}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), `provider "foo"`)
		require.Contains(t, err.Error(), "large model")
		require.Contains(t, err.Error(), "no base_url")
	})

	t.Run("small", func(t *testing.T) {
		_, err := configureWith(t, typo, map[SelectedModelType]SelectedModel{
			SelectedModelTypeSmall: {Provider: "foo", Model: "m"},
		}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), `provider "foo"`)
		require.Contains(t, err.Error(), "small model")
	})

	t.Run("base_url that resolves to empty", func(t *testing.T) {
		_, err := configureWith(t, map[string]ProviderConfig{
			"foo": {APIKey: "test-key", BaseURL: "$MISSING_API_URL"},
		}, map[SelectedModelType]SelectedModel{
			SelectedModelTypeLarge: {Provider: "foo", Model: "m"},
		}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), `provider "foo"`)
		require.Contains(t, err.Error(), "resolves to an empty string")
	})

	t.Run("base_url that fails to resolve", func(t *testing.T) {
		_, err := configureWith(t, map[string]ProviderConfig{
			"foo": {APIKey: "test-key", BaseURL: "$(exit 3)"},
		}, map[SelectedModelType]SelectedModel{
			SelectedModelTypeLarge: {Provider: "foo", Model: "m"},
		}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), `provider "foo"`)
		require.Contains(t, err.Error(), "cannot be resolved")
	})
}

// Cấu hình không liên quan phải chạy như cũ: provider không ai chọn mà thiếu
// base_url vẫn chỉ bị cảnh báo và bỏ qua, không chặn khởi động.
func TestConfigureProviders_UnselectedProviderWithoutEndpointStillSkipped(t *testing.T) {
	cfg, err := configureWith(t, map[string]ProviderConfig{
		"unused": {APIKey: "test-key"},
		"main": {
			BaseURL: "http://127.0.0.1:8080/v1",
			Models:  []catwalk.Model{{ID: "m"}},
		},
	}, map[SelectedModelType]SelectedModel{
		SelectedModelTypeLarge: {Provider: "main", Model: "m"},
		SelectedModelTypeSmall: {Provider: "main", Model: "m"},
	}, nil)
	require.NoError(t, err)

	_, unusedKept := cfg.Providers.Get("unused")
	require.False(t, unusedKept, "an unselected provider without base_url is dropped as before")
	_, mainKept := cfg.Providers.Get("main")
	require.True(t, mainKept)
}

// Đường "main" hợp lệ của crushrc mẫu (provider tự khai có base_url và model)
// được chọn làm large/small vẫn nạp bình thường.
func TestConfigureProviders_SelectedCustomProviderWithEndpointLoads(t *testing.T) {
	cfg, err := configureWith(t, map[string]ProviderConfig{
		"main": {
			BaseURL: "http://127.0.0.1:8080/v1",
			Models:  []catwalk.Model{{ID: "m"}},
		},
	}, map[SelectedModelType]SelectedModel{
		SelectedModelTypeLarge: {Provider: "main", Model: "m"},
		SelectedModelTypeSmall: {Provider: "main", Model: "m"},
	}, nil)
	require.NoError(t, err)
	_, ok := cfg.Providers.Get("main")
	require.True(t, ok)
}

// Provider dựng sẵn chưa có thông tin đăng nhập (chưa `agent-cli login`) không
// phải lỗi gõ sai: khởi động phải thành công, nếu không thì không thể chạy lệnh
// login để có thông tin đó.
func TestConfigureProviders_SelectedKnownProviderWithoutCredentialsStillStarts(t *testing.T) {
	known := []catwalk.Provider{{
		ID:          "openai",
		Name:        "OpenAI",
		Type:        catwalk.TypeOpenAI,
		APIEndpoint: "https://api.openai.com/v1",
		APIKey:      "$OPENAI_API_KEY",
		Models:      []catwalk.Model{{ID: "gpt-test"}},
	}}
	cfg, err := configureWith(t, map[string]ProviderConfig{
		"openai": {},
	}, map[SelectedModelType]SelectedModel{
		SelectedModelTypeLarge: {Provider: "openai", Model: "gpt-test"},
	}, known)
	require.NoError(t, err)
	_, ok := cfg.Providers.Get("openai")
	require.False(t, ok, "without credentials the provider stays unconfigured, as before")
}
