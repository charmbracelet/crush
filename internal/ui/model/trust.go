package model

import (
	"context"
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

// acceptProjectTrust marks all untrusted project configs as trusted,
// reloads the config in the background so they take effect, and starts
// any MCP servers they define.
func (m *UI) acceptProjectTrust() tea.Cmd {
	store := m.com.ConfigStore()
	if store == nil {
		return nil
	}
	m.dialog.CloseDialog(dialog.TrustID)
	return func() tea.Msg {
		if err := store.AcceptProjectTrust(context.Background()); err != nil {
			return util.NewErrorMsg(fmt.Errorf("failed to accept project trust: %w", err))
		}
		m.com.Workspace.ApplyTrustedConfig(context.Background())
		return util.InfoMsg{
			Type: util.InfoTypeInfo,
			Msg:  "Project configs trusted and loaded.",
			TTL:  5 * time.Second,
		}
	}
}

// rejectProjectTrust rejects the untrusted project configs and records
// "no" decisions for their content hashes so they are not re-prompted
// until they change.
func (m *UI) rejectProjectTrust() tea.Cmd {
	store := m.com.ConfigStore()
	if store == nil {
		return nil
	}
	m.dialog.CloseDialog(dialog.TrustID)
	return func() tea.Msg {
		if err := store.RejectProjectTrust(); err != nil {
			return util.NewErrorMsg(fmt.Errorf("failed to reject project trust: %w", err))
		}
		return util.InfoMsg{
			Type: util.InfoTypeInfo,
			Msg:  "Project configs rejected. Using global config only.",
			TTL:  5 * time.Second,
		}
	}
}

// retrustProjectConfigs clears rejection decisions for rejected project
// configs and re-asks the trust question.
func (m *UI) retrustProjectConfigs() tea.Cmd {
	store := m.com.ConfigStore()
	if store == nil {
		return nil
	}
	paths := store.RejectedProjectPaths()
	if len(paths) == 0 {
		return nil
	}
	return m.promptProjectTrust(paths)
}

// untrustProjectConfigs clears trust decisions for trusted project
// configs and re-asks the trust question.
func (m *UI) untrustProjectConfigs() tea.Cmd {
	store := m.com.ConfigStore()
	if store == nil {
		return nil
	}
	paths := store.TrustedProjectPaths()
	if len(paths) == 0 {
		return nil
	}
	return m.promptProjectTrust(paths)
}

// promptProjectTrust clears previous decisions for the given project
// configs in the background, reconciles the runtime with the reloaded
// config, and opens the trust dialog to ask again.
func (m *UI) promptProjectTrust(paths []string) tea.Cmd {
	store := m.com.ConfigStore()
	if store == nil {
		return nil
	}
	return func() tea.Msg {
		if err := store.PromptProjectTrust(context.Background(), paths); err != nil {
			return util.NewErrorMsg(fmt.Errorf("failed to re-open project trust: %w", err))
		}
		m.com.Workspace.ApplyTrustedConfig(context.Background())
		return promptProjectTrustMsg{}
	}
}

// promptProjectTrustMsg is emitted once previous trust decisions have
// been cleared and the configs are awaiting a new decision.
type promptProjectTrustMsg struct{}

// handlePromptProjectTrust opens the trust dialog after the trust store
// has been updated.
func (m *UI) handlePromptProjectTrust() tea.Cmd {
	return m.openTrustDialog()
}
