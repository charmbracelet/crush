package agent

import (
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/llm/provider"
)

func (a *agent) eventPromptSent(sessionID string) {
	event.PromptSent(
		a.eventCommon(sessionID)...,
	)
}

func (a *agent) eventPromptResponded(sessionID string, duration time.Duration) {
	event.PromptResponded(
		append(
			a.eventCommon(sessionID),
			"prompt duration pretty", duration.String(),
			"prompt duration in seconds", int64(duration.Seconds()),
		)...,
	)
}

func (a *agent) eventTokensUsed(sessionID string, usage provider.TokenUsage, cost float64) {
	event.TokensUsed(
		append(
			a.eventCommon(sessionID),
			"input tokens", usage.InputTokens,
			"output tokens", usage.OutputTokens,
			"cache read tokens", usage.CacheReadTokens,
			"cache creation tokens", usage.CacheCreationTokens,
			"total tokens", usage.InputTokens+usage.OutputTokens+usage.CacheReadTokens+usage.CacheCreationTokens,
			"cost", cost,
		)...,
	)
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
		"yolo mode", a.permissions.SkipRequests(),
	}
}
