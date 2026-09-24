package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
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

func TestScaledWheelLines(t *testing.T) {
	t.Parallel()

	speed := func(v float64) *float64 { return &v }

	type event struct {
		delta float64
		lines int
	}
	tests := []struct {
		name   string
		speed  *float64
		events []event
	}{
		{
			name:   "default speed passes deltas through",
			speed:  nil,
			events: []event{{delta: 3, lines: 3}, {delta: -1, lines: -1}},
		},
		{
			name:   "double speed scales up",
			speed:  speed(2),
			events: []event{{delta: 3, lines: 6}, {delta: -2, lines: -4}},
		},
		{
			name:   "fractional speed accumulates remainder",
			speed:  speed(0.5),
			events: []event{{delta: 1, lines: 0}, {delta: 1, lines: 1}, {delta: 1, lines: 0}, {delta: 1, lines: 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &config.Config{Options: &config.Options{TUI: &config.TUIOptions{ScrollSpeed: tt.speed}}}
			ui := newTestUIWithConfig(t, cfg)
			for _, e := range tt.events {
				require.Equal(t, e.lines, ui.scaledWheelLines(e.delta))
			}
		})
	}
}

func TestScrollSpeedValue(t *testing.T) {
	t.Parallel()

	require.Equal(t, config.DefaultScrollSpeed, (*config.TUIOptions)(nil).ScrollSpeedValue())
	require.Equal(t, config.DefaultScrollSpeed, (&config.TUIOptions{}).ScrollSpeedValue())

	v := 2.5
	require.Equal(t, 2.5, (&config.TUIOptions{ScrollSpeed: &v}).ScrollSpeedValue())

	low := 0.01
	require.Equal(t, 0.1, (&config.TUIOptions{ScrollSpeed: &low}).ScrollSpeedValue())
}
