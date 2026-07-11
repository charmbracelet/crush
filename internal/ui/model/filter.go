package model

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// inputFilterInterval targets roughly one filtered sample per 60 Hz frame.
// This keeps continuous mouse input responsive while bounding how many
// terminal escape samples can queue ahead of later key presses.
const inputFilterInterval = 16 * time.Millisecond

// Filter coalesces high-frequency terminal mouse input before it reaches the
// Bubble Tea update loop. It is safe for use with tea.WithFilter.
type Filter struct {
	now func() time.Time

	lastMotion time.Time
	lastWheel  time.Time
	wheelDX    float64
	wheelDY    float64
}

// NewFilter returns a Bubble Tea input filter that coalesces high-frequency
// terminal mouse input before it reaches the update queue.
func NewFilter() *Filter {
	return &Filter{now: time.Now}
}

// Filter is passed to tea.WithFilter and runs before Bubble Tea dispatches a
// message to the model. Returning nil drops noisy samples at the input
// boundary, which is early enough to keep later key presses from waiting
// behind a mouse flood in the update queue.
func (f *Filter) Filter(_ tea.Model, msg tea.Msg) tea.Msg {
	switch typed := msg.(type) {
	case tea.MouseWheelMsg:
		mouse := typed.Mouse()
		dx, dy := wheelDeltas(mouse)
		if oppositeSign(f.wheelDX, dx) {
			f.wheelDX = 0
		}
		if oppositeSign(f.wheelDY, dy) {
			f.wheelDY = 0
		}
		f.wheelDX += dx
		f.wheelDY += dy
		if !f.allow(&f.lastWheel) {
			return nil
		}
		aggregated := common.CoalescedWheelMsg{
			Mouse:  mouse,
			DeltaX: f.wheelDX,
			DeltaY: f.wheelDY,
		}
		f.wheelDX = 0
		f.wheelDY = 0
		return aggregated

	case tea.MouseMotionMsg:
		if !f.allow(&f.lastMotion) {
			return nil
		}
		return msg
	}

	return msg
}

// allow reports whether enough time has elapsed for the next sample in one
// coalescing stream. Each stream owns a separate last timestamp so wheel and
// motion events can be throttled independently.
func (f *Filter) allow(last *time.Time) bool {
	now := f.now
	if now == nil {
		now = time.Now
	}
	at := now()
	if !last.IsZero() && at.Sub(*last) < inputFilterInterval {
		return false
	}
	*last = at
	return true
}

// wheelDeltas converts Bubble Tea's wheel button direction into signed scroll
// deltas. Bubble Tea represents wheel movement as button presses, so we
// synthesize a unit delta for each sample and let Filter aggregate the units
// when events are coalesced.
func wheelDeltas(mouse tea.Mouse) (float64, float64) {
	switch mouse.Button {
	case tea.MouseWheelUp:
		return 0, -1
	case tea.MouseWheelDown:
		return 0, 1
	case tea.MouseWheelLeft:
		return -1, 0
	case tea.MouseWheelRight:
		return 1, 0
	default:
		return 0, 0
	}
}

func oppositeSign(a, b float64) bool {
	return (a < 0 && b > 0) || (a > 0 && b < 0)
}
