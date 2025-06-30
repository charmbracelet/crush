package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/charmbracelet/crush/internal/fur/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reset() {
	// Clear all environment variables that could affect config
	envVarsToUnset := []string{
		// API Keys
		"ANTHROPIC_API_KEY",
		"OPENAI_API_KEY",
		"GEMINI_API_KEY",
		"XAI_API_KEY",
		"OPENROUTER_API_KEY",

		// Google Cloud / VertexAI
		"GOOGLE_GENAI_USE_VERTEXAI",
		"GOOGLE_CLOUD_PROJECT",
		"GOOGLE_CLOUD_LOCATION",

		// AWS Credentials
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_REGION",
		"AWS_DEFAULT_REGION",
		"AWS_PROFILE",
		"AWS_DEFAULT_PROFILE",
		"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI",
		"AWS_CONTAINER_CREDENTIALS_FULL_URI",

		// Other
		"CRUSH_DEV_DEBUG",
	}

	for _, envVar := range envVarsToUnset {
		os.Unsetenv(envVar)
	}

	// Reset singleton
	once = sync.Once{}
	instance = nil
	cwd = ""
	testConfigDir = ""

	// Enable mock providers for all tests to avoid API calls
	UseMockProviders = true
	ResetProviders()
}

// Core Configuration Loading Tests

func TestInit_ValidWorkingDirectory(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, cwdDir, WorkingDirectory())
	assert.Equal(t, defaultDataDirectory, cfg.Options.DataDirectory)
	assert.Equal(t, defaultContextPaths, cfg.Options.ContextPaths)
}

func TestInit_WithDebugFlag(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	cfg, err := Init(cwdDir, true)

	require.NoError(t, err)
	assert.True(t, cfg.Options.Debug)
}

func TestInit_SingletonBehavior(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	cfg1, err1 := Init(cwdDir, false)
	cfg2, err2 := Init(cwdDir, false)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Same(t, cfg1, cfg2)
}

func TestGet_BeforeInitialization(t *testing.T) {
	reset()

	assert.Panics(t, func() {
		Get()
	})
}

func TestGet_AfterInitialization(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	cfg1, err := Init(cwdDir, false)
	require.NoError(t, err)

	cfg2 := Get()
	assert.Same(t, cfg1, cfg2)
}

func TestLoadConfig_NoConfigFiles(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.Len(t, cfg.Providers, 0)
	assert.Equal(t, defaultContextPaths, cfg.Options.ContextPaths)
}

func TestLoadConfig_OnlyGlobalConfig(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	globalConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:                provider.InferenceProviderOpenAI,
				APIKey:            "test-key",
				ProviderType:      provider.TypeOpenAI,
				DefaultLargeModel: "gpt-4",
				DefaultSmallModel: "gpt-3.5-turbo",
				Models: []Model{
					{
						ID:               "gpt-4",
						Name:             "GPT-4",
						CostPer1MIn:      30.0,
						CostPer1MOut:     60.0,
						ContextWindow:    8192,
						DefaultMaxTokens: 4096,
					},
					{
						ID:               "gpt-3.5-turbo",
						Name:             "GPT-3.5 Turbo",
						CostPer1MIn:      1.0,
						CostPer1MOut:     2.0,
						ContextWindow:    4096,
						DefaultMaxTokens: 4096,
					},
				},
			},
		},
		Options: Options{
			ContextPaths: []string{"custom-context.md"},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))

	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.Len(t, cfg.Providers, 1)
	assert.Contains(t, cfg.Providers, provider.InferenceProviderOpenAI)
	assert.Contains(t, cfg.Options.ContextPaths, "custom-context.md")
}

func TestLoadConfig_OnlyLocalConfig(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	localConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderAnthropic: {
				ID:                provider.InferenceProviderAnthropic,
				APIKey:            "local-key",
				ProviderType:      provider.TypeAnthropic,
				DefaultLargeModel: "claude-3-opus",
				DefaultSmallModel: "claude-3-haiku",
				Models: []Model{
					{
						ID:               "claude-3-opus",
						Name:             "Claude 3 Opus",
						CostPer1MIn:      15.0,
						CostPer1MOut:     75.0,
						ContextWindow:    200000,
						DefaultMaxTokens: 4096,
					},
					{
						ID:               "claude-3-haiku",
						Name:             "Claude 3 Haiku",
						CostPer1MIn:      0.25,
						CostPer1MOut:     1.25,
						ContextWindow:    200000,
						DefaultMaxTokens: 4096,
					},
				},
			},
		},
		Options: Options{
			TUI: TUIOptions{CompactMode: true},
		},
	}

	localConfigPath := filepath.Join(cwdDir, "crush.json")
	data, err := json.Marshal(localConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(localConfigPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.Len(t, cfg.Providers, 1)
	assert.Contains(t, cfg.Providers, provider.InferenceProviderAnthropic)
	assert.True(t, cfg.Options.TUI.CompactMode)
}

func TestLoadConfig_BothGlobalAndLocal(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	globalConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:                provider.InferenceProviderOpenAI,
				APIKey:            "global-key",
				ProviderType:      provider.TypeOpenAI,
				DefaultLargeModel: "gpt-4",
				DefaultSmallModel: "gpt-3.5-turbo",
				Models: []Model{
					{
						ID:               "gpt-4",
						Name:             "GPT-4",
						CostPer1MIn:      30.0,
						CostPer1MOut:     60.0,
						ContextWindow:    8192,
						DefaultMaxTokens: 4096,
					},
					{
						ID:               "gpt-3.5-turbo",
						Name:             "GPT-3.5 Turbo",
						CostPer1MIn:      1.0,
						CostPer1MOut:     2.0,
						ContextWindow:    4096,
						DefaultMaxTokens: 4096,
					},
				},
			},
		},
		Options: Options{
			ContextPaths: []string{"global-context.md"},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	localConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				APIKey: "local-key", // Override global
			},
			provider.InferenceProviderAnthropic: {
				ID:                provider.InferenceProviderAnthropic,
				APIKey:            "anthropic-key",
				ProviderType:      provider.TypeAnthropic,
				DefaultLargeModel: "claude-3-opus",
				DefaultSmallModel: "claude-3-haiku",
				Models: []Model{
					{
						ID:               "claude-3-opus",
						Name:             "Claude 3 Opus",
						CostPer1MIn:      15.0,
						CostPer1MOut:     75.0,
						ContextWindow:    200000,
						DefaultMaxTokens: 4096,
					},
					{
						ID:               "claude-3-haiku",
						Name:             "Claude 3 Haiku",
						CostPer1MIn:      0.25,
						CostPer1MOut:     1.25,
						ContextWindow:    200000,
						DefaultMaxTokens: 4096,
					},
				},
			},
		},
		Options: Options{
			ContextPaths: []string{"local-context.md"},
			TUI:          TUIOptions{CompactMode: true},
		},
	}

	localConfigPath := filepath.Join(cwdDir, "crush.json")
	data, err = json.Marshal(localConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(localConfigPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.Len(t, cfg.Providers, 2)

	openaiProvider := cfg.Providers[provider.InferenceProviderOpenAI]
	assert.Equal(t, "local-key", openaiProvider.APIKey)

	assert.Contains(t, cfg.Providers, provider.InferenceProviderAnthropic)

	assert.Contains(t, cfg.Options.ContextPaths, "global-context.md")
	assert.Contains(t, cfg.Options.ContextPaths, "local-context.md")
	assert.True(t, cfg.Options.TUI.CompactMode)
}

func TestLoadConfig_MalformedGlobalJSON(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`{invalid json`), 0o644))

	_, err := Init(cwdDir, false)
	assert.Error(t, err)
}

func TestLoadConfig_MalformedLocalJSON(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	localConfigPath := filepath.Join(cwdDir, "crush.json")
	require.NoError(t, os.WriteFile(localConfigPath, []byte(`{invalid json`), 0o644))

	_, err := Init(cwdDir, false)
	assert.Error(t, err)
}

func TestConfigWithoutEnv(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	cfg, _ := Init(cwdDir, false)
	assert.Len(t, cfg.Providers, 0)
}

func TestConfigWithEnv(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("GEMINI_API_KEY", "test-gemini-key")
	os.Setenv("XAI_API_KEY", "test-xai-key")
	os.Setenv("OPENROUTER_API_KEY", "test-openrouter-key")

	cfg, _ := Init(cwdDir, false)
	assert.Len(t, cfg.Providers, 5)
}

// Environment Variable Tests

func TestEnvVars_NoEnvironmentVariables(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.Len(t, cfg.Providers, 0)
}

func TestEnvVars_AllSupportedAPIKeys(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("GEMINI_API_KEY", "test-gemini-key")
	os.Setenv("XAI_API_KEY", "test-xai-key")
	os.Setenv("OPENROUTER_API_KEY", "test-openrouter-key")

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.Len(t, cfg.Providers, 5)

	anthropicProvider := cfg.Providers[provider.InferenceProviderAnthropic]
	assert.Equal(t, "test-anthropic-key", anthropicProvider.APIKey)
	assert.Equal(t, provider.TypeAnthropic, anthropicProvider.ProviderType)

	openaiProvider := cfg.Providers[provider.InferenceProviderOpenAI]
	assert.Equal(t, "test-openai-key", openaiProvider.APIKey)
	assert.Equal(t, provider.TypeOpenAI, openaiProvider.ProviderType)

	geminiProvider := cfg.Providers[provider.InferenceProviderGemini]
	assert.Equal(t, "test-gemini-key", geminiProvider.APIKey)
	assert.Equal(t, provider.TypeGemini, geminiProvider.ProviderType)

	xaiProvider := cfg.Providers[provider.InferenceProviderXAI]
	assert.Equal(t, "test-xai-key", xaiProvider.APIKey)
	assert.Equal(t, provider.TypeXAI, xaiProvider.ProviderType)

	openrouterProvider := cfg.Providers[provider.InferenceProviderOpenRouter]
	assert.Equal(t, "test-openrouter-key", openrouterProvider.APIKey)
	assert.Equal(t, provider.TypeOpenAI, openrouterProvider.ProviderType)
	assert.Equal(t, "https://openrouter.ai/api/v1", openrouterProvider.BaseURL)
}

func TestEnvVars_PartialEnvironmentVariables(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	os.Setenv("OPENAI_API_KEY", "test-openai-key")

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.Len(t, cfg.Providers, 2)
	assert.Contains(t, cfg.Providers, provider.InferenceProviderAnthropic)
	assert.Contains(t, cfg.Providers, provider.InferenceProviderOpenAI)
	assert.NotContains(t, cfg.Providers, provider.InferenceProviderGemini)
}

func TestEnvVars_VertexAIConfiguration(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("GOOGLE_GENAI_USE_VERTEXAI", "true")
	os.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")
	os.Setenv("GOOGLE_CLOUD_LOCATION", "us-central1")

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.Contains(t, cfg.Providers, provider.InferenceProviderVertexAI)

	vertexProvider := cfg.Providers[provider.InferenceProviderVertexAI]
	assert.Equal(t, provider.TypeVertexAI, vertexProvider.ProviderType)
	assert.Equal(t, "test-project", vertexProvider.ExtraParams["project"])
	assert.Equal(t, "us-central1", vertexProvider.ExtraParams["location"])
}

func TestEnvVars_VertexAIWithoutUseFlag(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")
	os.Setenv("GOOGLE_CLOUD_LOCATION", "us-central1")

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.NotContains(t, cfg.Providers, provider.InferenceProviderVertexAI)
}

func TestEnvVars_AWSBedrockWithAccessKeys(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")
	os.Setenv("AWS_DEFAULT_REGION", "us-east-1")

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.Contains(t, cfg.Providers, provider.InferenceProviderBedrock)

	bedrockProvider := cfg.Providers[provider.InferenceProviderBedrock]
	assert.Equal(t, provider.TypeBedrock, bedrockProvider.ProviderType)
	assert.Equal(t, "us-east-1", bedrockProvider.ExtraParams["region"])
}

func TestEnvVars_AWSBedrockWithProfile(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("AWS_PROFILE", "test-profile")
	os.Setenv("AWS_REGION", "eu-west-1")

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.Contains(t, cfg.Providers, provider.InferenceProviderBedrock)

	bedrockProvider := cfg.Providers[provider.InferenceProviderBedrock]
	assert.Equal(t, "eu-west-1", bedrockProvider.ExtraParams["region"])
}

func TestEnvVars_AWSBedrockWithContainerCredentials(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI", "/v2/credentials/test")
	os.Setenv("AWS_DEFAULT_REGION", "ap-southeast-1")

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.Contains(t, cfg.Providers, provider.InferenceProviderBedrock)
}

func TestEnvVars_AWSBedrockRegionPriority(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	os.Setenv("AWS_DEFAULT_REGION", "us-west-2")
	os.Setenv("AWS_REGION", "us-east-1")

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	bedrockProvider := cfg.Providers[provider.InferenceProviderBedrock]
	assert.Equal(t, "us-west-2", bedrockProvider.ExtraParams["region"])
}

func TestEnvVars_AWSBedrockFallbackRegion(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	os.Setenv("AWS_REGION", "us-east-1")

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	bedrockProvider := cfg.Providers[provider.InferenceProviderBedrock]
	assert.Equal(t, "us-east-1", bedrockProvider.ExtraParams["region"])
}

func TestEnvVars_NoAWSCredentials(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.NotContains(t, cfg.Providers, provider.InferenceProviderBedrock)
}

func TestEnvVars_CustomEnvironmentVariables(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "resolved-anthropic-key")

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	if len(cfg.Providers) > 0 {
		if anthropicProvider, exists := cfg.Providers[provider.InferenceProviderAnthropic]; exists {
			assert.Equal(t, "resolved-anthropic-key", anthropicProvider.APIKey)
		}
	}
}

func TestEnvVars_CombinedEnvironmentVariables(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic")
	os.Setenv("OPENAI_API_KEY", "test-openai")
	os.Setenv("GOOGLE_GENAI_USE_VERTEXAI", "true")
	os.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")
	os.Setenv("GOOGLE_CLOUD_LOCATION", "us-central1")
	os.Setenv("AWS_ACCESS_KEY_ID", "test-aws-key")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test-aws-secret")
	os.Setenv("AWS_DEFAULT_REGION", "us-west-1")

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)

	expectedProviders := []provider.InferenceProvider{
		provider.InferenceProviderAnthropic,
		provider.InferenceProviderOpenAI,
		provider.InferenceProviderVertexAI,
		provider.InferenceProviderBedrock,
	}

	for _, expectedProvider := range expectedProviders {
		assert.Contains(t, cfg.Providers, expectedProvider)
	}
}

func TestHasAWSCredentials_AccessKeys(t *testing.T) {
	reset()

	os.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")

	assert.True(t, hasAWSCredentials())
}

func TestHasAWSCredentials_Profile(t *testing.T) {
	reset()

	os.Setenv("AWS_PROFILE", "test-profile")

	assert.True(t, hasAWSCredentials())
}

func TestHasAWSCredentials_DefaultProfile(t *testing.T) {
	reset()

	os.Setenv("AWS_DEFAULT_PROFILE", "default")

	assert.True(t, hasAWSCredentials())
}

func TestHasAWSCredentials_Region(t *testing.T) {
	reset()

	os.Setenv("AWS_REGION", "us-east-1")

	assert.True(t, hasAWSCredentials())
}

func TestHasAWSCredentials_ContainerCredentials(t *testing.T) {
	reset()

	os.Setenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI", "/v2/credentials/test")

	assert.True(t, hasAWSCredentials())
}

func TestHasAWSCredentials_NoCredentials(t *testing.T) {
	reset()

	assert.False(t, hasAWSCredentials())
}

func TestProviderMerging_GlobalToBase(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	globalConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:                provider.InferenceProviderOpenAI,
				APIKey:            "global-openai-key",
				ProviderType:      provider.TypeOpenAI,
				DefaultLargeModel: "gpt-4",
				DefaultSmallModel: "gpt-3.5-turbo",
				Models: []Model{
					{
						ID:               "gpt-4",
						Name:             "GPT-4",
						ContextWindow:    8192,
						DefaultMaxTokens: 4096,
					},
					{
						ID:               "gpt-3.5-turbo",
						Name:             "GPT-3.5 Turbo",
						ContextWindow:    4096,
						DefaultMaxTokens: 2048,
					},
				},
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.Len(t, cfg.Providers, 1)

	openaiProvider := cfg.Providers[provider.InferenceProviderOpenAI]
	assert.Equal(t, "global-openai-key", openaiProvider.APIKey)
	assert.Equal(t, "gpt-4", openaiProvider.DefaultLargeModel)
	assert.Equal(t, "gpt-3.5-turbo", openaiProvider.DefaultSmallModel)
	assert.Len(t, openaiProvider.Models, 2)
}

func TestProviderMerging_LocalToBase(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	localConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderAnthropic: {
				ID:                provider.InferenceProviderAnthropic,
				APIKey:            "local-anthropic-key",
				ProviderType:      provider.TypeAnthropic,
				DefaultLargeModel: "claude-3-opus",
				DefaultSmallModel: "claude-3-haiku",
				Models: []Model{
					{
						ID:               "claude-3-opus",
						Name:             "Claude 3 Opus",
						ContextWindow:    200000,
						DefaultMaxTokens: 4096,
						CostPer1MIn:      15.0,
						CostPer1MOut:     75.0,
					},
					{
						ID:               "claude-3-haiku",
						Name:             "Claude 3 Haiku",
						ContextWindow:    200000,
						DefaultMaxTokens: 4096,
						CostPer1MIn:      0.25,
						CostPer1MOut:     1.25,
					},
				},
			},
		},
	}

	localConfigPath := filepath.Join(cwdDir, "crush.json")
	data, err := json.Marshal(localConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(localConfigPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.Len(t, cfg.Providers, 1)

	anthropicProvider := cfg.Providers[provider.InferenceProviderAnthropic]
	assert.Equal(t, "local-anthropic-key", anthropicProvider.APIKey)
	assert.Equal(t, "claude-3-opus", anthropicProvider.DefaultLargeModel)
	assert.Equal(t, "claude-3-haiku", anthropicProvider.DefaultSmallModel)
	assert.Len(t, anthropicProvider.Models, 2)
}

func TestProviderMerging_ConflictingSettings(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	globalConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:                provider.InferenceProviderOpenAI,
				APIKey:            "global-key",
				ProviderType:      provider.TypeOpenAI,
				DefaultLargeModel: "gpt-4",
				DefaultSmallModel: "gpt-3.5-turbo",
				Models: []Model{
					{
						ID:               "gpt-4",
						Name:             "GPT-4",
						ContextWindow:    8192,
						DefaultMaxTokens: 4096,
					},
					{
						ID:               "gpt-3.5-turbo",
						Name:             "GPT-3.5 Turbo",
						ContextWindow:    4096,
						DefaultMaxTokens: 2048,
					},
					{
						ID:               "gpt-4-turbo",
						Name:             "GPT-4 Turbo",
						ContextWindow:    128000,
						DefaultMaxTokens: 4096,
					},
				},
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	// Create local config that overrides
	localConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				APIKey:            "local-key",
				DefaultLargeModel: "gpt-4-turbo",
			},
		},
	}

	localConfigPath := filepath.Join(cwdDir, "crush.json")
	data, err = json.Marshal(localConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(localConfigPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)

	openaiProvider := cfg.Providers[provider.InferenceProviderOpenAI]
	assert.Equal(t, "local-key", openaiProvider.APIKey)
	assert.Equal(t, "gpt-4-turbo", openaiProvider.DefaultLargeModel)
	assert.False(t, openaiProvider.Disabled)
	assert.Equal(t, "gpt-3.5-turbo", openaiProvider.DefaultSmallModel)
}

func TestProviderMerging_CustomVsKnownProviders(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	customProviderID := provider.InferenceProvider("custom-provider")

	globalConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:                provider.InferenceProviderOpenAI,
				APIKey:            "openai-key",
				BaseURL:           "should-not-override",
				ProviderType:      provider.TypeAnthropic,
				DefaultLargeModel: "gpt-4",
				DefaultSmallModel: "gpt-3.5-turbo",
				Models: []Model{
					{
						ID:               "gpt-4",
						Name:             "GPT-4",
						ContextWindow:    8192,
						DefaultMaxTokens: 4096,
					},
					{
						ID:               "gpt-3.5-turbo",
						Name:             "GPT-3.5 Turbo",
						ContextWindow:    4096,
						DefaultMaxTokens: 2048,
					},
				},
			},
			customProviderID: {
				ID:                customProviderID,
				APIKey:            "custom-key",
				BaseURL:           "https://custom.api.com",
				ProviderType:      provider.TypeOpenAI,
				DefaultLargeModel: "custom-large",
				DefaultSmallModel: "custom-small",
				Models: []Model{
					{
						ID:               "custom-large",
						Name:             "Custom Large",
						ContextWindow:    8192,
						DefaultMaxTokens: 4096,
					},
					{
						ID:               "custom-small",
						Name:             "Custom Small",
						ContextWindow:    4096,
						DefaultMaxTokens: 2048,
					},
				},
			},
		},
	}

	localConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				BaseURL:      "https://should-not-change.com",
				ProviderType: provider.TypeGemini, // Should not change
			},
			customProviderID: {
				BaseURL:      "https://updated-custom.api.com",
				ProviderType: provider.TypeOpenAI,
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	localConfigPath := filepath.Join(cwdDir, "crush.json")
	data, err = json.Marshal(localConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(localConfigPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)

	openaiProvider := cfg.Providers[provider.InferenceProviderOpenAI]
	assert.NotEqual(t, "https://should-not-change.com", openaiProvider.BaseURL)
	assert.NotEqual(t, provider.TypeGemini, openaiProvider.ProviderType)

	customProvider := cfg.Providers[customProviderID]
	assert.Equal(t, "custom-key", customProvider.APIKey)
	assert.Equal(t, "https://updated-custom.api.com", customProvider.BaseURL)
	assert.Equal(t, provider.TypeOpenAI, customProvider.ProviderType)
}

func TestProviderValidation_CustomProviderMissingBaseURL(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	customProviderID := provider.InferenceProvider("custom-provider")

	globalConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			customProviderID: {
				ID:           customProviderID,
				APIKey:       "custom-key",
				ProviderType: provider.TypeOpenAI,
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.NotContains(t, cfg.Providers, customProviderID)
}

func TestProviderValidation_CustomProviderMissingAPIKey(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	customProviderID := provider.InferenceProvider("custom-provider")

	globalConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			customProviderID: {
				ID:           customProviderID,
				BaseURL:      "https://custom.api.com",
				ProviderType: provider.TypeOpenAI,
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.NotContains(t, cfg.Providers, customProviderID)
}

func TestProviderValidation_CustomProviderInvalidType(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	customProviderID := provider.InferenceProvider("custom-provider")

	globalConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			customProviderID: {
				ID:           customProviderID,
				APIKey:       "custom-key",
				BaseURL:      "https://custom.api.com",
				ProviderType: provider.Type("invalid-type"),
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.NotContains(t, cfg.Providers, customProviderID)
}

func TestProviderValidation_KnownProviderValid(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	globalConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:                provider.InferenceProviderOpenAI,
				APIKey:            "openai-key",
				ProviderType:      provider.TypeOpenAI,
				DefaultLargeModel: "gpt-4",
				DefaultSmallModel: "gpt-3.5-turbo",
				Models: []Model{
					{
						ID:               "gpt-4",
						Name:             "GPT-4",
						ContextWindow:    8192,
						DefaultMaxTokens: 4096,
					},
					{
						ID:               "gpt-3.5-turbo",
						Name:             "GPT-3.5 Turbo",
						ContextWindow:    4096,
						DefaultMaxTokens: 2048,
					},
				},

			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.Contains(t, cfg.Providers, provider.InferenceProviderOpenAI)
}

func TestProviderValidation_DisabledProvider(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	globalConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:                provider.InferenceProviderOpenAI,
				APIKey:            "openai-key",
				ProviderType:      provider.TypeOpenAI,
				Disabled:          true,
				DefaultLargeModel: "gpt-4",
				DefaultSmallModel: "gpt-3.5-turbo",
				Models: []Model{
					{
						ID:               "gpt-4",
						Name:             "GPT-4",
						ContextWindow:    8192,
						DefaultMaxTokens: 4096,
					},
					{
						ID:               "gpt-3.5-turbo",
						Name:             "GPT-3.5 Turbo",
						ContextWindow:    4096,
						DefaultMaxTokens: 2048,
					},
				},
			},
			provider.InferenceProviderAnthropic: {
				ID:                provider.InferenceProviderAnthropic,
				APIKey:            "anthropic-key",
				ProviderType:      provider.TypeAnthropic,
				Disabled:          false, // This one is enabled
				DefaultLargeModel: "claude-3-opus",
				DefaultSmallModel: "claude-3-haiku",
				Models: []Model{
					{
						ID:               "claude-3-opus",
						Name:             "Claude 3 Opus",
						ContextWindow:    200000,
						DefaultMaxTokens: 4096,
					},
					{
						ID:               "claude-3-haiku",
						Name:             "Claude 3 Haiku",
						ContextWindow:    200000,
						DefaultMaxTokens: 4096,
					},
				},
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.Contains(t, cfg.Providers, provider.InferenceProviderOpenAI)
	assert.True(t, cfg.Providers[provider.InferenceProviderOpenAI].Disabled)
	assert.Contains(t, cfg.Providers, provider.InferenceProviderAnthropic)
	assert.False(t, cfg.Providers[provider.InferenceProviderAnthropic].Disabled)
}

func TestProviderModels_AddingNewModels(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	globalConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:                provider.InferenceProviderOpenAI,
				APIKey:            "openai-key",
				ProviderType:      provider.TypeOpenAI,
				DefaultLargeModel: "gpt-4",
				DefaultSmallModel: "gpt-4-turbo",
				Models: []Model{
					{
						ID:               "gpt-4",
						Name:             "GPT-4",
						ContextWindow:    8192,
						DefaultMaxTokens: 4096,
					},
				},
			},
		},
	}

	localConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				Models: []Model{
					{
						ID:               "gpt-4-turbo",
						Name:             "GPT-4 Turbo",
						ContextWindow:    128000,
						DefaultMaxTokens: 4096,
					},
				},
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	localConfigPath := filepath.Join(cwdDir, "crush.json")
	data, err = json.Marshal(localConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(localConfigPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)

	openaiProvider := cfg.Providers[provider.InferenceProviderOpenAI]
	assert.Len(t, openaiProvider.Models, 2)

	modelIDs := make([]string, len(openaiProvider.Models))
	for i, model := range openaiProvider.Models {
		modelIDs[i] = model.ID
	}
	assert.Contains(t, modelIDs, "gpt-4")
	assert.Contains(t, modelIDs, "gpt-4-turbo")
}

func TestProviderModels_DuplicateModelHandling(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	globalConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:                provider.InferenceProviderOpenAI,
				APIKey:            "openai-key",
				ProviderType:      provider.TypeOpenAI,
				DefaultLargeModel: "gpt-4",
				DefaultSmallModel: "gpt-4",
				Models: []Model{
					{
						ID:               "gpt-4",
						Name:             "GPT-4",
						ContextWindow:    8192,
						DefaultMaxTokens: 4096,
					},
				},
			},
		},
	}

	localConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				Models: []Model{
					{
						ID:               "gpt-4",
						Name:             "GPT-4 Updated",
						ContextWindow:    16384,
						DefaultMaxTokens: 8192,
					},
				},
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	localConfigPath := filepath.Join(cwdDir, "crush.json")
	data, err = json.Marshal(localConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(localConfigPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)

	openaiProvider := cfg.Providers[provider.InferenceProviderOpenAI]
	assert.Len(t, openaiProvider.Models, 1)

	model := openaiProvider.Models[0]
	assert.Equal(t, "gpt-4", model.ID)
	assert.Equal(t, "GPT-4", model.Name)
	assert.Equal(t, int64(8192), model.ContextWindow)
}

func TestProviderModels_ModelCostAndCapabilities(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	globalConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:                provider.InferenceProviderOpenAI,
				APIKey:            "openai-key",
				ProviderType:      provider.TypeOpenAI,
				DefaultLargeModel: "gpt-4",
				DefaultSmallModel: "gpt-4",
				Models: []Model{
					{
						ID:                 "gpt-4",
						Name:               "GPT-4",
						CostPer1MIn:        30.0,
						CostPer1MOut:       60.0,
						CostPer1MInCached:  15.0,
						CostPer1MOutCached: 30.0,
						ContextWindow:      8192,
						DefaultMaxTokens:   4096,
						CanReason:          true,
						ReasoningEffort:    "medium",
						SupportsImages:     true,
					},
				},
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)

	openaiProvider := cfg.Providers[provider.InferenceProviderOpenAI]
	require.Len(t, openaiProvider.Models, 1)

	model := openaiProvider.Models[0]
	assert.Equal(t, 30.0, model.CostPer1MIn)
	assert.Equal(t, 60.0, model.CostPer1MOut)
	assert.Equal(t, 15.0, model.CostPer1MInCached)
	assert.Equal(t, 30.0, model.CostPer1MOutCached)
	assert.True(t, model.CanReason)
	assert.Equal(t, "medium", model.ReasoningEffort)
	assert.True(t, model.SupportsImages)
}

func TestDefaultAgents_CoderAgent(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-key")

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.Contains(t, cfg.Agents, AgentCoder)

	coderAgent := cfg.Agents[AgentCoder]
	assert.Equal(t, AgentCoder, coderAgent.ID)
	assert.Equal(t, "Coder", coderAgent.Name)
	assert.Equal(t, "An agent that helps with executing coding tasks.", coderAgent.Description)
	assert.Equal(t, LargeModel, coderAgent.Model)
	assert.False(t, coderAgent.Disabled)
	assert.Equal(t, cfg.Options.ContextPaths, coderAgent.ContextPaths)
	assert.Nil(t, coderAgent.AllowedTools)
}

func TestDefaultAgents_TaskAgent(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-key")

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	assert.Contains(t, cfg.Agents, AgentTask)

	taskAgent := cfg.Agents[AgentTask]
	assert.Equal(t, AgentTask, taskAgent.ID)
	assert.Equal(t, "Task", taskAgent.Name)
	assert.Equal(t, "An agent that helps with searching for context and finding implementation details.", taskAgent.Description)
	assert.Equal(t, LargeModel, taskAgent.Model)
	assert.False(t, taskAgent.Disabled)
	assert.Equal(t, cfg.Options.ContextPaths, taskAgent.ContextPaths)

	expectedTools := []string{"glob", "grep", "ls", "sourcegraph", "view"}
	assert.Equal(t, expectedTools, taskAgent.AllowedTools)

	assert.Equal(t, map[string][]string{}, taskAgent.AllowedMCP)
	assert.Equal(t, []string{}, taskAgent.AllowedLSP)
}

func TestAgentMerging_CustomAgent(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-key")

	globalConfig := Config{
		Agents: map[AgentID]Agent{
			AgentID("custom-agent"): {
				ID:           AgentID("custom-agent"),
				Name:         "Custom Agent",
				Description:  "A custom agent for testing",
				Model:        SmallModel,
				AllowedTools: []string{"glob", "grep"},
				AllowedMCP:   map[string][]string{"mcp1": {"tool1", "tool2"}},
				AllowedLSP:   []string{"typescript", "go"},
				ContextPaths: []string{"custom-context.md"},
			},
		},
		MCP: map[string]MCP{
			"mcp1": {
				Type:    MCPStdio,
				Command: "test-mcp-command",
				Args:    []string{"--test"},
			},
		},
		LSP: map[string]LSPConfig{
			"typescript": {
				Command: "typescript-language-server",
				Args:    []string{"--stdio"},
			},
			"go": {
				Command: "gopls",
				Args:    []string{},
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)

	assert.Contains(t, cfg.Agents, AgentCoder)
	assert.Contains(t, cfg.Agents, AgentTask)
	assert.Contains(t, cfg.Agents, AgentID("custom-agent"))

	customAgent := cfg.Agents[AgentID("custom-agent")]
	assert.Equal(t, "Custom Agent", customAgent.Name)
	assert.Equal(t, "A custom agent for testing", customAgent.Description)
	assert.Equal(t, SmallModel, customAgent.Model)
	assert.Equal(t, []string{"glob", "grep"}, customAgent.AllowedTools)
	assert.Equal(t, map[string][]string{"mcp1": {"tool1", "tool2"}}, customAgent.AllowedMCP)
	assert.Equal(t, []string{"typescript", "go"}, customAgent.AllowedLSP)
	expectedContextPaths := append(defaultContextPaths, "custom-context.md")
	assert.Equal(t, expectedContextPaths, customAgent.ContextPaths)
}

func TestAgentMerging_ModifyDefaultCoderAgent(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-key")

	globalConfig := Config{
		Agents: map[AgentID]Agent{
			AgentCoder: {
				Model:        SmallModel,
				AllowedMCP:   map[string][]string{"mcp1": {"tool1"}},
				AllowedLSP:   []string{"typescript"},
				ContextPaths: []string{"coder-specific.md"},
			},
		},
		MCP: map[string]MCP{
			"mcp1": {
				Type:    MCPStdio,
				Command: "test-mcp-command",
				Args:    []string{"--test"},
			},
		},
		LSP: map[string]LSPConfig{
			"typescript": {
				Command: "typescript-language-server",
				Args:    []string{"--stdio"},
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)

	coderAgent := cfg.Agents[AgentCoder]
	assert.Equal(t, AgentCoder, coderAgent.ID)
	assert.Equal(t, "Coder", coderAgent.Name)
	assert.Equal(t, "An agent that helps with executing coding tasks.", coderAgent.Description)

	expectedContextPaths := append(cfg.Options.ContextPaths, "coder-specific.md")
	assert.Equal(t, expectedContextPaths, coderAgent.ContextPaths)

	assert.Equal(t, SmallModel, coderAgent.Model)
	assert.Equal(t, map[string][]string{"mcp1": {"tool1"}}, coderAgent.AllowedMCP)
	assert.Equal(t, []string{"typescript"}, coderAgent.AllowedLSP)
}

func TestAgentMerging_ModifyDefaultTaskAgent(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-key")

	globalConfig := Config{
		Agents: map[AgentID]Agent{
			AgentTask: {
				Model:        SmallModel,
				AllowedMCP:   map[string][]string{"search-mcp": nil},
				AllowedLSP:   []string{"python"},
				Name:         "Search Agent",
				Description:  "Custom search agent",
				Disabled:     true,
				AllowedTools: []string{"glob", "grep", "view"},
			},
		},
		MCP: map[string]MCP{
			"search-mcp": {
				Type:    MCPStdio,
				Command: "search-mcp-command",
				Args:    []string{"--search"},
			},
		},
		LSP: map[string]LSPConfig{
			"python": {
				Command: "pylsp",
				Args:    []string{},
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)

	taskAgent := cfg.Agents[AgentTask]
	assert.Equal(t, "Task", taskAgent.Name)
	assert.Equal(t, "An agent that helps with searching for context and finding implementation details.", taskAgent.Description)
	assert.False(t, taskAgent.Disabled)
	assert.Equal(t, []string{"glob", "grep", "ls", "sourcegraph", "view"}, taskAgent.AllowedTools)

	assert.Equal(t, SmallModel, taskAgent.Model)
	assert.Equal(t, map[string][]string{"search-mcp": nil}, taskAgent.AllowedMCP)
	assert.Equal(t, []string{"python"}, taskAgent.AllowedLSP)
}

func TestAgentMerging_LocalOverridesGlobal(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-key")

	globalConfig := Config{
		Agents: map[AgentID]Agent{
			AgentID("test-agent"): {
				ID:           AgentID("test-agent"),
				Name:         "Global Agent",
				Description:  "Global description",
				Model:        LargeModel,
				AllowedTools: []string{"glob"},
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	// Create local config that overrides
	localConfig := Config{
		Agents: map[AgentID]Agent{
			AgentID("test-agent"): {
				Name:         "Local Agent",
				Description:  "Local description",
				Model:        SmallModel,
				Disabled:     true,
				AllowedTools: []string{"grep", "view"},
				AllowedMCP:   map[string][]string{"local-mcp": {"tool1"}},
			},
		},
		MCP: map[string]MCP{
			"local-mcp": {
				Type:    MCPStdio,
				Command: "local-mcp-command",
				Args:    []string{"--local"},
			},
		},
	}

	localConfigPath := filepath.Join(cwdDir, "crush.json")
	data, err = json.Marshal(localConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(localConfigPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)

	testAgent := cfg.Agents[AgentID("test-agent")]
	assert.Equal(t, "Local Agent", testAgent.Name)
	assert.Equal(t, "Local description", testAgent.Description)
	assert.Equal(t, SmallModel, testAgent.Model)
	assert.True(t, testAgent.Disabled)
	assert.Equal(t, []string{"grep", "view"}, testAgent.AllowedTools)
	assert.Equal(t, map[string][]string{"local-mcp": {"tool1"}}, testAgent.AllowedMCP)
}

func TestAgentModelTypeAssignment(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-key")

	globalConfig := Config{
		Agents: map[AgentID]Agent{
			AgentID("large-agent"): {
				ID:    AgentID("large-agent"),
				Name:  "Large Model Agent",
				Model: LargeModel,
			},
			AgentID("small-agent"): {
				ID:    AgentID("small-agent"),
				Name:  "Small Model Agent",
				Model: SmallModel,
			},
			AgentID("default-agent"): {
				ID:   AgentID("default-agent"),
				Name: "Default Model Agent",
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)

	assert.Equal(t, LargeModel, cfg.Agents[AgentID("large-agent")].Model)
	assert.Equal(t, SmallModel, cfg.Agents[AgentID("small-agent")].Model)
	assert.Equal(t, LargeModel, cfg.Agents[AgentID("default-agent")].Model)
}

func TestAgentContextPathOverrides(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-key")

	globalConfig := Config{
		Options: Options{
			ContextPaths: []string{"global-context.md", "shared-context.md"},
		},
		Agents: map[AgentID]Agent{
			AgentID("custom-context-agent"): {
				ID:           AgentID("custom-context-agent"),
				Name:         "Custom Context Agent",
				ContextPaths: []string{"agent-specific.md", "custom.md"},
			},
			AgentID("default-context-agent"): {
				ID:   AgentID("default-context-agent"),
				Name: "Default Context Agent",
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)

	customAgent := cfg.Agents[AgentID("custom-context-agent")]
	expectedCustomPaths := append(defaultContextPaths, "global-context.md", "shared-context.md", "agent-specific.md", "custom.md")
	assert.Equal(t, expectedCustomPaths, customAgent.ContextPaths)

	defaultAgent := cfg.Agents[AgentID("default-context-agent")]
	expectedContextPaths := append(defaultContextPaths, "global-context.md", "shared-context.md")
	assert.Equal(t, expectedContextPaths, defaultAgent.ContextPaths)

	coderAgent := cfg.Agents[AgentCoder]
	assert.Equal(t, expectedContextPaths, coderAgent.ContextPaths)
}

func TestOptionsMerging_ContextPaths(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-key")

	globalConfig := Config{
		Options: Options{
			ContextPaths: []string{"global1.md", "global2.md"},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	localConfig := Config{
		Options: Options{
			ContextPaths: []string{"local1.md", "local2.md"},
		},
	}

	localConfigPath := filepath.Join(cwdDir, "crush.json")
	data, err = json.Marshal(localConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(localConfigPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)

	expectedContextPaths := append(defaultContextPaths, "global1.md", "global2.md", "local1.md", "local2.md")
	assert.Equal(t, expectedContextPaths, cfg.Options.ContextPaths)
}

func TestOptionsMerging_TUIOptions(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-key")

	globalConfig := Config{
		Options: Options{
			TUI: TUIOptions{
				CompactMode: false,
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	localConfig := Config{
		Options: Options{
			TUI: TUIOptions{
				CompactMode: true,
			},
		},
	}

	localConfigPath := filepath.Join(cwdDir, "crush.json")
	data, err = json.Marshal(localConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(localConfigPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)

	assert.True(t, cfg.Options.TUI.CompactMode)
}

func TestOptionsMerging_DebugFlags(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-key")

	globalConfig := Config{
		Options: Options{
			Debug:                false,
			DebugLSP:             false,
			DisableAutoSummarize: false,
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	localConfig := Config{
		Options: Options{
			DebugLSP:             true,
			DisableAutoSummarize: true,
		},
	}

	localConfigPath := filepath.Join(cwdDir, "crush.json")
	data, err = json.Marshal(localConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(localConfigPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)

	assert.False(t, cfg.Options.Debug)
	assert.True(t, cfg.Options.DebugLSP)
	assert.True(t, cfg.Options.DisableAutoSummarize)
}

func TestOptionsMerging_DataDirectory(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-key")

	globalConfig := Config{
		Options: Options{
			DataDirectory: "global-data",
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	localConfig := Config{
		Options: Options{
			DataDirectory: "local-data",
		},
	}

	localConfigPath := filepath.Join(cwdDir, "crush.json")
	data, err = json.Marshal(localConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(localConfigPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)

	assert.Equal(t, "local-data", cfg.Options.DataDirectory)
}

func TestOptionsMerging_DefaultValues(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-key")

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)

	assert.Equal(t, defaultDataDirectory, cfg.Options.DataDirectory)
	assert.Equal(t, defaultContextPaths, cfg.Options.ContextPaths)
	assert.False(t, cfg.Options.TUI.CompactMode)
	assert.False(t, cfg.Options.Debug)
	assert.False(t, cfg.Options.DebugLSP)
	assert.False(t, cfg.Options.DisableAutoSummarize)
}

func TestOptionsMerging_DebugFlagFromInit(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-key")

	globalConfig := Config{
		Options: Options{
			Debug: false,
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	cfg, err := Init(cwdDir, true)

	require.NoError(t, err)

	// Debug flag from Init should take precedence
	assert.True(t, cfg.Options.Debug)
}

func TestOptionsMerging_ComplexScenario(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	// Set up a provider
	os.Setenv("ANTHROPIC_API_KEY", "test-key")

	// Create global config with various options
	globalConfig := Config{
		Options: Options{
			ContextPaths:         []string{"global-context.md"},
			DataDirectory:        "global-data",
			Debug:                false,
			DebugLSP:             false,
			DisableAutoSummarize: false,
			TUI: TUIOptions{
				CompactMode: false,
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	// Create local config that partially overrides
	localConfig := Config{
		Options: Options{
			ContextPaths:         []string{"local-context.md"},
			DebugLSP:             true, // Override
			DisableAutoSummarize: true, // Override
			TUI: TUIOptions{
				CompactMode: true, // Override
			},
			// DataDirectory and Debug not specified - should keep global values
		},
	}

	localConfigPath := filepath.Join(cwdDir, "crush.json")
	data, err = json.Marshal(localConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(localConfigPath, data, 0o644))

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)

	// Check merged results
	expectedContextPaths := append(defaultContextPaths, "global-context.md", "local-context.md")
	assert.Equal(t, expectedContextPaths, cfg.Options.ContextPaths)
	assert.Equal(t, "global-data", cfg.Options.DataDirectory) // From global
	assert.False(t, cfg.Options.Debug)                        // From global
	assert.True(t, cfg.Options.DebugLSP)                      // From local
	assert.True(t, cfg.Options.DisableAutoSummarize)          // From local
	assert.True(t, cfg.Options.TUI.CompactMode)               // From local
}

// Model Selection Tests

func TestModelSelection_PreferredModelSelection(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	// Set up multiple providers to test selection logic
	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	os.Setenv("OPENAI_API_KEY", "test-openai-key")

	cfg, err := Init(cwdDir, false)

	require.NoError(t, err)
	require.Len(t, cfg.Providers, 2)

	// Should have preferred models set
	assert.NotEmpty(t, cfg.Models.Large.ModelID)
	assert.NotEmpty(t, cfg.Models.Large.Provider)
	assert.NotEmpty(t, cfg.Models.Small.ModelID)
	assert.NotEmpty(t, cfg.Models.Small.Provider)

	// Both should use the same provider (first available)
	assert.Equal(t, cfg.Models.Large.Provider, cfg.Models.Small.Provider)
}

func TestValidation_InvalidModelReference(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	globalConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:                provider.InferenceProviderOpenAI,
				APIKey:            "test-key",
				ProviderType:      provider.TypeOpenAI,
				DefaultLargeModel: "non-existent-model",
				DefaultSmallModel: "gpt-3.5-turbo",
				Models: []Model{
					{
						ID:               "gpt-3.5-turbo",
						Name:             "GPT-3.5 Turbo",
						ContextWindow:    4096,
						DefaultMaxTokens: 2048,
					},
				},
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	_, err = Init(cwdDir, false)
	assert.Error(t, err)
}

func TestValidation_EmptyAPIKey(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	globalConfig := Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:           provider.InferenceProviderOpenAI,
				ProviderType: provider.TypeOpenAI,
				Models: []Model{
					{
						ID:               "gpt-4",
						Name:             "GPT-4",
						ContextWindow:    8192,
						DefaultMaxTokens: 4096,
					},
				},
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	_, err = Init(cwdDir, false)
	assert.Error(t, err)
}

func TestValidation_InvalidAgentModelType(t *testing.T) {
	reset()
	testConfigDir = t.TempDir()
	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-key")

	globalConfig := Config{
		Agents: map[AgentID]Agent{
			AgentID("invalid-agent"): {
				ID:    AgentID("invalid-agent"),
				Name:  "Invalid Agent",
				Model: ModelType("invalid"),
			},
		},
	}

	configPath := filepath.Join(testConfigDir, "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	data, err := json.Marshal(globalConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0o644))

	_, err = Init(cwdDir, false)
	assert.Error(t, err)
}
