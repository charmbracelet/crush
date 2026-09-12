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

// classifierModelSelection resolves which model the auto-mode classifier
// should use, in priority order: the explicit auto_mode.classifier
// selection, then the small model slot, then the large model slot
// (matching how the small model slot falls back to the primary model).
// The second return value reports whether any selection exists.
func classifierModelSelection(cfg *config.Config) (config.SelectedModel, bool) {
	if am := cfg.AutoMode; am != nil && am.Classifier != nil &&
		am.Classifier.Provider != "" && am.Classifier.Model != "" {
		return config.SelectedModel{
			Provider: am.Classifier.Provider,
			Model:    am.Classifier.Model,
		}, true
	}
	if small, ok := cfg.Models[config.SelectedModelTypeSmall]; ok && small.Model != "" {
		return small, true
	}
	if large, ok := cfg.Models[config.SelectedModelTypeLarge]; ok && large.Model != "" {
		return large, true
	}
	return config.SelectedModel{}, false
}

// automodeModelResolver builds the classifier's language model on demand,
// re-reading config so reloads apply. Model selection falls back from
// auto_mode.classifier to the small model slot to the large model slot;
// when none exists, auto mode runs in rules-only mode.
func (c *coordinator) automodeModelResolver() automode.LanguageModelResolver {
	return func(ctx context.Context) (fantasy.LanguageModel, error) {
		cfg := c.cfg.Config()
		selected, ok := classifierModelSelection(cfg)
		if !ok {
			return nil, errors.New("no auto_mode classifier or agent model configured")
		}
		providerCfg, ok := cfg.Providers.Get(selected.Provider)
		if !ok {
			return nil, fmt.Errorf("auto_mode classifier provider %q is not configured", selected.Provider)
		}
		provider, err := c.buildProvider(providerCfg, selected, false)
		if err != nil {
			return nil, fmt.Errorf("failed to build auto_mode classifier provider: %w", err)
		}
		model, err := provider.LanguageModel(ctx, selected.Model)
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

// OnAutoModeGrant forwards grant notifications to members that track
// auto-mode quota state.
func (h compositeHooks) OnAutoModeGrant(sessionID string) {
	if obs, ok := h.primary.(permission.AutoModeGrantObserver); ok {
		obs.OnAutoModeGrant(sessionID)
	}
	if obs, ok := h.secondary.(permission.AutoModeGrantObserver); ok {
		obs.OnAutoModeGrant(sessionID)
	}
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

// amOrDefault returns the auto_mode config section, or a zero-value
// section when unset, so the native hooks can always be installed and
// toggled at runtime.
func amOrDefault(cfg *config.ConfigStore) *config.AutoModeConfig {
	if am := cfg.Config().AutoMode; am != nil {
		return am
	}
	return &config.AutoModeConfig{}
}
