package agent

import (
	"time"

	"charm.land/fantasy"
)

// These hooks intentionally remain as local no-ops so agent execution does not
// depend on an external analytics service.
func (a *sessionAgent) eventPromptSent(string) {}

func (a *sessionAgent) eventPromptResponded(string, time.Duration) {}

func (a *sessionAgent) eventTokensUsed(string, Model, fantasy.Usage, float64) {}
