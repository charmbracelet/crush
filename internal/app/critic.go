package app

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"strings"
	"sync"

	"charm.land/catwalk/pkg/catwalk"
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
	"github.com/charmbracelet/crush/internal/skills/toolcoach"
	openaisdk "github.com/charmbracelet/openai-go/option"
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
		mw.SetCoachSummaryProvider(app)
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
			model, resolveErr = app.resolveCriticModel(ctx, cfg)
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
func (app *App) resolveCriticModel(ctx context.Context, cfg critic.CriticSkillConfig) (fantasy.LanguageModel, error) {
	return app.resolveSkillModel(ctx, cfg.Model, "critic")
}

// buildSkillProvider constructs a fantasy.Provider from config for a skill
// (critic or replacer). This is a simplified version of coordinator.buildProvider
// that covers the common API-key-based providers.
func (app *App) buildSkillProvider(providerCfg config.ProviderConfig) (fantasy.Provider, error) {
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
		var once sync.Once
		var model fantasy.LanguageModel
		var resolveErr error
		mw.SetModelResolver(func(ctx context.Context) (fantasy.LanguageModel, error) {
			once.Do(func() {
				model, resolveErr = app.resolveReplacerModel(ctx, cfg)
			})
			return model, resolveErr
		})
		return mw
	}
}

// buildToolcoachWrapper creates an AgentWrapper that injects toolcoach middleware
// around the primary agent when the tool pattern coach is enabled.
func (app *App) buildToolcoachWrapper(cfg toolcoach.ToolcoachConfig) func(agent.SessionAgent) agent.SessionAgent {
	if !cfg.Enabled {
		slog.Info("Toolcoach disabled in config")
		return nil
	}

	slog.Info("Toolcoach enabled", "max_patterns_per_turn", cfg.MaxPatternsPerTurn)

	return func(primary agent.SessionAgent) agent.SessionAgent {
		mw := toolcoach.NewMiddleware(primary, cfg)
		mw.SetMessageService(app.Messages)
		mw.SetStore(app.ToolcoachStore)
		app.toolcoachMw.Store(mw)
		return mw
	}
}

// resolveReplacerModel resolves the language model for the replacement agent.
// It prefers small, fast models: explicit replacer config > user's small model >
// auto-selected smallest available model.
func (app *App) resolveReplacerModel(ctx context.Context, cfg replacer.ReplacerConfig) (fantasy.LanguageModel, error) {
	return app.resolveSkillModel(ctx, cfg.Model, "replacer")
}

// resolveSkillModel resolves a language model for a skill (critic or replacer).
// Priority: explicit model ID > small model from config.
func (app *App) resolveSkillModel(ctx context.Context, modelID, skillName string) (fantasy.LanguageModel, error) {
	c := app.config.Config()

	var providerCfg config.ProviderConfig
	var modelCfg config.SelectedModel

	// 1. Explicit skill model override.
	if modelID != "" {
		for _, p := range c.EnabledProviders() {
			for _, m := range p.Models {
				if m.ID == modelID {
					providerCfg = p
					modelCfg = config.SelectedModel{Provider: p.ID, Model: m.ID}
					break
				}
			}
		}
	}

	// 2. User's configured small model — but validate it is actually small.
	if modelCfg.Provider == "" {
		if small, ok := c.Models[config.SelectedModelTypeSmall]; ok {
			if p, ok := c.Providers.Get(small.Provider); ok {
				if m := c.GetModel(small.Provider, small.Model); m != nil {
					score := modelSmallnessScore(*m)
					if score >= 50 {
						modelCfg = small
						providerCfg = p
					} else {
						slog.Warn("Configured small model scores poorly for coach, auto-selecting better one",
							"configured_model", small.Model,
							"configured_provider", small.Provider,
							"score", score,
						)
					}
				}
			}
		}
	}

	// 3. Auto-select the smallest/fastest available model.
	if modelCfg.Provider == "" {
		providerCfg, modelCfg = app.pickSmallestModel(c)
	}

	if modelCfg.Provider == "" {
		return nil, fmt.Errorf("no small model configured and no replacer model specified")
	}

	provider, err := app.buildSkillProvider(providerCfg)
	if err != nil {
		return nil, fmt.Errorf("build %s provider: %w", skillName, err)
	}

	model, err := provider.LanguageModel(ctx, modelCfg.Model)
	if err != nil {
		return nil, fmt.Errorf("get %s language model: %w", skillName, err)
	}

	slog.Debug("Resolved skill model", "skill", skillName, "provider", providerCfg.ID, "model", modelCfg.Model)
	return model, nil
}

// pickSmallestModel selects the smallest/fastest model from all enabled
// providers using a heuristic based on model ID patterns and metadata.
func (app *App) pickSmallestModel(c *config.Config) (config.ProviderConfig, config.SelectedModel) {
	type candidate struct {
		provider config.ProviderConfig
		model    catwalk.Model
		score    int
	}
	var candidates []candidate

	for _, p := range c.EnabledProviders() {
		for _, m := range p.Models {
			candidates = append(candidates, candidate{
				provider: p,
				model:    m,
				score:    modelSmallnessScore(m),
			})
		}
	}

	if len(candidates) == 0 {
		return config.ProviderConfig{}, config.SelectedModel{}
	}

	// Highest score = smallest/fastest.
	best := candidates[0]
	for _, cand := range candidates[1:] {
		if cand.score > best.score {
			best = cand
		}
	}

	return best.provider, config.SelectedModel{
		Provider:  best.provider.ID,
		Model:     best.model.ID,
		MaxTokens: best.model.DefaultMaxTokens,
	}
}

// modelSmallnessScore returns a heuristic score where higher values indicate
// smaller, faster, cheaper models that are better suited for the coach.
func modelSmallnessScore(m catwalk.Model) int {
	score := 0
	id := strings.ToLower(m.ID)

	// Strong small-model signals.
	smallKeywords := []string{"mini", "flash", "haiku", "nano", "small", "lite", "tiny"}
	for _, kw := range smallKeywords {
		if strings.Contains(id, kw) {
			score += 100
		}
	}

	// Large-model penalties.
	largeKeywords := []string{"max", "pro", "ultra", "large", "heavy", "opus", "reasoning", "thinking"}
	for _, kw := range largeKeywords {
		if strings.Contains(id, kw) {
			score -= 50
		}
	}

	// Reasoning models are slower.
	if m.CanReason {
		score -= 30
	}

	// Prefer models with smaller default max tokens (proxy for size/speed).
	if m.DefaultMaxTokens > 0 {
		if m.DefaultMaxTokens <= 4096 {
			score += 20
		} else if m.DefaultMaxTokens <= 8192 {
			score += 10
		} else if m.DefaultMaxTokens >= 32768 {
			score -= 10
		}
	}

	return score
}
