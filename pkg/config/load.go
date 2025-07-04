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

	"github.com/charmbracelet/crush/internal/fur/client"
	"github.com/charmbracelet/crush/internal/fur/provider"
	"github.com/charmbracelet/crush/pkg/env"
	"github.com/charmbracelet/crush/pkg/log"
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
		globalConfigData(),
		filepath.Join(workingDir, fmt.Sprintf("%s.json", appName)),
		filepath.Join(workingDir, fmt.Sprintf(".%s.json", appName)),
	}
	cfg, err := loadFromConfigPaths(configPaths)

	if debug {
		cfg.Options.Debug = true
	}

	// Init logs
	log.Init(
		filepath.Join(cfg.Options.DataDirectory, "logs", fmt.Sprintf("%s.log", appName)),
		cfg.Options.Debug,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	// TODO: maybe add a validation step here right after loading
	// e.x validate the models
	// e.x validate provider config

	cfg.setDefaults(workingDir)

	// Load known providers, this loads the config from fur
	providers, err := LoadProviders(client.New())
	if err != nil {
		return nil, fmt.Errorf("failed to load providers: %w", err)
	}

	env := env.New()
	// Configure providers
	valueResolver := NewShellVariableResolver(env)
	if err := cfg.configureProviders(env, valueResolver, providers); err != nil {
		return nil, fmt.Errorf("failed to configure providers: %w", err)
	}

	return cfg, nil
}

func (cfg *Config) configureProviders(env env.Env, resolver VariableResolver, knownProviders []provider.Provider) error {
	for _, p := range knownProviders {

		config, ok := cfg.Providers[string(p.ID)]
		// if the user configured a known provider we need to allow it to override a couple of parameters
		if ok {
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
					models = append(models, model)
				}
				for _, model := range p.Models {
					if seen[model.ID] {
						continue
					}
					seen[model.ID] = true
					models = append(models, model)
				}

				p.Models = models
			}
		}
		prepared := ProviderConfig{
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
				continue
			}
			prepared.ExtraParams["project"] = env.Get("GOOGLE_CLOUD_PROJECT")
			prepared.ExtraParams["location"] = env.Get("GOOGLE_CLOUD_LOCATION")
		case provider.InferenceProviderBedrock:
			if !hasAWSCredentials(env) {
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
				continue
			}
			v, err = resolver.ResolveValue(p.APIEndpoint)
			if v == "" || err != nil {
				continue
			}
		}
		cfg.Providers[string(p.ID)] = prepared
	}
	return nil
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

func (cfg *Config) setDefaults(workingDir string) {
	cfg.workingDir = workingDir
	if cfg.Options == nil {
		cfg.Options = &Options{}
	}
	if cfg.Options.TUI == nil {
		cfg.Options.TUI = &TUIOptions{}
	}
	if cfg.Options.ContextPaths == nil {
		cfg.Options.ContextPaths = []string{}
	}
	if cfg.Options.DataDirectory == "" {
		cfg.Options.DataDirectory = filepath.Join(workingDir, defaultDataDirectory)
	}
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]ProviderConfig)
	}
	if cfg.Models == nil {
		cfg.Models = make(map[string]SelectedModel)
	}
	if cfg.MCP == nil {
		cfg.MCP = make(map[string]MCPConfig)
	}
	if cfg.LSP == nil {
		cfg.LSP = make(map[string]LSPConfig)
	}

	// Add the default context paths if they are not already present
	cfg.Options.ContextPaths = append(defaultContextPaths, cfg.Options.ContextPaths...)
	slices.Sort(cfg.Options.ContextPaths)
	cfg.Options.ContextPaths = slices.Compact(cfg.Options.ContextPaths)
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
		return nil, fmt.Errorf("no configuration readers provided")
	}

	merged, err := Merge(readers)
	if err != nil {
		return nil, fmt.Errorf("failed to merge configuration readers: %w", err)
	}

	return LoadReader(merged)
}

func globalConfig() string {
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, "crush")
	}

	// return the path to the main config directory
	// for windows, it should be in `%LOCALAPPDATA%/crush/`
	// for linux and macOS, it should be in `$HOME/.config/crush/`
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(localAppData, appName)
	}

	return filepath.Join(os.Getenv("HOME"), ".config", appName, fmt.Sprintf("%s.json", appName))
}

// globalConfigData returns the path to the main data directory for the application.
// this config is used when the app overrides configurations instead of updating the global config.
func globalConfigData() string {
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome != "" {
		return filepath.Join(xdgDataHome, appName)
	}

	// return the path to the main data directory
	// for windows, it should be in `%LOCALAPPDATA%/crush/`
	// for linux and macOS, it should be in `$HOME/.local/share/crush/`
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(localAppData, appName)
	}

	return filepath.Join(os.Getenv("HOME"), ".local", "share", appName, fmt.Sprintf("%s.json", appName))
}
