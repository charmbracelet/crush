package model

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Title animation tuning.
const (
	// titleBlinkInterval is the delay between cursor blink toggles.
	titleBlinkInterval = 220 * time.Millisecond
	// titleStreamInterval is the delay between revealing each title rune.
	titleStreamInterval = 28 * time.Millisecond
	// titleBlinkCount is how many times the cursor blinks before streaming.
	titleBlinkCount = 2
	// titleCursorGlyph is the block drawn as the blinking/streaming cursor.
	titleCursorGlyph = "▌"
)

// titlePhase is the current stage of the title reveal animation.
type titlePhase int

const (
	titlePhaseBlink titlePhase = iota
	titlePhaseStream
)

// titleAnimState tracks the state of the session-title reveal animation: a
// dim cursor blinks a couple of times and then streams the new title in
// before disappearing.
type titleAnimState struct {
	active   bool
	target   string
	phase    titlePhase
	cursorOn bool
	blinks   int
	revealed int
	// gen guards against stale ticks when a new animation starts mid-flight.
	gen int
}

// titleAnimTickMsg advances the title animation. gen is matched against the
// active animation so ticks from a superseded animation are ignored.
type titleAnimTickMsg struct {
	gen int
}

func titleAnimTickCmd(gen int, d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return titleAnimTickMsg{gen: gen}
	})
}

// startTitleAnimation begins animating the reveal of newTitle. It supersedes
// any in-flight animation.
func (m *UI) startTitleAnimation(newTitle string) tea.Cmd {
	gen := m.titleAnim.gen + 1
	m.titleAnim = titleAnimState{
		active:   true,
		target:   newTitle,
		phase:    titlePhaseBlink,
		cursorOn: true,
		gen:      gen,
	}
	return titleAnimTickCmd(gen, titleBlinkInterval)
}

// handleTitleAnimTick advances the title animation one step and schedules the
// next tick, returning nil once the animation completes.
func (m *UI) handleTitleAnimTick(msg titleAnimTickMsg) tea.Cmd {
	if !m.titleAnim.active || msg.gen != m.titleAnim.gen {
		return nil
	}

	switch m.titleAnim.phase {
	case titlePhaseBlink:
		m.titleAnim.cursorOn = !m.titleAnim.cursorOn
		// Count a blink each time the cursor goes dark.
		if !m.titleAnim.cursorOn {
			m.titleAnim.blinks++
		}
		if m.titleAnim.blinks >= titleBlinkCount {
			m.titleAnim.phase = titlePhaseStream
			m.titleAnim.cursorOn = true
			m.titleAnim.revealed = 0
			return titleAnimTickCmd(m.titleAnim.gen, titleStreamInterval)
		}
		return titleAnimTickCmd(m.titleAnim.gen, titleBlinkInterval)

	case titlePhaseStream:
		runes := []rune(m.titleAnim.target)
		if m.titleAnim.revealed >= len(runes) {
			// Cursor disappears; the static title is shown from now on.
			m.titleAnim.active = false
			return nil
		}
		m.titleAnim.revealed++
		return titleAnimTickCmd(m.titleAnim.gen, titleStreamInterval)
	}

	return nil
}

// renderSessionTitle renders the session title within the given base style and
// width, drawing the blinking/streaming animation when one is active.
func (m *UI) renderSessionTitle(base lipgloss.Style, width int) string {
	if !m.titleAnim.active {
		return base.Width(width).MaxHeight(2).Render(m.session.Title)
	}

	container := lipgloss.NewStyle().Width(width).MaxHeight(2)
	cursor := base.Faint(true).Render(titleCursorGlyph)
	var text string
	switch m.titleAnim.phase {
	case titlePhaseBlink:
		if m.titleAnim.cursorOn {
			text = cursor
		}
	case titlePhaseStream:
		runes := []rune(m.titleAnim.target)
		n := min(m.titleAnim.revealed, len(runes))
		text = base.Render(string(runes[:n])) + cursor
	}
	return container.Render(text)
}
