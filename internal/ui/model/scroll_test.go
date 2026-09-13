package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyChatScroll_LargeDeltaIsNotRewoundBySelection(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.chat.ScrollToBottom()
	u.chat.SelectLast()
	before := u.chat.Offset()

	u.applyChatScroll(-60)
	require.Equal(t, before-60, u.chat.Offset(), "selection follow must not rewind the viewport")
	require.True(t, u.chat.SelectedItemInView())

	u.applyChatScroll(60)
	require.True(t, u.chat.AtBottom())
	require.Equal(t, u.chat.Len()-1, u.chat.Selected(), "reaching the bottom must select the last item")
}

func TestApplyChatScroll_OutOfViewSelectionGoesToNearestEdge(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.chat.ScrollToBottom()

	// Selection far above the viewport, user scrolls up: it should land on
	// the top row (nearest edge), not jump to the bottom row.
	u.chat.SetSelected(0)
	u.applyChatScroll(-5)
	got := u.chat.Selected()
	u.chat.SelectFirstInView()
	require.Equal(t, u.chat.Selected(), got, "selection above viewport must snap to the top edge")

	// Selection below the viewport, user scrolls down: bottom row.
	u.chat.ScrollToTop()
	u.chat.SetSelected(u.chat.Len() - 1)
	u.applyChatScroll(5)
	got = u.chat.Selected()
	u.chat.SelectLastInView()
	require.Equal(t, u.chat.Selected(), got, "selection below viewport must snap to the bottom edge")
}
