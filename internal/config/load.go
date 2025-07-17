package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/env"
	"github.com/charmbracelet/crush/internal/fur/client"
	"github.com/charmbracelet/crush/internal/fur/provider"
	"github.com/charmbracelet/crush/internal/log"
	"golang.org/x/exp/slog"
)

// LoadReader config via io.Reader.
func LoadReader(fd io.Reader) (*Config, error) {
	data, err := io.ReadAll(fd)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, err
}

// Load loads the configuration from the default paths.
func Load(workingDir string, debug bool) (*Config, error) {
	// uses default config paths
	configPaths := []string{
		globalConfig(),
		GlobalConfigData(),
		filepath.Join(workingDir, fmt.Sprintf("%s.json", appName)),
		filepath.Join(workingDir, fmt.Sprintf(".%s.json", appName)),
	}
	cfg, err := loadFromConfigPaths(configPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from paths %v: %w", configPaths, err)
	}

	cfg.dataConfigDir = GlobalConfigData()

	cfg.setDefaults(workingDir)

	if debug {
		cfg.Options.Debug = true
	}

	// Setup logs
	log.Setup(
		filepath.Join(cfg.Options.DataDirectory, "logs", fmt.Sprintf("%s.log", appName)),
		cfg.Options.Debug,
	)

	// Load known providers, this loads the config from fur
	providers, err := LoadProviders(client.New())
	if err != nil || len(providers) == 0 {
		return nil, fmt.Errorf("failed to load providers: %w", err)
	}
	cfg.knownProviders = providers

	env := env.New()
	// Configure providers
	valueResolver := NewShellVariableResolver(env)
	cfg.resolver = valueResolver
	if err := cfg.configureProviders(env, valueResolver, providers); err != nil {
		return nil, fmt.Errorf("failed to configure providers: %w", err)
	}

	// Test provider connections in parallel
	var testResults sync.Map
	var wg sync.WaitGroup

	for _, p := range cfg.Providers {
		if p.Type == provider.TypeOpenAI || p.Type == provider.TypeAnthropic {
			wg.Add(1)
			go func(provider ProviderConfig) {
				defer wg.Done()
				err := provider.TestConnection(cfg.resolver)
				testResults.Store(provider.ID, err == nil)
				if err != nil {
					slog.Error("Provider connection test failed", "provider", provider.ID, "error", err)
				}
			}(p)
		}
	}
	wg.Wait()

	// Remove failed providers
	testResults.Range(func(key, value any) bool {
		providerID := key.(string)
		passed := value.(bool)
		if !passed {
			delete(cfg.Providers, providerID)
		}
		return true
	})

	if !cfg.IsConfigured() {
		slog.Warn("No providers configured")
		return cfg, nil
	}

	if err := cfg.configureSelectedModels(providers); err != nil {
		return nil, fmt.Errorf("failed to configure selected models: %w", err)
	}
	cfg.SetupAgents()
	return cfg, nil
}

func (c *Config) configureProviders(env env.Env, resolver VariableResolver, knownProviders []provider.Provider) error {
	knownProviderNames := make(map[string]bool)
	for _, p := range knownProviders {
		knownProviderNames[string(p.ID)] = true
		config, configExists := c.Providers[string(p.ID)]
		// if the user configured a known provider we need to allow it to override a couple of parameters
		if configExists {
			if config.Disable {
				slog.Debug("Skipping provider due to disable flag", "provider", p.ID)
				delete(c.Providers, string(p.ID))
				continue
			}
			if config.BaseURL != "" {
				p.APIEndpoint = config.BaseURL
			}
			if config.APIKey != "" {
				p.APIKey = config.APIKey
			}
			if len(config.Models) > 0 {
				models := []provider.Model{}
				seen := make(map[string]bool)

				for _, model := range config.Models {
					if seen[model.ID] {
						continue
					}
					seen[model.ID] = true
					if model.Model == "" {
						model.Model = model.ID
					}
					models = append(models, model)
				}
				for _, model := range p.Models {
					if seen[model.ID] {
						continue
					}
					seen[model.ID] = true
					if model.Model == "" {
						model.Model = model.ID
					}
					models = append(models, model)
				}

				p.Models = models
			}
		}
		prepared := ProviderConfig{
			ID:           string(p.ID),
			Name:         p.Name,
			BaseURL:      p.APIEndpoint,
			APIKey:       p.APIKey,
			Type:         p.Type,
			Disable:      config.Disable,
			ExtraHeaders: config.ExtraHeaders,
			ExtraParams:  make(map[string]string),
			Models:       p.Models,
		}

		switch p.ID {
		// Handle specific providers that require additional configuration
		case provider.InferenceProviderVertexAI:
			if !hasVertexCredentials(env) {
				if configExists {
					slog.Warn("Skipping Vertex AI provider due to missing credentials")
					delete(c.Providers, string(p.ID))
				}
				continue
			}
			prepared.ExtraParams["project"] = env.Get("GOOGLE_CLOUD_PROJECT")
			prepared.ExtraParams["location"] = env.Get("GOOGLE_CLOUD_LOCATION")
		case provider.InferenceProviderBedrock:
			if !hasAWSCredentials(env) {
				if configExists {
					slog.Warn("Skipping Bedrock provider due to missing AWS credentials")
					delete(c.Providers, string(p.ID))
				}
				continue
			}
			for _, model := range p.Models {
				if !strings.HasPrefix(model.ID, "anthropic.") {
					return fmt.Errorf("bedrock provider only supports anthropic models for now, found: %s", model.ID)
				}
			}
		default:
			// if the provider api or endpoint are missing we skip them
			v, err := resolver.ResolveValue(p.APIKey)
			if v == "" || err != nil {
				if configExists {
					slog.Warn("Skipping provider due to missing API key", "provider", p.ID)
					delete(c.Providers, string(p.ID))
				}
				continue
			}
		}
		c.Providers[string(p.ID)] = prepared
	}

	// validate the custom providers
	for id, providerConfig := range c.Providers {
		if knownProviderNames[id] {
			continue
		}

		// Make sure the provider ID is set
		providerConfig.ID = id
		if providerConfig.Name == "" {
			providerConfig.Name = id // Use ID as name if not set
		}
		// default to OpenAI if not set
		if providerConfig.Type == "" {
			providerConfig.Type = provider.TypeOpenAI
		}

		if providerConfig.Disable {
			slog.Debug("Skipping custom provider due to disable flag", "provider", id)
			delete(c.Providers, id)
			continue
		}
		if providerConfig.APIKey == "" {
			slog.Warn("Provider is missing API key, this might be OK for local providers", "provider", id)
		}
		if providerConfig.BaseURL == "" {
			slog.Warn("Skipping custom provider due to missing API endpoint", "provider", id)
			delete(c.Providers, id)
			continue
		}
		if len(providerConfig.Models) == 0 {
			slog.Warn("Skipping custom provider because the provider has no models", "provider", id)
			delete(c.Providers, id)
			continue
		}
		if providerConfig.Type != provider.TypeOpenAI {
			slog.Warn("Skipping custom provider because the provider type is not supported", "provider", id, "type", providerConfig.Type)
			delete(c.Providers, id)
			continue
		}

		apiKey, err := resolver.ResolveValue(providerConfig.APIKey)
		if apiKey == "" || err != nil {
			slog.Warn("Provider is missing API key, this might be OK for local providers", "provider", id)
		}
		baseURL, err := resolver.ResolveValue(providerConfig.BaseURL)
		if baseURL == "" || err != nil {
			slog.Warn("Skipping custom provider due to missing API endpoint", "provider", id, "error", err)
			delete(c.Providers, id)
			continue
		}

		c.Providers[id] = providerConfig
	}
	return nil
}

func (c *Config) setDefaults(workingDir string) {
	c.workingDir = workingDir
	if c.Options == nil {
		c.Options = &Options{}
	}
	if c.Options.TUI == nil {
		c.Options.TUI = &TUIOptions{}
	}
	if c.Options.ContextPaths == nil {
		c.Options.ContextPaths = []string{}
	}
	if c.Options.DataDirectory == "" {
		c.Options.DataDirectory = filepath.Join(workingDir, defaultDataDirectory)
	}
	if c.Providers == nil {
		c.Providers = make(map[string]ProviderConfig)
	}
	if c.Models == nil {
		c.Models = make(map[SelectedModelType]SelectedModel)
	}
	if c.MCP == nil {
		c.MCP = make(map[string]MCPConfig)
	}
	if c.LSP == nil {
		c.LSP = make(map[string]LSPConfig)
	}

	// Add the default context paths if they are not already present
	c.Options.ContextPaths = append(defaultContextPaths, c.Options.ContextPaths...)
	slices.Sort(c.Options.ContextPaths)
	c.Options.ContextPaths = slices.Compact(c.Options.ContextPaths)
}

func (c *Config) defaultModelSelection(knownProviders []provider.Provider) (largeModel SelectedModel, smallModel SelectedModel, err error) {
	if len(knownProviders) == 0 && len(c.Providers) == 0 {
		err = fmt.Errorf("no providers configured, please configure at least one provider")
		return
	}

	// Use the first provider enabled based on the known providers order
	// if no provider found that is known use the first provider configured
	for _, p := range knownProviders {
		providerConfig, ok := c.Providers[string(p.ID)]
		if !ok || providerConfig.Disable {
			continue
		}
		defaultLargeModel := c.GetModel(string(p.ID), p.DefaultLargeModelID)
		if defaultLargeModel == nil {
			err = fmt.Errorf("default large model %s not found for provider %s", p.DefaultLargeModelID, p.ID)
			return
		}
		largeModel = SelectedModel{
			Provider:        string(p.ID),
			Model:           defaultLargeModel.ID,
			MaxTokens:       defaultLargeModel.DefaultMaxTokens,
			ReasoningEffort: defaultLargeModel.DefaultReasoningEffort,
		}

		defaultSmallModel := c.GetModel(string(p.ID), p.DefaultSmallModelID)
		if defaultSmallModel == nil {
			err = fmt.Errorf("default small model %s not found for provider %s", p.DefaultSmallModelID, p.ID)
			return
		}
		smallModel = SelectedModel{
			Provider:        string(p.ID),
			Model:           defaultSmallModel.ID,
			MaxTokens:       defaultSmallModel.DefaultMaxTokens,
			ReasoningEffort: defaultSmallModel.DefaultReasoningEffort,
		}
		return
	}

	enabledProviders := c.EnabledProviders()
	slices.SortFunc(enabledProviders, func(a, b ProviderConfig) int {
		return strings.Compare(a.ID, b.ID)
	})

	if len(enabledProviders) == 0 {
		err = fmt.Errorf("no providers configured, please configure at least one provider")
		return
	}

	providerConfig := enabledProviders[0]
	if len(providerConfig.Models) == 0 {
		err = fmt.Errorf("provider %s has no models configured", providerConfig.ID)
		return
	}
	defaultLargeModel := c.GetModel(providerConfig.ID, providerConfig.Models[0].ID)
	largeModel = SelectedModel{
		Provider:  providerConfig.ID,
		Model:     defaultLargeModel.ID,
		MaxTokens: defaultLargeModel.DefaultMaxTokens,
	}
	defaultSmallModel := c.GetModel(providerConfig.ID, providerConfig.Models[0].ID)
	smallModel = SelectedModel{
		Provider:  providerConfig.ID,
		Model:     defaultSmallModel.ID,
		MaxTokens: defaultSmallModel.DefaultMaxTokens,
	}
	return
}

func (c *Config) configureSelectedModels(knownProviders []provider.Provider) error {
	defaultLarge, defaultSmall, err := c.defaultModelSelection(knownProviders)
	if err != nil {
		return fmt.Errorf("failed to select default models: %w", err)
	}
	large, small := defaultLarge, defaultSmall

	largeModelSelected, largeModelConfigured := c.Models[SelectedModelTypeLarge]
	if largeModelConfigured {
		if largeModelSelected.Model != "" {
			large.Model = largeModelSelected.Model
		}
		if largeModelSelected.Provider != "" {
			large.Provider = largeModelSelected.Provider
		}
		model := c.GetModel(large.Provider, large.Model)
		if model == nil {
			large = defaultLarge
			// override the model type to large
			err := c.UpdatePreferredModel(SelectedModelTypeLarge, large)
			if err != nil {
				return fmt.Errorf("failed to update preferred large model: %w", err)
			}
		} else {
			if largeModelSelected.MaxTokens > 0 {
				large.MaxTokens = largeModelSelected.MaxTokens
			} else {
				large.MaxTokens = model.DefaultMaxTokens
			}
			if largeModelSelected.ReasoningEffort != "" {
				large.ReasoningEffort = largeModelSelected.ReasoningEffort
			}
			large.Think = largeModelSelected.Think
		}
	}
	smallModelSelected, smallModelConfigured := c.Models[SelectedModelTypeSmall]
	if smallModelConfigured {
		if smallModelSelected.Model != "" {
			small.Model = smallModelSelected.Model
		}
		if smallModelSelected.Provider != "" {
			small.Provider = smallModelSelected.Provider
		}

		model := c.GetModel(small.Provider, small.Model)
		if model == nil {
			small = defaultSmall
			// override the model type to small
			err := c.UpdatePreferredModel(SelectedModelTypeSmall, small)
			if err != nil {
				return fmt.Errorf("failed to update preferred small model: %w", err)
			}
		} else {
			if smallModelSelected.MaxTokens > 0 {
				small.MaxTokens = smallModelSelected.MaxTokens
			} else {
				small.MaxTokens = model.DefaultMaxTokens
			}
			small.ReasoningEffort = smallModelSelected.ReasoningEffort
			small.Think = smallModelSelected.Think
		}
	}
	c.Models[SelectedModelTypeLarge] = large
	c.Models[SelectedModelTypeSmall] = small
	return nil
}

func loadFromConfigPaths(configPaths []string) (*Config, error) {
	var configs []io.Reader

	for _, path := range configPaths {
		fd, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to open config file %s: %w", path, err)
		}
		defer fd.Close()

		configs = append(configs, fd)
	}

	return loadFromReaders(configs)
}

func loadFromReaders(readers []io.Reader) (*Config, error) {
	if len(readers) == 0 {
		return &Config{}, nil
	}

	merged, err := Merge(readers)
	if err != nil {
		return nil, fmt.Errorf("failed to merge configuration readers: %w", err)
	}

	return LoadReader(merged)
}

func hasVertexCredentials(env env.Env) bool {
	useVertex := env.Get("GOOGLE_GENAI_USE_VERTEXAI") == "true"
	hasProject := env.Get("GOOGLE_CLOUD_PROJECT") != ""
	hasLocation := env.Get("GOOGLE_CLOUD_LOCATION") != ""
	return useVertex && hasProject && hasLocation
}

func hasAWSCredentials(env env.Env) bool {
	if env.Get("AWS_ACCESS_KEY_ID") != "" && env.Get("AWS_SECRET_ACCESS_KEY") != "" {
		return true
	}

	if env.Get("AWS_PROFILE") != "" || env.Get("AWS_DEFAULT_PROFILE") != "" {
		return true
	}

	if env.Get("AWS_REGION") != "" || env.Get("AWS_DEFAULT_REGION") != "" {
		return true
	}

	if env.Get("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI") != "" ||
		env.Get("AWS_CONTAINER_CREDENTIALS_FULL_URI") != "" {
		return true
	}

	return false
}

func globalConfig() string {
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, appName, fmt.Sprintf("%s.json", appName))
	}

	// return the path to the main config directory
	// for windows, it should be in `%LOCALAPPDATA%/crush/`
	// for linux and macOS, it should be in `$HOME/.config/crush/`
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(localAppData, appName, fmt.Sprintf("%s.json", appName))
	}

	return filepath.Join(os.Getenv("HOME"), ".config", appName, fmt.Sprintf("%s.json", appName))
}

// GlobalConfigData returns the path to the main data directory for the application.
// this config is used when the app overrides configurations instead of updating the global config.
func GlobalConfigData() string {
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome != "" {
		return filepath.Join(xdgDataHome, appName, fmt.Sprintf("%s.json", appName))
	}

	// return the path to the main data directory
	// for windows, it should be in `%LOCALAPPDATA%/crush/`
	// for linux and macOS, it should be in `$HOME/.local/share/crush/`
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(localAppData, appName, fmt.Sprintf("%s.json", appName))
	}

	return filepath.Join(os.Getenv("HOME"), ".local", "share", appName, fmt.Sprintf("%s.json", appName))
}

func HomeDir() string {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = os.Getenv("USERPROFILE") // For Windows compatibility
	}
	if homeDir == "" {
		homeDir = os.Getenv("HOMEPATH") // Fallback for some environments
	}
	return homeDir
}
