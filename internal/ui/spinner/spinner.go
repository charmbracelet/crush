// Package spinner implements a spinner used to indicate processing is occurring.
package spinner

import (
	"log"
	"strings"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
)

const (
	fps        = 24
	pauseSteps = 72
	width      = 12
)

var blocks = []rune{
	' ',
	'▁',
	'▂',
	'▃',
	'▄',
	'▅',
	'▆',
	'▇',
	'█',
}

// Internal ID management. Used during animating to ensure that frame messages
// are received only by spinner components that sent them.
var lastID int64

func nextID() int {
	return int(atomic.AddInt64(&lastID, 1))
}

type StepMsg struct {
	ID  int
	tag int
}

type Spinner struct {
	id    int
	tag   int
	index int
	pause int
	cells []int
}

func NewSpinner() Spinner {
	return Spinner{
		id:    nextID(),
		cells: make([]int, width),
	}
}

func (s Spinner) Init() tea.Cmd {
	return nil
}

func (s Spinner) Update(msg tea.Msg) (Spinner, tea.Cmd) {
	if _, ok := msg.(StepMsg); ok {
		if msg.(StepMsg).ID != s.id {
			// Reject events from other spinners.
			return s, nil
		}

		if s.pause > 0 {
			s.pause--
			log.Println("pausing", s.pause)
		} else {
			s.index++
			if s.index > width {
				s.pause = pauseSteps
				s.index = 0
			}

		}

		for i, c := range s.cells {
			if s.index == i {
				s.cells[i] = len(blocks) - 1
			} else {
				s.cells[i] = max(0, c-1)
			}
		}

		s.tag++
		return s, s.Step()
	}
	return s, nil
}

func (s Spinner) Step() tea.Cmd {
	return tea.Tick(time.Second/time.Duration(fps), func(t time.Time) tea.Msg {
		return StepMsg{ID: s.id}
	})
}

func (s Spinner) View() string {
	if len(blocks) == 0 {
		return ""
	}

	var b strings.Builder
	for i := range s.cells {
		b.WriteRune(blocks[s.cells[i]])
	}

	return b.String()
}
