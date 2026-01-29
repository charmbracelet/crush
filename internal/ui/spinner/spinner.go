// Package spinner implements a spinner used to indicate processing is occurring.
package spinner

import (
	"strings"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
)

const (
	fps       = 24
	emptyChar = '░'
)

var blocks = []rune{
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

type StepMsg struct{ ID int }

type Config struct {
	Width int
}

func DefaultConfig() Config {
	return Config{Width: 12}
}

type Spinner struct {
	Config Config
	id     int
	index  int
}

func NewSpinner() Spinner {
	return Spinner{
		id:     nextID(),
		Config: DefaultConfig(),
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
		s.index++
		if s.index > s.Config.Width-1 {
			s.index = 0
		}
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
	var b strings.Builder
	for i := range s.Config.Width {
		if i == s.index {
			b.WriteRune(blocks[len(blocks)-1])
			continue
		}
		b.WriteRune(emptyChar)
	}

	return b.String()
}
