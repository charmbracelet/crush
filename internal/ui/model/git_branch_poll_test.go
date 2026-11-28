package model

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

func TestGitBranchPollDelay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		reports bool
		focused bool
		want    time.Duration
	}{
		{name: "focused", reports: true, focused: true, want: gitBranchPollInterval},
		{name: "unfocused", reports: true, focused: false, want: gitBranchBlurredPollInterval},
		{name: "no focus reporting", reports: false, focused: false, want: gitBranchPollInterval},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ui := &UI{notifyWindowFocused: tt.focused}
			ui.caps.ReportFocusEvents = tt.reports
			require.Equal(t, tt.want, ui.gitBranchPollDelay())
		})
	}
}

func TestGitBranchPollFollowsFocus(t *testing.T) {
	t.Parallel()

	u := newFrameTestUI(t)
	u.caps.ReportFocusEvents = true
	u.notifyWindowFocused = true

	slow := gitBranchPollMsg{gen: u.gitBranchPollGen}
	_, _ = u.Update(tea.BlurMsg{})
	require.Equal(t, gitBranchBlurredPollInterval, u.gitBranchPollDelay(),
		"an unfocused window polls rarely")

	_, _ = u.Update(tea.FocusMsg{})
	require.Equal(t, gitBranchPollInterval, u.gitBranchPollDelay(),
		"regaining focus restores the regular poll")
	require.NotEqual(t, slow.gen, u.gitBranchPollGen,
		"regaining focus restarts the poll timer")

	// The slow timer armed before the focus change is superseded: its
	// tick must not start a second poll chain.
	require.Nil(t, u.handleGitBranchPoll(slow))
	require.NotNil(t, u.handleGitBranchPoll(gitBranchPollMsg{gen: u.gitBranchPollGen}))

	// A focus event while already focused leaves the live timer alone.
	gen := u.gitBranchPollGen
	_, _ = u.Update(tea.FocusMsg{})
	require.Equal(t, gen, u.gitBranchPollGen)
}
