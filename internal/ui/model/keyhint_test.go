package model

import (
	"testing"

	"charm.land/bubbles/v2/help"
	"github.com/charmbracelet/crush/internal/ui/attachments"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/keyhint"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestHeaderUsesPlatformLabel(t *testing.T) {
	u := newPrismTestUI()

	got := renderHeaderDetails(u.com, u.session, 0, false, 120, nil, "")

	require.Contains(t, ansi.Strip(got), keyhint.Label("ctrl+d"))
}

func TestOnboardingUsesPlatformLabel(t *testing.T) {
	u := newTestUI()
	u.com.Workspace = &testWorkspace{cfg: &config.Config{
		Options: &config.Options{InitializeAs: "AGENTS.md"},
	}}

	got := u.initializeView()

	require.Contains(t, ansi.Strip(got), keyhint.Label("ctrl+p"))
}

func TestPillsUsePlatformLabel(t *testing.T) {
	u := newTestUI()
	u.session = &session.Session{
		ID:    "s1",
		Todos: []session.Todo{{Content: "a", Status: session.TodoStatusPending}},
	}
	u.updateLayoutAndSize()
	u.renderPills()

	require.Contains(t, ansi.Strip(u.pillsView), keyhint.Label("ctrl+t"))
}

func TestHelpViewsUsePlatformLabels(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.keyMap = DefaultKeyMap()
	u.attachments = attachments.New(nil, attachments.Keymap{})
	u.com.Workspace = &testWorkspace{cfg: &config.Config{}}

	h := help.New()
	h.SetWidth(200)

	short := h.View(keyhint.Wrap(u))
	h.ShowAll = true
	full := h.View(keyhint.Wrap(u))

	require.Contains(t, short, keyhint.Label("ctrl+c"), "short help should show the platform label")
	require.Contains(t, full, keyhint.Label("ctrl+c"), "full help should show the platform label")

	// Off macOS the label is the spelled-out chord, so there is nothing to
	// assert here; on darwin every rendered hint must have dropped it.
	if keyhint.Label("ctrl") != "ctrl" {
		require.NotContains(t, short, "ctrl", "short help still spells out a control chord")
		require.NotContains(t, full, "ctrl", "full help still spells out a control chord")
	}
}
