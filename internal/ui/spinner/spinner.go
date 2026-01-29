// Package spinner implements a spinner used to indicate processing is occurring.
package spinner

import (
	"image/color"
	"strings"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

const (
	fps        = 24
	pauseSteps = 48
	width      = 12
	lowChar    = '•'
	highChar   = '│'
)

// Internal ID management. Used during animating to ensure that frame messages
// are received only by spinner components that sent them.
var lastID int64

func nextID() int {
	return int(atomic.AddInt64(&lastID, 1))
}

type Config struct {
	LowColor  color.Color
	PeakColor color.Color
}

type StepMsg struct {
	ID  int
	tag int
}

type Spinner struct {
	Config Config
	id     int
	tag    int
	index  int
	pause  int
	cells  []int
	maxAt  []int // frame when cell reached max height
}

func NewSpinner() Spinner {
	return Spinner{
		Config: Config{
			PeakColor: charmtone.Charple,
			LowColor:  charmtone.Blush,
		},
		id:    nextID(),
		index: -1,
		cells: make([]int, width),
		maxAt: make([]int, width),
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
		} else {
			s.index++
			if s.index > width {
				s.pause = pauseSteps
				s.index = -1
			}

		}

		for i, c := range s.cells {
			if s.index == i {
				s.cells[i] = width - 1
				s.maxAt[i] = s.tag
			} else {
				if s.maxAt[i] >= 0 && s.tag-s.maxAt[i] < 8 {
					continue
				}
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
	if width == 0 {
		return ""
	}

	var b strings.Builder
	for i := range s.cells {
		if s.cells[i] == 0 {
			b.WriteRune(lowChar)
			continue
		}
		b.WriteRune(highChar)
	}

	return b.String()
}
