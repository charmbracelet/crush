package model

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/permission"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// press sends one key to the UI, as the runtime would.
func press(m *UI, code rune) {
	msg := tea.KeyPressMsg{Code: code}
	if code >= 0x20 && code != 0x7f {
		msg.Text = string(code)
	}
	_, _ = m.Update(msg)
}

// modalUI is a busy-capable UI whose prompt is in mode.
func modalUI(t *testing.T, mode string, busy bool) (*UI, *countingWorkspace) {
	t.Helper()
	pinTTLs(t)
	ws := &countingWorkspace{ready: true, agentBusy: busy}
	m := newBusyUI(ws)
	m.promptHistory.index = -1 // as New makes it: not browsing the history
	warmCaches(m, busy)
	m.prompt.SetMode(mode)
	m.prompt.SetWidth(80)
	return m, ws
}

// In vim and Helix modes Enter breaks the line while inserting and sends
// the message only in normal mode.
func TestModalEnterSendsOnlyInNormalMode(t *testing.T) {
	for _, mode := range []string{"vim", "helix"} {
		t.Run(mode, func(t *testing.T) {
			m, _ := modalUI(t, mode, false)
			press(m, 'i')
			press(m, 'a')
			press(m, tea.KeyEnter)
			press(m, 'b')
			require.Equal(t, "a\nb", m.prompt.Value(), "enter in insert mode is a newline")
			require.False(t, m.isAgentBusy(), "enter in insert mode sends nothing")

			press(m, tea.KeyEscape)
			require.Equal(t, "NORMAL", m.prompt.ModeName())
			press(m, tea.KeyEnter)
			require.True(t, m.isAgentBusy(), "enter in normal mode sends")
			require.Empty(t, m.prompt.Value())
		})
	}
}

// A key waiting for its character takes Enter too: it never sends.
func TestModalEnterAfterPendingKeyDoesNotSend(t *testing.T) {
	for _, mode := range []string{"vim", "helix"} {
		t.Run(mode, func(t *testing.T) {
			m, _ := modalUI(t, mode, false)
			m.prompt.SetValue("abc")
			press(m, 'f')
			press(m, tea.KeyEnter)
			require.False(t, m.isAgentBusy())
			require.Equal(t, "abc", m.prompt.Value())
		})
	}
}

// In vim and Helix modes Esc leaves insert mode first; only Esc in normal
// mode arms and then cancels the running request.
func TestModalEscCancelsOnlyInNormalMode(t *testing.T) {
	for _, mode := range []string{"vim", "helix"} {
		t.Run(mode, func(t *testing.T) {
			m, ws := modalUI(t, mode, true)
			press(m, 'i')
			require.Equal(t, "INSERT", m.prompt.ModeName())

			press(m, tea.KeyEscape)
			require.Equal(t, "NORMAL", m.prompt.ModeName(), "esc leaves insert mode")
			require.False(t, m.isCanceling, "esc in insert mode does not arm the cancel")

			press(m, tea.KeyEscape)
			require.True(t, m.isCanceling, "esc in normal mode arms the cancel")
			press(m, tea.KeyEscape)
			require.Equal(t, 1, ws.cancelCalls, "the second esc cancels")
		})
	}
}

// Emacs mode keeps Esc as the cancel key.
func TestEmacsEscCancels(t *testing.T) {
	m, ws := modalUI(t, "emacs", true)
	press(m, tea.KeyEscape)
	require.True(t, m.isCanceling)
	press(m, tea.KeyEscape)
	require.Equal(t, 1, ws.cancelCalls)
}

// The status shows the prompt's mode before the plan or permission badge.
func TestStatusShowsEditorMode(t *testing.T) {
	m, _ := modalUI(t, "vim", false)
	st := m.status

	st.SetEditorMode("")
	require.Empty(t, st.modeBadge())

	st.SetEditorMode("NORMAL")
	require.Equal(t, "NORMAL", strings.TrimSpace(ansi.Strip(st.modeBadge())))

	st.SetMode(uiInputModePlan, permission.PermissionModeNormal)
	st.SetEditorMode("INSERT")
	badge := ansi.Strip(st.modeBadge())
	require.Less(t, strings.Index(badge, "INSERT"), strings.Index(badge, "PLAN MODE"))
}

// The which-key popup of pending keys is drawn at the screen's far right,
// above the help, and scrolls on ctrl+d.
func TestPromptKeyHintsDrawnAtFarRight(t *testing.T) {
	m, _ := modalUI(t, "vim", false)
	m.updateLayoutAndSize()
	press(m, 'd')

	draw := func() []string {
		scr := uv.NewScreenBuffer(m.width, m.height)
		m.drawPromptKeyHints(scr, m.layout.status)
		return strings.Split(ansi.Strip(scr.Render()), "\n")
	}
	lines := draw()
	top := -1
	for i, l := range lines {
		if strings.Contains(l, "╭") {
			top = i
		}
	}
	require.GreaterOrEqual(t, top, 0, "popup drawn:\n%s", strings.Join(lines, "\n"))
	border := lines[top]
	left := ansi.StringWidth(border[:strings.Index(border, "╭")])
	require.Greater(t, left, m.width/2, "popup in the right half")
	require.Equal(t, m.width, ansi.StringWidth(strings.TrimRight(border, " ")), "popup flush with the right edge")
	require.Contains(t, lines[top+1], "of 13", "the d operator's hints")
	require.Less(t, top, m.layout.status.Min.Y, "popup above the help")

	require.True(t, m.scrollPromptKeyHints(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}))
	require.Equal(t, "O-PENDING", m.prompt.ModeName(), "scrolling keeps d pending")

	press(m, tea.KeyEscape)
	for _, l := range draw() {
		require.NotContains(t, l, "of 13", "popup gone after esc")
	}
}
