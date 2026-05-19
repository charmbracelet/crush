package app

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"sync"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/azure"
	"charm.land/fantasy/providers/bedrock"
	"charm.land/fantasy/providers/google"
	"charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openaicompat"
	"charm.land/fantasy/providers/openrouter"
	"charm.land/fantasy/providers/vercel"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/skills/critic"
	"github.com/charmbracelet/crush/internal/skills/replacer"
	openaisdk "github.com/openai/openai-go/v3/option"
)

// buildCriticWrapper creates an AgentWrapper that injects critic middleware
// around the primary agent when critic mode is enabled.
func (app *App) buildCriticWrapper(cfg critic.CriticSkillConfig) func(agent.SessionAgent) agent.SessionAgent {
	var err error
	workDir := ""
	if app.config != nil {
		workDir = app.config.WorkingDir()
	}
	cfg, err = critic.LoadSkillConfig(cfg, workDir)
	if err != nil {
		slog.Warn("Failed to load critic skill config", "error", err)
	}
	if !cfg.Enabled {
		return nil
	}

	criticSvc := critic.NewCriticService(cfg, nil)
	emitter := app.buildCriticEmitter(cfg)
	criticSvc.SetCheckpointEmitter(emitter)

	slog.Info("Critic enabled", "max_iterations", cfg.MaxIterations, "threshold", cfg.Threshold)

	return func(primary agent.SessionAgent) agent.SessionAgent {
		mw := critic.NewMiddleware(primary, cfg)
		mw.SetFileTracker(app.FileTracker)
		mw.SetLSPManager(app.LSPManager)
		mw.SetCriticService(criticSvc)
		mw.SetMessageService(app.Messages)
		mw.SetStore(app.CriticStore)
		return mw
	}
}

// buildCriticEmitter returns a CheckpointEmitter that calls a small language
// model to review checkpoints. The model is resolved lazily on the first call.
func (app *App) buildCriticEmitter(cfg critic.CriticSkillConfig) critic.CheckpointEmitter {
	var once sync.Once
	var model fantasy.LanguageModel
	var resolveErr error

	return func(ctx context.Context, cp critic.Checkpoint) (*critic.CriticFeedback, error) {
		once.Do(func() {
			model, resolveErr = app.resolveCriticModel(cfg)
		})
		if resolveErr != nil {
			event.Error(resolveErr, "critic_model_resolution_failed", true)
			return nil, fmt.Errorf("critic model resolution failed: %w", resolveErr)
		}

		workDir := ""
		if app.config != nil {
			workDir = app.config.WorkingDir()
		}
		promptText, err := critic.BuildCriticPrompt(cp, workDir)
		if err != nil {
			return nil, fmt.Errorf("build critic prompt: %w", err)
		}

		maxTokens := int64(2048)
		temp := 0.1
		resp, err := model.Generate(ctx, fantasy.Call{
			Prompt:          fantasy.Prompt{fantasy.NewUserMessage(promptText)},
			MaxOutputTokens: &maxTokens,
			Temperature:     &temp,
		})
		if err != nil {
			event.Error(err, "critic_generation_failed", true)
			return nil, fmt.Errorf("critic generation failed: %w", err)
		}

		feedback, err := critic.ParseFeedback(resp.Content.Text())
		if err != nil {
			return nil, fmt.Errorf("parse critic feedback: %w", err)
		}
		return feedback, nil
	}
}

// resolveCriticModel resolves the language model to use for the critic.
// Priority: cfg.Model > small model from config.
func (app *App) resolveCriticModel(cfg critic.CriticSkillConfig) (fantasy.LanguageModel, error) {
	c := app.config.Config()

	var providerCfg config.ProviderConfig
	var modelCfg config.SelectedModel

	if cfg.Model != "" {
		// Try to resolve the explicitly configured critic model.
		// Format is expected to be "provider/model".
		for _, p := range c.EnabledProviders() {
			for _, m := range p.Models {
				if m.ID == cfg.Model {
					providerCfg = p
					modelCfg = config.SelectedModel{Provider: p.ID, Model: m.ID}
					break
				}
			}
		}
	}

	if modelCfg.Provider == "" {
		// Fallback to the agent's small model.
		small, ok := c.Models[config.SelectedModelTypeSmall]
		if !ok {
			return nil, fmt.Errorf("no small model configured and no critic model specified")
		}
		modelCfg = small
		p, ok := c.Providers.Get(small.Provider)
		if !ok {
			return nil, fmt.Errorf("provider %s for small model not found", small.Provider)
		}
		providerCfg = p
	}

	provider, err := app.buildCriticProvider(providerCfg)
	if err != nil {
		return nil, fmt.Errorf("build critic provider: %w", err)
	}

	model, err := provider.LanguageModel(context.Background(), modelCfg.Model)
	if err != nil {
		return nil, fmt.Errorf("get critic language model: %w", err)
	}

	slog.Debug("Resolved critic model", "provider", providerCfg.ID, "model", modelCfg.Model)
	return model, nil
}

// buildCriticProvider constructs a fantasy.Provider from config for the critic.
// This is a simplified version of coordinator.buildProvider that covers the
// common API-key-based providers.
func (app *App) buildCriticProvider(providerCfg config.ProviderConfig) (fantasy.Provider, error) {
	apiKey, _ := app.config.Resolve(providerCfg.APIKey)
	baseURL, _ := app.config.Resolve(providerCfg.BaseURL)

	headers := maps.Clone(providerCfg.ExtraHeaders)
	if headers == nil {
		headers = make(map[string]string)
	}

	switch providerCfg.Type {
	case openai.Name:
		opts := []openai.Option{
			openai.WithAPIKey(apiKey),
			openai.WithUseResponsesAPI(),
		}
		if baseURL != "" {
			opts = append(opts, openai.WithBaseURL(baseURL))
		}
		if len(headers) > 0 {
			opts = append(opts, openai.WithHeaders(headers))
		}
		return openai.New(opts...)

	case anthropic.Name:
		opts := []anthropic.Option{anthropic.WithAPIKey(apiKey)}
		if baseURL != "" {
			opts = append(opts, anthropic.WithBaseURL(baseURL))
		}
		if len(headers) > 0 {
			opts = append(opts, anthropic.WithHeaders(headers))
		}
		return anthropic.New(opts...)

	case openrouter.Name:
		opts := []openrouter.Option{openrouter.WithAPIKey(apiKey)}
		if len(headers) > 0 {
			opts = append(opts, openrouter.WithHeaders(headers))
		}
		return openrouter.New(opts...)

	case google.Name:
		opts := []google.Option{google.WithGeminiAPIKey(apiKey)}
		if baseURL != "" {
			opts = append(opts, google.WithBaseURL(baseURL))
		}
		if len(headers) > 0 {
			opts = append(opts, google.WithHeaders(headers))
		}
		return google.New(opts...)

	case "google-vertex":
		opts := []google.Option{}
		if len(headers) > 0 {
			opts = append(opts, google.WithHeaders(headers))
		}
		project := providerCfg.ExtraParams["project"]
		location := providerCfg.ExtraParams["location"]
		opts = append(opts, google.WithVertex(project, location))
		return google.New(opts...)

	case azure.Name:
		opts := []azure.Option{
			azure.WithBaseURL(baseURL),
			azure.WithAPIKey(apiKey),
			azure.WithUseResponsesAPI(),
		}
		apiVersion := providerCfg.ExtraParams["apiVersion"]
		if apiVersion != "" {
			opts = append(opts, azure.WithAPIVersion(apiVersion))
		}
		if len(headers) > 0 {
			opts = append(opts, azure.WithHeaders(headers))
		}
		return azure.New(opts...)

	case bedrock.Name:
		var opts []bedrock.Option
		if len(headers) > 0 {
			opts = append(opts, bedrock.WithHeaders(headers))
		}
		if apiKey != "" {
			opts = append(opts, bedrock.WithAPIKey(apiKey))
		}
		return bedrock.New(opts...)

	case vercel.Name:
		opts := []vercel.Option{vercel.WithAPIKey(apiKey)}
		if len(headers) > 0 {
			opts = append(opts, vercel.WithHeaders(headers))
		}
		return vercel.New(opts...)

	case openaicompat.Name:
		opts := []openaicompat.Option{
			openaicompat.WithBaseURL(baseURL),
			openaicompat.WithAPIKey(apiKey),
		}
		if len(headers) > 0 {
			opts = append(opts, openaicompat.WithHeaders(headers))
		}
		for extraKey, extraValue := range providerCfg.ExtraBody {
			opts = append(opts, openaicompat.WithSDKOptions(openaisdk.WithJSONSet(extraKey, extraValue)))
		}
		return openaicompat.New(opts...)

	default:
		return nil, fmt.Errorf("unsupported critic provider type: %s", providerCfg.Type)
	}
}

// buildReplacerWrapper creates an AgentWrapper that injects replacer middleware
// around the primary agent when the replacement agent is enabled.
func (app *App) buildReplacerWrapper(cfg replacer.ReplacerConfig) func(agent.SessionAgent) agent.SessionAgent {
	// Force-enable via env var for debugging even when config is missing.
	if os.Getenv("CRUSH_REPLACER_FORCE_ENABLE") == "1" {
		cfg.Enabled = true
		if cfg.MaxIterations <= 0 {
			cfg.MaxIterations = replacer.DefaultMaxIterations
		}
		if cfg.Timeout <= 0 {
			cfg.Timeout = replacer.DefaultTimeout
		}
	}
	if !cfg.Enabled {
		slog.Info("Replacer disabled in config", "max_iterations", cfg.MaxIterations)
		return nil
	}

	slog.Info("Replacer enabled", "max_iterations", cfg.MaxIterations)

	return func(primary agent.SessionAgent) agent.SessionAgent {
		mw := replacer.NewMiddleware(primary, cfg)
		mw.SetMessageService(app.Messages)
		mw.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
			return app.resolveReplacerModel(cfg)
		})
		return mw
	}
}

// resolveReplacerModel resolves the language model for the replacement agent.
func (app *App) resolveReplacerModel(cfg replacer.ReplacerConfig) (fantasy.LanguageModel, error) {
	c := app.config.Config()

	var providerCfg config.ProviderConfig
	var modelCfg config.SelectedModel

	if cfg.Model != "" {
		for _, p := range c.EnabledProviders() {
			for _, m := range p.Models {
				if m.ID == cfg.Model {
					providerCfg = p
					modelCfg = config.SelectedModel{Provider: p.ID, Model: m.ID}
					break
				}
			}
		}
	}

	if modelCfg.Provider == "" {
		small, ok := c.Models[config.SelectedModelTypeSmall]
		if !ok {
			return nil, fmt.Errorf("no small model configured and no replacer model specified")
		}
		modelCfg = small
		p, ok := c.Providers.Get(small.Provider)
		if !ok {
			return nil, fmt.Errorf("provider %s for small model not found", small.Provider)
		}
		providerCfg = p
	}

	provider, err := app.buildCriticProvider(providerCfg)
	if err != nil {
		return nil, fmt.Errorf("build replacer provider: %w", err)
	}

	model, err := provider.LanguageModel(context.Background(), modelCfg.Model)
	if err != nil {
		return nil, fmt.Errorf("get replacer language model: %w", err)
	}

	slog.Debug("Resolved replacer model", "provider", providerCfg.ID, "model", modelCfg.Model)
	return model, nil
}
