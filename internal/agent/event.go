package agent

import (
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/permission"
)

func (a *sessionAgent) eventPromptSent(sessionID string) {
	event.PromptSent(
		a.eventCommon(sessionID, a.largeModel.Get())...,
	)
}

func (a *sessionAgent) eventPromptResponded(sessionID string, duration time.Duration) {
	event.PromptResponded(
		append(
			a.eventCommon(sessionID, a.largeModel.Get()),
			"prompt duration pretty", duration.String(),
			"prompt duration in seconds", int64(duration.Seconds()),
		)...,
	)
}

func (a *sessionAgent) eventTokensUsed(sessionID string, model Model, usage fantasy.Usage, cost float64) {
	event.TokensUsed(
		append(
			a.eventCommon(sessionID, model),
			"input tokens", usage.InputTokens,
			"output tokens", usage.OutputTokens,
			"cache read tokens", usage.CacheReadTokens,
			"cache creation tokens", usage.CacheCreationTokens,
			"total tokens", usage.InputTokens+usage.OutputTokens+usage.CacheReadTokens+usage.CacheCreationTokens,
			"cost", cost,
		)...,
	)
}

func (a *sessionAgent) eventCommon(sessionID string, model Model) []any {
	m := model.ModelCfg

	mode := "regular"
	if a.permissions != nil {
		switch a.permissions.GetMode() {
		case permission.ModeYolo:
			mode = "yolo"
		case permission.ModePlan:
			mode = "plan"
		}
	}

	return []any{
		"session id", sessionID,
		"provider", m.Provider,
		"model", m.Model,
		"reasoning effort", m.ReasoningEffort,
		"thinking mode", m.Think,
		"permission mode", mode,
	}
}
