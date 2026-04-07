package config

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/log"
	powernapConfig "github.com/charmbracelet/x/powernap/pkg/config"
	"github.com/qjebbs/go-jsons"
	"github.com/tidwall/gjson"
)

const defaultCatwalkURL = "https://catwalk.charm.sh"

// Load loads the configuration from the default paths and returns a
// ConfigStore that owns both the pure-data Config and all runtime state.
func Load(workingDir, dataDir string, debug bool) (*ConfigStore, error) {
	configPaths := lookupConfigs(workingDir)

	cfg, err := loadFromConfigPaths(configPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from paths %v: %w", configPaths, err)
	}

	workspaceDir := workspaceIdentityDir(workingDir)

	// Determine paths:
	// - workspaceConfigPath: project-level config in .crush/crush.json (can be committed)
	// - projectDataDir: centralized data storage (sessions, memory, logs)
	// 
	// For workspaceConfigPath, we use workingDir directly (not workspaceIdentityDir)
	// because workspaceIdentityDir may resolve to a different path via env vars like PWD.
	// We want the config to be at <workingDir>/.crush/crush.json.
	workspaceConfigPath := filepath.Join(workingDir, defaultDataDirectory, fmt.Sprintf("%s.json", appName))
	projectDataDir := ProjectDataDir(workingDir)

	// Allow explicit override via dataDir parameter or config
	if dataDir != "" {
		projectDataDir = dataDir
	}

	cfg.setDefaults(workingDir, projectDataDir)

	// Load SkipRequests (YOLO mode) from global data config only.
	// This field is marked json:"-" to prevent loading from untrusted project configs.
	if cfg.Permissions == nil {
		cfg.Permissions = &Permissions{}
	}
	if data, err := os.ReadFile(GlobalConfigData()); err == nil && len(data) > 0 {
		if gjson.GetBytes(data, "permissions.skip_requests").Bool() {
			cfg.Permissions.SkipRequests = true
		}
	}

	store := &ConfigStore{
		config:         cfg,
		workingDir:     workspaceDir,
		globalDataPath: GlobalConfigData(),
		workspacePath:  workspaceConfigPath,
		projectDataDir: projectDataDir,
	}

	if debug {
		cfg.Options.Debug = true
	}

	// Setup logs in centralized project data directory
	log.Setup(
		filepath.Join(projectDataDir, "logs", fmt.Sprintf("%s.log", appName)),
		cfg.Options.Debug,
	)

	// Load workspace config last so it has highest priority.
	// Preserve SkipRequests (YOLO mode) as it's loaded from global data config only.
	skipRequests := cfg.Permissions.SkipRequests
	if wsData, err := os.ReadFile(store.workspacePath); err == nil && len(wsData) > 0 {
		merged, mergeErr := loadFromBytes(append([][]byte{mustMarshalConfig(cfg)}, wsData))
		if mergeErr == nil {
			// Preserve project data directory from setDefaults.
			savedDataDir := projectDataDir
			*cfg = *merged
			cfg.setDefaults(workingDir, savedDataDir)
			store.config = cfg
		}
	}
	// Restore SkipRequests after workspace merge (it has json:"-" and is not persisted in workspace config).
	cfg.Permissions.SkipRequests = skipRequests

	if !isInsideWorktree() {
		const depth = 2
		const items = 100
		slog.Warn("No git repository detected in working directory, will limit file walk operations", "depth", depth, "items", items)
		assignIfNil(&cfg.Tools.Ls.MaxDepth, depth)
		assignIfNil(&cfg.Tools.Ls.MaxItems, items)
		assignIfNil(&cfg.Options.TUI.Completions.MaxDepth, depth)
		assignIfNil(&cfg.Options.TUI.Completions.MaxItems, items)
	}

	if isAppleTerminal() {
		slog.Warn("Detected Apple Terminal, enabling transparent mode")
		assignIfNil(&cfg.Options.TUI.Transparent, true)
	}

	// Load known providers, this loads the config from catwalk
	providers, err := Providers(cfg)
	if err != nil {
		return nil, err
	}
	store.knownProviders = providers

	env := env.New()
	// Configure providers
	valueResolver := NewShellVariableResolver(env)
	store.resolver = valueResolver
	if err := cfg.configureProviders(store, env, valueResolver, store.knownProviders); err != nil {
		return nil, fmt.Errorf("failed to configure providers: %w", err)
	}

	if !cfg.IsConfigured() {
		slog.Warn("No providers configured")
		return store, nil
	}

	if err := configureSelectedModels(store, store.knownProviders); err != nil {
		return nil, fmt.Errorf("failed to configure selected models: %w", err)
	}
	store.SetupAgents()
	return store, nil
}

// mustMarshalConfig marshals the config to JSON bytes, returning empty JSON on
// error.
func mustMarshalConfig(cfg *Config) []byte {
	data, err := json.Marshal(cfg)
	if err != nil {
		return []byte("{}")
	}
	return data
}

func PushPopCrushEnv() func() {
	var found []string
	for _, ev := range os.Environ() {
		if strings.HasPrefix(ev, "CRUSH_") {
			pair := strings.SplitN(ev, "=", 2)
			if len(pair) != 2 {
				continue
			}
			found = append(found, strings.TrimPrefix(pair[0], "CRUSH_"))
		}
	}
	backups := make(map[string]string)
	for _, ev := range found {
		backups[ev] = os.Getenv(ev)
	}

	for _, ev := range found {
		os.Setenv(ev, os.Getenv("CRUSH_"+ev))
	}

	restore := func() {
		for k, v := range backups {
			os.Setenv(k, v)
		}
	}
	return restore
}

func (c *Config) configureProviders(store *ConfigStore, env env.Env, resolver VariableResolver, knownProviders []catwalk.Provider) error {
	knownProviderNames := make(map[string]bool)
	restore := PushPopCrushEnv()
	defer restore()

	// When disable_default_providers is enabled, skip all default/embedded
	// providers entirely. Users must fully specify any providers they want.
	// We skip to the custom provider validation loop which handles all
	// user-configured providers uniformly.
	if c.Options.DisableDefaultProviders {
		knownProviders = nil
	}

	for _, p := range knownProviders {
		knownProviderNames[string(p.ID)] = true
		config, configExists := c.Providers.Get(string(p.ID))
		// if the user configured a known provider we need to allow it to override a couple of parameters
		if configExists {
			if config.BaseURL != "" {
				p.APIEndpoint = config.BaseURL
			}
			if config.APIKey != "" {
				p.APIKey = config.APIKey
			}
			if len(config.Models) > 0 {
				models := []catwalk.Model{}
				seen := make(map[string]bool)

				for _, model := range config.Models {
					if seen[model.ID] {
						continue
					}
					seen[model.ID] = true
					if model.Name == "" {
						model.Name = model.ID
					}
					models = append(models, model)
				}
				for _, model := range p.Models {
					if seen[model.ID] {
						continue
					}
					seen[model.ID] = true
					if model.Name == "" {
						model.Name = model.ID
					}
					models = append(models, model)
				}

				p.Models = models
			}
		}

		headers := map[string]string{}
		if len(p.DefaultHeaders) > 0 {
			maps.Copy(headers, p.DefaultHeaders)
		}
		if len(config.ExtraHeaders) > 0 {
			maps.Copy(headers, config.ExtraHeaders)
		}
		for k, v := range headers {
			resolved, err := resolver.ResolveValue(v)
			if err != nil {
				slog.Error("Could not resolve provider header", "err", err.Error())
				continue
			}
			headers[k] = resolved
		}
		prepared := ProviderConfig{
			ID:                 string(p.ID),
			Name:               p.Name,
			BaseURL:            p.APIEndpoint,
			APIKey:             p.APIKey,
			APIKeyTemplate:     p.APIKey, // Store original template for re-resolution
			OAuthToken:         config.OAuthToken,
			Type:               p.Type,
			Disable:            config.Disable,
			SystemPromptPrefix: config.SystemPromptPrefix,
			ExtraHeaders:       headers,
			ExtraBody:          config.ExtraBody,
			ExtraParams:        make(map[string]string),
			ResponsesWebSocket: config.ResponsesWebSocket,
			Models:             p.Models,
		}

		switch {
		case p.ID == catwalk.InferenceProviderAnthropic && config.OAuthToken != nil:
			// Claude Code subscription is not supported anymore. Remove to show onboarding.
			store.RemoveConfigField(ScopeGlobal, "providers.anthropic")
			c.Providers.Del(string(p.ID))
			continue
		case p.ID == catwalk.InferenceProviderCopilot && config.OAuthToken != nil:
			prepared.SetupGitHubCopilot()
		}

		switch p.ID {
		// Handle specific providers that require additional configuration
		case catwalk.InferenceProviderVertexAI:
			var (
				project  = env.Get("VERTEXAI_PROJECT")
				location = env.Get("VERTEXAI_LOCATION")
			)
			if project == "" || location == "" {
				if configExists {
					slog.Warn("Skipping Vertex AI provider due to missing credentials")
					c.Providers.Del(string(p.ID))
				}
				continue
			}
			prepared.ExtraParams["project"] = project
			prepared.ExtraParams["location"] = location
		case catwalk.InferenceProviderAzure:
			endpoint, err := resolver.ResolveValue(p.APIEndpoint)
			if err != nil || endpoint == "" {
				if configExists {
					slog.Warn("Skipping Azure provider due to missing API endpoint", "provider", p.ID, "error", err)
					c.Providers.Del(string(p.ID))
				}
				continue
			}
			prepared.BaseURL = endpoint
			prepared.ExtraParams["apiVersion"] = env.Get("AZURE_OPENAI_API_VERSION")
		case catwalk.InferenceProviderBedrock:
			if !hasAWSCredentials(env) {
				if configExists {
					slog.Warn("Skipping Bedrock provider due to missing AWS credentials")
					c.Providers.Del(string(p.ID))
				}
				continue
			}
			prepared.ExtraParams["region"] = env.Get("AWS_REGION")
			if prepared.ExtraParams["region"] == "" {
				prepared.ExtraParams["region"] = env.Get("AWS_DEFAULT_REGION")
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
					c.Providers.Del(string(p.ID))
				}
				continue
			}
		}
		c.Providers.Set(string(p.ID), prepared)
	}

	// validate the custom providers
	for id, providerConfig := range c.Providers.Seq2() {
		if knownProviderNames[id] {
			continue
		}

		// Make sure the provider ID is set
		providerConfig.ID = id
		providerConfig.Name = cmp.Or(providerConfig.Name, id) // Use ID as name if not set
		// default to OpenAI if not set
		providerConfig.Type = cmp.Or(providerConfig.Type, catwalk.TypeOpenAICompat)
		if !slices.Contains(catwalk.KnownProviderTypes(), providerConfig.Type) && providerConfig.Type != hyper.Name {
			slog.Warn("Skipping custom provider due to unsupported provider type", "provider", id)
			c.Providers.Del(id)
			continue
		}

		if providerConfig.Disable {
			slog.Debug("Skipping custom provider due to disable flag", "provider", id)
			c.Providers.Del(id)
			continue
		}
		if providerConfig.APIKey == "" {
			slog.Warn("Provider is missing API key, this might be OK for local providers", "provider", id)
		}
		if providerConfig.BaseURL == "" {
			slog.Warn("Skipping custom provider due to missing API endpoint", "provider", id)
			c.Providers.Del(id)
			continue
		}
		if len(providerConfig.Models) == 0 {
			slog.Warn("Skipping custom provider because the provider has no models", "provider", id)
			c.Providers.Del(id)
			continue
		}
		apiKey, err := resolver.ResolveValue(providerConfig.APIKey)
		if apiKey == "" || err != nil {
			slog.Warn("Provider is missing API key, this might be OK for local providers", "provider", id)
		}
		baseURL, err := resolver.ResolveValue(providerConfig.BaseURL)
		if baseURL == "" || err != nil {
			slog.Warn("Skipping custom provider due to missing API endpoint", "provider", id, "error", err)
			c.Providers.Del(id)
			continue
		}

		for k, v := range providerConfig.ExtraHeaders {
			resolved, err := resolver.ResolveValue(v)
			if err != nil {
				slog.Error("Could not resolve provider header", "err", err.Error())
				continue
			}
			providerConfig.ExtraHeaders[k] = resolved
		}

		c.Providers.Set(id, providerConfig)
	}

	if c.Providers.Len() == 0 && c.Options.DisableDefaultProviders {
		return fmt.Errorf("default providers are disabled and there are no custom providers are configured")
	}

	// Enrich all provider models with metadata from models.dev.
	// This fills in missing context window, costs, capabilities, etc.
	if devData := GetModelsDevData(); len(devData) > 0 {
		for id, pc := range c.Providers.Seq2() {
			for i := range pc.Models {
				EnrichModel(&pc.Models[i], devData)
			}
			c.Providers.Set(id, pc)
		}
	}

	return nil
}

func (c *Config) setDefaults(workingDir, projectDataDir string) {
	if c.Options == nil {
		c.Options = &Options{}
	}
	if c.Options.TUI == nil {
		c.Options.TUI = &TUIOptions{}
	}
	// DataDirectory is now always the centralized project data directory
	if projectDataDir != "" {
		c.Options.DataDirectory = projectDataDir
	} else {
		c.Options.DataDirectory = ProjectDataDir(workingDir)
	}
	if c.Providers == nil {
		c.Providers = csync.NewMap[string, ProviderConfig]()
	}
	if c.Models == nil {
		c.Models = make(map[SelectedModelType]SelectedModel)
	}
	if c.RecentModels == nil {
		c.RecentModels = make(map[SelectedModelType][]SelectedModel)
	}
	if c.MCP == nil {
		c.MCP = make(map[string]MCPConfig)
	}
	if c.LSP == nil {
		c.LSP = make(map[string]LSPConfig)
	}

	// Apply defaults to LSP configurations
	c.applyLSPDefaults()

	// Add the default context paths if they are not already present
	c.Options.ContextPaths = append(defaultContextPaths, c.Options.ContextPaths...)
	slices.Sort(c.Options.ContextPaths)
	c.Options.ContextPaths = slices.Compact(c.Options.ContextPaths)

	// Add the default skills directories if not already present.
	for _, dir := range GlobalSkillsDirs() {
		if !slices.Contains(c.Options.SkillsPaths, dir) {
			c.Options.SkillsPaths = append(c.Options.SkillsPaths, dir)
		}
	}

	// Project specific skills dirs.
	c.Options.SkillsPaths = append(c.Options.SkillsPaths, ProjectSkillsDir(workingDir)...)

	if str, ok := os.LookupEnv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE"); ok {
		c.Options.DisableProviderAutoUpdate, _ = strconv.ParseBool(str)
	}

	if str, ok := os.LookupEnv("CRUSH_DISABLE_DEFAULT_PROVIDERS"); ok {
		c.Options.DisableDefaultProviders, _ = strconv.ParseBool(str)
	}

	if c.Options.Attribution == nil {
		c.Options.Attribution = &Attribution{
			TrailerStyle:  TrailerStyleAssistedBy,
			GeneratedWith: true,
		}
	} else if c.Options.Attribution.TrailerStyle == "" {
		// Migrate deprecated co_authored_by or apply default
		if c.Options.Attribution.CoAuthoredBy != nil {
			if *c.Options.Attribution.CoAuthoredBy {
				c.Options.Attribution.TrailerStyle = TrailerStyleCoAuthoredBy
			} else {
				c.Options.Attribution.TrailerStyle = TrailerStyleNone
			}
		} else {
			c.Options.Attribution.TrailerStyle = TrailerStyleAssistedBy
		}
	}
	c.Options.InitializeAs = cmp.Or(c.Options.InitializeAs, defaultInitializeAs)
	if c.Options.PreferredPermissionMode == "" {
		c.Options.PreferredPermissionMode = cmp.Or(c.Options.PreferredCollaborationMode, "auto")
	}
}

// applyLSPDefaults applies default values from powernap to LSP configurations
func (c *Config) applyLSPDefaults() {
	// Get powernap's default configuration
	configManager := powernapConfig.NewManager()
	configManager.LoadDefaults()

	// Apply defaults to each LSP configuration
	for name, cfg := range c.LSP {
		// Try to get defaults from powernap based on name or command name.
		base, ok := configManager.GetServer(name)
		if !ok {
			base, ok = configManager.GetServer(cfg.Command)
			if !ok {
				continue
			}
		}
		if cfg.Options == nil {
			cfg.Options = base.Settings
		}
		if cfg.InitOptions == nil {
			cfg.InitOptions = base.InitOptions
		}
		if len(cfg.FileTypes) == 0 {
			cfg.FileTypes = base.FileTypes
		}
		if len(cfg.RootMarkers) == 0 {
			cfg.RootMarkers = base.RootMarkers
		}
		cfg.Command = cmp.Or(cfg.Command, base.Command)
		if len(cfg.Args) == 0 {
			cfg.Args = base.Args
		}
		if len(cfg.Env) == 0 {
			cfg.Env = base.Environment
		}
		// Update the config in the map
		c.LSP[name] = cfg
	}
}

func (c *Config) defaultModelSelection(knownProviders []catwalk.Provider) (largeModel SelectedModel, smallModel SelectedModel, err error) {
	if len(knownProviders) == 0 && c.Providers.Len() == 0 {
		err = fmt.Errorf("no providers configured, please configure at least one provider")
		return largeModel, smallModel, err
	}

	// Use the first provider enabled based on the known providers order
	// if no provider found that is known use the first provider configured
	for _, p := range knownProviders {
		providerConfig, ok := c.Providers.Get(string(p.ID))
		if !ok || providerConfig.Disable {
			continue
		}
		defaultLargeModel := c.GetModel(string(p.ID), p.DefaultLargeModelID)
		if defaultLargeModel == nil {
			err = fmt.Errorf("default large model %s not found for provider %s", p.DefaultLargeModelID, p.ID)
			return largeModel, smallModel, err
		}
		largeModel = SelectedModel{
			Provider:  string(p.ID),
			Model:     defaultLargeModel.ID,
			MaxTokens: defaultLargeModel.DefaultMaxTokens,
		}

		defaultSmallModel := c.GetModel(string(p.ID), p.DefaultSmallModelID)
		if defaultSmallModel == nil {
			err = fmt.Errorf("default small model %s not found for provider %s", p.DefaultSmallModelID, p.ID)
			return largeModel, smallModel, err
		}
		smallModel = SelectedModel{
			Provider:  string(p.ID),
			Model:     defaultSmallModel.ID,
			MaxTokens: defaultSmallModel.DefaultMaxTokens,
		}
		return largeModel, smallModel, err
	}

	enabledProviders := c.EnabledProviders()
	slices.SortFunc(enabledProviders, func(a, b ProviderConfig) int {
		return strings.Compare(a.ID, b.ID)
	})

	if len(enabledProviders) == 0 {
		err = fmt.Errorf("no providers configured, please configure at least one provider")
		return largeModel, smallModel, err
	}

	providerConfig := enabledProviders[0]
	if len(providerConfig.Models) == 0 {
		err = fmt.Errorf("provider %s has no models configured", providerConfig.ID)
		return largeModel, smallModel, err
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
	return largeModel, smallModel, err
}

func configureSelectedModels(store *ConfigStore, knownProviders []catwalk.Provider) error {
	c := store.config
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
			err := store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeLarge, large)
			if err != nil {
				return fmt.Errorf("failed to update preferred large model: %w", err)
			}
		} else {
			if largeModelSelected.MaxTokens > 0 {
				large.MaxTokens = largeModelSelected.MaxTokens
			} else {
				large.MaxTokens = model.DefaultMaxTokens
			}
			if largeModelSelected.ContextWindow > 0 {
				large.ContextWindow = largeModelSelected.ContextWindow
			}
			if largeModelSelected.Temperature != nil {
				large.Temperature = largeModelSelected.Temperature
			}
			if largeModelSelected.TopP != nil {
				large.TopP = largeModelSelected.TopP
			}
			if largeModelSelected.TopK != nil {
				large.TopK = largeModelSelected.TopK
			}
			if largeModelSelected.FrequencyPenalty != nil {
				large.FrequencyPenalty = largeModelSelected.FrequencyPenalty
			}
			if largeModelSelected.PresencePenalty != nil {
				large.PresencePenalty = largeModelSelected.PresencePenalty
			}
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
			err := store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeSmall, small)
			if err != nil {
				return fmt.Errorf("failed to update preferred small model: %w", err)
			}
		} else {
			if smallModelSelected.MaxTokens > 0 {
				small.MaxTokens = smallModelSelected.MaxTokens
			} else {
				small.MaxTokens = model.DefaultMaxTokens
			}
			if smallModelSelected.ContextWindow > 0 {
				small.ContextWindow = smallModelSelected.ContextWindow
			}
			if smallModelSelected.Temperature != nil {
				small.Temperature = smallModelSelected.Temperature
			}
			if smallModelSelected.TopP != nil {
				small.TopP = smallModelSelected.TopP
			}
			if smallModelSelected.TopK != nil {
				small.TopK = smallModelSelected.TopK
			}
			if smallModelSelected.FrequencyPenalty != nil {
				small.FrequencyPenalty = smallModelSelected.FrequencyPenalty
			}
			if smallModelSelected.PresencePenalty != nil {
				small.PresencePenalty = smallModelSelected.PresencePenalty
			}
		}
	}
	background := large
	backgroundModelSelected, backgroundModelConfigured := c.Models[SelectedModelTypeBackground]
	if !backgroundModelConfigured {
		handoffModelSelected, handoffModelConfigured := c.Models[SelectedModelTypeHandoff]
		if handoffModelConfigured {
			backgroundModelSelected = handoffModelSelected
			backgroundModelConfigured = true
		}
	}
	if backgroundModelConfigured {
		background = large
		if backgroundModelSelected.Model != "" {
			background.Model = backgroundModelSelected.Model
		}
		if backgroundModelSelected.Provider != "" {
			background.Provider = backgroundModelSelected.Provider
		}

		model := c.GetModel(background.Provider, background.Model)
		if model == nil {
			background = large
			err := store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeBackground, background)
			if err != nil {
				return fmt.Errorf("failed to update preferred background model: %w", err)
			}
		} else {
			if backgroundModelSelected.MaxTokens > 0 {
				background.MaxTokens = backgroundModelSelected.MaxTokens
			} else {
				background.MaxTokens = model.DefaultMaxTokens
			}
			if backgroundModelSelected.ContextWindow > 0 {
				background.ContextWindow = backgroundModelSelected.ContextWindow
			}
			if backgroundModelSelected.Temperature != nil {
				background.Temperature = backgroundModelSelected.Temperature
			}
			if backgroundModelSelected.TopP != nil {
				background.TopP = backgroundModelSelected.TopP
			}
			if backgroundModelSelected.TopK != nil {
				background.TopK = backgroundModelSelected.TopK
			}
			if backgroundModelSelected.FrequencyPenalty != nil {
				background.FrequencyPenalty = backgroundModelSelected.FrequencyPenalty
			}
			if backgroundModelSelected.PresencePenalty != nil {
				background.PresencePenalty = backgroundModelSelected.PresencePenalty
			}
			if backgroundModelSelected.Think != nil {
				background.Think = backgroundModelSelected.Think
			}
		}
	}
	migrateLegacyAutoClassifierSelection(c.Models)
	autoClassifier := resolveConfiguredModel(store, c, SelectedModelTypeAutoClassifier, resolveAutoClassifierFallback(c, backgroundModelConfigured, background, small, large))
	c.Models[SelectedModelTypeLarge] = large
	c.Models[SelectedModelTypeSmall] = small
	c.Models[SelectedModelTypeBackground] = background
	c.Models[SelectedModelTypeAutoClassifier] = autoClassifier
	delete(c.Models, SelectedModelTypeHandoff)
	return nil
}

func migrateLegacyAutoClassifierSelection(models map[SelectedModelType]SelectedModel) {
	if models == nil {
		return
	}

	if _, exists := models[SelectedModelTypeAutoClassifier]; !exists {
		switch {
		case models[SelectedModelTypeAutoClassifierReasoning].Model != "":
			models[SelectedModelTypeAutoClassifier] = models[SelectedModelTypeAutoClassifierReasoning]
		case models[SelectedModelTypeAutoClassifierFast].Model != "":
			models[SelectedModelTypeAutoClassifier] = models[SelectedModelTypeAutoClassifierFast]
		}
	}

	delete(models, SelectedModelTypeAutoClassifierFast)
	delete(models, SelectedModelTypeAutoClassifierReasoning)
}

func resolveConfiguredModel(store *ConfigStore, c *Config, modelType SelectedModelType, fallback SelectedModel) SelectedModel {
	selected, configured := c.Models[modelType]
	if !configured {
		return fallback
	}

	resolved := fallback
	if selected.Model != "" {
		resolved.Model = selected.Model
	}
	if selected.Provider != "" {
		resolved.Provider = selected.Provider
	}

	model := c.GetModel(resolved.Provider, resolved.Model)
	if model == nil {
		if err := store.UpdatePreferredModel(ScopeGlobal, modelType, fallback); err != nil {
			slog.Warn("Failed to update preferred model fallback", "model_type", modelType, "error", err)
		}
		return fallback
	}

	if selected.MaxTokens > 0 {
		resolved.MaxTokens = selected.MaxTokens
	} else {
		resolved.MaxTokens = model.DefaultMaxTokens
	}
	if selected.ContextWindow > 0 {
		resolved.ContextWindow = selected.ContextWindow
	}
	if selected.Temperature != nil {
		resolved.Temperature = selected.Temperature
	}
	if selected.TopP != nil {
		resolved.TopP = selected.TopP
	}
	if selected.TopK != nil {
		resolved.TopK = selected.TopK
	}
	if selected.FrequencyPenalty != nil {
		resolved.FrequencyPenalty = selected.FrequencyPenalty
	}
	if selected.PresencePenalty != nil {
		resolved.PresencePenalty = selected.PresencePenalty
	}
	if selected.Think != nil {
		resolved.Think = selected.Think
	}

	return resolved
}

func resolveAutoClassifierFallback(c *Config, backgroundConfigured bool, background, small, large SelectedModel) SelectedModel {
	if model := c.GetModel(small.Provider, small.Model); model != nil && model.CanReason {
		return small
	}
	if backgroundConfigured {
		if model := c.GetModel(background.Provider, background.Model); model != nil {
			return background
		}
	}
	if model := c.GetModel(small.Provider, small.Model); model != nil {
		return small
	}
	return large
}

// lookupConfigs searches config files recursively from CWD up to FS root
func lookupConfigs(cwd string) []string {
	// prepend default config paths
	configPaths := []string{
		GlobalConfig(),
		GlobalConfigData(),
	}

	configNames := []string{appName + ".json", "." + appName + ".json"}

	foundConfigs, err := fsext.Lookup(cwd, configNames...)
	if err != nil {
		// returns at least default configs
		return configPaths
	}

	// reverse order so last config has more priority
	slices.Reverse(foundConfigs)

	return append(configPaths, foundConfigs...)
}

func loadFromConfigPaths(configPaths []string) (*Config, error) {
	var configs [][]byte

	for _, path := range configPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to open config file %s: %w", path, err)
		}
		if len(data) == 0 {
			continue
		}
		configs = append(configs, data)
	}

	return loadFromBytes(configs)
}

func loadFromBytes(configs [][]byte) (*Config, error) {
	if len(configs) == 0 {
		return &Config{}, nil
	}

	data, err := jsons.Merge(configs)
	if err != nil {
		return nil, err
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func hasAWSCredentials(env env.Env) bool {
	if env.Get("AWS_BEARER_TOKEN_BEDROCK") != "" {
		return true
	}

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

	if _, err := os.Stat(filepath.Join(home.Dir(), ".aws/credentials")); err == nil && !testing.Testing() {
		return true
	}

	return false
}

// ProjectDataDir returns the centralized project data directory path.
// This is used for storing project-scoped data like sessions, memory, and logs
// in a central location rather than in the project directory itself.
// The path is: <global-data-root>/projects/<project-slug>/
// where project-slug is based on the git root (if available) or working directory.
func ProjectDataDir(workingDir string) string {
	root := filepath.Dir(GlobalConfigData())
	projectSlug := projectSlugFromDir(workingDir)
	return filepath.Join(root, "projects", projectSlug)
}

// projectSlugFromDir generates a unique, readable slug for a project directory.
// It uses the git root if available, otherwise the working directory.
// Format: <basename>-<hash6> (e.g., "crush-a1b2c3")
func projectSlugFromDir(workingDir string) string {
	identity := workspaceIdentityDir(workingDir)

	// Try to get git root for better identity
	gitRoot := findGitRoot(identity)
	if gitRoot != "" {
		identity = gitRoot
	}

	return workspaceDataDirName(identity)
}

// findGitRoot finds the canonical git repository root.
// For worktrees, it returns the main repo's common dir (not the worktree-specific .git).
// This ensures all worktrees of the same repo share the same project data.
func findGitRoot(workingDir string) string {
	// Use --git-common-dir to get the main repo's .git path for worktrees
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = workingDir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	gitCommonDir := strings.TrimSpace(string(output))
	if gitCommonDir == "" {
		return ""
	}

	// gitCommonDir could be relative or absolute
	if !filepath.IsAbs(gitCommonDir) {
		gitCommonDir = filepath.Join(workingDir, gitCommonDir)
	}

	// The git root is the parent of .git (or .git/worktrees/xxx for worktrees)
	// For worktrees, we want the main repo's root, so we navigate up from .git
	gitDir := filepath.Clean(gitCommonDir)

	// Handle worktree case: .git/worktrees/<name>
	// Navigate to find the actual repository root
	if filepath.Base(filepath.Dir(gitDir)) == "worktrees" {
		// This is a worktree, the common dir is in the main repo's .git
		// Go up to find the main repo's root
		gitDir = filepath.Dir(filepath.Dir(gitDir))
	}

	// Now gitDir should be the .git directory, parent is the repo root
	repoRoot := filepath.Dir(gitDir)

	// Verify this is actually the root by checking if .git exists there.
	// .git might be a file (worktree pointing to main repo), which is fine.
	if _, err := os.Stat(filepath.Join(repoRoot, ".git")); err != nil {
		return ""
	}

	return repoRoot
}

// GlobalConfig returns the global configuration file path for the application.
func GlobalConfig() string {
	if crushGlobal := os.Getenv("CRUSH_GLOBAL_CONFIG"); crushGlobal != "" {
		return filepath.Join(crushGlobal, fmt.Sprintf("%s.json", appName))
	}
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, appName, fmt.Sprintf("%s.json", appName))
	}
	return filepath.Join(home.Dir(), ".config", appName, fmt.Sprintf("%s.json", appName))
}

// GlobalConfigData returns the path to the main data directory for the application.
// this config is used when the app overrides configurations instead of updating the global config.
func GlobalConfigData() string {
	if crushData := os.Getenv("CRUSH_GLOBAL_DATA"); crushData != "" {
		return filepath.Join(crushData, fmt.Sprintf("%s.json", appName))
	}
	if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
		return filepath.Join(xdgDataHome, appName, fmt.Sprintf("%s.json", appName))
	}

	// return the path to the main data directory
	// for windows, it should be in `%LOCALAPPDATA%/crush/`
	// for linux and macOS, it should be in `$HOME/.local/share/crush/`
	if runtime.GOOS == "windows" {
		localAppData := cmp.Or(
			os.Getenv("LOCALAPPDATA"),
			filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local"),
		)
		return filepath.Join(localAppData, appName, fmt.Sprintf("%s.json", appName))
	}

	return filepath.Join(home.Dir(), ".local", "share", appName, fmt.Sprintf("%s.json", appName))
}

func assignIfNil[T any](ptr **T, val T) {
	if *ptr == nil {
		*ptr = &val
	}
}

func shouldUseGlobalWorkspaceDataDir(workingDir string) bool {
	return shouldUseGlobalWorkspaceDataDirForOS(runtime.GOOS, workingDir, os.Getenv("WINDIR"))
}

func shouldUseGlobalWorkspaceDataDirForOS(goos, workingDir, windowsDir string) bool {
	clean := filepath.Clean(strings.TrimSpace(workingDir))
	if clean == "" || clean == "." {
		return true
	}

	if goos == "windows" {
		system32 := filepath.Join(cmp.Or(strings.TrimSpace(windowsDir), `C:\Windows`), "System32")
		return isPathWithin(clean, system32)
	}

	return clean == string(filepath.Separator)
}

func isPathWithin(path, parent string) bool {
	path = filepath.Clean(path)
	parent = filepath.Clean(parent)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
		parent = strings.ToLower(parent)
	}
	if path == parent {
		return true
	}
	parentWithSep := parent + string(filepath.Separator)
	return strings.HasPrefix(path, parentWithSep)
}

func globalWorkspaceDataDir(workingDir string) string {
	root := filepath.Dir(GlobalConfigData())
	return filepath.Join(root, "workspaces", workspaceDataDirName(workingDir))
}

func workspaceIdentityDir(workingDir string) string {
	for _, key := range []string{
		"CRUSH_WORKSPACE_CWD",
		"ZED_WORKSPACE_ROOT",
		"ZED_WORKTREE_ROOT",
		"ZED_CWD",
		"VSCODE_CWD",
		"PROJECT_ROOT",
		"WORKSPACE_ROOT",
		"INIT_CWD",
		"PWD",
	} {
		value := normalizeWorkspaceIdentityDir(os.Getenv(key))
		if value == "" {
			continue
		}
		if shouldUseGlobalWorkspaceDataDir(value) {
			continue
		}
		return value
	}
	return normalizeWorkspaceIdentityDir(workingDir)
}

func normalizeWorkspaceIdentityDir(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(dir)
}

func workspaceDataDirName(workingDir string) string {
	normalized := filepath.Clean(strings.TrimSpace(workingDir))
	if normalized == "" || normalized == "." {
		normalized = "workspace"
	}
	if abs, err := filepath.Abs(normalized); err == nil {
		normalized = filepath.Clean(abs)
	}
	sum := sha256.Sum256([]byte(filepath.ToSlash(normalized)))
	hash := hex.EncodeToString(sum[:6])
	base := sanitizeWorkspaceSegment(filepath.Base(normalized))
	if base == "" || base == "." || base == string(filepath.Separator) {
		base = "workspace"
	}
	return fmt.Sprintf("%s-%s", base, hash)
}

func sanitizeWorkspaceSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	sanitized := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, value)
	return strings.Trim(sanitized, "_")
}

func isInsideWorktree() bool {
	bts, err := exec.CommandContext(
		context.Background(),
		"git", "rev-parse",
		"--is-inside-work-tree",
	).CombinedOutput()
	return err == nil && strings.TrimSpace(string(bts)) == "true"
}

// GlobalSkillsDirs returns the default directories for Agent Skills.
// Skills in these directories are auto-discovered and their files can be read
// without permission prompts.
func GlobalSkillsDirs() []string {
	if crushSkills := os.Getenv("CRUSH_SKILLS_DIR"); crushSkills != "" {
		return []string{crushSkills}
	}

	configHome := cmp.Or(
		os.Getenv("XDG_CONFIG_HOME"),
		filepath.Join(home.Dir(), ".config"),
	)

	paths := []string{
		filepath.Join(configHome, appName, "skills"),
		filepath.Join(configHome, "agents", "skills"),
	}

	// On Windows, also load from app data on top of `$HOME/.config/crush`.
	// This is here mostly for backwards compatibility.
	if runtime.GOOS == "windows" {
		appData := cmp.Or(
			os.Getenv("LOCALAPPDATA"),
			filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local"),
		)
		paths = append(
			paths,
			filepath.Join(appData, appName, "skills"),
			filepath.Join(appData, "agents", "skills"),
		)
	}

	return paths
}

// ProjectSkillsDir returns the default project directories for which Crush
// will look for skills.
func ProjectSkillsDir(workingDir string) []string {
	return []string{
		filepath.Join(workingDir, ".agents/skills"),
		filepath.Join(workingDir, ".crush/skills"),
		filepath.Join(workingDir, ".claude/skills"),
		filepath.Join(workingDir, ".cursor/skills"),
	}
}

// GlobalAgentsMD returns the path to the global AGENTS.md file.
// This file is loaded as a context file for all projects.
func GlobalAgentsMD() string {
	if crushGlobal := os.Getenv("CRUSH_GLOBAL_CONFIG"); crushGlobal != "" {
		return filepath.Join(crushGlobal, "AGENTS.md")
	}
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, appName, "AGENTS.md")
	}
	return filepath.Join(home.Dir(), ".config", appName, "AGENTS.md")
}

func isAppleTerminal() bool { return os.Getenv("TERM_PROGRAM") == "Apple_Terminal" }
