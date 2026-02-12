package model

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

var lastMouseEvent time.Time

func MouseEventFilter(m tea.Model, msg tea.Msg) tea.Msg {
	switch msg.(type) {
	case tea.MouseWheelMsg:
		now := time.Now()
		// Mouse wheel can send events very rapidly.
		// Throttle to prevent erratic scrolling behavior.
		if now.Sub(lastMouseEvent) < 50*time.Millisecond {
			return nil
		}
		lastMouseEvent = now
	case tea.MouseMotionMsg:
		now := time.Now()
		// Trackpad sends many motion events during drag.
		if now.Sub(lastMouseEvent) < 15*time.Millisecond {
			return nil
		}
		lastMouseEvent = now
	}
	return msg
}
