package model

// Plan mode và YOLO áp cho cả workspace (mọi session), không riêng session đang
// xem: lời báo khi bật/tắt phải nói đúng điều đó (T15).

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/ui/util"
)

// infoOf tìm InfoMsg thuộc loại typ trong các message lá của cmd.
func infoOf(t *testing.T, cmd tea.Cmd, typ util.InfoType) util.InfoMsg {
	t.Helper()
	for _, msg := range leafMsgs(cmd) {
		if info, ok := msg.(util.InfoMsg); ok && info.Type == typ {
			return info
		}
	}
	t.Fatalf("no info message of type %d", typ)
	return util.InfoMsg{}
}

func TestYoloToggleMessagesSayAllSessions(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	warmCaches(m, false)
	ctrlY := tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl}

	on := infoOf(t, m.handleKeyPressMsg(ctrlY), util.InfoTypeYolo)
	require.Contains(t, on.Msg, "all sessions")
	require.Contains(t, on.Msg, "workspace")
	require.True(t, ws.yolo)

	off := infoOf(t, m.handleKeyPressMsg(ctrlY), util.InfoTypeInfo)
	require.Contains(t, off.Msg, "all sessions")
	require.Contains(t, off.Msg, "workspace")
	require.Contains(t, off.Msg, "disabled")
	require.False(t, ws.yolo)
}

func TestModeSwitchBannersSayAllSessions(t *testing.T) {
	t.Parallel()

	m := newBusyUI(&countingWorkspace{ready: true})
	warmCaches(m, false)

	var plan, yolo util.InfoMsg
	for _, cmd := range m.applyModeSwitch(modeSwitchedMsg{mode: uiInputModePlan}) {
		for _, msg := range leafMsgs(cmd) {
			if info, ok := msg.(util.InfoMsg); ok && info.Type == util.InfoTypePlan {
				plan = info
			}
		}
	}
	for _, cmd := range m.applyModeSwitch(modeSwitchedMsg{mode: uiInputModeCode, yolo: true}) {
		for _, msg := range leafMsgs(cmd) {
			if info, ok := msg.(util.InfoMsg); ok && info.Type == util.InfoTypeYolo {
				yolo = info
			}
		}
	}
	for _, info := range []util.InfoMsg{plan, yolo} {
		require.Contains(t, info.Msg, "all sessions")
		require.Contains(t, info.Msg, "workspace")
	}
}
