package model

import (
	"context"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/usage"
)

// usagePollInterval is how often a declared quota report is refreshed. Quota
// does not move faster than a plan's turns do, so the Hyper credits cadence
// is plenty; a running session shows the same figures until the next poll.
const usagePollInterval = 60 * time.Second

// usageUpdatedMsg carries the meters reported by the provider behind the
// coder agent's current model, unfiltered: which of them apply to the model
// in use is decided at draw time, so switching models narrows the display
// without a refetch. The meters belong to the provider they were fetched
// from, so a stale fetch never describes the model in use. A nil or empty
// list hides it.
type usageUpdatedMsg struct {
	provider string
	meters   []usage.Meter
}

// usagePollMsg is sent by the usage poll timer.
type usagePollMsg struct{}

// usageProviderConfig returns the provider behind the coder agent's current
// model selection when it declares where its plan's quota is reported.
func (m *UI) usageProviderConfig() (config.ProviderConfig, bool) {
	cfg := m.com.Config()
	if cfg == nil || cfg.Providers == nil {
		return config.ProviderConfig{}, false
	}
	agent, ok := cfg.Agents[config.AgentCoder]
	if !ok {
		return config.ProviderConfig{}, false
	}
	selected, ok := cfg.Models[agent.Model]
	if !ok {
		return config.ProviderConfig{}, false
	}
	pc, ok := cfg.Providers.Get(selected.Provider)
	if !ok || pc.Disable || pc.Usage == nil {
		return config.ProviderConfig{}, false
	}
	return pc, true
}

// fetchUsage returns a command that reads the provider's remaining quota,
// refreshing an expired login first so a long-running session keeps a live
// figure. It is a no-op for models whose provider declares no report. The
// current selection is read when the command runs, not when it is built, so
// a fetch scheduled right after a model switch — or the sign-in that
// precedes one — reports the new provider, whose config write has landed by
// then.
func (m *UI) fetchUsage() tea.Cmd {
	return func() tea.Msg {
		pc, ok := m.usageProviderConfig()
		if !ok {
			return nil
		}
		bearer := ""
		if pc.OAuthToken != nil {
			if pc.OAuthToken.IsExpired() {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				if err := m.com.Workspace.RefreshOAuthToken(ctx, config.ScopeGlobal, pc.ID); err != nil {
					slog.Warn("Usage refresh: could not refresh the provider login", "provider", pc.ID, "error", err)
				} else if fresh, still := m.usageProviderConfig(); still {
					pc = fresh
				}
				cancel()
			}
			if pc.OAuthToken != nil {
				bearer = pc.OAuthToken.AccessToken
			}
		}
		if bearer == "" {
			resolved, err := m.com.Workspace.Resolver().ResolveValue(pc.APIKey)
			if err != nil {
				slog.Warn("Usage refresh: could not resolve the provider credential", "provider", pc.ID)
				return nil
			}
			bearer = resolved
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		meters, err := usage.Fetch(ctx, pc.Usage, bearer, pc.ExtraHeaders)
		if err != nil {
			// A stale figure beats no figure, and a flaky quota endpoint
			// must never nag the chat.
			slog.Debug("Usage refresh failed", "provider", pc.ID, "error", err)
			return nil
		}
		return usageUpdatedMsg{provider: pc.ID, meters: meters}
	}
}

// usageTicker schedules the next quota poll.
func (m *UI) usageTicker() tea.Cmd {
	return tea.Tick(usagePollInterval, func(time.Time) tea.Msg {
		return usagePollMsg{}
	})
}

// usageForCurrentModel returns the fetched meters that the coder agent's
// current model draws from, so the display shows the limits that apply
// rather than every limit the plan reports. Meters belonging to another
// provider — the model switched away from one before its fetch was replaced
// — are nothing to show, and the next poll or selection fetches the right
// ones.
func (m *UI) usageForCurrentModel() []usage.Meter {
	cfg := m.com.Config()
	if cfg == nil || len(m.usageMeters) == 0 {
		return nil
	}
	agent, ok := cfg.Agents[config.AgentCoder]
	if !ok {
		return nil
	}
	selected, ok := cfg.Models[agent.Model]
	if !ok {
		return nil
	}
	pc, hasSpec := cfg.Providers.Get(selected.Provider)
	if !hasSpec || pc.Usage == nil {
		return nil
	}
	if m.usageProvider != selected.Provider {
		return nil
	}
	return usage.ForModel(m.usageMeters, pc.Usage, selected.Model)
}
