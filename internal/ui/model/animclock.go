package model

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/anim"
)

// animTickMsg is one frame of the shared animation clock. gen is the clock
// generation the tick was armed for.
type animTickMsg struct{ gen uint64 }

// animClockLostAfter is how long an armed clock may go without its tick
// arriving before ensure assumes the command was lost and arms a new one.
// Generous enough that a slow frame never trips it.
const animClockLostAfter = 2 * time.Second

// animClock drives every spinner in the UI, in the transcript or the chrome.
// There is at most one tick outstanding: a tick carrying an older generation
// belongs to a superseded clock and is ignored, so a delayed tick can never
// run alongside its replacement. The zero value is a stopped, unfrozen clock.
type animClock struct {
	// frozen keeps ghost spinners still: a reloaded session whose agent is
	// idle may hold an assistant message that never got a Finish part.
	frozen bool

	running bool
	gen     uint64
	armedAt time.Time
	// now is an injectable clock for tests (nil == real time).
	now func() time.Time
}

// ensure arms the clock if something wants animating and no tick is
// outstanding. It is the only place a tick is armed.
func (c *animClock) ensure(wanted bool) tea.Cmd {
	if c.frozen || !wanted {
		c.running = false
		return nil
	}
	if c.running && c.clock().Sub(c.armedAt) < animClockLostAfter {
		return nil
	}
	c.running = true
	c.armedAt = c.clock()
	c.gen++
	gen := c.gen
	return tea.Tick(anim.FrameInterval(), func(time.Time) tea.Msg {
		return animTickMsg{gen: gen}
	})
}

// accept consumes a tick, reporting whether it belongs to the live clock.
// Either way the tick is spent, so the clock stays stopped until ensure.
func (c *animClock) accept(msg animTickMsg) bool {
	if msg.gen != c.gen {
		return false
	}
	c.running = false
	return true
}

func (c *animClock) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}
