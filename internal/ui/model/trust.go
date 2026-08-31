package model

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// openTrustDialog opens the trust confirmation dialog for untrusted
// project configs.
func (m *UI) openTrustDialog() tea.Cmd {
	if m.dialog.ContainsDialog(dialog.TrustID) {
		m.dialog.BringToFront(dialog.TrustID)
		return nil
	}

	store := m.com.ConfigStore()
	if store == nil || !store.HasUntrustedProjectConfigs() {
		return nil
	}

	trustDialog := dialog.NewTrust(
		m.com,
		store.UntrustedProjectPaths(),
		m.com.Workspace.WorkingDir(),
	)
	m.dialog.OpenDialog(trustDialog)
	return nil
}

// acceptProjectTrust marks all untrusted project configs as trusted.
func (m *UI) acceptProjectTrust() tea.Cmd {
	store := m.com.ConfigStore()
	if store == nil {
		return nil
	}
	if err := store.AcceptProjectTrust(); err != nil {
		return util.ReportError(fmt.Errorf("failed to accept project trust: %w", err))
	}
	m.dialog.CloseDialog(dialog.TrustID)
	return func() tea.Msg {
		return util.InfoMsg{
			Type: util.InfoTypeInfo,
			Msg:  "Project configs trusted and loaded.",
			TTL:  5 * time.Second,
		}
	}
}

// rejectProjectTrust rejects the untrusted project configs.
func (m *UI) rejectProjectTrust() tea.Cmd {
	store := m.com.ConfigStore()
	if store == nil {
		return nil
	}
	store.RejectProjectTrust()
	m.dialog.CloseDialog(dialog.TrustID)
	return func() tea.Msg {
		return util.InfoMsg{
			Type: util.InfoTypeInfo,
			Msg:  "Project configs rejected. Using global config only.",
			TTL:  5 * time.Second,
		}
	}
}
