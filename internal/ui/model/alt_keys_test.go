package model

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/dialog"
)

var (
	altS = tea.KeyPressMsg{Code: 's', Mod: tea.ModAlt}
	altN = tea.KeyPressMsg{Code: 'n', Mod: tea.ModAlt}
	altM = tea.KeyPressMsg{Code: 'm', Mod: tea.ModAlt}
	esc  = tea.KeyPressMsg{Code: tea.KeyEscape}
)

// modelsConfig is just enough config for the Models dialog to open offline.
func modelsConfig() *config.Config {
	providers := csync.NewMap[string, config.ProviderConfig]()
	providers.Set("p", config.ProviderConfig{ID: "p", Models: []catwalk.Model{{ID: "a"}}})
	return &config.Config{
		Options:   &config.Options{DisableDefaultProviders: true, DisableProviderAutoUpdate: true},
		Providers: providers,
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {Provider: "p", Model: "a"},
		},
	}
}

// The alt keys sit next to the old ctrl keys, which keep working.
func TestAltKeysOpenSessionsNewModels(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		keys     []tea.KeyPressMsg
		wantID   string
		wantFree bool // The key starts a fresh session instead of a dialog.
	}{
		{"alt+s", []tea.KeyPressMsg{altS}, dialog.SessionsID, false},
		{"ctrl+s", []tea.KeyPressMsg{{Code: 's', Mod: tea.ModCtrl}}, dialog.SessionsID, false},
		{"alt+m", []tea.KeyPressMsg{altM}, dialog.ModelsID, false},
		{"ctrl+l", []tea.KeyPressMsg{{Code: 'l', Mod: tea.ModCtrl}}, dialog.ModelsID, false},
		{"alt+n", []tea.KeyPressMsg{altN}, "", true},
		{"ctrl+n", []tea.KeyPressMsg{{Code: 'n', Mod: tea.ModCtrl}}, "", true},
	} {
		for _, focus := range []uiFocusState{uiFocusEditor, uiFocusMain} {
			m := newPanelUI(&countingWorkspace{ready: true, sessions: makeSessions(2), cfg: modelsConfig()})
			m.focus = focus
			for _, k := range tc.keys {
				// Hộp thoại Sessions mở khi danh sách session (nạp bằng Cmd) về.
				pressAndLoad(m, k)
			}
			if tc.wantFree {
				require.Nil(t, m.session, "%s focus %d", tc.name, focus)
				require.Equal(t, uiLanding, m.state, "%s focus %d", tc.name, focus)
				continue
			}
			require.True(t, m.dialog.ContainsDialog(tc.wantID), "%s focus %d", tc.name, focus)
		}
	}
}

// Esc in a dialog closes only the dialog: the run is not cancelled and the
// next esc in the chat is a first press, not the second one.
func TestEscInDialogDoesNotCancelRun(t *testing.T) {
	t.Parallel()

	for _, open := range []tea.KeyPressMsg{altS, altM} {
		m, ws := probedUI(t, "s1")
		ws.cfg = modelsConfig()
		pressAndLoad(m, open)
		require.True(t, m.dialog.HasDialogs(), open.String())

		m.handleKeyPressMsg(esc)
		require.False(t, m.dialog.HasDialogs(), open.String())
		require.False(t, m.isCanceling, open.String())
		require.Zero(t, ws.cancelCalls, open.String())

		m.handleKeyPressMsg(esc)
		require.True(t, m.isCanceling, "first esc in chat only arms: %s", open)
		require.Zero(t, ws.cancelCalls, "first esc in chat never cancels: %s", open)
	}
}

// Enter inside the Sessions dialog picks a session; it must not send the
// draft sitting in the editor.
func TestEnterInSessionsDialogKeepsDraft(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{ready: true, sessions: makeSessions(2)}
	m := newPanelUI(ws)
	m.textarea.SetValue("draft")

	pressAndLoad(m, altS)
	require.True(t, m.dialog.ContainsDialog(dialog.SessionsID))
	m.handleKeyPressMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	require.Equal(t, "draft", m.textarea.Value())
}

// The chat footer advertises the alt keys in this order, next to ctrl+p.
func TestChatShortHelpShowsAltKeys(t *testing.T) {
	t.Parallel()

	m := newPanelUI(&countingWorkspace{ready: true, sessions: makeSessions(2)})
	var got []string
	for _, b := range m.ShortHelp() {
		got = append(got, b.Help().Key+" "+b.Help().Desc)
	}
	require.Contains(t, strings.Join(got, "|"), "ctrl+p commands")
	i := slices.Index(got, "alt+w workspace")
	require.GreaterOrEqual(t, i, 0, "%q", got)
	require.Equal(t, []string{"alt+w workspace", "alt+s sessions", "alt+n new", "alt+m model"}, got[i:i+4])

	var full []string
	for _, row := range m.FullHelp() {
		for _, b := range row {
			full = append(full, b.Help().Key+" "+b.Help().Desc)
		}
	}
	for _, want := range []string{"ctrl+g less", "alt+w workspace", "alt+s sessions", "alt+n new", "alt+m model", "ctrl+y toggle yolo"} {
		require.Contains(t, full, want)
	}
}
