package model

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/remotecontrol"
	"github.com/charmbracelet/crush/internal/ui/util"
)

type remoteControlToggleResultMsg struct {
	enabled bool
	err     error
}

// toggleRemoteControl enables or disables remote control for the current
// session. Disable is synchronous; enable dials the relay off-thread.
func (m *UI) toggleRemoteControl() tea.Cmd {
	if !m.hasSession() {
		return util.ReportWarn("No active session")
	}
	sess := *m.session

	if m.remoteBridge != nil && m.remoteBridge.IsEnabled(sess.ID) {
		if err := m.remoteBridge.Disable(sess.ID); err != nil {
			return util.ReportError(err)
		}
		return util.ReportInfo("Remote control disabled for this session")
	}

	cfg := m.com.Config()
	fileURL, fileUser := "", ""
	if cfg != nil && cfg.Options != nil && cfg.Options.RemoteControl != nil {
		fileURL = cfg.Options.RemoteControl.RelayURL
		fileUser = cfg.Options.RemoteControl.Username
	}
	rcCfg, err := remotecontrol.ResolveConfig(fileURL, fileUser, "", "", "")
	if err != nil {
		return util.ReportError(err)
	}
	if m.remoteBridge == nil {
		m.remoteBridge = remotecontrol.NewBridge(rcCfg, m.com.Workspace)
	}

	bridge := m.remoteBridge
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := bridge.Enable(ctx, sess); err != nil {
			return remoteControlToggleResultMsg{err: err}
		}
		return remoteControlToggleResultMsg{enabled: true}
	}
}

func (m *UI) handleRemoteControlToggleResult(msg remoteControlToggleResultMsg) tea.Cmd {
	if msg.err != nil {
		return util.ReportError(msg.err)
	}
	if !msg.enabled {
		return util.ReportInfo("Remote control disabled")
	}
	n := 0
	if m.remoteBridge != nil {
		n = m.remoteBridge.EnabledCount()
	}
	return util.ReportInfo(fmt.Sprintf("Remote control enabled (%d session(s) shared)", n))
}
