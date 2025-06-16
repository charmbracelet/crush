package anim

import (
	"fmt"
	"image/color"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/spinner"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/google/uuid"
	"github.com/lucasb-eyer/go-colorful"
)

const (
	charCyclingFPS  = time.Second / 8 // Reduced from 22 to 8 FPS for better CPU efficiency
	colorCycleFPS   = time.Second / 3 // Reduced from 5 to 3 FPS
	maxCyclingChars = 60              // Reduced from 120 to 60 characters
)

var (
	charRunes    = []rune("0123456789abcdefABCDEF~!@#$£€%^&*()+=_")
	charRunePool = make([]rune, 1000) // Pre-generated pool of random characters
	poolIndex    = 0
)

func init() {
	// Pre-populate the character pool to avoid runtime random generation
	for i := range charRunePool {
		charRunePool[i] = charRunes[rand.IntN(len(charRunes))]
	}
}

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
	// Use pre-generated pool instead of runtime random generation
	poolIndex = (poolIndex + 1) % len(charRunePool)
	return charRunePool[poolIndex]
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
	c := anim{
		start:    time.Now(),
		label:    []rune(gap + label),
		ellipsis: spinner.New(spinner.WithSpinner(spinner.Ellipsis)),
		id:       id.String(),
	}

	for _, opt := range opts {
		opt(&c)
	}

	// If we're in truecolor mode (and there are enough cycling characters)
	// color the cycling characters with a gradient ramp.
	const minRampSize = 3
	if n >= minRampSize {
		// Optimized: single capacity allocation for color cycling
		c.ramp = make([]lipgloss.Style, 0, n*2)
		ramp := makeGradientRamp(n)
		for _, color := range ramp {
			c.ramp = append(c.ramp, lipgloss.NewStyle().Foreground(color))
		}
		// Create reversed copy for seamless color cycling
		reversed := make([]lipgloss.Style, len(c.ramp))
		for i, style := range c.ramp {
			reversed[len(c.ramp)-1-i] = style
		}
		c.ramp = append(c.ramp, reversed...)
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
	charSlice := *chars // dereference to avoid repeated pointer access
	for i, c := range charSlice {
		switch c.state(a.start) {
		case charInitialState:
			charSlice[i].currentValue = '.'
		case charCyclingState:
			charSlice[i].currentValue = c.randomRune()
		case charEndOfLifeState:
			charSlice[i].currentValue = c.finalValue
		}
	}
}

// View renders the animation.
func (a anim) View() tea.View {
	var (
		t = styles.CurrentTheme()
		b strings.Builder
	)

	// Optimized capacity calculation to reduce allocations
	const (
		bytesPerChar = 15 // Reduced estimate for ANSI styling
		bufferSize   = 30 // Reduced safety margin
	)
	estimatedCap := len(a.cyclingChars)*bytesPerChar + len(a.labelChars)*bytesPerChar + bufferSize
	b.Grow(estimatedCap)

	// Render cycling characters with gradient (if available)
	for i, c := range a.cyclingChars {
		if len(a.ramp) > i {
			b.WriteString(a.ramp[i].Render(string(c.currentValue)))
		} else {
			b.WriteRune(c.currentValue)
		}
	}

	// Render label characters and ellipsis
	if len(a.labelChars) > 1 {
		textStyle := t.S().Text
		for _, c := range a.labelChars {
			b.WriteString(textStyle.Render(string(c.currentValue)))
		}
		b.WriteString(textStyle.Render(a.ellipsis.View()))
	}

	return tea.NewView(b.String())
}

func GetColor(c color.Color) string {
	rgba := color.RGBAModel.Convert(c).(color.RGBA)
	return fmt.Sprintf("#%02x%02x%02x", rgba.R, rgba.G, rgba.B)
}

func makeGradientRamp(length int) []color.Color {
	t := styles.CurrentTheme()
	startColor := GetColor(t.Primary)
	endColor := GetColor(t.Secondary)
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
