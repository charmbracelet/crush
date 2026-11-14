package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/stretchr/testify/require"
)

// The auto-hide timeout is armed from the UI's Update tail instead of being
// handed back to whichever call site happened to scroll. Scroll calls are
// scattered across wheel handling, keybindings, hover scrolling and follow
// mode, so a returned command would be dropped by every one of them that
// ignores it, leaving the scrollbar stuck on screen.
func TestScrollbarHide_ArmedByUpdate(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.chat.scrollbarMode = config.ScrollbarDefault

	u.Update(common.CoalescedWheelMsg{
		Mouse:  tea.Mouse{X: u.layout.main.Min.X + 1, Y: u.layout.main.Min.Y + 1},
		DeltaY: 3,
	})
	require.True(t, u.chat.scrollbarVisible, "wheeling must reveal the scrollbar")
	require.Equal(t, u.chat.scrollbarHideSeq, u.chat.scrollbarHideArmed,
		"the update must arm the hide timeout")

	// The armed timeout fires.
	_, _ = u.Update(scrollbarHideMsg{seq: u.chat.scrollbarHideSeq})
	require.False(t, u.chat.scrollbarVisible, "the timeout must hide the scrollbar")
	require.Zero(t, u.chat.scrollbarHideArmed, "a hidden scrollbar has no timeout outstanding")
	require.Nil(t, u.chat.EnsureScrollbarHide(), "nothing left to arm")
}

func TestScrollbarHide_ScrollPushesTheDeadlineOut(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.chat.scrollbarMode = config.ScrollbarDefault

	require.Nil(t, u.chat.EnsureScrollbarHide(), "an invisible scrollbar has no timeout")

	u.chat.ScrollBy(-2)
	require.NotNil(t, u.chat.EnsureScrollbarHide(), "a visible scrollbar must be armed")
	require.Nil(t, u.chat.EnsureScrollbarHide(), "one tick per deadline")

	u.chat.ScrollBy(-2)
	require.NotEqual(t, u.chat.scrollbarHideArmed, u.chat.scrollbarHideSeq,
		"scrolling again must open a new deadline")
	require.NotNil(t, u.chat.EnsureScrollbarHide())

	stale := u.chat.scrollbarHideSeq - 1
	_, _ = u.Update(scrollbarHideMsg{seq: stale})
	require.True(t, u.chat.scrollbarVisible, "a stale timeout must not hide a freshly scrolled scrollbar")

	_, _ = u.Update(scrollbarHideMsg{seq: u.chat.scrollbarHideSeq})
	require.False(t, u.chat.scrollbarVisible, "the current timeout must hide the scrollbar")
}

func TestScrollbarHide_SettingMessagesDisarms(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.chat.scrollbarMode = config.ScrollbarDefault

	u.chat.ScrollBy(-2)
	require.NotNil(t, u.chat.EnsureScrollbarHide())

	u.chat.SetMessages(frameTestItems("fresh")...)
	require.False(t, u.chat.scrollbarVisible, "a new session starts with the scrollbar hidden")
	require.Zero(t, u.chat.scrollbarHideArmed, "the outgoing session's timeout must be forgotten")
	require.Nil(t, u.chat.EnsureScrollbarHide())
}

func TestScrollbarHide_NonDefaultModesNeverArm(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{config.ScrollbarAlways, config.ScrollbarNever} {
		u := newFrameTestUI(t)
		u.chat.scrollbarMode = mode

		u.chat.ScrollBy(-2)
		require.Nil(t, u.chat.EnsureScrollbarHide(), "%s mode must not schedule a hide", mode)

		_, _ = u.Update(scrollbarHideMsg{seq: u.chat.scrollbarHideSeq})
		require.False(t, u.chat.scrollbarVisible, "%s mode must not toggle visibility", mode)
	}
}
