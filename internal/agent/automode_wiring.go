package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/automode"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
)

// automodeModelResolver builds the classifier's language model on demand,
// re-reading config so reloads apply. It references the provider and
// model declared in auto_mode.classifier, which must exist in the
// providers config. The classifier never silently uses the main agent's
// model: without an explicit classifier selection, auto mode runs in
// rules-only mode.
func (c *coordinator) automodeModelResolver() automode.LanguageModelResolver {
	return func(ctx context.Context) (fantasy.LanguageModel, error) {
		cfg := c.cfg.Config()
		am := cfg.AutoMode
		if am == nil || am.Classifier == nil {
			return nil, errors.New("no auto_mode classifier configured")
		}
		if am.Classifier.Provider == "" || am.Classifier.Model == "" {
			return nil, errors.New("auto_mode classifier requires both provider and model")
		}
		providerCfg, ok := cfg.Providers.Get(am.Classifier.Provider)
		if !ok {
			return nil, fmt.Errorf("auto_mode classifier provider %q is not configured", am.Classifier.Provider)
		}
		selected := config.SelectedModel{
			Provider: am.Classifier.Provider,
			Model:    am.Classifier.Model,
		}
		provider, err := c.buildProvider(providerCfg, selected, false)
		if err != nil {
			return nil, fmt.Errorf("failed to build auto_mode classifier provider: %w", err)
		}
		model, err := provider.LanguageModel(ctx, am.Classifier.Model)
		if err != nil {
			return nil, fmt.Errorf("failed to build auto_mode classifier model: %w", err)
		}
		return model, nil
	}
}

// automodeOptions converts the config's auto_mode section into
// automode.Options.
func (c *coordinator) automodeOptions(am *config.AutoModeConfig) automode.Options {
	return automode.Options{
		ModelResolver:         c.automodeModelResolver(),
		Transcript:            c.automodeTranscript(),
		MaxTokens:             int64(am.MaxTokens),
		Timeout:               time.Duration(am.TimeoutSeconds) * time.Second,
		FailOpen:              am.FailOpen,
		Environment:           am.Environment,
		PromptStage1File:      am.PromptStage1File,
		PromptStage2File:      am.PromptStage2File,
		TranscriptMaxChars:    am.TranscriptMaxChars,
		MaxConsecutiveDenials: am.MaxConsecutiveDenials,
		MaxTotalDenials:       am.MaxTotalDenials,
	}
}

// automodeTranscript adapts the agent layer's shared reasoning-blind
// transcript builder into automode's TranscriptProvider.
func (c *coordinator) automodeTranscript() automode.TranscriptProvider {
	return func(ctx context.Context, sessionID string) string {
		return transcriptProvider(c.messages)(ctx, sessionID)
	}
}

// compositeHooks runs the primary hooks first and, when they express no
// opinion, falls back to the secondary hooks. The native auto mode is
// primary; external PrePermission hooks stay available as fallback so
// both can coexist.
type compositeHooks struct {
	primary   permission.PermissionHooks
	secondary permission.PermissionHooks
}

func (h compositeHooks) PrePermission(ctx context.Context, req permission.PermissionRequest) permission.PreHookResult {
	if res := h.primary.PrePermission(ctx, req); res.Decision != permission.HookDecisionNone {
		return res
	}
	return h.secondary.PrePermission(ctx, req)
}

func (h compositeHooks) PermissionDenied(ctx context.Context, req permission.PermissionRequest) {
	h.primary.PermissionDenied(ctx, req)
	h.secondary.PermissionDenied(ctx, req)
}

// AutoModeEnabled reports whether the native auto mode is enabled in the
// current config. Note: `auto_mode.enabled` is intended to be honored
// from user-level config only — a repository should not be able to grant
// itself auto mode. The merged config is honored today for
// demonstration; production should restrict the source.
func (c *coordinator) AutoModeEnabled() bool {
	am := c.cfg.Config().AutoMode
	return am != nil && am.Enabled
}
