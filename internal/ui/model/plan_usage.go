package model

import (
	"context"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/openai"
)

// A ChatGPT plan reports its usage on every response, so the readout stays
// current on its own while a session runs. These commands cover the rest: the
// first frame of a session, a fresh sign-in, and idle time.

type (
	// planUsageUpdatedMsg reports that a fetched snapshot was recorded, so
	// the header redraws with it.
	planUsageUpdatedMsg struct{}

	// planUsagePollMsg is sent by the plan usage poll timer.
	planUsagePollMsg struct{}
)

// planUsagePollInterval shortens as the plan fills up. A plan with room to
// spare can go a minute between reads; one about to run out is worth watching,
// and that is also when a window refilling is the news being waited on.
func planUsagePollInterval(spent float64) time.Duration {
	switch {
	case spent >= 0.99:
		return 5 * time.Second
	case spent >= 0.90:
		return 15 * time.Second
	case spent >= 0.75:
		return 30 * time.Second
	default:
		return 60 * time.Second
	}
}

// chatGPTPlanToken returns the OAuth token of a ChatGPT plan sign-in, or nil
// when the OpenAI provider is absent or authenticated with an API key. Plan
// usage only exists for the former.
func (m *UI) chatGPTPlanToken() *oauth.Token {
	// Only the selected provider's plan is shown, so fetching another
	// provider's is a request nobody reads.
	if m.com.SelectedProviderID() != string(catwalk.InferenceProviderOpenAI) {
		return nil
	}
	cfg := m.com.Config()
	if cfg == nil || cfg.Providers == nil {
		return nil
	}
	providerCfg, ok := cfg.Providers.Get(string(catwalk.InferenceProviderOpenAI))
	if !ok {
		return nil
	}
	return providerCfg.OAuthToken
}

// fetchPlanUsage reads plan utilization from the Codex backend. It is a no-op
// unless a ChatGPT plan is signed in.
func (m *UI) fetchPlanUsage() tea.Cmd {
	return func() tea.Msg {
		token := m.chatGPTPlanToken()
		if token == nil {
			return nil
		}

		if token.IsExpired() {
			ctxRefresh, cancelRefresh := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancelRefresh()
			if err := m.com.Workspace.RefreshOAuthToken(ctxRefresh, config.ScopeGlobal, string(catwalk.InferenceProviderOpenAI)); err != nil {
				slog.Warn("ChatGPT OAuth refresh failed before fetching plan usage, trying with existing token", "error", err)
			} else if refreshed := m.chatGPTPlanToken(); refreshed != nil {
				token = refreshed
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		usage, err := openai.FetchUsage(ctx, token)
		if err != nil {
			slog.Debug("Failed to fetch ChatGPT plan usage", "error", err)
			return nil
		}
		openai.RecordUsage(usage)
		return planUsageUpdatedMsg{}
	}
}

// planUsageTicker schedules the next poll, sooner the closer the plan is to
// its limit.
func (m *UI) planUsageTicker() tea.Cmd {
	var spent float64
	if usage, ok := openai.LatestUsage(); ok {
		spent = usage.Spent()
	}
	return tea.Tick(planUsagePollInterval(spent), func(time.Time) tea.Msg {
		return planUsagePollMsg{}
	})
}
