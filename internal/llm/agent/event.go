package agent

import (
	"log/slog"
	"time"

	"github.com/bwl/cliffy/internal/config"
	"github.com/bwl/cliffy/internal/llm/provider"
)

func (a *agent) eventPromptSent(sessionID string) {
	slog.Info("Prompt sent", a.eventCommon(sessionID)...)
}

func (a *agent) eventPromptResponded(sessionID string, duration time.Duration) {
	args := append(
		a.eventCommon(sessionID),
		"prompt duration pretty", duration.String(),
		"prompt duration in seconds", int64(duration.Seconds()),
	)
	slog.Info("Prompt responded", args...)
}

func (a *agent) eventTokensUsed(sessionID string, usage provider.TokenUsage, cost float64) {
	args := append(
		a.eventCommon(sessionID),
		"input tokens", usage.InputTokens,
		"output tokens", usage.OutputTokens,
		"cache read tokens", usage.CacheReadTokens,
		"cache creation tokens", usage.CacheCreationTokens,
		"total tokens", usage.InputTokens+usage.OutputTokens+usage.CacheReadTokens+usage.CacheCreationTokens,
		"cost", cost,
	)
	slog.Info("Tokens used", args...)
}

func (a *agent) eventCommon(sessionID string) []any {
	cfg := config.Get()
	currentModel := cfg.Models[cfg.Agents["coder"].Model]

	return []any{
		"session id", sessionID,
		"provider", currentModel.Provider,
		"model", currentModel.Model,
		"reasoning effort", currentModel.ReasoningEffort,
		"thinking mode", currentModel.Think,
	}
}
