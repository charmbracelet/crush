package config

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/discover"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/charmbracelet/crush/internal/filepathext"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/home"
	powernapConfig "github.com/charmbracelet/x/powernap/pkg/config"
	"github.com/qjebbs/go-jsons"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const defaultCatwalkURL = "https://catwalk.charm.land"

// Load loads the configuration from the default paths and returns a
// ConfigStore that owns both the pure-data Config and all runtime state.
func Load(workingDir, dataDir string, debug bool) (*ConfigStore, error) {
	// Migrate deprecated disable_notifications before loading config.
	migrateDisableNotifications()

	configPaths := lookupConfigs(workingDir)

	cfg, loadedPaths, err := loadFromConfigPaths(configPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from paths %v: %w", configPaths, err)
	}

	cfg.setDefaults(workingDir, dataDir)

	store := &ConfigStore{
		config:         cfg,
		workingDir:     workingDir,
		globalDataPath: GlobalConfigData(),
		workspacePath:  filepath.Join(cfg.Options.DataDirectory, fmt.Sprintf("%s.json", appName)),
		loadedPaths:    loadedPaths,
	}

	if debug {
		cfg.Options.Debug = true
	}

	// Load workspace config last so it has highest priority.
	if wsData, err := os.ReadFile(store.workspacePath); err == nil && len(wsData) > 0 {
		if !json.Valid(wsData) {
			return nil, fmt.Errorf("invalid JSON in config file %s", store.workspacePath)
		}
		merged, mergeErr := loadFromBytes(append([][]byte{mustMarshalConfig(cfg)}, wsData))
		if mergeErr == nil {
			// Preserve defaults that setDefaults already applied.
			dataDir := cfg.Options.DataDirectory
			*cfg = *merged
			cfg.setDefaults(workingDir, dataDir)
			store.config = cfg
			store.loadedPaths = append(store.loadedPaths, store.workspacePath)
		}
	}

	// Validate hooks after all config merging is complete so workspace
	// hooks also get their matcher regexes compiled.
	if err := cfg.ValidateHooks(); err != nil {
		return nil, fmt.Errorf("invalid hook configuration: %w", err)
	}

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

	// Hold writeMu during initial load to prevent configureProviders
	// from triggering auto-reload via RemoveConfigField.
	store.writeMu.Lock()
	defer store.writeMu.Unlock()

	if err := cfg.configureProviders(context.Background(), store, env, valueResolver, store.knownProviders); err != nil {
		return nil, fmt.Errorf("failed to configure providers: %w", err)
	}

	if !cfg.IsConfigured() {
		slog.Warn("No providers configured")
		return store, nil
	}

	if err := configureSelectedModels(store, store.knownProviders, true); err != nil {
		return nil, fmt.Errorf("failed to configure selected models: %w", err)
	}
	store.SetupAgents()

	// Capture initial staleness snapshot
	store.captureStalenessSnapshot(loadedPaths)

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

func (c *Config) configureProviders(ctx context.Context, store *ConfigStore, env env.Env, resolver VariableResolver, knownProviders []catwalk.Provider) error {
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
			if config.Disable {
				continue
			}
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
		// Provider headers use the same error contract as MCP headers:
		// a failing $(...) aborts the provider load with a clear
		// message, and a header that resolves to the empty string
		// (unset bare $VAR under lenient nounset, $(echo), or literal
		// "") is dropped from the outgoing request.
		for k, v := range headers {
			resolved, err := resolver.ResolveValue(v)
			if err != nil {
				return fmt.Errorf("resolving provider %s header %q: %w", p.ID, k, err)
			}
			if resolved == "" {
				delete(headers, k)
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
			Models:             p.Models,
		}

		switch {
		case p.ID == catwalk.InferenceProviderAnthropic && config.OAuthToken != nil:
			// Claude Code subscription is not supported anymore. Remove to show onboarding.
			// RemoveConfigField persists the deletion to disk. The in-memory
			// state is kept consistent by the Providers.Del call below; any
			// concurrent reload that races with this write will also see the
			// removal because it re-reads from disk.
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
		case catwalk.InferenceProviderBedrock, catwalk.InferenceProviderBedrockEurope:
			if p.APIKey == "" && !hasAWSCredentials(env) {
				if configExists {
					slog.Warn("Skipping Bedrock provider due to missing AWS credentials")
					c.Providers.Del(string(p.ID))
				}
				continue
			}
		case catwalk.InferenceProvider("hyper"):
			if apiKey := env.Get("HYPER_API_KEY"); apiKey != "" {
				prepared.APIKey = apiKey
				prepared.APIKeyTemplate = apiKey
			} else {
				v, err := resolver.ResolveValue(p.APIKey)
				if v == "" || err != nil {
					if configExists {
						slog.Warn("Skipping Hyper provider due to missing API key", "provider", p.ID)
						c.Providers.Del(string(p.ID))
					}
					continue
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

	// Discover models concurrently for custom providers that need it.
	// A provider needs discovery when discover_models is explicitly true,
	// or when the models list is empty (auto-trigger, unless opted out).
	type discoveryResult struct {
		models []catwalk.Model
		err    error
	}

	discoveryResults := make(map[string]discoveryResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	discoverCtx, discoverCancel := context.WithTimeout(ctx, 3*time.Second)
	for id, pc := range c.Providers.Seq2() {
		if knownProviderNames[id] {
			continue
		}
		if pc.Disable || pc.BaseURL == "" {
			continue
		}
		wantsDiscovery := pc.AutoDiscoverModels != nil && *pc.AutoDiscoverModels
		autoTrigger := len(pc.Models) == 0 && (pc.AutoDiscoverModels == nil || *pc.AutoDiscoverModels)
		if !wantsDiscovery && !autoTrigger {
			continue
		}
		providerID := cmp.Or(pc.ID, id)
		cfg := discover.Config{
			ID:             providerID,
			BaseURL:        pc.BaseURL,
			APIKey:         pc.APIKey,
			ExtraHeaders:   pc.ExtraHeaders,
			ExistingModels: pc.Models,
		}
		providerType := cmp.Or(pc.Type, catwalk.TypeOpenAICompat)
		wg.Go(func() {
			models, err := discover.DiscoverModels(discoverCtx, cfg, resolver)
			if err == nil && len(models) > 0 {
				if enricher := discover.GetEnricher(string(providerType)); enricher != nil {
					models, _ = enricher.EnrichModels(discoverCtx, cfg, resolver, models)
				}
			}
			mu.Lock()
			discoveryResults[id] = discoveryResult{models: models, err: err}
			mu.Unlock()
		})
	}
	wg.Wait()
	discoverCancel()

	// Validate the custom providers.
	for id, providerConfig := range c.Providers.Seq2() {
		if knownProviderNames[id] {
			continue
		}

		// Make sure the provider ID is set.
		providerConfig.ID = id
		providerConfig.Name = cmp.Or(providerConfig.Name, id) // Use ID as name if not set
		// Default to OpenAI if not set.
		providerConfig.Type = cmp.Or(providerConfig.Type, catwalk.TypeOpenAICompat)
		if !slices.Contains(catwalk.KnownProviderTypes(), providerConfig.Type) &&
			providerConfig.Type != hyper.Name &&
			!discover.IsKnownCustomProvider(string(providerConfig.Type)) {
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

		// Apply discovery results if available.
		if result, ok := discoveryResults[id]; ok {
			if result.err != nil {
				slog.Warn("Model discovery failed", "provider", id, "error", result.err)
				if len(providerConfig.Models) == 0 {
					slog.Warn("Skipping provider with no models after failed discovery", "provider", id)
					c.Providers.Del(id)
					continue
				}
			} else if len(result.models) > 0 {
				providerConfig.Models = result.models
				slog.Info("Discovered models for provider", "provider", id, "count", len(result.models))
			}
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

		// Custom-provider headers share the MCP error contract; see
		// the known-provider loop above.
		for k, v := range providerConfig.ExtraHeaders {
			resolved, err := resolver.ResolveValue(v)
			if err != nil {
				return fmt.Errorf("resolving provider %s header %q: %w", id, k, err)
			}
			if resolved == "" {
				delete(providerConfig.ExtraHeaders, k)
				continue
			}
			providerConfig.ExtraHeaders[k] = resolved
		}

		c.Providers.Set(id, providerConfig)
	}

	if c.Providers.Len() == 0 && c.Options.DisableDefaultProviders {
		return fmt.Errorf("default providers are disabled and there are no custom providers are configured")
	}

	return nil
}

func (c *Config) setDefaults(workingDir, dataDir string) {
	if c.Options == nil {
		c.Options = &Options{}
	}
	if c.Options.TUI == nil {
		c.Options.TUI = &TUIOptions{}
	}
	if len(c.Options.GlobalContextPaths) == 0 {
		crushConfigDir := filepath.Dir(GlobalConfig())
		c.Options.GlobalContextPaths = []string{
			filepath.Join(crushConfigDir, "CRUSH.md"),
			filepath.Join(filepath.Dir(crushConfigDir), "AGENTS.md"),
		}
	}
	slices.Sort(c.Options.GlobalContextPaths)
	c.Options.GlobalContextPaths = slices.Compact(c.Options.GlobalContextPaths)

	if dataDir != "" {
		c.Options.DataDirectory = dataDir
	} else if c.Options.DataDirectory == "" {
		if path, ok := fsext.LookupClosestBounded(workingDir, projectBoundary(workingDir), defaultDataDirectory); ok {
			c.Options.DataDirectory = path
		} else {
			c.Options.DataDirectory = filepath.Join(workingDir, defaultDataDirectory)
		}
	}
	c.Options.DataDirectory = filepath.Clean(filepathext.SmartJoin(workingDir, c.Options.DataDirectory))
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
	c.Options.ContextPaths = append(slices.Clone(defaultContextPaths), c.Options.ContextPaths...)

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
}

// powernapDefaults caches the powernap default LSP server catalog. The
// catalog is static and immutable for the life of the process, but
// building it (NewManager + LoadDefaults) is expensive and was previously
// repeated on every config reload. We load it once and only ever read from
// it via GetServer, so a shared instance is safe.
var (
	powernapDefaultsOnce sync.Once
	powernapDefaults     *powernapConfig.Manager
)

func lspDefaultsManager() *powernapConfig.Manager {
	powernapDefaultsOnce.Do(func() {
		m := powernapConfig.NewManager()
		// LoadDefaults only fails on malformed embedded defaults, which
		// would be a build-time bug; treat the manager as usable either
		// way so a transient error never wedges config loading.
		_ = m.LoadDefaults()
		powernapDefaults = m
	})
	return powernapDefaults
}

// applyLSPDefaults applies default values from powernap to LSP configurations
func (c *Config) applyLSPDefaults() {
	// Reuse the process-wide default catalog; building it per reload was a
	// significant chunk of reload latency.
	configManager := lspDefaultsManager()

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
			slog.Warn("Default large model %s not found for provider %s", p.DefaultLargeModelID, p.ID)
			if len(providerConfig.Models) == 0 {
				return largeModel, smallModel, fmt.Errorf("default large model %s not found for provider %s", p.DefaultLargeModelID, p.ID)
			}
			defaultLargeModel = &providerConfig.Models[0]
		}
		largeModel = SelectedModel{
			Provider:        string(p.ID),
			Model:           defaultLargeModel.ID,
			MaxTokens:       defaultLargeModel.DefaultMaxTokens,
			ReasoningEffort: defaultLargeModel.DefaultReasoningEffort,
		}

		defaultSmallModel := c.GetModel(string(p.ID), p.DefaultSmallModelID)
		if defaultSmallModel == nil {
			slog.Warn("Default small model %s not found for provider %s", p.DefaultSmallModelID, p.ID)
			if len(providerConfig.Models) == 0 {
				return largeModel, smallModel, fmt.Errorf("default small model %s not found for provider %s", p.DefaultSmallModelID, p.ID)
			}
			defaultSmallModel = &providerConfig.Models[0]
		}
		smallModel = SelectedModel{
			Provider:        string(p.ID),
			Model:           defaultSmallModel.ID,
			MaxTokens:       defaultSmallModel.DefaultMaxTokens,
			ReasoningEffort: defaultSmallModel.DefaultReasoningEffort,
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

func configureSelectedModels(store *ConfigStore, knownProviders []catwalk.Provider, persist bool) error {
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
			if persist {
				if err := store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeLarge, large); err != nil {
					return fmt.Errorf("failed to update preferred large model: %w", err)
				}
			}
		} else {
			if largeModelSelected.MaxTokens > 0 {
				large.MaxTokens = largeModelSelected.MaxTokens
			} else {
				large.MaxTokens = model.DefaultMaxTokens
			}
			if largeModelSelected.ReasoningEffort != "" {
				large.ReasoningEffort = largeModelSelected.ReasoningEffort
			} else {
				large.ReasoningEffort = model.DefaultReasoningEffort
			}
			large.Think = largeModelSelected.Think
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
			if persist {
				if err := store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeSmall, small); err != nil {
					return fmt.Errorf("failed to update preferred small model: %w", err)
				}
			}
		} else {
			if smallModelSelected.MaxTokens > 0 {
				small.MaxTokens = smallModelSelected.MaxTokens
			} else {
				small.MaxTokens = model.DefaultMaxTokens
			}
			if smallModelSelected.ReasoningEffort != "" {
				small.ReasoningEffort = smallModelSelected.ReasoningEffort
			} else {
				small.ReasoningEffort = model.DefaultReasoningEffort
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
			small.Think = smallModelSelected.Think
		}
	}

	// When small isn't explicitly configured and the provider isn't a
	// known built-in, use the large model as the small model. This
	// prevents two different models from being requested concurrently
	// for local/openai-compat providers.
	if !smallModelConfigured {
		isKnownProvider := false
		for _, kp := range knownProviders {
			if string(kp.ID) == small.Provider {
				isKnownProvider = true
				break
			}
		}
		if !isKnownProvider {
			slog.Warn("Using large model as small model for unknown provider", "provider", large.Provider, "model", large.Model)
			small = large
		}
	}

	c.Models[SelectedModelTypeLarge] = large
	c.Models[SelectedModelTypeSmall] = small
	return nil
}

// lookupConfigs searches config files starting at cwd and walking up
// through the current project. The upward walk stops at the git
// working tree root when one can be detected, otherwise at cwd itself,
// so an unrelated crush.json placed above the project is never picked
// up. Global user-level config locations are always included
// regardless of the boundary.
func lookupConfigs(cwd string) []string {
	// prepend default config paths
	configPaths := []string{
		GlobalConfig(),
		GlobalConfigData(),
	}

	configNames := []string{appName + ".json", "." + appName + ".json"}

	foundConfigs, err := fsext.LookupBounded(cwd, projectBoundary(cwd), configNames...)
	if err != nil {
		// returns at least default configs
		return configPaths
	}

	// reverse order so last config has more priority
	slices.Reverse(foundConfigs)

	return append(configPaths, foundConfigs...)
}

func loadFromConfigPaths(configPaths []string) (*Config, []string, error) {
	var configs [][]byte
	var loaded []string

	for _, path := range configPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, nil, fmt.Errorf("failed to open config file %s: %w", path, err)
		}
		if len(data) == 0 {
			continue
		}
		if !json.Valid(data) {
			return nil, nil, fmt.Errorf("invalid JSON in config file %s", path)
		}
		configs = append(configs, data)
		loaded = append(loaded, path)
	}

	cfg, err := loadFromBytes(configs)
	if err != nil {
		return nil, nil, err
	}
	return cfg, loaded, nil
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

	// File-based credential discovery requires filesystem stats, so do it
	// last and skip it under test. Checking testing.Testing() before the
	// os.Stat call (rather than after, in the && tail) ensures the syscall
	// is never issued during tests, where it otherwise ran unconditionally
	// and only had its result discarded.
	if testing.Testing() {
		return false
	}
	if _, err := os.Stat(filepath.Join(home.Dir(), ".aws/credentials")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(home.Dir(), ".aws/login")); err == nil {
		return true
	}

	return false
}

// migrateDisableNotifications migrates the deprecated disable_notifications
// field to notification_style. It checks both the user config (~/.config) and
// data config (~/.local) files. If disable_notifications is true, it sets
// notification_style to "disabled" in the data file. Regardless of value, it
// removes disable_notifications from any file that contains it.
func migrateDisableNotifications() {
	globalConfig := GlobalConfig()
	dataConfig := GlobalConfigData()

	var wasDisabled bool
	filesToClean := []string{}

	for _, path := range []string{globalConfig, dataConfig} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if gjson.Get(string(data), "options.disable_notifications").Exists() {
			filesToClean = append(filesToClean, path)
			if gjson.Get(string(data), "options.disable_notifications").Bool() {
				wasDisabled = true
			}
		}
	}

	if len(filesToClean) == 0 {
		return
	}

	// If notifications were disabled, persist the equivalent notification_style.
	if wasDisabled {
		data, err := os.ReadFile(dataConfig)
		if err == nil {
			if !gjson.Get(string(data), "options.notification_style").Exists() {
				updated, err := sjson.Set(string(data), "options.notification_style", "disabled")
				if err == nil {
					if err := atomicWriteFile(dataConfig, []byte(updated), 0o600); err != nil {
						slog.Warn("Failed to migrate disable_notifications to notification_style", "error", err)
					} else {
						slog.Info("Migrated disable_notifications: true to notification_style: disabled")
					}
				}
			}
		}
	}

	// Remove disable_notifications from all files that contain it.
	for _, path := range filesToClean {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		updated, err := sjson.Delete(string(data), "options.disable_notifications")
		if err != nil {
			slog.Warn("Failed to remove deprecated disable_notifications field", "path", path, "error", err)
			continue
		}
		if err := atomicWriteFile(path, []byte(updated), 0o600); err != nil {
			slog.Warn("Failed to write migrated config", "path", path, "error", err)
		}
	}
}

// GlobalConfig returns the global configuration file path for the application.
func GlobalConfig() string {
	if crushGlobal := os.Getenv("CRUSH_GLOBAL_CONFIG"); crushGlobal != "" {
		return filepath.Join(crushGlobal, fmt.Sprintf("%s.json", appName))
	}
	return filepath.Join(home.Config(), appName, fmt.Sprintf("%s.json", appName))
}

// GlobalCacheDir returns the path to the global cache directory for the
// application.
func GlobalCacheDir() string {
	if crushCache := os.Getenv("CRUSH_CACHE_DIR"); crushCache != "" {
		return crushCache
	}
	if xdgCacheHome := os.Getenv("XDG_CACHE_HOME"); xdgCacheHome != "" {
		return filepath.Join(xdgCacheHome, appName)
	}
	if runtime.GOOS == "windows" {
		localAppData := cmp.Or(
			os.Getenv("LOCALAPPDATA"),
			filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local"),
		)
		return filepath.Join(localAppData, appName, "cache")
	}
	return filepath.Join(home.Dir(), ".cache", appName)
}

// ProjectConfigs returns list of current project configs paths.
func ProjectConfigs(cwd string) []string {
	return lookupConfigs(cwd)
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

// GlobalWorkspaceDir returns the path to the global server workspace
// directory. This directory acts as a meta-workspace for the server
// process, giving it a real workingDir so that config loading, scoped
// writes, and provider resolution behave identically to project
// workspaces.
func GlobalWorkspaceDir() string {
	return filepath.Dir(GlobalConfigData())
}

func assignIfNil[T any](ptr **T, val T) {
	if *ptr == nil {
		*ptr = &val
	}
}

func isInsideWorktree() bool {
	bts, err := exec.CommandContext(
		context.Background(),
		"git", "rev-parse",
		"--is-inside-work-tree",
	).CombinedOutput()
	return err == nil && strings.TrimSpace(string(bts)) == "true"
}

// worktreeRoot returns the absolute path of the git working tree root for
// dir, or the empty string if dir is not inside a working tree (bare
// repositories, missing git binary, plain directories, or any other
// failure mode). Linked worktrees and submodules each report their own
// top-level, which is what callers want when bounding lookups.
// worktreeRootCache memoizes the git worktree root per directory. The root
// is stable for the life of the process, so we avoid re-shelling out to
// "git rev-parse" on every config reload. Keyed by the requested dir; the
// value is the resolved root ("" when dir is not in a git worktree).
var worktreeRootCache sync.Map // map[string]string

func worktreeRoot(dir string) string {
	if cached, ok := worktreeRootCache.Load(dir); ok {
		return cached.(string)
	}
	root := computeWorktreeRoot(dir)
	worktreeRootCache.Store(dir, root)
	return root
}

func computeWorktreeRoot(dir string) string {
	cmd := exec.CommandContext(
		context.Background(),
		"git", "rev-parse", "--show-toplevel",
	)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return ""
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	return abs
}

// projectBoundary returns the directory at which an upward configuration
// search rooted at dir should stop. It is the git working tree root when
// one can be detected, otherwise dir itself. Returning dir as a
// fallback keeps Crush from silently adopting state files placed above
// the current project.
func projectBoundary(dir string) string {
	if root := worktreeRoot(dir); root != "" {
		return root
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return abs
}

// GlobalSkillsDirs returns the default directories for Agent Skills.
// Skills in these directories are auto-discovered and their files can be read
// without permission prompts.
func GlobalSkillsDirs() []string {
	if crushSkills := os.Getenv("CRUSH_SKILLS_DIR"); crushSkills != "" {
		return []string{crushSkills}
	}

	paths := []string{
		filepath.Join(home.Config(), appName, "skills"),
		filepath.Join(home.Config(), "agents", "skills"),
		// Per the Agent Skills spec, scan ~/.agents/skills
		filepath.Join(home.Dir(), ".agents", "skills"),
		filepath.Join(home.Dir(), ".claude", "skills"),
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

// projectSkillSubdirs lists the conventional subdirectories where
// project-level skills are discovered. Shared across working-dir and
// git-root lookups to prevent drift when a new convention is added.
var projectSkillSubdirs = []string{
	".agents/skills",
	".crush/skills",
	".claude/skills",
	".cursor/skills",
}

// ProjectSkillsDir returns the default project directories for which Crush
// will look for skills. In addition to the working directory, it also
// checks the git working tree root so that monorepo-level skills are
// discovered when the user is inside a subdirectory.
// Working-directory paths come first so local skills take precedence
// over monorepo-level ones.
func ProjectSkillsDir(workingDir string) []string {
	dirs := make([]string, 0, len(projectSkillSubdirs)*2)
	for _, sub := range projectSkillSubdirs {
		dirs = append(dirs, filepath.Join(workingDir, sub))
	}

	// When the working directory is inside a git repository, also look at
	// the repository root so monorepo-level .agents/skills are found.
	if root := worktreeRoot(workingDir); root != "" && root != workingDir {
		for _, sub := range projectSkillSubdirs {
			dirs = append(dirs, filepath.Join(root, sub))
		}
	}

	return dirs
}

func isAppleTerminal() bool { return os.Getenv("TERM_PROGRAM") == "Apple_Terminal" }

// normalizeHookEvent maps user-provided event names to their canonical
// form. Matching is case-insensitive and accepts snake_case variants
// (e.g. "pre_tool_use" → "PreToolUse").
func normalizeHookEvent(name string) string {
	switch strings.ToLower(strings.ReplaceAll(name, "_", "")) {
	case "pretooluse":
		return "PreToolUse"
	default:
		return name
	}
}

// ValidateHooks normalizes event names and checks that every configured
// hook has a command and a syntactically valid matcher regex. Matcher
// compilation used for matching is owned by hooks.Runner; this function
// only validates up front so the user sees config errors at load time
// rather than on the first tool call.
func (c *Config) ValidateHooks() error {
	// Normalize event name keys.
	for event, eventHooks := range c.Hooks {
		canonical := normalizeHookEvent(event)
		if canonical != event {
			c.Hooks[canonical] = append(c.Hooks[canonical], eventHooks...)
			delete(c.Hooks, event)
		}
	}

	for event, eventHooks := range c.Hooks {
		for i, h := range eventHooks {
			if h.Command == "" {
				return fmt.Errorf("hook %s[%d]: command is required", event, i)
			}
			if h.Matcher == "" {
				continue
			}
			if _, err := regexp.Compile(h.Matcher); err != nil {
				return fmt.Errorf("hook %s[%d]: invalid matcher regex %q: %w", event, i, h.Matcher, err)
			}
		}
	}
	return nil
}
