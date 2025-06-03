package anim

import (
	"image/color"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/spinner"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/google/uuid"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

const (
	charCyclingFPS  = time.Second / 22
	colorCycleFPS   = time.Second / 5
	maxCyclingChars = 120
)

var charRunes = []rune("0123456789abcdefABCDEF~!@#$£€%^&*()+=_")

type charState int

const (
	charInitialState charState = iota
	charCyclingState
	charEndOfLifeState
)

// cyclingChar is a single animated character.
type cyclingChar struct {
	finalValue   rune // if < 0 cycle forever
	currentValue rune
	initialDelay time.Duration
	lifetime     time.Duration
}

func (c cyclingChar) randomRune() rune {
	return (charRunes)[rand.IntN(len(charRunes))] //nolint:gosec
}

func (c cyclingChar) state(start time.Time) charState {
	now := time.Now()
	if now.Before(start.Add(c.initialDelay)) {
		return charInitialState
	}
	if c.finalValue > 0 && now.After(start.Add(c.initialDelay)) {
		return charEndOfLifeState
	}
	return charCyclingState
}

type StepCharsMsg struct {
	id string
}

func stepChars(id string) tea.Cmd {
	return tea.Tick(charCyclingFPS, func(time.Time) tea.Msg {
		return StepCharsMsg{id}
	})
}

type ColorCycleMsg struct {
	id string
}

func cycleColors(id string) tea.Cmd {
	return tea.Tick(colorCycleFPS, func(time.Time) tea.Msg {
		return ColorCycleMsg{id}
	})
}

type Animation interface {
	util.Model
	ID() string
}

// anim is the model that manages the animation that displays while the
// output is being generated.
type anim struct {
	start           time.Time
	cyclingChars    []cyclingChar
	labelChars      []cyclingChar
	ramp            []lipgloss.Style
	label           []rune
	ellipsis        spinner.Model
	ellipsisStarted bool
	id              string
	cachedTheme     theme.Theme
	cachedTextStyle lipgloss.Style
}

type animOption func(*anim)

func WithId(id string) animOption {
	return func(a *anim) {
		a.id = id
	}
}

func New(cyclingCharsSize uint, label string, opts ...animOption) Animation {
	// #nosec G115
	n := min(int(cyclingCharsSize), maxCyclingChars)

	gap := " "
	if n == 0 {
		gap = ""
	}

	id := uuid.New()
	t := theme.CurrentTheme()
	c := anim{
		start:           time.Now(),
		label:           []rune(gap + label),
		ellipsis:        spinner.New(spinner.WithSpinner(spinner.Ellipsis)),
		id:              id.String(),
		cachedTheme:     t,
		cachedTextStyle: styles.BaseStyle().Foreground(t.Text()),
	}

	for _, opt := range opts {
		opt(&c)
	}

	// If we're in truecolor mode (and there are enough cycling characters)
	// color the cycling characters with a gradient ramp.
	const minRampSize = 3
	if n >= minRampSize {
		// Note: double capacity for color cycling as we'll need to reverse and
		// append the ramp for seamless transitions.
		c.ramp = make([]lipgloss.Style, n, n*2) //nolint:mnd
		ramp := makeGradientRamp(n, t)
		for i, color := range ramp {
			c.ramp[i] = lipgloss.NewStyle().Foreground(color)
		}
		c.ramp = append(c.ramp, reverse(c.ramp)...) // reverse and append for color cycling
	}

	makeDelay := func(a int32, b time.Duration) time.Duration {
		return time.Duration(rand.Int32N(a)) * (time.Millisecond * b) //nolint:gosec
	}

	makeInitialDelay := func() time.Duration {
		return makeDelay(8, 60) //nolint:mnd
	}

	// Initial characters that cycle forever.
	c.cyclingChars = make([]cyclingChar, n)

	for i := range n {
		c.cyclingChars[i] = cyclingChar{
			finalValue:   -1, // cycle forever
			initialDelay: makeInitialDelay(),
		}
	}

	// Label text that only cycles for a little while.
	c.labelChars = make([]cyclingChar, len(c.label))

	for i, r := range c.label {
		c.labelChars[i] = cyclingChar{
			finalValue:   r,
			initialDelay: makeInitialDelay(),
			lifetime:     makeDelay(5, 180), //nolint:mnd
		}
	}

	return c
}

// Init initializes the animation.
func (a anim) Init() tea.Cmd {
	return tea.Batch(stepChars(a.id), cycleColors(a.id))
}

// Update handles messages.
func (a anim) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case StepCharsMsg:
		if msg.id != a.id {
			return a, nil
		}
		a.updateChars(&a.cyclingChars)
		a.updateChars(&a.labelChars)

		if !a.ellipsisStarted {
			var eol int
			for _, c := range a.labelChars {
				if c.state(a.start) == charEndOfLifeState {
					eol++
				}
			}
			if eol == len(a.label) {
				// If our entire label has reached end of life, start the
				// ellipsis "spinner" after a short pause.
				a.ellipsisStarted = true
				cmd = tea.Tick(time.Millisecond*220, func(time.Time) tea.Msg { //nolint:mnd
					return a.ellipsis.Tick()
				})
			}
		}

		return a, tea.Batch(stepChars(a.id), cmd)
	case ColorCycleMsg:
		if msg.id != a.id {
			return a, nil
		}
		const minColorCycleSize = 2
		if len(a.ramp) < minColorCycleSize {
			return a, nil
		}
		a.ramp = append(a.ramp[1:], a.ramp[0])
		return a, cycleColors(a.id)
	case spinner.TickMsg:
		var cmd tea.Cmd
		a.ellipsis, cmd = a.ellipsis.Update(msg)
		return a, cmd
	default:
		return a, nil
	}
}

func (a anim) ID() string {
	return a.id
}

func (a *anim) updateChars(chars *[]cyclingChar) {
	for i, c := range *chars {
		switch c.state(a.start) {
		case charInitialState:
			(*chars)[i].currentValue = '.'
		case charCyclingState:
			(*chars)[i].currentValue = c.randomRune()
		case charEndOfLifeState:
			(*chars)[i].currentValue = c.finalValue
		}
	}
}

// View renders the animation.
func (a anim) View() tea.View {
	// Pre-allocate builder capacity to avoid reallocations.
	// Estimate: cycling chars + label chars + ellipsis + style overhead.
	const (
		bytesPerChar = 20 // accounts for ANSI color codes and styling
		bufferSize   = 50 // ellipsis and safety margin
	)
	estimatedCap := len(a.cyclingChars)*bytesPerChar + len(a.labelChars)*bytesPerChar + bufferSize
	var b strings.Builder
	b.Grow(estimatedCap)

	for i, c := range a.cyclingChars {
		if len(a.ramp) > i {
			b.WriteString(a.ramp[i].Render(string(c.currentValue)))
			continue
		}
		b.WriteRune(c.currentValue)
	}

	if len(a.labelChars) > 1 {
		for _, c := range a.labelChars {
			b.WriteString(
				a.cachedTextStyle.Render(string(c.currentValue)),
			)
		}
		b.WriteString(a.cachedTextStyle.Render(a.ellipsis.View()))
	}

	return tea.NewView(b.String())
}

func makeGradientRamp(length int, t theme.Theme) []color.Color {
	startColor := theme.GetColor(t.Primary())
	endColor := theme.GetColor(t.Secondary())
	var (
		c        = make([]color.Color, length)
		start, _ = colorful.Hex(startColor)
		end, _   = colorful.Hex(endColor)
	)
	for i := range length {
		step := start.BlendLuv(end, float64(i)/float64(length))
		c[i] = lipgloss.Color(step.Hex())
	}
	return c
}

func reverse[T any](in []T) []T {
	out := make([]T, len(in))
	copy(out, in[:])
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
