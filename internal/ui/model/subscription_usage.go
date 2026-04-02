package model

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/subscription"
)

const (
	subscriptionUsageRefreshInterval = 15 * time.Minute
	subscriptionUsageTTL             = 15 * time.Minute
)

func (m *UI) subscriptionUsageTick() tea.Cmd {
	return tea.Tick(subscriptionUsageRefreshInterval, func(time.Time) tea.Msg {
		return subscriptionUsageTickMsg{}
	})
}

func (m *UI) clearSubscriptionUsage() {
	m.subscriptionUsage = nil
	m.subscriptionUsageProviderID = ""
	m.subscriptionUsageLoading = false
	m.subscriptionUsageError = ""
	m.subscriptionUsageFetchedAt = time.Time{}
}

func (m *UI) refreshSubscriptionUsage(force bool) tea.Cmd {
	if m.state != uiChat || !m.hasSession() {
		return nil
	}

	model := m.selectedLargeModel()
	if model == nil {
		m.clearSubscriptionUsage()
		return nil
	}

	providerID := model.ModelCfg.Provider
	if !subscription.HasFetcher(providerID) {
		m.clearSubscriptionUsage()
		return nil
	}

	providerCfg, ok := m.com.Config().Providers.Get(providerID)
	if !ok {
		m.clearSubscriptionUsage()
		return nil
	}

	if m.subscriptionUsageLoading && m.subscriptionUsageProviderID == providerID {
		return nil
	}

	if !force && m.subscriptionUsageProviderID == providerID && !m.subscriptionUsageFetchedAt.IsZero() &&
		time.Since(m.subscriptionUsageFetchedAt) < subscriptionUsageTTL {
		return nil
	}

	m.subscriptionUsageLoading = true
	return func() tea.Msg {
		report, err := subscription.FetchUsage(context.Background(), providerID, providerCfg)
		return subscriptionUsageLoadedMsg{
			providerID: providerID,
			report:     report,
			err:        err,
			fetchedAt:  time.Now(),
		}
	}
}

func (m *UI) subscriptionUsageLines(providerID string) []string {
	if m.subscriptionUsageProviderID != providerID || m.subscriptionUsage == nil {
		if m.subscriptionUsageLoading {
			return []string{"Limits loading..."}
		}
		if m.subscriptionUsageError != "" {
			return []string{"Limits unavailable"}
		}
		return []string{"Limits unavailable"}
	}
	return subscription.SidebarLines(m.subscriptionUsage, time.Now())
}
